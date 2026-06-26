package resource

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const textProbeBytes = 8192

type sourceSnapshot struct {
	ref          string
	localPath    string
	purpose      string
	sessionID    string
	displayLabel string
	policy       SourceSnapshotPolicy
}

type sourceFileHandle struct {
	snapshotRef string
	path        string
}

type sourceEntry struct {
	descriptor SourceFileDescriptor
}

type SourceError struct {
	ReasonClass string
	Message     string
}

func (e SourceError) Error() string {
	if strings.TrimSpace(e.Message) != "" {
		return e.Message
	}
	return e.ReasonClass
}

func SourceErrorReason(err error) string {
	var sourceErr SourceError
	if errors.As(err, &sourceErr) {
		return sourceErr.ReasonClass
	}
	return ""
}

func (r *Registry) RegisterLocalZipSnapshot(localPath string, options SourceSnapshotOptions) (SourceSnapshotDescriptor, error) {
	if r == nil {
		return SourceSnapshotDescriptor{}, sourceError("resource_unavailable", "resource registry is unavailable")
	}
	localPath = strings.TrimSpace(localPath)
	if localPath == "" {
		return SourceSnapshotDescriptor{}, sourceError("invalid_locator", "local path is required")
	}
	if !filepath.IsAbs(localPath) {
		return SourceSnapshotDescriptor{}, sourceError("invalid_locator", "local path must be absolute")
	}
	info, err := os.Stat(localPath)
	if err != nil {
		return SourceSnapshotDescriptor{}, sourceError("resource_unavailable", "local path is unavailable")
	}
	if info.IsDir() {
		return SourceSnapshotDescriptor{}, sourceError("unsupported_source_kind", "local path must be a zip file")
	}
	purpose := strings.TrimSpace(options.Purpose)
	if purpose == "" {
		purpose = SourcePurposeAnalysis
	}
	ref := sourceSnapshotRefFor(localPath, info.Size(), info.ModTime().UnixNano(), purpose)
	policy := r.SourceSnapshotPolicy()
	entries, totalBytes, diagnostics, err := parseZipSourceEntries(localPath, ref, policy)
	if err != nil {
		return SourceSnapshotDescriptor{}, err
	}
	displayLabel := strings.TrimSpace(options.DisplayLabel)
	if displayLabel == "" {
		displayLabel = filepath.Base(localPath)
	}
	snapshot := sourceSnapshot{
		ref:          ref,
		localPath:    localPath,
		purpose:      purpose,
		sessionID:    strings.TrimSpace(options.SessionID),
		displayLabel: displayLabel,
		policy:       policy,
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sources == nil {
		r.sources = map[string]sourceSnapshot{}
	}
	if r.sourceFiles == nil {
		r.sourceFiles = map[string]sourceFileHandle{}
	}
	r.sources[ref] = snapshot
	for _, entry := range entries {
		if entry.descriptor.Kind != "file" {
			continue
		}
		r.sourceFiles[entry.descriptor.SourceFileRef] = sourceFileHandle{
			snapshotRef: ref,
			path:        entry.descriptor.Path,
		}
	}
	return sourceSnapshotDescriptor(snapshot, entries, totalBytes, diagnostics), nil
}

func (r *Registry) ListSourceSnapshotDescriptors(sessionID string) []SourceSnapshotDescriptor {
	if r == nil {
		return nil
	}
	sessionID = strings.TrimSpace(sessionID)
	r.mu.RLock()
	refs := make([]string, 0, len(r.sources))
	for ref, snapshot := range r.sources {
		if snapshot.sessionID != "" && snapshot.sessionID != sessionID {
			continue
		}
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	snapshots := make([]sourceSnapshot, 0, len(refs))
	for _, ref := range refs {
		snapshots = append(snapshots, r.sources[ref])
	}
	r.mu.RUnlock()
	descriptors := make([]SourceSnapshotDescriptor, 0, len(refs))
	for _, snapshot := range snapshots {
		entries, totalBytes, diagnostics, err := parseZipSourceEntries(snapshot.localPath, snapshot.ref, snapshot.policy)
		if err != nil {
			descriptors = append(descriptors, SourceSnapshotDescriptor{
				SourceSnapshotRef:   snapshot.ref,
				DisplayLabel:        snapshot.displayLabel,
				SourceKind:          SourceKindZip,
				Purpose:             snapshot.purpose,
				AvailableOperations: []string{ReferenceOperationSourceTree},
				Diagnostics: []SourceDiagnostic{{
					Code:    firstNonEmpty(SourceErrorReason(err), "source_unavailable"),
					Message: err.Error(),
				}},
			})
			continue
		}
		entries = r.filterAdmittedSourceEntries(snapshot.ref, entries)
		totalBytes = sourceEntriesTotalBytes(entries)
		descriptors = append(descriptors, sourceSnapshotDescriptor(snapshot, entries, totalBytes, diagnostics))
	}
	return descriptors
}

func (r *Registry) AdmitSourceTree(ref string, maxEntries *int) (SourceTreeRequest, SourceSnapshotDescriptor, string, error) {
	if classify := ClassifyReference(ref); classify == ReferenceVisibilityOwnerInternal {
		return SourceTreeRequest{}, SourceSnapshotDescriptor{}, "owner_internal_ref_not_source_snapshot", errors.New("source_snapshot_ref is an owner-internal ref, not a source snapshot")
	} else if classify == ReferenceVisibilityRuntimeHandle {
		return SourceTreeRequest{}, SourceSnapshotDescriptor{}, "runtime_handle_not_source_snapshot", errors.New("source_snapshot_ref is a runtime handle, not a source snapshot")
	}
	normalized, err := NormalizeRef(ref)
	if err != nil {
		return SourceTreeRequest{}, SourceSnapshotDescriptor{}, "invalid_source_snapshot_ref", err
	}
	r.mu.RLock()
	snapshot, ok := r.sources[normalized]
	r.mu.RUnlock()
	if !ok {
		return SourceTreeRequest{}, SourceSnapshotDescriptor{}, "unknown_source_snapshot_ref", fmt.Errorf("unknown source snapshot ref %q", normalized)
	}
	policy := NormalizeSourceSnapshotPolicy(snapshot.policy)
	limit := policy.DefaultTreeEntries
	if maxEntries != nil {
		limit = *maxEntries
	}
	if limit <= 0 {
		return SourceTreeRequest{}, SourceSnapshotDescriptor{}, "invalid_source_tree_request", errors.New("max_entries must be greater than zero")
	}
	if limit > policy.MaxTreeEntries {
		return SourceTreeRequest{}, SourceSnapshotDescriptor{}, "invalid_source_tree_request", fmt.Errorf("max_entries must be %d or fewer", policy.MaxTreeEntries)
	}
	entries, totalBytes, diagnostics, err := parseZipSourceEntries(snapshot.localPath, snapshot.ref, policy)
	if err != nil {
		return SourceTreeRequest{}, SourceSnapshotDescriptor{}, SourceErrorReason(err), err
	}
	entries = r.filterAdmittedSourceEntries(snapshot.ref, entries)
	totalBytes = sourceEntriesTotalBytes(entries)
	descriptor := sourceSnapshotDescriptor(snapshot, entries, totalBytes, diagnostics)
	return SourceTreeRequest{SourceSnapshotRef: normalized, MaxEntries: limit}, descriptor, "", nil
}

func (r *Registry) SourceTree(req SourceTreeRequest) (SourceTreeResult, error) {
	r.mu.RLock()
	snapshot, ok := r.sources[strings.TrimSpace(req.SourceSnapshotRef)]
	r.mu.RUnlock()
	if !ok {
		return SourceTreeResult{}, sourceError("unknown_source_snapshot_ref", "unknown source snapshot ref")
	}
	policy := NormalizeSourceSnapshotPolicy(snapshot.policy)
	entries, _, diagnostics, err := parseZipSourceEntries(snapshot.localPath, snapshot.ref, policy)
	if err != nil {
		return SourceTreeResult{}, err
	}
	entries = r.filterAdmittedSourceEntries(snapshot.ref, entries)
	limit := req.MaxEntries
	if limit <= 0 || limit > policy.MaxTreeEntries {
		limit = policy.DefaultTreeEntries
	}
	descriptors := make([]SourceFileDescriptor, 0, len(entries))
	for _, entry := range entries {
		descriptors = append(descriptors, entry.descriptor)
	}
	truncated := len(descriptors) > limit
	if truncated {
		descriptors = descriptors[:limit]
	}
	return SourceTreeResult{
		Status:            "completed",
		Executed:          true,
		SourceSnapshotRef: snapshot.ref,
		Entries:           descriptors,
		TotalEntries:      len(entries),
		Truncated:         truncated,
		Diagnostics:       diagnostics,
	}, nil
}

func (r *Registry) filterAdmittedSourceEntries(snapshotRef string, entries []sourceEntry) []sourceEntry {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	admittedFiles := map[string]string{}
	for ref, handle := range r.sourceFiles {
		if handle.snapshotRef != snapshotRef {
			continue
		}
		admittedFiles[ref] = handle.path
	}
	r.mu.RUnlock()
	if len(admittedFiles) == 0 {
		return nil
	}
	filtered := make([]sourceEntry, 0, len(entries))
	for _, entry := range entries {
		descriptor := entry.descriptor
		if descriptor.Kind == "file" {
			if _, ok := admittedFiles[descriptor.SourceFileRef]; ok {
				filtered = append(filtered, entry)
			}
			continue
		}
		prefix := strings.TrimSuffix(descriptor.Path, "/") + "/"
		for _, admittedPath := range admittedFiles {
			if strings.HasPrefix(admittedPath, prefix) {
				filtered = append(filtered, entry)
				break
			}
		}
	}
	return filtered
}

func sourceEntriesTotalBytes(entries []sourceEntry) int64 {
	var total int64
	for _, entry := range entries {
		if entry.descriptor.Kind == "file" {
			total += entry.descriptor.SizeBytes
		}
	}
	return total
}

func (r *Registry) AdmitSourceRead(ref string, offsetBytes *int, limitBytes *int) (SourceReadRequest, SourceFileDescriptor, string, error) {
	if classify := ClassifyReference(ref); classify == ReferenceVisibilityOwnerInternal {
		return SourceReadRequest{}, SourceFileDescriptor{}, "owner_internal_ref_not_source_file", errors.New("source_file_ref is an owner-internal ref, not a source file")
	} else if classify == ReferenceVisibilityRuntimeHandle {
		return SourceReadRequest{}, SourceFileDescriptor{}, "runtime_handle_not_source_file", errors.New("source_file_ref is a runtime handle, not a source file")
	}
	normalized, err := NormalizeRef(ref)
	if err != nil {
		return SourceReadRequest{}, SourceFileDescriptor{}, "invalid_source_file_ref", err
	}
	r.mu.RLock()
	handle, ok := r.sourceFiles[normalized]
	if !ok {
		r.mu.RUnlock()
		return SourceReadRequest{}, SourceFileDescriptor{}, "unknown_source_file_ref", fmt.Errorf("unknown source file ref %q", normalized)
	}
	snapshot, ok := r.sources[handle.snapshotRef]
	r.mu.RUnlock()
	if !ok {
		return SourceReadRequest{}, SourceFileDescriptor{}, "unknown_source_snapshot_ref", fmt.Errorf("unknown source snapshot ref %q", handle.snapshotRef)
	}
	policy := NormalizeSourceSnapshotPolicy(snapshot.policy)
	entries, _, _, err := parseZipSourceEntries(snapshot.localPath, snapshot.ref, policy)
	if err != nil {
		return SourceReadRequest{}, SourceFileDescriptor{}, SourceErrorReason(err), err
	}
	var descriptor SourceFileDescriptor
	for _, entry := range entries {
		if entry.descriptor.SourceFileRef == normalized {
			descriptor = entry.descriptor
			break
		}
	}
	if descriptor.SourceFileRef == "" {
		return SourceReadRequest{}, SourceFileDescriptor{}, "resource_unavailable", fmt.Errorf("source file %q is no longer available", normalized)
	}
	if !descriptor.TextReadable {
		return SourceReadRequest{}, SourceFileDescriptor{}, "binary_source_file", fmt.Errorf("source file %q is binary and cannot be read as text", descriptor.Path)
	}
	offset := 0
	if offsetBytes != nil {
		offset = *offsetBytes
	}
	if offset < 0 {
		return SourceReadRequest{}, SourceFileDescriptor{}, "invalid_source_read_request", errors.New("offset_bytes must be zero or greater")
	}
	limit := policy.DefaultReadBytes
	if limitBytes != nil {
		limit = *limitBytes
	}
	if limit <= 0 {
		return SourceReadRequest{}, SourceFileDescriptor{}, "invalid_source_read_request", errors.New("limit_bytes must be greater than zero")
	}
	if limit > policy.MaxReadBytes {
		return SourceReadRequest{}, SourceFileDescriptor{}, "invalid_source_read_request", fmt.Errorf("limit_bytes must be %d or fewer", policy.MaxReadBytes)
	}
	return SourceReadRequest{SourceFileRef: normalized, OffsetBytes: offset, LimitBytes: limit}, descriptor, "", nil
}

func (r *Registry) SourceRead(req SourceReadRequest) (ModelSourceReadResult, error) {
	r.mu.RLock()
	handle, ok := r.sourceFiles[strings.TrimSpace(req.SourceFileRef)]
	if !ok {
		r.mu.RUnlock()
		return ModelSourceReadResult{}, sourceError("unknown_source_file_ref", "unknown source file ref")
	}
	snapshot, ok := r.sources[handle.snapshotRef]
	r.mu.RUnlock()
	if !ok {
		return ModelSourceReadResult{}, sourceError("unknown_source_snapshot_ref", "unknown source snapshot ref")
	}
	text, descriptor, err := readZipSourceText(snapshot.localPath, snapshot.ref, handle.path, snapshot.policy)
	if err != nil {
		return ModelSourceReadResult{}, err
	}
	data := []byte(text)
	offset := req.OffsetBytes
	if offset > len(data) {
		offset = len(data)
	}
	end := offset + req.LimitBytes
	if end > len(data) {
		end = len(data)
	}
	visible := string(data[offset:end])
	result := ModelSourceReadResult{
		Status:        "completed",
		Executed:      true,
		SourceFileRef: req.SourceFileRef,
		Path:          descriptor.Path,
		MimeType:      descriptor.MimeType,
		Text:          visible,
		OffsetBytes:   offset,
		ReturnedBytes: len([]byte(visible)),
		OriginalBytes: len(data),
		Truncated:     end < len(data),
	}
	if result.Truncated {
		next := end
		result.NextOffsetBytes = &next
	}
	return result, nil
}

func parseZipSourceEntries(zipPath string, snapshotRef string, policy SourceSnapshotPolicy) ([]sourceEntry, int64, []SourceDiagnostic, error) {
	policy = NormalizeSourceSnapshotPolicy(policy)
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, nil, sourceError("resource_unavailable", "source zip is unavailable")
		}
		return nil, 0, nil, sourceError("invalid_source_archive", "source zip cannot be opened")
	}
	defer reader.Close()
	seen := map[string]bool{}
	var entries []sourceEntry
	var totalBytes int64
	fileCount := 0
	for _, file := range reader.File {
		normalized, err := normalizeZipEntryName(file.Name)
		if err != nil {
			return nil, 0, nil, err
		}
		if seen[normalized] {
			return nil, 0, nil, sourceError("duplicate_source_path", fmt.Sprintf("duplicate source path %q", normalized))
		}
		seen[normalized] = true
		isDir := file.FileInfo().IsDir() || strings.HasSuffix(file.Name, "/")
		descriptor := SourceFileDescriptor{
			SourceFileRef:       sourceFileRefFor(snapshotRef, normalized),
			Path:                normalized,
			Kind:                "directory",
			AvailableOperations: nil,
		}
		if !isDir {
			fileCount++
			if fileCount > policy.MaxFileCount {
				return nil, 0, nil, sourceError("source_file_count_exceeded", "source archive has too many files")
			}
			size := int64(file.UncompressedSize64)
			if size > policy.MaxPerFileUncompressedBytes {
				return nil, 0, nil, sourceError("source_file_size_exceeded", "source archive contains an oversized file")
			}
			totalBytes += size
			if totalBytes > policy.MaxTotalUncompressedBytes {
				return nil, 0, nil, sourceError("source_total_size_exceeded", "source archive exceeds total uncompressed size budget")
			}
			textReadable, err := zipEntryTextReadable(file)
			if err != nil {
				return nil, 0, nil, err
			}
			descriptor.Kind = "file"
			descriptor.MimeType = sourceMimeType(normalized, textReadable)
			descriptor.SizeBytes = size
			descriptor.TextReadable = textReadable
			if textReadable {
				descriptor.AvailableOperations = []string{ReferenceOperationSourceRead}
			}
		}
		entries = append(entries, sourceEntry{descriptor: descriptor})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].descriptor.Path < entries[j].descriptor.Path
	})
	diagnostics := []SourceDiagnostic{}
	if len(entries) == 0 {
		diagnostics = append(diagnostics, SourceDiagnostic{Code: "empty_archive", Message: "source archive has no entries"})
	}
	return entries, totalBytes, diagnostics, nil
}

