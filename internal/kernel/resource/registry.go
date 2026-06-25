package resource

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	defaultReadLimitBytes = 4096
	maxReadLimitBytes     = 16 * 1024
)

const (
	DefaultReadLimitBytes = defaultReadLimitBytes
	MaxReadLimitBytes     = maxReadLimitBytes
)

type Registry struct {
	items map[string]registeredResource
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

func NewRegistry(items []Descriptor) (*Registry, error) {
	registry := &Registry{
		items: map[string]registeredResource{},
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

func (r *Registry) Metadata(ref string) (Metadata, error) {
	if r == nil {
		return Metadata{}, errors.New("resource not found")
	}
	item, ok := r.items[strings.TrimSpace(ref)]
	if !ok {
		return Metadata{}, errors.New("resource not found")
	}
	sum := sha256.Sum256([]byte(item.text))
	return Metadata{
		Ref:           item.ref,
		MimeType:      item.mimeType,
		OriginalBytes: len([]byte(item.text)),
		ResourceHash:  "sha256:" + hex.EncodeToString(sum[:]),
		TextReadable:  isTextMimeType(item.mimeType),
	}, nil
}

func (r *Registry) DescribeReference(ref string) (ReferenceDescriptor, bool) {
	metadata, err := r.Metadata(ref)
	if err != nil || !metadata.TextReadable {
		return ReferenceDescriptor{}, false
	}
	return referenceDescriptorFromMetadata(metadata), true
}

func (r *Registry) ListReferenceDescriptors() []ReferenceDescriptor {
	if r == nil {
		return nil
	}
	refs := make([]string, 0, len(r.items))
	for ref := range r.items {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	descriptors := make([]ReferenceDescriptor, 0, len(refs))
	for _, ref := range refs {
		descriptor, ok := r.DescribeReference(ref)
		if ok {
			descriptors = append(descriptors, descriptor)
		}
	}
	return descriptors
}

func (r *Registry) Read(req ReadRequest) (ModelReadResult, error) {
	if r == nil {
		return ModelReadResult{}, errors.New("resource not found")
	}
	item, ok := r.items[strings.TrimSpace(req.ResourceRef)]
	if !ok {
		return ModelReadResult{}, errors.New("resource not found")
	}
	if !isTextMimeType(item.mimeType) {
		return ModelReadResult{}, fmt.Errorf("unsupported mime type %q", item.mimeType)
	}
	data := []byte(item.text)
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

func referenceDescriptorFromMetadata(metadata Metadata) ReferenceDescriptor {
	return ReferenceDescriptor{
		Ref:                 metadata.Ref,
		RefKind:             ReferenceKindTextResource,
		Owner:               ReferenceOwnerKernelResource,
		DisplayLabel:        metadata.Ref,
		AvailableOperations: []string{ReferenceOperationReadText},
		Scope:               "session",
		Provenance:          "kernel_resource_registry",
		PublicMetadata: map[string]string{
			"mime_type":      metadata.MimeType,
			"original_bytes": strconv.Itoa(metadata.OriginalBytes),
			"resource_hash":  metadata.ResourceHash,
		},
	}
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
