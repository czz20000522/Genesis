package kernel

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestShellExecHostEnvironmentDoesNotInheritSecretShapedDaemonEnv(t *testing.T) {
	t.Setenv("GENESIS_SHELL_ENV_SECRET", "sk-shell-env-secret")
	t.Setenv("GENESIS_SHELL_ENV_TOKEN", "token-shell-env-secret")
	k := newTestKernelWithPolicy(t, filepath.Join(testTempDir(t), "events.sqlite"), ToolPolicy{
		PermissionMode: PermissionModeYolo,
		WorkspaceRoot:  testTempDir(t),
	})

	operation, err := k.ExecShell(context.Background(), ShellExecRequest{
		SessionID: "shell-env-foreground",
		Command:   shellEnvironmentReadCommand("GENESIS_SHELL_ENV_SECRET", "GENESIS_SHELL_ENV_TOKEN"),
		CWD:       testTempDir(t),
	})
	if err != nil {
		t.Fatalf("ExecShell returned error: %v", err)
	}
	if operation.Status != "completed" {
		t.Fatalf("operation status = %q, want completed; operation = %+v", operation.Status, operation)
	}
	for _, forbidden := range []string{"sk-shell-env-secret", "token-shell-env-secret"} {
		if strings.Contains(operation.Stdout, forbidden) || strings.Contains(operation.Stderr, forbidden) {
			t.Fatalf("shell output leaked daemon env %q: stdout=%q stderr=%q", forbidden, operation.Stdout, operation.Stderr)
		}
	}
}

func TestManagedJobHostEnvironmentDoesNotInheritSecretShapedDaemonEnv(t *testing.T) {
	t.Setenv("GENESIS_SHELL_JOB_SECRET", "sk-managed-job-secret")
	arguments, err := json.Marshal(map[string]interface{}{
		"command":     shellEnvironmentReadCommand("GENESIS_SHELL_JOB_SECRET"),
		"cwd":         testTempDir(t),
		"timeout_sec": maxForegroundShellTimeoutSec + 1,
	})
	if err != nil {
		t.Fatalf("marshal shell args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{{
			ToolCallID: "call_managed_env",
			Name:       "shell_exec",
			Arguments:  json.RawMessage(arguments),
		}},
		final: "managed env observed",
	}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.sqlite"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeYolo,
			WorkspaceRoot:  testTempDir(t),
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "shell-env-managed-job",
		InputItems: []InputItem{{Type: "text", Text: "run managed job env probe"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	projection, err := k.Session("shell-env-managed-job")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Jobs) != 1 {
		t.Fatalf("jobs = %+v, want one managed job", projection.Jobs)
	}
	completed := waitForSessionJobStatus(t, k, "shell-env-managed-job", projection.Jobs[0].JobID, "completed")
	if strings.Contains(completed.Stdout, "sk-managed-job-secret") || strings.Contains(completed.Stderr, "sk-managed-job-secret") {
		t.Fatalf("managed job leaked daemon env: stdout=%q stderr=%q", completed.Stdout, completed.Stderr)
	}
}

func TestShellEnvironmentPolicyKeepsOrdinaryHostShellUsable(t *testing.T) {
	k := newTestKernelWithPolicy(t, filepath.Join(testTempDir(t), "events.sqlite"), ToolPolicy{
		PermissionMode: PermissionModeYolo,
		WorkspaceRoot:  testTempDir(t),
	})

	operation, err := k.ExecShell(context.Background(), ShellExecRequest{
		SessionID: "shell-env-positive",
		Command:   echoCommand("GENESIS_SHELL_ENV_POLICY_OK"),
		CWD:       testTempDir(t),
	})
	if err != nil {
		t.Fatalf("ExecShell returned error: %v", err)
	}
	if operation.Status != "completed" || !strings.Contains(operation.Stdout, "GENESIS_SHELL_ENV_POLICY_OK") {
		t.Fatalf("operation = %+v, want ordinary host shell command to remain usable", operation)
	}
}

func shellEnvironmentReadCommand(names ...string) string {
	if runtime.GOOS == "windows" {
		parts := make([]string, 0, len(names))
		for _, name := range names {
			parts = append(parts, "$env:"+name)
		}
		return strings.Join(parts, "; ")
	}
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, "printf '%s\\n' \"$"+name+"\"")
	}
	return strings.Join(parts, "; ")
}
