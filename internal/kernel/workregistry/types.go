package workregistry

import "time"

const (
	StatusOpen     = "open"
	StatusCanceled = "canceled"
)

type SubmitRequest struct {
	SessionID      string `json:"session_id"`
	Title          string `json:"title"`
	SourceRef      string `json:"source_ref"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

type CancelRequest struct {
	CancelAuthority   string `json:"cancel_authority"`
	CancelReason      string `json:"cancel_reason"`
	CancelEvidenceRef string `json:"cancel_evidence_ref"`
}

type WorkProjection struct {
	WorkID            string     `json:"work_id"`
	SessionID         string     `json:"session_id"`
	Title             string     `json:"title"`
	SourceRef         string     `json:"source_ref"`
	IdempotencyKey    string     `json:"idempotency_key,omitempty"`
	Status            string     `json:"status"`
	CreatedAt         time.Time  `json:"created_at"`
	CancelAuthority   string     `json:"cancel_authority,omitempty"`
	CancelReason      string     `json:"cancel_reason,omitempty"`
	CancelEvidenceRef string     `json:"cancel_evidence_ref,omitempty"`
	CanceledAt        *time.Time `json:"canceled_at,omitempty"`
}
