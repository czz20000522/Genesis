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
	Connector        string                `json:"connector"`
	ExternalEventID  string                `json:"external_event_id"`
	EventType        string                `json:"event_type"`
	ThreadRef        ExternalThreadRef     `json:"thread_ref"`
	SenderRef        ExternalRef           `json:"sender_ref"`
	MessageRef       ExternalRef           `json:"message_ref"`
	Body             string                `json:"body,omitempty"`
	ResourceRefs     []ExternalResourceRef `json:"resource_refs,omitempty"`
	ReceivedAt       time.Time             `json:"received_at,omitempty"`
	SourceValidation string                `json:"source_validation"`
	Metadata         map[string]string     `json:"metadata,omitempty"`
}

type RequestContext struct {
	RequestID            string                `json:"request_id"`
	DedupeKey            string                `json:"dedupe_key"`
	Connector            string                `json:"connector"`
	EventType            string                `json:"event_type"`
	ThreadRef            ExternalThreadRef     `json:"thread_ref"`
	SenderRef            ExternalRef           `json:"sender_ref"`
	MessageRef           ExternalRef           `json:"message_ref"`
	ResourceRefs         []ExternalResourceRef `json:"resource_refs,omitempty"`
	SourceValidation     string                `json:"source_validation"`
	ApplicationSessionID string                `json:"application_session_id"`
	KernelSessionID      string                `json:"kernel_session_id"`
	KernelIdempotencyKey string                `json:"kernel_idempotency_key"`
	Body                 string                `json:"body"`
	ReceivedAt           time.Time             `json:"received_at,omitempty"`
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

type SourceFailureRecord struct {
	RecordID          string    `json:"record_id"`
	Connector         string    `json:"connector"`
	EventSource       string    `json:"event_source"`
	SourceRunRef      string    `json:"source_run_ref,omitempty"`
	SourceAttemptRef  string    `json:"source_attempt_ref,omitempty"`
	Reason            string    `json:"reason"`
	Detail            string    `json:"detail"`
	DiagnosticExcerpt string    `json:"diagnostic_excerpt,omitempty"`
	PayloadHash       string    `json:"payload_hash,omitempty"`
	PayloadSizeBytes  int       `json:"payload_size_bytes,omitempty"`
	DebugRef          string    `json:"debug_ref,omitempty"`
	ResourceRef       string    `json:"resource_ref,omitempty"`
	SourceValidation  string    `json:"source_validation"`
	CreatedAt         time.Time `json:"created_at"`
}

type ProcessResult struct {
	Record          InboundSubmissionRecord `json:"record"`
	Duplicate       bool                    `json:"duplicate"`
	FinalText       string                  `json:"final_text,omitempty"`
	OutboxItem      *ConnectorOutboxItem    `json:"outbox_item,omitempty"`
	OutboxDuplicate bool                    `json:"outbox_duplicate,omitempty"`
	DeliveryReceipt *DeliveryReceipt        `json:"delivery_receipt,omitempty"`
	DeliveryError   string                  `json:"delivery_error,omitempty"`
}
