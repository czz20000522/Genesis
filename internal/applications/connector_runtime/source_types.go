package connectorruntime

import "time"

const (
	SourceRunStatusStarting = "starting"
	SourceRunStatusReady    = "ready"
	SourceRunStatusDegraded = "degraded"
	SourceRunStatusBlocked  = "blocked"
	SourceRunStatusStopped  = "stopped"

	SourceAttemptOutcomeReady   = "ready"
	SourceAttemptOutcomeFailed  = "failed"
	SourceAttemptOutcomeBlocked = "blocked"
	SourceAttemptOutcomeStopped = "stopped"

	SourceCursorKindExternalEventID = "external_event_id"

	SourceReadinessReasonMissingProfile         = "missing_profile"
	SourceReadinessReasonProfileExpired         = "profile_expired"
	SourceReadinessReasonPermissionDenied       = "permission_denied"
	SourceReadinessReasonRefreshRequired        = "refresh_required"
	SourceReadinessReasonOperatorActionRequired = "operator_action_required"
	SourceReadinessReasonSourceCommandInvalid   = "source_command_invalid"
	SourceReadinessReasonSourceRuntimeFailed    = "source_runtime_failed"

	SourceEvidenceKindWebhookSignature               = "webhook_signature"
	SourceEvidenceKindProviderEventSignature         = "provider_event_signature"
	SourceEvidenceKindTrustedLocalAdapterAttestation = "trusted_local_adapter_attestation"

	SourceOperatorActionClearBlocked   = "clear_blocked"
	SourceOperatorActionRequestRestart = "request_restart"
	SourceOperatorActionResetCursor    = "reset_cursor"
)

type SourceRun struct {
	SourceID          string    `json:"source_id"`
	Connector         string    `json:"connector"`
	AdapterRef        string    `json:"adapter_ref"`
	Status            string    `json:"status"`
	StartedAt         time.Time `json:"started_at"`
	StoppedAt         time.Time `json:"stopped_at,omitempty"`
	LastReadyAt       time.Time `json:"last_ready_at,omitempty"`
	BlockedReasonCode string    `json:"blocked_reason_code,omitempty"`
	BlockedReason     string    `json:"blocked_reason,omitempty"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type SourceAttempt struct {
	AttemptID   string    `json:"attempt_id"`
	SourceRunID string    `json:"source_run_id"`
	StartedAt   time.Time `json:"started_at"`
	EndedAt     time.Time `json:"ended_at,omitempty"`
	Outcome     string    `json:"outcome"`
	FailureRef  string    `json:"failure_ref,omitempty"`
}

type SourceCursor struct {
	SourceID    string    `json:"source_id"`
	CursorKind  string    `json:"cursor_kind"`
	CursorValue string    `json:"cursor_value"`
	WatermarkAt time.Time `json:"watermark_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type SourceVerificationEvidence struct {
	SourceEventRef   string    `json:"source_event_ref"`
	SourceBatchRef   string    `json:"source_batch_ref,omitempty"`
	SourceID         string    `json:"source_id"`
	Connector        string    `json:"connector"`
	ValidationStatus string    `json:"validation_status"`
	EvidenceKind     string    `json:"evidence_kind,omitempty"`
	EvidenceRef      string    `json:"evidence_ref,omitempty"`
	CheckedAt        time.Time `json:"checked_at"`
	AdapterRef       string    `json:"adapter_ref,omitempty"`
}

type SourceOperatorActionRecord struct {
	ActionID              string    `json:"action_id"`
	SourceID              string    `json:"source_id"`
	Action                string    `json:"action"`
	Reason                string    `json:"reason,omitempty"`
	PreviousStatus        string    `json:"previous_status,omitempty"`
	NewStatus             string    `json:"new_status,omitempty"`
	CursorKind            string    `json:"cursor_kind,omitempty"`
	CursorValue           string    `json:"cursor_value,omitempty"`
	AcceptedDuplicateRisk bool      `json:"accepted_duplicate_risk,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
}
