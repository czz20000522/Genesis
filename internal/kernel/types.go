package kernel

import "time"

type Config struct {
	LedgerPath string
	Provider   Provider
	Clock      func() time.Time
}

type ReadyResponse struct {
	Status     string `json:"status"`
	Provider   string `json:"provider"`
	LedgerPath string `json:"ledger_path"`
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
	SessionID string            `json:"session_id"`
	Turns     []TurnProjection  `json:"turns"`
	Events    []EventProjection `json:"events"`
}

type TurnProjection struct {
	TurnID       string       `json:"turn_id"`
	Status       string       `json:"status"`
	InputItems   []InputItem  `json:"input_items"`
	FinalMessage FinalMessage `json:"final,omitempty"`
	StartedAt    time.Time    `json:"started_at"`
	CompletedAt  time.Time    `json:"completed_at,omitempty"`
}

type EventProjection struct {
	EventID   string    `json:"event_id"`
	TurnID    string    `json:"turn_id"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
}

type Event struct {
	EventID   string      `json:"event_id"`
	SessionID string      `json:"session_id"`
	TurnID    string      `json:"turn_id"`
	Type      string      `json:"type"`
	CreatedAt time.Time   `json:"created_at"`
	Data      interface{} `json:"data"`
}

type StoredEvent struct {
	EventID   string    `json:"event_id"`
	SessionID string    `json:"session_id"`
	TurnID    string    `json:"turn_id"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	Data      EventData `json:"data"`
}

type EventData struct {
	InputItems []InputItem  `json:"input_items,omitempty"`
	Final      FinalMessage `json:"final,omitempty"`
}
