package kernel

import "time"

type AuditReplayResponse struct {
	TurnID          string            `json:"turn_id"`
	SessionID       string            `json:"session_id,omitempty"`
	Readiness       string            `json:"readiness"`
	ReadinessReason string            `json:"readiness_reason,omitempty"`
	Items           []AuditReplayItem `json:"items"`
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
	SessionID       string           `json:"session_id"`
	Readiness       string           `json:"readiness"`
	ReadinessReason string           `json:"readiness_reason,omitempty"`
	Items           []UITimelineItem `json:"items"`
}

type UITimelineDetailResponse struct {
	SessionID       string         `json:"session_id"`
	Readiness       string         `json:"readiness"`
	ReadinessReason string         `json:"readiness_reason,omitempty"`
	DetailRef       string         `json:"detail_ref"`
	Item            UITimelineItem `json:"item"`
}

type UITimelineItem struct {
	ItemID              string           `json:"item_id"`
	TurnID              string           `json:"turn_id"`
	ReasoningID         string           `json:"reasoning_id,omitempty"`
	Kind                string           `json:"kind"`
	Phase               string           `json:"phase,omitempty"`
	WaitReason          string           `json:"wait_reason,omitempty"`
	TerminalOutcome     string           `json:"terminal_outcome,omitempty"`
	TerminalCause       string           `json:"terminal_cause,omitempty"`
	Text                string           `json:"text,omitempty"`
	Tool                string           `json:"tool,omitempty"`
	ApprovalID          string           `json:"approval_id,omitempty"`
	JobID               string           `json:"job_id,omitempty"`
	CommandPreview      string           `json:"command_preview,omitempty"`
	OutputPreview       string           `json:"output_preview,omitempty"`
	VisibleOutput       string           `json:"visible_output,omitempty"`
	OutputSource        string           `json:"output_source,omitempty"`
	OutputTruncated     bool             `json:"output_truncated,omitempty"`
	OutputTruncation    string           `json:"output_truncation,omitempty"`
	StdoutOriginalBytes int              `json:"stdout_original_bytes,omitempty"`
	StderrOriginalBytes int              `json:"stderr_original_bytes,omitempty"`
	StdoutOmittedBytes  int              `json:"stdout_omitted_bytes,omitempty"`
	StderrOmittedBytes  int              `json:"stderr_omitted_bytes,omitempty"`
	OriginalBytes       int              `json:"original_bytes,omitempty"`
	ReturnedBytes       int              `json:"returned_bytes,omitempty"`
	FullOutputAvailable bool             `json:"full_output_available,omitempty"`
	DefaultOpen         bool             `json:"default_open,omitempty"`
	DetailRef           string           `json:"detail_ref,omitempty"`
	DetailAvailable     bool             `json:"detail_available,omitempty"`
	DurationMs          int64            `json:"duration_ms,omitempty"`
	ToolCount           int              `json:"tool_count,omitempty"`
	JobCount            int              `json:"job_count,omitempty"`
	CompactionCount     int              `json:"compaction_count,omitempty"`
	Children            []UITimelineItem `json:"children"`
	CreatedAt           time.Time        `json:"created_at"`
	UpdatedAt           time.Time        `json:"updated_at,omitempty"`
}

type ContextInspectionResponse struct {
	TurnID              string                       `json:"turn_id"`
	SessionID           string                       `json:"session_id,omitempty"`
	Readiness           string                       `json:"readiness"`
	ReadinessReason     string                       `json:"readiness_reason,omitempty"`
	InputItems          []InputItem                  `json:"input_items"`
	ModelInputKinds     []string                     `json:"model_input_kinds"`
	PrefixFingerprint   string                       `json:"prefix_fingerprint,omitempty"`
	PrefixChangeReasons []string                     `json:"prefix_change_reasons,omitempty"`
	ToolManifest        []ToolManifestInspection     `json:"tool_manifest"`
	SkillCatalog        []SkillCatalogItemProjection `json:"skill_catalog"`
	SourceSnapshots     []SourceSnapshotDescriptor   `json:"source_snapshots,omitempty"`
	HydratedContexts    []ContextHydrationProjection `json:"hydrated_contexts,omitempty"`
	Runtime             *ContextRuntimeSnapshot      `json:"runtime,omitempty"`
	UnavailableReason   string                       `json:"unavailable_reason,omitempty"`
}

type ToolManifestInspection struct {
	Name string `json:"name"`
}

type ContextRuntimeSnapshot struct {
	Provider           ProviderStatus           `json:"provider"`
	BudgetLease        BudgetLeaseProjection    `json:"budget_lease"`
	ShellTimeoutPolicy ShellTimeoutPolicy       `json:"shell_timeout_policy"`
	Limits             []RuntimeLimitProjection `json:"limits"`
	Permission         PermissionInspection     `json:"permission"`
}

type PermissionInspection struct {
	PermissionMode  string `json:"permission_mode"`
	AuthorityPolicy string `json:"authority_policy"`
	SandboxProfile  string `json:"sandbox_profile"`
	ApprovalPolicy  string `json:"approval_policy"`
}

type SessionProjection struct {
	SessionID        string                       `json:"session_id"`
	WorkspaceMode    string                       `json:"workspace_mode,omitempty"`
	Turns            []TurnProjection             `json:"turns"`
	Operations       []OperationProjection        `json:"operations"`
	Jobs             []JobProjection              `json:"jobs"`
	Approvals        []ApprovalProjection         `json:"approvals"`
	SandboxReadiness []SandboxReadinessProjection `json:"sandbox_readiness"`
	Works            []WorkProjection             `json:"works"`
	MemoryCandidates []MemoryCandidateProjection  `json:"memory_candidates"`
	Events           []EventProjection            `json:"events"`
}

type SessionListResponse struct {
	Items []SessionListItem `json:"items"`
}

type SessionListItem struct {
	SessionID string    `json:"session_id"`
	Title     string    `json:"title,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SessionSearchRequest struct {
	Query string
	Limit int
}

type SessionSearchResponse struct {
	Query string                `json:"query"`
	Items []SessionSearchResult `json:"items"`
}

type SessionSearchResult struct {
	SessionID   string    `json:"session_id"`
	Title       string    `json:"title,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
	MatchFields []string  `json:"match_fields"`
	Snippet     string    `json:"snippet,omitempty"`
}
