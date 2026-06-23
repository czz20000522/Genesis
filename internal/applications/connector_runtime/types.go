package connectorruntime

import "time"

const (
	OutboxStatusQueued           = "queued"
	OutboxStatusSent             = "sent"
	OutboxStatusRetrying         = "retrying"
	OutboxStatusDeadLetter       = "dead_lettered"
	OutboxStatusRecoveryRequired = "recovery_required"

	DeliveryStatusSent                = "sent"
	DeliveryStatusFailed              = "failed"
	DeliveryStatusRetrying            = "retrying"
	DeliveryStatusDuplicateSuppressed = "duplicate_suppressed"
	DeliveryStatusDeadLettered        = "dead_lettered"
	DeliveryStatusPartialSuccess      = "partial_success"
	DeliveryStatusAmbiguous           = "ambiguous"
)

type ExternalThreadRef struct {
	Connector  string            `json:"connector"`
	Kind       string            `json:"kind"`
	ExternalID string            `json:"external_id"`
	Display    string            `json:"display,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type ResourceRef struct {
	Connector  string `json:"connector"`
	ExternalID string `json:"external_id"`
	Kind       string `json:"kind"`
}

type AppCommand struct {
	CommandID            string            `json:"command_id"`
	Kind                 string            `json:"kind"`
	TargetRef            ExternalThreadRef `json:"target_ref"`
	Body                 string            `json:"body,omitempty"`
	ResourceRefs         []ResourceRef     `json:"resource_refs,omitempty"`
	RequiresConfirmation bool              `json:"requires_confirmation,omitempty"`
	DedupeKey            string            `json:"dedupe_key"`
	Metadata             map[string]string `json:"metadata,omitempty"`
	CreatedAt            time.Time         `json:"created_at,omitempty"`
}

type ConnectorOutboxItem struct {
	OutboxID       string            `json:"outbox_id"`
	CommandID      string            `json:"command_id"`
	Connector      string            `json:"connector"`
	ActionKind     string            `json:"action_kind"`
	TargetRef      ExternalThreadRef `json:"target_ref"`
	Payload        map[string]string `json:"payload,omitempty"`
	Status         string            `json:"status"`
	AttemptCount   int               `json:"attempt_count"`
	NextAttemptAt  time.Time         `json:"next_attempt_at,omitempty"`
	LeaseID        string            `json:"lease_id,omitempty"`
	LeaseOwner     string            `json:"lease_owner,omitempty"`
	LeaseExpiresAt time.Time         `json:"lease_expires_at,omitempty"`
	IdempotencyKey string            `json:"idempotency_key"`
	LastReceiptID  string            `json:"last_receipt_id,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

type ConnectorAction struct {
	OutboxID       string            `json:"outbox_id"`
	Connector      string            `json:"connector"`
	ActionKind     string            `json:"action_kind"`
	TargetRef      ExternalThreadRef `json:"target_ref"`
	Payload        map[string]string `json:"payload,omitempty"`
	IdempotencyKey string            `json:"idempotency_key"`
	Attempt        int               `json:"attempt"`
}

type ConnectorActionResult struct {
	ExternalActionRef string    `json:"external_action_ref,omitempty"`
	Status            string    `json:"status"`
	Reason            string    `json:"reason,omitempty"`
	NextAttemptAt     time.Time `json:"next_attempt_at,omitempty"`
}

type DeliveryReceipt struct {
	ReceiptID         string    `json:"receipt_id"`
	OutboxID          string    `json:"outbox_id"`
	Connector         string    `json:"connector"`
	ExternalActionRef string    `json:"external_action_ref,omitempty"`
	Status            string    `json:"status"`
	Reason            string    `json:"reason,omitempty"`
	Attempt           int       `json:"attempt"`
	NextAttemptAt     time.Time `json:"next_attempt_at,omitempty"`
	RecordedAt        time.Time `json:"recorded_at"`
}
