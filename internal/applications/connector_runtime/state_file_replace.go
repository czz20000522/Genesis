package connectorruntime

import (
	"errors"
	"os"
	"runtime"
)

func replaceConnectorStateFile(tmpPath string, targetPath string) error {
	if err := os.Rename(tmpPath, targetPath); err == nil {
		return nil
	} else if runtime.GOOS != "windows" {
		return err
	}
	if err := os.Remove(targetPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(tmpPath, targetPath)
}
