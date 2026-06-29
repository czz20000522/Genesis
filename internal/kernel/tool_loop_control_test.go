package kernel

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestSubmitTurnPausesToolLoopBudgetWithoutExecutingOverBudgetBatch(t *testing.T) {
	dir := testTempDir(t)
	provider := &repeatingToolProvider{
		toolName: "resource_read",
		args: func(round int) json.RawMessage {
			return mustMarshalToolArgs(t, map[string]interface{}{
				"resource_ref": "cf:tool-loop-pause",
				"limit_bytes":  64,
			})
		},
	}
	k := newTestKernelWithResources(t, filepath.Join(dir, "events.sqlite"), []ResourceDescriptor{{
		Ref:      "cf:tool-loop-pause",
		MimeType: "text/plain",
		Text:     "RESOURCE LOOP VALUE",
	}})
	k.provider = provider

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:      "tool-loop-pause",
		IdempotencyKey: "pause-on-budget",
		InputItems:     []InputItem{{Type: "text", Text: "keep reading until budget"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v, want paused response", err)
	}
	if resp.Final.Text != "" {
		t.Fatalf("final = %+v, want paused turn without final answer", resp.Final)
	}

	projection, err := k.Session("tool-loop-pause")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Turns) != 1 || projection.Turns[0].Phase != RuntimePhaseWaiting || projection.Turns[0].WaitReason != WaitReasonBudgetPause {
		t.Fatalf("turns = %+v, want paused turn", projection.Turns)
	}
	eventTypes := sessionEventTypes(projection.Events)
	if containsString(eventTypes, "turn.failed") {
		t.Fatalf("event types = %v, want pause not failure", eventTypes)
	}
	if !containsString(eventTypes, "turn.paused") {
		t.Fatalf("event types = %v, want turn.paused", eventTypes)
	}
	if got := countSessionEventType(projection.Events, "tool.result"); got != defaultModelToolRoundBudget {
		t.Fatalf("tool.result count = %d, want %d committed rounds before pause", got, defaultModelToolRoundBudget)
	}
	if got := countSessionEventType(projection.Events, "tool.call"); got != defaultModelToolRoundBudget {
		t.Fatalf("tool.call count = %d, want no admitted over-budget tool call", got)
	}
	if got := provider.CallCount(); got != defaultModelToolRoundBudget+1 {
		t.Fatalf("provider calls = %d, want budget plus over-budget detection step", got)
	}

	replayed, ok, err := k.turnByIdempotencyKey("tool-loop-pause", "pause-on-budget")
	if err != nil || !ok {
		t.Fatalf("turnByIdempotencyKey returned ok=%v err=%v, want paused replay", ok, err)
	}
	if replayed.TurnID != resp.TurnID || countEventType(replayed.Events, "turn.paused") != 1 {
		t.Fatalf("replayed response = %+v, want original paused turn evidence", replayed)
	}
}

