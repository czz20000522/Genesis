package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

func (k *Kernel) prepareManagedShellJob(req ShellExecRequest, turnID string, toolCallEventID string) (JobProjection, bool, error) {
	if err := validateShellRequest(req); err != nil {
		return JobProjection{}, false, err
	}
	timeoutSec := k.normalizedShellTimeoutSec(req.TimeoutSec)
	if !k.shellTimeoutExceedsForeground(timeoutSec) {
		return JobProjection{}, false, fmt.Errorf("managed shell jobs require timeout_sec greater than %d", k.shellTimeoutPolicy.ManagedJobThresholdSec)
	}
	sessionID := strings.TrimSpace(req.SessionID)
	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)

	k.jobMu.Lock()
	defer k.jobMu.Unlock()
	if idempotencyKey != "" {
		job, ok, err := k.lookupSessionJobByIdempotencyKey(sessionID, "shell_exec", idempotencyKey)
		if err != nil {
			return JobProjection{}, false, err
		}
		if ok {
			return job, false, nil
		}
	}

	startedAt := k.clock()
	jobID := newID("job", startedAt)
	receipt := fmt.Sprintf("Command was accepted as managed job %s. No synchronous command output is available in this tool result; terminal job evidence is recorded in the session events.", jobID)
	started := JobProjection{
		JobID:           jobID,
		SessionID:       sessionID,
		TurnID:          strings.TrimSpace(turnID),
		Tool:            "shell_exec",
		IdempotencyKey:  idempotencyKey,
		Status:          "running",
		CWD:             strings.TrimSpace(req.CWD),
		Command:         strings.TrimSpace(req.Command),
		TimeoutSec:      timeoutSec,
		Receipt:         receipt,
		StartedAt:       startedAt,
		ToolCallEventID: strings.TrimSpace(toolCallEventID),
	}
	if err := k.appendJobEvent("job.started", started); err != nil {
		return JobProjection{}, false, err
	}
	return started, true, nil
}

func (k *Kernel) prepareForegroundAttachedShellJob(operation OperationProjection) (JobProjection, bool, error) {
	sessionID := strings.TrimSpace(operation.SessionID)
	idempotencyKey := strings.TrimSpace(operation.IdempotencyKey)

	k.jobMu.Lock()
	defer k.jobMu.Unlock()
	if idempotencyKey != "" {
		job, ok, err := k.lookupSessionJobByIdempotencyKey(sessionID, "shell_exec", idempotencyKey)
		if err != nil {
			return JobProjection{}, false, err
		}
		if ok {
			return job, false, nil
		}
	}

	startedAt := k.clock()
	jobID := newID("job", startedAt)
	receipt := fmt.Sprintf("Foreground command continued as managed job %s after interruption. Terminal job evidence is recorded in the session events.", jobID)
	return JobProjection{
		JobID:           jobID,
		SessionID:       sessionID,
		TurnID:          strings.TrimSpace(operation.TurnID),
		Tool:            "shell_exec",
		IdempotencyKey:  idempotencyKey,
		Status:          "running",
		CWD:             strings.TrimSpace(operation.CWD),
		Command:         strings.TrimSpace(operation.Command),
		TimeoutSec:      operation.TimeoutSec,
		Receipt:         receipt,
		StartedAt:       startedAt,
		ToolCallEventID: idempotencyKey,
	}, true, nil
}

func (k *Kernel) appendJobStartedIfAbsent(job JobProjection) (JobProjection, bool, error) {
	k.jobMu.Lock()
	defer k.jobMu.Unlock()
	if strings.TrimSpace(job.IdempotencyKey) != "" {
		existing, ok, err := k.lookupSessionJobByIdempotencyKey(job.SessionID, job.Tool, job.IdempotencyKey)
		if err != nil {
			return JobProjection{}, false, err
		}
		if ok {
			return existing, false, nil
		}
	}
	if strings.TrimSpace(job.JobID) != "" {
		existing, ok, err := k.lookupSessionJob(job.SessionID, job.JobID)
		if err != nil {
			return JobProjection{}, false, err
		}
		if ok {
			return existing, false, nil
		}
	}
	if err := k.appendJobEvent("job.started", job); err != nil {
		return JobProjection{}, false, err
	}
	return job, true, nil
}

