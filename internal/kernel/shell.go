package kernel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	PermissionModePlan    = "plan"
	PermissionModeDefault = "default"
	PermissionModeYolo    = "yolo"

	maxShellOutputBytes = 64 * 1024
	maxShellDuration    = 30 * time.Second

	staleRunningOperationReason = "stale_running_operation"
)

func (k *Kernel) ExecShell(ctx context.Context, req ShellExecRequest) (OperationProjection, error) {
	return k.execShell(ctx, req, "")
}

func (k *Kernel) execShell(ctx context.Context, req ShellExecRequest, turnID string) (OperationProjection, error) {
	if err := validateShellRequest(req); err != nil {
		return OperationProjection{}, err
	}
	k.operationMu.Lock()
	defer k.operationMu.Unlock()

	now := k.clock()
	policy := k.toolPolicy
	rawCommand := strings.TrimSpace(req.Command)
	sessionID := strings.TrimSpace(req.SessionID)
	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if idempotencyKey != "" {
		operation, ok, err := k.operationByIdempotencyKey(sessionID, "shell.exec", idempotencyKey)
		if err != nil {
			return OperationProjection{}, err
		}
		if ok {
			if operation.Status == "running" {
				return k.failStaleRunningOperation(operation)
			}
			return operation, nil
		}
	}
	definition, ok := lookupKernelTool("shell.exec")
	if !ok {
		return OperationProjection{}, fmt.Errorf("%w: shell.exec is not registered", ErrToolInfrastructureFailed)
	}
	authorization := authorizeKernelTool(policy, definition)
	executionPlan := shellExecutionPlan{cwd: strings.TrimSpace(req.CWD)}
	reason := authorization.Reason
	if authorization.Allowed {
		executionPlan, reason = prepareShellExecution(policy, req)
	}
	operation := OperationProjection{
		OperationID:    newID("op", now),
		SessionID:      sessionID,
		TurnID:         strings.TrimSpace(turnID),
		Tool:           "shell.exec",
		IdempotencyKey: idempotencyKey,
		Status:         "running",
		PermissionMode: policy.PermissionMode,
		CWD:            executionPlan.cwd,
		Command:        rawCommand,
		StartedAt:      now,
	}

	if reason != "" {
		operation.Status = "blocked"
		operation.BlockedReason = reason
		operation.EndedAt = k.clock()
		operation = redactOperationEvidence(operation)
		if err := k.appendOperationEvent(operation); err != nil {
			return OperationProjection{}, err
		}
		return operation, nil
	}

	if err := k.appendOperationEvent(operation); err != nil {
		return OperationProjection{}, err
	}

	code := 0
	if executionPlan.controlled != nil {
		stdout, stderr, exitCode := executeControlledShellCommand(*executionPlan.controlled)
		code = exitCode
		applyOperationOutputCapture(&operation, stdout, stderr)
	} else {
		execCtx, cancel := context.WithTimeout(ctx, maxShellDuration)
		defer cancel()
		cmd := platformShellCommand(execCtx, rawCommand)
		cmd.Dir = operation.CWD
		var stdout cappedBuffer
		var stderr cappedBuffer
		stdout.limit = maxShellOutputBytes
		stderr.limit = maxShellOutputBytes
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		applyOperationOutputCapture(&operation, stdout.Capture(), stderr.Capture())
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				code = exitErr.ExitCode()
			} else {
				operation.EndedAt = k.clock()
				operation.Status = "tool_infrastructure_failed"
				operation.InfrastructureReason = "shell_runtime_failed"
				operation = redactOperationEvidence(operation)
				if appendErr := k.appendOperationEvent(operation); appendErr != nil {
					return OperationProjection{}, appendErr
				}
				return OperationProjection{}, fmt.Errorf("%w: shell runtime failed: %v", ErrToolInfrastructureFailed, err)
			}
		}
	}
	operation.EndedAt = k.clock()
	operation.ExitCode = &code
	if code == 0 {
		operation.Status = "completed"
	} else {
		operation.Status = "failed"
	}
	operation = redactOperationEvidence(operation)
	if err := k.appendOperationEvent(operation); err != nil {
		return OperationProjection{}, err
	}
	return operation, nil
}

