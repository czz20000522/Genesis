package kernel

import "time"

type EventProjection struct {
	EventID            string    `json:"event_id"`
	TurnID             string    `json:"turn_id"`
	OperationID        string    `json:"operation_id,omitempty"`
	JobID              string    `json:"job_id,omitempty"`
	WorkID             string    `json:"work_id,omitempty"`
	CandidateID        string    `json:"candidate_id,omitempty"`
	ApprovalID         string    `json:"approval_id,omitempty"`
	SandboxReadinessID string    `json:"sandbox_readiness_id,omitempty"`
	Type               string    `json:"type"`
	CreatedAt          time.Time `json:"created_at"`
	Data               EventData `json:"data,omitempty"`
}

type Event struct {
	EventID            string      `json:"event_id"`
	SessionID          string      `json:"session_id"`
	TurnID             string      `json:"turn_id"`
	OperationID        string      `json:"operation_id,omitempty"`
	JobID              string      `json:"job_id,omitempty"`
	WorkID             string      `json:"work_id,omitempty"`
	CandidateID        string      `json:"candidate_id,omitempty"`
	ApprovalID         string      `json:"approval_id,omitempty"`
	SandboxReadinessID string      `json:"sandbox_readiness_id,omitempty"`
	Type               string      `json:"type"`
	CreatedAt          time.Time   `json:"created_at"`
	Data               interface{} `json:"data"`
}

type StoredEvent struct {
	EventID            string    `json:"event_id"`
	SessionID          string    `json:"session_id"`
	TurnID             string    `json:"turn_id"`
	OperationID        string    `json:"operation_id,omitempty"`
	JobID              string    `json:"job_id,omitempty"`
	WorkID             string    `json:"work_id,omitempty"`
	CandidateID        string    `json:"candidate_id,omitempty"`
	ApprovalID         string    `json:"approval_id,omitempty"`
	SandboxReadinessID string    `json:"sandbox_readiness_id,omitempty"`
	Type               string    `json:"type"`
	CreatedAt          time.Time `json:"created_at"`
	Data               EventData `json:"data"`
}

type EventData struct {
	IdempotencyKey             string                               `json:"idempotency_key,omitempty"`
	InputItems                 []InputItem                          `json:"input_items,omitempty"`
	IngressRisks               []IngressRisk                        `json:"ingress_risks,omitempty"`
	ModelInputKinds            []string                             `json:"model_input_kinds,omitempty"`
	ToolManifest               []ToolSpec                           `json:"tool_manifest,omitempty"`
	SkillCatalog               []SkillCatalogItemProjection         `json:"skill_catalog,omitempty"`
	SourceSnapshots            []SourceSnapshotDescriptor           `json:"source_snapshots,omitempty"`
	RuntimeContext             *ContextRuntimeSnapshot              `json:"runtime_context,omitempty"`
	HydratedContexts           []ContextHydrationProjection         `json:"hydrated_contexts,omitempty"`
	ContextHydration           *ContextHydrationProjection          `json:"context_hydration,omitempty"`
	ToolCall                   *ToolCallProjection                  `json:"tool_call,omitempty"`
	ToolResult                 *ToolResultProjection                `json:"tool_result,omitempty"`
	ProviderAttempt            *ProviderAttemptProjection           `json:"provider_attempt,omitempty"`
	ModelContextAccounting     *ModelContextAccountingProjection    `json:"model_context_accounting,omitempty"`
	ContextCompaction          *ContextCompactionProjection         `json:"context_compaction,omitempty"`
	Reasoning                  *ReasoningMessage                    `json:"reasoning,omitempty"`
	Final                      *FinalMessage                        `json:"final,omitempty"`
	TurnPause                  *TurnPauseProjection                 `json:"turn_pause,omitempty"`
	TurnInterruption           *TurnInterruptionProjection          `json:"turn_interruption,omitempty"`
	TurnError                  *TurnError                           `json:"turn_error,omitempty"`
	Operation                  *OperationProjection                 `json:"operation,omitempty"`
	Job                        *JobProjection                       `json:"job,omitempty"`
	Approval                   *ApprovalProjection                  `json:"approval,omitempty"`
	SandboxReadiness           *SandboxReadinessProjection          `json:"sandbox_readiness,omitempty"`
	SessionDebug               *SessionDebugProjection              `json:"session_debug,omitempty"`
	SessionWorkspace           *SessionWorkspaceBinding             `json:"session_workspace,omitempty"`
	KernelObservationDelivery  *KernelObservationDeliveryProjection `json:"kernel_observation_delivery,omitempty"`
	Work                       *WorkProjection                      `json:"work,omitempty"`
	AgentInvocation            *AgentInvocationProjection           `json:"agent_invocation,omitempty"`
	AgentInvocationRun         *AgentInvocationRunProjection        `json:"agent_invocation_run,omitempty"`
	MemoryCandidate            *MemoryCandidateProjection           `json:"memory_candidate,omitempty"`
	ReplacementMemoryCandidate *MemoryCandidateProjection           `json:"replacement_memory_candidate,omitempty"`
	TaskGraph                  *TaskGraphEventProjection            `json:"task_graph,omitempty"`
}
