package kernel

import (
	"encoding/json"
	"fmt"
	"strings"
)

func (k *Kernel) startManagedShellJobReceipt(sessionID string, turnID string, toolCallEventID string, providerCallID string, toolName string, req ShellExecRequest) (ModelToolResult, error) {
	startedAt := k.clock()
	jobID := newID("job", startedAt)
	receipt := fmt.Sprintf("Command was accepted as managed job %s. No synchronous command output is available in this tool result; terminal job evidence is recorded in the session events.", jobID)
	started := JobProjection{
		JobID:           jobID,
		SessionID:       strings.TrimSpace(sessionID),
		TurnID:          strings.TrimSpace(turnID),
		Tool:            "shell_exec",
		Status:          "running",
		CWD:             strings.TrimSpace(req.CWD),
		Command:         strings.TrimSpace(req.Command),
		TimeoutSec:      normalizedShellTimeoutSec(req.TimeoutSec),
		Receipt:         receipt,
		StartedAt:       startedAt,
		ToolCallEventID: strings.TrimSpace(toolCallEventID),
	}
	if err := k.appendJobEvent("job.started", started); err != nil {
		return ModelToolResult{}, err
	}
	completed := started
	completed.Status = "completed"
	completed.CompletedAt = k.clock()
	content, err := json.Marshal(ModelManagedJobResult{
		Status:        "managed_job_started",
		Executed:      true,
		JobID:         jobID,
		VisibleOutput: receipt,
	})
	if err != nil {
		return ModelToolResult{}, err
	}
	return ModelToolResult{
		ToolCallID:           strings.TrimSpace(providerCallID),
		ToolCallEventID:      strings.TrimSpace(toolCallEventID),
		Name:                 strings.TrimSpace(toolName),
		Content:              string(content),
		PendingJobCompletion: &completed,
	}, nil
}

func (k *Kernel) appendJobEvent(eventType string, job JobProjection) error {
	createdAt := job.StartedAt
	if eventType != "job.started" && !job.CompletedAt.IsZero() {
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
