package kernel

import (
	"errors"
	"fmt"
	"strings"
)

const (
	defaultResourceReadLimitBytes = 4096
	maxResourceReadLimitBytes     = 16 * 1024
)

type resourceRegistry struct {
	items map[string]registeredResource
}

type registeredResource struct {
	ref      string
	mimeType string
	text     string
}

type resourceReadRequest struct {
	resourceRef string
	offsetBytes int
	limitBytes  int
}

func newResourceRegistry(items []ResourceDescriptor) (*resourceRegistry, error) {
	registry := &resourceRegistry{items: map[string]registeredResource{}}
	for _, item := range items {
		ref, err := normalizeResourceRef(item.Ref)
		if err != nil {
			return nil, err
		}
		if _, exists := registry.items[ref]; exists {
			return nil, fmt.Errorf("duplicate resource ref %q", ref)
		}
		mimeType := normalizedResourceMimeType(item.MimeType)
		if !isTextResourceMimeType(mimeType) {
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

func (r *resourceRegistry) lookup(ref string) (registeredResource, bool) {
	if r == nil {
		return registeredResource{}, false
	}
	item, ok := r.items[strings.TrimSpace(ref)]
	return item, ok
}

func (r *resourceRegistry) read(req resourceReadRequest) (ModelResourceReadResult, error) {
	item, ok := r.lookup(req.resourceRef)
	if !ok {
		return ModelResourceReadResult{}, errors.New("resource not found")
	}
	data := []byte(redactEvidenceText(item.text))
	offset := req.offsetBytes
	if offset > len(data) {
		offset = len(data)
	}
	limit := req.limitBytes
	end := offset + limit
	if end > len(data) {
		end = len(data)
	}
	text := string(data[offset:end])
	result := ModelResourceReadResult{
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

func normalizeResourceReadRequest(ref string, offsetBytes *int, limitBytes *int) (resourceReadRequest, string, error) {
	normalizedRef, err := normalizeResourceRef(ref)
	if err != nil {
		return resourceReadRequest{}, "invalid_resource_ref", err
	}
	offset := 0
	if offsetBytes != nil {
		offset = *offsetBytes
	}
	if offset < 0 {
		return resourceReadRequest{}, "invalid_resource_read_request", errors.New("offset_bytes must be zero or greater")
	}
	limit := defaultResourceReadLimitBytes
	if limitBytes != nil {
		limit = *limitBytes
	}
	if limit <= 0 {
		return resourceReadRequest{}, "invalid_resource_read_request", errors.New("limit_bytes must be greater than zero")
	}
	if limit > maxResourceReadLimitBytes {
		return resourceReadRequest{}, "invalid_resource_read_request", fmt.Errorf("limit_bytes must be %d or fewer", maxResourceReadLimitBytes)
	}
	return resourceReadRequest{
		resourceRef: normalizedRef,
		offsetBytes: offset,
		limitBytes:  limit,
	}, "", nil
}

func normalizeResourceRef(ref string) (string, error) {
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

func normalizedResourceMimeType(mimeType string) string {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if mimeType == "" {
		return "text/plain"
	}
	return mimeType
}

func isTextResourceMimeType(mimeType string) bool {
	return strings.HasPrefix(normalizedResourceMimeType(mimeType), "text/")
}
