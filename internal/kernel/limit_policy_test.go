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
	"strings"
	"testing"
)

func TestShellTimeoutPolicyDrivesManifestSchedulingAndInspection(t *testing.T) {
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     FakeProvider{},
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeYolo,
			WorkspaceRoot:  testTempDir(t),
			SandboxProfile: SandboxProfileHost,
		},
		ShellTimeoutPolicy: ShellTimeoutPolicy{
			DefaultForegroundTimeoutSec: 7,
			ForegroundTimeoutCapSec:     9,
			ManagedJobThresholdSec:      9,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	shellSpec, ok := toolSpecByName(k.toolGateway().ToolManifest(), "shell_exec")
	if !ok {
		t.Fatalf("tool manifest = %+v, want shell_exec", k.toolGateway().ToolManifest())
	}
	timeoutSpec := shellTimeoutSchema(t, shellSpec)
	description := strings.ToLower(stringValue(timeoutSpec["description"]))
	for _, want := range []string{"7 seconds", "above 9"} {
		if !strings.Contains(description, want) {
			t.Fatalf("timeout description = %q, want policy value %q", description, want)
		}
	}
	for _, forbidden := range []string{"30 seconds", "above 180"} {
		if strings.Contains(description, forbidden) {
			t.Fatalf("timeout description = %q, still contains old policy text %q", description, forbidden)
		}
	}

	foreground := k.shellExecAccessPlan("shell_exec", ".", 9)
	if foreground.EffectClass != ToolEffectClassWorkspaceWrite || foreground.ParallelPolicy != ToolParallelPolicySerialFence {
		t.Fatalf("foreground access plan = %+v, want foreground workspace write", foreground)
	}
	managed := k.shellExecAccessPlan("shell_exec", ".", 10)
	if managed.EffectClass != ToolEffectClassProcessStart || managed.ParallelPolicy != ToolParallelPolicyBackgroundAfterAdmission {
		t.Fatalf("managed access plan = %+v, want background-after-admission process start", managed)
	}

	capabilities := k.Capabilities()
	if capabilities.ShellTimeoutPolicy.DefaultForegroundTimeoutSec != 7 ||
		capabilities.ShellTimeoutPolicy.ForegroundTimeoutCapSec != 9 ||
		capabilities.ShellTimeoutPolicy.ManagedJobThresholdSec != 9 {
		t.Fatalf("capabilities shell timeout policy = %+v, want configured policy", capabilities.ShellTimeoutPolicy)
	}
	assertLimitClass(t, capabilities.Limits, "shell.foreground_timeout_cap_sec", LimitClassHardSafetyGuard, false)
	assertLimitClass(t, capabilities.Limits, "shell.default_foreground_timeout_sec", LimitClassShellTimeoutPolicy, false)

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "shell-policy-inspection",
		InputItems: []InputItem{{Type: "text", Text: "inspect runtime shell policy"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	inspection, err := k.ContextInspection(resp.TurnID)
	if err != nil {
		t.Fatalf("ContextInspection returned error: %v", err)
	}
	if inspection.Runtime == nil {
		t.Fatalf("runtime inspection is nil")
	}
	if inspection.Runtime.ShellTimeoutPolicy.DefaultForegroundTimeoutSec != 7 ||
		inspection.Runtime.ShellTimeoutPolicy.ForegroundTimeoutCapSec != 9 {
		t.Fatalf("context runtime shell policy = %+v, want configured policy", inspection.Runtime.ShellTimeoutPolicy)
	}
	assertLimitClass(t, inspection.Runtime.Limits, "shell.managed_job_threshold_sec", LimitClassShellTimeoutPolicy, false)
}

func TestDirectHTTPShellUsesConfiguredTimeoutPolicy(t *testing.T) {
	workspace := testTempDir(t)
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     FakeProvider{},
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeYolo,
			WorkspaceRoot:  workspace,
			SandboxProfile: SandboxProfileHost,
		},
		ShellTimeoutPolicy: ShellTimeoutPolicy{
			DefaultForegroundTimeoutSec: 5,
			ForegroundTimeoutCapSec:     6,
			ManagedJobThresholdSec:      6,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	status, body := postShellRequest(t, server.URL, testRuntimeToken, map[string]interface{}{
		"session_id": "http-shell-policy-foreground",
		"cwd":        workspace,
		"command":    "echo foreground",
	})
	if status != http.StatusOK {
		t.Fatalf("foreground status = %d body=%s, want 200", status, body)
	}
	var operation OperationProjection
	if err := json.Unmarshal(body, &operation); err != nil {
		t.Fatalf("decode foreground operation: %v body=%s", err, body)
	}
	if operation.TimeoutSec != 5 || operation.Status != "completed" {
		t.Fatalf("operation = %+v, want default timeout from policy and completed foreground", operation)
	}

	status, body = postShellRequest(t, server.URL, testRuntimeToken, map[string]interface{}{
		"session_id":  "http-shell-policy-managed",
		"cwd":         workspace,
		"command":     "echo managed",
		"timeout_sec": 7,
	})
	if status != http.StatusAccepted {
		t.Fatalf("managed status = %d body=%s, want 202", status, body)
	}
	var job JobProjection
	if err := json.Unmarshal(body, &job); err != nil {
		t.Fatalf("decode managed job: %v body=%s", err, body)
	}
	if job.TimeoutSec != 7 || job.Status != "running" {
		t.Fatalf("job = %+v, want managed job above configured threshold", job)
	}
}

func TestKernelLimitClassificationCoversActiveBudgetGuardAndProjectionCaps(t *testing.T) {
	k := newTestKernelWithBudgetAndResources(t, filepath.Join(testTempDir(t), "events.jsonl"), BudgetPolicy{
		ModelToolRoundBudget:  5,
		ModelToolRoundCeiling: 8,
	}, nil)

	capabilities := k.Capabilities()
	assertLimitClass(t, capabilities.Limits, "budget.model_tool_round_budget", LimitClassBudgetLease, false)
	assertLimitClass(t, capabilities.Limits, "budget.model_tool_round_ceiling", LimitClassBudgetLease, false)
	assertLimitClass(t, capabilities.Limits, "tool_loop.repeated_failure_threshold", LimitClassHardSafetyGuard, false)
	assertLimitClass(t, capabilities.Limits, "tool_loop.repeated_write_success_threshold", LimitClassHardSafetyGuard, false)
	assertLimitClass(t, capabilities.Limits, "projection.shell_output_max_bytes", LimitClassProjectionOutputCap, false)
	assertLimitClass(t, capabilities.Limits, "provider.transient_retry_attempts", LimitClassProviderRetryRepairCap, false)
	assertLimitClass(t, capabilities.Limits, "provider.visible_final_repair_attempts", LimitClassProviderRetryRepairCap, false)

	for _, limit := range capabilities.Limits {
		if strings.TrimSpace(limit.Name) == "" ||
			strings.TrimSpace(limit.Class) == "" ||
			strings.TrimSpace(limit.Owner) == "" ||
			strings.TrimSpace(limit.OverridePolicy) == "" {
			t.Fatalf("limit classification = %+v, want name/class/owner/override policy", limit)
		}
	}
}

func TestSourceSnapshotPolicyIsInspectableRuntimeLimit(t *testing.T) {
	dir := testTempDir(t)
	k, err := New(Config{
		LedgerPath:   filepath.Join(dir, "events.jsonl"),
		Provider:     FakeProvider{},
		RuntimeToken: testRuntimeToken,
		SourceSnapshotPolicy: SourceSnapshotPolicy{
			MaxFileCount:                77,
			MaxPerFileUncompressedBytes: 8 * 1024 * 1024,
			MaxTotalUncompressedBytes:   31 * 1024 * 1024,
			DefaultTreeEntries:          33,
			MaxTreeEntries:              99,
			DefaultReadBytes:            2048,
			MaxReadBytes:                8192,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	capabilities := k.Capabilities()
	assertLimitEffectiveValue(t, capabilities.Limits, "source_snapshot.max_file_count", 77)
	assertLimitEffectiveValue(t, capabilities.Limits, "source_snapshot.max_per_file_uncompressed_bytes", 8*1024*1024)
	assertLimitEffectiveValue(t, capabilities.Limits, "source_snapshot.max_total_uncompressed_bytes", 31*1024*1024)
	assertLimitEffectiveValue(t, capabilities.Limits, "source_snapshot.default_tree_entries", 33)
	assertLimitEffectiveValue(t, capabilities.Limits, "source_snapshot.max_tree_entries", 99)
	assertLimitEffectiveValue(t, capabilities.Limits, "source_snapshot.default_read_bytes", 2048)
	assertLimitEffectiveValue(t, capabilities.Limits, "source_snapshot.max_read_bytes", 8192)
	assertLimitClass(t, capabilities.Limits, "source_snapshot.max_total_uncompressed_bytes", LimitClassHardSafetyGuard, false)
	assertLimitClass(t, capabilities.Limits, "source_snapshot.default_read_bytes", LimitClassProjectionOutputCap, false)
}

func TestProjectionCapPreservesOwnerContentAndOnlyBoundsProjection(t *testing.T) {
	secretShapedOutput := "begin " + strings.Repeat("x", maxShellOutputBytes+128) + " api_key=sk-local-user-content-secret end"
	captured := captureBytes([]byte(secretShapedOutput), 64)
	if !captured.Truncated {
		t.Fatalf("captured output was not truncated")
	}
	if strings.Contains(captured.Text, "[REDACTED]") {
		t.Fatalf("captured output = %q, budget projection must not use redaction markers", captured.Text)
	}
	if captured.OriginalBytes != len([]byte(secretShapedOutput)) || captured.OmittedBytes == 0 {
		t.Fatalf("captured metadata = %+v, want original and omitted byte evidence", captured)
	}
	if !strings.Contains(secretShapedOutput, "sk-local-user-content-secret") {
		t.Fatalf("owner source content was unexpectedly modified")
	}
}

func TestShellTimeoutPolicyRejectsInvalidTimeoutBeforeSideEffects(t *testing.T) {
	workspace := testTempDir(t)
	target := filepath.Join(workspace, "should-not-exist.txt")
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     FakeProvider{},
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeYolo,
			WorkspaceRoot:  workspace,
			SandboxProfile: SandboxProfileHost,
		},
		ShellTimeoutPolicy: ShellTimeoutPolicy{
			DefaultForegroundTimeoutSec: 4,
			ForegroundTimeoutCapSec:     6,
			ManagedJobThresholdSec:      6,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	_, err = k.ExecShell(context.Background(), ShellExecRequest{
		SessionID:  "invalid-timeout-before-effect",
		CWD:        workspace,
		Command:    "Set-Content should-not-exist.txt bad",
		TimeoutSec: -1,
	})
	if err == nil {
		t.Fatalf("ExecShell returned nil error, want invalid timeout")
	}
	if _, statErr := os.Stat(target); statErr == nil {
		t.Fatalf("invalid timeout created %s", target)
	}
}

func shellTimeoutSchema(t *testing.T, spec ToolSpec) map[string]interface{} {
	t.Helper()
	properties, ok := spec.InputSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("input schema = %+v, want properties", spec.InputSchema)
	}
	timeoutSpec, ok := properties["timeout_sec"].(map[string]interface{})
	if !ok {
		t.Fatalf("properties = %+v, want timeout_sec", properties)
	}
	return timeoutSpec
}

func stringValue(value interface{}) string {
	text, _ := value.(string)
	return text
}

func assertLimitClass(t *testing.T, limits []RuntimeLimitProjection, name string, class string, modelVisible bool) {
	t.Helper()
	for _, limit := range limits {
		if limit.Name != name {
			continue
		}
		if limit.Class != class || limit.ModelVisible != modelVisible {
			t.Fatalf("limit %s = %+v, want class=%s model_visible=%v", name, limit, class, modelVisible)
		}
		return
	}
	t.Fatalf("limits = %+v, want %s", limits, name)
}

func assertLimitEffectiveValue(t *testing.T, limits []RuntimeLimitProjection, name string, value int) {
	t.Helper()
	for _, limit := range limits {
		if limit.Name == name {
			if limit.EffectiveValue != value {
				t.Fatalf("limit %s = %+v, want effective value %d", name, limit, value)
			}
			return
		}
	}
	t.Fatalf("limits = %+v, want %s", limits, name)
}

func toolSpecByName(specs []ToolSpec, name string) (ToolSpec, bool) {
	for _, spec := range specs {
		if spec.Name == name {
			return spec, true
		}
	}
	return ToolSpec{}, false
}

func postShellRequest(t *testing.T, baseURL string, token string, payload map[string]interface{}) (int, []byte) {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+"/tools/shell_exec", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /tools/shell_exec: %v", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	return resp.StatusCode, responseBody
}
