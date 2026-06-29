package kernel

import (
	"bytes"
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
	"strconv"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const (
	ledgerFrameHeaderBytes        = 8
	sqliteLedgerLockStaleAge      = 2 * time.Minute
	sqliteLedgerMaxFrameBodyBytes = 64 * 1024 * 1024
)

type SQLiteLedger struct {
	path         string
	mu           sync.Mutex
	lockKey      string
	lockAcquired bool
	indexRebuilt bool
}

func NewSQLiteLedger(path string) *SQLiteLedger {
	return &SQLiteLedger{path: path}
}

type sqliteLedgerLockHolder struct {
	file     *os.File
	lockPath string
	refs     int
}

var sqliteLedgerLockRegistry = struct {
	sync.Mutex
	holders map[string]*sqliteLedgerLockHolder
}{
	holders: make(map[string]*sqliteLedgerLockHolder),
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
	tx, err := db.Begin()
	if err != nil {
		return classifySQLiteError(err, ErrLedgerUnwritable)
	}
	_, err = tx.Exec(`INSERT INTO session_events (
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
		_ = tx.Rollback()
		return classifySQLiteError(err, ErrLedgerUnwritable)
	}
	if err := upsertSQLiteSessionFromEvent(tx, event); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return classifySQLiteError(err, ErrLedgerUnwritable)
	}
	return nil
}

func (l *SQLiteLedger) Load() ([]StoredEvent, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.loadLocked()
}

func (l *SQLiteLedger) ListSessions() ([]SessionListItem, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	db, err := l.openDBLocked()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`SELECT session_id, title, updated_at, updated_at_unix_nano FROM sessions ORDER BY updated_at_unix_nano DESC, session_id ASC`)
	if err != nil {
		return nil, classifySQLiteError(err, ErrLedgerUnreadable)
	}
	defer rows.Close()

	var items []SessionListItem
	for rows.Next() {
		var sessionID string
		var title string
		var updatedAtText string
		var updatedAtUnixNano int64
		if err := rows.Scan(&sessionID, &title, &updatedAtText, &updatedAtUnixNano); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrLedgerUnreadable, err)
		}
		updatedAt, err := time.Parse("2006-01-02T15:04:05.999999999Z07:00", updatedAtText)
		if err != nil {
			updatedAt = time.Unix(0, updatedAtUnixNano).UTC()
		}
		items = append(items, SessionListItem{SessionID: sessionID, Title: title, UpdatedAt: updatedAt})
	}
	if err := rows.Err(); err != nil {
		return nil, classifySQLiteError(err, ErrLedgerUnreadable)
	}
	return items, nil
}

func (l *SQLiteLedger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.lockAcquired || l.lockKey == "" {
		return nil
	}
	sqliteLedgerLockRegistry.Lock()
	defer sqliteLedgerLockRegistry.Unlock()

	holder := sqliteLedgerLockRegistry.holders[l.lockKey]
	if holder == nil {
		l.lockAcquired = false
		l.lockKey = ""
		return nil
	}
	holder.refs--
	var closeErr error
	if holder.refs <= 0 {
		closeErr = holder.file.Close()
		removeErr := os.Remove(holder.lockPath)
		if closeErr == nil && removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			closeErr = removeErr
		}
		delete(sqliteLedgerLockRegistry.holders, l.lockKey)
	}
	l.lockAcquired = false
	l.lockKey = ""
	if closeErr != nil {
		return fmt.Errorf("%w: release ledger lock: %w", ErrLedgerUnwritable, closeErr)
	}
	return nil
}

func (l *SQLiteLedger) loadLocked() ([]StoredEvent, error) {
	db, err := l.openDBLocked()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`SELECT event_id, session_id, turn_id, event_type, file_ref, frame_offset, frame_bytes, event_bytes, event_hash FROM session_events ORDER BY created_at_unix_nano ASC, seq ASC`)
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

func (l *SQLiteLedger) rebuildSQLiteIndexFromEventFramesLocked(db *sql.DB) error {
	info, err := os.Stat(l.eventFilesDir())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: %w", ErrLedgerUnreadable, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: session event store is not a directory", ErrLedgerCorrupt)
	}
	tx, err := db.Begin()
	if err != nil {
		return classifySQLiteError(err, ErrLedgerUnwritable)
	}
	walkErr := filepath.WalkDir(l.eventFilesDir(), func(framePath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		fileRef, err := filepath.Rel(filepath.Dir(l.path), framePath)
		if err != nil {
			return err
		}
		fileRef = filepath.ToSlash(fileRef)
		if !strings.HasPrefix(fileRef, "session-events/") {
			return fmt.Errorf("%w: event file outside session-events: %s", ErrLedgerCorrupt, fileRef)
		}
		return l.reconcileSQLiteEventFileLocked(tx, fileRef, framePath)
	})
	if walkErr != nil {
		_ = tx.Rollback()
		if errors.Is(walkErr, ErrLedgerCorrupt) || errors.Is(walkErr, ErrLedgerUnreadable) || errors.Is(walkErr, ErrLedgerUnwritable) {
			return walkErr
		}
		return fmt.Errorf("%w: %w", ErrLedgerUnreadable, walkErr)
	}
	if err := tx.Commit(); err != nil {
		return classifySQLiteError(err, ErrLedgerUnwritable)
	}
	return nil
}

func (l *SQLiteLedger) reconcileSQLiteEventFileLocked(tx *sql.Tx, fileRef string, framePath string) error {
	f, err := os.Open(framePath)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrLedgerUnreadable, err)
	}
	defer f.Close()
	var offset int64
	for {
		header := make([]byte, ledgerFrameHeaderBytes)
		if _, err := io.ReadFull(f, header); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("%w: incomplete frame header in %s at %d: %w", ErrLedgerCorrupt, fileRef, offset, err)
		}
		bodyBytes := binary.BigEndian.Uint64(header)
		if bodyBytes == 0 || bodyBytes > sqliteLedgerMaxFrameBodyBytes {
			return fmt.Errorf("%w: invalid frame body length in %s at %d", ErrLedgerCorrupt, fileRef, offset)
		}
		encoded := make([]byte, int(bodyBytes))
		if _, err := io.ReadFull(f, encoded); err != nil {
			return fmt.Errorf("%w: incomplete frame body in %s at %d: %w", ErrLedgerCorrupt, fileRef, offset, err)
		}
		var event StoredEvent
		if err := json.Unmarshal(encoded, &event); err != nil {
			return fmt.Errorf("%w: unmarshal frame in %s at %d: %w", ErrLedgerCorrupt, fileRef, offset, err)
		}
		if event.EventID == "" {
			return fmt.Errorf("%w: event frame missing event_id in %s at %d", ErrLedgerCorrupt, fileRef, offset)
		}
		if expected := l.sessionEventFileRef(event.SessionID); fileRef != expected {
			return fmt.Errorf("%w: event %s stored in %s, want %s", ErrLedgerCorrupt, event.EventID, fileRef, expected)
		}
		eventHash := sha256.Sum256(encoded)
		row := sqliteLedgerEventRow{
			EventID:     event.EventID,
			SessionID:   event.SessionID,
			TurnID:      event.TurnID,
			EventType:   event.Type,
			FileRef:     fileRef,
			FrameOffset: offset,
			FrameBytes:  ledgerFrameHeaderBytes + len(encoded),
			EventBytes:  len(encoded),
			EventHash:   hex.EncodeToString(eventHash[:]),
		}
		if err := reconcileSQLiteEventRow(tx, row, event); err != nil {
			return err
		}
		offset += int64(row.FrameBytes)
	}
}

func reconcileSQLiteEventRow(tx *sql.Tx, row sqliteLedgerEventRow, event StoredEvent) error {
	var existing sqliteLedgerEventRow
	err := tx.QueryRow(`SELECT event_id, session_id, turn_id, event_type, file_ref, frame_offset, frame_bytes, event_bytes, event_hash FROM session_events WHERE event_id = ?`, row.EventID).
		Scan(&existing.EventID, &existing.SessionID, &existing.TurnID, &existing.EventType, &existing.FileRef, &existing.FrameOffset, &existing.FrameBytes, &existing.EventBytes, &existing.EventHash)
	if err == nil {
		if existing != row {
			return fmt.Errorf("%w: indexed metadata mismatch for %s", ErrLedgerCorrupt, row.EventID)
		}
		return upsertSQLiteSessionFromEvent(tx, event)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return classifySQLiteError(err, ErrLedgerUnreadable)
	}
	_, err = tx.Exec(`INSERT INTO session_events (
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
		row.FileRef,
		row.FrameOffset,
		row.FrameBytes,
		row.EventBytes,
		row.EventHash,
	)
	if err != nil {
		return classifySQLiteError(err, ErrLedgerUnwritable)
	}
	return upsertSQLiteSessionFromEvent(tx, event)
}

func upsertSQLiteSessionFromEvent(tx interface {
	Exec(string, ...interface{}) (sql.Result, error)
}, event StoredEvent) error {
	sessionID := strings.TrimSpace(event.SessionID)
	if sessionID == "" {
		return nil
	}
	title := sqliteSessionTitleFromEvent(event)
	_, err := tx.Exec(`INSERT INTO sessions (
		session_id,
		title,
		updated_at,
		updated_at_unix_nano
	) VALUES (?, ?, ?, ?)
	ON CONFLICT(session_id) DO UPDATE SET
		title = CASE WHEN sessions.title = '' AND excluded.title != '' THEN excluded.title ELSE sessions.title END,
		updated_at = CASE WHEN excluded.updated_at_unix_nano > sessions.updated_at_unix_nano THEN excluded.updated_at ELSE sessions.updated_at END,
		updated_at_unix_nano = CASE WHEN excluded.updated_at_unix_nano > sessions.updated_at_unix_nano THEN excluded.updated_at_unix_nano ELSE sessions.updated_at_unix_nano END`,
		sessionID,
		title,
		event.CreatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
		event.CreatedAt.UnixNano(),
	)
	if err != nil {
		return classifySQLiteError(err, ErrLedgerUnwritable)
	}
	return nil
}

func sqliteSessionTitleFromEvent(event StoredEvent) string {
	if event.Type != "turn.submitted" {
		return ""
	}
	for _, item := range event.Data.InputItems {
		if item.Type != "text" {
			continue
		}
		title := strings.Join(strings.Fields(item.Text), " ")
		if title == "" {
			continue
		}
		const maxRunes = 80
		runes := []rune(title)
		if len(runes) > maxRunes {
			title = string(runes[:maxRunes-3]) + "..."
		}
		return title
	}
	return ""
}

func (l *SQLiteLedger) openDBLocked() (*sql.DB, error) {
	if strings.TrimSpace(l.path) == "" {
		return nil, fmt.Errorf("%w: ledger path is required", ErrLedgerUnwritable)
	}
	if err := l.ensureSingleWriterLocked(); err != nil {
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
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS sessions (
		session_id TEXT PRIMARY KEY,
		title TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL,
		updated_at_unix_nano INTEGER NOT NULL
	)`); err != nil {
		db.Close()
		return nil, classifySQLiteError(err, ErrLedgerUnwritable)
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS sessions_updated_idx ON sessions(updated_at_unix_nano)`); err != nil {
		db.Close()
		return nil, classifySQLiteError(err, ErrLedgerUnwritable)
	}
	if !l.indexRebuilt {
		if err := l.rebuildSQLiteIndexFromEventFramesLocked(db); err != nil {
			db.Close()
			return nil, err
		}
		l.indexRebuilt = true
	}
	return db, nil
}

func (l *SQLiteLedger) ensureSingleWriterLocked() error {
	if l.lockAcquired {
		return nil
	}
	absPath, err := filepath.Abs(filepath.Clean(l.path))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrLedgerUnwritable, err)
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return fmt.Errorf("%w: %w", ErrLedgerUnwritable, err)
	}
	lockPath := absPath + ".lock"

	sqliteLedgerLockRegistry.Lock()
	defer sqliteLedgerLockRegistry.Unlock()

	if holder := sqliteLedgerLockRegistry.holders[absPath]; holder != nil {
		holder.refs++
		l.lockKey = absPath
		l.lockAcquired = true
		return nil
	}
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			recovered, recoverErr := recoverStaleSQLiteLedgerLock(lockPath, time.Now())
			if recoverErr != nil {
				return fmt.Errorf("%w: %w", ErrLedgerUnreadable, recoverErr)
			}
			if recovered {
				file, err = os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
				if err == nil {
					goto writeLock
				}
			}
			return fmt.Errorf("%w: %s", ErrLedgerLocked, lockPath)
		}
		return fmt.Errorf("%w: %w", ErrLedgerUnwritable, err)
	}
