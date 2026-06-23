package connectorruntime

import "time"

const (
	SubmissionStatusPending   = "pending"
	SubmissionStatusSubmitted = "submitted"
	SubmissionStatusFailed    = "failed"

	SourceValidationVerified  = "verified"
	SourceValidationUnchecked = "unchecked"
	SourceValidationRejected  = "rejected"
)

type ExternalRef struct {
	Connector  string            `json:"connector"`
	Kind       string            `json:"kind"`
	ExternalID string            `json:"external_id"`
	Display    string            `json:"display,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type ExternalEvent struct {
	Connector        string            `json:"connector"`
	ExternalEventID  string            `json:"external_event_id"`
	EventType        string            `json:"event_type"`
	ThreadRef        ExternalThreadRef `json:"thread_ref"`
	SenderRef        ExternalRef       `json:"sender_ref"`
	MessageRef       ExternalRef       `json:"message_ref"`
	Body             string            `json:"body,omitempty"`
	ResourceRefs     []ResourceRef     `json:"resource_refs,omitempty"`
	ReceivedAt       time.Time         `json:"received_at,omitempty"`
	SourceValidation string            `json:"source_validation"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

type RequestContext struct {
	RequestID            string            `json:"request_id"`
	DedupeKey            string            `json:"dedupe_key"`
	Connector            string            `json:"connector"`
	EventType            string            `json:"event_type"`
	ThreadRef            ExternalThreadRef `json:"thread_ref"`
	SenderRef            ExternalRef       `json:"sender_ref"`
	MessageRef           ExternalRef       `json:"message_ref"`
	ResourceRefs         []ResourceRef     `json:"resource_refs,omitempty"`
	SourceValidation     string            `json:"source_validation"`
	ApplicationSessionID string            `json:"application_session_id"`
	KernelSessionID      string            `json:"kernel_session_id"`
	KernelIdempotencyKey string            `json:"kernel_idempotency_key"`
	Body                 string            `json:"body"`
	ReceivedAt           time.Time         `json:"received_at,omitempty"`
}

type ApplicationSessionMapping struct {
	ApplicationSessionID string `json:"application_session_id"`
	KernelSessionID      string `json:"kernel_session_id"`
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

type InboundSubmissionRecord struct {
	RequestID            string    `json:"request_id"`
	DedupeKey            string    `json:"dedupe_key"`
	KernelIdempotencyKey string    `json:"kernel_idempotency_key"`
	Connector            string    `json:"connector"`
	EventType            string    `json:"event_type"`
	ApplicationSessionID string    `json:"application_session_id"`
	KernelSessionID      string    `json:"kernel_session_id"`
	TurnID               string    `json:"turn_id,omitempty"`
	Status               string    `json:"status"`
	KernelError          string    `json:"kernel_error,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type ProcessResult struct {
	Record    InboundSubmissionRecord `json:"record"`
	Duplicate bool                    `json:"duplicate"`
	FinalText string                  `json:"final_text,omitempty"`
}
