package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExecShellPlanBlocksMutatingCommand(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	workspace := testTempDir(t)
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
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	workspace := testTempDir(t)
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
	if operation.AuthorityPolicy != AuthorityPolicyWorkspaceWrite ||
		operation.SandboxProfile != SandboxProfileControlledWorkspace ||
		operation.ApprovalPolicy != ApprovalPolicyNever {
		t.Fatalf("operation policy = %+v, want resolved default policy profile", operation)
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
	if projection.Operations[0].AuthorityPolicy != AuthorityPolicyWorkspaceWrite ||
		projection.Operations[0].SandboxProfile != SandboxProfileControlledWorkspace ||
		projection.Operations[0].ApprovalPolicy != ApprovalPolicyNever {
		t.Fatalf("projected operation policy = %+v, want resolved default policy profile", projection.Operations[0])
	}
	if len(projection.Events) != 2 || projection.Events[0].OperationID != operation.OperationID || projection.Events[1].OperationID != operation.OperationID {
		t.Fatalf("events = %+v, want operation event", projection.Events)
	}
}

func TestExecShellIdempotencyKeySurvivesRestartWithoutRepeatingEffect(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	workspace := testTempDir(t)
	k := newTestKernelWithPolicy(t, ledgerPath, ToolPolicy{
		PermissionMode: PermissionModeDefault,
		WorkspaceRoot:  workspace,
	})

	first, err := k.ExecShell(context.Background(), ShellExecRequest{
		SessionID:      "shell-idempotent",
		CWD:            workspace,
		Command:        writeFileCommand("idempotent.txt", "first"),
		IdempotencyKey: "shell-write-1",
	})
	if err != nil {
		t.Fatalf("first ExecShell returned error: %v", err)
	}
	if first.Status != "completed" {
		t.Fatalf("first status = %q, want completed; stderr=%q", first.Status, first.Stderr)
	}

	restarted := newTestKernelWithPolicy(t, ledgerPath, ToolPolicy{
		PermissionMode: PermissionModeDefault,
		WorkspaceRoot:  workspace,
	})
	second, err := restarted.ExecShell(context.Background(), ShellExecRequest{
		SessionID:      "shell-idempotent",
		CWD:            workspace,
		Command:        writeFileCommand("idempotent.txt", "second"),
		IdempotencyKey: "shell-write-1",
	})
	if err != nil {
		t.Fatalf("second ExecShell returned error: %v", err)
	}
	if second.OperationID != first.OperationID {
		t.Fatalf("second operation id = %q, want %q", second.OperationID, first.OperationID)
	}
	if second.Command != first.Command {
		t.Fatalf("second command = %q, want original command %q", second.Command, first.Command)
	}
	content, err := os.ReadFile(filepath.Join(workspace, "idempotent.txt"))
	if err != nil {
		t.Fatalf("read idempotent output: %v", err)
	}
	if string(content) != "first" {
		t.Fatalf("file content = %q, want first", string(content))
	}

	projection, err := restarted.Session("shell-idempotent")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Operations) != 1 {
		t.Fatalf("len(Operations) = %d, want 1", len(projection.Operations))
	}
	if len(projection.Events) != 2 {
		t.Fatalf("len(Events) = %d, want 2 operation events", len(projection.Events))
	}
	if projection.Operations[0].IdempotencyKey != "shell-write-1" {
		t.Fatalf("projected idempotency key = %q, want shell-write-1", projection.Operations[0].IdempotencyKey)
	}
}

