package kernel

type ResourceDescriptor struct {
	Ref      string
	MimeType string
	Text     string
}

type ModelResourceReadResult struct {
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