func (k *Kernel) failStaleRunningOperation(operation OperationProjection) (OperationProjection, error) {
	operation.Status = "failed"
	operation.BlockedReason = staleRunningOperationReason
	operation.Stderr = staleRunningOperationReason
	operation.EndedAt = k.clock()
	operation = redactOperationEvidence(operation)
	if err := k.appendOperationEvent(operation); err != nil {
		return OperationProjection{}, err
	}
	return operation, nil
}

func (k *Kernel) appendOperationEvent(operation OperationProjection) error {
	operation = redactOperationEvidence(operation)
	eventType := "operation." + operation.Status
	createdAt := operation.EndedAt
	if createdAt.IsZero() {
		createdAt = operation.StartedAt
	}
	return k.appendEvent(StoredEvent{
		EventID:     newID("evt", createdAt),
		SessionID:   operation.SessionID,
		TurnID:      operation.TurnID,
		OperationID: operation.OperationID,
		Type:        eventType,
		CreatedAt:   createdAt,
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
	if err := validateIdempotencyKey(req.IdempotencyKey); err != nil {
		return err
	}
	return nil
}

func (k *Kernel) operationByIdempotencyKey(sessionID string, tool string, key string) (OperationProjection, bool, error) {
	events, err := k.loadEvents()
	if err != nil {
		return OperationProjection{}, false, err
	}
	var latest OperationProjection
	found := false
	for _, event := range events {
		if event.SessionID != sessionID || event.Data.Operation == nil {
			continue
		}
		operation := *event.Data.Operation
		if operation.Tool != tool || operation.IdempotencyKey != key {
			continue
		}
		latest = operation
		found = true
	}
	if !found {
		return OperationProjection{}, false, nil
	}
	return redactOperationEvidence(latest), true, nil
}

func validateIdempotencyKey(key string) error {
	if key == "" {
		return nil
	}
	if strings.TrimSpace(key) != key {
		return errors.New("idempotency_key must not contain leading or trailing whitespace")
	}
	if len(key) > 128 {
		return errors.New("idempotency_key must be 128 characters or fewer")
	}
	for _, char := range key {
		switch {
		case char >= 'a' && char <= 'z':
		case char >= 'A' && char <= 'Z':
		case char >= '0' && char <= '9':
		case char == '-' || char == '_' || char == '.' || char == ':':
		default:
			return errors.New("idempotency_key may contain only letters, numbers, '.', '_', '-', or ':'")
		}
	}
	return nil
}

type shellExecutionPlan struct {
	cwd        string
	controlled *controlledShellCommand
}

type controlledShellCommand struct {
	kind   string
	path   string
	value  string
	stdout string
}

func prepareShellExecution(policy ToolPolicy, req ShellExecRequest) (shellExecutionPlan, string) {
	plan := shellExecutionPlan{cwd: strings.TrimSpace(req.CWD)}
	switch policy.PermissionMode {
	case PermissionModeDefault:
		return prepareDefaultShellExecution(policy, req)
	}
	return plan, ""
}

func normalizedToolPolicy(policy ToolPolicy) ToolPolicy {
	mode := normalizedPermissionMode(policy.PermissionMode)
	if mode != PermissionModePlan && mode != PermissionModeDefault && mode != PermissionModeYolo {
		mode = PermissionModePlan
	}
	return ToolPolicy{
		PermissionMode: mode,
		WorkspaceRoot:  strings.TrimSpace(policy.WorkspaceRoot),
	}
}

func normalizedPermissionMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		return PermissionModePlan
	}
	return mode
}

func prepareDefaultShellExecution(policy ToolPolicy, req ShellExecRequest) (shellExecutionPlan, string) {
	plan := shellExecutionPlan{cwd: strings.TrimSpace(req.CWD)}
	if strings.TrimSpace(policy.WorkspaceRoot) == "" {
		return plan, "workspace_root_required"
	}
	if !pathWithin(req.CWD, policy.WorkspaceRoot) {
		return plan, "cwd_outside_workspace"
	}
	cwd, err := canonicalPathForContainment(req.CWD)
	if err != nil {
		return plan, "cwd_outside_workspace"
	}
	plan.cwd = cwd
	fields, err := splitCommandFields(req.Command)
	if err != nil || len(fields) == 0 {
		return plan, "unsupported_default_command"
	}
	action, reason := controlledDefaultCommand(fields, plan.cwd, policy.WorkspaceRoot)
	if reason != "" {
		return plan, reason
	}
	plan.controlled = &action
	return plan, ""
}

