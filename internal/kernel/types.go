package kernel

import "time"

type Config struct {
	LedgerPath string
	Provider   Provider
	Clock      func() time.Time
}

type ReadyResponse struct {
	Status     string         `json:"status"`
	Provider   ProviderStatus `json:"provider"`
	LedgerPath string         `json:"ledger_path"`
}

type ProviderStatus struct {
	Name   string `json:"name"`
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
	RecalledMemories []MemoryRecall `json:"recalled_memories,omitempty"`
	FinalMessage     FinalMessage   `json:"final,omitempty"`
	StartedAt        time.Time      `json:"started_at"`
	CompletedAt      time.Time      `json:"completed_at,omitempty"`
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
	PermissionMode string `json:"permission_mode"`
	WorkspaceRoot  string `json:"workspace_root,omitempty"`
	CWD            string `json:"cwd"`
	Command        string `json:"command"`
}

type OperationProjection struct {
	OperationID    string    `json:"operation_id"`
	SessionID      string    `json:"session_id"`
	Tool           string    `json:"tool"`
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
}

type MemoryCandidateProjection struct {
	CandidateID string     `json:"candidate_id"`
	SessionID   string     `json:"session_id"`
	Text        string     `json:"text"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	ApprovedAt  *time.Time `json:"approved_at,omitempty"`
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
	RecalledMemories []MemoryRecall             `json:"recalled_memories,omitempty"`
	Final            *FinalMessage              `json:"final,omitempty"`
	Operation        *OperationProjection       `json:"operation,omitempty"`
	MemoryCandidate  *MemoryCandidateProjection `json:"memory_candidate,omitempty"`
}
