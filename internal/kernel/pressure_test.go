package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestKernelPressureLongRunningClosedLoop(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.jsonl")
	workspace := testTempDir(t)
	outsideWorkspace := testTempDir(t)
	provider := newKernelPressureProvider(workspace, outsideWorkspace)
	k := newKernelPressureKernel(t, ledgerPath, workspace, provider)
	sessionID := "kernel-pressure"

	var completed []TurnResponse
	var failedTurns int
	for i := 0; i < kernelPressureTurnCount; i++ {
		resp, err := k.SubmitTurn(context.Background(), TurnRequest{
			SessionID:      sessionID,
			IdempotencyKey: fmt.Sprintf("pressure-turn-%02d", i),
			InputItems: []InputItem{{
				Type: "text",
				Text: fmt.Sprintf("pressure prompt %02d", i),
			}},
		})
		action := provider.ActionAt(i)
		if action.kind == kernelPressureProviderFailure {
			failedTurns++
			if err == nil || !strings.Contains(err.Error(), "pressure provider failure") {
				t.Fatalf("SubmitTurn(%d) error = %v, want pressure provider failure", i, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("SubmitTurn(%d) returned error: %v", i, err)
		}
		if !strings.Contains(resp.Final.Text, "pressure final") {
			t.Fatalf("SubmitTurn(%d) final = %q, want pressure final", i, resp.Final.Text)
		}
		completed = append(completed, resp)
	}
	if failedTurns != 1 {
		t.Fatalf("failed turns = %d, want 1 provider failure", failedTurns)
	}
	if len(completed) != kernelPressureTurnCount-failedTurns {
		t.Fatalf("completed turns = %d, want %d", len(completed), kernelPressureTurnCount-failedTurns)
	}
	if data, err := os.ReadFile(filepath.Join(workspace, "pressure-write-01.txt")); err != nil || string(data) != "pressure01" {
		t.Fatalf("written file = %q, %v; want pressure01", string(data), err)
	}

	stats := provider.Stats()
	if stats.compactionRequests == 0 {
		t.Fatal("provider compaction requests = 0, want at least one pressure compaction")
	}
	for _, want := range []string{"completed", "failed", "permission_denied", "tool_request_invalid"} {
		if stats.toolResultStatuses[want] == 0 {
			t.Fatalf("tool result statuses = %+v, want %q", stats.toolResultStatuses, want)
		}
	}

	restartedProvider := newKernelPressureProvider(workspace, outsideWorkspace)
	restarted := newKernelPressureKernel(t, ledgerPath, workspace, restartedProvider)
	replayed, err := restarted.SubmitTurn(context.Background(), TurnRequest{
		SessionID:      sessionID,
		IdempotencyKey: "pressure-turn-01",
		InputItems: []InputItem{{
			Type: "text",
			Text: "retry must replay without provider call",
		}},
	})
	if err != nil {
		t.Fatalf("idempotent replay returned error: %v", err)
	}
	if replayed.TurnID != completed[1].TurnID {
		t.Fatalf("replayed turn id = %q, want %q", replayed.TurnID, completed[1].TurnID)
	}
	if replayStats := restartedProvider.Stats(); replayStats.normalRequests != 0 || replayStats.compactionRequests != 0 {
		t.Fatalf("replay provider stats = %+v, want no provider calls for idempotent replay", replayStats)
	}

	session, err := restarted.Session(sessionID)
	if err != nil {
		t.Fatalf("Session after restart returned error: %v", err)
	}
	assertKernelPressureSession(t, session, kernelPressureTurnCount, failedTurns)

	contextProjection, err := restarted.ProviderContextProjection(completed[len(completed)-1].TurnID)
	if err != nil {
		t.Fatalf("ProviderContextProjection after restart returned error: %v", err)
	}
	contextText := modelUserText(contextProjection.InputItems)
	if !strings.Contains(contextText, "pressure compacted summary") {
		t.Fatalf("provider context = %q, want compacted summary after restart", contextText)
	}

	timeline, err := restarted.UITimeline(sessionID)
	if err != nil {
		t.Fatalf("UITimeline after restart returned error: %v", err)
	}
	var compactionNotice bool
	if timelineAnyItem(timeline.Items, func(item UITimelineItem) bool {
		if strings.Contains(item.Text, "pressure compacted summary") {
			t.Fatalf("timeline leaked compaction summary: %+v", item)
		}
		if item.Kind == "compaction_notice" && item.Phase == RuntimePhaseEnded && item.TerminalOutcome == TerminalOutcomeSucceeded && item.Text != "" {
			compactionNotice = true
		}
		return false
	}) {
		t.Fatalf("unexpected timeline match")
	}
	if !compactionNotice {
		t.Fatalf("timeline items = %+v, want completed compaction notice", timeline.Items)
	}
}

func newKernelPressureKernel(t *testing.T, ledgerPath string, workspace string, provider Provider) *Kernel {
	t.Helper()
	k, err := New(Config{
		LedgerPath:   ledgerPath,
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeDefault,
			WorkspaceRoot:  workspace,
		},
		ContextPolicy: ContextPolicy{
			ContextWindowTokens: 20,
			AutoCompactRatio:    0.6,
			RecentTurnLimit:     2,
			RecentTailTokens:    45,
			RetryBackoffTurns:   1,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	return k
}

func assertKernelPressureSession(t *testing.T, session SessionProjection, wantTurns int, wantFailedTurns int) {
	t.Helper()
	if len(session.Turns) != wantTurns {
		t.Fatalf("turns = %d, want %d", len(session.Turns), wantTurns)
	}
	terminalOutcomes := map[string]int{}
	for _, turn := range session.Turns {
		terminalOutcomes[turn.TerminalOutcome]++
	}
	if terminalOutcomes[TerminalOutcomeFailed] != wantFailedTurns || terminalOutcomes[TerminalOutcomeSucceeded] != wantTurns-wantFailedTurns {
		t.Fatalf("turn terminal outcomes = %+v, want succeeded=%d failed=%d", terminalOutcomes, wantTurns-wantFailedTurns, wantFailedTurns)
	}
	operationStatuses := map[string]int{}
	for _, operation := range session.Operations {
		operationStatuses[operation.Status]++
	}
	for _, want := range []string{"completed", "failed", "blocked"} {
		if operationStatuses[want] == 0 {
			t.Fatalf("operation statuses = %+v, want %q", operationStatuses, want)
		}
	}
	eventTypes := map[string]int{}
	for _, event := range session.Events {
		eventTypes[event.Type]++
	}
	for _, want := range []string{"tool.call", "tool.result", "model.context.accounted", "context.compaction.completed", "turn.failed"} {
		if eventTypes[want] == 0 {
			t.Fatalf("event type counts = %+v, want %q", eventTypes, want)
		}
	}
}

const kernelPressureTurnCount = 12

const (
	kernelPressurePlain pressureActionKind = iota
	kernelPressureWrite
	kernelPressureInvalid
	kernelPressureBlocked
	kernelPressureFailedCommand
	kernelPressureUnsupported
	kernelPressureProviderFailure
)

type pressureActionKind int

type kernelPressureAction struct {
	kind pressureActionKind
}

type kernelPressureProvider struct {
	mu                 sync.Mutex
	workspace          string
	outsideWorkspace   string
	actions            []kernelPressureAction
	initialRequests    int
	normalRequests     int
	compactionRequests int
	turnActions        map[string]kernelPressureAction
	toolResultStatuses map[string]int
}

type kernelPressureStats struct {
	normalRequests     int
	compactionRequests int
	toolResultStatuses map[string]int
}

func newKernelPressureProvider(workspace string, outsideWorkspace string) *kernelPressureProvider {
	return &kernelPressureProvider{
		workspace:        workspace,
		outsideWorkspace: outsideWorkspace,
		actions: []kernelPressureAction{
			{kind: kernelPressurePlain},
			{kind: kernelPressureWrite},
			{kind: kernelPressureInvalid},
			{kind: kernelPressureBlocked},
			{kind: kernelPressureFailedCommand},
			{kind: kernelPressurePlain},
			{kind: kernelPressureWrite},
			{kind: kernelPressureUnsupported},
			{kind: kernelPressureProviderFailure},
			{kind: kernelPressurePlain},
			{kind: kernelPressureBlocked},
			{kind: kernelPressureWrite},
		},
		turnActions:        map[string]kernelPressureAction{},
		toolResultStatuses: map[string]int{},
	}
}

func (p *kernelPressureProvider) Name() string {
	return "kernel-pressure-provider"
}

func (p *kernelPressureProvider) Ready() ProviderStatus {
	return ProviderStatus{Name: p.Name(), Readiness: ReadinessReady}
}

func (p *kernelPressureProvider) Complete(_ context.Context, req ModelRequest) (ModelResponse, error) {
	if len(req.InputItems) > 0 && req.InputItems[0].Kind == "context_compaction_source" {
		p.mu.Lock()
		p.compactionRequests++
		p.mu.Unlock()
		return ModelResponse{
			Text:  "pressure compacted summary",
			Model: "pressure-compaction-model",
			Usage: &TokenUsage{InputTokens: 8, OutputTokens: 2, TotalTokens: 10, CacheMissTokens: 8},
		}, nil
	}

	if len(req.ToolRounds) > 0 {
		statuses := toolRoundStatuses(req.ToolRounds)
		p.mu.Lock()
		p.normalRequests++
		for _, status := range statuses {
			p.toolResultStatuses[status]++
		}
		p.mu.Unlock()
		return ModelResponse{
			Text:  "pressure final after tool " + strings.Join(statuses, ","),
			Model: "pressure-model",
			Usage: pressureUsage(),
		}, nil
	}

	p.mu.Lock()
	actionIndex := p.initialRequests
	p.initialRequests++
	p.normalRequests++
	action := p.actions[actionIndex%len(p.actions)]
	p.turnActions[req.TurnID] = action
	p.mu.Unlock()

	switch action.kind {
	case kernelPressureProviderFailure:
		return ModelResponse{}, errors.New("pressure provider failure")
	case kernelPressurePlain:
		return ModelResponse{Text: "pressure final plain", Model: "pressure-model", Usage: pressureUsage()}, nil
	case kernelPressureWrite:
		command := writeFileCommand(fmt.Sprintf("pressure-write-%02d.txt", actionIndex), fmt.Sprintf("pressure%02d", actionIndex))
		return p.toolResponse(req, actionIndex, "shell_exec", p.shellArgs(p.workspace, command)), nil
	case kernelPressureInvalid:
		return p.toolResponse(req, actionIndex, "shell_exec", map[string]string{"cwd": p.workspace}), nil
	case kernelPressureBlocked:
		return p.toolResponse(req, actionIndex, "shell_exec", p.shellArgs(p.outsideWorkspace, echoCommand("blocked"))), nil
	case kernelPressureFailedCommand:
		return p.toolResponse(req, actionIndex, "shell_exec", p.shellArgs(p.workspace, readMissingFileCommand("missing-pressure.txt"))), nil
	case kernelPressureUnsupported:
		return p.toolResponse(req, actionIndex, "missing_external_tool", map[string]string{}), nil
	default:
		return ModelResponse{Text: "pressure final plain", Model: "pressure-model", Usage: pressureUsage()}, nil
	}
}

func (p *kernelPressureProvider) ActionAt(index int) kernelPressureAction {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.actions[index%len(p.actions)]
}

func (p *kernelPressureProvider) Stats() kernelPressureStats {
	p.mu.Lock()
	defer p.mu.Unlock()
	statuses := map[string]int{}
	for status, count := range p.toolResultStatuses {
		statuses[status] = count
	}
	return kernelPressureStats{
		normalRequests:     p.normalRequests,
		compactionRequests: p.compactionRequests,
		toolResultStatuses: statuses,
	}
}

func (p *kernelPressureProvider) shellArgs(cwd string, command string) map[string]string {
	return map[string]string{
		"cwd":     cwd,
		"command": command,
	}
}

func (p *kernelPressureProvider) toolResponse(req ModelRequest, index int, name string, args map[string]string) ModelResponse {
	return ModelResponse{
		Model:     "pressure-model",
		Usage:     pressureUsage(),
		ToolCalls: []ModelToolCall{p.toolCall(req, index, name, args)},
	}
}

func (p *kernelPressureProvider) toolCall(req ModelRequest, index int, name string, args map[string]string) ModelToolCall {
	payload, err := json.Marshal(args)
	if err != nil {
		panic(err)
	}
	return ModelToolCall{
		ToolCallID: fmt.Sprintf("pressure_call_%02d", index),
		Name:       name,
		Arguments:  json.RawMessage(payload),
	}
}

func pressureUsage() *TokenUsage {
	return &TokenUsage{InputTokens: 30, OutputTokens: 3, TotalTokens: 33, CacheHitTokens: 10, CacheMissTokens: 20}
}

func toolRoundStatuses(rounds []ModelToolRound) []string {
	var statuses []string
	for _, round := range rounds {
		for _, result := range round.Results {
			var payload struct {
				Status string `json:"status"`
			}
			if err := json.Unmarshal([]byte(result.Content), &payload); err != nil || strings.TrimSpace(payload.Status) == "" {
				statuses = append(statuses, "unreadable")
				continue
			}
			statuses = append(statuses, payload.Status)
		}
	}
	return statuses
}
