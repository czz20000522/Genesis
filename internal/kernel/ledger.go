package kernel

import (
	"errors"
)

var (
	ErrLedgerUnwritable = errors.New("ledger unwritable")
	ErrLedgerUnreadable = errors.New("ledger unreadable")
	ErrLedgerCorrupt    = errors.New("ledger corrupt")
	ErrLedgerLocked     = errors.New("ledger locked")
)

type Ledger interface {
	Append(event StoredEvent) error
	Load() ([]StoredEvent, error)
	Ready() ReadyCheck
	Path() string
}
