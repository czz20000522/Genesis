package kernel

import (
	"encoding/json"
	"time"
)

type ToolSpec struct {
	Name            string                 `json:"name"`
	Description     string                 `json:"description"`
	InputSchema     map[string]interface{} `json:"input_schema"`
	SideEffectLevel string                 `json:"side_effect_level"`
	ExecutionKind   string                 `json:"execution_kind"`
	Scheduling      ToolSchedulingSpec     `json:"-"`
}

type ToolSchedulingSpec struct {
	EffectClass       string                `json:"-"`
	ResourceFootprint ToolResourceFootprint `json:"-"`
	ParallelPolicy    string                `json:"-"`
}

type ToolResourceFootprint struct {
	ReadScopes      []string `json:"-"`
	WriteScopes     []string `json:"-"`
	StateScopes     []string `json:"-"`
	Handles         []string `json:"-"`
	ExternalTargets []string `json:"-"`
}

type ModelToolCall struct {
	ToolCallID      string          `json:"tool_call_id"`
	ToolCallEventID string          `json:"tool_call_event_id,omitempty"`
	Name            string          `json:"name"`
	Arguments       json.RawMessage `json:"arguments,omitempty"`
}

func (c ModelToolCall) MarshalJSON() ([]byte, error) {
	type payload struct {
		ToolCallID      string          `json:"tool_call_id,omitempty"`
		ToolCallEventID string          `json:"tool_call_event_id,omitempty"`
		Name            string          `json:"name"`
		Arguments       json.RawMessage `json:"arguments,omitempty"`
		RawArguments    string          `json:"raw_arguments,omitempty"`
	}
	next := payload{
		ToolCallID:      c.ToolCallID,
		ToolCallEventID: c.ToolCallEventID,
		Name:            c.Name,
	}
	if len(c.Arguments) > 0 {
		if json.Valid(c.Arguments) {
			next.Arguments = append(json.RawMessage(nil), c.Arguments...)
		} else {
			next.RawArguments = string(c.Arguments)
		}
	}
	return json.Marshal(next)
}

func (c *ModelToolCall) UnmarshalJSON(data []byte) error {
	type payload struct {
		ToolCallID      string           `json:"tool_call_id"`
		ToolCallEventID string           `json:"tool_call_event_id,omitempty"`
		Name            string           `json:"name"`
		Arguments       *json.RawMessage `json:"arguments,omitempty"`
		RawArguments    *string          `json:"raw_arguments,omitempty"`
	}
	var next payload
	if err := json.Unmarshal(data, &next); err != nil {
		return err
	}
	c.ToolCallID = next.ToolCallID
	c.ToolCallEventID = next.ToolCallEventID
	c.Name = next.Name
	c.Arguments = nil
	switch {
	case next.RawArguments != nil:
		c.Arguments = json.RawMessage(*next.RawArguments)
	case next.Arguments != nil:
		c.Arguments = append(json.RawMessage(nil), (*next.Arguments)...)
	}
	return nil
}

type ModelToolRound struct {
	Calls   []ModelToolCall   `json:"calls"`
	Results []ModelToolResult `json:"results"`
}

type ModelToolResult struct {
	ToolCallID      string         `json:"tool_call_id"`
	ToolCallEventID string         `json:"tool_call_event_id,omitempty"`
	Name            string         `json:"name"`
	Content         string         `json:"content"`
	PendingJobStart *JobProjection `json:"-"`
}

type ToolRequestInvalidProjection struct {
	Status   string           `json:"status"`
	Tool     string           `json:"tool"`
	Executed bool             `json:"executed"`
	Error    ToolRequestError `json:"error"`
}

type ToolRequestError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ToolCapabilityProjection struct {
	Name            string `json:"name"`
	SideEffectLevel string `json:"side_effect_level"`
	ExecutionKind   string `json:"execution_kind"`
	Status          string `json:"status"`
}

