package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func commandProviderTestRequest() ModelRequest {
	return ModelRequest{
		SessionID:  "command-session",
		TurnID:     "turn-command",
		InputItems: []ModelInputItem{{Kind: ModelInputKindUserText, Text: "hello command provider"}},
		ToolManifest: []ToolSpec{{
			Name:            "shell_exec",
			Description:     "execute a governed shell command",
			InputSchema:     map[string]interface{}{"type": "object"},
			SideEffectLevel: "write",
			ExecutionKind:   "sandboxed_process",
		}},
	}
}

func providerToolNamesFromRequest(t *testing.T, tools []interface{}) []string {
	t.Helper()
	names := make([]string, 0, len(tools))
	for _, item := range tools {
		tool, ok := item.(map[string]interface{})
		if !ok {
			t.Fatalf("tool descriptor = %#v", item)
		}
		function, ok := tool["function"].(map[string]interface{})
		if !ok {
			t.Fatalf("tool function = %#v", tool["function"])
		}
		name, _ := function["name"].(string)
		names = append(names, name)
	}
	return names
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func toolSpecNames(specs []ToolSpec) []string {
	names := make([]string, 0, len(specs))
	for _, spec := range specs {
		names = append(names, spec.Name)
	}
	return names
}

func inspectedToolNames(items []ToolManifestInspection) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return names
}

func countSessionEventType(events []EventProjection, eventType string) int {
	count := 0
	for _, event := range events {
		if event.Type == eventType {
			count++
		}
	}
	return count
}

