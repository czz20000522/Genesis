package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestInterruptSessionCancelsActiveProviderTurnWithoutCancellingBackgroundJob(t *testing.T) {
	workspace := testTempDir(t)
	executor := newBlockingManagedJobExecutor()
	startProvider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{
				ToolCallID: "call_start_background_job",
				Name:       "shell_exec",
				Arguments: mustJSONRaw(t, map[string]interface{}{
					"cwd":         workspace,
					"command":     longRunningShellCommand(30),
					"timeout_sec": 181,
				}),
			},
		},
		final: "background job started",
	}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     startProvider,
		JobExecutor:  executor,
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
		SessionID:  "interrupt-keeps-background",
		InputItems: []InputItem{{Type: "text", Text: "start background job"}},
	}); err != nil {
		t.Fatalf("SubmitTurn start returned error: %v", err)
	}
	startedJob := executor.startedJob(t)
	if startedJob.Status != "running" {
		t.Fatalf("started job = %+v, want running", startedJob)
	}

	blockingProvider := newBlockingProvider()
	k.provider = blockingProvider
	resultCh := make(chan submitTurnResult, 1)
	go func() {
		resp, err := k.SubmitTurn(context.Background(), TurnRequest{
			SessionID:  "interrupt-keeps-background",
			InputItems: []InputItem{{Type: "text", Text: "wait until interrupted"}},
		})
		resultCh <- submitTurnResult{response: resp, err: err}
	}()
	blockingProvider.waitStarted(t)

	interrupt, err := k.InterruptSession("interrupt-keeps-background", TurnInterruptRequest{Reason: "user pressed stop"})
	if err != nil {
		t.Fatalf("InterruptSession returned error: %v", err)
	}
	if interrupt.Status != "interrupt_requested" || interrupt.TurnID == "" {
		t.Fatalf("interrupt = %+v, want interrupt_requested with active turn id", interrupt)
	}

	result := waitSubmitTurnResult(t, resultCh)
	if !errors.Is(result.err, ErrTurnInterrupted) {
		t.Fatalf("SubmitTurn error = %v, want ErrTurnInterrupted", result.err)
	}
	if result.response.Error == nil || result.response.Error.Code != "turn_interrupted" {
		t.Fatalf("turn response = %+v, want turn_interrupted error", result.response)
	}

	projection, err := k.Session("interrupt-keeps-background")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if got := countSessionEventType(projection.Events, "assistant.interrupted"); got != 1 {
		t.Fatalf("assistant.interrupted count = %d, want 1", got)
	}
	if got := countSessionEventType(projection.Events, "turn.failed"); got != 0 {
		t.Fatalf("turn.failed count = %d, want interrupted turn not failure", got)
	}
	if got := executor.cancelCount(); got != 0 {
		t.Fatalf("executor cancel count = %d, want interrupt not to cancel background job", got)
	}
	if len(projection.Jobs) != 1 || projection.Jobs[0].Status != "running" {
		t.Fatalf("jobs = %+v, want existing background job still running", projection.Jobs)
	}
	var interruptedTurn TurnProjection
	for _, turn := range projection.Turns {
		if turn.TurnID == result.response.TurnID {
			interruptedTurn = turn
			break
		}
	}
	if interruptedTurn.Status != "interrupted" {
		t.Fatalf("interrupted turn = %+v, want interrupted status", interruptedTurn)
	}
}

