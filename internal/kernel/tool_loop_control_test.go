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
	k := newTestKernelWithResources(t, filepath.Join(dir, "events.jsonl"), []ResourceDescriptor{{
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
	if len(projection.Turns) != 1 || projection.Turns[0].Status != "paused" {
		t.Fatalf("turns = %+v, want paused turn", projection.Turns)
	}
	eventTypes := sessionEventTypes(projection.Events)
	if containsString(eventTypes, "turn.failed") {
		t.Fatalf("event types = %v, want pause not failure", eventTypes)
	}
	if !containsString(eventTypes, "turn.paused") {
		t.Fatalf("event types = %v, want turn.paused", eventTypes)
	}
	if got := countSessionEventType(projection.Events, "tool.result"); got != maxModelToolRounds {
		t.Fatalf("tool.result count = %d, want %d committed rounds before pause", got, maxModelToolRounds)
	}
	if got := countSessionEventType(projection.Events, "tool.call"); got != maxModelToolRounds {
		t.Fatalf("tool.call count = %d, want no admitted over-budget tool call", got)
	}
	if got := provider.CallCount(); got != maxModelToolRounds+1 {
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
	k := newTestKernelWithResources(t, filepath.Join(dir, "events.jsonl"), []ResourceDescriptor{{
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
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.jsonl"))
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
	k := newTestKernelWithResources(t, filepath.Join(dir, "events.jsonl"), []ResourceDescriptor{{
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
		LedgerPath:   filepath.Join(dir, "events.jsonl"),
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

type repeatingToolProvider struct {
	mu       sync.Mutex
	toolName string
	args     func(round int) json.RawMessage
	requests []ModelRequest
}

func (p *repeatingToolProvider) Name() string { return "repeating-tool" }

func (p *repeatingToolProvider) Ready() ProviderStatus {
	return ProviderStatus{Name: p.Name(), Status: "ok"}
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
	return ProviderStatus{Name: p.Name(), Status: "ok"}
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
	return ProviderStatus{Name: p.Name(), Status: "ok"}
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
