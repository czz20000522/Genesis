//go:build !windows

package connectorruntime

import (
	"errors"
	"os"
	"syscall"
)

func connectorProcessLiveness(pid int) (bool, bool) {
	if pid <= 0 {
		return false, true
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, true
	}
	err = process.Signal(syscall.Signal(0))
	switch {
	case err == nil:
		return true, true
	case errors.Is(err, syscall.ESRCH):
		return false, true
	case errors.Is(err, syscall.EPERM):
		return true, true
	default:
		return false, false
	}
}