func TestExecShellStaleRunningIdempotencyKeyFailsClosedAfterRestart(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	workspace := testTempDir(t)
	k := newTestKernelWithPolicy(t, ledgerPath, ToolPolicy{
		PermissionMode: PermissionModeDefault,
		WorkspaceRoot:  workspace,
	})
	startedAt := time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC)
	stale := OperationProjection{
		OperationID:    "op-stale-running",
		SessionID:      "shell-stale-idempotent",
		Tool:           "shell_exec",
		IdempotencyKey: "stale-key",
		Status:         "running",
		PermissionMode: PermissionModeDefault,
		CWD:            workspace,
		Command:        writeFileCommand("stale.txt", "first"),
		StartedAt:      startedAt,
	}
	if err := k.appendOperationEvent(stale); err != nil {
		t.Fatalf("append stale running operation: %v", err)
	}

	restarted := newTestKernelWithPolicy(t, ledgerPath, ToolPolicy{
		PermissionMode: PermissionModeDefault,
		WorkspaceRoot:  workspace,
	})
	recovered, err := restarted.ExecShell(context.Background(), ShellExecRequest{
		SessionID:      "shell-stale-idempotent",
		CWD:            workspace,
		Command:        writeFileCommand("stale.txt", "second"),
		IdempotencyKey: "stale-key",
	})
	if err != nil {
		t.Fatalf("ExecShell returned error: %v", err)
	}
	if recovered.OperationID != stale.OperationID {
		t.Fatalf("operation id = %q, want stale operation id %q", recovered.OperationID, stale.OperationID)
	}
	if recovered.Status != "failed" {
		t.Fatalf("status = %q, want failed stale operation", recovered.Status)
	}
	if recovered.BlockedReason != "stale_running_operation" {
		t.Fatalf("blocked reason = %q, want stale_running_operation", recovered.BlockedReason)
	}
	if _, err := os.Stat(filepath.Join(workspace, "stale.txt")); !os.IsNotExist(err) {
		t.Fatalf("stale retry executed file effect, stat err = %v", err)
	}

	projection, err := restarted.Session("shell-stale-idempotent")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Operations) != 1 {
		t.Fatalf("len(Operations) = %d, want 1", len(projection.Operations))
	}
	if projection.Operations[0].Status != "failed" || projection.Operations[0].BlockedReason != "stale_running_operation" {
		t.Fatalf("operation projection = %+v, want failed stale operation", projection.Operations[0])
	}
	if len(projection.Events) != 2 || projection.Events[0].Type != "operation.running" || projection.Events[1].Type != "operation.failed" {
		t.Fatalf("events = %+v, want running then failed recovery event", projection.Events)
	}
}

func TestExecShellBlockedOperationIsIdempotent(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	k := newTestKernelWithPolicy(t, ledgerPath, ToolPolicy{
		PermissionMode: PermissionModePlan,
	})

	first, err := k.ExecShell(context.Background(), ShellExecRequest{
		SessionID:      "shell-blocked-idempotent",
		CWD:            testTempDir(t),
		Command:        "echo first",
		IdempotencyKey: "blocked-1",
	})
	if err != nil {
		t.Fatalf("first ExecShell returned error: %v", err)
	}
	second, err := k.ExecShell(context.Background(), ShellExecRequest{
		SessionID:      "shell-blocked-idempotent",
		CWD:            testTempDir(t),
		Command:        "echo second",
		IdempotencyKey: "blocked-1",
	})
	if err != nil {
		t.Fatalf("second ExecShell returned error: %v", err)
	}
	if second.OperationID != first.OperationID {
		t.Fatalf("second operation id = %q, want %q", second.OperationID, first.OperationID)
	}
	if second.Status != "blocked" || second.BlockedReason != "blocked_by_permission_mode=plan" {
		t.Fatalf("second operation = %+v, want original blocked operation", second)
	}
	projection, err := k.Session("shell-blocked-idempotent")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Operations) != 1 || len(projection.Events) != 1 {
		t.Fatalf("projection = %+v, want one blocked operation event", projection)
	}
}

func TestExecShellRejectsInvalidIdempotencyKey(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))

	_, err := k.ExecShell(context.Background(), ShellExecRequest{
		SessionID:      "shell-bad-idempotency",
		CWD:            testTempDir(t),
		Command:        "echo hello",
		IdempotencyKey: "bad key",
	})
	if err == nil {
		t.Fatal("ExecShell returned nil error for invalid idempotency key")
	}
	if _, err := k.Session("shell-bad-idempotency"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("Session error = %v, want ErrSessionNotFound", err)
	}
}