func hasParentTraversal(token string) bool {
	normalized := strings.ReplaceAll(token, "\\", "/")
	return normalized == ".." ||
		strings.HasPrefix(normalized, "../") ||
		strings.Contains(normalized, "/../") ||
		strings.HasSuffix(normalized, "/..") ||
		strings.Contains(normalized, "=../") ||
		strings.HasSuffix(normalized, "=..")
}

func pathWithin(path string, root string) bool {
	if pathHasLinkOrReparsePoint(path) || pathHasLinkOrReparsePoint(root) {
		return false
	}
	candidate, err := canonicalPathForContainment(path)
	if err != nil {
		return false
	}
	canonicalRoot, err := canonicalExistingPath(root)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(canonicalRoot, candidate)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}

func pathHasLinkOrReparsePoint(path string) bool {
	current, err := filepath.Abs(path)
	if err != nil {
		return true
	}
	current = filepath.Clean(current)
	for {
		info, err := os.Lstat(current)
		if err == nil {
			mode := info.Mode()
			if mode&os.ModeSymlink != 0 || (runtime.GOOS == "windows" && mode&os.ModeIrregular != 0) {
				return true
			}
		} else if !os.IsNotExist(err) {
			return true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false
		}
		current = parent
	}
}

func canonicalExistingPath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

func canonicalPathForContainment(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(absPath); err == nil {
		return filepath.Clean(resolved), nil
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(absPath))
	if err != nil {
		return "", err
	}
	return filepath.Clean(filepath.Join(parent, filepath.Base(absPath))), nil
}

func controlledDefaultCommand(fields []string, cwd string, workspaceRoot string) (controlledShellCommand, string) {
	name := strings.ToLower(fields[0])
	switch name {
	case "echo", "write-output":
		if hasUnsupportedDefaultToken(fields[1:], false) {
			return controlledShellCommand{}, "unsupported_default_command"
		}
		return controlledShellCommand{kind: "stdout", stdout: strings.Join(fields[1:], " ") + "\n"}, ""
	case "printf":
		return controlledPrintfCommand(fields, cwd, workspaceRoot)
	case "set-content":
		return controlledSetContentCommand(fields, cwd, workspaceRoot)
	case "cat", "type", "get-content":
		return controlledReadCommand(fields, cwd, workspaceRoot)
	case "pwd":
		if len(fields) != 1 {
			return controlledShellCommand{}, "unsupported_default_command"
		}
		return controlledShellCommand{kind: "stdout", stdout: cwd + "\n"}, ""
	default:
		return controlledShellCommand{}, "unsupported_default_command"
	}
}

func controlledPrintfCommand(fields []string, cwd string, workspaceRoot string) (controlledShellCommand, string) {
	redirectAt := -1
	for i, field := range fields {
		if field == ">" {
			redirectAt = i
			break
		}
	}
	if redirectAt == -1 {
		if hasUnsupportedDefaultToken(fields[1:], false) {
			return controlledShellCommand{}, "unsupported_default_command"
		}
		return controlledShellCommand{kind: "stdout", stdout: strings.Join(fields[1:], " ")}, ""
	}
	if redirectAt == 1 || redirectAt != len(fields)-2 || hasUnsupportedDefaultToken(fields[1:redirectAt], false) {
		return controlledShellCommand{}, "unsupported_default_command"
	}
	path, reason := resolveWorkspacePath(cwd, workspaceRoot, fields[len(fields)-1])
	if reason != "" {
		return controlledShellCommand{}, reason
	}
	return controlledShellCommand{kind: "write", path: path, value: strings.Join(fields[1:redirectAt], " ")}, ""
}

func controlledSetContentCommand(fields []string, cwd string, workspaceRoot string) (controlledShellCommand, string) {
	pathArg, value, noNewline, ok := parseSetContentFields(fields[1:])
	if !ok || hasUnsupportedDefaultToken([]string{value}, false) {
		return controlledShellCommand{}, "unsupported_default_command"
	}
	path, reason := resolveWorkspacePath(cwd, workspaceRoot, pathArg)
	if reason != "" {
		return controlledShellCommand{}, reason
	}
	if !noNewline {
		value += "\n"
	}
	return controlledShellCommand{kind: "write", path: path, value: value}, ""
}

