package kernel

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestHTTPShellExecAndSessionProjection(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.jsonl")
	workspace := testTempDir(t)
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
	resp, err := postJSONWithAuth(server.URL+"/tools/shell_exec", payload)
	if err != nil {
		t.Fatalf("POST /tools/shell_exec failed: %v", err)
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

func TestHTTPShellExecLongTimeoutReturnsManagedJobReceipt(t *testing.T) {
	workspace := testTempDir(t)
	ledgerPath := filepath.Join(testTempDir(t), "events.jsonl")
	k := newTestKernelWithPolicy(t, ledgerPath, ToolPolicy{
		PermissionMode: PermissionModeYolo,
		WorkspaceRoot:  workspace,
	})
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	payload, err := json.Marshal(map[string]interface{}{
		"session_id":      "http-shell-managed",
		"cwd":             workspace,
		"command":         echoCommand("http-managed"),
		"timeout_sec":     maxForegroundShellTimeoutSec + 1,
		"idempotency_key": "http-managed-1",
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	resp, err := postJSONWithAuth(server.URL+"/tools/shell_exec", payload)
	if err != nil {
		t.Fatalf("POST /tools/shell_exec failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", resp.StatusCode)
	}
	var job JobProjection
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		t.Fatalf("decode managed job response: %v", err)
	}
	if job.JobID == "" || job.Status != "running" || job.Tool != "shell_exec" {
		t.Fatalf("job response = %+v, want running shell_exec job", job)
	}
	if job.IdempotencyKey != "http-managed-1" {
		t.Fatalf("job idempotency key = %q, want http-managed-1", job.IdempotencyKey)
	}
	if strings.TrimSpace(job.Receipt) == "" {
		t.Fatalf("job response = %+v, want receipt", job)
	}

	completed := waitForSessionJobStatus(t, k, "http-shell-managed", job.JobID, "completed")
	if completed.ExitCode == nil || *completed.ExitCode != 0 || !strings.Contains(completed.Stdout, "http-managed") {
		t.Fatalf("completed job = %+v, want terminal output", completed)
	}
	projection, err := k.Session("http-shell-managed")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Operations) != 0 {
		t.Fatalf("operations = %+v, want direct managed path to create no operation", projection.Operations)
	}
	if got := countSessionEventType(projection.Events, "job.started"); got != 1 {
		t.Fatalf("job.started count = %d, want 1", got)
	}

	secondResp, err := postJSONWithAuth(server.URL+"/tools/shell_exec", payload)
	if err != nil {
		t.Fatalf("second POST /tools/shell_exec failed: %v", err)
	}
	defer secondResp.Body.Close()
	if secondResp.StatusCode != http.StatusOK {
		t.Fatalf("second status = %d, want 200 for existing job projection", secondResp.StatusCode)
	}
	var second JobProjection
	if err := json.NewDecoder(secondResp.Body).Decode(&second); err != nil {
		t.Fatalf("decode second managed job response: %v", err)
	}
	if second.JobID != job.JobID {
		t.Fatalf("second job id = %q, want %q", second.JobID, job.JobID)
	}
	projection, err = k.Session("http-shell-managed")
	if err != nil {
		t.Fatalf("Session after second request returned error: %v", err)
	}
	if got := countSessionEventType(projection.Events, "job.started"); got != 1 {
		t.Fatalf("job.started count after idempotent retry = %d, want 1", got)
	}
}

func TestHTTPShellExecLongTimeoutDoesNotBypassDefaultSandbox(t *testing.T) {
	workspace := testTempDir(t)
	outside := filepath.Join(testTempDir(t), "managed-bypass.txt")
	k := newTestKernelWithPolicy(t, filepath.Join(testTempDir(t), "events.jsonl"), ToolPolicy{
		PermissionMode: PermissionModeDefault,
		WorkspaceRoot:  workspace,
	})
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	payload, err := json.Marshal(map[string]interface{}{
		"session_id":      "http-managed-default-blocked",
		"cwd":             workspace,
		"command":         writeFileCommand(outside, "bypass"),
		"timeout_sec":     maxForegroundShellTimeoutSec + 1,
		"idempotency_key": "http-managed-default-blocked",
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	resp, err := postJSONWithAuth(server.URL+"/tools/shell_exec", payload)
	if err != nil {
		t.Fatalf("POST /tools/shell_exec failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 blocked operation", resp.StatusCode)
	}
	var operation OperationProjection
	if err := json.NewDecoder(resp.Body).Decode(&operation); err != nil {
		t.Fatalf("decode operation response: %v", err)
	}
	if operation.Status != "blocked" {
		t.Fatalf("operation = %+v, want blocked", operation)
	}
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Fatalf("outside file stat error = %v, want file not created", err)
	}
	projection, err := k.Session("http-managed-default-blocked")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Jobs) != 0 {
		t.Fatalf("jobs = %+v, want no managed job when default sandbox blocks command", projection.Jobs)
	}
}

func TestHTTPShellExecManagedJobRetryPreservesTerminalOutputProjection(t *testing.T) {
	workspace := testTempDir(t)
	k := newTestKernelWithPolicy(t, filepath.Join(testTempDir(t), "events.jsonl"), ToolPolicy{
		PermissionMode: PermissionModeYolo,
		WorkspaceRoot:  workspace,
	})
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	payload, err := json.Marshal(map[string]interface{}{
		"session_id":      "http-managed-redaction",
		"cwd":             workspace,
		"command":         secretEchoCommand(),
		"timeout_sec":     maxForegroundShellTimeoutSec + 1,
		"idempotency_key": "http-managed-redaction",
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	resp, err := postJSONWithAuth(server.URL+"/tools/shell_exec", payload)
	if err != nil {
		t.Fatalf("POST /tools/shell_exec failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", resp.StatusCode)
	}
	var started JobProjection
	if err := json.NewDecoder(resp.Body).Decode(&started); err != nil {
		t.Fatalf("decode started job: %v", err)
	}
	_ = waitForSessionJobStatus(t, k, "http-managed-redaction", started.JobID, "completed")

	retryResp, err := postJSONWithAuth(server.URL+"/tools/shell_exec", payload)
	if err != nil {
		t.Fatalf("retry POST /tools/shell_exec failed: %v", err)
	}
	defer retryResp.Body.Close()
	if retryResp.StatusCode != http.StatusOK {
		t.Fatalf("retry status = %d, want 200", retryResp.StatusCode)
	}
	body, err := io.ReadAll(retryResp.Body)
	if err != nil {
		t.Fatalf("read retry response: %v", err)
	}
	for _, want := range []string{"sk-secret123", "tokentest123456", "sk-jsonsecret"} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("managed job retry lost terminal output %q: %s", want, string(body))
		}
	}
	if strings.Contains(string(body), "[REDACTED]") {
		t.Fatalf("managed job retry should not use lossy redaction: %s", string(body))
	}
}

func TestHTTPShellExecRejectsExplicitZeroTimeout(t *testing.T) {
	workspace := testTempDir(t)
	k := newTestKernelWithPolicy(t, filepath.Join(testTempDir(t), "events.jsonl"), ToolPolicy{
		PermissionMode: PermissionModeDefault,
		WorkspaceRoot:  workspace,
	})
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	body := []byte(`{"session_id":"http-shell-zero-timeout","cwd":` + strconv.Quote(workspace) + `,"command":` + strconv.Quote(echoCommand("zero")) + `,"timeout_sec":0}`)
	resp, err := postJSONWithAuth(server.URL+"/tools/shell_exec", body)
	if err != nil {
		t.Fatalf("POST /tools/shell_exec failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if _, err := k.Session("http-shell-zero-timeout"); err != ErrSessionNotFound {
		t.Fatalf("Session error = %v, want ErrSessionNotFound", err)
	}
}

func TestHTTPShellExecIdempotencyKeyDoesNotCrossFromOperationToJob(t *testing.T) {
	workspace := testTempDir(t)
	k := newTestKernelWithPolicy(t, filepath.Join(testTempDir(t), "events.jsonl"), ToolPolicy{
		PermissionMode: PermissionModeYolo,
		WorkspaceRoot:  workspace,
	})
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	firstPayload, err := json.Marshal(map[string]interface{}{
		"session_id":      "http-shell-operation-first",
		"cwd":             workspace,
		"command":         echoCommand("operation-first"),
		"idempotency_key": "same-effect-key",
	})
	if err != nil {
		t.Fatalf("marshal first request: %v", err)
	}
	firstResp, err := postJSONWithAuth(server.URL+"/tools/shell_exec", firstPayload)
	if err != nil {
		t.Fatalf("first POST /tools/shell_exec failed: %v", err)
	}
	defer firstResp.Body.Close()
	if firstResp.StatusCode != http.StatusOK {
		t.Fatalf("first status = %d, want 200", firstResp.StatusCode)
	}
	var first OperationProjection
	if err := json.NewDecoder(firstResp.Body).Decode(&first); err != nil {
		t.Fatalf("decode first operation: %v", err)
	}

	secondPayload, err := json.Marshal(map[string]interface{}{
		"session_id":      "http-shell-operation-first",
		"cwd":             workspace,
		"command":         longRunningShellCommand(30),
		"timeout_sec":     maxForegroundShellTimeoutSec + 1,
		"idempotency_key": "same-effect-key",
	})
	if err != nil {
		t.Fatalf("marshal second request: %v", err)
	}
	secondResp, err := postJSONWithAuth(server.URL+"/tools/shell_exec", secondPayload)
	if err != nil {
		t.Fatalf("second POST /tools/shell_exec failed: %v", err)
	}
	defer secondResp.Body.Close()
	if secondResp.StatusCode != http.StatusOK {
		t.Fatalf("second status = %d, want 200 existing operation", secondResp.StatusCode)
	}
	var second OperationProjection
	if err := json.NewDecoder(secondResp.Body).Decode(&second); err != nil {
		t.Fatalf("decode second operation: %v", err)
	}
	if second.OperationID != first.OperationID || second.Status != "completed" {
		t.Fatalf("second operation = %+v, want original operation %s", second, first.OperationID)
	}
	projection, err := k.Session("http-shell-operation-first")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Jobs) != 0 {
		t.Fatalf("jobs = %+v, want no managed job after operation-owned idempotency key", projection.Jobs)
	}
}

func TestHTTPShellExecIdempotencyKeyDoesNotCrossFromJobToOperation(t *testing.T) {
	workspace := testTempDir(t)
	k := newTestKernelWithPolicy(t, filepath.Join(testTempDir(t), "events.jsonl"), ToolPolicy{
		PermissionMode: PermissionModeYolo,
		WorkspaceRoot:  workspace,
	})
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	firstPayload, err := json.Marshal(map[string]interface{}{
		"session_id":      "http-shell-job-first",
		"cwd":             workspace,
		"command":         echoCommand("job-first"),
		"timeout_sec":     maxForegroundShellTimeoutSec + 1,
		"idempotency_key": "same-effect-key",
	})
	if err != nil {
		t.Fatalf("marshal first request: %v", err)
	}
	firstResp, err := postJSONWithAuth(server.URL+"/tools/shell_exec", firstPayload)
	if err != nil {
		t.Fatalf("first POST /tools/shell_exec failed: %v", err)
	}
	defer firstResp.Body.Close()
	if firstResp.StatusCode != http.StatusAccepted {
		t.Fatalf("first status = %d, want 202", firstResp.StatusCode)
	}
	var first JobProjection
	if err := json.NewDecoder(firstResp.Body).Decode(&first); err != nil {
		t.Fatalf("decode first job: %v", err)
	}

	secondPayload, err := json.Marshal(map[string]interface{}{
		"session_id":      "http-shell-job-first",
		"cwd":             workspace,
		"command":         echoCommand("should-not-run"),
		"idempotency_key": "same-effect-key",
	})
	if err != nil {
		t.Fatalf("marshal second request: %v", err)
	}
	secondResp, err := postJSONWithAuth(server.URL+"/tools/shell_exec", secondPayload)
	if err != nil {
		t.Fatalf("second POST /tools/shell_exec failed: %v", err)
	}
	defer secondResp.Body.Close()
	if secondResp.StatusCode != http.StatusOK {
		t.Fatalf("second status = %d, want 200 existing job", secondResp.StatusCode)
	}
	var second JobProjection
	if err := json.NewDecoder(secondResp.Body).Decode(&second); err != nil {
		t.Fatalf("decode second job: %v", err)
	}
	if second.JobID != first.JobID {
		t.Fatalf("second job id = %q, want %q", second.JobID, first.JobID)
	}
	projection, err := k.Session("http-shell-job-first")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Operations) != 0 {
		t.Fatalf("operations = %+v, want no operation after job-owned idempotency key", projection.Operations)
	}
	if got := countSessionEventType(projection.Events, "job.started"); got != 1 {
		t.Fatalf("job.started count = %d, want 1", got)
	}
}

func TestHTTPShellExecIdempotencyKeyReturnsExistingOperation(t *testing.T) {
	workspace := testTempDir(t)
	k := newTestKernelWithPolicy(t, filepath.Join(testTempDir(t), "events.jsonl"), ToolPolicy{
		PermissionMode: PermissionModeDefault,
		WorkspaceRoot:  workspace,
	})
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	firstPayload, err := json.Marshal(ShellExecRequest{
		SessionID:      "http-shell-idempotent",
		CWD:            workspace,
		Command:        writeFileCommand("idempotent-http.txt", "first"),
		IdempotencyKey: "http-shell-write-1",
	})
	if err != nil {
		t.Fatalf("marshal first shell request: %v", err)
	}
	firstResp, err := postJSONWithAuth(server.URL+"/tools/shell_exec", firstPayload)
	if err != nil {
		t.Fatalf("first POST /tools/shell_exec failed: %v", err)
	}
	defer firstResp.Body.Close()
	if firstResp.StatusCode != http.StatusOK {
		t.Fatalf("first status = %d, want 200", firstResp.StatusCode)
	}
	var first OperationProjection
	if err := json.NewDecoder(firstResp.Body).Decode(&first); err != nil {
		t.Fatalf("decode first shell response: %v", err)
	}

	secondPayload, err := json.Marshal(ShellExecRequest{
		SessionID:      "http-shell-idempotent",
		CWD:            workspace,
		Command:        writeFileCommand("idempotent-http.txt", "second"),
		IdempotencyKey: "http-shell-write-1",
	})
	if err != nil {
		t.Fatalf("marshal second shell request: %v", err)
	}
	secondResp, err := postJSONWithAuth(server.URL+"/tools/shell_exec", secondPayload)
	if err != nil {
		t.Fatalf("second POST /tools/shell_exec failed: %v", err)
	}
	defer secondResp.Body.Close()
	if secondResp.StatusCode != http.StatusOK {
		t.Fatalf("second status = %d, want 200", secondResp.StatusCode)
	}
	var second OperationProjection
	if err := json.NewDecoder(secondResp.Body).Decode(&second); err != nil {
		t.Fatalf("decode second shell response: %v", err)
	}
	if second.OperationID != first.OperationID {
		t.Fatalf("second operation id = %q, want %q", second.OperationID, first.OperationID)
	}
	content, err := os.ReadFile(filepath.Join(workspace, "idempotent-http.txt"))
	if err != nil {
		t.Fatalf("read idempotent http output: %v", err)
	}
	if string(content) != "first" {
		t.Fatalf("file content = %q, want first", string(content))
	}

	sessionResp, err := getWithAuth(server.URL + "/sessions/http-shell-idempotent")
	if err != nil {
		t.Fatalf("GET /sessions failed: %v", err)
	}
	defer sessionResp.Body.Close()
	var projection SessionProjection
	if err := json.NewDecoder(sessionResp.Body).Decode(&projection); err != nil {
		t.Fatalf("decode session response: %v", err)
	}
	if len(projection.Operations) != 1 || len(projection.Events) != 2 {
		t.Fatalf("projection = %+v, want one operation and two events", projection)
	}
}

func TestHTTPShellExecStaleRunningIdempotencyKeyReturnsFailedOperation(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.jsonl")
	workspace := testTempDir(t)
	k := newTestKernelWithPolicy(t, ledgerPath, ToolPolicy{
		PermissionMode: PermissionModeDefault,
		WorkspaceRoot:  workspace,
	})
	stale := OperationProjection{
		OperationID:    "op-http-stale-running",
		SessionID:      "http-shell-stale",
		Tool:           "shell_exec",
		IdempotencyKey: "http-stale-key",
		Status:         "running",
		PermissionMode: PermissionModeDefault,
		CWD:            workspace,
		Command:        writeFileCommand("http-stale.txt", "first"),
		StartedAt:      time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC),
	}
	if err := k.appendOperationEvent(stale); err != nil {
		t.Fatalf("append stale operation: %v", err)
	}

	restarted := newTestKernelWithPolicy(t, ledgerPath, ToolPolicy{
		PermissionMode: PermissionModeDefault,
		WorkspaceRoot:  workspace,
	})
	server := httptest.NewServer(Handler(restarted))
	defer server.Close()

	payload, err := json.Marshal(ShellExecRequest{
		SessionID:      "http-shell-stale",
		CWD:            workspace,
		Command:        writeFileCommand("http-stale.txt", "second"),
		IdempotencyKey: "http-stale-key",
	})
	if err != nil {
		t.Fatalf("marshal stale shell request: %v", err)
	}
	resp, err := postJSONWithAuth(server.URL+"/tools/shell_exec", payload)
	if err != nil {
		t.Fatalf("POST /tools/shell_exec failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 with failed operation projection", resp.StatusCode)
	}
	var operation OperationProjection
	if err := json.NewDecoder(resp.Body).Decode(&operation); err != nil {
		t.Fatalf("decode operation response: %v", err)
	}
	if operation.Status != "failed" || operation.BlockedReason != "stale_running_operation" {
		t.Fatalf("operation = %+v, want failed stale operation", operation)
	}
	if _, err := os.Stat(filepath.Join(workspace, "http-stale.txt")); !os.IsNotExist(err) {
		t.Fatalf("stale HTTP retry executed file effect, stat err = %v", err)
	}
}

func TestHTTPRejectsUnknownShellFields(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.jsonl")
	k := newTestKernel(t, ledgerPath)
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	body := []byte(`{"session_id":"bad-shell","permission_mode":"default","cwd":".","command":"echo hello","unexpected":true}`)
	resp, err := postJSONWithAuth(server.URL+"/tools/shell_exec", body)
	if err != nil {
		t.Fatalf("POST /tools/shell_exec failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if _, err := k.Session("bad-shell"); err != ErrSessionNotFound {
		t.Fatalf("Session error = %v, want ErrSessionNotFound", err)
	}
}
