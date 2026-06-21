package kernel

import "time"

type Config struct {
	LedgerPath   string
	Provider     Provider
	RuntimeToken string
	ToolPolicy   ToolPolicy
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
	SessionID  string      `json:"session_id,omitempty"`
	InputItems []InputItem `json:"input_items"`
}

type InputItem struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type TurnResponse struct {
	SessionID string       `json:"session_id"`
	TurnID    string       `json:"turn_id"`
	Events    []Event      `json:"events"`
	Final     FinalMessage `json:"final"`
}

type TurnEventsResponse struct {
	Items []Event `json:"items"`
}

type FinalMessage struct {
	Text  string `json:"text"`
	Model string `json:"model"`
}

type SessionProjection struct {
	SessionID        string                      `json:"session_id"`
	Turns            []TurnProjection            `json:"turns"`
	Operations       []OperationProjection       `json:"operations"`
	MemoryCandidates []MemoryCandidateProjection `json:"memory_candidates"`
	Events           []EventProjection           `json:"events"`
}

type TurnProjection struct {
	TurnID           string         `json:"turn_id"`
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
	OperationID    string    `json:"operation_id"`
	SessionID      string    `json:"session_id"`
	Tool           string    `json:"tool"`
	IdempotencyKey string    `json:"idempotency_key,omitempty"`
	Status         string    `json:"status"`
	PermissionMode string    `json:"permission_mode"`
	CWD            string    `json:"cwd"`
	Command        string    `json:"command"`
	ExitCode       *int      `json:"exit_code,omitempty"`
	Stdout         string    `json:"stdout,omitempty"`
	Stderr         string    `json:"stderr,omitempty"`
	BlockedReason  string    `json:"blocked_reason,omitempty"`
	StartedAt      time.Time `json:"started_at"`
	EndedAt        time.Time `json:"ended_at"`
}

type MemoryCandidateRequest struct {
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
	SourceRef string `json:"source_ref"`
}

type MemoryCandidateListResponse struct {
	Items []MemoryCandidateProjection `json:"items"`
}

type MemoryApprovalRequest struct {
	ApprovalAuthority   string `json:"approval_authority"`
	ApprovalReason      string `json:"approval_reason"`
	ApprovalEvidenceRef string `json:"approval_evidence_ref"`
}

type MemoryCandidateProjection struct {
	CandidateID         string     `json:"candidate_id"`
	SessionID           string     `json:"session_id"`
	Text                string     `json:"text"`
	SourceRef           string     `json:"source_ref"`
	Status              string     `json:"status"`
	CreatedAt           time.Time  `json:"created_at"`
	ApprovalAuthority   string     `json:"approval_authority,omitempty"`
	ApprovalReason      string     `json:"approval_reason,omitempty"`
	ApprovalEvidenceRef string     `json:"approval_evidence_ref,omitempty"`
	ApprovedAt          *time.Time `json:"approved_at,omitempty"`
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
	CandidateID string    `json:"candidate_id,omitempty"`
	Type        string    `json:"type"`
	CreatedAt   time.Time `json:"created_at"`
	Data        EventData `json:"data"`
}

type EventData struct {
	InputItems       []InputItem                `json:"input_items,omitempty"`
	IngressRisks     []IngressRisk              `json:"ingress_risks,omitempty"`
	RecalledMemories []MemoryRecall             `json:"recalled_memories,omitempty"`
	Final            *FinalMessage              `json:"final,omitempty"`
	TurnError        *TurnError                 `json:"turn_error,omitempty"`
	Operation        *OperationProjection       `json:"operation,omitempty"`
	MemoryCandidate  *MemoryCandidateProjection `json:"memory_candidate,omitempty"`
}
