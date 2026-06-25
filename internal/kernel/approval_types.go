package kernel

import "time"

const (
	ApprovalStatusPending  = "pending"
	ApprovalStatusApproved = "approved"
	ApprovalStatusDenied   = "denied"
	ApprovalStatusExpired  = "expired"

	ApprovalDecisionApproved = "approved"
	ApprovalDecisionDenied   = "denied"

	SandboxReadinessReady       = "ready"
	SandboxReadinessUnavailable = "unavailable"

	defaultApprovalTTL = 15 * time.Minute
)

type ApprovalListResponse struct {
	Items []ApprovalProjection `json:"items"`
}

type ApprovalDecisionRequest struct {
	ApprovalID          string `json:"approval_id,omitempty"`
	Decision            string `json:"decision"`
	DecisionAuthority   string `json:"decision_authority"`
	DecisionReason      string `json:"decision_reason"`
	DecisionEvidenceRef string `json:"decision_evidence_ref"`
}

type ApprovalProjection struct {
	ApprovalID          string                 `json:"approval_id"`
	SessionID           string                 `json:"session_id"`
	TurnID              string                 `json:"turn_id,omitempty"`
	ToolCallEventID     string                 `json:"tool_call_event_id,omitempty"`
	OperationID         string                 `json:"operation_id,omitempty"`
	Status              string                 `json:"status"`
	Tool                string                 `json:"tool"`
	PolicySnapshot      ApprovalPolicySnapshot `json:"policy_snapshot"`
	Effect              ApprovalEffectSummary  `json:"effect"`
	RequestedAt         time.Time              `json:"requested_at"`
	ExpiresAt           time.Time              `json:"expires_at"`
	DecidedAt           time.Time              `json:"decided_at,omitempty"`
	DecisionAuthority   string                 `json:"decision_authority,omitempty"`
	DecisionReason      string                 `json:"decision_reason,omitempty"`
	DecisionEvidenceRef string                 `json:"decision_evidence_ref,omitempty"`
	BlockedReason       string                 `json:"blocked_reason,omitempty"`
}

type ApprovalPolicySnapshot struct {
	PermissionMode  string `json:"permission_mode"`
	AuthorityPolicy string `json:"authority_policy"`
	SandboxProfile  string `json:"sandbox_profile"`
	ApprovalPolicy  string `json:"approval_policy"`
	WorkspaceRoot   string `json:"workspace_root,omitempty"`
	ExecutorAdapter string `json:"executor_adapter"`
}

type ApprovalEffectSummary struct {
	Tool           string `json:"tool"`
	ExecutionKind  string `json:"execution_kind"`
	SideEffect     string `json:"side_effect"`
	CWD            string `json:"cwd,omitempty"`
	CommandPreview string `json:"command_preview,omitempty"`
	TimeoutSec     int    `json:"timeout_sec,omitempty"`
}

type SandboxReadinessProjection struct {
	SandboxReadinessID string    `json:"sandbox_readiness_id"`
	SessionID          string    `json:"session_id"`
	TurnID             string    `json:"turn_id,omitempty"`
	OperationID        string    `json:"operation_id,omitempty"`
	SandboxProfile     string    `json:"sandbox_profile"`
	WorkspaceRoot      string    `json:"workspace_root,omitempty"`
	ExecutorAdapter    string    `json:"executor_adapter"`
	Status             string    `json:"status"`
	UnavailableReason  string    `json:"unavailable_reason,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}
