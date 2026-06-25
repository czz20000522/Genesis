//go:build windows

package connectorruntime

import "os"

func connectorProcessLiveness(pid int) (bool, bool) {
	if pid <= 0 {
		return false, true
	}
	if pid == os.Getpid() {
		return true, true
	}
	return false, false
}
