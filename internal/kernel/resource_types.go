package kernel

import (
	"time"

	"genesis/internal/kernel/resource"
)

type ResourceDescriptor = resource.Descriptor
type ResourceMetadata = resource.Metadata
type ModelResourceReadResult = resource.ModelReadResult

type ContextHydrationAdmissionRequest struct {
	SessionID       string   `json:"session_id"`
	TurnID          string   `json:"turn_id,omitempty"`
	SourceOwner     string   `json:"source_owner"`
	ResourceRef     string   `json:"resource_ref"`
	MaxVisibleBytes int      `json:"max_visible_bytes,omitempty"`
	DerivationRefs  []string `json:"derivation_refs,omitempty"`
	Reason          string   `json:"reason,omitempty"`
}

type ContextHydrationProjection struct {
	HydrationID    string    `json:"hydration_id,omitempty"`
	SessionID      string    `json:"session_id"`
	TurnID         string    `json:"turn_id,omitempty"`
	Status         string    `json:"status"`
	SourceOwner    string    `json:"source_owner,omitempty"`
	ResourceRef    string    `json:"resource_ref,omitempty"`
	ResourceHash   string    `json:"resource_hash,omitempty"`
	MimeType       string    `json:"mime_type,omitempty"`
	OriginalBytes  int       `json:"original_bytes,omitempty"`
	VisibleBytes   int       `json:"visible_bytes,omitempty"`
	Truncated      bool      `json:"truncated,omitempty"`
	InputKind      string    `json:"input_kind,omitempty"`
	Reason         string    `json:"reason,omitempty"`
	RejectedReason string    `json:"rejected_reason,omitempty"`
	DerivationRefs []string  `json:"derivation_refs,omitempty"`
	VisibleText    string    `json:"visible_text,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}
