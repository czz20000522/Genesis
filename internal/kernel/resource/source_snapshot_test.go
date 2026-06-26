package resource

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"genesis/internal/testsupport"
)

func TestSourceSnapshotRegistersLocalZipWithoutLeakingHostPath(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "source-snapshot-local")
	zipPath := filepath.Join(dir, "package.zip")
	writeZipFixture(t, zipPath, map[string][]byte{
		"README.md":       []byte("# Package\nhello"),
		"cmd/main.go":     []byte("package main\n"),
		"assets/logo.bin": {0, 1, 2, 3},
	})
	registry, err := NewRegistry(nil)
	if err != nil {
		t.Fatalf("NewRegistry returned error: %v", err)
	}

	descriptor, err := registry.RegisterLocalZipSnapshot(zipPath, SourceSnapshotOptions{
		Purpose:      SourcePurposeAnalysis,
		SessionID:    "session-source",
		DisplayLabel: "package.zip",
	})
	if err != nil {
		t.Fatalf("RegisterLocalZipSnapshot returned error: %v", err)
	}

	if descriptor.SourceSnapshotRef == "" || !strings.HasPrefix(descriptor.SourceSnapshotRef, "source_snapshot_") {
		t.Fatalf("snapshot ref = %q, want system-generated source_snapshot_ ref", descriptor.SourceSnapshotRef)
	}
	if descriptor.SourceKind != SourceKindZip || descriptor.Purpose != SourcePurposeAnalysis {
		t.Fatalf("descriptor source = %+v, want zip/source_analysis", descriptor)
	}
	if !containsString(descriptor.AvailableOperations, ReferenceOperationSourceTree) {
		t.Fatalf("available operations = %v, want source_tree", descriptor.AvailableOperations)
	}
	payload, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatalf("marshal descriptor: %v", err)
	}
	for _, forbidden := range []string{zipPath, dir, "storage_ref", "object_key", "host_path"} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("descriptor leaked %q: %s", forbidden, string(payload))
		}
	}
}

func TestSourceTreeAndReadReturnBoundedArchiveContent(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "source-snapshot-read")
	zipPath := filepath.Join(dir, "package.zip")
	longText := strings.Repeat("abcdef", 1000)
	writeZipFixture(t, zipPath, map[string][]byte{
		"README.md":   []byte("# Package\nhello"),
		"src/app.go":  []byte(longText),
		"image/logo":  {0, 1, 2, 3},
		"docs/guide/": nil,
	})
	registry, err := NewRegistry(nil)
	if err != nil {
		t.Fatalf("NewRegistry returned error: %v", err)
	}
	descriptor, err := registry.RegisterLocalZipSnapshot(zipPath, SourceSnapshotOptions{
		Purpose: SourcePurposeAnalysis,
	})
	if err != nil {
		t.Fatalf("RegisterLocalZipSnapshot returned error: %v", err)
	}

	treeReq, _, code, err := registry.AdmitSourceTree(descriptor.SourceSnapshotRef, nil)
	if err != nil {
		t.Fatalf("AdmitSourceTree returned %s: %v", code, err)
	}
	tree, err := registry.SourceTree(treeReq)
	if err != nil {
		t.Fatalf("SourceTree returned error: %v", err)
	}
	if tree.Status != "completed" || !tree.Executed || tree.SourceSnapshotRef != descriptor.SourceSnapshotRef {
		t.Fatalf("tree result = %+v, want completed snapshot tree", tree)
	}
	readable := sourceFileRefByPath(tree.Entries, "src/app.go")
	if readable == "" {
		t.Fatalf("tree entries = %+v, want src/app.go file ref", tree.Entries)
	}
	binaryRef := sourceFileRefByPath(tree.Entries, "image/logo")
	if binaryRef == "" {
		t.Fatalf("tree entries = %+v, want binary entry file ref", tree.Entries)
	}

	limit := 12
	readReq, _, code, err := registry.AdmitSourceRead(readable, nil, &limit)
	if err != nil {
		t.Fatalf("AdmitSourceRead returned %s: %v", code, err)
	}
	read, err := registry.SourceRead(readReq)
	if err != nil {
		t.Fatalf("SourceRead returned error: %v", err)
	}
	if read.Status != "completed" || !read.Executed || read.Text != longText[:12] || !read.Truncated {
		t.Fatalf("read result = %+v, want bounded text slice", read)
	}
	if read.OriginalBytes != len([]byte(longText)) || read.ReturnedBytes != 12 || read.NextOffsetBytes == nil || *read.NextOffsetBytes != 12 {
		t.Fatalf("read metadata = %+v, want byte budget evidence", read)
	}

	_, _, code, err = registry.AdmitSourceRead(binaryRef, nil, nil)
	if err == nil || code != "binary_source_file" {
		t.Fatalf("binary source read code=%q err=%v, want binary_source_file refusal", code, err)
	}
}