func (k *Kernel) attachInterruptedForegroundShell(ctx context.Context, operation OperationProjection, reason string) (*JobProjection, string, error) {
	if managedJobExecutorCapabilities(k.jobExecutor).ForegroundAttach == false {
		return nil, foregroundAttachUnavailableKilledReason, nil
	}
	attachExecutor, ok := k.jobExecutor.(managedJobForegroundAttachExecutor)
	if !ok {
		return nil, foregroundAttachUnavailableKilledReason, nil
	}
	job, created, err := k.prepareForegroundAttachedShellJob(operation)
	if err != nil {
		return nil, foregroundAttachUnavailableKilledReason, err
	}
	if !created {
		return &job, foregroundAttachedManagedJobReason, nil
	}

	var startOnce sync.Once
	var started JobProjection
	var startErr error
	ensureStarted := func() error {
		startOnce.Do(func() {
			started, _, startErr = k.appendJobStartedIfAbsent(job)
		})
		return startErr
	}
	request := ManagedJobForegroundAttachRequest{
		SessionID:     strings.TrimSpace(operation.SessionID),
		TurnID:        strings.TrimSpace(operation.TurnID),
		OperationID:   strings.TrimSpace(operation.OperationID),
		Job:           cloneJobProjection(job),
		Reason:        strings.TrimSpace(reason),
		InterruptedAt: k.clock(),
		Observe: func(progress JobProjection) {
			if err := ensureStarted(); err != nil {
				return
			}
			progress = mergeJobOutputSnapshot(started, progress)
			_ = k.appendJobOutputEvent(progress)
		},
		Complete: func(done JobProjection) {
			if err := ensureStarted(); err != nil {
				return
			}
			done = bindAttachedTerminalJob(started, done)
			if done.Status == "" || done.Status == "running" {
				done.Status = "completed"
			}
			_ = k.appendTerminalJobEvent(done)
		},
	}
	attachCtx := context.Background()
	if ctx != nil {
		attachCtx = context.WithoutCancel(ctx)
	}
	result, err := attachExecutor.AttachForeground(attachCtx, request)
	if err != nil || !result.Attached {
		return nil, foregroundAttachUnavailableKilledReason, nil
	}
	if err := ensureStarted(); err != nil {
		return nil, foregroundAttachUnavailableKilledReason, err
	}
	return &started, foregroundAttachedManagedJobReason, nil
}

func bindAttachedTerminalJob(started JobProjection, done JobProjection) JobProjection {
	status := strings.TrimSpace(done.Status)
	exitCode := done.ExitCode
	stdout := done.Stdout
	stderr := done.Stderr
	stdoutTruncated := done.StdoutTruncated
	stderrTruncated := done.StderrTruncated
	failureReason := done.FailureReason
	cancelReason := done.CancelReason
	completedAt := done.CompletedAt

	done = mergeJobOutputSnapshot(started, JobProjection{
		Stdout:          stdout,
		Stderr:          stderr,
		StdoutTruncated: stdoutTruncated,
		StderrTruncated: stderrTruncated,
	})
	done.Status = status
	done.ExitCode = exitCode
	done.FailureReason = failureReason
	done.CancelReason = cancelReason
	done.CompletedAt = completedAt
	return done
}

func (k *Kernel) startManagedJobExecutor(job JobProjection) error {
	if k.jobExecutor == nil {
		failed := job
		failed.Status = "failed"
		failed.FailureReason = "managed job executor is unavailable"
		return k.appendTerminalJobEvent(failed)
	}
	if err := k.jobExecutor.Start(context.Background(), ManagedJobStartRequest{
		Job: job,
		Observe: func(progress JobProjection) {
			progress = mergeJobOutputSnapshot(job, progress)
			if err := k.appendJobOutputEvent(progress); err != nil {
				return
			}
		},
		Complete: func(done JobProjection) {
			if err := k.appendTerminalJobEvent(done); err != nil {
				return
			}
		},
	}); err != nil {
		failed := job
		failed.Status = "failed"
		failed.FailureReason = "managed job executor start failed: " + err.Error()
		return k.appendTerminalJobEvent(failed)
	}
	return nil
}

func (k *Kernel) appendJobOutputEvent(job JobProjection) error {
	k.jobMu.Lock()
	defer k.jobMu.Unlock()

	latest, ok, err := k.lookupSessionJob(job.SessionID, job.JobID)
	if err != nil {
		return err
	}
	if !ok || isTerminalJobStatus(latest.Status) {
		return nil
	}
	job = mergeJobOutputSnapshot(latest, job)
	normalizeJobOutputSnapshot(&job)
	return k.appendJobEvent("job.output", job)
}

func (k *Kernel) appendTerminalJobEvent(job JobProjection) error {
	k.jobMu.Lock()
	defer k.jobMu.Unlock()

	latest, ok, err := k.lookupSessionJob(job.SessionID, job.JobID)
	if err != nil {
		return err
	}
	if ok && isTerminalJobStatus(latest.Status) {
		return nil
	}
	if strings.TrimSpace(job.Status) == "" {
		job.Status = "failed"
	}
	if job.CompletedAt.IsZero() {
		job.CompletedAt = k.clock()
	}
	switch strings.TrimSpace(job.Status) {
	case "completed":
		return k.appendJobEvent("job.completed", job)
	case "cancelled":
		return k.appendJobEvent("job.cancelled", job)
	default:
		job.Status = "failed"
		return k.appendJobEvent("job.failed", job)
	}
}

