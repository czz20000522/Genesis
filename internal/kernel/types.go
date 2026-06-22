package kernel

import (
	"encoding/json"
	"time"
)

type Config struct {
	LedgerPath   string
	Provider     Provider
	RuntimeToken string
	ToolPolicy   ToolPolicy
	SkillRoots   []string
	Clock        func() time.Time
}

type ToolPolicy struct {
	PermissionMode string
	WorkspaceRoot  string
}

type ReadyResponse struct {
	Status      string         `json:"status"`
	Provider    ProviderStatus `json:"provider"`
	RuntimeAuth ReadyCheck     `json:"runtime_auth"`
	Ledger      ReadyCheck     `json:"ledger"`
	LedgerPath  string         `json:"ledger_path"`
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

type ModelToolDescriptor struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type ModelToolCall struct {
	ToolCallID string          `json:"tool_call_id"`
	Name       string          `json:"name"`
	Arguments  json.RawMessage `json:"arguments,omitempty"`
}

type ModelToolRound struct {
	Calls   []ModelToolCall   `json:"calls"`
	Results []ModelToolResult `json:"results"`
}

type ModelToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Name       string `json:"name"`
	Content    string `json:"content"`
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
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Status string `json:"status"`
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

type SkillReadProjection struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
	Truncated   bool   `json:"truncated,omitempty"`
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

type FinalMessage struct {
	Text  string      `json:"text"`
	Model string      `json:"model"`
	Usage *TokenUsage `json:"usage,omitempty"`
}

type TokenUsage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
	TotalTokens  int `json:"total_tokens,omitempty"`
}

type SessionProjection struct {
	SessionID        string                      `json:"session_id"`
	Turns            []TurnProjection            `json:"turns"`
	Operations       []OperationProjection       `json:"operations"`
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
	WorkID      string    `json:"work_id,omitempty"`
	CandidateID string    `json:"candidate_id,omitempty"`
	Type        string    `json:"type"`
	CreatedAt   time.Time `json:"created_at"`
}

type ShellExecRequest struct {
	SessionID      string `json:"session_id"`
	CWD            string `json:"cwd"`
	Command        string `json:"command"`
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
	CWD                  string    `json:"cwd"`
	Command              string    `json:"command"`
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

type ModelOperationResult struct {
	Tool                 string `json:"tool"`
	Status               string `json:"status"`
	PermissionMode       string `json:"permission_mode"`
	CWD                  string `json:"cwd"`
	Command              string `json:"command"`
	ExitCode             *int   `json:"exit_code,omitempty"`
	Stdout               string `json:"stdout,omitempty"`
	Stderr               string `json:"stderr,omitempty"`
	StdoutTruncated      bool   `json:"stdout_truncated,omitempty"`
	StderrTruncated      bool   `json:"stderr_truncated,omitempty"`
	StdoutOriginalBytes  int    `json:"stdout_original_bytes,omitempty"`
	StderrOriginalBytes  int    `json:"stderr_original_bytes,omitempty"`
	StdoutOmittedBytes   int    `json:"stdout_omitted_bytes,omitempty"`
	StderrOmittedBytes   int    `json:"stderr_omitted_bytes,omitempty"`
	OutputTruncation     string `json:"output_truncation,omitempty"`
	BlockedReason        string `json:"blocked_reason,omitempty"`
	InfrastructureReason string `json:"infrastructure_reason,omitempty"`
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
	WorkID      string    `json:"work_id,omitempty"`
	CandidateID string    `json:"candidate_id,omitempty"`
	Type        string    `json:"type"`
	CreatedAt   time.Time `json:"created_at"`
	Data        EventData `json:"data"`
}

type EventData struct {
	IdempotencyKey             string                     `json:"idempotency_key,omitempty"`
	InputItems                 []InputItem                `json:"input_items,omitempty"`
	IngressRisks               []IngressRisk              `json:"ingress_risks,omitempty"`
	RecalledMemories           []MemoryRecall             `json:"recalled_memories,omitempty"`
	ModelToolCalls             []ModelToolCallRecord      `json:"model_tool_calls,omitempty"`
	Final                      *FinalMessage              `json:"final,omitempty"`
	TurnError                  *TurnError                 `json:"turn_error,omitempty"`
	Operation                  *OperationProjection       `json:"operation,omitempty"`
	Work                       *WorkProjection            `json:"work,omitempty"`
	MemoryCandidate            *MemoryCandidateProjection `json:"memory_candidate,omitempty"`
	ReplacementMemoryCandidate *MemoryCandidateProjection `json:"replacement_memory_candidate,omitempty"`
}

type ModelToolCallRecord struct {
	ToolCallID string `json:"tool_call_id"`
	Tool       string `json:"tool"`
}