func TestSourceSnapshotRejectsUnsafeZipEntriesAndBudgets(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "source-snapshot-unsafe")
	cases := map[string]map[string][]byte{
		"dotdot":      {"../escape.txt": []byte("escape")},
		"absolute":    {"/escape.txt": []byte("escape")},
		"windows":     {"C:/escape.txt": []byte("escape")},
		"backslash":   {"nested\\escape.txt": []byte("escape")},
		"duplicates":  {"dup.txt": []byte("one"), "./dup.txt": []byte("two")},
		"large-file":  {"large.txt": bytes.Repeat([]byte("x"), DefaultSourcePerFileLimitBytes+1)},
		"many-files":  manyZipEntries(DefaultSourceFileCountLimit + 1),
		"total-bytes": manyZipEntriesWithSize(3, DefaultSourceTotalLimitBytes/3+1),
	}
	for name, entries := range cases {
		t.Run(name, func(t *testing.T) {
			zipPath := filepath.Join(dir, name+".zip")
			writeZipFixture(t, zipPath, entries)
			registry, err := NewRegistry(nil)
			if err != nil {
				t.Fatalf("NewRegistry returned error: %v", err)
			}
			_, err = registry.RegisterLocalZipSnapshot(zipPath, SourceSnapshotOptions{Purpose: SourcePurposeAnalysis})
			if err == nil {
				t.Fatalf("RegisterLocalZipSnapshot accepted unsafe/budget-breaking archive %q", name)
			}
		})
	}
}

func TestSourceSnapshotMissingLocalPathAndHostPathToolRefsFailClosed(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "source-snapshot-missing")
	missing := filepath.Join(dir, "missing.zip")
	registry, err := NewRegistry(nil)
	if err != nil {
		t.Fatalf("NewRegistry returned error: %v", err)
	}
	if _, err := registry.RegisterLocalZipSnapshot(missing, SourceSnapshotOptions{Purpose: SourcePurposeAnalysis}); err == nil {
		t.Fatal("RegisterLocalZipSnapshot accepted missing local path")
	}
	if _, _, code, err := registry.AdmitSourceTree(missing, nil); err == nil || code != "owner_internal_ref_not_source_snapshot" {
		t.Fatalf("AdmitSourceTree(host path) code=%q err=%v, want owner_internal_ref_not_source_snapshot", code, err)
	}
	if _, _, code, err := registry.AdmitSourceRead(missing, nil, nil); err == nil || code != "owner_internal_ref_not_source_file" {
		t.Fatalf("AdmitSourceRead(host path) code=%q err=%v, want owner_internal_ref_not_source_file", code, err)
	}
}

func TestSourceFileRefsAreScopedToSourceSnapshot(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "source-snapshot-file-ref-scope")
	zipPath := filepath.Join(dir, "package.zip")
	writeZipFixture(t, zipPath, map[string][]byte{
		"src/app.go": []byte("package main\n"),
	})
	registry, err := NewRegistry(nil)
	if err != nil {
		t.Fatalf("NewRegistry returned error: %v", err)
	}
	first, err := registry.RegisterLocalZipSnapshot(zipPath, SourceSnapshotOptions{Purpose: "source_analysis_a"})
	if err != nil {
		t.Fatalf("RegisterLocalZipSnapshot(first) returned error: %v", err)
	}
	second, err := registry.RegisterLocalZipSnapshot(zipPath, SourceSnapshotOptions{Purpose: "source_analysis_b"})
	if err != nil {
		t.Fatalf("RegisterLocalZipSnapshot(second) returned error: %v", err)
	}
	firstTree := sourceTreeForSnapshot(t, registry, first.SourceSnapshotRef)
	secondTree := sourceTreeForSnapshot(t, registry, second.SourceSnapshotRef)
	firstRef := sourceFileRefByPath(firstTree.Entries, "src/app.go")
	secondRef := sourceFileRefByPath(secondTree.Entries, "src/app.go")
	if firstRef == "" || secondRef == "" {
		t.Fatalf("missing source file refs: first=%q second=%q", firstRef, secondRef)
	}
	if firstRef == secondRef {
		t.Fatalf("source_file_ref should be scoped to source snapshot, both were %q", firstRef)
	}
}

func writeZipFixture(t *testing.T, path string, entries map[string][]byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create zip dir: %v", err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	defer file.Close()
	writer := zip.NewWriter(file)
	for name, body := range entries {
		if strings.HasSuffix(name, "/") {
			if _, err := writer.Create(name); err != nil {
				t.Fatalf("create zip dir entry %q: %v", name, err)
			}
			continue
		}
		w, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %q: %v", name, err)
		}
		if _, err := w.Write(body); err != nil {
			t.Fatalf("write zip entry %q: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
}

func manyZipEntries(count int) map[string][]byte {
	entries := make(map[string][]byte, count)
	for i := 0; i < count; i++ {
		entries[filepath.ToSlash(filepath.Join("files", "file-"+string(rune('a'+(i%26)))+"-"+string(rune('a'+((i/26)%26)))+".txt"))] = []byte("x")
	}
	return entries
}

func manyZipEntriesWithSize(count int, size int) map[string][]byte {
	entries := make(map[string][]byte, count)
	for i := 0; i < count; i++ {
		entries[filepath.ToSlash(filepath.Join("files", "large-"+string(rune('a'+i))+".txt"))] = bytes.Repeat([]byte("x"), size)
	}
	return entries
}

func sourceFileRefByPath(entries []SourceFileDescriptor, path string) string {
	for _, entry := range entries {
		if entry.Path == path {
			return entry.SourceFileRef
		}
	}
	return ""
}

func sourceTreeForSnapshot(t *testing.T, registry *Registry, snapshotRef string) SourceTreeResult {
	t.Helper()
	req, _, code, err := registry.AdmitSourceTree(snapshotRef, nil)
	if err != nil {
		t.Fatalf("AdmitSourceTree(%q) returned %s: %v", snapshotRef, code, err)
	}
	tree, err := registry.SourceTree(req)
	if err != nil {
		t.Fatalf("SourceTree(%q) returned error: %v", snapshotRef, err)
	}
	return tree
}
