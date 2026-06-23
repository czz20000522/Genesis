package kernel

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"time"
)

func runShellProcess(ctx context.Context, cwd string, command string, timeout time.Duration) (capturedOutput, capturedOutput, int, bool, bool, error) {
	if timeout <= 0 {
		timeout = defaultShellDuration
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	stdout, stderr, exitCode, err := runShellProcessContext(execCtx, cwd, command)
	timedOut := errors.Is(execCtx.Err(), context.DeadlineExceeded)
	interrupted := errors.Is(execCtx.Err(), context.Canceled)
	return stdout, stderr, exitCode, timedOut, interrupted, err
}

func runShellProcessContext(ctx context.Context, cwd string, command string) (capturedOutput, capturedOutput, int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := platformShellCommand(ctx, command)
	cmd.Dir = cwd
	configureShellProcessTermination(cmd)
	cmd.WaitDelay = 2 * time.Second
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