func waitForSessionJobStatus(t *testing.T, k *Kernel, sessionID string, jobID string, status string) JobProjection {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var latest JobProjection
	for time.Now().Before(deadline) {
		projection, err := k.Session(sessionID)
		if err != nil {
			t.Fatalf("Session returned error while waiting for job %s: %v", jobID, err)
		}
		for _, job := range projection.Jobs {
			if job.JobID != jobID {
				continue
			}
			latest = job
			if job.Status == status {
				return job
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("job %s status = %+v, want %s", jobID, latest, status)
	return JobProjection{}
}

func modelInputTextByKind(items []ModelInputItem, kind string) (string, bool) {
	for _, item := range items {
		if item.Kind == kind {
			return item.Text, true
		}
	}
	return "", false
}

func submitCompletedManagedJobForTest(t *testing.T, ledgerPath string, workspace string, sessionID string) string {
	t.Helper()
	arguments, err := json.Marshal(map[string]interface{}{
		"cwd":         workspace,
		"command":     echoCommand("managed-job"),
		"timeout_sec": 181,
	})
	if err != nil {
		t.Fatalf("marshal shell args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_create_managed_job", Name: "shell_exec", Arguments: json.RawMessage(arguments)},
		},
		final: "managed job created",
	}
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
	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  sessionID,
		InputItems: []InputItem{{Type: "text", Text: "create managed job"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	projection, err := k.Session(sessionID)
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Jobs) != 1 || projection.Jobs[0].JobID == "" || projection.Jobs[0].Status != "completed" {
		t.Fatalf("jobs = %+v, want one completed managed job", projection.Jobs)
	}
	return projection.Jobs[0].JobID
}

func assertHeadTailOmissionMarker(t *testing.T, streamName string, text string, headMarker string, tailMarker string) {
	t.Helper()
	headAt := strings.Index(text, headMarker)
	omissionAt := strings.Index(text, " bytes omitted ...]")
	tailAt := strings.Index(text, tailMarker)
	if headAt < 0 || omissionAt < 0 || tailAt < 0 || !(headAt < omissionAt && omissionAt < tailAt) {
		t.Fatalf("%s = %q, want visible omission marker between head and tail", streamName, text)
	}
}

func providerCommandHelperMode() string {
	mode := ""
	if len(os.Args) > 0 {
		mode = os.Args[len(os.Args)-1]
	}
	if len(os.Args) >= 2 && os.Args[len(os.Args)-2] == "tool-loop" {
		mode = "tool-loop"
	}
	return mode
}

func writeProviderCommandHelperResponse(t *testing.T, resp providerCommandResponse) {
	t.Helper()
	if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
		t.Fatalf("write provider command response: %v", err)
	}
	os.Exit(0)
}

const testRuntimeToken = "test-runtime-token"

func testApprovalRequest(evidenceRef string) MemoryApprovalRequest {
	return MemoryApprovalRequest{
		ApprovalAuthority:   "runtime:test",
		ApprovalReason:      "approved in test",
		ApprovalEvidenceRef: evidenceRef,
	}
}

func createMemoryCandidateOverHTTP(t *testing.T, serverURL string, req MemoryCandidateRequest) MemoryCandidateProjection {
	t.Helper()
	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal candidate request: %v", err)
	}
	resp, err := postJSONWithAuth(serverURL+"/memory/candidates", payload)
	if err != nil {
		t.Fatalf("POST /memory/candidates failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("candidate status = %d, want 200", resp.StatusCode)
	}
	var candidate MemoryCandidateProjection
	if err := json.NewDecoder(resp.Body).Decode(&candidate); err != nil {
		t.Fatalf("decode candidate response: %v", err)
	}
	return candidate
}

type staticLedger struct {
	events []StoredEvent
}

func newStaticLedger(events ...StoredEvent) *staticLedger {
	return &staticLedger{events: append([]StoredEvent(nil), events...)}
}

func (l *staticLedger) Append(event StoredEvent) error {
	l.events = append(l.events, event)
	return nil
}

func (l *staticLedger) Load() ([]StoredEvent, error) {
	return append([]StoredEvent(nil), l.events...), nil
}

func (l *staticLedger) Ready() ReadyCheck {
	return ReadyCheck{Readiness: ReadinessReady}
}

func (l *staticLedger) Path() string {
	return "static-ledger"
}

type failOnOperationLedger struct {
	mu     sync.Mutex
	events []StoredEvent
}

func (l *failOnOperationLedger) Append(event StoredEvent) error {
	if strings.HasPrefix(event.Type, "operation.") {
		return ErrLedgerUnwritable
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
	return nil
}

func (l *failOnOperationLedger) Load() ([]StoredEvent, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]StoredEvent(nil), l.events...), nil
}

func (l *failOnOperationLedger) Ready() ReadyCheck {
	return ReadyCheck{Readiness: ReadinessReady}
}

func (l *failOnOperationLedger) Path() string {
	return "fail-on-operation-ledger"
}

type reviewRaceLedger struct {
	mu                         sync.Mutex
	events                     []StoredEvent
	firstTerminalAppendStarted chan struct{}
	secondReviewLoadObserved   chan struct{}
	firstAppendOnce            sync.Once
	secondLoadOnce             sync.Once
}

func newReviewRaceLedger(events ...StoredEvent) *reviewRaceLedger {
	copied := append([]StoredEvent(nil), events...)
	return &reviewRaceLedger{
		events:                     copied,
		firstTerminalAppendStarted: make(chan struct{}),
		secondReviewLoadObserved:   make(chan struct{}),
	}
}

func (l *reviewRaceLedger) Append(event StoredEvent) error {
	if isMemoryReviewTerminalEvent(event.Type) {
		l.firstAppendOnce.Do(func() {
			close(l.firstTerminalAppendStarted)
		})
		select {
		case <-l.secondReviewLoadObserved:
		case <-time.After(250 * time.Millisecond):
		}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
	return nil
}

func (l *reviewRaceLedger) Load() ([]StoredEvent, error) {
	l.mu.Lock()
	events := append([]StoredEvent(nil), l.events...)
	l.mu.Unlock()
	select {
	case <-l.firstTerminalAppendStarted:
		l.secondLoadOnce.Do(func() {
			close(l.secondReviewLoadObserved)
		})
	default:
	}
	return events, nil
}

func (l *reviewRaceLedger) Ready() ReadyCheck {
	return ReadyCheck{Readiness: ReadinessReady}
}

func (l *reviewRaceLedger) Path() string {
	return "review-race-ledger"
}

func (l *reviewRaceLedger) terminalReviewEvents(candidateID string) []StoredEvent {
	l.mu.Lock()
	defer l.mu.Unlock()
	var terminal []StoredEvent
	for _, event := range l.events {
		if event.CandidateID == candidateID && isMemoryReviewTerminalEvent(event.Type) {
			terminal = append(terminal, event)
		}
	}
	return terminal
}

func isMemoryReviewTerminalEvent(eventType string) bool {
	return eventType == "memory.candidate.approved" ||
		eventType == "memory.candidate.rejected" ||
		eventType == "memory.candidate.superseded" ||
		eventType == "memory.candidate.forgotten"
}

type singleToolCallProvider struct {
	call ModelToolCall
}

type multiToolCallProvider struct {
	calls []ModelToolCall
}

type toolFeedbackProvider struct {
	mu       sync.Mutex
	calls    []ModelToolCall
	final    string
	usages   []*TokenUsage
	requests []ModelRequest
}

type jobObservationFailingProvider struct {
	mu       sync.Mutex
	call     ModelToolCall
	requests []ModelRequest
}

type completingManagedJobExecutor struct{}

type countingTextProvider struct {
	mu    sync.Mutex
	calls int
	text  string
}

type recordingTextProvider struct {
	mu       sync.Mutex
	text     string
	requests []ModelRequest
}

func (completingManagedJobExecutor) Start(_ context.Context, request ManagedJobStartRequest) error {
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

func (completingManagedJobExecutor) Cancel(_ string, _ string) (bool, error) {
	return false, nil
}

func (p singleToolCallProvider) Name() string {
	return "single-tool-call"
}

func (p singleToolCallProvider) Ready() ProviderStatus {
	return ProviderStatus{Name: p.Name(), Readiness: ReadinessReady}
}

func (p singleToolCallProvider) Complete(_ context.Context, _ ModelRequest) (ModelResponse, error) {
	return ModelResponse{
		Model:     "single-tool-call-model",
		ToolCalls: []ModelToolCall{p.call},
	}, nil
}

func (p multiToolCallProvider) Name() string {
	return "multi-tool-call"
}

func (p multiToolCallProvider) Ready() ProviderStatus {
	return ProviderStatus{Name: p.Name(), Readiness: ReadinessReady}
}

func (p multiToolCallProvider) Complete(_ context.Context, _ ModelRequest) (ModelResponse, error) {
	return ModelResponse{
		Model:     "multi-tool-call-model",
		ToolCalls: p.calls,
	}, nil
}

func (p *toolFeedbackProvider) Name() string {
	return "tool-feedback"
}

func (p *toolFeedbackProvider) Ready() ProviderStatus {
	return ProviderStatus{Name: p.Name(), Readiness: ReadinessReady}
}

func (p *toolFeedbackProvider) Complete(_ context.Context, req ModelRequest) (ModelResponse, error) {
	p.mu.Lock()
	p.requests = append(p.requests, req)
	callCount := len(p.requests)
	var usage *TokenUsage
	if callCount <= len(p.usages) {
		usage = p.usages[callCount-1]
	}
	p.mu.Unlock()
	if callCount == 1 {
		return ModelResponse{
			Model:     "tool-feedback-model",
			Usage:     usage,
			ToolCalls: p.calls,
		}, nil
	}
	final := p.final
	if final == "" {
		final = "tool feedback observed"
	}
	return ModelResponse{
		Text:  final,
		Model: "tool-feedback-model",
		Usage: usage,
	}, nil
}

func (p *toolFeedbackProvider) Requests() []ModelRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]ModelRequest(nil), p.requests...)
}