func mergeJobOutputSnapshot(latest JobProjection, snapshot JobProjection) JobProjection {
	stdout := snapshot.Stdout
	stderr := snapshot.Stderr
	stdoutTruncated := snapshot.StdoutTruncated
	stderrTruncated := snapshot.StderrTruncated
	snapshot = JobProjection{}
	snapshot.SessionID = latest.SessionID
	snapshot.TurnID = latest.TurnID
	snapshot.JobID = latest.JobID
	snapshot.Tool = latest.Tool
	snapshot.IdempotencyKey = latest.IdempotencyKey
	if strings.TrimSpace(latest.Status) != "" {
		snapshot.Status = latest.Status
	} else {
		snapshot.Status = "running"
	}
	snapshot.CWD = latest.CWD
	snapshot.Command = latest.Command
	snapshot.TimeoutSec = latest.TimeoutSec
	snapshot.Receipt = latest.Receipt
	snapshot.StartedAt = latest.StartedAt
	snapshot.ToolCallEventID = latest.ToolCallEventID
	snapshot.Stdout = stdout
	snapshot.Stderr = stderr
	snapshot.StdoutTruncated = stdoutTruncated
	snapshot.StderrTruncated = stderrTruncated
	return snapshot
}

func normalizeJobOutputSnapshot(job *JobProjection) {
	if job == nil {
		return
	}
	stdoutAlreadyTruncated := job.StdoutTruncated
	stderrAlreadyTruncated := job.StderrTruncated
	applyJobOutputCapture(job, captureBytes([]byte(job.Stdout), maxShellOutputBytes), captureBytes([]byte(job.Stderr), maxShellOutputBytes))
	job.StdoutTruncated = job.StdoutTruncated || stdoutAlreadyTruncated
	job.StderrTruncated = job.StderrTruncated || stderrAlreadyTruncated
}

func (k *Kernel) appendJobEvent(eventType string, job JobProjection) error {
	createdAt := time.Time{}
	if eventType == "job.started" {
		createdAt = job.StartedAt
	} else if !job.CompletedAt.IsZero() {
		createdAt = job.CompletedAt
	}
	if createdAt.IsZero() {
		createdAt = k.clock()
	}
	eventID := newID("evt", createdAt)
	if eventType == "job.started" {
		eventID = job.JobID
	}
	return k.appendEvent(StoredEvent{
		EventID:   eventID,
		SessionID: job.SessionID,
		TurnID:    job.TurnID,
		JobID:     job.JobID,
		Type:      eventType,
		CreatedAt: createdAt,
		Data: EventData{
			Job: &job,
		},
	})
}

func (k *Kernel) jobStatusModelToolResult(sessionID string, toolCallEventID string, providerCallID string, toolName string, jobID string) (ModelToolResult, error) {
	job, ok, err := k.lookupSessionJob(sessionID, jobID)
	if err != nil {
		return ModelToolResult{}, err
	}
	if !ok {
		return invalidModelToolResult(toolCallEventID, providerCallID, toolName, "job_not_found", fmt.Sprintf("job %q was not found", jobID))
	}
	content, err := json.Marshal(modelJobControlResult(job, false, jobStatusVisibleOutput(job)))
	if err != nil {
		return ModelToolResult{}, err
	}
	return ModelToolResult{
		ToolCallID:      strings.TrimSpace(providerCallID),
		ToolCallEventID: strings.TrimSpace(toolCallEventID),
		Name:            strings.TrimSpace(toolName),
		Content:         string(content),
	}, nil
}

