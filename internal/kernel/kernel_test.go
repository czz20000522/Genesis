package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSubmitTurnPersistsAndProjectsAfterRestart(t *testing.T) {
	ledgerPath := filepath.Join(t.TempDir(), "events.jsonl")
	k := newTestKernel(t, ledgerPath)

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID: "session-test",
		InputItems: []InputItem{
			{Type: "text", Text: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.SessionID != "session-test" {
		t.Fatalf("SessionID = %q, want session-test", resp.SessionID)
	}
	if resp.Final.Text != "fake: hello" {
		t.Fatalf("Final.Text = %q, want fake: hello", resp.Final.Text)
	}
	if len(resp.Events) != 2 {
		t.Fatalf("len(Events) = %d, want 2", len(resp.Events))
	}

	restarted := newTestKernel(t, ledgerPath)
	projection, err := restarted.Session("session-test")
	if err != nil {
		t.Fatalf("Session after restart returned error: %v", err)
	}
	if len(projection.Turns) != 1 {
		t.Fatalf("len(Turns) = %d, want 1", len(projection.Turns))
	}
	turn := projection.Turns[0]
	if turn.Status != "completed" {
		t.Fatalf("turn status = %q, want completed", turn.Status)
	}
	if turn.FinalMessage.Text != "fake: hello" {
		t.Fatalf("turn final = %q, want fake: hello", turn.FinalMessage.Text)
	}
	if len(projection.Events) != 2 {
		t.Fatalf("len(Events) = %d, want 2", len(projection.Events))
	}
}

func TestSubmitTurnRejectsInvalidInput(t *testing.T) {
	k := newTestKernel(t, filepath.Join(t.TempDir(), "events.jsonl"))

	_, err := k.SubmitTurn(context.Background(), TurnRequest{})
	if err == nil {
		t.Fatal("SubmitTurn returned nil error for missing input_items")
	}

	_, err = k.SubmitTurn(context.Background(), TurnRequest{
		InputItems: []InputItem{{Type: "image", Text: "not supported"}},
	})
	if err == nil {
		t.Fatal("SubmitTurn returned nil error for unsupported input type")
	}
}

func TestModelInputItemsInjectsApprovedMemoryContextBeforeProvider(t *testing.T) {
	items := modelInputItems(
		[]InputItem{{Type: "text", Text: "你记得我的回答偏好吗？"}},
		[]MemoryRecall{
			{Text: "我偏好中文回答", Source: "turn:memory-source"},
			{Text: "  "},
		},
	)

	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].Type != "text" || items[0].Text != "Approved memories:\n- 我偏好中文回答" {
		t.Fatalf("memory context item = %+v", items[0])
	}
	if items[1].Text != "你记得我的回答偏好吗？" {
		t.Fatalf("user item = %+v", items[1])
	}
}

func TestHTTPReadyTurnAndSession(t *testing.T) {
	k := newTestKernel(t, filepath.Join(t.TempDir(), "events.jsonl"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	readyResp, err := http.Get(server.URL + "/ready")
	if err != nil {
		t.Fatalf("GET /ready failed: %v", err)
	}
	defer readyResp.Body.Close()
	if readyResp.StatusCode != http.StatusOK {
		t.Fatalf("ready status = %d, want 200", readyResp.StatusCode)
	}
	var ready ReadyResponse
	if err := json.NewDecoder(readyResp.Body).Decode(&ready); err != nil {
		t.Fatalf("decode ready response: %v", err)
	}
	if ready.Status != "ok" || ready.Provider.Name != "fake" || ready.Provider.Status != "ok" {
		t.Fatalf("ready = %+v, want ok fake provider", ready)
	}
	if ready.RuntimeAuth.Status != "ok" {
		t.Fatalf("runtime auth ready = %+v, want ok", ready.RuntimeAuth)
	}

	body := []byte(`{"session_id":"http-session","input_items":[{"type":"text","text":"hello over http"}]}`)
	turnResp, err := postJSONWithAuth(server.URL+"/turn", body)
	if err != nil {
		t.Fatalf("POST /turn failed: %v", err)
	}
	defer turnResp.Body.Close()
	if turnResp.StatusCode != http.StatusOK {
		t.Fatalf("turn status = %d, want 200", turnResp.StatusCode)
	}
	var turn TurnResponse
	if err := json.NewDecoder(turnResp.Body).Decode(&turn); err != nil {
		t.Fatalf("decode turn response: %v", err)
	}
	if turn.Final.Text != "fake: hello over http" {
		t.Fatalf("turn final = %q, want fake: hello over http", turn.Final.Text)
	}

	sessionResp, err := getWithAuth(server.URL + "/sessions/http-session")
	if err != nil {
		t.Fatalf("GET /sessions failed: %v", err)
	}
	defer sessionResp.Body.Close()
	if sessionResp.StatusCode != http.StatusOK {
		t.Fatalf("session status = %d, want 200", sessionResp.StatusCode)
	}
	var projection SessionProjection
	if err := json.NewDecoder(sessionResp.Body).Decode(&projection); err != nil {
		t.Fatalf("decode session response: %v", err)
	}
	if len(projection.Turns) != 1 {
		t.Fatalf("len(Turns) = %d, want 1", len(projection.Turns))
	}
}

func TestHTTPRejectsUnknownTurnFields(t *testing.T) {
	ledgerPath := filepath.Join(t.TempDir(), "events.jsonl")
	k := newTestKernel(t, ledgerPath)
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	body := []byte(`{"session_id":"bad-session","input_items":[{"type":"text","text":"hello"}],"unexpected":true}`)
	resp, err := postJSONWithAuth(server.URL+"/turn", body)
	if err != nil {
		t.Fatalf("POST /turn failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if _, err := k.Session("bad-session"); err != ErrSessionNotFound {
		t.Fatalf("Session error = %v, want ErrSessionNotFound", err)
	}
}

func TestHTTPRejectsTrailingJSON(t *testing.T) {
	ledgerPath := filepath.Join(t.TempDir(), "events.jsonl")
	k := newTestKernel(t, ledgerPath)
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	body := []byte(`{"session_id":"bad-session","input_items":[{"type":"text","text":"hello"}]}{}`)
	resp, err := postJSONWithAuth(server.URL+"/turn", body)
	if err != nil {
		t.Fatalf("POST /turn failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if _, err := k.Session("bad-session"); err != ErrSessionNotFound {
		t.Fatalf("Session error = %v, want ErrSessionNotFound", err)
	}
}

func TestHTTPRejectsOversizedTurnRequest(t *testing.T) {
	ledgerPath := filepath.Join(t.TempDir(), "events.jsonl")
	k := newTestKernel(t, ledgerPath)
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	body := bytes.Repeat([]byte(" "), maxRequestBytes+1)
	resp, err := postJSONWithAuth(server.URL+"/turn", body)
	if err != nil {
		t.Fatalf("POST /turn failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestHTTPProtectedRoutesRequireRuntimeToken(t *testing.T) {
	k := newTestKernel(t, filepath.Join(t.TempDir(), "events.jsonl"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	body := []byte(`{"session_id":"http-session","input_items":[{"type":"text","text":"hello"}]}`)
	resp, err := http.Post(server.URL+"/turn", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /turn failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestHTTPProtectedRoutesFailClosedWithoutConfiguredRuntimeToken(t *testing.T) {
	k := newTestKernelWithRuntimeToken(t, filepath.Join(t.TempDir(), "events.jsonl"), "")
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	readyResp, err := http.Get(server.URL + "/ready")
	if err != nil {
		t.Fatalf("GET /ready failed: %v", err)
	}
	defer readyResp.Body.Close()
	if readyResp.StatusCode != http.StatusOK {
		t.Fatalf("ready status = %d, want 200", readyResp.StatusCode)
	}
	var ready ReadyResponse
	if err := json.NewDecoder(readyResp.Body).Decode(&ready); err != nil {
		t.Fatalf("decode ready response: %v", err)
	}
	if ready.Status != "blocked" {
		t.Fatalf("ready status = %q, want blocked", ready.Status)
	}
	if ready.RuntimeAuth.Status != "blocked" || ready.RuntimeAuth.Reason != "runtime_token_missing" {
		t.Fatalf("runtime auth ready = %+v, want runtime_token_missing blocker", ready.RuntimeAuth)
	}

	body := []byte(`{"session_id":"http-session","input_items":[{"type":"text","text":"hello"}]}`)
	resp, err := postJSONWithAuth(server.URL+"/turn", body)
	if err != nil {
		t.Fatalf("POST /turn failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
}

func TestHTTPRejectsNonJSONContentType(t *testing.T) {
	k := newTestKernel(t, filepath.Join(t.TempDir(), "events.jsonl"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	req, err := http.NewRequest(http.MethodPost, server.URL+"/turn", strings.NewReader(`{"input_items":[{"type":"text","text":"hello"}]}`))
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+testRuntimeToken)
	req.Header.Set("Content-Type", "text/plain")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /turn failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415", resp.StatusCode)
	}
}

func TestExecShellPlanBlocksMutatingCommand(t *testing.T) {
	ledgerPath := filepath.Join(t.TempDir(), "events.jsonl")
	workspace := t.TempDir()
	k := newTestKernel(t, ledgerPath)

	operation, err := k.ExecShell(context.Background(), ShellExecRequest{
		SessionID: "shell-plan",
		CWD:       workspace,
		Command:   "Set-Content -LiteralPath blocked.txt -Value no",
	})
	if err != nil {
		t.Fatalf("ExecShell returned error: %v", err)
	}
	if operation.Status != "blocked" {
		t.Fatalf("status = %q, want blocked", operation.Status)
	}
	if operation.BlockedReason != "blocked_by_permission_mode=plan" {
		t.Fatalf("blocked reason = %q, want plan blocker", operation.BlockedReason)
	}
	if _, err := os.Stat(filepath.Join(workspace, "blocked.txt")); !os.IsNotExist(err) {
		t.Fatalf("blocked command wrote file, stat err = %v", err)
	}
}

func TestExecShellDefaultCompletesInsideWorkspaceAndProjectsAfterRestart(t *testing.T) {
	ledgerPath := filepath.Join(t.TempDir(), "events.jsonl")
	workspace := t.TempDir()
	k := newTestKernelWithPolicy(t, ledgerPath, ToolPolicy{
		PermissionMode: PermissionModeDefault,
		WorkspaceRoot:  workspace,
	})

	operation, err := k.ExecShell(context.Background(), ShellExecRequest{
		SessionID: "shell-default",
		CWD:       workspace,
		Command:   writeFileCommand("output.txt", "ok"),
	})
	if err != nil {
		t.Fatalf("ExecShell returned error: %v", err)
	}
	if operation.Status != "completed" {
		t.Fatalf("status = %q, want completed; stderr=%q", operation.Status, operation.Stderr)
	}
	if operation.ExitCode == nil || *operation.ExitCode != 0 {
		t.Fatalf("exit code = %v, want 0", operation.ExitCode)
	}
	content, err := os.ReadFile(filepath.Join(workspace, "output.txt"))
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if string(content) != "ok" {
		t.Fatalf("output file = %q, want ok", string(content))
	}

	restarted := newTestKernelWithPolicy(t, ledgerPath, ToolPolicy{
		PermissionMode: PermissionModeDefault,
		WorkspaceRoot:  workspace,
	})
	projection, err := restarted.Session("shell-default")
	if err != nil {
		t.Fatalf("Session after restart returned error: %v", err)
	}
	if len(projection.Operations) != 1 {
		t.Fatalf("len(Operations) = %d, want 1", len(projection.Operations))
	}
	if projection.Operations[0].OperationID != operation.OperationID {
		t.Fatalf("operation id = %q, want %q", projection.Operations[0].OperationID, operation.OperationID)
	}
	if len(projection.Events) != 2 || projection.Events[0].OperationID != operation.OperationID || projection.Events[1].OperationID != operation.OperationID {
		t.Fatalf("events = %+v, want operation event", projection.Events)
	}
}

func TestExecShellDefaultBlocksOutsideWorkspace(t *testing.T) {
	ledgerPath := filepath.Join(t.TempDir(), "events.jsonl")
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	k := newTestKernelWithPolicy(t, ledgerPath, ToolPolicy{
		PermissionMode: PermissionModeDefault,
		WorkspaceRoot:  workspace,
	})

	operation, err := k.ExecShell(context.Background(), ShellExecRequest{
		SessionID: "shell-outside",
		CWD:       outside,
		Command:   echoCommand("hello"),
	})
	if err != nil {
		t.Fatalf("ExecShell returned error: %v", err)
	}
	if operation.Status != "blocked" {
		t.Fatalf("status = %q, want blocked", operation.Status)
	}
	if operation.BlockedReason != "cwd_outside_workspace" {
		t.Fatalf("blocked reason = %q, want cwd_outside_workspace", operation.BlockedReason)
	}
}

func TestExecShellDefaultBlocksMutatingCommandPathEscapesWorkspace(t *testing.T) {
	ledgerPath := filepath.Join(t.TempDir(), "events.jsonl")
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	k := newTestKernelWithPolicy(t, ledgerPath, ToolPolicy{
		PermissionMode: PermissionModeDefault,
		WorkspaceRoot:  workspace,
	})

	operation, err := k.ExecShell(context.Background(), ShellExecRequest{
		SessionID: "shell-escape",
		CWD:       workspace,
		Command:   "Set-Content -LiteralPath .." + string(filepath.Separator) + "outside.txt -Value no",
	})
	if err != nil {
		t.Fatalf("ExecShell returned error: %v", err)
	}
	if operation.Status != "blocked" {
		t.Fatalf("status = %q, want blocked", operation.Status)
	}
	if operation.BlockedReason != "command_path_outside_workspace" {
		t.Fatalf("blocked reason = %q, want command_path_outside_workspace", operation.BlockedReason)
	}
	if _, err := os.Stat(filepath.Join(root, "outside.txt")); !os.IsNotExist(err) {
		t.Fatalf("blocked command wrote outside file, stat err = %v", err)
	}

	equalFormOperation, err := k.ExecShell(context.Background(), ShellExecRequest{
		SessionID: "shell-escape-equal",
		CWD:       workspace,
		Command:   "Set-Content -LiteralPath=.." + string(filepath.Separator) + "outside-equal.txt -Value no",
	})
	if err != nil {
		t.Fatalf("ExecShell with equal-form path returned error: %v", err)
	}
	if equalFormOperation.Status != "blocked" {
		t.Fatalf("equal-form status = %q, want blocked", equalFormOperation.Status)
	}
	if _, err := os.Stat(filepath.Join(root, "outside-equal.txt")); !os.IsNotExist(err) {
		t.Fatalf("blocked equal-form command wrote outside file, stat err = %v", err)
	}

	absoluteOutsideFile := filepath.Join(root, "absolute-outside.txt")
	absoluteOperation, err := k.ExecShell(context.Background(), ShellExecRequest{
		SessionID: "shell-escape-absolute",
		CWD:       workspace,
		Command:   writeFileCommand(absoluteOutsideFile, "no"),
	})
	if err != nil {
		t.Fatalf("ExecShell with absolute outside path returned error: %v", err)
	}
	if absoluteOperation.Status != "blocked" {
		t.Fatalf("absolute path status = %q, want blocked", absoluteOperation.Status)
	}
	if absoluteOperation.BlockedReason != "command_path_outside_workspace" {
		t.Fatalf("absolute path blocked reason = %q, want command_path_outside_workspace", absoluteOperation.BlockedReason)
	}
	if _, err := os.Stat(absoluteOutsideFile); !os.IsNotExist(err) {
		t.Fatalf("blocked absolute path command wrote outside file, stat err = %v", err)
	}
}

func TestExecShellDefaultBlocksLinkedCWDOutsideWorkspace(t *testing.T) {
	ledgerPath := filepath.Join(t.TempDir(), "events.jsonl")
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	outside := filepath.Join(root, "outside")
	linkedCWD := filepath.Join(workspace, "linked-outside")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	createDirectoryLinkForTest(t, outside, linkedCWD)
	k := newTestKernelWithPolicy(t, ledgerPath, ToolPolicy{
		PermissionMode: PermissionModeDefault,
		WorkspaceRoot:  workspace,
	})

	operation, err := k.ExecShell(context.Background(), ShellExecRequest{
		SessionID: "shell-linked-cwd",
		CWD:       linkedCWD,
		Command:   echoCommand("hello"),
	})
	if err != nil {
		t.Fatalf("ExecShell returned error: %v", err)
	}
	if operation.Status != "blocked" {
		t.Fatalf("status = %q, want blocked", operation.Status)
	}
	if operation.BlockedReason != "cwd_outside_workspace" {
		t.Fatalf("blocked reason = %q, want cwd_outside_workspace", operation.BlockedReason)
	}
}

func TestExecShellDefaultBlocksRawShellAndEnvironmentAccess(t *testing.T) {
	ledgerPath := filepath.Join(t.TempDir(), "events.jsonl")
	workspace := t.TempDir()
	k := newTestKernelWithPolicy(t, ledgerPath, ToolPolicy{
		PermissionMode: PermissionModeDefault,
		WorkspaceRoot:  workspace,
	})

	for _, command := range []string{
		"env",
		"Write-Output $env:PATH",
		"echo hello; env",
	} {
		operation, err := k.ExecShell(context.Background(), ShellExecRequest{
			SessionID: "shell-default-unsupported",
			CWD:       workspace,
			Command:   command,
		})
		if err != nil {
			t.Fatalf("ExecShell returned error for %q: %v", command, err)
		}
		if operation.Status != "blocked" {
			t.Fatalf("status for %q = %q, want blocked", command, operation.Status)
		}
		if operation.BlockedReason != "unsupported_default_command" {
			t.Fatalf("blocked reason for %q = %q, want unsupported_default_command", command, operation.BlockedReason)
		}
	}
}

func TestExecShellRedactsSecretEvidenceBeforePersistence(t *testing.T) {
	ledgerPath := filepath.Join(t.TempDir(), "events.jsonl")
	workspace := t.TempDir()
	k := newTestKernelWithPolicy(t, ledgerPath, ToolPolicy{
		PermissionMode: PermissionModeYolo,
		WorkspaceRoot:  workspace,
	})

	operation, err := k.ExecShell(context.Background(), ShellExecRequest{
		SessionID: "shell-redaction",
		CWD:       workspace,
		Command:   secretEchoCommand(),
	})
	if err != nil {
		t.Fatalf("ExecShell returned error: %v", err)
	}
	if operation.Status != "completed" {
		t.Fatalf("status = %q, want completed; stderr=%q", operation.Status, operation.Stderr)
	}
	ledgerData, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	for _, leaked := range []string{"sk-secret123", "tokentest123456", "sk-jsonsecret"} {
		if strings.Contains(operation.Command, leaked) || strings.Contains(operation.Stdout, leaked) || strings.Contains(operation.Stderr, leaked) {
			t.Fatalf("operation evidence leaked %q: %+v", leaked, operation)
		}
		if strings.Contains(string(ledgerData), leaked) {
			t.Fatalf("ledger leaked %q: %s", leaked, string(ledgerData))
		}
	}
	if !strings.Contains(operation.Command+operation.Stdout+string(ledgerData), "[REDACTED]") {
		t.Fatalf("redaction marker missing from operation/ledger evidence")
	}
}

func TestHTTPShellExecAndSessionProjection(t *testing.T) {
	ledgerPath := filepath.Join(t.TempDir(), "events.jsonl")
	workspace := t.TempDir()
	k := newTestKernelWithPolicy(t, ledgerPath, ToolPolicy{
		PermissionMode: PermissionModeDefault,
		WorkspaceRoot:  workspace,
	})
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	payload, err := json.Marshal(ShellExecRequest{
		SessionID: "http-shell",
		CWD:       workspace,
		Command:   echoCommand("hello"),
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	resp, err := postJSONWithAuth(server.URL+"/tools/shell.exec", payload)
	if err != nil {
		t.Fatalf("POST /tools/shell.exec failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var operation OperationProjection
	if err := json.NewDecoder(resp.Body).Decode(&operation); err != nil {
		t.Fatalf("decode shell response: %v", err)
	}
	if operation.Status != "completed" {
		t.Fatalf("status = %q, want completed; stderr=%q", operation.Status, operation.Stderr)
	}
	if !strings.Contains(operation.Stdout, "hello") {
		t.Fatalf("stdout = %q, want hello", operation.Stdout)
	}

	sessionResp, err := getWithAuth(server.URL + "/sessions/http-shell")
	if err != nil {
		t.Fatalf("GET /sessions failed: %v", err)
	}
	defer sessionResp.Body.Close()
	if sessionResp.StatusCode != http.StatusOK {
		t.Fatalf("session status = %d, want 200", sessionResp.StatusCode)
	}
	var projection SessionProjection
	if err := json.NewDecoder(sessionResp.Body).Decode(&projection); err != nil {
		t.Fatalf("decode session response: %v", err)
	}
	if len(projection.Operations) != 1 {
		t.Fatalf("len(Operations) = %d, want 1", len(projection.Operations))
	}
}

func TestHTTPRejectsUnknownShellFields(t *testing.T) {
	ledgerPath := filepath.Join(t.TempDir(), "events.jsonl")
	k := newTestKernel(t, ledgerPath)
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	body := []byte(`{"session_id":"bad-shell","permission_mode":"default","cwd":".","command":"echo hello","unexpected":true}`)
	resp, err := postJSONWithAuth(server.URL+"/tools/shell.exec", body)
	if err != nil {
		t.Fatalf("POST /tools/shell.exec failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if _, err := k.Session("bad-shell"); err != ErrSessionNotFound {
		t.Fatalf("Session error = %v, want ErrSessionNotFound", err)
	}
}

func TestUnapprovedMemoryCandidateIsNotRecalled(t *testing.T) {
	ledgerPath := filepath.Join(t.TempDir(), "events.jsonl")
	k := newTestKernel(t, ledgerPath)
	_, err := k.CreateMemoryCandidate(MemoryCandidateRequest{
		SessionID: "memory-source",
		Text:      "我偏好中文回答",
		SourceRef: "turn:memory-source",
	})
	if err != nil {
		t.Fatalf("CreateMemoryCandidate returned error: %v", err)
	}

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "memory-consumer",
		InputItems: []InputItem{{Type: "text", Text: "你记得我的回答偏好吗？"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if strings.Contains(resp.Final.Text, "我偏好中文回答") {
		t.Fatalf("unapproved memory was recalled in final text: %q", resp.Final.Text)
	}
}

func TestCreateMemoryCandidateRequiresSourceRef(t *testing.T) {
	k := newTestKernel(t, filepath.Join(t.TempDir(), "events.jsonl"))

	_, err := k.CreateMemoryCandidate(MemoryCandidateRequest{
		SessionID: "memory-source",
		Text:      "我偏好中文回答",
	})
	if err == nil {
		t.Fatal("CreateMemoryCandidate returned nil error without source_ref")
	}
}

func TestApprovedMemoryCandidateRecallsAcrossSessionsAfterRestart(t *testing.T) {
	ledgerPath := filepath.Join(t.TempDir(), "events.jsonl")
	k := newTestKernel(t, ledgerPath)
	candidate, err := k.CreateMemoryCandidate(MemoryCandidateRequest{
		SessionID: "memory-source",
		Text:      "我偏好中文回答",
		SourceRef: "turn:memory-source",
	})
	if err != nil {
		t.Fatalf("CreateMemoryCandidate returned error: %v", err)
	}

	restarted := newTestKernel(t, ledgerPath)
	approved, err := restarted.ApproveMemoryCandidate(candidate.CandidateID, testApprovalRequest("approval:memory-source"))
	if err != nil {
		t.Fatalf("ApproveMemoryCandidate returned error: %v", err)
	}
	if approved.Status != MemoryCandidateApproved {
		t.Fatalf("approved status = %q, want approved", approved.Status)
	}

	consumer := newTestKernel(t, ledgerPath)
	resp, err := consumer.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "memory-consumer",
		InputItems: []InputItem{{Type: "text", Text: "你记得我的回答偏好吗？"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if !strings.Contains(resp.Final.Text, "我偏好中文回答") {
		t.Fatalf("final text = %q, want recalled memory", resp.Final.Text)
	}

	sourceProjection, err := consumer.Session("memory-source")
	if err != nil {
		t.Fatalf("source Session returned error: %v", err)
	}
	if len(sourceProjection.MemoryCandidates) != 1 {
		t.Fatalf("len(MemoryCandidates) = %d, want 1", len(sourceProjection.MemoryCandidates))
	}
	if sourceProjection.MemoryCandidates[0].Status != MemoryCandidateApproved {
		t.Fatalf("candidate status = %q, want approved", sourceProjection.MemoryCandidates[0].Status)
	}
	if sourceProjection.MemoryCandidates[0].SourceRef != "turn:memory-source" {
		t.Fatalf("candidate source ref = %q, want turn:memory-source", sourceProjection.MemoryCandidates[0].SourceRef)
	}
	if sourceProjection.MemoryCandidates[0].ApprovalEvidenceRef != "approval:memory-source" {
		t.Fatalf("approval evidence ref = %q, want approval:memory-source", sourceProjection.MemoryCandidates[0].ApprovalEvidenceRef)
	}

	consumerProjection, err := consumer.Session("memory-consumer")
	if err != nil {
		t.Fatalf("consumer Session returned error: %v", err)
	}
	if len(consumerProjection.Turns) != 1 {
		t.Fatalf("len(Turns) = %d, want 1", len(consumerProjection.Turns))
	}
	if len(consumerProjection.Turns[0].RecalledMemories) != 1 {
		t.Fatalf("recalled memories = %+v, want one", consumerProjection.Turns[0].RecalledMemories)
	}
	if consumerProjection.Turns[0].RecalledMemories[0].Source != "turn:memory-source" {
		t.Fatalf("recall source = %q, want turn:memory-source", consumerProjection.Turns[0].RecalledMemories[0].Source)
	}
}

func TestHTTPMemoryCandidateApproveAndRecall(t *testing.T) {
	ledgerPath := filepath.Join(t.TempDir(), "events.jsonl")
	k := newTestKernel(t, ledgerPath)
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	candidatePayload, err := json.Marshal(MemoryCandidateRequest{
		SessionID: "http-memory-source",
		Text:      "我偏好中文回答",
		SourceRef: "turn:http-memory-source",
	})
	if err != nil {
		t.Fatalf("marshal candidate request: %v", err)
	}
	candidateResp, err := postJSONWithAuth(server.URL+"/memory/candidates", candidatePayload)
	if err != nil {
		t.Fatalf("POST /memory/candidates failed: %v", err)
	}
	defer candidateResp.Body.Close()
	if candidateResp.StatusCode != http.StatusOK {
		t.Fatalf("candidate status = %d, want 200", candidateResp.StatusCode)
	}
	var candidate MemoryCandidateProjection
	if err := json.NewDecoder(candidateResp.Body).Decode(&candidate); err != nil {
		t.Fatalf("decode candidate response: %v", err)
	}

	approvalPayload, err := json.Marshal(testApprovalRequest("approval:http-memory-source"))
	if err != nil {
		t.Fatalf("marshal approval request: %v", err)
	}
	approveResp, err := postJSONWithAuth(server.URL+"/memory/candidates/"+candidate.CandidateID+"/approve", approvalPayload)
	if err != nil {
		t.Fatalf("POST approve failed: %v", err)
	}
	defer approveResp.Body.Close()
	if approveResp.StatusCode != http.StatusOK {
		t.Fatalf("approve status = %d, want 200", approveResp.StatusCode)
	}
	var approved MemoryCandidateProjection
	if err := json.NewDecoder(approveResp.Body).Decode(&approved); err != nil {
		t.Fatalf("decode approved response: %v", err)
	}
	if approved.Status != MemoryCandidateApproved {
		t.Fatalf("approved status = %q, want approved", approved.Status)
	}

	turnPayload := []byte(`{"session_id":"http-memory-consumer","input_items":[{"type":"text","text":"你记得我的回答偏好吗？"}]}`)
	turnResp, err := postJSONWithAuth(server.URL+"/turn", turnPayload)
	if err != nil {
		t.Fatalf("POST /turn failed: %v", err)
	}
	defer turnResp.Body.Close()
	if turnResp.StatusCode != http.StatusOK {
		t.Fatalf("turn status = %d, want 200", turnResp.StatusCode)
	}
	var turn TurnResponse
	if err := json.NewDecoder(turnResp.Body).Decode(&turn); err != nil {
		t.Fatalf("decode turn response: %v", err)
	}
	if !strings.Contains(turn.Final.Text, "我偏好中文回答") {
		t.Fatalf("final text = %q, want recalled memory", turn.Final.Text)
	}
}

func TestHTTPMemoryCandidateListAndReadAfterRestart(t *testing.T) {
	ledgerPath := filepath.Join(t.TempDir(), "events.jsonl")
	k := newTestKernel(t, ledgerPath)
	server := httptest.NewServer(Handler(k))

	firstCandidate := createMemoryCandidateOverHTTP(t, server.URL, MemoryCandidateRequest{
		SessionID: "http-memory-source-one",
		Text:      "我偏好中文回答",
		SourceRef: "turn:http-memory-source-one",
	})
	secondCandidate := createMemoryCandidateOverHTTP(t, server.URL, MemoryCandidateRequest{
		SessionID: "http-memory-source-two",
		Text:      "我偏好短回答",
		SourceRef: "turn:http-memory-source-two",
	})
	approvalPayload, err := json.Marshal(testApprovalRequest("approval:http-memory-source-one"))
	if err != nil {
		t.Fatalf("marshal approval request: %v", err)
	}
	approveResp, err := postJSONWithAuth(server.URL+"/memory/candidates/"+firstCandidate.CandidateID+"/approve", approvalPayload)
	if err != nil {
		t.Fatalf("POST approve failed: %v", err)
	}
	approveResp.Body.Close()
	if approveResp.StatusCode != http.StatusOK {
		t.Fatalf("approve status = %d, want 200", approveResp.StatusCode)
	}
	server.Close()

	restarted := newTestKernel(t, ledgerPath)
	restartedServer := httptest.NewServer(Handler(restarted))
	defer restartedServer.Close()

	pendingResp, err := getWithAuth(restartedServer.URL + "/memory/candidates?status=pending")
	if err != nil {
		t.Fatalf("GET pending candidates failed: %v", err)
	}
	defer pendingResp.Body.Close()
	if pendingResp.StatusCode != http.StatusOK {
		t.Fatalf("pending status = %d, want 200", pendingResp.StatusCode)
	}
	var pending MemoryCandidateListResponse
	if err := json.NewDecoder(pendingResp.Body).Decode(&pending); err != nil {
		t.Fatalf("decode pending candidates: %v", err)
	}
	if len(pending.Items) != 1 || pending.Items[0].CandidateID != secondCandidate.CandidateID {
		t.Fatalf("pending candidates = %+v, want second candidate only", pending.Items)
	}
	if pending.Items[0].SourceRef != "turn:http-memory-source-two" {
		t.Fatalf("pending source ref = %q, want turn:http-memory-source-two", pending.Items[0].SourceRef)
	}

	readResp, err := getWithAuth(restartedServer.URL + "/memory/candidates/" + firstCandidate.CandidateID)
	if err != nil {
		t.Fatalf("GET memory candidate failed: %v", err)
	}
	defer readResp.Body.Close()
	if readResp.StatusCode != http.StatusOK {
		t.Fatalf("read status = %d, want 200", readResp.StatusCode)
	}
	var approved MemoryCandidateProjection
	if err := json.NewDecoder(readResp.Body).Decode(&approved); err != nil {
		t.Fatalf("decode approved candidate: %v", err)
	}
	if approved.Status != MemoryCandidateApproved {
		t.Fatalf("approved status = %q, want approved", approved.Status)
	}
	if approved.ApprovalEvidenceRef != "approval:http-memory-source-one" {
		t.Fatalf("approval evidence ref = %q, want approval:http-memory-source-one", approved.ApprovalEvidenceRef)
	}

	badStatusResp, err := getWithAuth(restartedServer.URL + "/memory/candidates?status=unknown")
	if err != nil {
		t.Fatalf("GET bad status failed: %v", err)
	}
	defer badStatusResp.Body.Close()
	if badStatusResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad status response = %d, want 400", badStatusResp.StatusCode)
	}
}

func TestHTTPApproveUnknownMemoryCandidateReturnsNotFound(t *testing.T) {
	k := newTestKernel(t, filepath.Join(t.TempDir(), "events.jsonl"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	approvalPayload, err := json.Marshal(testApprovalRequest("approval:missing"))
	if err != nil {
		t.Fatalf("marshal approval request: %v", err)
	}
	resp, err := postJSONWithAuth(server.URL+"/memory/candidates/missing/approve", approvalPayload)
	if err != nil {
		t.Fatalf("POST approve failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestHTTPApproveMemoryCandidateRejectsMissingEvidence(t *testing.T) {
	k := newTestKernel(t, filepath.Join(t.TempDir(), "events.jsonl"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	resp, err := postJSONWithAuth(server.URL+"/memory/candidates/anything/approve", []byte(`{"approval_authority":"runtime"}`))
	if err != nil {
		t.Fatalf("POST approve failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestHTTPReportsBlockedProvider(t *testing.T) {
	ledgerPath := filepath.Join(t.TempDir(), "events.jsonl")
	k, err := New(Config{
		LedgerPath:   ledgerPath,
		Provider:     NewOpenAICompatibleProvider(OpenAICompatibleConfig{}),
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	readyResp, err := http.Get(server.URL + "/ready")
	if err != nil {
		t.Fatalf("GET /ready failed: %v", err)
	}
	defer readyResp.Body.Close()
	var ready ReadyResponse
	if err := json.NewDecoder(readyResp.Body).Decode(&ready); err != nil {
		t.Fatalf("decode ready response: %v", err)
	}
	if ready.Status != "blocked" || ready.Provider.Status != "blocked" {
		t.Fatalf("ready = %+v, want blocked provider", ready)
	}

	body := []byte(`{"session_id":"blocked-session","input_items":[{"type":"text","text":"hello"}]}`)
	turnResp, err := postJSONWithAuth(server.URL+"/turn", body)
	if err != nil {
		t.Fatalf("POST /turn failed: %v", err)
	}
	defer turnResp.Body.Close()
	if turnResp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("turn status = %d, want 503", turnResp.StatusCode)
	}

	restarted := newTestKernelWithRuntimeToken(t, ledgerPath, testRuntimeToken)
	projection, err := restarted.Session("blocked-session")
	if err != nil {
		t.Fatalf("Session after provider failure returned error: %v", err)
	}
	if len(projection.Turns) != 1 {
		t.Fatalf("len(Turns) = %d, want 1", len(projection.Turns))
	}
	if projection.Turns[0].Status != "failed" {
		t.Fatalf("turn status = %q, want failed", projection.Turns[0].Status)
	}
	if projection.Turns[0].Error == nil || projection.Turns[0].Error.Code != "provider_unavailable" {
		t.Fatalf("turn error = %+v, want provider_unavailable", projection.Turns[0].Error)
	}
	if len(projection.Events) != 2 || projection.Events[0].Type != "turn.submitted" || projection.Events[1].Type != "turn.failed" {
		t.Fatalf("events = %+v, want submitted then failed", projection.Events)
	}
}

func TestOpenAICompatibleProviderReadyRequiresConfiguration(t *testing.T) {
	provider := NewOpenAICompatibleProvider(OpenAICompatibleConfig{})

	status := provider.Ready()
	if status.Status != "blocked" {
		t.Fatalf("status = %q, want blocked", status.Status)
	}
	if status.Reason == "" {
		t.Fatal("status reason is empty")
	}
}

func TestOpenAICompatibleProviderCompletesAgainstCompatibleServer(t *testing.T) {
	var sawAuth bool
	var sawPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		if r.Header.Get("Authorization") == "Bearer test-key" {
			sawAuth = true
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		var req chatCompletionRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if req.Model != "test-model" {
			t.Fatalf("model = %q, want test-model", req.Model)
		}
		if len(req.Messages) != 1 || req.Messages[0].Role != "user" || req.Messages[0].Content != "hello\nworld" {
			t.Fatalf("messages = %+v, want one joined user message", req.Messages)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"served-model","choices":[{"message":{"role":"assistant","content":"provider answer"}}]}`))
	}))
	defer server.Close()

	provider := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	})
	resp, err := provider.Complete(context.Background(), ModelRequest{
		InputItems: []InputItem{
			{Type: "text", Text: "hello"},
			{Type: "text", Text: "world"},
		},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if sawPath != "/chat/completions" {
		t.Fatalf("path = %q, want /chat/completions", sawPath)
	}
	if !sawAuth {
		t.Fatal("provider did not send expected bearer token")
	}
	if resp.Text != "provider answer" || resp.Model != "served-model" {
		t.Fatalf("response = %+v", resp)
	}
}

func TestKernelBuildsApprovedMemoryContextBeforeOpenAICompatibleProvider(t *testing.T) {
	var providerContent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		var req chatCompletionRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if len(req.Messages) != 1 {
			t.Fatalf("messages = %+v, want one user message", req.Messages)
		}
		providerContent = req.Messages[0].Content
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"served-model","choices":[{"message":{"role":"assistant","content":"provider answer"}}]}`))
	}))
	defer server.Close()

	ledgerPath := filepath.Join(t.TempDir(), "events.jsonl")
	k, err := New(Config{
		LedgerPath: ledgerPath,
		Provider: NewOpenAICompatibleProvider(OpenAICompatibleConfig{
			BaseURL: server.URL,
			APIKey:  "test-key",
			Model:   "test-model",
		}),
		RuntimeToken: testRuntimeToken,
		Clock: func() time.Time {
			return time.Date(2026, 6, 22, 1, 2, 3, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	candidate, err := k.CreateMemoryCandidate(MemoryCandidateRequest{
		SessionID: "provider-context-source",
		Text:      "prefer concise answers",
		SourceRef: "turn:provider-context-source",
	})
	if err != nil {
		t.Fatalf("CreateMemoryCandidate returned error: %v", err)
	}
	if _, err := k.ApproveMemoryCandidate(candidate.CandidateID, testApprovalRequest("approval:provider-context-source")); err != nil {
		t.Fatalf("ApproveMemoryCandidate returned error: %v", err)
	}

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "provider-context-consumer",
		InputItems: []InputItem{{Type: "text", Text: "Do you remember prefer concise answers?"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "provider answer" {
		t.Fatalf("final text = %q, want provider answer", resp.Final.Text)
	}
	if !strings.Contains(providerContent, "Approved memories:\n- prefer concise answers") {
		t.Fatalf("provider content = %q, want approved memory context", providerContent)
	}
	if !strings.Contains(providerContent, "Do you remember prefer concise answers?") {
		t.Fatalf("provider content = %q, want user text", providerContent)
	}

	projection, err := k.Session("provider-context-consumer")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Turns) != 1 || len(projection.Turns[0].RecalledMemories) != 1 {
		t.Fatalf("projection turns = %+v, want recalled memory", projection.Turns)
	}
	if projection.Turns[0].RecalledMemories[0].Source != "turn:provider-context-source" {
		t.Fatalf("recall source = %q, want turn:provider-context-source", projection.Turns[0].RecalledMemories[0].Source)
	}
}

func TestLiveOpenAICompatibleProviderThroughKernel(t *testing.T) {
	if os.Getenv("GENESIS_LIVE_PROVIDER") != "1" {
		t.Skip("set GENESIS_LIVE_PROVIDER=1 with Genesis provider env to run live provider smoke")
	}
	baseURL := strings.TrimSpace(os.Getenv("GENESIS_PROVIDER_BASE_URL"))
	model := strings.TrimSpace(os.Getenv("GENESIS_PROVIDER_MODEL"))
	apiKeyEnv := strings.TrimSpace(os.Getenv("GENESIS_PROVIDER_API_KEY_ENV"))
	if apiKeyEnv == "" {
		apiKeyEnv = "GENESIS_PROVIDER_API_KEY"
	}
	apiKey := strings.TrimSpace(os.Getenv(apiKeyEnv))
	switch {
	case baseURL == "":
		t.Fatal("GENESIS_PROVIDER_BASE_URL is required for live provider smoke")
	case model == "":
		t.Fatal("GENESIS_PROVIDER_MODEL is required for live provider smoke")
	case apiKey == "":
		t.Fatalf("%s is required for live provider smoke", apiKeyEnv)
	}

	k, err := New(Config{
		LedgerPath: filepath.Join(t.TempDir(), "events.jsonl"),
		Provider: NewOpenAICompatibleProvider(OpenAICompatibleConfig{
			BaseURL: baseURL,
			APIKey:  apiKey,
			Model:   model,
		}),
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	ready := k.Ready()
	if ready.Status != "ok" {
		t.Fatalf("ready = %+v, want ok", ready)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	resp, err := k.SubmitTurn(ctx, TurnRequest{
		SessionID:  "live-provider-smoke",
		InputItems: []InputItem{{Type: "text", Text: "Reply with a short confirmation that Genesis live provider smoke succeeded."}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if strings.TrimSpace(resp.Final.Text) == "" {
		t.Fatal("live provider returned empty final text")
	}
	projection, err := k.Session("live-provider-smoke")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Turns) != 1 || projection.Turns[0].Status != "completed" {
		t.Fatalf("projection turns = %+v, want one completed turn", projection.Turns)
	}
}

const testRuntimeToken = "test-runtime-token"

func testApprovalRequest(evidenceRef string) MemoryApprovalRequest {
	return MemoryApprovalRequest{
		ApprovalAuthority:   "runtime:test",
		ApprovalReason:      "approved in test",
		ApprovalEvidenceRef: evidenceRef,
	}
}

func createMemoryCandidateOverHTTP(t *testing.T, serverURL string, req MemoryCandidateRequest) MemoryCandidateProjection {
	t.Helper()
	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal candidate request: %v", err)
	}
	resp, err := postJSONWithAuth(serverURL+"/memory/candidates", payload)
	if err != nil {
		t.Fatalf("POST /memory/candidates failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("candidate status = %d, want 200", resp.StatusCode)
	}
	var candidate MemoryCandidateProjection
	if err := json.NewDecoder(resp.Body).Decode(&candidate); err != nil {
		t.Fatalf("decode candidate response: %v", err)
	}
	return candidate
}

func newTestKernel(t *testing.T, ledgerPath string) *Kernel {
	t.Helper()
	return newTestKernelWithRuntimeTokenAndPolicy(t, ledgerPath, testRuntimeToken, ToolPolicy{
		PermissionMode: PermissionModePlan,
	})
}

func newTestKernelWithPolicy(t *testing.T, ledgerPath string, policy ToolPolicy) *Kernel {
	t.Helper()
	return newTestKernelWithRuntimeTokenAndPolicy(t, ledgerPath, testRuntimeToken, policy)
}

func newTestKernelWithRuntimeToken(t *testing.T, ledgerPath string, token string) *Kernel {
	t.Helper()
	return newTestKernelWithRuntimeTokenAndPolicy(t, ledgerPath, token, ToolPolicy{
		PermissionMode: PermissionModePlan,
	})
}

func newTestKernelWithRuntimeTokenAndPolicy(t *testing.T, ledgerPath string, token string, policy ToolPolicy) *Kernel {
	t.Helper()
	k, err := New(Config{
		LedgerPath:   ledgerPath,
		Provider:     FakeProvider{},
		RuntimeToken: token,
		ToolPolicy:   policy,
		Clock: func() time.Time {
			return time.Date(2026, 6, 22, 1, 2, 3, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	return k
}

func postJSONWithAuth(url string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+testRuntimeToken)
	req.Header.Set("Content-Type", "application/json")
	return http.DefaultClient.Do(req)
}

func getWithAuth(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+testRuntimeToken)
	return http.DefaultClient.Do(req)
}

func writeFileCommand(filename string, value string) string {
	if runtime.GOOS == "windows" {
		return "Set-Content -LiteralPath " + filename + " -Value " + value + " -NoNewline"
	}
	return "printf " + value + " > " + filename
}

func echoCommand(value string) string {
	if runtime.GOOS == "windows" {
		return "Write-Output " + value
	}
	return "printf " + value
}

func secretEchoCommand() string {
	if runtime.GOOS == "windows" {
		return `Write-Output 'GENESIS_PROVIDER_API_KEY=sk-secret123'; Write-Output 'Authorization: Bearer tokentest123456'; Write-Output '{"api_key":"sk-jsonsecret"}'`
	}
	return `printf '%s\n' 'GENESIS_PROVIDER_API_KEY=sk-secret123' 'Authorization: Bearer tokentest123456' '{"api_key":"sk-jsonsecret"}'`
}

func createDirectoryLinkForTest(t *testing.T, target string, link string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		cmd := exec.Command("cmd.exe", "/c", "mklink", "/J", link, target)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("create junction failed: %v; output=%s", err, string(output))
		}
		t.Cleanup(func() {
			_ = exec.Command("cmd.exe", "/c", "rmdir", link).Run()
		})
		return
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("create symlink failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Remove(link)
	})
}
