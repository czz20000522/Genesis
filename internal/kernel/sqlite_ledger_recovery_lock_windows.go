//go:build windows

package kernel

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

const (
	sqliteLedgerLockfileFailImmediately = 0x00000001
	sqliteLedgerLockfileExclusiveLock   = 0x00000002
	sqliteLedgerErrorLockViolation      = syscall.Errno(33)
)

var sqliteLedgerProcLockFileEx = syscall.NewLazyDLL("kernel32.dll").NewProc("LockFileEx")

func tryLockSQLiteLedgerRecoveryFile(file *os.File) (bool, error) {
	var overlapped syscall.Overlapped
	r1, _, err := sqliteLedgerProcLockFileEx.Call(
		file.Fd(),
		sqliteLedgerLockfileExclusiveLock|sqliteLedgerLockfileFailImmediately,
		0,
		0xffffffff,
		0xffffffff,
		uintptr(unsafe.Pointer(&overlapped)),
	)
	if r1 != 0 {
		return true, nil
	}
	if errors.Is(err, sqliteLedgerErrorLockViolation) {
		return false, nil
	}
	if err != syscall.Errno(0) {
		return false, err
	}
	return false, fmt.Errorf("LockFileEx failed")
}
