package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestLocalForegroundInterruptAttachesRealShellAsManagedJob(t *testing.T) {
	requireProcessTreeShellSupport(t)
	workspace := testTempDir(t)
	ready := filepath.Join(workspace, "handoff-ready.txt")
	command := foregroundHandoffCompletionCommand(ready)
	k, provider, result := submitLocalForegroundShellAndInterrupt(t, "local-foreground-handoff-completes", workspace, command, ready)
	if !errors.Is(result.err, ErrTurnInterrupted) {
		t.Fatalf("SubmitTurn error = %v, want ErrTurnInterrupted", result.err)
	}
	if len(provider.Requests()) != 1 {
		t.Fatalf("provider requests = %d, want no provider step after foreground handoff", len(provider.Requests()))
	}

	projection, err := k.Session("local-foreground-handoff-completes")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if got := countSessionEventType(projection.Events, "job.started"); got != 1 {
		t.Fatalf("job.started count = %d, want foreground handoff managed job; events=%+v", got, projection.Events)
	}
	if got := countSessionEventType(projection.Events, "operation.completed"); got != 0 {
		t.Fatalf("operation.completed count = %d, handoff process must not double-write operation terminal", got)
	}
	if len(projection.Operations) != 1 || projection.Operations[0].Status != "interrupted" || projection.Operations[0].InterruptReason != foregroundAttachedManagedJobReason {
		t.Fatalf("operations = %+v, want interrupted handoff receipt", projection.Operations)
	}
	if projection.Operations[0].ExitCode != nil {
		t.Fatalf("interrupted operation exit code = %d, handoff receipt must not claim command exit", *projection.Operations[0].ExitCode)
	}
	toolResult := requireToolResultPayload(t, projection)
	if toolResult["status"] != "managed_job_started" {
		t.Fatalf("tool result payload = %+v, want managed job receipt", toolResult)
	}
	if len(projection.Jobs) != 1 || projection.Jobs[0].Status != "running" {
		t.Fatalf("jobs = %+v, want running managed job after handoff", projection.Jobs)
	}
	if projection.Jobs[0].SourceOperationID != projection.Operations[0].OperationID {
		t.Fatalf("job source operation = %q, want %q", projection.Jobs[0].SourceOperationID, projection.Operations[0].OperationID)
	}
	jobID := projection.Jobs[0].JobID

	completed := waitForSessionJobStatus(t, k, "local-foreground-handoff-completes", jobID, "completed")
	if completed.ExitCode == nil || *completed.ExitCode != 0 {
		t.Fatalf("completed job = %+v, want zero exit code", completed)
	}
	if !strings.Contains(completed.Stdout, "handoff-after-interrupt") || !strings.Contains(completed.Stdout, "handoff-complete") {
		t.Fatalf("completed stdout = %q, want output produced after foreground interrupt", completed.Stdout)
	}
	if got := countSessionEventType(mustSessionProjection(t, k, "local-foreground-handoff-completes").Events, "job.completed"); got != 1 {
		t.Fatalf("job.completed count = %d, want exactly one job terminal fact", got)
	}
}

