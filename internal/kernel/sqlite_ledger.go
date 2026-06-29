package kernel

import (
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
)

const ledgerFrameHeaderBytes = 8

type SQLiteLedger struct {
	path string
	mu   sync.Mutex
}

func NewSQLiteLedger(path string) *SQLiteLedger {
	return &SQLiteLedger{path: path}
}

func (l *SQLiteLedger) Path() string {
	return l.path
}

func (l *SQLiteLedger) Ready() ReadyCheck {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, err := l.loadLocked(); err != nil {
		return ReadyCheck{Readiness: ReadinessNotReady, ReadinessReason: ledgerErrorCode(err)}
	}
	return ReadyCheck{Readiness: ReadinessReady}
}

func (l *SQLiteLedger) Append(event StoredEvent) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	db, err := l.openDBLocked()
	if err != nil {
		return err
	}
	defer db.Close()

	encoded, err := json.Marshal(event)
	if err != nil {
		return err
	}
	eventHash := sha256.Sum256(encoded)
	fileRef := l.sessionEventFileRef(event.SessionID)
	offset, frameBytes, err := l.appendEventFrameLocked(fileRef, encoded)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO session_events (
		event_id,
		session_id,
		turn_id,
		event_type,
		created_at,
		created_at_unix_nano,
		file_ref,
		frame_offset,
		frame_bytes,
		event_bytes,
		event_hash,
		event_inline
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)`,
		event.EventID,
		event.SessionID,
		event.TurnID,
		event.Type,
		event.CreatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
		event.CreatedAt.UnixNano(),
		fileRef,
		offset,
		frameBytes,
		len(encoded),
		hex.EncodeToString(eventHash[:]),
	)
	if err != nil {
		return classifySQLiteError(err, ErrLedgerUnwritable)
	}
	return nil
}

func (l *SQLiteLedger) Load() ([]StoredEvent, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.loadLocked()
}

func (l *SQLiteLedger) loadLocked() ([]StoredEvent, error) {
	db, err := l.openDBLocked()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`SELECT event_id, session_id, turn_id, event_type, file_ref, frame_offset, frame_bytes, event_bytes, event_hash FROM session_events ORDER BY seq ASC`)
	if err != nil {
		return nil, classifySQLiteError(err, ErrLedgerUnreadable)
	}
	defer rows.Close()

	var events []StoredEvent
	for rows.Next() {
		var row sqliteLedgerEventRow
		if err := rows.Scan(&row.EventID, &row.SessionID, &row.TurnID, &row.EventType, &row.FileRef, &row.FrameOffset, &row.FrameBytes, &row.EventBytes, &row.EventHash); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrLedgerUnreadable, err)
		}
		event, err := l.readEventFrame(row)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, classifySQLiteError(err, ErrLedgerUnreadable)
	}
	return events, nil
}

type sqliteLedgerEventRow struct {
	EventID     string
	SessionID   string
	TurnID      string
	EventType   string
	FileRef     string
	FrameOffset int64
	FrameBytes  int
	EventBytes  int
	EventHash   string
}

func (l *SQLiteLedger) openDBLocked() (*sql.DB, error) {
	if strings.TrimSpace(l.path) == "" {
		return nil, fmt.Errorf("%w: ledger path is required", ErrLedgerUnwritable)
	}
	if err := l.failIfIndexMissingButEventFilesExist(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrLedgerUnwritable, err)
	}
	db, err := sql.Open("sqlite", l.path)
	if err != nil {
		return nil, classifySQLiteError(err, ErrLedgerUnwritable)
	}
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		db.Close()
		return nil, classifySQLiteError(err, ErrLedgerUnwritable)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS session_events (
		seq INTEGER PRIMARY KEY AUTOINCREMENT,
		event_id TEXT NOT NULL UNIQUE,
		session_id TEXT NOT NULL,
		turn_id TEXT NOT NULL,
		event_type TEXT NOT NULL,
		created_at TEXT NOT NULL,
		created_at_unix_nano INTEGER NOT NULL,
		file_ref TEXT NOT NULL,
		frame_offset INTEGER NOT NULL,
		frame_bytes INTEGER NOT NULL,
		event_bytes INTEGER NOT NULL,
		event_hash TEXT NOT NULL,
		event_inline TEXT,
		CHECK (event_inline IS NULL)
	)`); err != nil {
		db.Close()
		return nil, classifySQLiteError(err, ErrLedgerUnwritable)
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS session_events_session_updated_idx ON session_events(session_id, created_at_unix_nano)`); err != nil {
		db.Close()
		return nil, classifySQLiteError(err, ErrLedgerUnwritable)
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS session_events_turn_idx ON session_events(turn_id, seq)`); err != nil {
		db.Close()
		return nil, classifySQLiteError(err, ErrLedgerUnwritable)
	}
	return db, nil
}

func (l *SQLiteLedger) failIfIndexMissingButEventFilesExist() error {
	if _, err := os.Stat(l.path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: %w", ErrLedgerUnreadable, err)
	}
	found := false
	err := filepath.WalkDir(l.eventFilesDir(), func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: %w", ErrLedgerUnreadable, err)
	}
	if found {
		return fmt.Errorf("%w: sqlite index is missing but session event files exist", ErrLedgerCorrupt)
	}
	return nil
}

