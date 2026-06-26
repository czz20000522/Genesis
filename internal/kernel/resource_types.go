package kernel

import (
	"time"

	"genesis/internal/kernel/resource"
)

type ResourceDescriptor = resource.Descriptor
type ResourceMetadata = resource.Metadata
type ModelResourceReadResult = resource.ModelReadResult
type SourceSnapshotDescriptor = resource.SourceSnapshotDescriptor
type SourceFileDescriptor = resource.SourceFileDescriptor
type SourceDiagnostic = resource.SourceDiagnostic
type SourceTreeResult = resource.SourceTreeResult
type ModelSourceReadResult = resource.ModelSourceReadResult

const (
	SourcePurposeAnalysis        = resource.SourcePurposeAnalysis
	SourceKindZip                = resource.SourceKindZip
	ReferenceOperationSourceTree = resource.ReferenceOperationSourceTree
	ReferenceOperationSourceRead = resource.ReferenceOperationSourceRead
	MaterialLocatorKindLocalPath = "local_path"
)

type MaterialLocator struct {
	Kind string `json:"kind"`
	Path string `json:"path,omitempty"`
}

type MaterialIntakeRequest struct {
	SessionID string          `json:"session_id,omitempty"`
	Purpose   string          `json:"purpose"`
	Locator   MaterialLocator `json:"locator"`
}

type MaterialIntakeProjection struct {
	AdmissionResult     string                   `json:"admission_result"`
	RefusalReasonClass  string                   `json:"refusal_reason_class,omitempty"`
	SourceSnapshotRef   string                   `json:"source_snapshot_ref,omitempty"`
	Root                SourceSnapshotDescriptor `json:"root,omitempty"`
	AvailableOperations []string                 `json:"available_operations,omitempty"`
	Diagnostics         []SourceDiagnostic       `json:"diagnostics,omitempty"`
}

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
	HydrationID        string    `json:"hydration_id,omitempty"`
	SessionID          string    `json:"session_id"`
	TurnID             string    `json:"turn_id,omitempty"`
	AdmissionResult    string    `json:"admission_result"`
	SourceOwner        string    `json:"source_owner,omitempty"`
	ResourceRef        string    `json:"resource_ref,omitempty"`
	ResourceHash       string    `json:"resource_hash,omitempty"`
	MimeType           string    `json:"mime_type,omitempty"`
	OriginalBytes      int       `json:"original_bytes,omitempty"`
	VisibleBytes       int       `json:"visible_bytes,omitempty"`
	Truncated          bool      `json:"truncated,omitempty"`
	InputKind          string    `json:"input_kind,omitempty"`
	Reason             string    `json:"reason,omitempty"`
	RefusalReasonClass string    `json:"refusal_reason_class,omitempty"`
	DerivationRefs     []string  `json:"derivation_refs,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}
