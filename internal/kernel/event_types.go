package kernel

import "time"

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
	ProviderAttempt            *ProviderAttemptProjection           `json:"provider_attempt,omitempty"`
	ModelContextAccounting     *ModelContextAccountingProjection    `json:"model_context_accounting,omitempty"`
	ContextCompaction          *ContextCompactionProjection         `json:"context_compaction,omitempty"`
	Final                      *FinalMessage                        `json:"final,omitempty"`
	TurnPause                  *TurnPauseProjection                 `json:"turn_pause,omitempty"`
	TurnInterruption           *TurnInterruptionProjection          `json:"turn_interruption,omitempty"`
	TurnError                  *TurnError                           `json:"turn_error,omitempty"`
	Operation                  *OperationProjection                 `json:"operation,omitempty"`
	Job                        *JobProjection                       `json:"job,omitempty"`
	KernelObservationDelivery  *KernelObservationDeliveryProjection `json:"kernel_observation_delivery,omitempty"`
	Work                       *WorkProjection                      `json:"work,omitempty"`
	MemoryCandidate            *MemoryCandidateProjection           `json:"memory_candidate,omitempty"`
	ReplacementMemoryCandidate *MemoryCandidateProjection           `json:"replacement_memory_candidate,omitempty"`
}
