package kernel

import "time"

type TurnRequest struct {
	SessionID      string      `json:"session_id,omitempty"`
	IdempotencyKey string      `json:"idempotency_key,omitempty"`
	InputItems     []InputItem `json:"input_items"`
}

type InputItem struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type TurnResponse struct {
	SessionID string               `json:"session_id"`
	TurnID    string               `json:"turn_id"`
	Events    []Event              `json:"events"`
	Final     FinalMessage         `json:"final"`
	Pause     *TurnPauseProjection `json:"pause,omitempty"`
	Error     *TurnError           `json:"error,omitempty"`
}

type TurnInterruptRequest struct {
	Reason string `json:"reason,omitempty"`
}

type TurnInterruptionProjection struct {
	SessionID       string    `json:"session_id"`
	TurnID          string    `json:"turn_id"`
	Phase           string    `json:"phase"`
	TerminalOutcome string    `json:"terminal_outcome"`
	TerminalCause   string    `json:"terminal_cause,omitempty"`
	Reason          string    `json:"reason,omitempty"`
	InterruptedAt   time.Time `json:"interrupted_at"`
}

type TurnPauseProjection struct {
	SessionID           string                `json:"session_id"`
	TurnID              string                `json:"turn_id"`
	Phase               string                `json:"phase"`
	WaitReason          string                `json:"wait_reason"`
	Reason              string                `json:"reason"`
	RoundBudget         int                   `json:"round_budget"`
	BudgetLease         BudgetLeaseProjection `json:"budget_lease"`
	CompletedToolRounds int                   `json:"completed_tool_rounds"`
	PausedAt            time.Time             `json:"paused_at"`
}

type TurnEventsResponse struct {
	Items []Event `json:"items"`
}

type FinalMessage struct {
	Text  string      `json:"text"`
	Model string      `json:"model"`
	Usage *TokenUsage `json:"usage,omitempty"`
}

type TurnProjection struct {
	TurnID           string                      `json:"turn_id"`
	IdempotencyKey   string                      `json:"idempotency_key,omitempty"`
	Phase            string                      `json:"phase"`
	WaitReason       string                      `json:"wait_reason,omitempty"`
	TerminalOutcome  string                      `json:"terminal_outcome,omitempty"`
	TerminalCause    string                      `json:"terminal_cause,omitempty"`
	InputItems       []InputItem                 `json:"input_items"`
	IngressRisks     []IngressRisk               `json:"ingress_risks,omitempty"`
	ModelInputKinds  []string                    `json:"model_input_kinds,omitempty"`
	RecalledMemories []MemoryRecall              `json:"recalled_memories,omitempty"`
	FinalMessage     FinalMessage                `json:"final,omitempty"`
	Pause            *TurnPauseProjection        `json:"pause,omitempty"`
	Error            *TurnError                  `json:"error,omitempty"`
	Interruption     *TurnInterruptionProjection `json:"interruption,omitempty"`
	StartedAt        time.Time                   `json:"started_at"`
	CompletedAt      time.Time                   `json:"completed_at,omitempty"`
}

type TurnError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