func TestInterruptSessionDuringForegroundShellWritesInterruptedToolResult(t *testing.T) {
	workspace := testTempDir(t)
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{
				ToolCallID: "call_interrupt_foreground",
				Name:       "shell_exec",
				Arguments: mustJSONRaw(t, map[string]interface{}{
					"cwd":         workspace,
					"command":     longRunningShellCommand(30),
					"timeout_sec": 30,
				}),
			},
		},
		final: "must not reach final provider step",
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
	defer k.Close()

	resultCh := make(chan submitTurnResult, 1)
	go func() {
		resp, err := k.SubmitTurn(context.Background(), TurnRequest{
			SessionID:  "interrupt-foreground-shell",
			InputItems: []InputItem{{Type: "text", Text: "run foreground shell until interrupted"}},
		})
		resultCh <- submitTurnResult{response: resp, err: err}
	}()
	waitForSessionEventType(t, k, "interrupt-foreground-shell", "operation.running")

	if _, err := k.InterruptSession("interrupt-foreground-shell", TurnInterruptRequest{Reason: "stop foreground command"}); err != nil {
		t.Fatalf("InterruptSession returned error: %v", err)
	}
	result := waitSubmitTurnResult(t, resultCh)
	if !errors.Is(result.err, ErrTurnInterrupted) {
		t.Fatalf("SubmitTurn error = %v, want ErrTurnInterrupted", result.err)
	}

	projection, err := k.Session("interrupt-foreground-shell")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	for _, want := range []string{"tool.call", "operation.running", "operation.interrupted", "tool.result", "assistant.interrupted"} {
		if got := countSessionEventType(projection.Events, want); got != 1 {
			t.Fatalf("%s count = %d, want 1; events=%+v", want, got, projection.Events)
		}
	}
	if got := countSessionEventType(projection.Events, "model.final"); got != 0 {
		t.Fatalf("model.final count = %d, want no final provider step after interrupt", got)
	}
	if len(projection.Operations) != 1 || projection.Operations[0].Status != "interrupted" {
		t.Fatalf("operations = %+v, want interrupted foreground operation", projection.Operations)
	}
	var toolResult *ToolResultProjection
	for i := range projection.Events {
		event := projection.Events[i]
		if event.Type == "tool.result" && event.Data.ToolResult != nil {
			toolResult = event.Data.ToolResult
			break
		}
	}
	if toolResult == nil {
		t.Fatal("tool.result projection missing")
	}
	payload := decodeJSONMap(t, toolResult.Content)
	if payload["status"] != "interrupted" || payload["executed"] != true {
		t.Fatalf("tool result payload = %+v, want interrupted executed result", payload)
	}
	if payload["interrupt_reason"] != foregroundAttachUnavailableKilledReason {
		t.Fatalf("tool result payload = %+v, want foreground attach unavailable kill reason", payload)
	}
	if got := countSessionEventType(projection.Events, "job.started"); got != 0 {
		t.Fatalf("job.started count = %d, want no managed job when foreground attach is unavailable", got)
	}
}

func TestForegroundInterruptAttachesToManagedJobWhenExecutorSupportsAttach(t *testing.T) {
	workspace := testTempDir(t)
	executor := &foregroundAttachTestExecutor{
		attachResult: ManagedJobForegroundAttachResult{Attached: true},
		complete: &JobProjection{
			Status: "completed",
			Stdout: "attached command finished",
		},
		hostHandle: "host-pid-123 signal=TERM process_handle=abc",
	}
	k, provider, result := submitInterruptedForegroundShellTurn(t, "foreground-attach-success", workspace, executor, "")
	if !errors.Is(result.err, ErrTurnInterrupted) {
		t.Fatalf("SubmitTurn error = %v, want ErrTurnInterrupted", result.err)
	}
	if got := executor.attachCallCount(); got != 1 {
		t.Fatalf("attach calls = %d, want 1", got)
	}
	request := executor.attachRequest(t)
	if request.SessionID != "foreground-attach-success" || request.TurnID == "" || request.OperationID == "" {
		t.Fatalf("attach request = %+v, want kernel-owned session/turn/operation identity", request)
	}
	if strings.TrimSpace(request.Job.JobID) == "" || request.Job.SessionID != "foreground-attach-success" || request.Job.Tool != "shell_exec" {
		t.Fatalf("attach request job = %+v, want kernel-owned managed job identity", request.Job)
	}
	if len(provider.Requests()) != 1 {
		t.Fatalf("provider requests = %d, want no provider step after interrupted foreground attach", len(provider.Requests()))
	}

	projection, err := k.Session("foreground-attach-success")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	for _, want := range []string{"tool.call", "operation.running", "job.started", "operation.interrupted", "tool.result", "assistant.interrupted", "job.completed"} {
		if got := countSessionEventType(projection.Events, want); got != 1 {
			t.Fatalf("%s count = %d, want 1; events=%+v", want, got, projection.Events)
		}
	}
	if len(projection.Operations) != 1 || projection.Operations[0].InterruptReason != foregroundAttachedManagedJobReason {
		t.Fatalf("operations = %+v, want foreground attached interrupt reason", projection.Operations)
	}
	if len(projection.Jobs) != 1 || projection.Jobs[0].Status != "completed" || !strings.Contains(projection.Jobs[0].Stdout, "attached command finished") {
		t.Fatalf("jobs = %+v, want completed attached managed job", projection.Jobs)
	}
	toolResult := requireToolResultPayload(t, projection)
	if toolResult["status"] != "managed_job_started" || toolResult["executed"] != true {
		t.Fatalf("tool result payload = %+v, want managed job receipt", toolResult)
	}
	if strings.TrimSpace(fmt.Sprint(toolResult["job_id"])) == "" || !strings.Contains(fmt.Sprint(toolResult["visible_output"]), "continued as managed job") {
		t.Fatalf("tool result payload = %+v, want visible managed job continuation receipt", toolResult)
	}
}

