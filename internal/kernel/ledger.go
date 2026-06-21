package kernel

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

const maxLedgerLineBytes = 16 * 1024 * 1024

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

	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return ReadyCheck{Status: "blocked", Reason: "ledger_unwritable"}
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return ReadyCheck{Status: "blocked", Reason: "ledger_unwritable"}
	}
	if err := f.Close(); err != nil {
		return ReadyCheck{Status: "blocked", Reason: "ledger_unwritable"}
	}
	return ReadyCheck{Status: "ok"}
}

func (l *JSONLLedger) Append(event StoredEvent) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	encoded, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(encoded, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

func (l *JSONLLedger) Load() ([]StoredEvent, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	f, err := os.Open(l.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []StoredEvent
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024), maxLedgerLineBytes)
	for scanner.Scan() {
		var event StoredEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return events, nil
}