func (k *Kernel) cancelJobModelToolResult(sessionID string, turnID string, toolCallEventID string, providerCallID string, toolName string, jobID string, reason string) (ModelToolResult, error) {
	k.jobMu.Lock()
	job, ok, err := k.lookupSessionJob(sessionID, jobID)
	if err != nil {
		k.jobMu.Unlock()
		return ModelToolResult{}, err
	}
	if !ok {
		k.jobMu.Unlock()
		return invalidModelToolResult(toolCallEventID, providerCallID, toolName, "job_not_found", fmt.Sprintf("job %q was not found", jobID))
	}
	if isTerminalJobStatus(job.Status) {
		k.jobMu.Unlock()
		content, err := json.Marshal(modelJobControlResult(job, false, jobCancelVisibleOutput(job, false)))
		if err != nil {
			return ModelToolResult{}, err
		}
		return ModelToolResult{
			ToolCallID:      strings.TrimSpace(providerCallID),
			ToolCallEventID: strings.TrimSpace(toolCallEventID),
			Name:            strings.TrimSpace(toolName),
			Content:         string(content),
		}, nil
	}

	requested := job
	requested.SessionID = strings.TrimSpace(sessionID)
	requested.TurnID = strings.TrimSpace(turnID)
	requested.Status = "cancel_requested"
	requested.CancelReason = strings.TrimSpace(reason)
	requested.CompletedAt = time.Time{}
	alreadyRequested := strings.TrimSpace(job.Status) == "cancel_requested"
	if !alreadyRequested {
		if err := k.appendJobEvent("job.cancel_requested", requested); err != nil {
			k.jobMu.Unlock()
			return ModelToolResult{}, err
		}
	}
	k.jobMu.Unlock()
	if k.jobExecutor != nil {
		if _, err := k.jobExecutor.Cancel(jobID, reason); err != nil {
			return ModelToolResult{}, fmt.Errorf("%w: managed job cancel failed: %v", ErrToolInfrastructureFailed, err)
		}
	}
	content, err := json.Marshal(modelJobControlResult(requested, true, jobCancelVisibleOutput(requested, true)))
	if err != nil {
		return ModelToolResult{}, err
	}
	return ModelToolResult{
		ToolCallID:      strings.TrimSpace(providerCallID),
		ToolCallEventID: strings.TrimSpace(toolCallEventID),
		Name:            strings.TrimSpace(toolName),
		Content:         string(content),
	}, nil
}

func (k *Kernel) lookupSessionJob(sessionID string, jobID string) (JobProjection, bool, error) {
	sessionID = strings.TrimSpace(sessionID)
	jobID = strings.TrimSpace(jobID)
	if sessionID == "" || jobID == "" {
		return JobProjection{}, false, nil
	}
	events, err := k.loadEvents()
	if err != nil {
		return JobProjection{}, false, err
	}
	var latest JobProjection
	found := false
	for _, event := range events {
		if event.SessionID != sessionID || event.Data.Job == nil {
			continue
		}
		if !isJobFactEvent(event.Type) {
			continue
		}
		job := *event.Data.Job
		if strings.TrimSpace(job.JobID) == "" {
			job.JobID = strings.TrimSpace(event.JobID)
		}
		if strings.TrimSpace(job.JobID) == "" && event.Type == "job.started" {
			job.JobID = strings.TrimSpace(event.EventID)
		}
		if strings.TrimSpace(job.JobID) != jobID {
			continue
		}
		latest = job
		found = true
	}
	if !found {
		return JobProjection{}, false, nil
	}
	return latest, true, nil
}

func (k *Kernel) lookupSessionJobByIdempotencyKey(sessionID string, tool string, key string) (JobProjection, bool, error) {
	sessionID = strings.TrimSpace(sessionID)
	tool = strings.TrimSpace(tool)
	key = strings.TrimSpace(key)
	if sessionID == "" || tool == "" || key == "" {
		return JobProjection{}, false, nil
	}
	events, err := k.loadEvents()
	if err != nil {
		return JobProjection{}, false, err
	}
	var latest JobProjection
	found := false
	for _, event := range events {
		if event.SessionID != sessionID || event.Data.Job == nil {
			continue
		}
		if !isJobFactEvent(event.Type) {
			continue
		}
		job := *event.Data.Job
		if strings.TrimSpace(job.Tool) != tool || strings.TrimSpace(job.IdempotencyKey) != key {
			continue
		}
		if strings.TrimSpace(job.JobID) == "" {
			job.JobID = strings.TrimSpace(event.JobID)
		}
		if strings.TrimSpace(job.JobID) == "" && event.Type == "job.started" {
			job.JobID = strings.TrimSpace(event.EventID)
		}
		latest = job
		found = true
	}
	if !found {
		return JobProjection{}, false, nil
	}
	return latest, true, nil
}

func isJobFactEvent(eventType string) bool {
	switch eventType {
	case "job.started", "job.output", "job.cancel_requested", "job.completed", "job.failed", "job.cancelled":
		return true
	default:
		return false
	}
}

func isTerminalJobStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "completed", "failed", "cancelled":
		return true
	default:
		return false
	}
}

func jobStatusVisibleOutput(job JobProjection) string {
	jobID := strings.TrimSpace(job.JobID)
	status := strings.TrimSpace(job.Status)
	tool := strings.TrimSpace(job.Tool)
	if tool == "" {
		return fmt.Sprintf("job %s is %s.", jobID, status)
	}
	return fmt.Sprintf("job %s is %s for tool %s.", jobID, status, tool)
}

func jobCancelVisibleOutput(job JobProjection, cancelRequested bool) string {
	jobID := strings.TrimSpace(job.JobID)
	if cancelRequested {
		return fmt.Sprintf("job %s cancellation was requested. The executor will record a terminal job fact when cancellation is confirmed.", jobID)
	}
	return fmt.Sprintf("job %s is already %s; no new cancellation event was recorded.", jobID, strings.TrimSpace(job.Status))
}