func normalizeZipEntryName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", sourceError("invalid_source_path", "zip entry name is empty")
	}
	if strings.Contains(name, "\\") {
		return "", sourceError("invalid_source_path", "zip entry uses backslash path separators")
	}
	if strings.HasPrefix(name, "/") || path.IsAbs(name) || looksLikeWindowsDriveRef(strings.ToLower(name)) {
		return "", sourceError("invalid_source_path", "zip entry path must be relative")
	}
	trimmed := strings.TrimSuffix(name, "/")
	if trimmed == "" {
		return "", sourceError("invalid_source_path", "zip entry name is empty")
	}
	for _, segment := range strings.Split(trimmed, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", sourceError("invalid_source_path", "zip entry path escapes archive root")
		}
	}
	cleaned := path.Clean(trimmed)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", sourceError("invalid_source_path", "zip entry path escapes archive root")
	}
	return cleaned, nil
}

func zipEntryTextReadable(file *zip.File) (bool, error) {
	reader, err := file.Open()
	if err != nil {
		return false, sourceError("invalid_source_archive", "source zip entry cannot be opened")
	}
	defer reader.Close()
	limit := textProbeBytes
	if int64(limit) > int64(file.UncompressedSize64) {
		limit = int(file.UncompressedSize64)
	}
	buf := make([]byte, limit)
	n, err := io.ReadFull(reader, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return false, sourceError("invalid_source_archive", "source zip entry cannot be read")
	}
	buf = buf[:n]
	if bytesContainsNUL(buf) {
		return false, nil
	}
	return utf8.Valid(buf), nil
}

