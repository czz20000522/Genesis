//go:build unix

package kernel

import (
	"os"
	"syscall"
)

func regularFileHasMultipleLinks(_ string, info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return true
	}
	return stat.Nlink > 1
}
