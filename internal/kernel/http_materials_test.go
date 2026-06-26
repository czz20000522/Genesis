package kernel

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"genesis/internal/testsupport"
)

func TestHTTPMaterialIntakeLocalPathReturnsSourceSnapshot(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "http-material-local")
	zipPath := filepath.Join(dir, "package.zip")
	writeKernelZipFixture(t, zipPath, map[string]string{"README.md": "hello"})
	provider := &recordingTextProvider{text: "done"}
	k, err := New(Config{
		LedgerPath:   filepath.Join(dir, "events.jsonl"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	body := []byte(`{"session_id":"http-material-session","purpose":"source_analysis","locator":{"kind":"local_path","path":` + strconvQuote(zipPath) + `}}`)
	resp, err := postJSONWithAuth(server.URL+"/materials/intake", body)
	if err != nil {
		t.Fatalf("POST /materials/intake failed: %v", err)
	}
	defer resp.Body.Close()
	payload := readAll(t, resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /materials/intake status=%d body=%s", resp.StatusCode, payload)
	}
	var projection MaterialIntakeProjection
	if err := json.Unmarshal(payload, &projection); err != nil {
		t.Fatalf("unmarshal material projection: %v\n%s", err, payload)
	}
	if projection.AdmissionResult != "admitted" || projection.SourceSnapshotRef == "" {
		t.Fatalf("projection = %+v, want admitted source snapshot", projection)
	}
	for _, forbidden := range []string{zipPath, dir, "host_path", "storage_ref"} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("material intake response leaked %q: %s", forbidden, payload)
		}
	}

	turnResp, err := postJSONWithAuth(server.URL+"/turn", []byte(`{"session_id":"http-material-session","input_items":[{"type":"text","text":"inspect package"}]}`))
	if err != nil {
		t.Fatalf("POST /turn failed: %v", err)
	}
	defer turnResp.Body.Close()
	if turnResp.StatusCode != http.StatusOK {
		t.Fatalf("POST /turn status=%d body=%s", turnResp.StatusCode, readAll(t, turnResp.Body))
	}
	sourceContext, ok := modelInputTextByKind(provider.Requests()[0].InputItems, ModelInputKindSourceSnapshotContext)
	if !ok || !strings.Contains(sourceContext, projection.SourceSnapshotRef) {
		t.Fatalf("source context = %q ok=%v, want snapshot ref", sourceContext, ok)
	}
	if strings.Contains(sourceContext, zipPath) || strings.Contains(sourceContext, dir) {
		t.Fatalf("source context leaked host path: %q", sourceContext)
	}
}

func TestHTTPMaterialUploadStoresByGeneratedPathAndParsesZip(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "http-material-upload")
	k := newTestKernel(t, filepath.Join(dir, "events.jsonl"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	zipBody := zipBytesFixture(t, map[string]string{"src/main.go": "package main\n"})
	resp := postMultipartMaterialUpload(t, server.URL+"/materials/upload", map[string]string{
		"session_id": "upload-session",
		"purpose":    SourcePurposeAnalysis,
	}, "../../evil.zip", zipBody)
	defer resp.Body.Close()
	payload := readAll(t, resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /materials/upload status=%d body=%s", resp.StatusCode, payload)
	}
	var projection MaterialIntakeProjection
	if err := json.Unmarshal(payload, &projection); err != nil {
		t.Fatalf("unmarshal material projection: %v\n%s", err, payload)
	}
	if projection.AdmissionResult != "admitted" || projection.SourceSnapshotRef == "" {
		t.Fatalf("projection = %+v, want uploaded source snapshot", projection)
	}
	for _, forbidden := range []string{"..", "material-store", dir, "storage_ref", "object_key"} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("upload response leaked storage/path marker %q: %s", forbidden, payload)
		}
	}

	treeReq, _, code, err := k.resourceRegistry.AdmitSourceTree(projection.SourceSnapshotRef, nil)
	if err != nil {
		t.Fatalf("AdmitSourceTree uploaded snapshot returned %s: %v", code, err)
	}
	tree, err := k.resourceRegistry.SourceTree(treeReq)
	if err != nil {
		t.Fatalf("SourceTree uploaded snapshot returned error: %v", err)
	}
	if sourceFileRefByPathForKernel(tree.Entries, "src/main.go") == "" {
		t.Fatalf("uploaded tree entries = %+v, want src/main.go", tree.Entries)
	}
}

func TestHTTPMaterialUploadRejectsNonZipBinary(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "http-material-upload-binary")
	k := newTestKernel(t, filepath.Join(dir, "events.jsonl"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	resp := postMultipartMaterialUpload(t, server.URL+"/materials/upload", map[string]string{
		"session_id": "upload-session",
		"purpose":    SourcePurposeAnalysis,
	}, "payload.bin", []byte{0, 1, 2, 3})
	defer resp.Body.Close()
	body := readAll(t, resp.Body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("POST /materials/upload status=%d body=%s, want 400", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "invalid_source_archive") {
		t.Fatalf("upload binary response = %s, want invalid_source_archive", body)
	}
}

func postMultipartMaterialUpload(t *testing.T, url string, fields map[string]string, filename string, data []byte) *http.Response {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("write multipart field: %v", err)
		}
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, &body)
	if err != nil {
		t.Fatalf("new multipart request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+testRuntimeToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do multipart request: %v", err)
	}
	return resp
}

func zipBytesFixture(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for name, body := range entries {
		w, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %q: %v", name, err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("write zip entry %q: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	return buf.Bytes()
}

func sourceFileRefByPathForKernel(entries []SourceFileDescriptor, path string) string {
	for _, entry := range entries {
		if entry.Path == path {
			return entry.SourceFileRef
		}
	}
	return ""
}

func readAll(t *testing.T, r io.Reader) []byte {
	t.Helper()
	body, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return body
}

func strconvQuote(value string) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(data)
}