func readZipSourceText(zipPath string, snapshotRef string, sourcePath string, policy SourceSnapshotPolicy) (string, SourceFileDescriptor, error) {
	policy = NormalizeSourceSnapshotPolicy(policy)
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", SourceFileDescriptor{}, sourceError("resource_unavailable", "source zip is unavailable")
		}
		return "", SourceFileDescriptor{}, sourceError("invalid_source_archive", "source zip cannot be opened")
	}
	defer reader.Close()
	for _, file := range reader.File {
		normalized, err := normalizeZipEntryName(file.Name)
		if err != nil {
			return "", SourceFileDescriptor{}, err
		}
		if normalized != sourcePath {
			continue
		}
		if file.FileInfo().IsDir() || strings.HasSuffix(file.Name, "/") {
			return "", SourceFileDescriptor{}, sourceError("source_path_not_file", "source path is a directory")
		}
		if file.UncompressedSize64 > uint64(policy.MaxPerFileUncompressedBytes) {
			return "", SourceFileDescriptor{}, sourceError("source_file_size_exceeded", "source file exceeds read size budget")
		}
		textReadable, err := zipEntryTextReadable(file)
		if err != nil {
			return "", SourceFileDescriptor{}, err
		}
		descriptor := SourceFileDescriptor{
			SourceFileRef:       sourceFileRefFor(snapshotRef, normalized),
			Path:                normalized,
			Kind:                "file",
			MimeType:            sourceMimeType(normalized, textReadable),
			SizeBytes:           int64(file.UncompressedSize64),
			TextReadable:        textReadable,
			AvailableOperations: []string{ReferenceOperationSourceRead},
		}
		if !textReadable {
			return "", descriptor, sourceError("binary_source_file", "source file is binary")
		}
		rc, err := file.Open()
		if err != nil {
			return "", SourceFileDescriptor{}, sourceError("invalid_source_archive", "source zip entry cannot be opened")
		}
		defer rc.Close()
		data, err := io.ReadAll(rc)
		if err != nil {
			return "", SourceFileDescriptor{}, sourceError("invalid_source_archive", "source zip entry cannot be read")
		}
		if !utf8.Valid(data) {
			return "", descriptor, sourceError("binary_source_file", "source file is not valid UTF-8")
		}
		return string(data), descriptor, nil
	}
	return "", SourceFileDescriptor{}, sourceError("resource_unavailable", "source file is unavailable")
}