func (p *jobObservationFailingProvider) Name() string {
	return "job-observation-failing"
}

func (p *jobObservationFailingProvider) Ready() ProviderStatus {
	return ProviderStatus{Name: p.Name(), Readiness: ReadinessReady}
}

func (p *jobObservationFailingProvider) Complete(_ context.Context, req ModelRequest) (ModelResponse, error) {
	p.mu.Lock()
	p.requests = append(p.requests, req)
	callCount := len(p.requests)
	p.mu.Unlock()
	if callCount == 1 {
		return ModelResponse{
			Model:     "job-observation-failing-model",
			ToolCalls: []ModelToolCall{p.call},
		}, nil
	}
	return ModelResponse{}, errors.New("provider failed after observation")
}

func (p *jobObservationFailingProvider) Requests() []ModelRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]ModelRequest(nil), p.requests...)
}

func (p *countingTextProvider) Name() string {
	return "counting-text"
}

func (p *countingTextProvider) Ready() ProviderStatus {
	return ProviderStatus{Name: p.Name(), Readiness: ReadinessReady}
}

func (p *countingTextProvider) Complete(_ context.Context, _ ModelRequest) (ModelResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	return ModelResponse{
		Text:  p.text,
		Model: "counting-text-model",
	}, nil
}

func (p *countingTextProvider) Calls() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

