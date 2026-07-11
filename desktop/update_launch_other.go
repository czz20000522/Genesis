//go:build !windows

package main

import "errors"

func launchDesktopInstaller(string) error {
	return errors.New("desktop updates are only supported on Windows")
}
