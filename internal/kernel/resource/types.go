package resource

const (
	ReferenceVisibilityPublic        = "public_reference"
	ReferenceVisibilityRuntimeHandle = "runtime_handle"
	ReferenceVisibilityOwnerInternal = "owner_internal_ref"

	ReferenceKindTextResource   = "text_resource"
	ReferenceKindSourceSnapshot = "source_snapshot"
	ReferenceKindSourceFile     = "source_file"

	ReferenceOperationReadText   = "read_text"
	ReferenceOperationSourceTree = "source_tree"
	ReferenceOperationSourceRead = "source_read"

	ReferenceOwnerKernelResource = "kernel.resource"
	ReferenceOwnerKernelSource   = "kernel.source_snapshot"
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

const (
	SourceKindZip         = "zip"
	SourcePurposeAnalysis = "source_analysis"
)

const (
	DefaultSourceFileCountLimit    = 4096
	DefaultSourcePerFileLimitBytes = 8 * 1024 * 1024
	DefaultSourceTotalLimitBytes   = 64 * 1024 * 1024
	DefaultSourceTreeEntryLimit    = 200
	DefaultSourceTreeMaxEntryLimit = 4096
	DefaultSourceReadLimitBytes    = defaultReadLimitBytes
	DefaultSourceReadMaxLimitBytes = maxReadLimitBytes
)

type SourceSnapshotPolicy struct {
	MaxFileCount                int   `json:"max_file_count"`
	MaxPerFileUncompressedBytes int64 `json:"max_per_file_uncompressed_bytes"`
	MaxTotalUncompressedBytes   int64 `json:"max_total_uncompressed_bytes"`
	DefaultTreeEntries          int   `json:"default_tree_entries"`
	MaxTreeEntries              int   `json:"max_tree_entries"`
	DefaultReadBytes            int   `json:"default_read_bytes"`
	MaxReadBytes                int   `json:"max_read_bytes"`
}

type SourceSnapshotOptions struct {
	Purpose      string
	SessionID    string
	DisplayLabel string
}

type SourceDiagnostic struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

type SourceSnapshotDescriptor struct {
	SourceSnapshotRef      string             `json:"source_snapshot_ref"`
	DisplayLabel           string             `json:"display_label,omitempty"`
	SourceKind             string             `json:"source_kind"`
	Purpose                string             `json:"purpose"`
	EntryCount             int                `json:"entry_count"`
	TotalUncompressedBytes int64              `json:"total_uncompressed_bytes"`
	AvailableOperations    []string           `json:"available_operations"`
	Diagnostics            []SourceDiagnostic `json:"diagnostics,omitempty"`
}

type SourceFileDescriptor struct {
	SourceFileRef       string   `json:"source_file_ref"`
	Path                string   `json:"path"`
	Kind                string   `json:"kind"`
	MimeType            string   `json:"mime_type,omitempty"`
	SizeBytes           int64    `json:"size_bytes,omitempty"`
	TextReadable        bool     `json:"text_readable,omitempty"`
	AvailableOperations []string `json:"available_operations,omitempty"`
}

type SourceTreeRequest struct {
	SourceSnapshotRef string
	MaxEntries        int
}

type SourceTreeResult struct {
	Status            string                 `json:"status"`
	Executed          bool                   `json:"executed"`
	SourceSnapshotRef string                 `json:"source_snapshot_ref"`
	Entries           []SourceFileDescriptor `json:"entries"`
	TotalEntries      int                    `json:"total_entries"`
	Truncated         bool                   `json:"truncated"`
	Diagnostics       []SourceDiagnostic     `json:"diagnostics,omitempty"`
}

type SourceReadRequest struct {
	SourceFileRef string
	OffsetBytes   int
	LimitBytes    int
}

type ModelSourceReadResult struct {
	Status          string `json:"status"`
	Executed        bool   `json:"executed"`
	SourceFileRef   string `json:"source_file_ref"`
	Path            string `json:"path"`
	MimeType        string `json:"mime_type"`
	Text            string `json:"text"`
	OffsetBytes     int    `json:"offset_bytes"`
	ReturnedBytes   int    `json:"returned_bytes"`
	OriginalBytes   int    `json:"original_bytes"`
	Truncated       bool   `json:"truncated"`
	NextOffsetBytes *int   `json:"next_offset_bytes,omitempty"`
}