func (p *recordingTextProvider) Name() string {
	return "recording-text"
}

func (p *recordingTextProvider) Ready() ProviderStatus {
	return ProviderStatus{Name: p.Name(), Readiness: ReadinessReady}
}

func (p *recordingTextProvider) Complete(_ context.Context, req ModelRequest) (ModelResponse, error) {
	p.mu.Lock()
	p.requests = append(p.requests, req)
	p.mu.Unlock()
	text := p.text
	if text == "" {
		text = "recorded"
	}
	return ModelResponse{Text: text, Model: "recording-text-model"}, nil
}

func (p *recordingTextProvider) Requests() []ModelRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]ModelRequest(nil), p.requests...)
}

func newTestKernel(t *testing.T, ledgerPath string) *Kernel {
	t.Helper()
	return newTestKernelWithRuntimeTokenAndPolicy(t, ledgerPath, testRuntimeToken, ToolPolicy{
		PermissionMode: PermissionModePlan,
	})
}

func newTestKernelWithPolicy(t *testing.T, ledgerPath string, policy ToolPolicy) *Kernel {
	t.Helper()
	return newTestKernelWithRuntimeTokenAndPolicy(t, ledgerPath, testRuntimeToken, policy)
}

func newTestKernelWithRuntimeToken(t *testing.T, ledgerPath string, token string) *Kernel {
	t.Helper()
	return newTestKernelWithRuntimeTokenAndPolicy(t, ledgerPath, token, ToolPolicy{
		PermissionMode: PermissionModePlan,
	})
}

