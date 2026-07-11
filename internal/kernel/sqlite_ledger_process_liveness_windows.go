//go:build windows

package kernel

import (
	"errors"
	"os"
	"syscall"
)

const (
	windowsErrorInvalidParameter = syscall.Errno(87)
)

func sqliteLedgerProcessLiveness(pid int) (bool, bool) {
	if pid <= 0 {
		return false, true
	}
	if pid == os.Getpid() {
		return true, true
	}
	handle, err := syscall.OpenProcess(syscall.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		if errors.Is(err, windowsErrorInvalidParameter) {
			return false, true
		}
		return false, false
	}
	defer syscall.CloseHandle(handle)
	return sqliteLedgerWindowsProcessHandleLiveness(handle)
}

func sqliteLedgerWindowsProcessHandleLiveness(handle syscall.Handle) (bool, bool) {
	result, err := syscall.WaitForSingleObject(handle, 0)
	if err != nil {
		return false, false
	}
	switch result {
	case syscall.WAIT_TIMEOUT:
		return true, true
	case syscall.WAIT_OBJECT_0:
		return false, true
	default:
		return false, false
	}
}
