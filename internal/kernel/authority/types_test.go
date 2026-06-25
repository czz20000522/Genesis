package authority

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestApprovalProjectionJSONShape(t *testing.T) {
	expiresAt := time.Date(2026, 6, 25, 10, 15, 0, 0, time.UTC)
	payload, err := json.Marshal(ApprovalProjection{
		ApprovalID:      "approval_1",
		SessionID:       "session_1",
		Status:          ApprovalStatusPending,
		Tool:            "shell_exec",
		PolicySnapshot:  ApprovalPolicySnapshot{PermissionMode: "default", AuthorityPolicy: "controlled_workspace", SandboxProfile: "controlled_workspace", ApprovalPolicy: "on_request", ExecutorAdapter: "local"},
		Effect:          ApprovalEffectSummary{Tool: "shell_exec", ExecutionKind: "foreground", SideEffect: "workspace_write", CWD: "D:/repo", TimeoutSec: 30},
		RequestedAt:     expiresAt.Add(-15 * time.Minute),
		ExpiresAt:       expiresAt,
		ToolCallEventID: "evt_tool",
	})
	if err != nil {
		t.Fatalf("marshal ApprovalProjection: %v", err)
	}
	text := string(payload)
	for _, want := range []string{
		`"approval_id":"approval_1"`,
		`"status":"pending"`,
		`"permission_mode":"default"`,
		`"sandbox_profile":"controlled_workspace"`,
		`"tool":"shell_exec"`,
		`"timeout_sec":30`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("approval payload = %s, want %s", text, want)
		}
	}
}

func TestSandboxReadinessProjectionJSONShape(t *testing.T) {
	payload, err := json.Marshal(SandboxReadinessProjection{
		SandboxReadinessID: "sandbox_ready_1",
		SessionID:          "session_1",
		OperationID:        "op_1",
		SandboxProfile:     "os_workspace",
		WorkspaceRoot:      "D:/repo",
		ExecutorAdapter:    "local",
		Status:             SandboxReadinessUnavailable,
		UnavailableReason:  "adapter_missing",
		CreatedAt:          time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("marshal SandboxReadinessProjection: %v", err)
	}
	text := string(payload)
	for _, want := range []string{
		`"sandbox_readiness_id":"sandbox_ready_1"`,
		`"sandbox_profile":"os_workspace"`,
		`"status":"unavailable"`,
		`"unavailable_reason":"adapter_missing"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("sandbox readiness payload = %s, want %s", text, want)
		}
	}
}
