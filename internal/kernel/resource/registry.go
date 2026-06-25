package resource

import (
	"errors"
	"fmt"
	"strings"
)

const (
	defaultReadLimitBytes = 4096
	maxReadLimitBytes     = 16 * 1024
)

type Registry struct {
	items  map[string]registeredResource
	redact func(string) string
}

type registeredResource struct {
	ref      string
	mimeType string
	text     string
}

type ReadRequest struct {
	ResourceRef string
	OffsetBytes int
	LimitBytes  int
}

func NewRegistry(items []Descriptor, redactor func(string) string) (*Registry, error) {
	if redactor == nil {
		redactor = func(text string) string { return text }
	}
	registry := &Registry{
		items:  map[string]registeredResource{},
		redact: redactor,
	}
	for _, item := range items {
		ref, err := NormalizeRef(item.Ref)
		if err != nil {
			return nil, err
		}
		if _, exists := registry.items[ref]; exists {
			return nil, fmt.Errorf("duplicate resource ref %q", ref)
		}
		mimeType := normalizedMimeType(item.MimeType)
		if !isTextMimeType(mimeType) {
			return nil, fmt.Errorf("resource %q has unsupported mime type %q", ref, mimeType)
		}
		registry.items[ref] = registeredResource{
			ref:      ref,
			mimeType: mimeType,
			text:     item.Text,
		}
	}
	return registry, nil
}

func (r *Registry) Has(ref string) bool {
	if r == nil {
		return false
	}
	_, ok := r.items[strings.TrimSpace(ref)]
	return ok
}

func (r *Registry) Read(req ReadRequest) (ModelReadResult, error) {
	if r == nil {
		return ModelReadResult{}, errors.New("resource not found")
	}
	item, ok := r.items[strings.TrimSpace(req.ResourceRef)]
	if !ok {
		return ModelReadResult{}, errors.New("resource not found")
	}
	data := []byte(r.redact(item.text))
	offset := req.OffsetBytes
	if offset > len(data) {
		offset = len(data)
	}
	limit := req.LimitBytes
	end := offset + limit
	if end > len(data) {
		end = len(data)
	}
	text := string(data[offset:end])
	result := ModelReadResult{
		Status:        "completed",
		Executed:      true,
		ResourceRef:   item.ref,
		MimeType:      item.mimeType,
		Text:          text,
		OffsetBytes:   offset,
		ReturnedBytes: len([]byte(text)),
		OriginalBytes: len(data),
		Truncated:     end < len(data),
	}
	if result.Truncated {
		next := end
		result.NextOffsetBytes = &next
	}
	return result, nil
}

func NormalizeReadRequest(ref string, offsetBytes *int, limitBytes *int) (ReadRequest, string, error) {
	normalizedRef, err := NormalizeRef(ref)
	if err != nil {
		return ReadRequest{}, "invalid_resource_ref", err
	}
	offset := 0
	if offsetBytes != nil {
		offset = *offsetBytes
	}
	if offset < 0 {
		return ReadRequest{}, "invalid_resource_read_request", errors.New("offset_bytes must be zero or greater")
	}
	limit := defaultReadLimitBytes
	if limitBytes != nil {
		limit = *limitBytes
	}
	if limit <= 0 {
		return ReadRequest{}, "invalid_resource_read_request", errors.New("limit_bytes must be greater than zero")
	}
	if limit > maxReadLimitBytes {
		return ReadRequest{}, "invalid_resource_read_request", fmt.Errorf("limit_bytes must be %d or fewer", maxReadLimitBytes)
	}
	return ReadRequest{
		ResourceRef: normalizedRef,
		OffsetBytes: offset,
		LimitBytes:  limit,
	}, "", nil
}

func NormalizeRef(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", errors.New("resource_ref is required")
	}
	if len(ref) > 128 {
		return "", errors.New("resource_ref must be 128 characters or fewer")
	}
	for _, char := range ref {
		switch {
		case char >= 'a' && char <= 'z':
		case char >= 'A' && char <= 'Z':
		case char >= '0' && char <= '9':
		case char == '-' || char == '_' || char == '.' || char == ':':
		default:
			return "", errors.New("resource_ref may contain only letters, numbers, '.', '_', '-', or ':'")
		}
	}
	return ref, nil
}

func normalizedMimeType(mimeType string) string {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if mimeType == "" {
		return "text/plain"
	}
	return mimeType
}

func isTextMimeType(mimeType string) bool {
	return strings.HasPrefix(normalizedMimeType(mimeType), "text/")
}
