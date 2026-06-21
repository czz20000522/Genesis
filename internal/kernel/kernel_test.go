package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestApprovedMemoryCandidateRecallsAcrossSessionsAfterRestart(t *testing.T) {
	ledgerPath := filepath.Join(t.TempDir(), "events.jsonl")
	k := newTestKernel(t, ledgerPath)
	candidate, err := k.CreateMemoryCandidate(MemoryCandidateRequest{
		SessionID: "memory-source",
		Text:      "我偏好中文回答",
	})
	if err != nil {
		t.Fatalf("CreateMemoryCandidate returned error: %v", err)
	}

	restarted := newTestKernel(t, ledgerPath)
	approved, err := restarted.ApproveMemoryCandidate(candidate.CandidateID)
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
}

func TestHTTPMemoryCandidateApproveAndRecall(t *testing.T) {
	ledgerPath := filepath.Join(t.TempDir(), "events.jsonl")
	k := newTestKernel(t, ledgerPath)
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	candidatePayload, err := json.Marshal(MemoryCandidateRequest{
		SessionID: "http-memory-source",
		Text:      "我偏好中文回答",
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

	approveResp, err := postEmptyWithAuth(server.URL + "/memory/candidates/" + candidate.CandidateID + "/approve")
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

func TestHTTPApproveUnknownMemoryCandidateReturnsNotFound(t *testing.T) {
	k := newTestKernel(t, filepath.Join(t.TempDir(), "events.jsonl"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	resp, err := postEmptyWithAuth(server.URL + "/memory/candidates/missing/approve")
	if err != nil {
		t.Fatalf("POST approve failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestHTTPApproveMemoryCandidateRejectsBody(t *testing.T) {
	k := newTestKernel(t, filepath.Join(t.TempDir(), "events.jsonl"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	req, err := http.NewRequest(http.MethodPost, server.URL+"/memory/candidates/anything/approve", strings.NewReader(`{"unexpected":true}`))
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+testRuntimeToken)
	resp, err := http.DefaultClient.Do(req)
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

const testRuntimeToken = "test-runtime-token"

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

func postEmptyWithAuth(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+testRuntimeToken)
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
