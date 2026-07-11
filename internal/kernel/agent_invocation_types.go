package kernel

import "time"

const (
	AgentInvocationStatusAdmitted = "admitted"

	AgentInvocationRunStatusRunning   = "running"
	AgentInvocationRunStatusCompleted = "completed"
	AgentInvocationRunStatusFailed    = "failed"
)

type CapabilityGrant struct {
	ToolNames []string `json:"tool_names,omitempty"`
}

type AgentInvocationAdmissionRequest struct {
	SessionID           string          `json:"session_id"`
	ParentTurnID        string          `json:"parent_turn_id,omitempty"`
	ParentInvocationID  string          `json:"parent_invocation_id,omitempty"`
	Principal           string          `json:"principal"`
	AgentProfileRef     string          `json:"agent_profile_ref,omitempty"`
	CapabilityGrant     CapabilityGrant `json:"capability_grant"`
	ContextScope        string          `json:"context_scope,omitempty"`
	ParentResultChannel string          `json:"parent_result_channel,omitempty"`
	IdempotencyKey      string          `json:"idempotency_key,omitempty"`
}

type WorkerInvocationAdmissionRequest struct {
	ConfigRoot          string   `json:"config_root,omitempty"`
	ParentID            string   `json:"parent_id,omitempty"`
	RoleID              string   `json:"role_id,omitempty"`
	SessionID           string   `json:"session_id"`
	ParentTurnID        string   `json:"parent_turn_id,omitempty"`
	ParentInvocationID  string   `json:"parent_invocation_id,omitempty"`
	Principal           string   `json:"principal"`
	RequestedToolNames  []string `json:"requested_tool_names,omitempty"`
	ContextScope        string   `json:"context_scope,omitempty"`
	ParentResultChannel string   `json:"parent_result_channel,omitempty"`
	IdempotencyKey      string   `json:"idempotency_key,omitempty"`
}

type AgentInvocationProjection struct {
	InvocationID        string          `json:"invocation_id"`
	SessionID           string          `json:"session_id"`
	ParentRoleID        string          `json:"parent_role_id,omitempty"`
	ParentTurnID        string          `json:"parent_turn_id,omitempty"`
	ParentInvocationID  string          `json:"parent_invocation_id,omitempty"`
	Principal           string          `json:"principal"`
	AgentProfileRef     string          `json:"agent_profile_ref,omitempty"`
	ModelProfileID      string          `json:"model_profile_id,omitempty"`
	CapabilityGrant     CapabilityGrant `json:"capability_grant"`
	ContextScope        string          `json:"context_scope,omitempty"`
	ParentResultChannel string          `json:"parent_result_channel,omitempty"`
	Status              string          `json:"status"`
	IdempotencyKey      string          `json:"idempotency_key,omitempty"`
	AdmittedAt          time.Time       `json:"admitted_at"`
}

type AgentInvocationRunRequest struct {
	InvocationID   string      `json:"invocation_id"`
	Principal      string      `json:"principal"`
	InputItems     []InputItem `json:"input_items"`
	IdempotencyKey string      `json:"idempotency_key,omitempty"`
}

type AgentInvocationRunProjection struct {
	InvocationID    string       `json:"invocation_id"`
	RunID           string       `json:"run_id"`
	SessionID       string       `json:"session_id"`
	Principal       string       `json:"principal"`
	Status          string       `json:"status"`
	ModelInputKinds []string     `json:"model_input_kinds,omitempty"`
	Model           string       `json:"model,omitempty"`
	Usage           *TokenUsage  `json:"usage,omitempty"`
	Final           FinalMessage `json:"final,omitempty"`
	Error           *TurnError   `json:"error,omitempty"`
	IdempotencyKey  string       `json:"idempotency_key,omitempty"`
	StartedAt       time.Time    `json:"started_at"`
	CompletedAt     time.Time    `json:"completed_at,omitempty"`
}

type AgentInvocationChildConversationProjection struct {
	InvocationID       string       `json:"invocation_id"`
	RunID              string       `json:"run_id,omitempty"`
	SessionID          string       `json:"session_id"`
	ParentInvocationID string       `json:"parent_invocation_id,omitempty"`
	Principal          string       `json:"principal,omitempty"`
	RoleID             string       `json:"role_id,omitempty"`
	AgentProfileRef    string       `json:"agent_profile_ref,omitempty"`
	ContextScope       string       `json:"context_scope,omitempty"`
	Status             string       `json:"status"`
	ToolSet            []string     `json:"tool_set,omitempty"`
	ModelInputKinds    []string     `json:"model_input_kinds,omitempty"`
	Model              string       `json:"model,omitempty"`
	Usage              *TokenUsage  `json:"usage,omitempty"`
	Final              FinalMessage `json:"final,omitempty"`
	Error              *TurnError   `json:"error,omitempty"`
	EvidenceRefs       []string     `json:"evidence_refs,omitempty"`
	AdmittedAt         time.Time    `json:"admitted_at"`
	StartedAt          time.Time    `json:"started_at,omitempty"`
	CompletedAt        time.Time    `json:"completed_at,omitempty"`
}