func newTestKernelWithRuntimeTokenAndPolicy(t *testing.T, ledgerPath string, token string, policy ToolPolicy) *Kernel {
	t.Helper()
	k, err := New(Config{
		LedgerPath:   ledgerPath,
		Provider:     FakeProvider{},
		RuntimeToken: token,
		ToolPolicy:   policy,
		Clock: func() time.Time {
			return time.Date(2026, 6, 22, 1, 2, 3, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	t.Cleanup(k.Close)
	return k
}

type compactionProvider struct {
	normalRequests          []ModelRequest
	normalUsages            []*TokenUsage
	normalAttemptNumber     int
	compactionRequests      []ModelRequest
	failCompactionAttempts  int
	compactionAttemptNumber int
}

func (p *compactionProvider) Name() string {
	return "compaction-provider"
}

func (p *compactionProvider) Ready() ProviderStatus {
	return ProviderStatus{Name: p.Name(), Readiness: ReadinessReady}
}

func (p *compactionProvider) Complete(_ context.Context, req ModelRequest) (ModelResponse, error) {
	if len(req.InputItems) > 0 && req.InputItems[0].Kind == "context_compaction_source" {
		p.compactionRequests = append(p.compactionRequests, req)
		p.compactionAttemptNumber++
		if p.compactionAttemptNumber <= p.failCompactionAttempts {
			return ModelResponse{}, errors.New("compaction summarizer unavailable")
		}
		return ModelResponse{
			Text:  "summary of compacted earlier context",
			Model: "compaction-model",
			Usage: &TokenUsage{InputTokens: 4, OutputTokens: 2, TotalTokens: 6},
		}, nil
	}
	p.normalRequests = append(p.normalRequests, req)
	usage := &TokenUsage{InputTokens: 9, OutputTokens: 1, TotalTokens: 10}
	if p.normalAttemptNumber < len(p.normalUsages) {
		usage = p.normalUsages[p.normalAttemptNumber]
	}
	p.normalAttemptNumber++
	return ModelResponse{
		Text:  "normal answer",
		Model: "chat-model",
		Usage: usage,
	}, nil
}

type compactionToolPairProvider struct {
	requests           []ModelRequest
	compactionRequests []ModelRequest
}

func (p *compactionToolPairProvider) Name() string {
	return "compaction-tool-pair"
}

func (p *compactionToolPairProvider) Ready() ProviderStatus {
	return ProviderStatus{Name: p.Name(), Readiness: ReadinessReady}
}

func (p *compactionToolPairProvider) Complete(_ context.Context, req ModelRequest) (ModelResponse, error) {
	if len(req.InputItems) > 0 && req.InputItems[0].Kind == "context_compaction_source" {
		p.compactionRequests = append(p.compactionRequests, req)
		return ModelResponse{
			Text:  "summary with tool pair",
			Model: "compaction-model",
			Usage: &TokenUsage{InputTokens: 10, OutputTokens: 2, TotalTokens: 12},
		}, nil
	}
	p.requests = append(p.requests, req)
	if len(req.ToolRounds) == 0 && len(p.requests) == 1 {
		return ModelResponse{
			Model: "tool-pair-model",
			Usage: &TokenUsage{InputTokens: 9, OutputTokens: 1, TotalTokens: 10, CacheMissTokens: 9},
			ToolCalls: []ModelToolCall{{
				ToolCallID: "call_tool_pair",
				Name:       "shell_exec",
				Arguments:  json.RawMessage(`{"cwd":"C:\\tmp","command":"echo GENESIS_TOOL_PAIR_MARKER"}`),
			}},
		}, nil
	}
	text := "normal answer"
	if len(req.ToolRounds) > 0 {
		text = "tool pair final"
	}
	return ModelResponse{
		Text:  text,
		Model: "tool-pair-model",
		Usage: &TokenUsage{InputTokens: 9, OutputTokens: 1, TotalTokens: 10, CacheMissTokens: 9},
	}, nil
}

func ledgerPathUnderFile(t *testing.T) string {
	t.Helper()
	root := testTempDir(t)
	filePath := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(filePath, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("write non-directory ledger parent: %v", err)
	}
	return filepath.Join(filePath, "events.sqlite")
}

func corruptLedgerPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join(testTempDir(t), "events.sqlite")
	if err := os.WriteFile(path, []byte("not a sqlite database\n"), 0o644); err != nil {
		t.Fatalf("write corrupt ledger: %v", err)
	}
	return path
}

func assertErrorCode(t *testing.T, resp *http.Response, status int, code string) {
	t.Helper()
	if resp.StatusCode != status {
		t.Fatalf("status = %d, want %d", resp.StatusCode, status)
	}
	var envelope errorEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode error envelope: %v", err)
	}
	if envelope.Error.Code != code {
		t.Fatalf("error code = %q, want %s", envelope.Error.Code, code)
	}
}

func assertJSONUsage(t *testing.T, finalValue interface{}, inputTokens int, outputTokens int, totalTokens int) {
	t.Helper()
	final, ok := finalValue.(map[string]interface{})
	if !ok {
		t.Fatalf("final = %#v, want object", finalValue)
	}
	usage, ok := final["usage"].(map[string]interface{})
	if !ok {
		t.Fatalf("final.usage = %#v, want usage object", final["usage"])
	}
	assertJSONNumber(t, usage, "input_tokens", inputTokens)
	assertJSONNumber(t, usage, "output_tokens", outputTokens)
	assertJSONNumber(t, usage, "total_tokens", totalTokens)
}

func assertJSONNumber(t *testing.T, values map[string]interface{}, key string, want int) {
	t.Helper()
	got, ok := values[key].(float64)
	if !ok {
		t.Fatalf("%s = %#v, want JSON number", key, values[key])
	}
	if int(got) != want {
		t.Fatalf("%s = %d, want %d", key, int(got), want)
	}
}

func assertBoolMapValue(t *testing.T, values map[string]interface{}, key string, want bool) {
	t.Helper()
	got, ok := values[key].(bool)
	if !ok || got != want {
		t.Fatalf("%s = %#v, want %v", key, values[key], want)
	}
}

func assertStringMapValue(t *testing.T, values map[string]interface{}, key string, want string) {
	t.Helper()
	got, ok := values[key].(string)
	if !ok || got != want {
		t.Fatalf("%s = %#v, want %q", key, values[key], want)
	}
}

func assertMapNumberGreaterThan(t *testing.T, values map[string]interface{}, key string, floor int) {
	t.Helper()
	got, ok := values[key].(float64)
	if !ok {
		t.Fatalf("%s = %#v, want JSON number", key, values[key])
	}
	if int(got) <= floor {
		t.Fatalf("%s = %d, want > %d", key, int(got), floor)
	}
}

func decodeJSONMap(t *testing.T, content string) map[string]interface{} {
	t.Helper()
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		t.Fatalf("decode JSON content: %v; content=%s", err, content)
	}
	return payload
}

func toolRepairPayloadByCallID(t *testing.T, results []ModelToolResult) map[string]map[string]interface{} {
	t.Helper()
	payloads := make(map[string]map[string]interface{}, len(results))
	for _, result := range results {
		payloads[result.ToolCallID] = decodeJSONMap(t, result.Content)
	}
	return payloads
}

func operationJSONMap(t *testing.T, operation OperationProjection) map[string]interface{} {
	t.Helper()
	data, err := json.Marshal(operation)
	if err != nil {
		t.Fatalf("marshal operation: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("decode operation JSON: %v", err)
	}
	return payload
}

func postJSONWithAuth(url string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+testRuntimeToken)
	req.Header.Set("Content-Type", "application/json")
	return http.DefaultClient.Do(req)
}

func getWithAuth(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+testRuntimeToken)
	return http.DefaultClient.Do(req)
}

