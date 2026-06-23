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
	workspace := t.TempDir()
	sessionID := "job-output-snapshot"
	turnID := "turn_job_output_snapshot"
	jobID := "job_output_snapshot_001"
	provider := &recordingTextProvider{text: "continued"}
	k, err := New(Config{
		LedgerPath:   filepath.Join(t.TempDir(), "events.jsonl"),
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
	foundProgress := false
	for _, item := range timeline.Items {
		if item.Kind == "tool" && item.Tool == "shell_exec" && item.Status == "running" && strings.Contains(item.OutputPreview, "downloaded 43%") {
			foundProgress = true
			break
		}
	}
	if !foundProgress {
		t.Fatalf("timeline items = %+v, want running shell tool progress output", timeline.Items)
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
	workspace := t.TempDir()
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
		LedgerPath:   filepath.Join(t.TempDir(), "events.jsonl"),
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
	workspace := t.TempDir()
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
		LedgerPath:   filepath.Join(t.TempDir(), "events.jsonl"),
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
	workspace := t.TempDir()
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
		LedgerPath: filepath.Join(t.TempDir(), "events.jsonl"),
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

func TestUITimelineFoldsDirectManagedJobEventsByJobID(t *testing.T) {
	workspace := t.TempDir()
	sessionID := "direct-job-timeline"
	jobID := "direct-job-001"
	k, err := New(Config{
		LedgerPath:   filepath.Join(t.TempDir(), "events.jsonl"),
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
	toolItems := []UITimelineItem{}
	for _, item := range timeline.Items {
		if item.Kind == "tool" && item.Tool == "shell_exec" {
			toolItems = append(toolItems, item)
		}
	}
	if len(toolItems) != 1 {
		t.Fatalf("tool timeline items = %+v, want one folded direct job item", toolItems)
	}
	if toolItems[0].Status != "completed" || !strings.Contains(toolItems[0].OutputPreview, "direct complete") {
		t.Fatalf("folded tool item = %+v, want completed direct job output", toolItems[0])
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
