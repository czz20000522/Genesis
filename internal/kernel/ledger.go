package kernel

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const maxLedgerLineBytes = 16 * 1024 * 1024

var (
	ErrLedgerUnwritable = errors.New("ledger unwritable")
	ErrLedgerUnreadable = errors.New("ledger unreadable")
	ErrLedgerCorrupt    = errors.New("ledger corrupt")
)

type Ledger interface {
	Append(event StoredEvent) error
	Load() ([]StoredEvent, error)
	Ready() ReadyCheck
	Path() string
}

type JSONLLedger struct {
	path string
	mu   sync.Mutex
}

func NewJSONLLedger(path string) *JSONLLedger {
	return &JSONLLedger{path: path}
}

func (l *JSONLLedger) Path() string {
	return l.path
}

func (l *JSONLLedger) Ready() ReadyCheck {
	l.mu.Lock()
	defer l.mu.Unlock()

	f, err := l.openAppendLocked()
	if err != nil {
		return ReadyCheck{Readiness: ReadinessNotReady, ReadinessReason: ledgerErrorCode(err)}
	}
	if err := f.Close(); err != nil {
		return ReadyCheck{Readiness: ReadinessNotReady, ReadinessReason: "ledger_unwritable"}
	}
	if _, err := l.loadLocked(); err != nil {
		return ReadyCheck{Readiness: ReadinessNotReady, ReadinessReason: ledgerErrorCode(err)}
	}
	return ReadyCheck{Readiness: ReadinessReady}
}

func (l *JSONLLedger) Append(event StoredEvent) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	f, err := l.openAppendLocked()
	if err != nil {
		return err
	}
	defer f.Close()

	encoded, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(encoded, '\n')); err != nil {
		return fmt.Errorf("%w: %w", ErrLedgerUnwritable, err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("%w: %w", ErrLedgerUnwritable, err)
	}
	return nil
}

func (l *JSONLLedger) openAppendLocked() (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrLedgerUnwritable, err)
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrLedgerUnwritable, err)
	}
	return f, nil
}

func (l *JSONLLedger) Load() ([]StoredEvent, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.loadLocked()
}

func (l *JSONLLedger) loadLocked() ([]StoredEvent, error) {
	f, err := os.Open(l.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrLedgerUnreadable, err)
	}
	defer f.Close()

	var events []StoredEvent
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024), maxLedgerLineBytes)
	line := 0
	for scanner.Scan() {
		line++
		var event StoredEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, fmt.Errorf("%w: line %d: %w", ErrLedgerCorrupt, line, err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrLedgerUnreadable, err)
	}
	return events, nil
}
