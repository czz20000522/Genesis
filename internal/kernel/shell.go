package kernel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	PermissionModePlan    = "plan"
	PermissionModeDefault = "default"
	PermissionModeYolo    = "yolo"

	maxShellOutputBytes          = 64 * 1024
	defaultShellTimeoutSec       = 30
	maxForegroundShellTimeoutSec = 180
	defaultShellDuration         = time.Duration(defaultShellTimeoutSec) * time.Second

	outputOmissionMarkerFormat = "\n[... %d bytes omitted ...]\n"

	staleRunningOperationReason = "stale_running_operation"
	foregroundTimeoutReason     = "foreground_timeout"
	foregroundTimeoutExitCode   = 124
)

type shellInvokeResult struct {
	Operation       *OperationProjection
	Job             *JobProjection
	CreatedJob      bool
	PendingJobStart *JobProjection
}

func (k *Kernel) ExecShell(ctx context.Context, req ShellExecRequest) (OperationProjection, error) {
	return k.toolGateway().ExecShell(ctx, req, "")
}

func (g ToolGateway) InvokeShell(ctx context.Context, req ShellExecRequest, turnID string, toolCallEventID string, startManagedJob bool) (shellInvokeResult, error) {
	if err := validateShellRequest(req); err != nil {
		return shellInvokeResult{}, err
	}
	req.SessionID = strings.TrimSpace(req.SessionID)
	req.IdempotencyKey = strings.TrimSpace(req.IdempotencyKey)
	turnID = strings.TrimSpace(turnID)
	toolCallEventID = strings.TrimSpace(toolCallEventID)

	if req.IdempotencyKey != "" {
		existing, ok, err := g.lookupShellEffectByIdempotencyKey(req.SessionID, req.IdempotencyKey)
		if err != nil {
			return shellInvokeResult{}, err
		}
		if ok {
			return existing, nil
		}
	}

	timeoutSec := normalizedShellTimeoutSec(req.TimeoutSec)
	if timeoutSec <= maxForegroundShellTimeoutSec {
		operation, err := g.ExecShell(ctx, req, turnID)
		if err != nil {
			return shellInvokeResult{}, err
		}
		return shellInvokeResult{Operation: &operation}, nil
	}

	definition, ok := g.registry.Resolve("shell_exec")
	if !ok {
		return shellInvokeResult{}, fmt.Errorf("%w: shell_exec is not registered", ErrToolInfrastructureFailed)
	}
	authorization := authorizeKernelTool(g.kernel.toolPolicy, definition.Spec)
	if !authorization.Allowed {
		operation, err := g.recordBlockedShellOperation(req, turnID, authorization.Reason)
		if err != nil {
			return shellInvokeResult{}, err
		}
		return shellInvokeResult{Operation: &operation}, nil
	}
	resolvedPolicy := resolveToolPolicy(g.kernel.toolPolicy)
	executionPlan, reason := prepareShellExecution(resolvedPolicy, req)
	if reason != "" {
		operation, err := g.recordBlockedShellOperationWithCWD(req, turnID, reason, executionPlan.cwd)
		if err != nil {
			return shellInvokeResult{}, err
		}
		return shellInvokeResult{Operation: &operation}, nil
	}
	if resolvedPolicy.SandboxProfile != SandboxProfileHost {
		operation, err := g.recordBlockedShellOperationWithCWD(req, turnID, "managed_job_requires_host_sandbox", executionPlan.cwd)
		if err != nil {
			return shellInvokeResult{}, err
		}
		return shellInvokeResult{Operation: &operation}, nil
	}

	job, created, err := g.kernel.prepareManagedShellJob(req, turnID, toolCallEventID)
	if err != nil {
		return shellInvokeResult{}, err
	}
	result := shellInvokeResult{Job: &job, CreatedJob: created}
	if created {
		result.PendingJobStart = &job
	}
	if created && startManagedJob {
		if err := g.kernel.startManagedJobExecutor(job); err != nil {
			return shellInvokeResult{}, err
		}
	}
	return result, nil
}

