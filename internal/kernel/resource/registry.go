package resource

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
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
	items        map[string]registeredResource
	sources      map[string]sourceSnapshot
	sourceFiles  map[string]sourceFileHandle
	sourcePolicy SourceSnapshotPolicy
	mu           sync.RWMutex
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
	return NewRegistryWithSourceSnapshotPolicy(items, SourceSnapshotPolicy{})
}

func NewRegistryWithSourceSnapshotPolicy(items []Descriptor, policy SourceSnapshotPolicy) (*Registry, error) {
	registry := &Registry{
		items:        map[string]registeredResource{},
		sources:      map[string]sourceSnapshot{},
		sourceFiles:  map[string]sourceFileHandle{},
		sourcePolicy: NormalizeSourceSnapshotPolicy(policy),
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

func DefaultSourceSnapshotPolicy() SourceSnapshotPolicy {
	return SourceSnapshotPolicy{
		MaxFileCount:                DefaultSourceFileCountLimit,
		MaxPerFileUncompressedBytes: DefaultSourcePerFileLimitBytes,
		MaxTotalUncompressedBytes:   DefaultSourceTotalLimitBytes,
		DefaultTreeEntries:          DefaultSourceTreeEntryLimit,
		MaxTreeEntries:              DefaultSourceTreeMaxEntryLimit,
		DefaultReadBytes:            DefaultSourceReadLimitBytes,
		MaxReadBytes:                DefaultSourceReadMaxLimitBytes,
	}
}

func NormalizeSourceSnapshotPolicy(policy SourceSnapshotPolicy) SourceSnapshotPolicy {
	defaults := DefaultSourceSnapshotPolicy()
	if policy.MaxFileCount <= 0 {
		policy.MaxFileCount = defaults.MaxFileCount
	}
	if policy.MaxPerFileUncompressedBytes <= 0 {
		policy.MaxPerFileUncompressedBytes = defaults.MaxPerFileUncompressedBytes
	}
	if policy.MaxTotalUncompressedBytes <= 0 {
		policy.MaxTotalUncompressedBytes = defaults.MaxTotalUncompressedBytes
	}
	if policy.MaxTreeEntries <= 0 {
		policy.MaxTreeEntries = defaults.MaxTreeEntries
	}
	if policy.DefaultTreeEntries <= 0 {
		policy.DefaultTreeEntries = defaults.DefaultTreeEntries
	}
	if policy.DefaultTreeEntries > policy.MaxTreeEntries {
		policy.DefaultTreeEntries = policy.MaxTreeEntries
	}
	if policy.MaxReadBytes <= 0 {
		policy.MaxReadBytes = defaults.MaxReadBytes
	}
	if policy.DefaultReadBytes <= 0 {
		policy.DefaultReadBytes = defaults.DefaultReadBytes
	}
	if policy.DefaultReadBytes > policy.MaxReadBytes {
		policy.DefaultReadBytes = policy.MaxReadBytes
	}
	return policy
}

func (r *Registry) SourceSnapshotPolicy() SourceSnapshotPolicy {
	if r == nil {
		return DefaultSourceSnapshotPolicy()
	}
	return NormalizeSourceSnapshotPolicy(r.sourcePolicy)
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
	offset, end = utf8SafeByteRange(data, offset, end)
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

func (r *Registry) AdmitReadText(ref string, offsetBytes *int, limitBytes *int) (ReadRequest, ReferenceDescriptor, string, error) {
	req, code, err := NormalizeReadRequest(ref, offsetBytes, limitBytes)
	if err != nil {
		return ReadRequest{}, ReferenceDescriptor{}, code, err
	}
	switch ClassifyReference(req.ResourceRef) {
	case ReferenceVisibilityRuntimeHandle:
		return ReadRequest{}, ReferenceDescriptor{}, "runtime_handle_not_resource", errors.New("resource_ref is a runtime handle, not a readable resource")
	case ReferenceVisibilityOwnerInternal:
		return ReadRequest{}, ReferenceDescriptor{}, "owner_internal_ref_not_resource", errors.New("resource_ref is an owner-internal ref, not a readable resource")
	}
	descriptor, ok := r.DescribeReference(req.ResourceRef)
	if !ok {
		metadata, metadataErr := r.Metadata(req.ResourceRef)
		if metadataErr == nil && !metadata.TextReadable {
			return ReadRequest{}, ReferenceDescriptor{}, "unsupported_mime_type", fmt.Errorf("resource %q has unsupported mime type %q", req.ResourceRef, metadata.MimeType)
		}
		return ReadRequest{}, ReferenceDescriptor{}, "unknown_resource_ref", fmt.Errorf("unknown resource ref %q", req.ResourceRef)
	}
	if !referenceDescriptorHasOperation(descriptor, ReferenceOperationReadText) {
		return ReadRequest{}, ReferenceDescriptor{}, "read_text_unavailable", fmt.Errorf("resource %q does not currently expose read_text", req.ResourceRef)
	}
	return req, descriptor, "", nil
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

func ClassifyReference(ref string) string {
	ref = strings.TrimSpace(ref)
	lower := strings.ToLower(ref)
	switch {
	case looksLikeRuntimeHandle(lower):
		return ReferenceVisibilityRuntimeHandle
	case looksLikeOwnerInternalRef(lower):
		return ReferenceVisibilityOwnerInternal
	default:
		return ReferenceVisibilityPublic
	}
}

func looksLikeRuntimeHandle(ref string) bool {
	for _, prefix := range []string{
		"job_",
		"job:",
		"event:",
		"evt_",
		"tool_call_event:",
		"operation:",
		"op_",
		"work:",
		"work_",
		"request:",
		"req_",
		"checkpoint:",
		"checkpoint_",
		"turn:",
		"turn_",
	} {
		if strings.HasPrefix(ref, prefix) {
			return true
		}
	}
	return false
}

func looksLikeOwnerInternalRef(ref string) bool {
	if looksLikeWindowsDriveRef(ref) || strings.Contains(ref, "skill.md") {
		return true
	}
	for _, prefix := range []string{
		"storage:",
		"storage_ref:",
		"object:",
		"object_key:",
		"db:",
		"database:",
		"row:",
		"provider_payload:",
		"provider_raw:",
		"raw_provider:",
		"raw_payload:",
		"debug_trace:",
		"debug:",
		"connector_payload:",
		"connector_raw:",
		"skill_package:",
		"skill_path:",
		"package_root:",
		"host_path:",
		"path:",
		"file:",
	} {
		if strings.HasPrefix(ref, prefix) {
			return true
		}
	}
	return false
}

func looksLikeWindowsDriveRef(ref string) bool {
	if len(ref) < 2 || ref[1] != ':' {
		return false
	}
	first := ref[0]
	return first >= 'a' && first <= 'z'
}

func referenceDescriptorHasOperation(descriptor ReferenceDescriptor, operation string) bool {
	for _, candidate := range descriptor.AvailableOperations {
		if candidate == operation {
			return true
		}
	}
	return false
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
