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