func TestExecShellDefaultBlocksOutsideWorkspace(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	root := testTempDir(t)
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
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	root := testTempDir(t)
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
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	root := testTempDir(t)
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

func TestExecShellDefaultBlocksHardlinkAlias(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	root := testTempDir(t)
	workspace := filepath.Join(root, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	outsideFile := filepath.Join(root, "outside-hardlink.txt")
	if err := os.WriteFile(outsideFile, []byte("outside-secret"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	aliasPath := filepath.Join(workspace, "alias.txt")
	if err := os.Link(outsideFile, aliasPath); err != nil {
		t.Skipf("create hardlink failed: %v", err)
	}
	k := newTestKernelWithPolicy(t, ledgerPath, ToolPolicy{
		PermissionMode: PermissionModeDefault,
		WorkspaceRoot:  workspace,
	})

	readOperation, err := k.ExecShell(context.Background(), ShellExecRequest{
		SessionID: "shell-hardlink-read",
		CWD:       workspace,
		Command:   readMissingFileCommand("alias.txt"),
	})
	if err != nil {
		t.Fatalf("read hardlink ExecShell returned error: %v", err)
	}
	if readOperation.Status != "blocked" {
		t.Fatalf("read status = %q, want blocked; stdout=%q stderr=%q", readOperation.Status, readOperation.Stdout, readOperation.Stderr)
	}
	if readOperation.BlockedReason != "command_path_unsafe_link" {
		t.Fatalf("read blocked reason = %q, want command_path_unsafe_link", readOperation.BlockedReason)
	}

	writeOperation, err := k.ExecShell(context.Background(), ShellExecRequest{
		SessionID: "shell-hardlink-write",
		CWD:       workspace,
		Command:   writeFileCommand("alias.txt", "mutated"),
	})
	if err != nil {
		t.Fatalf("write hardlink ExecShell returned error: %v", err)
	}
	if writeOperation.Status != "blocked" {
		t.Fatalf("write status = %q, want blocked; stdout=%q stderr=%q", writeOperation.Status, writeOperation.Stdout, writeOperation.Stderr)
	}
	if writeOperation.BlockedReason != "command_path_unsafe_link" {
		t.Fatalf("write blocked reason = %q, want command_path_unsafe_link", writeOperation.BlockedReason)
	}
	content, err := os.ReadFile(outsideFile)
	if err != nil {
		t.Fatalf("read outside file: %v", err)
	}
	if string(content) != "outside-secret" {
		t.Fatalf("outside hardlink target mutated to %q", string(content))
	}
}

func TestExecShellDefaultBlocksRawShellAndEnvironmentAccess(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	workspace := testTempDir(t)
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

func TestExecShellPreservesTerminalVisibleContentInLocalProjection(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	workspace := testTempDir(t)
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
	ledgerEvents, err := k.loadEvents()
	if err != nil {
		t.Fatalf("load ledger events: %v", err)
	}
	ledgerData, err := json.Marshal(ledgerEvents)
	if err != nil {
		t.Fatalf("marshal ledger events: %v", err)
	}
	for _, leaked := range []string{"sk-secret123", "tokentest123456", "sk-jsonsecret"} {
		if !strings.Contains(operation.Command+operation.Stdout+operation.Stderr, leaked) {
			t.Fatalf("operation projection lost terminal-visible content %q: %+v", leaked, operation)
		}
		if !strings.Contains(string(ledgerData), leaked) {
			t.Fatalf("ledger lost raw evidence %q: %s", leaked, string(ledgerData))
		}
	}
	if strings.Contains(operation.Command+operation.Stdout+operation.Stderr, "[REDACTED]") {
		t.Fatalf("operation projection should use budgeted content, not lossy redaction: %+v", operation)
	}

	session, err := k.Session("shell-redaction")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	sessionJSON, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("marshal session projection: %v", err)
	}
	for _, leaked := range []string{"sk-secret123", "tokentest123456", "sk-jsonsecret"} {
		if !strings.Contains(string(sessionJSON), leaked) {
			t.Fatalf("session projection lost shell content %q: %s", leaked, string(sessionJSON))
		}
	}
	if strings.Contains(string(sessionJSON), "[REDACTED]") {
		t.Fatalf("session projection should not use lossy redaction for shell output: %s", string(sessionJSON))
	}
}

func TestExecShellReportsHeadTailTruncationMetadata(t *testing.T) {
	workspace := testTempDir(t)
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.sqlite"),
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeYolo,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	operation, err := k.ExecShell(context.Background(), ShellExecRequest{
		SessionID:      "head-tail-truncation",
		CWD:            workspace,
		Command:        longStdoutStderrCommand(),
		IdempotencyKey: "call_head_tail_truncation",
	})
	if err != nil {
		t.Fatalf("ExecShell returned error: %v", err)
	}
	if operation.Status != "completed" {
		t.Fatalf("operation status = %q, want completed; stderr=%q", operation.Status, operation.Stderr)
	}
	payload := operationJSONMap(t, operation)
	assertBoolMapValue(t, payload, "stdout_truncated", true)
	assertBoolMapValue(t, payload, "stderr_truncated", true)
	assertStringMapValue(t, payload, "output_truncation", "head_tail")
	if len([]byte(operation.Stdout)) > maxShellOutputBytes {
		t.Fatalf("stdout bytes = %d, want <= %d", len([]byte(operation.Stdout)), maxShellOutputBytes)
	}
	if len([]byte(operation.Stderr)) > maxShellOutputBytes {
		t.Fatalf("stderr bytes = %d, want <= %d", len([]byte(operation.Stderr)), maxShellOutputBytes)
	}
	if !strings.Contains(operation.Stdout, "GENESIS_STDOUT_HEAD") || !strings.Contains(operation.Stdout, "GENESIS_STDOUT_TAIL") {
		t.Fatalf("stdout = %q, want head and tail markers", operation.Stdout)
	}
	if !strings.Contains(operation.Stderr, "GENESIS_STDERR_HEAD") || !strings.Contains(operation.Stderr, "GENESIS_STDERR_TAIL") {
		t.Fatalf("stderr = %q, want head and tail markers", operation.Stderr)
	}
	assertHeadTailOmissionMarker(t, "stdout", operation.Stdout, "GENESIS_STDOUT_HEAD", "GENESIS_STDOUT_TAIL")
	assertHeadTailOmissionMarker(t, "stderr", operation.Stderr, "GENESIS_STDERR_HEAD", "GENESIS_STDERR_TAIL")
	assertMapNumberGreaterThan(t, payload, "stdout_original_bytes", len([]byte(operation.Stdout)))
	assertMapNumberGreaterThan(t, payload, "stderr_original_bytes", len([]byte(operation.Stderr)))
	assertMapNumberGreaterThan(t, payload, "stdout_omitted_bytes", 0)
	assertMapNumberGreaterThan(t, payload, "stderr_omitted_bytes", 0)
}

func TestExecShellControlledReadFailureDoesNotExposeAbsolutePath(t *testing.T) {
	workspace := testTempDir(t)
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.sqlite"),
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeDefault,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	operation, err := k.ExecShell(context.Background(), ShellExecRequest{
		SessionID:      "controlled-read-failure",
		CWD:            workspace,
		Command:        readMissingFileCommand("missing.txt"),
		IdempotencyKey: "controlled-read-failure",
	})
	if err != nil {
		t.Fatalf("ExecShell returned error: %v", err)
	}
	if operation.Status != "failed" {
		t.Fatalf("operation status = %q, want failed", operation.Status)
	}
	if operation.Stderr == "" {
		t.Fatal("stderr is empty, want bounded command failure")
	}
	for _, forbidden := range pathLeakVariants(workspace) {
		if strings.Contains(operation.Stderr, forbidden) {
			t.Fatalf("stderr = %q, must not expose workspace path %q", operation.Stderr, forbidden)
		}
	}
}
