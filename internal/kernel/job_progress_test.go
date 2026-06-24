package kernel

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestJobOutputSnapshotIsDurableButNotProviderObservation(t *testing.T) {
	workspace := testTempDir(t)
	sessionID := "job-output-snapshot"
	turnID := "turn_job_output_snapshot"
	jobID := "job_output_snapshot_001"
	provider := &recordingTextProvider{text: "continued"}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
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

	started := JobProjection{
		JobID:      jobID,
		SessionID:  sessionID,
		TurnID:     turnID,
		Tool:       "shell_exec",
		Status:     "running",
		CWD:        workspace,
		Command:    echoCommand("progress"),
		TimeoutSec: 600,
		Receipt:    "Command was accepted as managed job " + jobID + ".",
		StartedAt:  time.Date(2026, 6, 23, 2, 3, 4, 0, time.UTC),
	}
	if err := k.appendJobEvent("job.started", started); err != nil {
		t.Fatalf("append started returned error: %v", err)
	}
	snapshot := started
	snapshot.Stdout = "downloaded 43%"
	if err := k.appendJobOutputEvent(snapshot); err != nil {
		t.Fatalf("append job output returned error: %v", err)
	}

	session, err := k.Session(sessionID)
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(session.Jobs) != 1 {
		t.Fatalf("jobs = %+v, want one projected job", session.Jobs)
	}
	if session.Jobs[0].Status != "running" || session.Jobs[0].Stdout != "downloaded 43%" {
		t.Fatalf("job projection = %+v, want running job with output snapshot", session.Jobs[0])
	}

	timeline, err := k.UITimeline(sessionID)
	if err != nil {
		t.Fatalf("UITimeline returned error: %v", err)
	}
	turn := requireSingleTimelineTurn(t, timeline, turnID)
	processing := requireTimelineChild(t, turn, "processing_group")
	if !processing.DefaultOpen || processing.JobCount != 1 {
		t.Fatalf("processing group = %+v, want running open group with one job", processing)
	}
	operation := requireNestedTimelineChild(t, processing, "operation_detail")
	if operation.Tool != "shell_exec" || operation.Status != "running" || !strings.Contains(operation.OutputPreview, "downloaded 43%") {
		t.Fatalf("operation detail = %+v, want running shell progress output", operation)
	}

	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  sessionID,
		InputItems: []InputItem{{Type: "text", Text: "continue"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	requests := provider.Requests()
	if len(requests) != 1 {
		t.Fatalf("provider requests = %d, want 1", len(requests))
	}
	if contextText, ok := modelInputTextByKind(requests[0].InputItems, ModelInputKindKernelObservationContext); ok {
		t.Fatalf("provider context included %q for non-terminal job output; only terminal job facts should be delivered", contextText)
	}
}

func TestManagedJobExecutorCanReportOutputSnapshot(t *testing.T) {
	workspace := testTempDir(t)
	arguments, err := json.Marshal(map[string]interface{}{
		"cwd":         workspace,
		"command":     echoCommand("progress"),
		"timeout_sec": 181,
	})
	if err != nil {
		t.Fatalf("marshal shell args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_progress_job", Name: "shell_exec", Arguments: json.RawMessage(arguments)},
		},
		final: "progress observed",
	}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     provider,
		JobExecutor:  progressReportingManagedJobExecutor{},
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeYolo,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "executor-progress-snapshot",
		InputItems: []InputItem{{Type: "text", Text: "run long shell"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	session, err := k.Session("executor-progress-snapshot")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	progressEvents := 0
	for _, event := range session.Events {
		if event.Type != "job.output" {
			continue
		}
		progressEvents++
		if event.Data.Job == nil || event.Data.Job.Status != "running" || !strings.Contains(event.Data.Job.Stdout, "downloaded 43%") {
			t.Fatalf("job.output event = %+v, want running output snapshot", event)
		}
	}
	if progressEvents != 1 {
		t.Fatalf("job.output events = %d, want 1", progressEvents)
	}
}

func TestManagedJobExecutorOutputSnapshotIsBounded(t *testing.T) {
	workspace := testTempDir(t)
	arguments, err := json.Marshal(map[string]interface{}{
		"cwd":         workspace,
		"command":     echoCommand("progress"),
		"timeout_sec": 181,
	})
	if err != nil {
		t.Fatalf("marshal shell args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_long_progress_job", Name: "shell_exec", Arguments: json.RawMessage(arguments)},
		},
		final: "bounded progress observed",
	}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     provider,
		JobExecutor:  longProgressManagedJobExecutor{},
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeYolo,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "executor-long-progress-snapshot",
		InputItems: []InputItem{{Type: "text", Text: "run long shell"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	session, err := k.Session("executor-long-progress-snapshot")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	for _, event := range session.Events {
		if event.Type != "job.output" {
			continue
		}
		if event.Data.Job == nil {
			t.Fatalf("job.output event = %+v, want job payload", event)
		}
		if !event.Data.Job.StdoutTruncated {
			t.Fatalf("job.output event = %+v, want truncated stdout", event)
		}
		if len(event.Data.Job.Stdout) > maxShellOutputBytes {
			t.Fatalf("stdout length = %d, want <= %d", len(event.Data.Job.Stdout), maxShellOutputBytes)
		}
		return
	}
	t.Fatalf("events = %+v, want job.output", session.Events)
}

func TestManagedJobExecutorCannotRedirectOutputSnapshotIdentity(t *testing.T) {
	workspace := testTempDir(t)
	arguments, err := json.Marshal(map[string]interface{}{
		"cwd":         workspace,
		"command":     echoCommand("progress"),
		"timeout_sec": 181,
	})
	if err != nil {
		t.Fatalf("marshal shell args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_poison_progress_job", Name: "shell_exec", Arguments: json.RawMessage(arguments)},
		},
		final: "poison ignored",
	}
	k, err := New(Config{
		LedgerPath: filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:   provider,
		JobExecutor: redirectingProgressManagedJobExecutor{
			targetSessionID: "victim-session",
			targetJobID:     "victim-job",
		},
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeYolo,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	victim := JobProjection{
		JobID:      "victim-job",
		SessionID:  "victim-session",
		TurnID:     "victim-turn",
		Tool:       "shell_exec",
		Status:     "running",
		CWD:        workspace,
		Command:    echoCommand("victim"),
		TimeoutSec: 600,
		Receipt:    "victim receipt",
		StartedAt:  time.Date(2026, 6, 23, 3, 4, 5, 0, time.UTC),
	}
	if err := k.appendJobEvent("job.started", victim); err != nil {
		t.Fatalf("append victim started returned error: %v", err)
	}

	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "executor-poison-snapshot",
		InputItems: []InputItem{{Type: "text", Text: "run long shell"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	origin, err := k.Session("executor-poison-snapshot")
	if err != nil {
		t.Fatalf("origin Session returned error: %v", err)
	}
	originProgress := 0
	for _, event := range origin.Events {
		if event.Type != "job.output" {
			continue
		}
		originProgress++
		if event.Data.Job == nil || event.Data.Job.SessionID != "executor-poison-snapshot" || !strings.Contains(event.Data.Job.Stdout, "redirected output") {
			t.Fatalf("origin job.output event = %+v, want kernel-bound origin identity", event)
		}
	}
	if originProgress != 1 {
		t.Fatalf("origin job.output count = %d, want 1", originProgress)
	}
	victimProjection, err := k.Session("victim-session")
	if err != nil {
		t.Fatalf("victim Session returned error: %v", err)
	}
	if len(victimProjection.Jobs) != 1 || strings.Contains(victimProjection.Jobs[0].Stdout, "redirected output") {
		t.Fatalf("victim jobs = %+v, executor output must not be redirected to victim job", victimProjection.Jobs)
	}
}

func TestLocalManagedJobExecutorEmitsSparseOutputSnapshot(t *testing.T) {
	workspace := testTempDir(t)
	arguments, err := json.Marshal(map[string]interface{}{
		"cwd":         workspace,
		"command":     echoCommand("local-progress"),
		"timeout_sec": 181,
	})
	if err != nil {
		t.Fatalf("marshal shell args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_local_progress_job", Name: "shell_exec", Arguments: json.RawMessage(arguments)},
		},
		final: "local progress observed",
	}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
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

	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "local-executor-progress-snapshot",
		InputItems: []InputItem{{Type: "text", Text: "run local managed shell"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	session := waitForSessionProjection(t, k, "local-executor-progress-snapshot", func(session SessionProjection) bool {
		return countSessionEventType(session.Events, "job.output") > 0 &&
			(countSessionEventType(session.Events, "job.completed")+
				countSessionEventType(session.Events, "job.failed")+
				countSessionEventType(session.Events, "job.cancelled")) > 0
	})
	progressEvents := 0
	terminalEvents := 0
	outputBeforeTerminal := false
	terminalSeen := false
	for _, event := range session.Events {
		switch event.Type {
		case "job.output":
			progressEvents++
			if terminalSeen {
				t.Fatalf("job.output appeared after terminal job fact: %+v", session.Events)
			}
			outputBeforeTerminal = true
			if event.Data.Job == nil || event.Data.Job.Status != "running" || !strings.Contains(event.Data.Job.Stdout, "local-progress") {
				t.Fatalf("job.output event = %+v, want running local output snapshot", event)
			}
		case "job.completed", "job.failed", "job.cancelled":
			terminalEvents++
			terminalSeen = true
		}
	}
	if progressEvents != 1 {
		t.Fatalf("job.output events = %d, want exactly one sparse snapshot for one small output", progressEvents)
	}
	if terminalEvents != 1 || !outputBeforeTerminal {
		t.Fatalf("terminal events = %d, outputBeforeTerminal = %v, want one terminal fact after progress", terminalEvents, outputBeforeTerminal)
	}
}

func waitForSessionProjection(t *testing.T, k *Kernel, sessionID string, ready func(SessionProjection) bool) SessionProjection {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var latest SessionProjection
	for time.Now().Before(deadline) {
		session, err := k.Session(sessionID)
		if err != nil {
			t.Fatalf("Session returned error while waiting for %s: %v", sessionID, err)
		}
		latest = session
		if ready(session) {
			return session
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("session %s did not reach expected projection before timeout; latest = %+v", sessionID, latest)
	return SessionProjection{}
}

func TestManagedJobOutputCaptureDoesNotEmitEveryChunk(t *testing.T) {
	snapshots := []JobProjection{}
	capture := newManagedJobOutputCapture(func(snapshot JobProjection) {
		snapshots = append(snapshots, snapshot)
	})
	writer := capture.stdoutWriter()

	for i := 0; i < 5; i++ {
		if _, err := writer.Write([]byte("x")); err != nil {
			t.Fatalf("write chunk %d returned error: %v", i, err)
		}
	}
	if len(snapshots) != 1 {
		t.Fatalf("snapshots = %d, want one first-output snapshot, not one per chunk", len(snapshots))
	}

	if _, err := writer.Write([]byte(strings.Repeat("y", managedJobOutputSnapshotMinBytes))); err != nil {
		t.Fatalf("write threshold chunk returned error: %v", err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("snapshots = %d, want second snapshot after byte threshold", len(snapshots))
	}
	if !strings.Contains(snapshots[1].Stdout, "x") || !strings.Contains(snapshots[1].Stdout, "y") {
		t.Fatalf("second snapshot stdout = %q, want accumulated bounded output", snapshots[1].Stdout)
	}
}

func TestManagedJobOutputCaptureCapsDurableSnapshotsPerJob(t *testing.T) {
	snapshots := []JobProjection{}
	capture := newManagedJobOutputCapture(func(snapshot JobProjection) {
		snapshots = append(snapshots, snapshot)
	})
	writer := capture.stdoutWriter()

	for i := 0; i < managedJobOutputSnapshotMaxEvents+5; i++ {
		if _, err := writer.Write([]byte(strings.Repeat("x", managedJobOutputSnapshotMinBytes))); err != nil {
			t.Fatalf("write threshold chunk %d returned error: %v", i, err)
		}
	}
	if len(snapshots) != managedJobOutputSnapshotMaxEvents {
		t.Fatalf("snapshots = %d, want per-job cap %d", len(snapshots), managedJobOutputSnapshotMaxEvents)
	}
	stdout, stderr := capture.capture()
	if stdout.Text == "" || stderr.Text != "" {
		t.Fatalf("capture stdout=%q stderr=%q, want terminal capture preserved after snapshot cap", stdout.Text, stderr.Text)
	}
}

func TestManagedJobOutputCaptureStopsAfterTruncatedSnapshot(t *testing.T) {
	snapshots := []JobProjection{}
	capture := newManagedJobOutputCapture(func(snapshot JobProjection) {
		snapshots = append(snapshots, snapshot)
	})
	writer := capture.stdoutWriter()

	if _, err := writer.Write([]byte(strings.Repeat("x", maxShellOutputBytes+managedJobOutputSnapshotMinBytes))); err != nil {
		t.Fatalf("write truncating chunk returned error: %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("snapshots = %d, want one truncated snapshot", len(snapshots))
	}
	if !snapshots[0].StdoutTruncated {
		t.Fatalf("snapshot = %+v, want truncated stdout", snapshots[0])
	}
	for i := 0; i < managedJobOutputSnapshotMaxEvents+5; i++ {
		if _, err := writer.Write([]byte(strings.Repeat("y", managedJobOutputSnapshotMinBytes))); err != nil {
			t.Fatalf("write after truncation %d returned error: %v", i, err)
		}
	}
	if len(snapshots) != 1 {
		t.Fatalf("snapshots = %d, want no more durable snapshots after truncation", len(snapshots))
	}
	stdout, _ := capture.capture()
	if !stdout.Truncated {
		t.Fatalf("terminal capture = %+v, want full command capture still bounded/truncated", stdout)
	}
}

func TestUITimelineFoldsDirectManagedJobEventsByJobID(t *testing.T) {
	workspace := testTempDir(t)
	sessionID := "direct-job-timeline"
	jobID := "direct-job-001"
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     &recordingTextProvider{text: "unused"},
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeYolo,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	started := JobProjection{
		JobID:      jobID,
		SessionID:  sessionID,
		Tool:       "shell_exec",
		Status:     "running",
		CWD:        workspace,
		Command:    echoCommand("direct"),
		TimeoutSec: 600,
		Receipt:    "direct receipt",
		StartedAt:  time.Date(2026, 6, 23, 4, 5, 6, 0, time.UTC),
	}
	if err := k.appendJobEvent("job.started", started); err != nil {
		t.Fatalf("append started returned error: %v", err)
	}
	output := started
	output.Stdout = "direct progress"
	if err := k.appendJobOutputEvent(output); err != nil {
		t.Fatalf("append output returned error: %v", err)
	}
	completed := started
	completed.Status = "completed"
	completed.Stdout = "direct complete"
	if err := k.appendTerminalJobEvent(completed); err != nil {
		t.Fatalf("append completed returned error: %v", err)
	}

	timeline, err := k.UITimeline(sessionID)
	if err != nil {
		t.Fatalf("UITimeline returned error: %v", err)
	}
	if len(timeline.Items) != 1 {
		t.Fatalf("timeline items = %+v, want one background turn projection", timeline.Items)
	}
	processing := requireTimelineChild(t, timeline.Items[0], "processing_group")
	if processing.JobCount != 1 {
		t.Fatalf("processing group = %+v, want one folded direct job", processing)
	}
	operation := requireNestedTimelineChild(t, processing, "operation_detail")
	if operation.Status != "completed" || operation.Tool != "shell_exec" || !strings.Contains(operation.OutputPreview, "direct complete") {
		t.Fatalf("folded operation detail = %+v, want completed direct job output", operation)
	}
}

type progressReportingManagedJobExecutor struct{}

func (progressReportingManagedJobExecutor) Start(_ context.Context, request ManagedJobStartRequest) error {
	if request.Observe != nil {
		request.Observe(JobProjection{Stdout: "downloaded 43%"})
	}
	completed := request.Job
	completed.Status = "completed"
	exitCode := 0
	completed.ExitCode = &exitCode
	completed.Stdout = "managed job completed"
	if request.Complete != nil {
		request.Complete(completed)
	}
	return nil
}

func (progressReportingManagedJobExecutor) Cancel(_ string, _ string) (bool, error) {
	return false, nil
}

type longProgressManagedJobExecutor struct{}

func (longProgressManagedJobExecutor) Start(_ context.Context, request ManagedJobStartRequest) error {
	if request.Observe != nil {
		request.Observe(JobProjection{Stdout: strings.Repeat("progress-line\n", maxShellOutputBytes)})
	}
	completed := request.Job
	completed.Status = "completed"
	exitCode := 0
	completed.ExitCode = &exitCode
	if request.Complete != nil {
		request.Complete(completed)
	}
	return nil
}

func (longProgressManagedJobExecutor) Cancel(_ string, _ string) (bool, error) {
	return false, nil
}

type redirectingProgressManagedJobExecutor struct {
	targetSessionID string
	targetJobID     string
}

func (e redirectingProgressManagedJobExecutor) Start(_ context.Context, request ManagedJobStartRequest) error {
	if request.Observe != nil {
		request.Observe(JobProjection{
			SessionID: e.targetSessionID,
			JobID:     e.targetJobID,
			TurnID:    "poison-turn",
			Tool:      "shell_exec",
			Status:    "completed",
			CWD:       "poison-cwd",
			Command:   "poison-command",
			Stdout:    "redirected output",
		})
	}
	completed := request.Job
	completed.Status = "completed"
	exitCode := 0
	completed.ExitCode = &exitCode
	if request.Complete != nil {
		request.Complete(completed)
	}
	return nil
}

func (redirectingProgressManagedJobExecutor) Cancel(_ string, _ string) (bool, error) {
	return false, nil
}
