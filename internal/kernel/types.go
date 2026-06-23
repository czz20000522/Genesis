package kernel

import (
	"encoding/json"
	"time"
)

type Config struct {
	LedgerPath    string
	Provider      Provider
	RuntimeToken  string
	ToolPolicy    ToolPolicy
	ContextPolicy ContextPolicy
	SkillRoots    []string
	Clock         func() time.Time
}

type ToolPolicy struct {
	PermissionMode string
	WorkspaceRoot  string
}

type ContextPolicy struct {
	ContextWindowTokens int
	AutoCompactRatio    float64
	RecentTurnLimit     int
	RecentTailTokens    int
	SkillIndexChars     int
	RetryBackoffTurns   int
}

type ReadyResponse struct {
	Status      string         `json:"status"`
	Provider    ProviderStatus `json:"provider"`
	RuntimeAuth ReadyCheck     `json:"runtime_auth"`
	Ledger      ReadyCheck     `json:"ledger"`
}

type CapabilitiesResponse struct {
	Status       string                     `json:"status"`
	Provider     ProviderStatus             `json:"provider"`
	RuntimeAuth  ReadyCheck                 `json:"runtime_auth"`
	Ledger       ReadyCheck                 `json:"ledger"`
	Tools        []ToolCapabilityProjection `json:"tools"`
	SkillCatalog SkillCatalogProjection     `json:"skill_catalog"`
}

type ProviderStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type ReadyCheck struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type TurnRequest struct {
	SessionID      string      `json:"session_id,omitempty"`
	IdempotencyKey string      `json:"idempotency_key,omitempty"`
	InputItems     []InputItem `json:"input_items"`
}

type InputItem struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type ToolSpec struct {
	Name            string                 `json:"name"`
	Description     string                 `json:"description"`
	InputSchema     map[string]interface{} `json:"input_schema"`
	SideEffectLevel string                 `json:"side_effect_level"`
	ExecutionKind   string                 `json:"execution_kind"`
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
	ToolCallID           string         `json:"tool_call_id"`
	ToolCallEventID      string         `json:"tool_call_event_id,omitempty"`
	Name                 string         `json:"name"`
	Content              string         `json:"content"`
	PendingJobCompletion *JobProjection `json:"-"`
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

type SkillDescriptor struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	InstructionPath string `json:"-"`
}

type ToolCapabilityProjection struct {
	Name            string `json:"name"`
	SideEffectLevel string `json:"side_effect_level"`
	ExecutionKind   string `json:"execution_kind"`
	Status          string `json:"status"`
}

type SkillCatalogProjection struct {
	Status     string                            `json:"status"`
	Count      int                               `json:"count"`
	Items      []SkillCatalogItemProjection      `json:"items"`
	Exclusions []SkillCatalogExclusionProjection `json:"exclusions,omitempty"`
}

type SkillCatalogItemProjection struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type SkillCatalogExclusionProjection struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

type TurnResponse struct {
	SessionID string       `json:"session_id"`
	TurnID    string       `json:"turn_id"`
	Events    []Event      `json:"events"`
	Final     FinalMessage `json:"final"`
	Error     *TurnError   `json:"error,omitempty"`
}

type TurnEventsResponse struct {
	Items []Event `json:"items"`
}

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

type FinalMessage struct {
	Text  string      `json:"text"`
	Model string      `json:"model"`
	Usage *TokenUsage `json:"usage,omitempty"`
}

