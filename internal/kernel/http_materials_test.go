package kernel

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
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
		LedgerPath:   filepath.Join(dir, "events.sqlite"),
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
	k := newTestKernel(t, filepath.Join(dir, "events.sqlite"))
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

func TestMaterialUploadRestoresSourceSnapshotAfterRestart(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "http-material-process-lifetime")
	storePath := filepath.Join(dir, "material-store")
	k, err := New(Config{
		LedgerPath:        filepath.Join(dir, "events.sqlite"),
		Provider:          FakeProvider{},
		RuntimeToken:      testRuntimeToken,
		MaterialStorePath: storePath,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	resp := postMultipartMaterialUpload(t, server.URL+"/materials/upload", map[string]string{
		"session_id": "upload-process-lifetime",
		"purpose":    SourcePurposeAnalysis,
	}, "package.zip", zipBytesFixture(t, map[string]string{"src/main.go": "package main\n"}))
	defer resp.Body.Close()
	payload := readAll(t, resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /materials/upload status=%d body=%s", resp.StatusCode, payload)
	}
	var projection MaterialIntakeProjection
	if err := json.Unmarshal(payload, &projection); err != nil {
		t.Fatalf("unmarshal material projection: %v", err)
	}

	restarted, err := New(Config{
		LedgerPath:        filepath.Join(dir, "events.sqlite"),
		Provider:          FakeProvider{},
		RuntimeToken:      testRuntimeToken,
		MaterialStorePath: storePath,
	})
	if err != nil {
		t.Fatalf("New restarted kernel returned error: %v", err)
	}
	treeReq, _, code, err := restarted.resourceRegistry.AdmitSourceTree(projection.SourceSnapshotRef, nil)
	if err != nil {
		t.Fatalf("restarted AdmitSourceTree code=%q err=%v, want restored source ref", code, err)
	}
	tree, err := restarted.resourceRegistry.SourceTree(treeReq)
	if err != nil {
		t.Fatalf("restarted SourceTree returned error: %v", err)
	}
	fileRef := sourceFileRefByPathForKernel(tree.Entries, "src/main.go")
	if fileRef == "" {
		t.Fatalf("restarted entries = %+v, want src/main.go", tree.Entries)
	}
	readReq, _, code, err := restarted.resourceRegistry.AdmitSourceRead(fileRef, nil, nil)
	if err != nil {
		t.Fatalf("restarted AdmitSourceRead code=%q err=%v, want restored source file", code, err)
	}
	read, err := restarted.resourceRegistry.SourceRead(readReq)
	if err != nil || read.Text != "package main\n" {
		t.Fatalf("restarted SourceRead result=%+v err=%v, want uploaded body", read, err)
	}
	capabilities := restarted.Capabilities()
	if capabilities.SourceSnapshotPersistence.Readiness != ReadinessReady ||
		capabilities.SourceSnapshotPersistence.ReadinessReason != "uploaded_snapshot_recovery" {
		t.Fatalf("source snapshot persistence = %+v, want ready/uploaded_snapshot_recovery", capabilities.SourceSnapshotPersistence)
	}
}

func TestMaterialSourceSnapshotRecoveryRejectsTrailingIndexData(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "http-material-index-corrupt")
	storePath := filepath.Join(dir, "material-store")
	if err := os.MkdirAll(storePath, 0o755); err != nil {
		t.Fatalf("MkdirAll material store: %v", err)
	}
	if err := os.WriteFile(filepath.Join(storePath, "source-snapshots.json"), []byte(`{"snapshots":[]} trailing`), 0o600); err != nil {
		t.Fatalf("WriteFile source index: %v", err)
	}
	k, err := New(Config{
		LedgerPath:        filepath.Join(dir, "events.sqlite"),
		Provider:          FakeProvider{},
		RuntimeToken:      testRuntimeToken,
		MaterialStorePath: storePath,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	capabilities := k.Capabilities()
	if capabilities.SourceSnapshotPersistence.Readiness != ReadinessNotReady ||
		capabilities.SourceSnapshotPersistence.ReadinessReason != "source_snapshot_index_unavailable" {
		t.Fatalf("source snapshot persistence = %+v, want not_ready/source_snapshot_index_unavailable", capabilities.SourceSnapshotPersistence)
	}
}

func TestMaterialLocalPathSnapshotDoesNotRestoreAfterRestart(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "http-material-local-restart")
	zipPath := filepath.Join(dir, "package.zip")
	writeKernelZipFixture(t, zipPath, map[string]string{"README.md": "local body\n"})
	ledgerPath := filepath.Join(dir, "events.sqlite")
	storePath := filepath.Join(dir, "material-store")
	k, err := New(Config{
		LedgerPath:        ledgerPath,
		Provider:          FakeProvider{},
		RuntimeToken:      testRuntimeToken,
		MaterialStorePath: storePath,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	intake, err := k.IntakeMaterial(MaterialIntakeRequest{
		SessionID: "local-restart-session",
		Purpose:   SourcePurposeAnalysis,
		Locator:   MaterialLocator{Kind: MaterialLocatorKindLocalPath, Path: zipPath},
	})
	if err != nil {
		t.Fatalf("IntakeMaterial returned error: %v", err)
	}
	restarted, err := New(Config{
		LedgerPath:        ledgerPath,
		Provider:          FakeProvider{},
		RuntimeToken:      testRuntimeToken,
		MaterialStorePath: storePath,
	})
	if err != nil {
		t.Fatalf("New restarted kernel returned error: %v", err)
	}
	if _, _, code, err := restarted.resourceRegistry.AdmitSourceTree(intake.SourceSnapshotRef, nil); err == nil || code != "unknown_source_snapshot_ref" {
		t.Fatalf("restarted AdmitSourceTree code=%q err=%v, want local snapshot absent", code, err)
	}
}

func TestMaterialUploadRecoveryRejectsIndexPathOutsideMaterialStore(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "http-material-index-escape")
	ledgerPath := filepath.Join(dir, "events.sqlite")
	storePath := filepath.Join(dir, "material-store")
	k, err := New(Config{
		LedgerPath:        ledgerPath,
		Provider:          FakeProvider{},
		RuntimeToken:      testRuntimeToken,
		MaterialStorePath: storePath,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	server := httptest.NewServer(Handler(k))
	defer server.Close()
	resp := postMultipartMaterialUpload(t, server.URL+"/materials/upload", map[string]string{
		"session_id": "upload-index-escape",
		"purpose":    SourcePurposeAnalysis,
	}, "package.zip", zipBytesFixture(t, map[string]string{"README.md": "trusted body\n"}))
	defer resp.Body.Close()
	var intake MaterialIntakeProjection
	if err := json.Unmarshal(readAll(t, resp.Body), &intake); err != nil {
		t.Fatalf("unmarshal upload projection: %v", err)
	}
	outsidePath := filepath.Join(dir, "outside.zip")
	writeKernelZipFixture(t, outsidePath, map[string]string{"README.md": "outside body\n"})
	indexPath := filepath.Join(storePath, "source-snapshots.json")
	var index map[string]interface{}
	indexBody, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("ReadFile source index: %v", err)
	}
	if err := json.Unmarshal(indexBody, &index); err != nil {
		t.Fatalf("unmarshal source index: %v", err)
	}
	snapshots, ok := index["snapshots"].([]interface{})
	if !ok || len(snapshots) != 1 {
		t.Fatalf("source index snapshots = %#v, want one", index["snapshots"])
	}
	record, ok := snapshots[0].(map[string]interface{})
	if !ok {
		t.Fatalf("source index record = %#v, want object", snapshots[0])
	}
	record["object_name"] = "..\\" + filepath.Base(outsidePath)
	tampered, err := json.Marshal(index)
	if err != nil {
		t.Fatalf("marshal tampered source index: %v", err)
	}
	if err := os.WriteFile(indexPath, tampered, 0o600); err != nil {
		t.Fatalf("WriteFile tampered source index: %v", err)
	}
	restarted, err := New(Config{
		LedgerPath:        ledgerPath,
		Provider:          FakeProvider{},
		RuntimeToken:      testRuntimeToken,
		MaterialStorePath: storePath,
	})
	if err != nil {
		t.Fatalf("New restarted kernel returned error: %v", err)
	}
	capabilities := restarted.Capabilities()
	if capabilities.SourceSnapshotPersistence.Readiness != ReadinessNotReady || capabilities.SourceSnapshotPersistence.ReadinessReason != "source_snapshot_index_unavailable" {
		t.Fatalf("source snapshot persistence = %+v, want not_ready/source_snapshot_index_unavailable", capabilities.SourceSnapshotPersistence)
	}
	if _, _, code, err := restarted.resourceRegistry.AdmitSourceTree(intake.SourceSnapshotRef, nil); err == nil || code != "unknown_source_snapshot_ref" {
		t.Fatalf("restarted AdmitSourceTree code=%q err=%v, want no escaped source authority", code, err)
	}
}

func TestMaterialUploadRecoveryRejectsChangedOwnedArchive(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "http-material-index-integrity")
	ledgerPath := filepath.Join(dir, "events.sqlite")
	storePath := filepath.Join(dir, "material-store")
	k, err := New(Config{
		LedgerPath:        ledgerPath,
		Provider:          FakeProvider{},
		RuntimeToken:      testRuntimeToken,
		MaterialStorePath: storePath,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	server := httptest.NewServer(Handler(k))
	defer server.Close()
	resp := postMultipartMaterialUpload(t, server.URL+"/materials/upload", map[string]string{
		"session_id": "upload-integrity",
		"purpose":    SourcePurposeAnalysis,
	}, "package.zip", zipBytesFixture(t, map[string]string{"README.md": "original body\n"}))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload status=%d body=%s", resp.StatusCode, readAll(t, resp.Body))
	}
	indexBody, err := os.ReadFile(filepath.Join(storePath, "source-snapshots.json"))
	if err != nil {
		t.Fatalf("ReadFile source index: %v", err)
	}
	var index map[string]interface{}
	if err := json.Unmarshal(indexBody, &index); err != nil {
		t.Fatalf("unmarshal source index: %v", err)
	}
	snapshots := index["snapshots"].([]interface{})
	record := snapshots[0].(map[string]interface{})
	objectPath := filepath.Join(storePath, record["object_name"].(string))
	if err := os.WriteFile(objectPath, zipBytesFixture(t, map[string]string{"README.md": "changed body\n"}), 0o600); err != nil {
		t.Fatalf("overwrite owned archive: %v", err)
	}
	restarted, err := New(Config{
		LedgerPath:        ledgerPath,
		Provider:          FakeProvider{},
		RuntimeToken:      testRuntimeToken,
		MaterialStorePath: storePath,
	})
	if err != nil {
		t.Fatalf("New restarted kernel returned error: %v", err)
	}
	var intakeIndex map[string]interface{}
	if err := json.Unmarshal(indexBody, &intakeIndex); err != nil {
		t.Fatalf("unmarshal source index again: %v", err)
	}
	ref := intakeIndex["snapshots"].([]interface{})[0].(map[string]interface{})["ref"].(string)
	if _, _, code, err := restarted.resourceRegistry.AdmitSourceTree(ref, nil); err == nil || code != "invalid_source_archive" {
		t.Fatalf("restarted AdmitSourceTree code=%q err=%v, want invalid_source_archive for changed archive", code, err)
	}
}

func TestMaterialUploadRecoveryRejectsMissingIndexRecord(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "http-material-index-missing")
	ledgerPath := filepath.Join(dir, "events.sqlite")
	storePath := filepath.Join(dir, "material-store")
	k, err := New(Config{
		LedgerPath:        ledgerPath,
		Provider:          FakeProvider{},
		RuntimeToken:      testRuntimeToken,
		MaterialStorePath: storePath,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	server := httptest.NewServer(Handler(k))
	defer server.Close()
	resp := postMultipartMaterialUpload(t, server.URL+"/materials/upload", map[string]string{
		"session_id": "upload-index-missing",
		"purpose":    SourcePurposeAnalysis,
	}, "package.zip", zipBytesFixture(t, map[string]string{"README.md": "indexed body\n"}))
	defer resp.Body.Close()
	var intake MaterialIntakeProjection
	if err := json.Unmarshal(readAll(t, resp.Body), &intake); err != nil {
		t.Fatalf("unmarshal upload projection: %v", err)
	}
	if err := os.WriteFile(filepath.Join(storePath, "source-snapshots.json"), []byte(`{"snapshots":[]}`), 0o600); err != nil {
		t.Fatalf("WriteFile empty source index: %v", err)
	}
	restarted, err := New(Config{
		LedgerPath:        ledgerPath,
		Provider:          FakeProvider{},
		RuntimeToken:      testRuntimeToken,
		MaterialStorePath: storePath,
	})
	if err != nil {
		t.Fatalf("New restarted kernel returned error: %v", err)
	}
	capabilities := restarted.Capabilities()
	if capabilities.SourceSnapshotPersistence.Readiness != ReadinessNotReady || capabilities.SourceSnapshotPersistence.ReadinessReason != "source_snapshot_index_unavailable" {
		t.Fatalf("source snapshot persistence = %+v, want not_ready/source_snapshot_index_unavailable", capabilities.SourceSnapshotPersistence)
	}
	if _, _, code, err := restarted.resourceRegistry.AdmitSourceTree(intake.SourceSnapshotRef, nil); err == nil || code != "unknown_source_snapshot_ref" {
		t.Fatalf("restarted AdmitSourceTree code=%q err=%v, want missing source record refusal", code, err)
	}
}

func TestMaterialUploadRecoveryRetainsOpaqueRefWhenStoredObjectIsMissing(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "http-material-object-missing")
	ledgerPath := filepath.Join(dir, "events.sqlite")
	storePath := filepath.Join(dir, "material-store")
	k, err := New(Config{
		LedgerPath:        ledgerPath,
		Provider:          FakeProvider{},
		RuntimeToken:      testRuntimeToken,
		MaterialStorePath: storePath,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	server := httptest.NewServer(Handler(k))
	defer server.Close()
	resp := postMultipartMaterialUpload(t, server.URL+"/materials/upload", map[string]string{
		"session_id": "upload-object-missing",
		"purpose":    SourcePurposeAnalysis,
	}, "package.zip", zipBytesFixture(t, map[string]string{"README.md": "stored body\n"}))
	defer resp.Body.Close()
	var intake MaterialIntakeProjection
	if err := json.Unmarshal(readAll(t, resp.Body), &intake); err != nil {
		t.Fatalf("unmarshal upload projection: %v", err)
	}
	indexBody, err := os.ReadFile(filepath.Join(storePath, "source-snapshots.json"))
	if err != nil {
		t.Fatalf("ReadFile source index: %v", err)
	}
	var index map[string]interface{}
	if err := json.Unmarshal(indexBody, &index); err != nil {
		t.Fatalf("unmarshal source index: %v", err)
	}
	objectName := index["snapshots"].([]interface{})[0].(map[string]interface{})["object_name"].(string)
	if err := os.Remove(filepath.Join(storePath, objectName)); err != nil {
		t.Fatalf("remove stored object: %v", err)
	}
	restarted, err := New(Config{
		LedgerPath:        ledgerPath,
		Provider:          FakeProvider{},
		RuntimeToken:      testRuntimeToken,
		MaterialStorePath: storePath,
	})
	if err != nil {
		t.Fatalf("New restarted kernel returned error: %v", err)
	}
	if _, _, code, err := restarted.resourceRegistry.AdmitSourceTree(intake.SourceSnapshotRef, nil); err == nil || code != "resource_unavailable" {
		t.Fatalf("restarted AdmitSourceTree code=%q err=%v, want resource_unavailable", code, err)
	}
}

func TestMaterialUploadRecoveryRejectsArchiveChangedAfterRestart(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "http-material-post-restart-integrity")
	ledgerPath := filepath.Join(dir, "events.sqlite")
	storePath := filepath.Join(dir, "material-store")
	k, err := New(Config{
		LedgerPath:        ledgerPath,
		Provider:          FakeProvider{},
		RuntimeToken:      testRuntimeToken,
		MaterialStorePath: storePath,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	server := httptest.NewServer(Handler(k))
	defer server.Close()
	resp := postMultipartMaterialUpload(t, server.URL+"/materials/upload", map[string]string{
		"session_id": "upload-post-restart-integrity",
		"purpose":    SourcePurposeAnalysis,
	}, "package.zip", zipBytesFixture(t, map[string]string{"README.md": "original body\n"}))
	defer resp.Body.Close()
	var intake MaterialIntakeProjection
	if err := json.Unmarshal(readAll(t, resp.Body), &intake); err != nil {
		t.Fatalf("unmarshal upload projection: %v", err)
	}
	restarted, err := New(Config{
		LedgerPath:        ledgerPath,
		Provider:          FakeProvider{},
		RuntimeToken:      testRuntimeToken,
		MaterialStorePath: storePath,
	})
	if err != nil {
		t.Fatalf("New restarted kernel returned error: %v", err)
	}
	indexBody, err := os.ReadFile(filepath.Join(storePath, "source-snapshots.json"))
	if err != nil {
		t.Fatalf("ReadFile source index: %v", err)
	}
	var index map[string]interface{}
	if err := json.Unmarshal(indexBody, &index); err != nil {
		t.Fatalf("unmarshal source index: %v", err)
	}
	objectName := index["snapshots"].([]interface{})[0].(map[string]interface{})["object_name"].(string)
	if err := os.WriteFile(filepath.Join(storePath, objectName), zipBytesFixture(t, map[string]string{"README.md": "changed after restart\n"}), 0o600); err != nil {
		t.Fatalf("overwrite stored object: %v", err)
	}
	descriptors := restarted.resourceRegistry.ListSourceSnapshotDescriptors("upload-post-restart-integrity")
	if len(descriptors) != 1 || len(descriptors[0].Diagnostics) != 1 || descriptors[0].Diagnostics[0].Code != "invalid_source_archive" {
		t.Fatalf("source descriptors = %+v, want one invalid_source_archive descriptor", descriptors)
	}
	if _, _, code, err := restarted.resourceRegistry.AdmitSourceTree(intake.SourceSnapshotRef, nil); err == nil || code != "invalid_source_archive" {
		t.Fatalf("restarted AdmitSourceTree code=%q err=%v, want invalid_source_archive", code, err)
	}
}

func TestHTTPMaterialUploadRejectsNonZipBinary(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "http-material-upload-binary")
	k := newTestKernel(t, filepath.Join(dir, "events.sqlite"))
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