func writeFileCommand(filename string, value string) string {
	if runtime.GOOS == "windows" {
		return "Set-Content -LiteralPath " + filename + " -Value " + value + " -NoNewline"
	}
	return "printf " + value + " > " + filename
}

func echoCommand(value string) string {
	if runtime.GOOS == "windows" {
		return "Write-Output " + value
	}
	return "printf " + value
}

func longRunningShellCommand(seconds int) string {
	if seconds <= 0 {
		seconds = 30
	}
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("Start-Sleep -Seconds %d", seconds)
	}
	return fmt.Sprintf("sleep %d", seconds)
}

func timeoutAfterOutputCommand() string {
	if runtime.GOOS == "windows" {
		return "Write-Output before-timeout; Start-Sleep -Seconds 3"
	}
	return "printf before-timeout; sleep 3"
}

func readMissingFileCommand(filename string) string {
	if runtime.GOOS == "windows" {
		return "Get-Content -LiteralPath " + filename
	}
	return "cat " + filename
}

func failingShellCommand() string {
	if runtime.GOOS == "windows" {
		return `Write-Error 'GENESIS_TOOL_COMMAND_FAILURE'; exit 7`
	}
	return `printf '%s\n' 'GENESIS_TOOL_COMMAND_FAILURE' >&2; exit 7`
}

func longStdoutStderrCommand() string {
	if runtime.GOOS == "windows" {
		return `$out = 'GENESIS_STDOUT_HEAD' + ('A' * 70000) + 'GENESIS_STDOUT_TAIL'; $err = 'GENESIS_STDERR_HEAD' + ('B' * 70000) + 'GENESIS_STDERR_TAIL'; [Console]::Out.Write($out); [Console]::Error.Write($err)`
	}
	return `printf 'GENESIS_STDOUT_HEAD'; yes A | head -c 70000; printf 'GENESIS_STDOUT_TAIL'; { printf 'GENESIS_STDERR_HEAD'; yes B | head -c 70000; printf 'GENESIS_STDERR_TAIL'; } >&2`
}

func secretEchoCommand() string {
	if runtime.GOOS == "windows" {
		return `Write-Output 'GENESIS_PROVIDER_API_KEY=sk-secret123'; Write-Output 'Authorization: Bearer tokentest123456'; Write-Output '{"api_key":"sk-jsonsecret"}'`
	}
	return `printf '%s\n' 'GENESIS_PROVIDER_API_KEY=sk-secret123' 'Authorization: Bearer tokentest123456' '{"api_key":"sk-jsonsecret"}'`
}

func createDirectoryLinkForTest(t *testing.T, target string, link string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		cmd := exec.Command("cmd.exe", "/c", "mklink", "/J", link, target)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("create junction failed: %v; output=%s", err, string(output))
		}
		t.Cleanup(func() {
			_ = exec.Command("cmd.exe", "/c", "rmdir", link).Run()
		})
		return
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("create symlink failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Remove(link)
	})
}