func TestForegroundAttachFailureKeepsKillFallback(t *testing.T) {
	workspace := testTempDir(t)
	executor := &foregroundAttachTestExecutor{
		attachResult: ManagedJobForegroundAttachResult{Attached: false, FailureReason: "attach_not_available"},
	}
	k, _, result := submitInterruptedForegroundShellTurn(t, "foreground-attach-failure", workspace, executor, "")
	if !errors.Is(result.err, ErrTurnInterrupted) {
		t.Fatalf("SubmitTurn error = %v, want ErrTurnInterrupted", result.err)
	}
	projection, err := k.Session("foreground-attach-failure")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if got := countSessionEventType(projection.Events, "job.started"); got != 0 {
		t.Fatalf("job.started count = %d, want no forged managed job when attach fails", got)
	}
	toolResult := requireToolResultPayload(t, projection)
	if toolResult["status"] != "interrupted" || toolResult["interrupt_reason"] != foregroundAttachUnavailableKilledReason {
		t.Fatalf("tool result payload = %+v, want truthful kill fallback", toolResult)
	}
}

func TestForegroundAttachDoesNotExposeHostProcessHandle(t *testing.T) {
	workspace := testTempDir(t)
	executor := &foregroundAttachTestExecutor{
		attachResult: ManagedJobForegroundAttachResult{Attached: true},
		observe: JobProjection{
			Stdout: "attached progress",
		},
		hostHandle: "HOST_PROCESS_HANDLE_SHOULD_NOT_LEAK pid=123 SIGTERM",
	}
	k, _, result := submitInterruptedForegroundShellTurn(t, "foreground-attach-hidden-handle", workspace, executor, "")
	if !errors.Is(result.err, ErrTurnInterrupted) {
		t.Fatalf("SubmitTurn error = %v, want ErrTurnInterrupted", result.err)
	}
	projection, err := k.Session("foreground-attach-hidden-handle")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	encoded, err := json.Marshal(projection)
	if err != nil {
		t.Fatalf("marshal projection: %v", err)
	}
	for _, forbidden := range []string{"HOST_PROCESS_HANDLE_SHOULD_NOT_LEAK", "process_handle", "pid=123", "SIGTERM"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("projection leaked host process detail %q: %s", forbidden, string(encoded))
		}
	}
}

