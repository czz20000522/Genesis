//go:build !unix && !windows

package kernel

import "os"

func regularFileHasMultipleLinks(_ string, _ os.FileInfo) bool {
	return true
}