func (g ToolGateway) ExecShell(ctx context.Context, req ShellExecRequest, turnID string) (OperationProjection, error) {
	if err := validateShellRequest(req); err != nil {
		return OperationProjection{}, err
	}
	timeoutSec := normalizedShellTimeoutSec(req.TimeoutSec)
	k := g.kernel
	k.operationMu.Lock()
	defer k.operationMu.Unlock()

	now := k.clock()
	policy := k.toolPolicy
	resolvedPolicy := resolveToolPolicy(policy)
	rawCommand := strings.TrimSpace(req.Command)
	sessionID := strings.TrimSpace(req.SessionID)
	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if idempotencyKey != "" {
		operation, ok, err := k.operationByIdempotencyKey(sessionID, "shell_exec", idempotencyKey)
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
	definition, ok := g.registry.Resolve("shell_exec")
	if !ok {
		return OperationProjection{}, fmt.Errorf("%w: shell_exec is not registered", ErrToolInfrastructureFailed)
	}
	authorization := authorizeKernelTool(policy, definition.Spec)
	executionPlan := shellExecutionPlan{cwd: strings.TrimSpace(req.CWD)}
	reason := authorization.Reason
	if authorization.Allowed {
		executionPlan, reason = prepareShellExecution(resolvedPolicy, req)
	}
	operation := OperationProjection{
		OperationID:     newID("op", now),
		SessionID:       sessionID,
		TurnID:          strings.TrimSpace(turnID),
		Tool:            "shell_exec",
		IdempotencyKey:  idempotencyKey,
		Status:          "running",
		PermissionMode:  resolvedPolicy.PermissionMode,
		AuthorityPolicy: resolvedPolicy.AuthorityPolicy,
		SandboxProfile:  resolvedPolicy.SandboxProfile,
		ApprovalPolicy:  resolvedPolicy.ApprovalPolicy,
		CWD:             executionPlan.cwd,
		Command:         rawCommand,
		TimeoutSec:      timeoutSec,
		StartedAt:       now,
	}

	if reason != "" {
		operation.Status = "blocked"
		operation.BlockedReason = reason
		operation.EndedAt = k.clock()
		operation.ElapsedMs = operationElapsedMs(operation.StartedAt, operation.EndedAt)
		if err := k.appendOperationEvent(operation); err != nil {
			return OperationProjection{}, err
		}
		return redactOperationEvidence(operation), nil
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
		stdout, stderr, exitCode, timedOut, err := runShellProcess(ctx, operation.CWD, rawCommand, time.Duration(timeoutSec)*time.Second)
		code = exitCode
		if timedOut {
			code = foregroundTimeoutExitCode
			operation.TimedOut = true
			operation.TimeoutReason = foregroundTimeoutReason
		}
		applyOperationOutputCapture(&operation, stdout, stderr)
		if err != nil && !timedOut {
			operation.EndedAt = k.clock()
			operation.ElapsedMs = operationElapsedMs(operation.StartedAt, operation.EndedAt)
			operation.Status = "tool_infrastructure_failed"
			operation.InfrastructureReason = "shell_runtime_failed"
			if appendErr := k.appendOperationEvent(operation); appendErr != nil {
				return OperationProjection{}, appendErr
			}
			return OperationProjection{}, fmt.Errorf("%w: shell runtime failed: %v", ErrToolInfrastructureFailed, err)
		}
	}
	operation.EndedAt = k.clock()
	operation.ElapsedMs = operationElapsedMs(operation.StartedAt, operation.EndedAt)
	operation.ExitCode = &code
	if code == 0 {
		operation.Status = "completed"
	} else {
		operation.Status = "failed"
	}
	if err := k.appendOperationEvent(operation); err != nil {
		return OperationProjection{}, err
	}
	return redactOperationEvidence(operation), nil
}

func (g ToolGateway) lookupShellEffectByIdempotencyKey(sessionID string, key string) (shellInvokeResult, bool, error) {
	operation, ok, err := g.kernel.operationByIdempotencyKey(sessionID, "shell_exec", key)
	if err != nil {
		return shellInvokeResult{}, false, err
	}
	if ok {
		if operation.Status == "running" {
			failed, err := g.kernel.failStaleRunningOperation(operation)
			if err != nil {
				return shellInvokeResult{}, false, err
			}
			return shellInvokeResult{Operation: &failed}, true, nil
		}
		return shellInvokeResult{Operation: &operation}, true, nil
	}
	job, ok, err := g.kernel.lookupSessionJobByIdempotencyKey(sessionID, "shell_exec", key)
	if err != nil {
		return shellInvokeResult{}, false, err
	}
	if ok {
		return shellInvokeResult{Job: &job}, true, nil
	}
	return shellInvokeResult{}, false, nil
}

func (g ToolGateway) recordBlockedShellOperation(req ShellExecRequest, turnID string, reason string) (OperationProjection, error) {
	return g.recordBlockedShellOperationWithCWD(req, turnID, reason, strings.TrimSpace(req.CWD))
}

func (g ToolGateway) recordBlockedShellOperationWithCWD(req ShellExecRequest, turnID string, reason string, cwd string) (OperationProjection, error) {
	k := g.kernel
	now := k.clock()
	resolvedPolicy := resolveToolPolicy(k.toolPolicy)
	operation := OperationProjection{
		OperationID:     newID("op", now),
		SessionID:       strings.TrimSpace(req.SessionID),
		TurnID:          strings.TrimSpace(turnID),
		Tool:            "shell_exec",
		IdempotencyKey:  strings.TrimSpace(req.IdempotencyKey),
		Status:          "blocked",
		PermissionMode:  resolvedPolicy.PermissionMode,
		AuthorityPolicy: resolvedPolicy.AuthorityPolicy,
		SandboxProfile:  resolvedPolicy.SandboxProfile,
		ApprovalPolicy:  resolvedPolicy.ApprovalPolicy,
		CWD:             strings.TrimSpace(cwd),
		Command:         strings.TrimSpace(req.Command),
		TimeoutSec:      normalizedShellTimeoutSec(req.TimeoutSec),
		BlockedReason:   strings.TrimSpace(reason),
		StartedAt:       now,
		EndedAt:         now,
		ElapsedMs:       1,
	}
	if err := k.appendOperationEvent(operation); err != nil {
		return OperationProjection{}, err
	}
	return redactOperationEvidence(operation), nil
}

func (k *Kernel) failStaleRunningOperation(operation OperationProjection) (OperationProjection, error) {
	operation.Status = "failed"
	operation.BlockedReason = staleRunningOperationReason
	operation.Stderr = staleRunningOperationReason
	operation.EndedAt = k.clock()
	operation.ElapsedMs = operationElapsedMs(operation.StartedAt, operation.EndedAt)
	if err := k.appendOperationEvent(operation); err != nil {
		return OperationProjection{}, err
	}
	return redactOperationEvidence(operation), nil
}

func operationElapsedMs(startedAt time.Time, endedAt time.Time) int64 {
	if startedAt.IsZero() || endedAt.IsZero() || endedAt.Before(startedAt) {
		return 0
	}
	elapsed := endedAt.Sub(startedAt).Milliseconds()
	if elapsed == 0 {
		return 1
	}
	return elapsed
}

func (k *Kernel) appendOperationEvent(operation OperationProjection) error {
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
	if req.TimeoutSec < 0 {
		return errors.New("timeout_sec must be greater than zero")
	}
	if err := validateIdempotencyKey(req.IdempotencyKey); err != nil {
		return err
	}
	return nil
}

func normalizedShellTimeoutSec(timeoutSec int) int {
	if timeoutSec == 0 {
		return defaultShellTimeoutSec
	}
	return timeoutSec
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
	text, omittedBytes := boundedHeadTailText(b.head, b.tail, b.total, b.limit)
	return capturedOutput{
		Text:          text,
		Truncated:     true,
		OriginalBytes: b.total,
		OmittedBytes:  omittedBytes,
	}
}

func boundedHeadTailText(head []byte, tail []byte, total int, limit int) (string, int) {
	if limit <= 0 {
		return "", maxInt(0, total)
	}
	headLen := len(head)
	tailLen := len(tail)
	for i := 0; i < 8; i++ {
		omittedBytes := maxInt(0, total-headLen-tailLen)
		marker := []byte(fmt.Sprintf(outputOmissionMarkerFormat, omittedBytes))
		budget := limit - len(marker)
		if budget <= 0 {
			return string(marker[:minInt(len(marker), limit)]), maxInt(0, total)
		}
		nextHeadLen, nextTailLen := headTailLimits(budget)
		nextHeadLen = minInt(nextHeadLen, len(head))
		nextTailLen = minInt(nextTailLen, len(tail))
		if nextHeadLen == headLen && nextTailLen == tailLen {
			return joinHeadTailWithMarker(head, tail, headLen, tailLen, marker), omittedBytes
		}
		headLen = nextHeadLen
		tailLen = nextTailLen
	}
	omittedBytes := maxInt(0, total-headLen-tailLen)
	marker := []byte(fmt.Sprintf(outputOmissionMarkerFormat, omittedBytes))
	if len(marker)+headLen+tailLen > limit {
		budget := maxInt(0, limit-len(marker))
		headLen, tailLen = headTailLimits(budget)
		headLen = minInt(headLen, len(head))
		tailLen = minInt(tailLen, len(tail))
		omittedBytes = maxInt(0, total-headLen-tailLen)
		marker = []byte(fmt.Sprintf(outputOmissionMarkerFormat, omittedBytes))
	}
	return joinHeadTailWithMarker(head, tail, headLen, tailLen, marker), omittedBytes
}

func joinHeadTailWithMarker(head []byte, tail []byte, headLen int, tailLen int, marker []byte) string {
	text := make([]byte, 0, headLen+len(marker)+tailLen)
	text = append(text, head[:headLen]...)
	text = append(text, marker...)
	if tailLen > 0 {
		text = append(text, tail[len(tail)-tailLen:]...)
	}
	return string(text)
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

func applyJobOutputCapture(job *JobProjection, stdout capturedOutput, stderr capturedOutput) {
	job.Stdout = stdout.Text
	job.Stderr = stderr.Text
	if stdout.Truncated {
		job.StdoutTruncated = true
	}
	if stderr.Truncated {
		job.StderrTruncated = true
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

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
