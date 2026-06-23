package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func (k *Kernel) prepareManagedShellJob(req ShellExecRequest, turnID string, toolCallEventID string) (JobProjection, bool, error) {
	if err := validateShellRequest(req); err != nil {
		return JobProjection{}, false, err
	}
	timeoutSec := normalizedShellTimeoutSec(req.TimeoutSec)
	if timeoutSec <= maxForegroundShellTimeoutSec {
		return JobProjection{}, false, fmt.Errorf("managed shell jobs require timeout_sec greater than %d", maxForegroundShellTimeoutSec)
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