func controlledReadCommand(fields []string, cwd string, workspaceRoot string) (controlledShellCommand, string) {
	pathArg, ok := parsePathOnlyFields(fields[1:])
	if !ok {
		return controlledShellCommand{}, "unsupported_default_command"
	}
	path, reason := resolveWorkspacePath(cwd, workspaceRoot, pathArg)
	if reason != "" {
		return controlledShellCommand{}, reason
	}
	return controlledShellCommand{kind: "read", path: path}, ""
}

func parseSetContentFields(fields []string) (string, string, bool, bool) {
	var pathArg string
	var value string
	noNewline := false
	for i := 0; i < len(fields); i++ {
		field := fields[i]
		lower := strings.ToLower(field)
		switch {
		case lower == "-literalpath" || lower == "-path":
			i++
			if i >= len(fields) {
				return "", "", false, false
			}
			pathArg = fields[i]
		case strings.HasPrefix(lower, "-literalpath=") || strings.HasPrefix(lower, "-path="):
			_, pathArg, _ = strings.Cut(field, "=")
		case lower == "-value":
			i++
			if i >= len(fields) {
				return "", "", false, false
			}
			value = fields[i]
		case strings.HasPrefix(lower, "-value="):
			_, value, _ = strings.Cut(field, "=")
		case lower == "-nonewline":
			noNewline = true
		default:
			return "", "", false, false
		}
	}
	return pathArg, value, noNewline, pathArg != "" && value != ""
}

func parsePathOnlyFields(fields []string) (string, bool) {
	if len(fields) == 1 {
		lower := strings.ToLower(fields[0])
		if strings.HasPrefix(lower, "-literalpath=") || strings.HasPrefix(lower, "-path=") {
			_, value, ok := strings.Cut(fields[0], "=")
			return value, ok
		}
		return fields[0], true
	}
	if len(fields) == 2 {
		lower := strings.ToLower(fields[0])
		if lower == "-literalpath" || lower == "-path" {
			return fields[1], true
		}
	}
	return "", false
}

func hasUnsupportedDefaultToken(fields []string, allowRedirect bool) bool {
	for _, field := range fields {
		if allowRedirect && field == ">" {
			continue
		}
		if strings.ContainsAny(field, "\r\n;|&`$<>") {
			return true
		}
	}
	return false
}

func resolveWorkspacePath(cwd string, workspaceRoot string, pathArg string) (string, string) {
	pathArg = strings.TrimSpace(pathArg)
	if pathArg == "" {
		return "", "unsupported_default_command"
	}
	if hasParentTraversal(pathArg) {
		return "", "command_path_outside_workspace"
	}
	target := pathArg
	if !filepath.IsAbs(target) {
		target = filepath.Join(cwd, target)
	}
	if !pathWithin(target, workspaceRoot) {
		return "", "command_path_outside_workspace"
	}
	resolved, err := canonicalPathForContainment(target)
	if err != nil {
		return "", "command_path_outside_workspace"
	}
	return resolved, ""
}

func executeControlledShellCommand(command controlledShellCommand) (capturedOutput, capturedOutput, int) {
	switch command.kind {
	case "stdout":
		return captureBytes([]byte(command.stdout), maxShellOutputBytes), capturedOutput{}, 0
	case "write":
		if err := os.WriteFile(command.path, []byte(command.value), 0o644); err != nil {
			return capturedOutput{}, captureBytes([]byte("write failed"), maxShellOutputBytes), -1
		}
		return capturedOutput{}, capturedOutput{}, 0
	case "read":
		data, err := os.ReadFile(command.path)
		if err != nil {
			return capturedOutput{}, captureBytes([]byte("read failed"), maxShellOutputBytes), -1
		}
		return captureBytes(data, maxShellOutputBytes), capturedOutput{}, 0
	default:
		return capturedOutput{}, captureBytes([]byte(fmt.Sprintf("unsupported controlled shell command kind %q", command.kind)), maxShellOutputBytes), -1
	}
}

