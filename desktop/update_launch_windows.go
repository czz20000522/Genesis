//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

func launchDesktopInstaller(installer string) error {
	command := exec.Command("cmd.exe", "/C", "timeout /T 2 /NOBREAK > NUL & start \"\" \""+installer+"\"")
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return command.Start()
}
