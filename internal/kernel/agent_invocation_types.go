package kernel

import "time"

const (
	AgentInvocationStatusAdmitted = "admitted"
)

type CapabilityGrant struct {
	ToolNames []string `json:"tool_names,omitempty"`
}

type AgentInvocationAdmissionRequest struct {
	SessionID           string          `json:"session_id"`
	ParentInvocationID  string          `json:"parent_invocation_id,omitempty"`
	Principal           string          `json:"principal"`
	AgentProfileRef     string          `json:"agent_profile_ref,omitempty"`
	CapabilityGrant     CapabilityGrant `json:"capability_grant"`
	ContextScope        string          `json:"context_scope,omitempty"`
	ParentResultChannel string          `json:"parent_result_channel,omitempty"`
	IdempotencyKey      string          `json:"idempotency_key,omitempty"`
}

type AgentInvocationProjection struct {
	InvocationID        string          `json:"invocation_id"`
	SessionID           string          `json:"session_id"`
	ParentInvocationID  string          `json:"parent_invocation_id,omitempty"`
	Principal           string          `json:"principal"`
	AgentProfileRef     string          `json:"agent_profile_ref,omitempty"`
	CapabilityGrant     CapabilityGrant `json:"capability_grant"`
	ContextScope        string          `json:"context_scope,omitempty"`
	ParentResultChannel string          `json:"parent_result_channel,omitempty"`
	Status              string          `json:"status"`
	IdempotencyKey      string          `json:"idempotency_key,omitempty"`
	AdmittedAt          time.Time       `json:"admitted_at"`
}