type ShellExecRequest struct {
	SessionID      string `json:"session_id"`
	CWD            string `json:"cwd"`
	Command        string `json:"command"`
	TimeoutSec     int    `json:"timeout_sec,omitempty"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	approvedByID   string
}

type OperationProjection struct {
	OperationID          string    `json:"operation_id"`
	SessionID            string    `json:"session_id"`
	TurnID               string    `json:"turn_id,omitempty"`
	Tool                 string    `json:"tool"`
	IdempotencyKey       string    `json:"idempotency_key,omitempty"`
	Status               string    `json:"status"`
	PermissionMode       string    `json:"permission_mode"`
	AuthorityPolicy      string    `json:"authority_policy,omitempty"`
	SandboxProfile       string    `json:"sandbox_profile,omitempty"`
	ApprovalPolicy       string    `json:"approval_policy,omitempty"`
	CWD                  string    `json:"cwd"`
	Command              string    `json:"command"`
	TimeoutSec           int       `json:"timeout_sec,omitempty"`
	TimedOut             bool      `json:"timed_out,omitempty"`
	TimeoutReason        string    `json:"timeout_reason,omitempty"`
	Interrupted          bool      `json:"interrupted,omitempty"`
	InterruptReason      string    `json:"interrupt_reason,omitempty"`
	ElapsedMs            int64     `json:"elapsed_ms,omitempty"`
	ExitCode             *int      `json:"exit_code,omitempty"`
	Stdout               string    `json:"stdout,omitempty"`
	Stderr               string    `json:"stderr,omitempty"`
	StdoutTruncated      bool      `json:"stdout_truncated,omitempty"`
	StderrTruncated      bool      `json:"stderr_truncated,omitempty"`
	StdoutOriginalBytes  int       `json:"stdout_original_bytes,omitempty"`
	StderrOriginalBytes  int       `json:"stderr_original_bytes,omitempty"`
	StdoutOmittedBytes   int       `json:"stdout_omitted_bytes,omitempty"`
	StderrOmittedBytes   int       `json:"stderr_omitted_bytes,omitempty"`
	OutputTruncation     string    `json:"output_truncation,omitempty"`
	BlockedReason        string    `json:"blocked_reason,omitempty"`
	InfrastructureReason string    `json:"infrastructure_reason,omitempty"`
	StartedAt            time.Time `json:"started_at"`
	EndedAt              time.Time `json:"ended_at"`
}

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

type KernelObservationDeliveryProjection struct {
	ObservationEventIDs []string `json:"observation_event_ids,omitempty"`
	ModelInputKind      string   `json:"model_input_kind,omitempty"`
}

type ModelOperationResult struct {
	Status              string `json:"status"`
	Executed            bool   `json:"executed"`
	ExitCode            *int   `json:"exit_code,omitempty"`
	TimedOut            bool   `json:"timed_out,omitempty"`
	TimeoutReason       string `json:"timeout_reason,omitempty"`
	Interrupted         bool   `json:"interrupted,omitempty"`
	InterruptReason     string `json:"interrupt_reason,omitempty"`
	ElapsedMs           int64  `json:"elapsed_ms,omitempty"`
	Stdout              string `json:"stdout,omitempty"`
	Stderr              string `json:"stderr,omitempty"`
	StdoutTruncated     bool   `json:"stdout_truncated,omitempty"`
	StderrTruncated     bool   `json:"stderr_truncated,omitempty"`
	StdoutOriginalBytes int    `json:"stdout_original_bytes,omitempty"`
	StderrOriginalBytes int    `json:"stderr_original_bytes,omitempty"`
	StdoutOmittedBytes  int    `json:"stdout_omitted_bytes,omitempty"`
	StderrOmittedBytes  int    `json:"stderr_omitted_bytes,omitempty"`
	OutputTruncation    string `json:"output_truncation,omitempty"`
}

type ModelManagedJobResult struct {
	Status        string `json:"status"`
	Executed      bool   `json:"executed"`
	JobID         string `json:"job_id"`
	VisibleOutput string `json:"visible_output"`
}

type ModelJobControlResult struct {
	Status          string `json:"status"`
	Executed        bool   `json:"executed"`
	JobID           string `json:"job_id"`
	Tool            string `json:"tool,omitempty"`
	CancelRequested bool   `json:"cancel_requested,omitempty"`
	VisibleOutput   string `json:"visible_output,omitempty"`
	ExitCode        *int   `json:"exit_code,omitempty"`
	Stdout          string `json:"stdout,omitempty"`
	Stderr          string `json:"stderr,omitempty"`
	StdoutTruncated bool   `json:"stdout_truncated,omitempty"`
	StderrTruncated bool   `json:"stderr_truncated,omitempty"`
}

type ToolCallProjection struct {
	ToolCallEventID    string `json:"tool_call_event_id"`
	ProviderToolCallID string `json:"provider_tool_call_id,omitempty"`
	Tool               string `json:"tool"`
	Arguments          string `json:"arguments,omitempty"`
}

type ToolResultProjection struct {
	ToolCallEventID    string `json:"tool_call_event_id"`
	ProviderToolCallID string `json:"provider_tool_call_id,omitempty"`
	Tool               string `json:"tool"`
	ForEventID         string `json:"for_event_id"`
	Status             string `json:"status"`
	Content            string `json:"content,omitempty"`
}
