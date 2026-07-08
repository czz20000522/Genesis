package kernel

import (
	"archive/zip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"genesis/internal/testsupport"
)

func TestSourceSnapshotContextOmitsHostPathAndAdvertisesTypedTools(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "source-tools-context")
	zipPath := filepath.Join(dir, "package.zip")
	writeKernelZipFixture(t, zipPath, map[string]string{
		"README.md": "# Package\nhello",
	})
	provider := &recordingTextProvider{text: "ready"}
	k, err := New(Config{
		LedgerPath:   filepath.Join(dir, "events.sqlite"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	intake, err := k.IntakeMaterial(MaterialIntakeRequest{
		SessionID: "source-session",
		Purpose:   SourcePurposeAnalysis,
		Locator: MaterialLocator{
			Kind: MaterialLocatorKindLocalPath,
			Path: zipPath,
		},
	})
	if err != nil {
		t.Fatalf("IntakeMaterial returned error: %v", err)
	}
	if intake.SourceSnapshotRef == "" || intake.Root.SourceSnapshotRef != intake.SourceSnapshotRef {
		t.Fatalf("intake projection = %+v, want source snapshot ref", intake)
	}

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "source-session",
		InputItems: []InputItem{{Type: "text", Text: "Inspect the package"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "ready" {
		t.Fatalf("final text = %q, want ready", resp.Final.Text)
	}
	requests := provider.Requests()
	if len(requests) != 1 {
		t.Fatalf("provider requests = %+v, want one request", requests)
	}
	sourceContext, ok := modelInputTextByKind(requests[0].InputItems, ModelInputKindSourceSnapshotContext)
	if !ok {
		t.Fatalf("provider input kinds = %+v, want source snapshot context", modelInputKinds(requests[0].InputItems))
	}
	for _, want := range []string{intake.SourceSnapshotRef, "source_tree", "source_read"} {
		if !strings.Contains(sourceContext, want) {
			t.Fatalf("source context = %q, missing %q", sourceContext, want)
		}
	}
	for _, forbidden := range []string{zipPath, dir, "storage_ref", "object_key", "host_path"} {
		if strings.Contains(sourceContext, forbidden) {
			t.Fatalf("source context leaked %q: %q", forbidden, sourceContext)
		}
	}
}

func TestSourceSnapshotContextIsBounded(t *testing.T) {
	snapshots := make([]SourceSnapshotDescriptor, 0, 80)
	for i := 0; i < 80; i++ {
		snapshots = append(snapshots, SourceSnapshotDescriptor{
			SourceSnapshotRef:   "source_snapshot_" + strings.Repeat("a", 24) + string(rune('a'+i%26)),
			SourceKind:          SourceKindZip,
			Purpose:             SourcePurposeAnalysis,
			DisplayLabel:        strings.Repeat("label", 80),
			AvailableOperations: []string{ReferenceOperationSourceTree},
		})
	}

	context := sourceSnapshotContext(snapshots)
	if len([]byte(context)) > sourceSnapshotContextBytes {
		t.Fatalf("source context bytes = %d, want <= %d", len([]byte(context)), sourceSnapshotContextBytes)
	}
	if !strings.Contains(context, "additional source snapshots omitted by context budget") {
		t.Fatalf("source context missing omission hint: %q", context)
	}
	if strings.Contains(context, strings.Repeat("label", 40)) {
		t.Fatalf("source context did not bound display label: %q", context)
	}
}

func TestSourceTreeAndReadToolLoopUsesOpaqueRefs(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "source-tools-loop")
	zipPath := filepath.Join(dir, "package.zip")
	writeKernelZipFixture(t, zipPath, map[string]string{
		"README.md":   "# Package\nhello",
		"src/main.go": "package main\nfunc main() {}\n",
	})
	provider := &sourceSnapshotToolProvider{}
	k, err := New(Config{
		LedgerPath:   filepath.Join(dir, "events.sqlite"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	intake, err := k.IntakeMaterial(MaterialIntakeRequest{
		SessionID: "source-tool-session",
		Purpose:   SourcePurposeAnalysis,
		Locator: MaterialLocator{
			Kind: MaterialLocatorKindLocalPath,
			Path: zipPath,
		},
	})
	if err != nil {
		t.Fatalf("IntakeMaterial returned error: %v", err)
	}
	provider.snapshotRef = intake.SourceSnapshotRef

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "source-tool-session",
		InputItems: []InputItem{{Type: "text", Text: "Read source package"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if !strings.Contains(resp.Final.Text, "main.go observed") {
		t.Fatalf("final text = %q, want source-based answer", resp.Final.Text)
	}
	requests := provider.Requests()
	if len(requests) != 3 {
		t.Fatalf("provider requests = %+v, want source_tree, source_read, final", requests)
	}
	treeResult := requests[1].ToolRounds[0].Results[0]
	readResult := requests[2].ToolRounds[1].Results[0]
	for _, result := range []ModelToolResult{treeResult, readResult} {
		if strings.Contains(result.Content, zipPath) || strings.Contains(result.Content, dir) {
			t.Fatalf("tool result leaked host path: %+v", result)
		}
	}
}

func TestSourceTreeTruncationProvidesModelContinuationContract(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "source-tools-tree-continuation")
	zipPath := filepath.Join(dir, "package.zip")
	writeKernelZipFixture(t, zipPath, map[string]string{
		"a.txt": "a",
		"b.txt": "b",
		"c.txt": "c",
		"d.txt": "d",
	})
	k, err := New(Config{
		LedgerPath:   filepath.Join(dir, "events.sqlite"),
		Provider:     FakeProvider{},
		RuntimeToken: testRuntimeToken,
		SourceSnapshotPolicy: SourceSnapshotPolicy{
			DefaultTreeEntries: 2,
			MaxTreeEntries:     5,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	intake, err := k.IntakeMaterial(MaterialIntakeRequest{
		SessionID: "source-continuation-session",
		Purpose:   SourcePurposeAnalysis,
		Locator: MaterialLocator{
			Kind: MaterialLocatorKindLocalPath,
			Path: zipPath,
		},
	})
	if err != nil {
		t.Fatalf("IntakeMaterial returned error: %v", err)
	}

	prepared, err := k.toolGateway().PrepareBatch([]ModelToolCall{{
		ToolCallID:      "call_source_tree",
		ToolCallEventID: "evt_source_tree",
		Name:            "source_tree",
		Arguments:       mustMarshalToolArgs(t, map[string]interface{}{"source_snapshot_ref": intake.SourceSnapshotRef}),
	}})
	if err != nil {
		t.Fatalf("PrepareBatch returned error: %v", err)
	}
	result, err := k.toolGateway().Execute(context.Background(), "source-continuation-session", "turn-source-continuation", prepared[0])
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	var payload SourceTreeResult
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatalf("unmarshal source_tree payload: %v\n%s", err, result.Content)
	}
	if !payload.Truncated || payload.TotalEntries != 4 || len(payload.Entries) != 2 {
		t.Fatalf("source_tree payload = %+v, want truncated first page", payload)
	}
	if payload.NextMaxEntries == nil || *payload.NextMaxEntries != 4 || payload.MaxEntriesLimit != 5 {
		t.Fatalf("source_tree continuation = %+v, want next max_entries=4 limit=5", payload)
	}
	for _, want := range []string{"max_entries", "offset_entries"} {
		if !strings.Contains(payload.ContinuationHint, want) {
			t.Fatalf("continuation hint = %q, missing %q", payload.ContinuationHint, want)
		}
	}
}

func TestSourceTreeAtMaxEntriesLimitReturnsTerminalContinuationHint(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "source-tools-tree-continuation-cap")
	zipPath := filepath.Join(dir, "package.zip")
	writeKernelZipFixture(t, zipPath, map[string]string{
		"a.txt": "a",
		"b.txt": "b",
		"c.txt": "c",
		"d.txt": "d",
	})
	k, err := New(Config{
		LedgerPath:   filepath.Join(dir, "events.sqlite"),
		Provider:     FakeProvider{},
		RuntimeToken: testRuntimeToken,
		SourceSnapshotPolicy: SourceSnapshotPolicy{
			DefaultTreeEntries: 2,
			MaxTreeEntries:     2,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	intake, err := k.IntakeMaterial(MaterialIntakeRequest{
		SessionID: "source-continuation-cap-session",
		Purpose:   SourcePurposeAnalysis,
		Locator: MaterialLocator{
			Kind: MaterialLocatorKindLocalPath,
			Path: zipPath,
		},
	})
	if err != nil {
		t.Fatalf("IntakeMaterial returned error: %v", err)
	}

	prepared, err := k.toolGateway().PrepareBatch([]ModelToolCall{{
		ToolCallID:      "call_source_tree_cap",
		ToolCallEventID: "evt_source_tree_cap",
		Name:            "source_tree",
		Arguments:       mustMarshalToolArgs(t, map[string]interface{}{"source_snapshot_ref": intake.SourceSnapshotRef}),
	}})
	if err != nil {
		t.Fatalf("PrepareBatch returned error: %v", err)
	}
	result, err := k.toolGateway().Execute(context.Background(), "source-continuation-cap-session", "turn-source-continuation-cap", prepared[0])
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	var payload SourceTreeResult
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatalf("unmarshal source_tree payload: %v\n%s", err, result.Content)
	}
	if !payload.Truncated || payload.NextMaxEntries != nil || payload.MaxEntriesLimit != 2 {
		t.Fatalf("source_tree continuation = %+v, want terminal cap with no next max_entries", payload)
	}
	for _, want := range []string{"max_entries_limit", "Use source_read"} {
		if !strings.Contains(payload.ContinuationHint, want) {
			t.Fatalf("terminal continuation hint = %q, missing %q", payload.ContinuationHint, want)
		}
	}
	if strings.Contains(payload.ContinuationHint, "narrower source exploration surface") {
		t.Fatalf("terminal continuation hint is not executable by current tool schema: %q", payload.ContinuationHint)
	}
}

func TestSourceTreeUnknownOffsetEntriesReturnsRepairableContinuationHint(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))

	prepared, err := k.toolGateway().PrepareBatch([]ModelToolCall{{
		ToolCallID:      "call_source_tree_invalid",
		ToolCallEventID: "evt_source_tree_invalid",
		Name:            "source_tree",
		Arguments:       mustMarshalToolArgs(t, map[string]interface{}{"source_snapshot_ref": "source_snapshot_test", "offset_entries": 2}),
	}})
	if err != nil {
		t.Fatalf("PrepareBatch returned error: %v", err)
	}
	result, err := k.toolGateway().Execute(context.Background(), "source-continuation-session", "turn-source-continuation", prepared[0])
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	var payload ToolRequestInvalidProjection
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatalf("unmarshal invalid payload: %v\n%s", err, result.Content)
	}
	if payload.Status != "tool_request_invalid" || payload.Executed || payload.Error.Code != "invalid_tool_arguments" {
		t.Fatalf("invalid payload = %+v, want repairable invalid arguments", payload)
	}
	for _, want := range []string{"offset_entries is not supported", "max_entries"} {
		if !strings.Contains(payload.Error.Message, want) {
			t.Fatalf("invalid message = %q, missing %q", payload.Error.Message, want)
		}
	}
}

func TestSourceToolsRejectHostPathsAsModelArguments(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "source-tools-host-path")
	zipPath := filepath.Join(dir, "package.zip")
	writeKernelZipFixture(t, zipPath, map[string]string{"README.md": "hello"})
	k := newTestKernel(t, filepath.Join(dir, "events.sqlite"))

	for _, tc := range []struct {
		name     string
		toolName string
		args     map[string]interface{}
		wantCode string
	}{
		{
			name:     "source_tree",
			toolName: "source_tree",
			args:     map[string]interface{}{"source_snapshot_ref": zipPath},
			wantCode: "owner_internal_ref_not_source_snapshot",
		},
		{
			name:     "source_read",
			toolName: "source_read",
			args:     map[string]interface{}{"source_file_ref": zipPath},
			wantCode: "owner_internal_ref_not_source_file",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prepared, err := k.toolGateway().PrepareBatch([]ModelToolCall{{
				ToolCallID:      "call_" + tc.name,
				ToolCallEventID: "evt_tool_" + tc.name,
				Name:            tc.toolName,
				Arguments:       mustMarshalToolArgs(t, tc.args),
			}})
			if err != nil {
				t.Fatalf("PrepareBatch returned error: %v", err)
			}
			result, err := k.toolGateway().Execute(context.Background(), "source-session", "turn-source", prepared[0])
			if err != nil {
				t.Fatalf("Execute returned error: %v", err)
			}
			var payload ToolRequestInvalidProjection
			if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
				t.Fatalf("unmarshal invalid payload: %v\n%s", err, result.Content)
			}
			if payload.Status != "tool_request_invalid" || payload.Executed || payload.Error.Code != tc.wantCode {
				t.Fatalf("invalid payload = %+v, want code %s", payload, tc.wantCode)
			}
		})
	}
}

type sourceSnapshotToolProvider struct {
	mu          sync.Mutex
	snapshotRef string
	fileRef     string
	requests    []ModelRequest
}

func (p *sourceSnapshotToolProvider) Name() string {
	return "source-snapshot-tool-provider"
}

func (p *sourceSnapshotToolProvider) Ready() ProviderStatus {
	return ProviderStatus{Name: p.Name(), Readiness: ReadinessReady}
}

func (p *sourceSnapshotToolProvider) Complete(_ context.Context, req ModelRequest) (ModelResponse, error) {
	p.mu.Lock()
	p.requests = append(p.requests, req)
	callCount := len(p.requests)
	p.mu.Unlock()
	switch callCount {
	case 1:
		return ModelResponse{
			Model: "source-tool-model",
			ToolCalls: []ModelToolCall{{
				ToolCallID: "call_source_tree",
				Name:       "source_tree",
				Arguments:  mustMarshalToolArgsForProvider(map[string]interface{}{"source_snapshot_ref": p.snapshotRef}),
			}},
		}, nil
	case 2:
		var tree struct {
			Entries []struct {
				SourceFileRef string `json:"source_file_ref"`
				Path          string `json:"path"`
			} `json:"entries"`
		}
		_ = json.Unmarshal([]byte(req.ToolRounds[0].Results[0].Content), &tree)
		for _, entry := range tree.Entries {
			if entry.Path == "src/main.go" {
				p.fileRef = entry.SourceFileRef
			}
		}
		return ModelResponse{
			Model: "source-tool-model",
			ToolCalls: []ModelToolCall{{
				ToolCallID: "call_source_read",
				Name:       "source_read",
				Arguments:  mustMarshalToolArgsForProvider(map[string]interface{}{"source_file_ref": p.fileRef, "limit_bytes": 80}),
			}},
		}, nil
	default:
		return ModelResponse{Text: "main.go observed", Model: "source-tool-model"}, nil
	}
}

func (p *sourceSnapshotToolProvider) Requests() []ModelRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]ModelRequest(nil), p.requests...)
}

func writeKernelZipFixture(t *testing.T, path string, entries map[string]string) {
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
}

func mustMarshalToolArgsForProvider(args map[string]interface{}) json.RawMessage {
	data, err := json.Marshal(args)
	if err != nil {
		panic(err)
	}
	return data
}