func TestSubmitTurnUsesConfiguredBudgetLeaseForToolRounds(t *testing.T) {
	dir := testTempDir(t)
	steps := make([]scriptedToolStep, 0, 5)
	for i := 0; i < 5; i++ {
		steps = append(steps, scriptedToolStep{
			name: "resource_read",
			args: mustMarshalToolArgs(t, map[string]interface{}{
				"resource_ref": "cf:tool-loop-lease",
				"limit_bytes":  64,
			}),
		})
	}
	provider := &scriptedToolProvider{
		steps: steps,
		final: "finished after configured lease",
	}
	k := newTestKernelWithBudgetAndResources(t, filepath.Join(dir, "events.sqlite"), BudgetPolicy{
		ModelToolRoundBudget: 6,
	}, []ResourceDescriptor{{
		Ref:      "cf:tool-loop-lease",
		MimeType: "text/plain",
		Text:     "LEASED RESOURCE VALUE",
	}})
	k.provider = provider

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:      "tool-loop-lease",
		IdempotencyKey: "configured-budget",
		InputItems:     []InputItem{{Type: "text", Text: "read five times then answer"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "finished after configured lease" || resp.Pause != nil {
		t.Fatalf("response = %+v, want final answer without pause", resp)
	}
	projection, err := k.Session("tool-loop-lease")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if got := countSessionEventType(projection.Events, "tool.result"); got != 5 {
		t.Fatalf("tool.result count = %d, want 5 committed rounds", got)
	}
	if containsString(sessionEventTypes(projection.Events), "turn.paused") {
		t.Fatalf("events = %+v, want no turn.paused before configured lease is exhausted", projection.Events)
	}
}

func TestSubmitTurnNormalizesZeroBudgetLeaseToDefault(t *testing.T) {
	dir := testTempDir(t)
	provider := &repeatingToolProvider{
		toolName: "resource_read",
		args: func(round int) json.RawMessage {
			return mustMarshalToolArgs(t, map[string]interface{}{
				"resource_ref": "cf:tool-loop-default-lease",
				"limit_bytes":  64,
			})
		},
	}
	k := newTestKernelWithBudgetAndResources(t, filepath.Join(dir, "events.sqlite"), BudgetPolicy{
		ModelToolRoundBudget: 0,
	}, []ResourceDescriptor{{
		Ref:      "cf:tool-loop-default-lease",
		MimeType: "text/plain",
		Text:     "DEFAULT LEASE RESOURCE",
	}})
	k.provider = provider

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "tool-loop-default-lease",
		InputItems: []InputItem{{Type: "text", Text: "keep reading until default pause"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v, want paused response", err)
	}
	if resp.Pause == nil {
		t.Fatalf("response = %+v, want pause", resp)
	}
	if resp.Pause.RoundBudget != defaultModelToolRoundBudget {
		t.Fatalf("pause round budget = %d, want default %d", resp.Pause.RoundBudget, defaultModelToolRoundBudget)
	}
	if got := provider.CallCount(); got != defaultModelToolRoundBudget+1 {
		t.Fatalf("provider calls = %d, want default budget plus over-budget detection step", got)
	}
}

func TestBudgetLeaseIsInspectableButNotModelVisible(t *testing.T) {
	dir := testTempDir(t)
	k := newTestKernelWithBudgetAndResources(t, filepath.Join(dir, "events.sqlite"), BudgetPolicy{
		ModelToolRoundBudget:  7,
		ModelToolRoundCeiling: 9,
	}, nil)

	capabilities := k.Capabilities()
	if capabilities.BudgetLease.ModelToolRoundBudget != 7 || capabilities.BudgetLease.ModelToolRoundCeiling != 9 {
		t.Fatalf("capabilities budget lease = %+v, want configured effective lease", capabilities.BudgetLease)
	}

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "budget-lease-inspection",
		InputItems: []InputItem{{Type: "text", Text: "inspect lease"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	inspection, err := k.ContextInspection(resp.TurnID)
	if err != nil {
		t.Fatalf("ContextInspection returned error: %v", err)
	}
	if inspection.Runtime == nil || inspection.Runtime.BudgetLease.ModelToolRoundBudget != 7 {
		t.Fatalf("context runtime = %+v, want budget lease projection", inspection.Runtime)
	}

	payload, err := json.Marshal(k.toolGateway().ToolManifest())
	if err != nil {
		t.Fatalf("marshal tool manifest: %v", err)
	}
	manifest := strings.ToLower(string(payload))
	for _, forbidden := range []string{"budget", "lease", "round_budget", "model_tool_round"} {
		if strings.Contains(manifest, forbidden) {
			t.Fatalf("model-visible tool manifest exposes budget control %q: %s", forbidden, manifest)
		}
	}
}

func TestBudgetLeaseClampsConfiguredBudgetToCeiling(t *testing.T) {
	k := newTestKernelWithBudgetAndResources(t, filepath.Join(testTempDir(t), "events.sqlite"), BudgetPolicy{
		ModelToolRoundBudget:  50,
		ModelToolRoundCeiling: 6,
	}, nil)

	capabilities := k.Capabilities()
	if capabilities.BudgetLease.ModelToolRoundBudget != 6 || capabilities.BudgetLease.ModelToolRoundCeiling != 6 {
		t.Fatalf("capabilities budget lease = %+v, want configured budget clamped to ceiling", capabilities.BudgetLease)
	}
}

func TestBudgetDocsSeparateExecutionLeasesFromHardSafetyCaps(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "..", "docs", "requirements", "kernel-foundation-capabilities.md"),
		filepath.Join("..", "..", "docs", "design", "kernel-foundation-capabilities.md"),
	} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := strings.ToLower(strings.Join(strings.Fields(string(body)), " "))
		for _, want := range []string{"budgetlease", "execution budget", "hard safety"} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing budget classification phrase %q", path, want)
			}
		}
	}
}

