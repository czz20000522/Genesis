package resource

const (
	ReferenceVisibilityPublic        = "public_reference"
	ReferenceVisibilityRuntimeHandle = "runtime_handle"
	ReferenceVisibilityOwnerInternal = "owner_internal_ref"

	ReferenceKindTextResource = "text_resource"

	ReferenceOperationReadText = "read_text"

	ReferenceOwnerKernelResource = "kernel.resource"
)

type Descriptor struct {
	Ref      string
	MimeType string
	Text     string
}

type ReferenceDescriptor struct {
	Ref                 string            `json:"ref"`
	RefKind             string            `json:"ref_kind"`
	Owner               string            `json:"owner"`
	DisplayLabel        string            `json:"display_label,omitempty"`
	AvailableOperations []string          `json:"available_operations"`
	Scope               string            `json:"scope,omitempty"`
	Provenance          string            `json:"provenance,omitempty"`
	PublicMetadata      map[string]string `json:"public_metadata,omitempty"`
}

type Metadata struct {
	Ref           string
	MimeType      string
	OriginalBytes int
	ResourceHash  string
	TextReadable  bool
}

type ModelReadResult struct {
	Status          string `json:"status"`
	Executed        bool   `json:"executed"`
	ResourceRef     string `json:"resource_ref"`
	MimeType        string `json:"mime_type"`
	Text            string `json:"text"`
	OffsetBytes     int    `json:"offset_bytes"`
	ReturnedBytes   int    `json:"returned_bytes"`
	OriginalBytes   int    `json:"original_bytes"`
	Truncated       bool   `json:"truncated"`
	NextOffsetBytes *int   `json:"next_offset_bytes,omitempty"`
}
