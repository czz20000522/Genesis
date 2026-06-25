package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSubmitTurnRoutesLongShellTimeoutToManagedJobReceipt(t *testing.T) {
	workspace := testTempDir(t)
	arguments, err := json.Marshal(map[string]interface{}{
		"cwd":         workspace,
		"command":     longRunningShellCommand(30),
		"timeout_sec": 181,
	})
	if err != nil {
		t.Fatalf("marshal shell args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_managed_job", Name: "shell_exec", Arguments: json.RawMessage(arguments)},
		},
		final: "managed job receipt observed",
	}
	ledgerPath := filepath.Join(testTempDir(t), "events.jsonl")
	k, err := New(Config{
		LedgerPath:   ledgerPath,
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

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "managed-job-timeout",
		InputItems: []InputItem{{Type: "text", Text: "run long shell"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "managed job receipt observed" {
		t.Fatalf("final text = %q, want managed job receipt observed", resp.Final.Text)
	}
	resultPayload := decodeJSONMap(t, provider.Requests()[1].ToolRounds[0].Results[0].Content)
	if resultPayload["status"] != "managed_job_started" || resultPayload["executed"] != true {
		t.Fatalf("tool result payload = %+v, want managed job receipt", resultPayload)
	}
	if resultPayload["job_id"] == "" {
		t.Fatalf("tool result payload = %+v, want job_id receipt", resultPayload)
	}
	jobID, _ := resultPayload["job_id"].(string)
	projection, err := k.Session("managed-job-timeout")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Operations) != 0 {
		t.Fatalf("operations = %+v, want no foreground shell operation for managed job", projection.Operations)
	}
	eventTypes := make([]string, 0, len(projection.Events))
	for _, event := range projection.Events {
		eventTypes = append(eventTypes, event.Type)
	}
	wantTypes := []string{"turn.submitted", "tool.call", "job.started", "tool.result", "model.final"}
	if strings.Join(eventTypes, ",") != strings.Join(wantTypes, ",") {
		t.Fatalf("event types = %v, want %v", eventTypes, wantTypes)
	}
	if len(projection.Jobs) != 1 || projection.Jobs[0].JobID != jobID || projection.Jobs[0].Status != "running" {
		t.Fatalf("jobs = %+v, want running managed job %s", projection.Jobs, jobID)
	}

	reloaded, err := New(Config{
		LedgerPath:   ledgerPath,
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeYolo,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	replayed, err := reloaded.Session("managed-job-timeout")
	if err != nil {
		t.Fatalf("reloaded Session returned error: %v", err)
	}
	replayedTypes := make([]string, 0, len(replayed.Events))
	for _, event := range replayed.Events {
		replayedTypes = append(replayedTypes, event.Type)
	}
	wantReloadedTypes := append(append([]string{}, wantTypes...), "job.failed")
	if strings.Join(replayedTypes, ",") != strings.Join(wantReloadedTypes, ",") {
		t.Fatalf("replayed event types = %v, want %v", replayedTypes, wantReloadedTypes)
	}
	if len(replayed.Jobs) != 1 || replayed.Jobs[0].Status != "failed" || replayed.Jobs[0].FailureReason != "managed_job_lost_ownership" {
		t.Fatalf("replayed jobs = %+v, want truthful lost ownership terminal fact", replayed.Jobs)
	}
}

func TestSubmitTurnLongShellTimeoutDefaultModeDoesNotStartManagedHostJob(t *testing.T) {
	workspace := testTempDir(t)
	arguments, err := json.Marshal(map[string]interface{}{
		"cwd":         workspace,
		"command":     echoCommand("default-long"),
		"timeout_sec": maxForegroundShellTimeoutSec + 1,
	})
	if err != nil {
		t.Fatalf("marshal shell args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_default_managed_blocked", Name: "shell_exec", Arguments: json.RawMessage(arguments)},
		},
		final: "default managed block observed",
	}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeDefault,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "default-managed-blocked",
		InputItems: []InputItem{{Type: "text", Text: "try default long shell"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "default managed block observed" {
		t.Fatalf("final text = %q, want default managed block observed", resp.Final.Text)
	}
	payload := decodeJSONMap(t, provider.Requests()[1].ToolRounds[0].Results[0].Content)
	if payload["status"] != "permission_denied" || payload["executed"] != false {
		t.Fatalf("tool result payload = %+v, want permission_denied without execution", payload)
	}
	projection, err := k.Session("default-managed-blocked")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Jobs) != 0 {
		t.Fatalf("jobs = %+v, want no managed host job in default mode", projection.Jobs)
	}
	if len(projection.Operations) != 1 || projection.Operations[0].Status != "blocked" {
		t.Fatalf("operations = %+v, want blocked operation", projection.Operations)
	}
	if projection.Operations[0].BlockedReason != "managed_job_requires_host_sandbox" {
		t.Fatalf("blocked reason = %q, want managed_job_requires_host_sandbox", projection.Operations[0].BlockedReason)
	}
}

func TestSubmitTurnDeliversCompletedJobObservationToNextProviderStep(t *testing.T) {
	workspace := testTempDir(t)
	arguments, err := json.Marshal(map[string]interface{}{
		"cwd":         workspace,
		"command":     echoCommand("queued-job"),
		"timeout_sec": 181,
	})
	if err != nil {
		t.Fatalf("marshal shell args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_job_observation", Name: "shell_exec", Arguments: json.RawMessage(arguments)},
		},
		final: "job observation received",
	}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     provider,
		JobExecutor:  completingManagedJobExecutor{},
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeYolo,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "job-observation-delivery",
		InputItems: []InputItem{{Type: "text", Text: "run long shell"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	requests := provider.Requests()
	if len(requests) != 2 {
		t.Fatalf("provider requests = %d, want 2", len(requests))
	}
	observationContext, ok := modelInputTextByKind(requests[1].InputItems, ModelInputKindKernelObservationContext)
	if !ok {
		t.Fatalf("second provider input items = %+v, want kernel observation context", requests[1].InputItems)
	}
	if !strings.Contains(observationContext, "job.completed") || !strings.Contains(observationContext, "shell_exec") {
		t.Fatalf("observation context = %q, want completed shell job fact", observationContext)
	}
	if strings.Contains(observationContext, "event_id=") {
		t.Fatalf("observation context = %q, must not expose kernel event ids", observationContext)
	}

	projection, err := k.Session("job-observation-delivery")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	var completedEventID string
	var delivered *KernelObservationDeliveryProjection
	for _, event := range projection.Events {
		switch event.Type {
		case "job.completed":
			completedEventID = event.EventID
		case "kernel.observation.delivered":
			delivered = event.Data.KernelObservationDelivery
		}
	}
	if completedEventID == "" {
		t.Fatalf("session events = %+v, want job.completed", projection.Events)
	}
	if delivered == nil || !containsString(delivered.ObservationEventIDs, completedEventID) {
		t.Fatalf("delivery event = %+v, want completed event id %s", delivered, completedEventID)
	}
	if strings.Contains(observationContext, completedEventID) {
		t.Fatalf("observation context = %q, must not expose completed event id %s", observationContext, completedEventID)
	}
	if resp.Final.Text != "job observation received" {
		t.Fatalf("final text = %q, want job observation received", resp.Final.Text)
	}
}

func TestSubmitTurnDeliversAllTerminalJobObservationKinds(t *testing.T) {
	for _, tc := range []struct {
		status    string
		eventType string
	}{
		{status: "completed", eventType: "job.completed"},
		{status: "failed", eventType: "job.failed"},
		{status: "cancelled", eventType: "job.cancelled"},
	} {
		t.Run(tc.status, func(t *testing.T) {
			workspace := testTempDir(t)
			sessionID := "job-terminal-observation-" + tc.status
			jobID := "job_terminal_" + tc.status
			provider := &recordingTextProvider{text: "terminal observation delivered"}
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
				TurnID:     "turn_terminal_observation",
				Tool:       "shell_exec",
				Status:     "running",
				CWD:        workspace,
				Command:    echoCommand("terminal-observation"),
				TimeoutSec: 600,
				Receipt:    "Command was accepted as managed job " + jobID + ".",
				StartedAt:  time.Date(2026, 6, 23, 1, 2, 3, 0, time.UTC),
			}
			if err := k.appendJobEvent("job.started", started); err != nil {
				t.Fatalf("append started returned error: %v", err)
			}
			terminal := started
			terminal.Status = tc.status
			terminal.CompletedAt = time.Date(2026, 6, 23, 1, 2, 4, 0, time.UTC)
			switch tc.status {
			case "completed":
				exitCode := 0
				terminal.ExitCode = &exitCode
				terminal.Stdout = "completed output"
			case "failed":
				exitCode := 7
				terminal.ExitCode = &exitCode
				terminal.Stderr = "failed output"
				terminal.FailureReason = "command exited nonzero"
			case "cancelled":
				terminal.CancelReason = "user stopped it"
			}
			if err := k.appendTerminalJobEvent(terminal); err != nil {
				t.Fatalf("append terminal returned error: %v", err)
			}

			if _, err := k.SubmitTurn(context.Background(), TurnRequest{
				SessionID:  sessionID,
				InputItems: []InputItem{{Type: "text", Text: "continue after terminal job"}},
			}); err != nil {
				t.Fatalf("SubmitTurn returned error: %v", err)
			}
			requests := provider.Requests()
			if len(requests) != 1 {
				t.Fatalf("provider requests = %d, want 1", len(requests))
			}
			observationContext, ok := modelInputTextByKind(requests[0].InputItems, ModelInputKindKernelObservationContext)
			if !ok {
				t.Fatalf("provider input items = %+v, want kernel observation context", requests[0].InputItems)
			}
			if !strings.Contains(observationContext, tc.eventType) || !strings.Contains(observationContext, jobID) {
				t.Fatalf("observation context = %q, want %s for %s", observationContext, tc.eventType, jobID)
			}
			if strings.Contains(observationContext, "event_id=") {
				t.Fatalf("observation context = %q, must not expose kernel event ids", observationContext)
			}
			projection, err := k.Session(sessionID)
			if err != nil {
				t.Fatalf("Session returned error: %v", err)
			}
			var terminalEventID string
			var delivered *KernelObservationDeliveryProjection
			for _, event := range projection.Events {
				switch event.Type {
				case tc.eventType:
					terminalEventID = event.EventID
				case "kernel.observation.delivered":
					delivered = event.Data.KernelObservationDelivery
				}
			}
			if terminalEventID == "" {
				t.Fatalf("events = %+v, want %s", projection.Events, tc.eventType)
			}
			if delivered == nil || !containsString(delivered.ObservationEventIDs, terminalEventID) {
				t.Fatalf("delivery = %+v, want terminal event id %s", delivered, terminalEventID)
			}
		})
	}
}

func TestProviderFailureDoesNotMarkJobObservationDelivered(t *testing.T) {
	workspace := testTempDir(t)
	arguments, err := json.Marshal(map[string]interface{}{
		"cwd":         workspace,
		"command":     echoCommand("queued-job"),
		"timeout_sec": 181,
	})
	if err != nil {
		t.Fatalf("marshal shell args: %v", err)
	}
	provider := &jobObservationFailingProvider{
		call: ModelToolCall{ToolCallID: "call_job_observation_failure", Name: "shell_exec", Arguments: json.RawMessage(arguments)},
	}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     provider,
		JobExecutor:  completingManagedJobExecutor{},
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeYolo,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	_, err = k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "job-observation-provider-failure",
		InputItems: []InputItem{{Type: "text", Text: "run long shell"}},
	})
	if err == nil {
		t.Fatal("SubmitTurn returned nil error, want provider failure")
	}
	requests := provider.Requests()
	if len(requests) != 2 {
		t.Fatalf("provider requests = %d, want 2", len(requests))
	}
	if _, ok := modelInputTextByKind(requests[1].InputItems, ModelInputKindKernelObservationContext); !ok {
		t.Fatalf("second provider input items = %+v, want kernel observation context before failure", requests[1].InputItems)
	}
	projection, err := k.Session("job-observation-provider-failure")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	for _, event := range projection.Events {
		if event.Type == "kernel.observation.delivered" {
			t.Fatalf("events = %+v, want no delivered observation after provider failure", projection.Events)
		}
	}
}

func TestDeliveredJobObservationIsNotProjectedAgainAfterRestart(t *testing.T) {
	workspace := testTempDir(t)
	arguments, err := json.Marshal(map[string]interface{}{
		"cwd":         workspace,
		"command":     echoCommand("queued-job"),
		"timeout_sec": 181,
	})
	if err != nil {
		t.Fatalf("marshal shell args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_job_observation_restart", Name: "shell_exec", Arguments: json.RawMessage(arguments)},
		},
		final: "job observation received",
	}
	ledgerPath := filepath.Join(testTempDir(t), "events.jsonl")
	k, err := New(Config{
		LedgerPath:   ledgerPath,
		Provider:     provider,
		JobExecutor:  completingManagedJobExecutor{},
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeYolo,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "job-observation-restart",
		InputItems: []InputItem{{Type: "text", Text: "run long shell"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	projection, err := k.Session("job-observation-restart")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	var completedEventID string
	var delivered *KernelObservationDeliveryProjection
	for _, event := range projection.Events {
		switch event.Type {
		case "job.completed":
			completedEventID = event.EventID
		case "kernel.observation.delivered":
			delivered = event.Data.KernelObservationDelivery
		}
	}
	if completedEventID == "" {
		t.Fatalf("events = %+v, want job.completed before restart", projection.Events)
	}
	if delivered == nil || !containsString(delivered.ObservationEventIDs, completedEventID) {
		t.Fatalf("delivery = %+v, want delivered completed event %s before restart", delivered, completedEventID)
	}

	reloaded, err := New(Config{
		LedgerPath:   ledgerPath,
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeYolo,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	contextProjection, err := reloaded.ProviderContextProjection(resp.TurnID)
	if err != nil {
		t.Fatalf("ProviderContextProjection after restart returned error: %v", err)
	}
	if _, ok := modelInputTextByKind(contextProjection.InputItems, ModelInputKindKernelObservationContext); ok {
		t.Fatalf("provider context after restart = %+v, want delivered observation suppressed", contextProjection.InputItems)
	}
}

func TestSubmitTurnLiveManagedExecutorRecordsCompletedOutput(t *testing.T) {
	workspace := testTempDir(t)
	ledgerPath := filepath.Join(testTempDir(t), "events.jsonl")
	sessionID := "job-live-completion"
	arguments, err := json.Marshal(map[string]interface{}{
		"cwd":         workspace,
		"command":     echoCommand("live-complete"),
		"timeout_sec": 181,
	})
	if err != nil {
		t.Fatalf("marshal shell args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_live_completion", Name: "shell_exec", Arguments: json.RawMessage(arguments)},
		},
		final: "job started",
	}
	k, err := New(Config{
		LedgerPath:   ledgerPath,
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

	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  sessionID,
		InputItems: []InputItem{{Type: "text", Text: "start live completion"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	projection, err := k.Session(sessionID)
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Jobs) != 1 || projection.Jobs[0].Status != "running" {
		t.Fatalf("jobs after receipt = %+v, want one running job", projection.Jobs)
	}
	jobID := projection.Jobs[0].JobID
	completed := waitForSessionJobStatus(t, k, sessionID, jobID, "completed")
	if completed.ExitCode == nil || *completed.ExitCode != 0 || !strings.Contains(completed.Stdout, "live-complete") {
		t.Fatalf("completed job = %+v, want exit 0 with stdout", completed)
	}
	if strings.TrimSpace(completed.Receipt) == "" {
		t.Fatalf("completed job = %+v, want original receipt preserved", completed)
	}
	projection, err = k.Session(sessionID)
	if err != nil {
		t.Fatalf("Session after completion returned error: %v", err)
	}
	if got := countSessionEventType(projection.Events, "job.completed"); got != 1 {
		t.Fatalf("job.completed count = %d, want 1", got)
	}
}

func TestSubmitTurnProjectsGenericJobControlToolManifest(t *testing.T) {
	provider := &recordingTextProvider{text: "manifest observed"}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ToolPolicy:   ToolPolicy{PermissionMode: PermissionModePlan},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "job-control-manifest",
		InputItems: []InputItem{{Type: "text", Text: "show tools"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	requests := provider.Requests()
	if len(requests) != 1 {
		t.Fatalf("provider requests = %d, want 1", len(requests))
	}
	names := toolSpecNames(requests[0].ToolManifest)
	for _, want := range []string{"shell_exec", "job_status", "job_wait", "job_cancel"} {
		if !containsString(names, want) {
			t.Fatalf("tool manifest names = %v, want %s", names, want)
		}
	}
	if containsString(names, "job_terminate") {
		t.Fatalf("tool manifest names = %v, must not expose process-level terminate tool", names)
	}
}

func TestSubmitTurnJobStatusReturnsCompletedJobAfterRestartWithoutOperation(t *testing.T) {
	workspace := testTempDir(t)
	ledgerPath := filepath.Join(testTempDir(t), "events.jsonl")
	sessionID := "job-status-completed"
	jobID := submitCompletedManagedJobForTest(t, ledgerPath, workspace, sessionID)
	arguments, err := json.Marshal(map[string]string{"job_id": jobID})
	if err != nil {
		t.Fatalf("marshal job_status args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_job_status_completed", Name: "job_status", Arguments: json.RawMessage(arguments)},
		},
		final: "job status observed",
	}
	reloaded, err := New(Config{
		LedgerPath:   ledgerPath,
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

	if _, err := reloaded.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  sessionID,
		InputItems: []InputItem{{Type: "text", Text: "check job"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	payload := decodeJSONMap(t, provider.Requests()[1].ToolRounds[0].Results[0].Content)
	if payload["status"] != "completed" || payload["job_id"] != jobID || payload["tool"] != "shell_exec" {
		t.Fatalf("job_status payload = %+v, want completed shell job %s", payload, jobID)
	}
	projection, err := reloaded.Session(sessionID)
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Operations) != 0 {
		t.Fatalf("operations = %+v, want job_status to create no operations", projection.Operations)
	}
}

func TestSubmitTurnJobStatusPreservesTerminalOutputProjection(t *testing.T) {
	workspace := testTempDir(t)
	ledgerPath := filepath.Join(testTempDir(t), "events.jsonl")
	sessionID := "job-status-redaction"
	jobID := "job_status_redaction"
	seed, err := New(Config{
		LedgerPath:   ledgerPath,
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New seed returned error: %v", err)
	}
	startedAt := time.Date(2026, 6, 22, 1, 2, 3, 0, time.UTC)
	started := JobProjection{
		JobID:      jobID,
		SessionID:  sessionID,
		Tool:       "shell_exec",
		Status:     "running",
		CWD:        workspace,
		Command:    secretEchoCommand(),
		TimeoutSec: 600,
		StartedAt:  startedAt,
	}
	if err := seed.appendJobEvent("job.started", started); err != nil {
		t.Fatalf("append started job: %v", err)
	}
	completed := started
	completed.Status = "completed"
	completed.Stdout = "GENESIS_PROVIDER_API_KEY=sk-secret123\nAuthorization: Bearer tokentest123456\n"
	completed.Stderr = `{"api_key":"sk-jsonsecret"}`
	completed.CompletedAt = startedAt.Add(time.Second)
	if err := seed.appendJobEvent("job.completed", completed); err != nil {
		t.Fatalf("append completed job: %v", err)
	}

	arguments, err := json.Marshal(map[string]string{"job_id": jobID})
	if err != nil {
		t.Fatalf("marshal job_status args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_job_status_redaction", Name: "job_status", Arguments: json.RawMessage(arguments)},
		},
		final: "job status redaction observed",
	}
	reloaded, err := New(Config{
		LedgerPath:   ledgerPath,
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeYolo,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New reloaded returned error: %v", err)
	}
	if _, err := reloaded.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  sessionID,
		InputItems: []InputItem{{Type: "text", Text: "check job output"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	payload := decodeJSONMap(t, provider.Requests()[1].ToolRounds[0].Results[0].Content)
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	for _, want := range []string{"sk-secret123", "tokentest123456", "sk-jsonsecret"} {
		if !strings.Contains(string(payloadJSON), want) {
			t.Fatalf("job_status payload lost terminal output %q: %s", want, string(payloadJSON))
		}
	}
	if strings.Contains(string(payloadJSON), "[REDACTED]") {
		t.Fatalf("job_status payload should not use lossy redaction: %s", string(payloadJSON))
	}
}

func TestSubmitTurnJobStatusReturnsRunningFailedAndCancelledStates(t *testing.T) {
	for _, tc := range []struct {
		status    string
		eventType string
	}{
		{status: "running", eventType: ""},
		{status: "failed", eventType: "job.failed"},
		{status: "cancelled", eventType: "job.cancelled"},
	} {
		t.Run(tc.status, func(t *testing.T) {
			workspace := testTempDir(t)
			ledgerPath := filepath.Join(testTempDir(t), "events.jsonl")
			sessionID := "job-status-" + tc.status
			jobID := "job_status_" + tc.status
			k, err := New(Config{
				LedgerPath:   ledgerPath,
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
				TurnID:     "turn_job_status",
				Tool:       "shell_exec",
				Status:     "running",
				CWD:        workspace,
				Command:    echoCommand("status"),
				TimeoutSec: 600,
				Receipt:    "Command was accepted as managed job " + jobID + ".",
				StartedAt:  time.Date(2026, 6, 23, 1, 2, 3, 0, time.UTC),
			}
			if err := k.appendJobEvent("job.started", started); err != nil {
				t.Fatalf("append started returned error: %v", err)
			}
			if tc.eventType != "" {
				terminal := started
				terminal.Status = tc.status
				terminal.CompletedAt = time.Date(2026, 6, 23, 1, 2, 4, 0, time.UTC)
				if tc.status == "failed" {
					exitCode := 7
					terminal.ExitCode = &exitCode
					terminal.Stderr = "failed"
					terminal.FailureReason = "command exited nonzero"
				}
				if tc.status == "cancelled" {
					terminal.CancelReason = "user stopped it"
				}
				if err := k.appendTerminalJobEvent(terminal); err != nil {
					t.Fatalf("append terminal returned error: %v", err)
				}
			}
			arguments, err := json.Marshal(map[string]string{"job_id": jobID})
			if err != nil {
				t.Fatalf("marshal job_status args: %v", err)
			}
			provider := &toolFeedbackProvider{
				calls: []ModelToolCall{
					{ToolCallID: "call_job_status_" + tc.status, Name: "job_status", Arguments: json.RawMessage(arguments)},
				},
				final: "job status observed",
			}
			k.provider = provider
			if _, err := k.SubmitTurn(context.Background(), TurnRequest{
				SessionID:  sessionID,
				InputItems: []InputItem{{Type: "text", Text: "check job status"}},
			}); err != nil {
				t.Fatalf("SubmitTurn returned error: %v", err)
			}
			payload := decodeJSONMap(t, provider.Requests()[1].ToolRounds[0].Results[0].Content)
			if payload["status"] != tc.status || payload["job_id"] != jobID {
				t.Fatalf("job_status payload = %+v, want %s for %s", payload, tc.status, jobID)
			}
		})
	}
}

func TestSubmitTurnJobStatusReturnsRepairFeedbackForUnknownJob(t *testing.T) {
	arguments := json.RawMessage(`{"job_id":"job_missing"}`)
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_job_status_missing", Name: "job_status", Arguments: arguments},
		},
		final: "job status repair observed",
	}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ToolPolicy:   ToolPolicy{PermissionMode: PermissionModePlan},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "job-status-missing",
		InputItems: []InputItem{{Type: "text", Text: "check missing job"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	payload := decodeJSONMap(t, provider.Requests()[1].ToolRounds[0].Results[0].Content)
	if payload["status"] != "tool_request_invalid" {
		t.Fatalf("job_status payload = %+v, want repair feedback", payload)
	}
	errorPayload, ok := payload["error"].(map[string]interface{})
	if !ok || errorPayload["code"] != "job_not_found" {
		t.Fatalf("job_status error = %+v, want job_not_found", payload["error"])
	}
}

func TestSubmitTurnJobWaitReturnsTerminalJobBeforeTimeout(t *testing.T) {
	workspace := testTempDir(t)
	ledgerPath := filepath.Join(testTempDir(t), "events.jsonl")
	sessionID := "job-wait-terminal"
	jobID := "job_wait_terminal"
	k, err := New(Config{
		LedgerPath:   ledgerPath,
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
		TurnID:     "turn_job_wait",
		Tool:       "shell_exec",
		Status:     "running",
		CWD:        workspace,
		Command:    longRunningShellCommand(30),
		TimeoutSec: 600,
		Receipt:    "Command was accepted as managed job " + jobID + ".",
		StartedAt:  time.Now().UTC(),
	}
	if err := k.appendJobEvent("job.started", started); err != nil {
		t.Fatalf("append started returned error: %v", err)
	}
	go func() {
		time.Sleep(100 * time.Millisecond)
		completed := started
		completed.Status = "completed"
		completed.Stdout = "waited complete"
		_ = k.appendTerminalJobEvent(completed)
	}()

	waitArgs, err := json.Marshal(map[string]interface{}{"job_id": jobID, "timeout_sec": 2})
	if err != nil {
		t.Fatalf("marshal job_wait args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_job_wait_terminal", Name: "job_wait", Arguments: json.RawMessage(waitArgs)},
		},
		final: "job wait observed",
	}
	k.provider = provider
	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  sessionID,
		InputItems: []InputItem{{Type: "text", Text: "wait for job"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	payload := decodeJSONMap(t, provider.Requests()[1].ToolRounds[0].Results[0].Content)
	if payload["status"] != "completed" || payload["job_id"] != jobID || !strings.Contains(fmt.Sprint(payload["stdout"]), "waited complete") {
		t.Fatalf("job_wait payload = %+v, want completed job output", payload)
	}
	projection, err := k.Session(sessionID)
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Operations) != 0 {
		t.Fatalf("operations = %+v, want job_wait to create no operations", projection.Operations)
	}
}

func TestSubmitTurnJobWaitReturnsRunningAfterBoundedTimeout(t *testing.T) {
	workspace := testTempDir(t)
	ledgerPath := filepath.Join(testTempDir(t), "events.jsonl")
	sessionID := "job-wait-timeout"
	jobID := "job_wait_timeout"
	k, err := New(Config{
		LedgerPath:   ledgerPath,
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeYolo,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if err := k.appendJobEvent("job.started", JobProjection{
		JobID:      jobID,
		SessionID:  sessionID,
		TurnID:     "turn_job_wait_timeout",
		Tool:       "shell_exec",
		Status:     "running",
		CWD:        workspace,
		Command:    longRunningShellCommand(30),
		TimeoutSec: 600,
		Receipt:    "Command was accepted as managed job " + jobID + ".",
		StartedAt:  time.Now().UTC(),
	}); err != nil {
		t.Fatalf("append started returned error: %v", err)
	}
	waitArgs, err := json.Marshal(map[string]interface{}{"job_id": jobID, "timeout_sec": 1})
	if err != nil {
		t.Fatalf("marshal job_wait args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_job_wait_timeout", Name: "job_wait", Arguments: json.RawMessage(waitArgs)},
		},
		final: "job wait timeout observed",
	}
	k.provider = provider
	startedAt := time.Now()
	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  sessionID,
		InputItems: []InputItem{{Type: "text", Text: "wait for running job"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if elapsed := time.Since(startedAt); elapsed > 3*time.Second {
		t.Fatalf("job_wait elapsed %s, want bounded timeout", elapsed)
	}
	payload := decodeJSONMap(t, provider.Requests()[1].ToolRounds[0].Results[0].Content)
	if payload["status"] != "running" || payload["timed_out"] != true {
		t.Fatalf("job_wait payload = %+v, want running timed_out observation", payload)
	}
}

func TestJobWaitDeadlineReloadDoesNotMarkTerminalJobTimedOut(t *testing.T) {
	workspace := testTempDir(t)
	ledgerPath := filepath.Join(testTempDir(t), "events.jsonl")
	sessionID := "job-wait-terminal-deadline"
	jobID := "job_wait_terminal_deadline"
	k, err := New(Config{
		LedgerPath:   ledgerPath,
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
		TurnID:     "turn_job_wait_terminal_deadline",
		Tool:       "shell_exec",
		Status:     "running",
		CWD:        workspace,
		Command:    longRunningShellCommand(30),
		TimeoutSec: 600,
		StartedAt:  time.Now().UTC(),
	}
	if err := k.appendJobEvent("job.started", started); err != nil {
		t.Fatalf("append started returned error: %v", err)
	}
	completed := started
	completed.Status = "completed"
	completed.Stdout = "completed before deadline return"
	if err := k.appendTerminalJobEvent(completed); err != nil {
		t.Fatalf("append completed returned error: %v", err)
	}
	result, err := k.jobWaitModelToolResult(context.Background(), sessionID, "evt_wait_deadline", "call_wait_deadline", "job_wait", jobID, 1)
	if err != nil {
		t.Fatalf("jobWaitModelToolResult returned error: %v", err)
	}
	payload := decodeJSONMap(t, result.Content)
	if payload["status"] != "completed" {
		t.Fatalf("job_wait payload = %+v, want completed", payload)
	}
	if payload["timed_out"] == true {
		t.Fatalf("job_wait payload = %+v, terminal job must not be marked timed_out", payload)
	}
}

func TestSubmitTurnJobCancelTerminalJobReturnsCurrentStateWithoutCompetingTerminalEvent(t *testing.T) {
	workspace := testTempDir(t)
	ledgerPath := filepath.Join(testTempDir(t), "events.jsonl")
	sessionID := "job-cancel-terminal"
	jobID := submitCompletedManagedJobForTest(t, ledgerPath, workspace, sessionID)
	arguments, err := json.Marshal(map[string]string{"job_id": jobID, "reason": "no longer needed"})
	if err != nil {
		t.Fatalf("marshal job_cancel args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_job_cancel_completed", Name: "job_cancel", Arguments: json.RawMessage(arguments)},
		},
		final: "job cancel observed",
	}
	k, err := New(Config{
		LedgerPath:   ledgerPath,
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
		SessionID:  sessionID,
		InputItems: []InputItem{{Type: "text", Text: "cancel completed job"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	payload := decodeJSONMap(t, provider.Requests()[1].ToolRounds[0].Results[0].Content)
	if payload["status"] != "completed" || payload["job_id"] != jobID {
		t.Fatalf("job_cancel terminal payload = %+v, want current completed state", payload)
	}
	projection, err := k.Session(sessionID)
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if got := countSessionEventType(projection.Events, "job.cancel_requested"); got != 0 {
		t.Fatalf("job.cancel_requested count = %d, want 0 for terminal no-op", got)
	}
	if got := countSessionEventType(projection.Events, "job.cancelled"); got != 0 {
		t.Fatalf("job.cancelled count = %d, want 0 for terminal no-op", got)
	}
}

func TestSubmitTurnJobCancelLedgerOnlyRunningJobRecordsRequestWithoutForgingTerminalFact(t *testing.T) {
	workspace := testTempDir(t)
	ledgerPath := filepath.Join(testTempDir(t), "events.jsonl")
	sessionID := "job-cancel-running"
	jobID := "job_running_cancel"
	cancelArgs, err := json.Marshal(map[string]string{"job_id": jobID, "reason": "user requested stop"})
	if err != nil {
		t.Fatalf("marshal job_cancel args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_job_cancel_running", Name: "job_cancel", Arguments: json.RawMessage(cancelArgs)},
		},
		final: "job cancel observed",
	}
	k, err := New(Config{
		LedgerPath:   ledgerPath,
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
	if err := k.appendJobEvent("job.started", JobProjection{
		JobID:      jobID,
		SessionID:  sessionID,
		TurnID:     "turn_background_job",
		Tool:       "shell_exec",
		Status:     "running",
		CWD:        workspace,
		Command:    echoCommand("running"),
		TimeoutSec: 600,
		Receipt:    "Command was accepted as managed job " + jobID + ".",
		StartedAt:  time.Date(2026, 6, 23, 1, 2, 3, 0, time.UTC),
	}); err != nil {
		t.Fatalf("appendJobEvent returned error: %v", err)
	}

	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  sessionID,
		InputItems: []InputItem{{Type: "text", Text: "cancel running job"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	payload := decodeJSONMap(t, provider.Requests()[1].ToolRounds[0].Results[0].Content)
	if payload["status"] != "cancel_requested" || payload["job_id"] != jobID || payload["cancel_requested"] != true {
		t.Fatalf("job_cancel running payload = %+v, want cancel_requested job", payload)
	}
	projection, err := k.Session(sessionID)
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Jobs) != 1 || projection.Jobs[0].Status != "cancel_requested" {
		t.Fatalf("jobs = %+v, want cancel_requested job projection", projection.Jobs)
	}
	if got := countSessionEventType(projection.Events, "job.cancel_requested"); got != 1 {
		t.Fatalf("job.cancel_requested count = %d, want 1", got)
	}
	if got := countSessionEventType(projection.Events, "job.cancelled"); got != 0 {
		t.Fatalf("job.cancelled count = %d, want 0 without executor confirmation", got)
	}

	secondProvider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_job_cancel_running_again", Name: "job_cancel", Arguments: json.RawMessage(cancelArgs)},
		},
		final: "job cancel observed again",
	}
	reloaded, err := New(Config{
		LedgerPath:   ledgerPath,
		Provider:     secondProvider,
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeYolo,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if _, err := reloaded.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  sessionID,
		InputItems: []InputItem{{Type: "text", Text: "cancel running job again"}},
	}); err != nil {
		t.Fatalf("second SubmitTurn returned error: %v", err)
	}
	replayed, err := reloaded.Session(sessionID)
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if got := countSessionEventType(replayed.Events, "job.cancel_requested"); got != 1 {
		t.Fatalf("job.cancel_requested count after duplicate = %d, want 1", got)
	}
	if got := countSessionEventType(replayed.Events, "job.cancelled"); got != 0 {
		t.Fatalf("job.cancelled count after duplicate = %d, want 0 without executor confirmation", got)
	}
}

func TestSubmitTurnJobCancelPlanModeReturnsPermissionDeniedWithoutCancelEvent(t *testing.T) {
	workspace := testTempDir(t)
	ledgerPath := filepath.Join(testTempDir(t), "events.jsonl")
	sessionID := "job-cancel-plan-denied"
	jobID := "job_plan_denied"
	cancelArgs, err := json.Marshal(map[string]string{"job_id": jobID, "reason": "should be denied"})
	if err != nil {
		t.Fatalf("marshal job_cancel args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_job_cancel_plan", Name: "job_cancel", Arguments: json.RawMessage(cancelArgs)},
		},
		final: "job cancel denied",
	}
	k, err := New(Config{
		LedgerPath:   ledgerPath,
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModePlan,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if err := k.appendJobEvent("job.started", JobProjection{
		JobID:      jobID,
		SessionID:  sessionID,
		TurnID:     "turn_plan_denied",
		Tool:       "shell_exec",
		Status:     "running",
		CWD:        workspace,
		Command:    echoCommand("running"),
		TimeoutSec: 600,
		Receipt:    "Command was accepted as managed job " + jobID + ".",
		StartedAt:  time.Date(2026, 6, 23, 1, 2, 3, 0, time.UTC),
	}); err != nil {
		t.Fatalf("appendJobEvent returned error: %v", err)
	}

	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  sessionID,
		InputItems: []InputItem{{Type: "text", Text: "cancel running job"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	payload := decodeJSONMap(t, provider.Requests()[1].ToolRounds[0].Results[0].Content)
	if payload["status"] != "permission_denied" || payload["executed"] != false {
		t.Fatalf("job_cancel payload = %+v, want permission_denied without execution", payload)
	}
	projection, err := k.Session(sessionID)
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if got := countSessionEventType(projection.Events, "job.cancel_requested"); got != 0 {
		t.Fatalf("job.cancel_requested count = %d, want 0 for denied cancel", got)
	}
}

func TestSubmitTurnJobCancelReachesLiveManagedExecutor(t *testing.T) {
	workspace := testTempDir(t)
	ledgerPath := filepath.Join(testTempDir(t), "events.jsonl")
	sessionID := "job-cancel-live-executor"
	startArgs, err := json.Marshal(map[string]interface{}{
		"cwd":         workspace,
		"command":     longRunningShellCommand(30),
		"timeout_sec": 181,
	})
	if err != nil {
		t.Fatalf("marshal shell args: %v", err)
	}
	startProvider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_start_live_job", Name: "shell_exec", Arguments: json.RawMessage(startArgs)},
		},
		final: "job started",
	}
	k, err := New(Config{
		LedgerPath:   ledgerPath,
		Provider:     startProvider,
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

	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  sessionID,
		InputItems: []InputItem{{Type: "text", Text: "start live job"}},
	}); err != nil {
		t.Fatalf("SubmitTurn start returned error: %v", err)
	}
	projection, err := k.Session(sessionID)
	if err != nil {
		t.Fatalf("Session after start returned error: %v", err)
	}
	if len(projection.Jobs) != 1 || projection.Jobs[0].Status != "running" {
		t.Fatalf("jobs after start = %+v, want running live job", projection.Jobs)
	}
	jobID := projection.Jobs[0].JobID

	cancelArgs, err := json.Marshal(map[string]string{"job_id": jobID, "reason": "test cancellation"})
	if err != nil {
		t.Fatalf("marshal job_cancel args: %v", err)
	}
	cancelProvider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_cancel_live_job", Name: "job_cancel", Arguments: json.RawMessage(cancelArgs)},
		},
		final: "job cancellation requested",
	}
	k.provider = cancelProvider
	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  sessionID,
		InputItems: []InputItem{{Type: "text", Text: "cancel live job"}},
	}); err != nil {
		t.Fatalf("SubmitTurn cancel returned error: %v", err)
	}
	payload := decodeJSONMap(t, cancelProvider.Requests()[1].ToolRounds[0].Results[0].Content)
	if payload["status"] != "cancel_requested" || payload["cancel_requested"] != true {
		t.Fatalf("job_cancel payload = %+v, want cancel_requested receipt", payload)
	}
	cancelled := waitForSessionJobStatus(t, k, sessionID, jobID, "cancelled")
	if strings.TrimSpace(cancelled.CancelReason) != "test cancellation" {
		t.Fatalf("cancelled job = %+v, want cancel reason", cancelled)
	}
	projection, err = k.Session(sessionID)
	if err != nil {
		t.Fatalf("Session after cancellation returned error: %v", err)
	}
	if got := countSessionEventType(projection.Events, "job.cancel_requested"); got != 1 {
		t.Fatalf("job.cancel_requested count = %d, want 1", got)
	}
	if got := countSessionEventType(projection.Events, "job.cancelled"); got != 1 {
		t.Fatalf("job.cancelled count = %d, want 1", got)
	}
}

func TestSubmitTurnRejectsJobControlToolControlPlaneFields(t *testing.T) {
	for _, tc := range []struct {
		name      string
		tool      string
		arguments json.RawMessage
	}{
		{name: "status permission mode", tool: "job_status", arguments: json.RawMessage(`{"job_id":"job_x","permission_mode":"yolo"}`)},
		{name: "cancel terminate signal", tool: "job_cancel", arguments: json.RawMessage(`{"job_id":"job_x","reason":"stop","signal":"kill"}`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &toolFeedbackProvider{
				calls: []ModelToolCall{
					{ToolCallID: "call_" + strings.ReplaceAll(tc.name, " ", "_"), Name: tc.tool, Arguments: tc.arguments},
				},
				final: "repair observed",
			}
			k, err := New(Config{
				LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
				Provider:     provider,
				RuntimeToken: testRuntimeToken,
				ToolPolicy:   ToolPolicy{PermissionMode: PermissionModePlan},
			})
			if err != nil {
				t.Fatalf("New returned error: %v", err)
			}
			if _, err := k.SubmitTurn(context.Background(), TurnRequest{
				SessionID:  "job-control-field-" + strings.ReplaceAll(tc.name, " ", "-"),
				InputItems: []InputItem{{Type: "text", Text: "try invalid job control"}},
			}); err != nil {
				t.Fatalf("SubmitTurn returned error: %v", err)
			}
			payload := decodeJSONMap(t, provider.Requests()[1].ToolRounds[0].Results[0].Content)
			if payload["status"] != "tool_request_invalid" {
				t.Fatalf("%s payload = %+v, want repair feedback", tc.tool, payload)
			}
			errorPayload, ok := payload["error"].(map[string]interface{})
			if !ok || errorPayload["code"] != "invalid_tool_arguments" {
				t.Fatalf("%s error = %+v, want invalid_tool_arguments", tc.tool, payload["error"])
			}
		})
	}
}
