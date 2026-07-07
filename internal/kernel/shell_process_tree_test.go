package kernel

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
)

func TestForegroundShellTimeoutTerminatesDescendantProcessTree(t *testing.T) {
	requireProcessTreeShellSupport(t)
	workspace := testTempDir(t)
	marker := filepath.Join(workspace, "timeout-descendant-survived.txt")
	ready := filepath.Join(workspace, "timeout-descendant-ready.txt")
	k := newTestKernelWithPolicy(t, filepath.Join(testTempDir(t), "events.sqlite"), ToolPolicy{
		PermissionMode: PermissionModeYolo,
		WorkspaceRoot:  workspace,
	})

	operation, err := k.ExecShell(context.Background(), ShellExecRequest{
		SessionID:  "foreground-timeout-process-tree",
		CWD:        workspace,
		Command:    descendantMarkerCommand(marker, ready, 6),
		TimeoutSec: 3,
	})
	if err != nil {
		t.Fatalf("ExecShell returned error: %v", err)
	}
	if operation.Status != "failed" || !operation.TimedOut || operation.TimeoutReason != foregroundTimeoutReason {
		t.Fatalf("operation = %+v, want foreground timeout outcome", operation)
	}
	if !fileExists(ready) {
		t.Fatalf("ready marker %q missing; test command did not launch descendant before timeout", ready)
	}
	assertFileDoesNotAppear(t, marker, 7*time.Second)
}

func TestForegroundShellInterruptHandsOffDescendantProcessTree(t *testing.T) {
	requireProcessTreeShellSupport(t)
	workspace := testTempDir(t)
	marker := filepath.Join(workspace, "interrupt-descendant-survived.txt")
	ready := filepath.Join(workspace, "interrupt-descendant-ready.txt")
	arguments, err := json.Marshal(map[string]interface{}{
		"cwd":         workspace,
		"command":     descendantMarkerCommand(marker, ready, 4),
		"timeout_sec": 30,
	})
	if err != nil {
		t.Fatalf("marshal shell args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{{
			ToolCallID: "call_interrupt_process_tree",
			Name:       "shell_exec",
			Arguments:  json.RawMessage(arguments),
		}},
		final: "must not reach final provider step",
	}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.sqlite"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeYolo,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer k.Close()

	resultCh := make(chan submitTurnResult, 1)
	go func() {
		resp, err := k.SubmitTurn(context.Background(), TurnRequest{
			SessionID:  "foreground-interrupt-process-tree",
			InputItems: []InputItem{{Type: "text", Text: "run foreground process tree until interrupted"}},
		})
		resultCh <- submitTurnResult{response: resp, err: err}
	}()
	waitForSessionEventType(t, k, "foreground-interrupt-process-tree", "operation.running")
	waitForFile(t, ready, 5*time.Second)

	if _, err := k.InterruptSession("foreground-interrupt-process-tree", TurnInterruptRequest{Reason: "stop foreground process tree"}); err != nil {
		t.Fatalf("InterruptSession returned error: %v", err)
	}
	result := waitSubmitTurnResult(t, resultCh)
	if !errors.Is(result.err, ErrTurnInterrupted) {
		t.Fatalf("SubmitTurn error = %v, want ErrTurnInterrupted", result.err)
	}
	waitForFile(t, marker, 5*time.Second)
	projection, err := k.Session("foreground-interrupt-process-tree")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if got := countSessionEventType(projection.Events, "job.started"); got != 1 {
		t.Fatalf("job.started count = %d, want handoff managed job; events=%+v", got, projection.Events)
	}
	if got := countSessionEventType(projection.Events, "operation.completed"); got != 0 {
		t.Fatalf("operation.completed count = %d, handoff must not double-write operation terminal", got)
	}
	if len(projection.Operations) != 1 || projection.Operations[0].InterruptReason != foregroundAttachedManagedJobReason {
		t.Fatalf("operations = %+v, want foreground attached operation", projection.Operations)
	}
}

func descendantMarkerCommand(markerPath string, readyPath string, childDelaySeconds int) string {
	if runtime.GOOS == "windows" {
		childScript := fmt.Sprintf(
			"Start-Sleep -Seconds %d; Set-Content -LiteralPath %s -Value 'survived'",
			childDelaySeconds,
			powershellSingleQuoted(markerPath),
		)
		return fmt.Sprintf(
			"Start-Process -FilePath pwsh.exe -ArgumentList @('-NoProfile','-NonInteractive','-EncodedCommand','%s') -WindowStyle Hidden; Set-Content -LiteralPath %s -Value 'ready'; Start-Sleep -Seconds 30",
			powershellEncodedCommand(childScript),
			powershellSingleQuoted(readyPath),
		)
	}
	childScript := fmt.Sprintf("sleep %d; printf %%s survived > %s", childDelaySeconds, shellSingleQuoted(markerPath))
	return fmt.Sprintf("sh -c %s & printf %%s ready > %s; sleep 30", shellSingleQuoted(childScript), shellSingleQuoted(readyPath))
}

func requireProcessTreeShellSupport(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("pwsh.exe"); err != nil {
			t.Skipf("pwsh.exe unavailable: %v", err)
		}
		return
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skipf("sh unavailable: %v", err)
	}
}

func powershellEncodedCommand(script string) string {
	encoded := utf16.Encode([]rune(script))
	buf := make([]byte, len(encoded)*2)
	for i, r := range encoded {
		binary.LittleEndian.PutUint16(buf[i*2:], r)
	}
	return base64.StdEncoding.EncodeToString(buf)
}

func powershellSingleQuoted(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func shellSingleQuoted(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func waitForFile(t *testing.T, path string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fileExists(path) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("file %q did not appear within %s", path, timeout)
}

func assertFileDoesNotAppear(t *testing.T, path string, duration time.Duration) {
	t.Helper()
	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		if fileExists(path) {
			content, _ := os.ReadFile(path)
			t.Fatalf("descendant process survived kernel termination and wrote %q: %q", path, string(content))
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
