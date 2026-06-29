package kernel

import (
	"errors"
)

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