func TestForegroundAttachReplayDoesNotDuplicateManagedJob(t *testing.T) {
	workspace := testTempDir(t)
	executor := &foregroundAttachTestExecutor{
		attachResult: ManagedJobForegroundAttachResult{Attached: true},
	}
	k, _, result := submitInterruptedForegroundShellTurn(t, "foreground-attach-replay", workspace, executor, "replay-key")
	if !errors.Is(result.err, ErrTurnInterrupted) {
		t.Fatalf("SubmitTurn error = %v, want ErrTurnInterrupted", result.err)
	}
	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:      "foreground-attach-replay",
		IdempotencyKey: "replay-key",
		InputItems:     []InputItem{{Type: "text", Text: "same turn replay"}},
	}); !errors.Is(err, ErrTurnInterrupted) {
		t.Fatalf("replay SubmitTurn error = %v, want ErrTurnInterrupted", err)
	}
	if got := executor.attachCallCount(); got != 1 {
		t.Fatalf("attach calls after replay = %d, want original attach only", got)
	}
	projection, err := k.Session("foreground-attach-replay")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if got := countSessionEventType(projection.Events, "job.started"); got != 1 {
		t.Fatalf("job.started count = %d, want no duplicate managed job on replay", got)
	}
}

func TestAttachedManagedJobObservationRequiresContinuation(t *testing.T) {
	workspace := testTempDir(t)
	executor := &foregroundAttachTestExecutor{
		attachResult: ManagedJobForegroundAttachResult{Attached: true},
		complete: &JobProjection{
			Status: "completed",
			Stdout: "attached terminal output",
		},
	}
	k, provider, result := submitInterruptedForegroundShellTurn(t, "foreground-attach-continuation", workspace, executor, "")
	if !errors.Is(result.err, ErrTurnInterrupted) {
		t.Fatalf("SubmitTurn error = %v, want ErrTurnInterrupted", result.err)
	}
	if len(provider.Requests()) != 1 {
		t.Fatalf("provider requests = %d, want no autonomous provider step for attached job completion", len(provider.Requests()))
	}

	continuationProvider := &recordingTextProvider{text: "continued after attached job"}
	k.provider = continuationProvider
	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "foreground-attach-continuation",
		InputItems: []InputItem{{Type: "text", Text: "continue"}},
	}); err != nil {
		t.Fatalf("SubmitTurn continuation returned error: %v", err)
	}
	requests := continuationProvider.Requests()
	if len(requests) != 1 {
		t.Fatalf("continuation provider requests = %d, want 1", len(requests))
	}
	contextText, ok := modelInputTextByKind(requests[0].InputItems, ModelInputKindKernelObservationContext)
	if !ok || !strings.Contains(contextText, "attached terminal output") {
		t.Fatalf("kernel observation context = %q ok=%v, want attached terminal output on user-triggered continuation", contextText, ok)
	}
}

func TestAttachedJobOutputSnapshotIsBounded(t *testing.T) {
	workspace := testTempDir(t)
	executor := &foregroundAttachTestExecutor{
		attachResult: ManagedJobForegroundAttachResult{Attached: true},
		observe: JobProjection{
			Stdout: strings.Repeat("x", maxShellOutputBytes+managedJobOutputSnapshotMinBytes),
		},
	}
	k, _, result := submitInterruptedForegroundShellTurn(t, "foreground-attach-bounded-output", workspace, executor, "")
	if !errors.Is(result.err, ErrTurnInterrupted) {
		t.Fatalf("SubmitTurn error = %v, want ErrTurnInterrupted", result.err)
	}
	projection, err := k.Session("foreground-attach-bounded-output")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if got := countSessionEventType(projection.Events, "job.output"); got != 1 {
		t.Fatalf("job.output count = %d, want one attached output snapshot", got)
	}
	if len(projection.Jobs) != 1 || !projection.Jobs[0].StdoutTruncated || len(projection.Jobs[0].Stdout) > maxShellOutputBytes+len(outputOmissionMarkerFormat)+32 {
		t.Fatalf("job projection = %+v, want bounded/truncated attached output", projection.Jobs)
	}
}

