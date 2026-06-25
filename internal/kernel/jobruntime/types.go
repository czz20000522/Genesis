package jobruntime

import "time"

type JobProjection struct {
	JobID           string    `json:"job_id"`
	SessionID       string    `json:"session_id"`
	TurnID          string    `json:"turn_id,omitempty"`
	Tool            string    `json:"tool"`
	IdempotencyKey  string    `json:"idempotency_key,omitempty"`
	Status          string    `json:"status"`
	CWD             string    `json:"cwd,omitempty"`
	Command         string    `json:"command,omitempty"`
	TimeoutSec      int       `json:"timeout_sec,omitempty"`
	ExitCode        *int      `json:"exit_code,omitempty"`
	Stdout          string    `json:"stdout,omitempty"`
	Stderr          string    `json:"stderr,omitempty"`
	StdoutTruncated bool      `json:"stdout_truncated,omitempty"`
	StderrTruncated bool      `json:"stderr_truncated,omitempty"`
	Receipt         string    `json:"receipt,omitempty"`
	FailureReason   string    `json:"failure_reason,omitempty"`
	CancelReason    string    `json:"cancel_reason,omitempty"`
	StartedAt       time.Time `json:"started_at"`
	CompletedAt     time.Time `json:"completed_at,omitempty"`
	ToolCallEventID string    `json:"tool_call_event_id,omitempty"`
}

type ObservationDelivery struct {
	ObservationEventIDs []string `json:"observation_event_ids,omitempty"`
	ModelInputKind      string   `json:"model_input_kind,omitempty"`
}