func TestLocalForegroundHandoffJobCancelTerminatesRealProcess(t *testing.T) {
	requireProcessTreeShellSupport(t)
	workspace := testTempDir(t)
	ready := filepath.Join(workspace, "handoff-cancel-ready.txt")
	command := foregroundHandoffLongRunningCommand(ready)
	k, _, result := submitLocalForegroundShellAndInterrupt(t, "local-foreground-handoff-cancel", workspace, command, ready)
	if !errors.Is(result.err, ErrTurnInterrupted) {
		t.Fatalf("SubmitTurn error = %v, want ErrTurnInterrupted", result.err)
	}
	projection, err := k.Session("local-foreground-handoff-cancel")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Jobs) != 1 || projection.Jobs[0].Status != "running" {
		t.Fatalf("jobs = %+v, want running managed job after handoff", projection.Jobs)
	}
	jobID := projection.Jobs[0].JobID

	cancelArgs, err := json.Marshal(map[string]string{"job_id": jobID, "reason": "stop attached foreground job"})
	if err != nil {
		t.Fatalf("marshal job_cancel args: %v", err)
	}
	cancelProvider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_cancel_attached_job", Name: "job_cancel", Arguments: json.RawMessage(cancelArgs)},
		},
		final: "cancel requested",
	}
	k.provider = cancelProvider
	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "local-foreground-handoff-cancel",
		InputItems: []InputItem{{Type: "text", Text: "cancel attached foreground job"}},
	}); err != nil {
		t.Fatalf("SubmitTurn cancel returned error: %v", err)
	}
	payload := decodeJSONMap(t, cancelProvider.Requests()[1].ToolRounds[0].Results[0].Content)
	if payload["status"] != "cancel_requested" || payload["cancel_requested"] != true {
		t.Fatalf("job_cancel payload = %+v, want cancel_requested receipt", payload)
	}
	cancelled := waitForSessionJobStatus(t, k, "local-foreground-handoff-cancel", jobID, "cancelled")
	if strings.TrimSpace(cancelled.CancelReason) != "stop attached foreground job" {
		t.Fatalf("cancelled job = %+v, want cancellation reason", cancelled)
	}
}

func TestRestartMarksLocalManagedJobLostOwnershipWithoutRerun(t *testing.T) {
	requireProcessTreeShellSupport(t)
	workspace := testTempDir(t)
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	startedMarker := filepath.Join(workspace, "lost-ownership-started.txt")
	command := foregroundLostOwnershipCommand(startedMarker)
	startArgs, err := json.Marshal(map[string]interface{}{
		"cwd":         workspace,
		"command":     command,
		"timeout_sec": 181,
	})
	if err != nil {
		t.Fatalf("marshal shell args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_lost_ownership_job", Name: "shell_exec", Arguments: json.RawMessage(startArgs)},
		},
		final: "job started",
	}
	owner, err := New(Config{
		LedgerPath:   ledgerPath,
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeYolo,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New owner returned error: %v", err)
	}
	defer owner.Close()
	if _, err := owner.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "local-managed-lost-ownership",
		InputItems: []InputItem{{Type: "text", Text: "start managed job"}},
	}); err != nil {
		t.Fatalf("SubmitTurn start returned error: %v", err)
	}
	waitForFile(t, startedMarker, 5*time.Second)
	original := waitForSessionProjection(t, owner, "local-managed-lost-ownership", func(session SessionProjection) bool {
		return len(session.Jobs) == 1 && session.Jobs[0].Status == "running"
	})
	jobID := original.Jobs[0].JobID

	restarted, err := New(Config{
		LedgerPath:   ledgerPath,
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeYolo,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New restarted returned error: %v", err)
	}
	defer restarted.Close()
	failed := waitForSessionJobStatus(t, restarted, "local-managed-lost-ownership", jobID, "failed")
	if failed.FailureReason != "managed_job_lost_ownership" {
		t.Fatalf("failed job = %+v, want truthful lost ownership evidence", failed)
	}
	startedContent, err := os.ReadFile(startedMarker)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if strings.Count(string(startedContent), "started") != 1 {
		t.Fatalf("started marker = %q, recovery must not rerun command", string(startedContent))
	}
}

