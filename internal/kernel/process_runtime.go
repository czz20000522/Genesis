package kernel

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
)

func runShellProcess(ctx context.Context, cwd string, command string) (capturedOutput, capturedOutput, int, error) {
	execCtx, cancel := context.WithTimeout(ctx, maxShellDuration)
	defer cancel()
	cmd := platformShellCommand(execCtx, command)
	cmd.Dir = cwd
	var stdout cappedBuffer
	var stderr cappedBuffer
	stdout.limit = maxShellOutputBytes
	stderr.limit = maxShellOutputBytes
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		return stdout.Capture(), stderr.Capture(), 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return stdout.Capture(), stderr.Capture(), exitErr.ExitCode(), nil
	}
	return stdout.Capture(), stderr.Capture(), 0, err
}

func platformShellCommand(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", command)
	}
	return exec.CommandContext(ctx, "/bin/sh", "-c", command)
}