func TestAttachedJobProgressDoesNotBecomeProviderContextByDefault(t *testing.T) {
	workspace := testTempDir(t)
	executor := &foregroundAttachTestExecutor{
		attachResult: ManagedJobForegroundAttachResult{Attached: true},
		observe: JobProjection{
			Stdout: "attached running progress",
		},
	}
	k, _, result := submitInterruptedForegroundShellTurn(t, "foreground-attach-progress-not-provider", workspace, executor, "")
	if !errors.Is(result.err, ErrTurnInterrupted) {
		t.Fatalf("SubmitTurn error = %v, want ErrTurnInterrupted", result.err)
	}
	continuationProvider := &recordingTextProvider{text: "continued without progress observation"}
	k.provider = continuationProvider
	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "foreground-attach-progress-not-provider",
		InputItems: []InputItem{{Type: "text", Text: "continue"}},
	}); err != nil {
		t.Fatalf("SubmitTurn continuation returned error: %v", err)
	}
	requests := continuationProvider.Requests()
	if len(requests) != 1 {
		t.Fatalf("continuation provider requests = %d, want 1", len(requests))
	}
	if contextText, ok := modelInputTextByKind(requests[0].InputItems, ModelInputKindKernelObservationContext); ok {
		t.Fatalf("provider context included non-terminal attached progress %q; only terminal job observations should be delivered", contextText)
	}
}

func TestLocalManagedJobExecutorDoesNotAdvertiseForegroundAttach(t *testing.T) {
	capabilities := managedJobExecutorCapabilities(newLocalManagedJobExecutor())
	if capabilities.ForegroundAttach {
		t.Fatalf("local managed executor capabilities = %+v, want no foreground attach support", capabilities)
	}
}

func TestForegroundAttachCapabilityRequiresAttachMethod(t *testing.T) {
	capabilities := managedJobExecutorCapabilities(attachAdvertisingManagedJobExecutor{})
	if capabilities.ForegroundAttach {
		t.Fatalf("foreground attach capability = %+v, want disabled without an attach method", capabilities)
	}

	capabilities = managedJobExecutorCapabilities(attachCapableManagedJobExecutor{})
	if !capabilities.ForegroundAttach {
		t.Fatalf("foreground attach capability = %+v, want enabled only with an attach method", capabilities)
	}
}