func TestRestartDoesNotMarkForeignExecutorJobLostOwnership(t *testing.T) {
	workspace := testTempDir(t)
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	seed, err := New(Config{
		LedgerPath:   ledgerPath,
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeYolo,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New seed returned error: %v", err)
	}
	started := JobProjection{
		JobID:       "foreign_executor_job",
		SessionID:   "foreign-executor-job",
		TurnID:      "turn_foreign_executor_job",
		Tool:        "shell_exec",
		ExecutorRef: "external_test_executor",
		Status:      "running",
		CWD:         workspace,
		Command:     longRunningShellCommand(30),
		TimeoutSec:  600,
		StartedAt:   time.Now().UTC(),
	}
	if err := seed.appendJobEvent("job.started", started); err != nil {
		t.Fatalf("append started returned error: %v", err)
	}
	restarted, err := New(Config{
		LedgerPath:   ledgerPath,
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeYolo,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New restarted returned error: %v", err)
	}
	projection, err := restarted.Session("foreign-executor-job")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Jobs) != 1 || projection.Jobs[0].Status != "running" {
		t.Fatalf("jobs = %+v, want foreign executor running job left untouched", projection.Jobs)
	}
	if got := countSessionEventType(projection.Events, "job.failed"); got != 0 {
		t.Fatalf("job.failed count = %d, foreign executor job must not get local lost-ownership fact", got)
	}
}

func TestLocalManagedJobStartReservesJobBeforeProcessStart(t *testing.T) {
	requireProcessTreeShellSupport(t)
	workspace := testTempDir(t)
	marker := filepath.Join(workspace, "duplicate-managed-job-start.txt")
	executor := newLocalManagedJobExecutor()
	defer executor.Close()

	processStartEntered := make(chan struct{})
	releaseProcessStart := make(chan struct{})
	var processStartHooks int32
	executor.beforeProcessStart = func() {
		if atomic.AddInt32(&processStartHooks, 1) == 1 {
			close(processStartEntered)
			<-releaseProcessStart
		}
	}

	job := JobProjection{
		JobID:      "duplicate_start_job",
		SessionID:  "duplicate-start",
		TurnID:     "turn_duplicate_start",
		Tool:       "shell_exec",
		Status:     "running",
		CWD:        workspace,
		Command:    appendMarkerCommand(marker),
		TimeoutSec: 600,
		StartedAt:  time.Now().UTC(),
	}
	firstDone := make(chan error, 1)
	firstComplete := make(chan JobProjection, 1)
	go func() {
		firstDone <- executor.Start(context.Background(), ManagedJobStartRequest{
			Job:      job,
			Complete: func(completed JobProjection) { firstComplete <- completed },
		})
	}()
	select {
	case <-processStartEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("first process start hook did not run")
	}

	duplicateErr := executor.Start(context.Background(), ManagedJobStartRequest{
		Job:      job,
		Complete: func(JobProjection) {},
	})
	if duplicateErr == nil {
		t.Fatal("duplicate managed job start returned nil error")
	}
	if got := atomic.LoadInt32(&processStartHooks); got != 1 {
		t.Fatalf("process start hooks = %d, duplicate admission must fail before process start", got)
	}

	close(releaseProcessStart)
	if err := <-firstDone; err != nil {
		t.Fatalf("first Start returned error: %v", err)
	}
	completed := <-firstComplete
	if completed.Status != "completed" {
		t.Fatalf("completed job = %+v, want completed", completed)
	}
	assertMarkerLineCount(t, marker, 1)
}

func TestLocalForegroundRunReservesOperationBeforeProcessStart(t *testing.T) {
	requireProcessTreeShellSupport(t)
	workspace := testTempDir(t)
	marker := filepath.Join(workspace, "duplicate-foreground-start.txt")
	executor := newLocalManagedJobExecutor()
	defer executor.Close()

	processStartEntered := make(chan struct{})
	releaseProcessStart := make(chan struct{})
	var processStartHooks int32
	executor.beforeProcessStart = func() {
		if atomic.AddInt32(&processStartHooks, 1) == 1 {
			close(processStartEntered)
			<-releaseProcessStart
		}
	}

	request := ManagedShellForegroundRequest{
		OperationID: "duplicate_foreground_operation",
		CWD:         workspace,
		Command:     appendMarkerCommand(marker),
		Timeout:     5 * time.Second,
	}
	firstDone := make(chan error, 1)
	go func() {
		_, err := executor.RunForeground(context.Background(), request)
		firstDone <- err
	}()
	select {
	case <-processStartEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("first foreground process start hook did not run")
	}

	_, duplicateErr := executor.RunForeground(context.Background(), request)
	if duplicateErr == nil {
		t.Fatal("duplicate foreground operation returned nil error")
	}
	if got := atomic.LoadInt32(&processStartHooks); got != 1 {
		t.Fatalf("process start hooks = %d, duplicate foreground admission must fail before process start", got)
	}

	close(releaseProcessStart)
	if err := <-firstDone; err != nil {
		t.Fatalf("first RunForeground returned error: %v", err)
	}
	assertMarkerLineCount(t, marker, 1)
}

func submitLocalForegroundShellAndInterrupt(t *testing.T, sessionID string, workspace string, command string, readyPath string) (*Kernel, *toolFeedbackProvider, submitTurnResult) {
	t.Helper()
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{{
			ToolCallID: "call_local_foreground_handoff",
			Name:       "shell_exec",
			Arguments: mustJSONRaw(t, map[string]interface{}{
				"cwd":         workspace,
				"command":     command,
				"timeout_sec": 30,
			}),
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
	t.Cleanup(k.Close)

	resultCh := make(chan submitTurnResult, 1)
	go func() {
		resp, err := k.SubmitTurn(context.Background(), TurnRequest{
			SessionID:  sessionID,
			InputItems: []InputItem{{Type: "text", Text: "run foreground shell until interrupted"}},
		})
		resultCh <- submitTurnResult{response: resp, err: err}
	}()
	waitForSessionEventType(t, k, sessionID, "operation.running")
	waitForFile(t, readyPath, 5*time.Second)
	if _, err := k.InterruptSession(sessionID, TurnInterruptRequest{Reason: "detach foreground wait"}); err != nil {
		t.Fatalf("InterruptSession returned error: %v", err)
	}
	return k, provider, waitSubmitTurnResult(t, resultCh)
}

func foregroundHandoffCompletionCommand(readyPath string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf(
			"Set-Content -LiteralPath %s -Value 'ready' -NoNewline; Write-Output 'handoff-before-interrupt'; Start-Sleep -Milliseconds 700; Write-Output 'handoff-after-interrupt'; Start-Sleep -Milliseconds 300; Write-Output 'handoff-complete'",
			powershellSingleQuoted(readyPath),
		)
	}
	return fmt.Sprintf("printf %%s ready > %s; printf 'handoff-before-interrupt\\n'; sleep 0.7; printf 'handoff-after-interrupt\\n'; sleep 0.3; printf 'handoff-complete\\n'", shellSingleQuoted(readyPath))
}

func foregroundHandoffLongRunningCommand(readyPath string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("Set-Content -LiteralPath %s -Value 'ready' -NoNewline; Write-Output 'handoff-running'; Start-Sleep -Seconds 30", powershellSingleQuoted(readyPath))
	}
	return fmt.Sprintf("printf %%s ready > %s; printf 'handoff-running\\n'; sleep 30", shellSingleQuoted(readyPath))
}

func foregroundLostOwnershipCommand(startedPath string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("Add-Content -LiteralPath %s -Value 'started'; Start-Sleep -Seconds 30", powershellSingleQuoted(startedPath))
	}
	return fmt.Sprintf("printf 'started\\n' >> %s; sleep 30", shellSingleQuoted(startedPath))
}

func appendMarkerCommand(markerPath string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("Add-Content -LiteralPath %s -Value 'started'", powershellSingleQuoted(markerPath))
	}
	return fmt.Sprintf("printf 'started\\n' >> %s", shellSingleQuoted(markerPath))
}

func assertMarkerLineCount(t *testing.T, markerPath string, want int) {
	t.Helper()
	waitForFile(t, markerPath, 5*time.Second)
	content, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if got := strings.Count(string(content), "started"); got != want {
		t.Fatalf("marker content = %q, got %d marker lines, want %d", string(content), got, want)
	}
}

func mustSessionProjection(t *testing.T, k *Kernel, sessionID string) SessionProjection {
	t.Helper()
	projection, err := k.Session(sessionID)
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	return projection
}
