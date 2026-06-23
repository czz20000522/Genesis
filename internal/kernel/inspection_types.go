package kernel

import "time"

type AuditReplayResponse struct {
	TurnID    string            `json:"turn_id"`
	SessionID string            `json:"session_id,omitempty"`
	Status    string            `json:"status"`
	Items     []AuditReplayItem `json:"items"`
}

type AuditReplayItem struct {
	EventID              string      `json:"event_id"`
	EventType            string      `json:"event_type"`
	TurnID               string      `json:"turn_id"`
	OperationID          string      `json:"operation_id,omitempty"`
	JobID                string      `json:"job_id,omitempty"`
	CreatedAt            time.Time   `json:"created_at"`
	ModelInputKinds      []string    `json:"model_input_kinds,omitempty"`
	Tool                 string      `json:"tool,omitempty"`
	ToolStatus           string      `json:"tool_status,omitempty"`
	OutputPreview        string      `json:"output_preview,omitempty"`
	OutputTruncated      bool        `json:"output_truncated,omitempty"`
	OutputTruncation     string      `json:"output_truncation,omitempty"`
	StdoutOriginalBytes  int         `json:"stdout_original_bytes,omitempty"`
	StderrOriginalBytes  int         `json:"stderr_original_bytes,omitempty"`
	StdoutOmittedBytes   int         `json:"stdout_omitted_bytes,omitempty"`
	StderrOmittedBytes   int         `json:"stderr_omitted_bytes,omitempty"`
	ProviderContextKinds []string    `json:"provider_context_kinds,omitempty"`
	Usage                *TokenUsage `json:"usage,omitempty"`
	ErrorCode            string      `json:"error_code,omitempty"`
	ErrorMessage         string      `json:"error_message,omitempty"`
}

type UITimelineResponse struct {
	SessionID string           `json:"session_id"`
	Status    string           `json:"status"`
	Items     []UITimelineItem `json:"items"`
}

type UITimelineItem struct {
	ItemID              string    `json:"item_id"`
	TurnID              string    `json:"turn_id"`
	Kind                string    `json:"kind"`
	Status              string    `json:"status,omitempty"`
	Text                string    `json:"text,omitempty"`
	Tool                string    `json:"tool,omitempty"`
	OutputPreview       string    `json:"output_preview,omitempty"`
	OutputSource        string    `json:"output_source,omitempty"`
	OutputTruncated     bool      `json:"output_truncated,omitempty"`
	FullOutputAvailable bool      `json:"full_output_available,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at,omitempty"`
}

type ContextInspectionResponse struct {
	TurnID            string                       `json:"turn_id"`
	SessionID         string                       `json:"session_id,omitempty"`
	Status            string                       `json:"status"`
	InputItems        []InputItem                  `json:"input_items,omitempty"`
	ModelInputKinds   []string                     `json:"model_input_kinds,omitempty"`
	ToolManifest      []ToolSpec                   `json:"tool_manifest,omitempty"`
	SkillCatalog      []SkillCatalogItemProjection `json:"skill_catalog,omitempty"`
	RecalledMemories  []MemoryRecall               `json:"recalled_memories,omitempty"`
	Runtime           *ContextRuntimeSnapshot      `json:"runtime,omitempty"`
	UnavailableReason string                       `json:"unavailable_reason,omitempty"`
}

type ContextRuntimeSnapshot struct {
	Provider   ProviderStatus       `json:"provider"`
	Permission PermissionInspection `json:"permission"`
}

type PermissionInspection struct {
	PermissionMode  string `json:"permission_mode"`
	AuthorityPolicy string `json:"authority_policy"`
	SandboxProfile  string `json:"sandbox_profile"`
	ApprovalPolicy  string `json:"approval_policy"`
}

type SessionProjection struct {
	SessionID        string                      `json:"session_id"`
	Turns            []TurnProjection            `json:"turns"`
	Operations       []OperationProjection       `json:"operations"`
	Jobs             []JobProjection             `json:"jobs,omitempty"`
	Works            []WorkProjection            `json:"works"`
	MemoryCandidates []MemoryCandidateProjection `json:"memory_candidates"`
	Events           []EventProjection           `json:"events"`
}