func TestForegroundInterruptReasonStaysKillFallbackUntilAttachIsImplemented(t *testing.T) {
	k, err := New(Config{
		LedgerPath:  filepath.Join(testTempDir(t), "events.jsonl"),
		JobExecutor: attachAdvertisingManagedJobExecutor{},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if reason := k.foregroundShellInterruptReason(); reason != foregroundAttachUnavailableKilledReason {
		t.Fatalf("foreground interrupt reason = %q, want truthful kill fallback until attach execution path exists", reason)
	}
}

func TestHTTPInterruptSessionRequestsActiveTurnCancellation(t *testing.T) {
	provider := newBlockingProvider()
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	server := httptest.NewServer(Handler(k))
	defer server.Close()
	defer server.CloseClientConnections()

	turnResultCh := make(chan httpTurnResult, 1)
	go func() {
		resp, err := postJSONWithAuth(server.URL+"/turn", []byte(`{"session_id":"http-interrupt-session","input_items":[{"type":"text","text":"wait"}]}`))
		if err != nil {
			turnResultCh <- httpTurnResult{err: err}
			return
		}
		defer resp.Body.Close()
		body, readErr := io.ReadAll(resp.Body)
		turnResultCh <- httpTurnResult{status: resp.StatusCode, body: body, err: readErr}
	}()
	provider.waitStarted(t)

	interruptResp, err := postJSONWithAuth(server.URL+"/sessions/http-interrupt-session/interrupt", []byte(`{"reason":"operator stop"}`))
	if err != nil {
		t.Fatalf("POST interrupt failed: %v", err)
	}
	defer interruptResp.Body.Close()
	if interruptResp.StatusCode != http.StatusAccepted {
		t.Fatalf("interrupt status = %d, want 202", interruptResp.StatusCode)
	}
	var interrupt TurnInterruptionProjection
	if err := json.NewDecoder(interruptResp.Body).Decode(&interrupt); err != nil {
		t.Fatalf("decode interrupt response: %v", err)
	}
	if interrupt.Status != "interrupt_requested" || interrupt.TurnID == "" {
		t.Fatalf("interrupt = %+v, want interrupt_requested", interrupt)
	}

	turnResult := waitHTTPTurnResult(t, turnResultCh)
	if turnResult.err != nil {
		t.Fatalf("POST /turn result error: %v", turnResult.err)
	}
	if turnResult.status != http.StatusConflict {
		t.Fatalf("turn status = %d body=%s, want 409 interrupted response", turnResult.status, string(turnResult.body))
	}
	if !jsonBodyContains(t, turnResult.body, "turn_interrupted") {
		t.Fatalf("turn body = %s, want turn_interrupted", string(turnResult.body))
	}
}

type submitTurnResult struct {
	response TurnResponse
	err      error
}

type blockingProvider struct {
	started chan struct{}
	once    sync.Once
}

func newBlockingProvider() *blockingProvider {
	return &blockingProvider{started: make(chan struct{})}
}

func (p *blockingProvider) Name() string {
	return "blocking-provider"
}

func (p *blockingProvider) Ready() ProviderStatus {
	return ProviderStatus{Name: p.Name(), Readiness: ReadinessReady}
}

func (p *blockingProvider) Complete(ctx context.Context, _ ModelRequest) (ModelResponse, error) {
	p.once.Do(func() {
		close(p.started)
	})
	<-ctx.Done()
	return ModelResponse{}, ctx.Err()
}

func (p *blockingProvider) waitStarted(t *testing.T) {
	t.Helper()
	select {
	case <-p.started:
	case <-time.After(2 * time.Second):
		t.Fatal("provider did not start")
	}
}

type blockingManagedJobExecutor struct {
	mu        sync.Mutex
	started   []JobProjection
	cancels   int
	startedCh chan JobProjection
}

type attachAdvertisingManagedJobExecutor struct{}

func (attachAdvertisingManagedJobExecutor) Start(_ context.Context, _ ManagedJobStartRequest) error {
	return nil
}

func (attachAdvertisingManagedJobExecutor) Cancel(_ string, _ string) (bool, error) {
	return false, nil
}

func (attachAdvertisingManagedJobExecutor) Capabilities() ManagedJobExecutorCapabilities {
	return ManagedJobExecutorCapabilities{ForegroundAttach: true}
}

type attachCapableManagedJobExecutor struct {
	attachAdvertisingManagedJobExecutor
}

func (attachCapableManagedJobExecutor) AttachForeground(_ context.Context, _ ManagedJobForegroundAttachRequest) (ManagedJobForegroundAttachResult, error) {
	return ManagedJobForegroundAttachResult{Attached: false, FailureReason: "test_only"}, nil
}

type foregroundAttachTestExecutor struct {
	attachAdvertisingManagedJobExecutor

	mu           sync.Mutex
	attachCalls  int
	requests     []ManagedJobForegroundAttachRequest
	attachResult ManagedJobForegroundAttachResult
	attachErr    error
	observe      JobProjection
	complete     *JobProjection
	hostHandle   string
}

func (e *foregroundAttachTestExecutor) AttachForeground(_ context.Context, request ManagedJobForegroundAttachRequest) (ManagedJobForegroundAttachResult, error) {
	e.mu.Lock()
	e.attachCalls++
	e.requests = append(e.requests, request)
	result := e.attachResult
	err := e.attachErr
	observe := e.observe
	var complete *JobProjection
	if e.complete != nil {
		copied := *e.complete
		complete = &copied
	}
	_ = e.hostHandle
	e.mu.Unlock()
	if err != nil {
		return ManagedJobForegroundAttachResult{}, err
	}
	if result.Attached {
		if request.Observe != nil && (strings.TrimSpace(observe.Stdout) != "" || strings.TrimSpace(observe.Stderr) != "") {
			request.Observe(observe)
		}
		if request.Complete != nil && complete != nil {
			request.Complete(*complete)
		}
	}
	return result, nil
}

func (e *foregroundAttachTestExecutor) attachCallCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.attachCalls
}

func (e *foregroundAttachTestExecutor) attachRequest(t *testing.T) ManagedJobForegroundAttachRequest {
	t.Helper()
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.requests) == 0 {
		t.Fatal("attach request missing")
	}
	return e.requests[0]
}