writeLock:
	if _, err := fmt.Fprintf(file, "pid=%d\ncreated_at=%s\nledger=%s\n", os.Getpid(), time.Now().Format(time.RFC3339Nano), absPath); err != nil {
		file.Close()
		os.Remove(lockPath)
		return fmt.Errorf("%w: %w", ErrLedgerUnwritable, err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		os.Remove(lockPath)
		return fmt.Errorf("%w: %w", ErrLedgerUnwritable, err)
	}
	sqliteLedgerLockRegistry.holders[absPath] = &sqliteLedgerLockHolder{file: file, lockPath: lockPath, refs: 1}
	l.lockKey = absPath
	l.lockAcquired = true
	return nil
}

type sqliteLedgerLockRecord struct {
	PID       int
	CreatedAt time.Time
}

func recoverStaleSQLiteLedgerLock(lockPath string, now time.Time) (bool, error) {
	info, err := os.Stat(lockPath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	content, err := os.ReadFile(lockPath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	record := parseSQLiteLedgerLockRecord(content)
	createdAt := record.CreatedAt
	if createdAt.IsZero() {
		createdAt = info.ModTime()
	}
	if createdAt.IsZero() || now.Sub(createdAt) < sqliteLedgerLockStaleAge {
		return false, nil
	}
	if record.PID > 0 {
		live, known := sqliteLedgerProcessLiveness(record.PID)
		if !known || live {
			return false, nil
		}
	}
	current, err := os.ReadFile(lockPath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !bytes.Equal(content, current) {
		return false, nil
	}
	if err := os.Remove(lockPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func parseSQLiteLedgerLockRecord(content []byte) sqliteLedgerLockRecord {
	var record sqliteLedgerLockRecord
	for _, line := range strings.Split(string(content), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "pid":
			pid, err := strconv.Atoi(strings.TrimSpace(value))
			if err == nil {
				record.PID = pid
			}
		case "created_at":
			createdAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value))
			if err == nil {
				record.CreatedAt = createdAt
			}
		}
	}
	return record
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