func sourceSnapshotDescriptor(snapshot sourceSnapshot, entries []sourceEntry, totalBytes int64, diagnostics []SourceDiagnostic) SourceSnapshotDescriptor {
	return SourceSnapshotDescriptor{
		SourceSnapshotRef:      snapshot.ref,
		DisplayLabel:           snapshot.displayLabel,
		SourceKind:             SourceKindZip,
		Purpose:                snapshot.purpose,
		EntryCount:             len(entries),
		TotalUncompressedBytes: totalBytes,
		AvailableOperations:    []string{ReferenceOperationSourceTree},
		Diagnostics:            append([]SourceDiagnostic(nil), diagnostics...),
	}
}

func sourceSnapshotRefFor(localPath string, size int64, modNano int64, purpose string) string {
	return "source_snapshot_" + shortHash(strings.Join([]string{filepath.Clean(localPath), strconv.FormatInt(size, 10), strconv.FormatInt(modNano, 10), purpose}, "\x00"))
}

func sourceFileRefFor(snapshotRef string, sourcePath string) string {
	return "source_file_" + shortHash(strings.Join([]string{strings.TrimSpace(snapshotRef), sourcePath}, "\x00"))
}

func shortHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])[:20]
}

func sourceMimeType(path string, textReadable bool) string {
	if !textReadable {
		return "application/octet-stream"
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "text/x-go"
	case ".md":
		return "text/markdown"
	case ".json":
		return "application/json"
	default:
		return "text/plain"
	}
}

func bytesContainsNUL(data []byte) bool {
	for _, b := range data {
		if b == 0 {
			return true
		}
	}
	return false
}

func sourceError(reason string, message string) SourceError {
	return SourceError{ReasonClass: reason, Message: message}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