func submitInterruptedForegroundShellTurn(t *testing.T, sessionID string, workspace string, executor ManagedJobExecutor, idempotencyKey string) (*Kernel, *toolFeedbackProvider, submitTurnResult) {
	t.Helper()
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{
				ToolCallID: "call_interrupt_foreground",
				Name:       "shell_exec",
				Arguments: mustJSONRaw(t, map[string]interface{}{
					"cwd":         workspace,
					"command":     longRunningShellCommand(30),
					"timeout_sec": 30,
				}),
			},
		},
		final: "must not reach final provider step",
	}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     provider,
		JobExecutor:  executor,
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
			SessionID:      sessionID,
			IdempotencyKey: idempotencyKey,
			InputItems:     []InputItem{{Type: "text", Text: "run foreground shell until interrupted"}},
		})
		resultCh <- submitTurnResult{response: resp, err: err}
	}()
	waitForSessionEventType(t, k, sessionID, "operation.running")
	if _, err := k.InterruptSession(sessionID, TurnInterruptRequest{Reason: "stop foreground command"}); err != nil {
		t.Fatalf("InterruptSession returned error: %v", err)
	}
	return k, provider, waitSubmitTurnResult(t, resultCh)
}

func requireToolResultPayload(t *testing.T, projection SessionProjection) map[string]interface{} {
	t.Helper()
	for _, event := range projection.Events {
		if event.Type == "tool.result" && event.Data.ToolResult != nil {
			return decodeJSONMap(t, event.Data.ToolResult.Content)
		}
	}
	t.Fatal("tool.result projection missing")
	return nil
}

func newBlockingManagedJobExecutor() *blockingManagedJobExecutor {
	return &blockingManagedJobExecutor{startedCh: make(chan JobProjection, 1)}
}

func (e *blockingManagedJobExecutor) Start(_ context.Context, request ManagedJobStartRequest) error {
	e.mu.Lock()
	e.started = append(e.started, request.Job)
	e.mu.Unlock()
	e.startedCh <- request.Job
	return nil
}

func (e *blockingManagedJobExecutor) Cancel(_ string, _ string) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cancels++
	return true, nil
}

func (e *blockingManagedJobExecutor) startedJob(t *testing.T) JobProjection {
	t.Helper()
	select {
	case job := <-e.startedCh:
		return job
	case <-time.After(2 * time.Second):
		t.Fatal("managed job was not started")
		return JobProjection{}
	}
}

func (e *blockingManagedJobExecutor) cancelCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cancels
}

func waitSubmitTurnResult(t *testing.T, ch <-chan submitTurnResult) submitTurnResult {
	t.Helper()
	select {
	case result := <-ch:
		return result
	case <-time.After(5 * time.Second):
		t.Fatal("SubmitTurn did not return")
		return submitTurnResult{}
	}
}

type httpTurnResult struct {
	status int
	body   []byte
	err    error
}

func waitHTTPTurnResult(t *testing.T, ch <-chan httpTurnResult) httpTurnResult {
	t.Helper()
	select {
	case result := <-ch:
		return result
	case <-time.After(5 * time.Second):
		t.Fatal("HTTP /turn did not return")
		return httpTurnResult{}
	}
}

func waitForSessionEventType(t *testing.T, k *Kernel, sessionID string, eventType string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		projection, err := k.Session(sessionID)
		if err == nil && countSessionEventType(projection.Events, eventType) > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("session %s did not record %s", sessionID, eventType)
}

func mustJSONRaw(t *testing.T, value interface{}) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	return json.RawMessage(data)
}

func jsonBodyContains(t *testing.T, body []byte, needle string) bool {
	t.Helper()
	var decoded interface{}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode JSON body: %v", err)
	}
	encoded, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("re-marshal JSON body: %v", err)
	}
	return strings.Contains(string(encoded), needle)
}
