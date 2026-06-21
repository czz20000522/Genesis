package kernel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	PermissionModePlan    = "plan"
	PermissionModeDefault = "default"
	PermissionModeYolo    = "yolo"

	maxShellOutputBytes = 64 * 1024
)

func (k *Kernel) ExecShell(ctx context.Context, req ShellExecRequest) (OperationProjection, error) {
	if err := validateShellRequest(req); err != nil {
		return OperationProjection{}, err
	}
	now := k.clock()
	operation := OperationProjection{
		OperationID:    newID("op", now),
		SessionID:      strings.TrimSpace(req.SessionID),
		Tool:           "shell.exec",
		Status:         "running",
		PermissionMode: normalizedPermissionMode(req.PermissionMode),
		CWD:            strings.TrimSpace(req.CWD),
		Command:        strings.TrimSpace(req.Command),
		StartedAt:      now,
	}

	if reason := shellBlockReason(req); reason != "" {
		operation.Status = "blocked"
		operation.BlockedReason = reason
		operation.EndedAt = k.clock()
		if err := k.appendOperationEvent(operation); err != nil {
			return OperationProjection{}, err
		}
		return operation, nil
	}

	cmd := platformShellCommand(ctx, operation.Command)
	cmd.Dir = operation.CWD
	var stdout cappedBuffer
	var stderr cappedBuffer
	stdout.limit = maxShellOutputBytes
	stderr.limit = maxShellOutputBytes
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	code := 0
	operation.Stdout = stdout.String()
	operation.Stderr = stderr.String()
	operation.EndedAt = k.clock()
	if err != nil {
		operation.Status = "failed"
		code = exitCode(err)
		if operation.Stderr == "" {
			operation.Stderr = err.Error()
		}
	} else {
		operation.Status = "completed"
	}
	operation.ExitCode = &code
	if err := k.appendOperationEvent(operation); err != nil {
		return OperationProjection{}, err
	}
	return operation, nil
}

func (k *Kernel) appendOperationEvent(operation OperationProjection) error {
	eventType := "operation." + operation.Status
	return k.ledger.Append(StoredEvent{
		EventID:     newID("evt", operation.EndedAt),
		SessionID:   operation.SessionID,
		OperationID: operation.OperationID,
		Type:        eventType,
		CreatedAt:   operation.EndedAt,
		Data: EventData{
			Operation: &operation,
		},
	})
}

func validateShellRequest(req ShellExecRequest) error {
	if strings.TrimSpace(req.SessionID) == "" {
		return errors.New("session_id is required")
	}
	if strings.TrimSpace(req.CWD) == "" {
		return errors.New("cwd is required")
	}
	if strings.TrimSpace(req.Command) == "" {
		return errors.New("command is required")
	}
	mode := normalizedPermissionMode(req.PermissionMode)
	if mode != PermissionModePlan && mode != PermissionModeDefault && mode != PermissionModeYolo {
		return fmt.Errorf("permission_mode must be plan, default, or yolo")
	}
	return nil
}

func shellBlockReason(req ShellExecRequest) string {
	mode := normalizedPermissionMode(req.PermissionMode)
	switch mode {
	case PermissionModePlan:
		if commandLooksMutating(req.Command) {
			return "blocked_by_permission_mode=plan"
		}
	case PermissionModeDefault:
		if strings.TrimSpace(req.WorkspaceRoot) == "" {
			return "workspace_root_required"
		}
		if !pathWithin(req.CWD, req.WorkspaceRoot) {
			return "cwd_outside_workspace"
		}
	}
	return ""
}

func normalizedPermissionMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		return PermissionModeDefault
	}
	return mode
}

func commandLooksMutating(command string) bool {
	lower := strings.ToLower(command)
	mutatingMarkers := []string{
		">", ">>", "| out-file", "| set-content",
		"set-content", "add-content", "new-item", "remove-item", "move-item", "copy-item",
		"mkdir", "touch ", "rm ", "del ", "erase ", "rmdir", "mv ", "cp ",
	}
	for _, marker := range mutatingMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func pathWithin(path string, root string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}

func platformShellCommand(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", command)
	}
	return exec.CommandContext(ctx, "/bin/sh", "-c", command)
}

func exitCode(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

type cappedBuffer struct {
	limit int
	buf   bytes.Buffer
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	remaining := b.limit - b.buf.Len()
	if remaining > 0 {
		if len(p) > remaining {
			_, _ = b.buf.Write(p[:remaining])
		} else {
			_, _ = b.buf.Write(p)
		}
	}
	return len(p), nil
}

func (b *cappedBuffer) String() string {
	return b.buf.String()
}