func splitCommandFields(command string) ([]string, error) {
	var fields []string
	var current strings.Builder
	var quote rune
	for _, char := range command {
		switch {
		case quote != 0:
			if char == quote {
				quote = 0
				continue
			}
			current.WriteRune(char)
		case char == '\'' || char == '"':
			quote = char
		case char == ' ' || char == '\t':
			if current.Len() > 0 {
				fields = append(fields, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(char)
		}
	}
	if quote != 0 {
		return nil, errors.New("unterminated quote")
	}
	if current.Len() > 0 {
		fields = append(fields, current.String())
	}
	return fields, nil
}

func platformShellCommand(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", command)
	}
	return exec.CommandContext(ctx, "/bin/sh", "-c", command)
}

type cappedBuffer struct {
	limit     int
	full      bytes.Buffer
	head      []byte
	tail      []byte
	total     int
	truncated bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	written := len(p)
	if written == 0 {
		return 0, nil
	}
	if b.limit <= 0 {
		b.total += written
		b.truncated = true
		return written, nil
	}
	b.total += written
	if !b.truncated && b.full.Len()+written <= b.limit {
		_, _ = b.full.Write(p)
		return written, nil
	}
	if !b.truncated {
		combined := make([]byte, 0, b.full.Len()+written)
		combined = append(combined, b.full.Bytes()...)
		combined = append(combined, p...)
		b.full.Reset()
		b.initializeHeadTail(combined)
		return written, nil
	}
	b.appendTail(p)
	return written, nil
}

func (b *cappedBuffer) String() string {
	return b.Capture().Text
}

func (b *cappedBuffer) Capture() capturedOutput {
	if !b.truncated {
		return capturedOutput{
			Text:          b.full.String(),
			OriginalBytes: b.full.Len(),
		}
	}
	text := string(append(append([]byte(nil), b.head...), b.tail...))
	return capturedOutput{
		Text:          text,
		Truncated:     true,
		OriginalBytes: b.total,
		OmittedBytes:  maxInt(0, b.total-len(b.head)-len(b.tail)),
	}
}

func (b *cappedBuffer) initializeHeadTail(data []byte) {
	b.truncated = true
	headLimit, tailLimit := headTailLimits(b.limit)
	if headLimit > len(data) {
		headLimit = len(data)
	}
	b.head = append([]byte(nil), data[:headLimit]...)
	if tailLimit > len(data)-headLimit {
		tailLimit = len(data) - headLimit
	}
	if tailLimit > 0 {
		b.tail = append([]byte(nil), data[len(data)-tailLimit:]...)
	}
}

func (b *cappedBuffer) appendTail(data []byte) {
	_, tailLimit := headTailLimits(b.limit)
	if tailLimit <= 0 {
		return
	}
	b.tail = append(b.tail, data...)
	if len(b.tail) > tailLimit {
		b.tail = append([]byte(nil), b.tail[len(b.tail)-tailLimit:]...)
	}
}

type capturedOutput struct {
	Text          string
	Truncated     bool
	OriginalBytes int
	OmittedBytes  int
}

func captureBytes(data []byte, limit int) capturedOutput {
	var buffer cappedBuffer
	buffer.limit = limit
	_, _ = buffer.Write(data)
	return buffer.Capture()
}

func applyOperationOutputCapture(operation *OperationProjection, stdout capturedOutput, stderr capturedOutput) {
	operation.Stdout = stdout.Text
	operation.Stderr = stderr.Text
	if stdout.Truncated {
		operation.StdoutTruncated = true
		operation.StdoutOriginalBytes = stdout.OriginalBytes
		operation.StdoutOmittedBytes = stdout.OmittedBytes
		operation.OutputTruncation = "head_tail"
	}
	if stderr.Truncated {
		operation.StderrTruncated = true
		operation.StderrOriginalBytes = stderr.OriginalBytes
		operation.StderrOmittedBytes = stderr.OmittedBytes
		operation.OutputTruncation = "head_tail"
	}
}

func headTailLimits(limit int) (int, int) {
	if limit <= 0 {
		return 0, 0
	}
	headLimit := limit / 2
	if headLimit == 0 {
		headLimit = 1
	}
	return headLimit, limit - headLimit
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
