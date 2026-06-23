package messageingress

import "time"

const (
	SubmissionStatusPending   = "pending"
	SubmissionStatusSubmitted = "submitted"
	SubmissionStatusFailed    = "failed"
)

type ChannelMessage struct {
	Channel    string            `json:"channel"`
	Adapter    string            `json:"adapter"`
	MessageID  string            `json:"message_id"`
	ThreadID   string            `json:"thread_id"`
	UserID     string            `json:"user_id"`
	Text       string            `json:"text"`
	ReceivedAt time.Time         `json:"received_at,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type TurnInputItem struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type TurnSubmitRequest struct {
	SessionID      string          `json:"session_id,omitempty"`
	IdempotencyKey string          `json:"idempotency_key,omitempty"`
	InputItems     []TurnInputItem `json:"input_items"`
}

type TurnSubmitResponse struct {
	SessionID string           `json:"session_id"`
	TurnID    string           `json:"turn_id"`
	Final     FinalAnswer      `json:"final"`
	Error     *KernelTurnError `json:"error,omitempty"`
}

type FinalAnswer struct {
	Text string `json:"text"`
}

type KernelTurnError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type SubmissionRecord struct {
	RawKey            string    `json:"raw_key"`
	KernelIdempotency string    `json:"kernel_idempotency"`
	Channel           string    `json:"channel"`
	Adapter           string    `json:"adapter"`
	MessageID         string    `json:"message_id"`
	ThreadID          string    `json:"thread_id"`
	UserID            string    `json:"user_id"`
	SessionID         string    `json:"session_id"`
	TurnID            string    `json:"turn_id,omitempty"`
	Status            string    `json:"status"`
	KernelError       string    `json:"kernel_error,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type ProcessResult struct {
	Record    SubmissionRecord `json:"record"`
	Duplicate bool             `json:"duplicate"`
	FinalText string           `json:"final_text,omitempty"`
}