func (l *SQLiteLedger) appendEventFrameLocked(fileRef string, encoded []byte) (int64, int, error) {
	eventPath, err := l.resolveFileRef(fileRef)
	if err != nil {
		return 0, 0, err
	}
	if err := os.MkdirAll(filepath.Dir(eventPath), 0o755); err != nil {
		return 0, 0, fmt.Errorf("%w: %w", ErrLedgerUnwritable, err)
	}
	f, err := os.OpenFile(eventPath, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0o644)
	if err != nil {
		return 0, 0, fmt.Errorf("%w: %w", ErrLedgerUnwritable, err)
	}
	defer f.Close()
	offset, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, 0, fmt.Errorf("%w: %w", ErrLedgerUnwritable, err)
	}
	frame := make([]byte, ledgerFrameHeaderBytes+len(encoded))
	binary.BigEndian.PutUint64(frame[:ledgerFrameHeaderBytes], uint64(len(encoded)))
	copy(frame[ledgerFrameHeaderBytes:], encoded)
	if _, err := f.Write(frame); err != nil {
		return 0, 0, fmt.Errorf("%w: %w", ErrLedgerUnwritable, err)
	}
	if err := f.Sync(); err != nil {
		return 0, 0, fmt.Errorf("%w: %w", ErrLedgerUnwritable, err)
	}
	return offset, len(frame), nil
}

func (l *SQLiteLedger) readEventFrame(row sqliteLedgerEventRow) (StoredEvent, error) {
	if row.FrameBytes != ledgerFrameHeaderBytes+row.EventBytes {
		return StoredEvent{}, fmt.Errorf("%w: indexed frame length mismatch for %s", ErrLedgerCorrupt, row.EventID)
	}
	eventPath, err := l.resolveFileRef(row.FileRef)
	if err != nil {
		return StoredEvent{}, err
	}
	f, err := os.Open(eventPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return StoredEvent{}, fmt.Errorf("%w: missing event file for %s", ErrLedgerCorrupt, row.EventID)
		}
		return StoredEvent{}, fmt.Errorf("%w: %w", ErrLedgerUnreadable, err)
	}
	defer f.Close()
	frame := make([]byte, row.FrameBytes)
	if _, err := f.ReadAt(frame, row.FrameOffset); err != nil {
		return StoredEvent{}, fmt.Errorf("%w: read frame for %s: %w", ErrLedgerCorrupt, row.EventID, err)
	}
	size := int(binary.BigEndian.Uint64(frame[:ledgerFrameHeaderBytes]))
	if size != row.EventBytes || size != len(frame)-ledgerFrameHeaderBytes {
		return StoredEvent{}, fmt.Errorf("%w: frame header mismatch for %s", ErrLedgerCorrupt, row.EventID)
	}
	encoded := frame[ledgerFrameHeaderBytes:]
	eventHash := sha256.Sum256(encoded)
	if hex.EncodeToString(eventHash[:]) != row.EventHash {
		return StoredEvent{}, fmt.Errorf("%w: event hash mismatch for %s", ErrLedgerCorrupt, row.EventID)
	}
	var event StoredEvent
	if err := json.Unmarshal(encoded, &event); err != nil {
		return StoredEvent{}, fmt.Errorf("%w: unmarshal event %s: %w", ErrLedgerCorrupt, row.EventID, err)
	}
	if event.EventID != row.EventID || event.SessionID != row.SessionID || event.TurnID != row.TurnID || event.Type != row.EventType {
		return StoredEvent{}, fmt.Errorf("%w: indexed metadata mismatch for %s", ErrLedgerCorrupt, row.EventID)
	}
	return event, nil
}

func (l *SQLiteLedger) sessionEventFileRef(sessionID string) string {
	key := strings.TrimSpace(sessionID)
	if key == "" {
		key = "_system"
	}
	sum := sha256.Sum256([]byte(key))
	return path.Join("session-events", hex.EncodeToString(sum[:])+".events")
}

func (l *SQLiteLedger) resolveFileRef(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("%w: empty event file ref", ErrLedgerCorrupt)
	}
	clean := path.Clean("/" + ref)
	clean = strings.TrimPrefix(clean, "/")
	if clean == "." || strings.HasPrefix(clean, "../") || path.IsAbs(ref) {
		return "", fmt.Errorf("%w: invalid event file ref", ErrLedgerCorrupt)
	}
	base := filepath.Clean(filepath.Dir(l.path))
	full := filepath.Clean(filepath.Join(base, filepath.FromSlash(clean)))
	if full != base && !strings.HasPrefix(full, base+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: event file ref escapes ledger root", ErrLedgerCorrupt)
	}
	return full, nil
}

func (l *SQLiteLedger) eventFilesDir() string {
	return filepath.Join(filepath.Dir(l.path), "session-events")
}

func classifySQLiteError(err error, fallback error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "file is not a database") ||
		strings.Contains(message, "database disk image is malformed") ||
		strings.Contains(message, "file is encrypted") ||
		strings.Contains(message, "not a database") {
		return fmt.Errorf("%w: %w", ErrLedgerCorrupt, err)
	}
	return fmt.Errorf("%w: %w", fallback, err)
}