func TestPausedTurnContinuationIncludesCommittedToolRoundsInProviderContext(t *testing.T) {
	dir := testTempDir(t)
	provider := &repeatingToolProvider{
		toolName: "resource_read",
		args: func(round int) json.RawMessage {
			return mustMarshalToolArgs(t, map[string]string{
				"resource_ref": "cf:tool-loop-resume",
			})
		},
	}
	k := newTestKernelWithResources(t, filepath.Join(dir, "events.sqlite"), []ResourceDescriptor{{
		Ref:      "cf:tool-loop-resume",
		MimeType: "text/plain",
		Text:     "PAUSED RESOURCE CONTEXT",
	}})
	k.provider = provider

	_, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "tool-loop-resume",
		InputItems: []InputItem{{Type: "text", Text: "read until pause"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn pause returned error: %v", err)
	}

	recorder := &recordingTextProvider{text: "continued"}
	k.provider = recorder
	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "tool-loop-resume",
		InputItems: []InputItem{{Type: "text", Text: "continue"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn continuation returned error: %v", err)
	}
	if resp.Final.Text != "continued" {
		t.Fatalf("final text = %q, want continuation final", resp.Final.Text)
	}
	requests := recorder.Requests()
	if len(requests) != 1 {
		t.Fatalf("recorded provider requests = %d, want 1", len(requests))
	}
	history, ok := modelInputTextByKind(requests[0].InputItems, ModelInputKindConversationHistoryContext)
	if !ok {
		t.Fatalf("provider input items = %+v, want conversation history context", requests[0].InputItems)
	}
	for _, want := range []string{"tool loop paused", "resource_read", "PAUSED RESOURCE CONTEXT"} {
		if !strings.Contains(history, want) {
			t.Fatalf("history context = %q, want %q", history, want)
		}
	}
	for _, forbidden := range []string{"evt_", "operation_id", "audit"} {
		if strings.Contains(history, forbidden) {
			t.Fatalf("history context leaked %q: %s", forbidden, history)
		}
	}
}

func TestToolLoopStormGuardAugmentsRepeatedFailureFeedback(t *testing.T) {
	provider := &untilLoopGuardProvider{
		toolName: "email.send",
		args: func(round int) json.RawMessage {
			return mustMarshalToolArgs(t, map[string]interface{}{
				"attempt": round,
				"body":    strings.Repeat("x", round+1),
			})
		},
		final: "changed approach after guard",
	}
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	k.provider = provider

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "tool-loop-storm-failure",
		InputItems: []InputItem{{Type: "text", Text: "try unsupported mail repeatedly"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "changed approach after guard" {
		t.Fatalf("final = %q, want guarded final", resp.Final.Text)
	}

	requests := provider.Requests()
	lastRounds := requests[len(requests)-1].ToolRounds
	lastRound := lastRounds[len(lastRounds)-1]
	guarded := decodeJSONMap(t, lastRound.Results[0].Content)
	if guarded["status"] != "tool_request_invalid" {
		t.Fatalf("guarded payload = %+v, want original invalid status preserved", guarded)
	}
	if _, ok := guarded["loop_guard"].(map[string]interface{}); !ok {
		t.Fatalf("guarded payload = %+v, want loop_guard directive", guarded)
	}
}

func TestToolLoopStormGuardBlocksRepeatedWriteSuccessBeforeEffect(t *testing.T) {
	workspace := testTempDir(t)
	target := "repeat-write.txt"
	command := writeFileCommand(target, "X")
	provider := &untilLoopGuardProvider{
		toolName: "shell_exec",
		args: func(round int) json.RawMessage {
			return mustMarshalToolArgs(t, map[string]interface{}{
				"cwd":     workspace,
				"command": command,
			})
		},
		final: "write loop stopped",
	}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.sqlite"),
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
		SessionID:  "tool-loop-repeat-write",
		InputItems: []InputItem{{Type: "text", Text: "repeat same write"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "write loop stopped" {
		t.Fatalf("final = %q, want guarded final", resp.Final.Text)
	}
	content, err := os.ReadFile(filepath.Join(workspace, target))
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(content) != "X" {
		t.Fatalf("file content = %q, want controlled write value", string(content))
	}
	projection, err := k.Session("tool-loop-repeat-write")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Operations) != 2 {
		t.Fatalf("operations = %+v, want exactly two executed writes before guarded block", projection.Operations)
	}

	requests := provider.Requests()
	lastRounds := requests[len(requests)-1].ToolRounds
	lastRound := lastRounds[len(lastRounds)-1]
	guarded := decodeJSONMap(t, lastRound.Results[0].Content)
	if guarded["status"] != "tool_loop_guarded" || guarded["executed"] != false {
		t.Fatalf("guarded payload = %+v, want non-executed tool_loop_guarded", guarded)
	}
}

func TestToolLoopStormGuardResetsAfterSuccessfulProgress(t *testing.T) {
	dir := testTempDir(t)
	k := newTestKernelWithResources(t, filepath.Join(dir, "events.sqlite"), []ResourceDescriptor{{
		Ref:      "cf:tool-loop-reset",
		MimeType: "text/plain",
		Text:     "RESET PROGRESS",
	}})
	provider := &scriptedToolProvider{
		steps: []scriptedToolStep{
			{name: "email.send", args: mustMarshalToolArgs(t, map[string]interface{}{"attempt": 1})},
			{name: "email.send", args: mustMarshalToolArgs(t, map[string]interface{}{"attempt": 2})},
			{name: "resource_read", args: mustMarshalToolArgs(t, map[string]string{"resource_ref": "cf:tool-loop-reset"})},
			{name: "email.send", args: mustMarshalToolArgs(t, map[string]interface{}{"attempt": 3})},
		},
		final: "progress reset guard",
	}
	k.provider = provider

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "tool-loop-failure-reset",
		InputItems: []InputItem{{Type: "text", Text: "make progress between failures"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "progress reset guard" {
		t.Fatalf("final = %q, want reset final", resp.Final.Text)
	}
	if requestContainsLoopGuard(provider.Requests()[len(provider.Requests())-1]) {
		t.Fatalf("provider requests = %+v, want no loop guard after successful progress reset", provider.Requests())
	}
}

func TestToolLoopRepeatSuccessGuardResetsAfterReadProgress(t *testing.T) {
	dir := testTempDir(t)
	workspace := filepath.Join(dir, "workspace")
	command := writeFileCommand("repeat-after-read.txt", "X")
	provider := &scriptedToolProvider{
		steps: []scriptedToolStep{
			{name: "shell_exec", args: mustMarshalToolArgs(t, map[string]interface{}{"cwd": workspace, "command": command})},
			{name: "shell_exec", args: mustMarshalToolArgs(t, map[string]interface{}{"cwd": workspace, "command": command})},
			{name: "resource_read", args: mustMarshalToolArgs(t, map[string]string{"resource_ref": "cf:tool-loop-write-reset"})},
			{name: "shell_exec", args: mustMarshalToolArgs(t, map[string]interface{}{"cwd": workspace, "command": command})},
		},
		final: "read reset write guard",
	}
	k, err := New(Config{
		LedgerPath:   filepath.Join(dir, "events.sqlite"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeDefault,
			WorkspaceRoot:  workspace,
		},
		Resources: []ResourceDescriptor{{
			Ref:      "cf:tool-loop-write-reset",
			MimeType: "text/plain",
			Text:     "WRITE RESET PROGRESS",
		}},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "tool-loop-write-reset",
		InputItems: []InputItem{{Type: "text", Text: "verify between repeated writes"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "read reset write guard" {
		t.Fatalf("final = %q, want read reset final", resp.Final.Text)
	}
	projection, err := k.Session("tool-loop-write-reset")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Operations) != 3 {
		t.Fatalf("operations = %+v, want third write allowed after read progress", projection.Operations)
	}
	if requestContainsLoopGuard(provider.Requests()[len(provider.Requests())-1]) {
		t.Fatalf("provider requests = %+v, want no loop guard after read progress reset", provider.Requests())
	}
}

func TestToolLoopRepeatSuccessGuardResetsAfterDifferentWriteProgress(t *testing.T) {
	dir := testTempDir(t)
	workspace := filepath.Join(dir, "workspace")
	commandA := writeFileCommand("repeat-after-different-write-a.txt", "A")
	commandB := writeFileCommand("repeat-after-different-write-b.txt", "B")
	provider := &scriptedToolProvider{
		steps: []scriptedToolStep{
			{name: "shell_exec", args: mustMarshalToolArgs(t, map[string]interface{}{"cwd": workspace, "command": commandA})},
			{name: "shell_exec", args: mustMarshalToolArgs(t, map[string]interface{}{"cwd": workspace, "command": commandA})},
			{name: "shell_exec", args: mustMarshalToolArgs(t, map[string]interface{}{"cwd": workspace, "command": commandB})},
			{name: "shell_exec", args: mustMarshalToolArgs(t, map[string]interface{}{"cwd": workspace, "command": commandA})},
		},
		final: "different write reset guard",
	}
	k, err := New(Config{
		LedgerPath:   filepath.Join(dir, "events.sqlite"),
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
		SessionID:  "tool-loop-write-different-reset",
		InputItems: []InputItem{{Type: "text", Text: "make different write progress between repeated writes"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "different write reset guard" {
		t.Fatalf("final = %q, want different write reset final", resp.Final.Text)
	}
	projection, err := k.Session("tool-loop-write-different-reset")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Operations) != 4 {
		t.Fatalf("operations = %+v, want fourth write allowed after different write progress", projection.Operations)
	}
	if requestContainsLoopGuard(provider.Requests()[len(provider.Requests())-1]) {
		t.Fatalf("provider requests = %+v, want no loop guard after different write progress", provider.Requests())
	}
}

func TestToolLoopGuardStateResetsRepeatedWriteAfterDifferentWriteProgress(t *testing.T) {
	guard := newToolLoopGuard()
	callA := preparedModelToolCall{
		eventID:                "evt_a",
		providerCallID:         "call_a",
		name:                   "shell_exec",
		repeatSuccessSignature: "shell_exec\x00workspace\x00write a",
	}
	callB := preparedModelToolCall{
		eventID:                "evt_b",
		providerCallID:         "call_b",
		name:                   "shell_exec",
		repeatSuccessSignature: "shell_exec\x00workspace\x00write b",
	}
	success := ModelToolResult{Name: "shell_exec", Content: `{"status":"completed","executed":true}`}

	for i := 0; i < toolLoopRepeatedSuccessThreshold; i++ {
		if _, blocked, err := guard.beforeExecute(callA); err != nil || blocked {
			t.Fatalf("call A before progress blocked on iteration %d: blocked=%v err=%v", i, blocked, err)
		}
		if _, err := guard.afterExecute(callA, success); err != nil {
			t.Fatalf("record call A success %d: %v", i, err)
		}
	}
	if _, blocked, err := guard.beforeExecute(callB); err != nil || blocked {
		t.Fatalf("different write progress blocked: blocked=%v err=%v", blocked, err)
	}
	if _, err := guard.afterExecute(callB, success); err != nil {
		t.Fatalf("record call B success: %v", err)
	}
	if result, blocked, err := guard.beforeExecute(callA); err != nil || blocked {
		t.Fatalf("call A after different write progress = blocked %v result %+v err %v, want allowed", blocked, result, err)
	}
}

type repeatingToolProvider struct {
	mu       sync.Mutex
	toolName string
	args     func(round int) json.RawMessage
	requests []ModelRequest
}

func (p *repeatingToolProvider) Name() string { return "repeating-tool" }

func (p *repeatingToolProvider) Ready() ProviderStatus {
	return ProviderStatus{Name: p.Name(), Readiness: ReadinessReady}
}

func (p *repeatingToolProvider) Complete(_ context.Context, req ModelRequest) (ModelResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests = append(p.requests, req)
	round := len(p.requests)
	return ModelResponse{
		Model: "repeating-tool-model",
		ToolCalls: []ModelToolCall{{
			ToolCallID: "call_repeat_" + strconv.Itoa(round),
			Name:       p.toolName,
			Arguments:  p.args(round),
		}},
	}, nil
}

func (p *repeatingToolProvider) CallCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.requests)
}

func newTestKernelWithBudgetAndResources(t *testing.T, ledgerPath string, budget BudgetPolicy, resources []ResourceDescriptor) *Kernel {
	t.Helper()
	k, err := New(Config{
		LedgerPath:   ledgerPath,
		Provider:     FakeProvider{},
		RuntimeToken: testRuntimeToken,
		BudgetPolicy: budget,
		ToolPolicy:   ToolPolicy{PermissionMode: PermissionModePlan},
		Resources:    resources,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	return k
}

type scriptedToolStep struct {
	name string
	args json.RawMessage
}

type scriptedToolProvider struct {
	mu       sync.Mutex
	steps    []scriptedToolStep
	final    string
	requests []ModelRequest
}

func (p *scriptedToolProvider) Name() string { return "scripted-tool" }

func (p *scriptedToolProvider) Ready() ProviderStatus {
	return ProviderStatus{Name: p.Name(), Readiness: ReadinessReady}
}

func (p *scriptedToolProvider) Complete(_ context.Context, req ModelRequest) (ModelResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests = append(p.requests, req)
	if len(p.requests) > len(p.steps) {
		return ModelResponse{Text: p.final, Model: "scripted-tool-model"}, nil
	}
	round := len(p.requests)
	step := p.steps[round-1]
	return ModelResponse{
		Model: "scripted-tool-model",
		ToolCalls: []ModelToolCall{{
			ToolCallID: "call_scripted_" + strconv.Itoa(round),
			Name:       step.name,
			Arguments:  step.args,
		}},
	}, nil
}

func (p *scriptedToolProvider) Requests() []ModelRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]ModelRequest(nil), p.requests...)
}

type untilLoopGuardProvider struct {
	mu       sync.Mutex
	toolName string
	args     func(round int) json.RawMessage
	final    string
	requests []ModelRequest
}

func (p *untilLoopGuardProvider) Name() string { return "until-loop-guard" }

func (p *untilLoopGuardProvider) Ready() ProviderStatus {
	return ProviderStatus{Name: p.Name(), Readiness: ReadinessReady}
}

func (p *untilLoopGuardProvider) Complete(_ context.Context, req ModelRequest) (ModelResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests = append(p.requests, req)
	if requestContainsLoopGuard(req) {
		return ModelResponse{Text: p.final, Model: "until-loop-guard-model"}, nil
	}
	round := len(p.requests)
	return ModelResponse{
		Model: "until-loop-guard-model",
		ToolCalls: []ModelToolCall{{
			ToolCallID: "call_guard_" + strconv.Itoa(round),
			Name:       p.toolName,
			Arguments:  p.args(round),
		}},
	}, nil
}

func (p *untilLoopGuardProvider) Requests() []ModelRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]ModelRequest(nil), p.requests...)
}

func requestContainsLoopGuard(req ModelRequest) bool {
	for _, round := range req.ToolRounds {
		for _, result := range round.Results {
			if strings.Contains(result.Content, "loop_guard") {
				return true
			}
		}
	}
	return false
}

func sessionEventTypes(events []EventProjection) []string {
	types := make([]string, 0, len(events))
	for _, event := range events {
		types = append(types, event.Type)
	}
	return types
}

func countEventType(events []Event, eventType string) int {
	count := 0
	for _, event := range events {
		if event.Type == eventType {
			count++
		}
	}
	return count
}