type TokenUsage struct {
	InputTokens     int `json:"input_tokens,omitempty"`
	OutputTokens    int `json:"output_tokens,omitempty"`
	TotalTokens     int `json:"total_tokens,omitempty"`
	CacheHitTokens  int `json:"cache_hit_tokens,omitempty"`
	CacheMissTokens int `json:"cache_miss_tokens,omitempty"`
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

type TurnProjection struct {
	TurnID           string         `json:"turn_id"`
	IdempotencyKey   string         `json:"idempotency_key,omitempty"`
	Status           string         `json:"status"`
	InputItems       []InputItem    `json:"input_items"`
	IngressRisks     []IngressRisk  `json:"ingress_risks,omitempty"`
	ModelInputKinds  []string       `json:"model_input_kinds,omitempty"`
	RecalledMemories []MemoryRecall `json:"recalled_memories,omitempty"`
	FinalMessage     FinalMessage   `json:"final,omitempty"`
	Error            *TurnError     `json:"error,omitempty"`
	StartedAt        time.Time      `json:"started_at"`
	CompletedAt      time.Time      `json:"completed_at,omitempty"`
}

type TurnError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type EventProjection struct {
	EventID     string    `json:"event_id"`
	TurnID      string    `json:"turn_id"`
	OperationID string    `json:"operation_id,omitempty"`
	JobID       string    `json:"job_id,omitempty"`
	WorkID      string    `json:"work_id,omitempty"`
	CandidateID string    `json:"candidate_id,omitempty"`
	Type        string    `json:"type"`
	CreatedAt   time.Time `json:"created_at"`
	Data        EventData `json:"data,omitempty"`
}

type ShellExecRequest struct {
	SessionID      string `json:"session_id"`
	CWD            string `json:"cwd"`
	Command        string `json:"command"`
	TimeoutSec     int    `json:"timeout_sec,omitempty"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
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
	Status          string    `json:"status"`
	CWD             string    `json:"cwd,omitempty"`
	Command         string    `json:"command,omitempty"`
	TimeoutSec      int       `json:"timeout_sec,omitempty"`
	Receipt         string    `json:"receipt,omitempty"`
	FailureReason   string    `json:"failure_reason,omitempty"`
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

type WorkSubmitRequest struct {
	SessionID      string `json:"session_id"`
	Title          string `json:"title"`
	SourceRef      string `json:"source_ref"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

type WorkCancelRequest struct {
	CancelAuthority   string `json:"cancel_authority"`
	CancelReason      string `json:"cancel_reason"`
	CancelEvidenceRef string `json:"cancel_evidence_ref"`
}

type WorkProjection struct {
	WorkID            string     `json:"work_id"`
	SessionID         string     `json:"session_id"`
	Title             string     `json:"title"`
	SourceRef         string     `json:"source_ref"`
	IdempotencyKey    string     `json:"idempotency_key,omitempty"`
	Status            string     `json:"status"`
	CreatedAt         time.Time  `json:"created_at"`
	CancelAuthority   string     `json:"cancel_authority,omitempty"`
	CancelReason      string     `json:"cancel_reason,omitempty"`
	CancelEvidenceRef string     `json:"cancel_evidence_ref,omitempty"`
	CanceledAt        *time.Time `json:"canceled_at,omitempty"`
}

type MemoryCandidateRequest struct {
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
	SourceRef string `json:"source_ref"`
}

type MemoryCandidateListResponse struct {
	Items []MemoryCandidateProjection `json:"items"`
}

type MemoryRecallRequest struct {
	InputItems []InputItem `json:"input_items"`
}

type MemoryRecallResponse struct {
	Items []MemoryRecall `json:"items"`
}

type MemoryApprovalRequest struct {
	ApprovalAuthority   string `json:"approval_authority"`
	ApprovalReason      string `json:"approval_reason"`
	ApprovalEvidenceRef string `json:"approval_evidence_ref"`
}

type MemoryRejectionRequest struct {
	RejectionAuthority   string `json:"rejection_authority"`
	RejectionReason      string `json:"rejection_reason"`
	RejectionEvidenceRef string `json:"rejection_evidence_ref"`
}

type MemorySupersessionRequest struct {
	ReplacementText         string `json:"replacement_text"`
	ReplacementSourceRef    string `json:"replacement_source_ref"`
	SupersessionAuthority   string `json:"supersession_authority"`
	SupersessionReason      string `json:"supersession_reason"`
	SupersessionEvidenceRef string `json:"supersession_evidence_ref"`
}

type MemorySupersessionProjection struct {
	Superseded  MemoryCandidateProjection `json:"superseded"`
	Replacement MemoryCandidateProjection `json:"replacement"`
}

type MemoryCandidateProjection struct {
	CandidateID             string     `json:"candidate_id"`
	SessionID               string     `json:"session_id"`
	Text                    string     `json:"text"`
	SourceRef               string     `json:"source_ref"`
	Status                  string     `json:"status"`
	CreatedAt               time.Time  `json:"created_at"`
	ApprovalAuthority       string     `json:"approval_authority,omitempty"`
	ApprovalReason          string     `json:"approval_reason,omitempty"`
	ApprovalEvidenceRef     string     `json:"approval_evidence_ref,omitempty"`
	ApprovedAt              *time.Time `json:"approved_at,omitempty"`
	RejectionAuthority      string     `json:"rejection_authority,omitempty"`
	RejectionReason         string     `json:"rejection_reason,omitempty"`
	RejectionEvidenceRef    string     `json:"rejection_evidence_ref,omitempty"`
	RejectedAt              *time.Time `json:"rejected_at,omitempty"`
	SupersessionAuthority   string     `json:"supersession_authority,omitempty"`
	SupersessionReason      string     `json:"supersession_reason,omitempty"`
	SupersessionEvidenceRef string     `json:"supersession_evidence_ref,omitempty"`
	ReplacementCandidateID  string     `json:"replacement_candidate_id,omitempty"`
	SupersededAt            *time.Time `json:"superseded_at,omitempty"`
}

type MemoryRecall struct {
	CandidateID string `json:"candidate_id"`
	Text        string `json:"text"`
	Source      string `json:"source"`
}

type Event struct {
	EventID     string      `json:"event_id"`
	SessionID   string      `json:"session_id"`
	TurnID      string      `json:"turn_id"`
	OperationID string      `json:"operation_id,omitempty"`
	JobID       string      `json:"job_id,omitempty"`
	WorkID      string      `json:"work_id,omitempty"`
	CandidateID string      `json:"candidate_id,omitempty"`
	Type        string      `json:"type"`
	CreatedAt   time.Time   `json:"created_at"`
	Data        interface{} `json:"data"`
}

type StoredEvent struct {
	EventID     string    `json:"event_id"`
	SessionID   string    `json:"session_id"`
	TurnID      string    `json:"turn_id"`
	OperationID string    `json:"operation_id,omitempty"`
	JobID       string    `json:"job_id,omitempty"`
	WorkID      string    `json:"work_id,omitempty"`
	CandidateID string    `json:"candidate_id,omitempty"`
	Type        string    `json:"type"`
	CreatedAt   time.Time `json:"created_at"`
	Data        EventData `json:"data"`
}

type EventData struct {
	IdempotencyKey             string                               `json:"idempotency_key,omitempty"`
	InputItems                 []InputItem                          `json:"input_items,omitempty"`
	IngressRisks               []IngressRisk                        `json:"ingress_risks,omitempty"`
	ModelInputKinds            []string                             `json:"model_input_kinds,omitempty"`
	ToolManifest               []ToolSpec                           `json:"tool_manifest,omitempty"`
	SkillCatalog               []SkillCatalogItemProjection         `json:"skill_catalog,omitempty"`
	RuntimeContext             *ContextRuntimeSnapshot              `json:"runtime_context,omitempty"`
	RecalledMemories           []MemoryRecall                       `json:"recalled_memories,omitempty"`
	ToolCall                   *ToolCallProjection                  `json:"tool_call,omitempty"`
	ToolResult                 *ToolResultProjection                `json:"tool_result,omitempty"`
	ModelContextAccounting     *ModelContextAccountingProjection    `json:"model_context_accounting,omitempty"`
	ContextCompaction          *ContextCompactionProjection         `json:"context_compaction,omitempty"`
	Final                      *FinalMessage                        `json:"final,omitempty"`
	TurnError                  *TurnError                           `json:"turn_error,omitempty"`
	Operation                  *OperationProjection                 `json:"operation,omitempty"`
	Job                        *JobProjection                       `json:"job,omitempty"`
	KernelObservationDelivery  *KernelObservationDeliveryProjection `json:"kernel_observation_delivery,omitempty"`
	Work                       *WorkProjection                      `json:"work,omitempty"`
	MemoryCandidate            *MemoryCandidateProjection           `json:"memory_candidate,omitempty"`
	ReplacementMemoryCandidate *MemoryCandidateProjection           `json:"replacement_memory_candidate,omitempty"`
}

type ModelContextAccountingProjection struct {
	RoundIndex                int         `json:"round_index,omitempty"`
	Model                     string      `json:"model,omitempty"`
	ModelInputKinds           []string    `json:"model_input_kinds,omitempty"`
	HistoryTurnIDs            []string    `json:"history_turn_ids,omitempty"`
	CompactedThroughTurnID    string      `json:"compacted_through_turn_id,omitempty"`
	Usage                     *TokenUsage `json:"usage,omitempty"`
	ProcessedInputTokens      int         `json:"processed_input_tokens,omitempty"`
	ProcessedInputTokenSource string      `json:"processed_input_token_source,omitempty"`
	ToolRoundCount            int         `json:"tool_round_count,omitempty"`
	ToolCallCount             int         `json:"tool_call_count,omitempty"`
	ToolResultCount           int         `json:"tool_result_count,omitempty"`
}

type ContextCompactionProjection struct {
	Trigger                  string                           `json:"trigger"`
	Status                   string                           `json:"status,omitempty"`
	Summary                  string                           `json:"summary,omitempty"`
	CompactedThroughTurnID   string                           `json:"compacted_through_turn_id,omitempty"`
	CompactedTurnCount       int                              `json:"compacted_turn_count,omitempty"`
	SourceInputTokens        int                              `json:"source_input_tokens,omitempty"`
	SourceUsage              *TokenUsage                      `json:"source_usage,omitempty"`
	CacheStability           *ContextCacheStabilityProjection `json:"cache_stability,omitempty"`
	FailureReason            string                           `json:"failure_reason,omitempty"`
	PreviousFailureReason    string                           `json:"previous_failure_reason,omitempty"`
	RetryAfterCompletedTurns int                              `json:"retry_after_completed_turns,omitempty"`
	BackoffRemainingTurns    int                              `json:"backoff_remaining_turns,omitempty"`
	Model                    string                           `json:"model,omitempty"`
	Usage                    *TokenUsage                      `json:"usage,omitempty"`
}

type ContextCacheStabilityProjection struct {
	Samples               int    `json:"samples,omitempty"`
	CacheHitTokens        int    `json:"cache_hit_tokens,omitempty"`
	CacheMissTokens       int    `json:"cache_miss_tokens,omitempty"`
	HitRatePermille       int    `json:"hit_rate_per_mille,omitempty"`
	FirstHitRatePermille  int    `json:"first_hit_rate_per_mille,omitempty"`
	LatestHitRatePermille int    `json:"latest_hit_rate_per_mille,omitempty"`
	LatestCacheHitTokens  int    `json:"latest_cache_hit_tokens,omitempty"`
	LatestCacheMissTokens int    `json:"latest_cache_miss_tokens,omitempty"`
	Trend                 string `json:"trend,omitempty"`
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
