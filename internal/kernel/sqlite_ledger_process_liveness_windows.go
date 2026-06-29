//go:build windows

package kernel

import "os"

func sqliteLedgerProcessLiveness(pid int) (bool, bool) {
	if pid <= 0 {
		return false, true
	}
	if pid == os.Getpid() {
		return true, true
	}
	return false, false
}
