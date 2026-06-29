package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAutoCompactionProjectsSummaryPlusRecentTail(t *testing.T) {
	provider := &compactionProvider{}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ContextPolicy: ContextPolicy{
			ContextWindowTokens: 10,
			AutoCompactRatio:    0.5,
			RecentTurnLimit:     1,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	for _, text := range []string{"first fact should be compacted", "second fact should stay recent", "third asks with compacted context"} {
		if _, err := k.SubmitTurn(context.Background(), TurnRequest{
			SessionID:  "compact-session",
			InputItems: []InputItem{{Type: "text", Text: text}},
		}); err != nil {
			t.Fatalf("SubmitTurn(%q) returned error: %v", text, err)
		}
	}

	events, err := k.loadEvents()
	if err != nil {
		t.Fatalf("loadEvents returned error: %v", err)
	}
	var started int
	var compacted []ContextCompactionProjection
	for _, event := range events {
		if event.Type == "context.compaction.started" && event.Data.ContextCompaction != nil {
			started++
		}
		if event.Type == "context.compaction.completed" && event.Data.ContextCompaction != nil {
			compacted = append(compacted, *event.Data.ContextCompaction)
		}
	}
	if started == 0 {
		t.Fatalf("events = %+v, want context.compaction.started event", events)
	}
	if len(compacted) == 0 {
		t.Fatalf("events = %+v, want context.compaction.completed event", events)
	}
	if compacted[0].Summary != "summary of compacted earlier context" || compacted[0].CompactedTurnCount != 1 {
		t.Fatalf("first compaction = %+v", compacted[0])
	}
	if len(provider.compactionRequests) == 0 || !strings.Contains(modelUserText(provider.compactionRequests[0].InputItems), "first fact should be compacted") {
		t.Fatalf("compaction requests = %+v, want first fact in compaction source", provider.compactionRequests)
	}

	if len(provider.normalRequests) < 3 {
		t.Fatalf("normal requests = %d, want at least 3", len(provider.normalRequests))
	}
	thirdContext := modelUserText(provider.normalRequests[2].InputItems)
	if !strings.Contains(thirdContext, "Compacted earlier conversation:") || !strings.Contains(thirdContext, "summary of compacted earlier context") {
		t.Fatalf("third context = %q, want compaction summary", thirdContext)
	}
	if !strings.Contains(thirdContext, "second fact should stay recent") || !strings.Contains(thirdContext, "third asks with compacted context") {
		t.Fatalf("third context = %q, want recent tail and current input", thirdContext)
	}
	if strings.Contains(thirdContext, "first fact should be compacted") {
		t.Fatalf("third context = %q, must not include compacted raw turn", thirdContext)
	}

	timeline, err := k.UITimeline("compact-session")
	if err != nil {
		t.Fatalf("UITimeline returned error: %v", err)
	}
	noticeCount := 0
	if timelineAnyItem(timeline.Items, func(item UITimelineItem) bool {
		if strings.Contains(item.Text, "summary of compacted earlier context") {
			t.Fatalf("timeline item leaked compaction summary: %+v", item)
		}
		if item.Kind == "compaction_notice" && item.Phase == RuntimePhaseEnded && item.TerminalOutcome == TerminalOutcomeSucceeded && item.Text != "" {
			noticeCount++
		}
		return false
	}) {
		t.Fatalf("unexpected timeline match")
	}
	if noticeCount == 0 {
		t.Fatalf("timeline items = %+v, want completed compaction notice", timeline.Items)
	}
}

func TestManualCompactionControlSurfaceRunsSharedRunnerWhenIdle(t *testing.T) {
	provider := &compactionProvider{}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	for _, text := range []string{"first manual compaction fact", "second manual compaction fact", "third manual tail"} {
		if _, err := k.SubmitTurn(context.Background(), TurnRequest{
			SessionID:  "manual-compact-idle",
			InputItems: []InputItem{{Type: "text", Text: text}},
		}); err != nil {
			t.Fatalf("SubmitTurn(%q) returned error: %v", text, err)
		}
	}

	status, body := postSessionContextCompact(t, k, "manual-compact-idle", `{}`)
	if status != http.StatusOK {
		t.Fatalf("manual compact status = %d body=%v, want 200", status, body)
	}
	if body["admission_result"] != "admitted" {
		t.Fatalf("manual compact body = %+v, want admitted", body)
	}

	events, err := k.loadEvents()
	if err != nil {
		t.Fatalf("loadEvents returned error: %v", err)
	}
	var started, completed *ContextCompactionProjection
	for _, event := range events {
		switch event.Type {
		case "context.compaction.started":
			started = event.Data.ContextCompaction
		case "context.compaction.completed":
			completed = event.Data.ContextCompaction
		}
	}
	if started == nil || completed == nil {
		t.Fatalf("events = %+v, want manual compaction started/completed", events)
	}
	if started.Trigger != "manual" || started.Status != contextCompactionStatusRunning {
		t.Fatalf("started compaction = %+v, want manual running event shape", started)
	}
	if completed.Trigger != "manual" || completed.Status != contextCompactionStatusCompleted || completed.Summary != "summary of compacted earlier context" {
		t.Fatalf("completed compaction = %+v, want manual completed summary", completed)
	}
	if len(provider.compactionRequests) != 1 {
		t.Fatalf("compaction requests = %d, want manual request through existing runner", len(provider.compactionRequests))
	}

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "manual-compact-idle",
		InputItems: []InputItem{{Type: "text", Text: "read manual compaction context"}},
	})
	if err != nil {
		t.Fatalf("post-manual SubmitTurn returned error: %v", err)
	}
	contextProjection, err := k.ProviderContextProjection(resp.TurnID)
	if err != nil {
		t.Fatalf("ProviderContextProjection returned error: %v", err)
	}
	contextText := modelUserText(contextProjection.InputItems)
	if !strings.Contains(contextText, "summary of compacted earlier context") || !strings.Contains(contextText, "third manual tail") {
		t.Fatalf("provider context = %q, want manual summary plus recent tail", contextText)
	}

	timeline, err := k.UITimeline("manual-compact-idle")
	if err != nil {
		t.Fatalf("UITimeline returned error: %v", err)
	}
	if timelineAnyItem(timeline.Items, func(item UITimelineItem) bool {
		if strings.Contains(item.Text, "summary of compacted earlier context") {
			t.Fatalf("timeline leaked compaction summary: %+v", item)
		}
		return false
	}) {
		t.Fatalf("unexpected timeline match")
	}
}

func TestManualCompactionControlSurfaceRefusesRunningSession(t *testing.T) {
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     &compactionProvider{},
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	now := k.clock()
	if err := k.appendEvent(StoredEvent{
		EventID:   newID("evt", now),
		SessionID: "manual-compact-running",
		TurnID:    "turn_manual_compact_running",
		Type:      "turn.submitted",
		CreatedAt: now,
		Data: EventData{
			InputItems:      []InputItem{{Type: "text", Text: "active turn"}},
			ModelInputKinds: []string{ModelInputKindUserText},
		},
	}); err != nil {
		t.Fatalf("append active turn: %v", err)
	}

	status, body := postSessionContextCompact(t, k, "manual-compact-running", `{}`)
	if status != http.StatusConflict {
		t.Fatalf("manual compact status = %d body=%v, want 409", status, body)
	}
	if body["admission_result"] != "refused" || body["reason_class"] != "active_turn_running" {
		t.Fatalf("manual compact body = %+v, want active_turn_running refusal", body)
	}
	events, err := k.loadEvents()
	if err != nil {
		t.Fatalf("loadEvents returned error: %v", err)
	}
	for _, event := range events {
		if strings.HasPrefix(event.Type, "context.compaction.") {
			t.Fatalf("running-session refusal wrote compaction event: %+v", event)
		}
	}
}

func TestManualCompactionAdmissionChecksActiveTurnOwnership(t *testing.T) {
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     &compactionProvider{},
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	_, finishActiveTurn := k.beginActiveTurn(context.Background(), "manual-compact-active-owner", "turn_active_owner")
	defer finishActiveTurn()

	resp, err := k.CompactSessionContext(context.Background(), "manual-compact-active-owner")
	if err != nil {
		t.Fatalf("CompactSessionContext returned error: %v", err)
	}
	if resp.AdmissionResult != contextCompactionAdmissionRefused || resp.ReasonClass != contextCompactionRefusalActiveTurn {
		t.Fatalf("CompactSessionContext response = %+v, want active-turn refusal", resp)
	}
	events, err := k.loadEvents()
	if err != nil {
		t.Fatalf("loadEvents returned error: %v", err)
	}
	for _, event := range events {
		if strings.HasPrefix(event.Type, "context.compaction.") {
			t.Fatalf("active-turn refusal wrote compaction event: %+v", event)
		}
	}
}

func TestSubmitTurnRefusesWhileManualCompactionOwnsSession(t *testing.T) {
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     &compactionProvider{},
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	finishControl, admitted := k.reserveActiveSessionControl("manual-compact-owns-session", activeSessionKindContextCompaction)
	if !admitted {
		t.Fatal("reserveActiveSessionControl did not admit test compaction control")
	}
	defer finishControl()

	_, err = k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "manual-compact-owns-session",
		InputItems: []InputItem{{Type: "text", Text: "new turn must not start during manual compaction"}},
	})
	if !errors.Is(err, ErrSessionActive) {
		t.Fatalf("SubmitTurn error = %v, want ErrSessionActive", err)
	}
	events, err := k.loadEvents()
	if err != nil {
		t.Fatalf("loadEvents returned error: %v", err)
	}
	for _, event := range events {
		if event.Type == "turn.submitted" {
			t.Fatalf("SubmitTurn wrote turn.submitted while manual compaction owned session: %+v", event)
		}
	}
}

func TestManualCompactionControlSurfaceRejectsCallerControlFields(t *testing.T) {
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     &compactionProvider{},
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	status, body := postSessionContextCompact(t, k, "manual-compact-control-fields", `{"turn_id":"caller-owned","event_id":"caller-owned","compacted_through_turn_id":"caller-owned"}`)
	if status != http.StatusBadRequest {
		t.Fatalf("manual compact status = %d body=%v, want 400 for caller control fields", status, body)
	}
}

func TestCompactionFailureDoesNotFailUserTurn(t *testing.T) {
	provider := &compactionProvider{failCompactionAttempts: 1}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	for _, text := range []string{"first user turn survives", "second user turn survives", "third user turn survives"} {
		if _, err := k.SubmitTurn(context.Background(), TurnRequest{
			SessionID:  "manual-compact-failure",
			InputItems: []InputItem{{Type: "text", Text: text}},
		}); err != nil {
			t.Fatalf("SubmitTurn(%q) returned error: %v", text, err)
		}
	}

	status, body := postSessionContextCompact(t, k, "manual-compact-failure", `{}`)
	if status != http.StatusOK || body["admission_result"] != "admitted" {
		t.Fatalf("manual compact status=%d body=%v, want admitted even when runner records failure", status, body)
	}
	session, err := k.Session("manual-compact-failure")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	for _, turn := range session.Turns {
		if turn.TerminalOutcome != TerminalOutcomeSucceeded {
			t.Fatalf("turn = %+v, compaction failure must not fail completed user turn", turn)
		}
	}
	var failedCompaction bool
	for _, event := range session.Events {
		if event.Type == "turn.failed" {
			t.Fatalf("session events = %+v, compaction failure must not write turn.failed", session.Events)
		}
		if event.Type == "context.compaction.failed" {
			failedCompaction = true
		}
	}
	if !failedCompaction {
		t.Fatalf("session events = %+v, want context.compaction.failed evidence", session.Events)
	}
}

func TestAutoCompactionFailureIsRecordedAndRetried(t *testing.T) {
	provider := &compactionProvider{failCompactionAttempts: 1}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ContextPolicy: ContextPolicy{
			ContextWindowTokens: 10,
			AutoCompactRatio:    0.5,
			RecentTurnLimit:     1,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	for _, text := range []string{"first fact", "second fact", "third fact", "fourth fact"} {
		if _, err := k.SubmitTurn(context.Background(), TurnRequest{
			SessionID:  "compact-failure-retry",
			InputItems: []InputItem{{Type: "text", Text: text}},
		}); err != nil {
			t.Fatalf("SubmitTurn(%q) returned error: %v", text, err)
		}
	}

	events, err := k.loadEvents()
	if err != nil {
		t.Fatalf("loadEvents returned error: %v", err)
	}
	var failed int
	var completed int
	for _, event := range events {
		switch event.Type {
		case "context.compaction.failed":
			failed++
			if event.Data.ContextCompaction == nil || event.Data.ContextCompaction.FailureReason == "" {
				t.Fatalf("failed compaction event = %+v, want structured failure reason", event)
			}
		case "context.compaction.completed":
			completed++
		}
	}
	if failed == 0 {
		t.Fatalf("events = %+v, want context.compaction.failed event", events)
	}
	if completed == 0 {
		t.Fatalf("events = %+v, want retry to eventually complete compaction", events)
	}
	if len(provider.compactionRequests) < 2 {
		t.Fatalf("compaction requests = %d, want retry on later turn", len(provider.compactionRequests))
	}
}

func TestAutoCompactionBacksOffAfterSummarizerFailure(t *testing.T) {
	provider := &compactionProvider{failCompactionAttempts: 1}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ContextPolicy: ContextPolicy{
			ContextWindowTokens: 10,
			AutoCompactRatio:    0.5,
			RecentTurnLimit:     1,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	for _, text := range []string{"first fact", "second fact fails compaction", "third fact is backoff", "fourth fact retries"} {
		if _, err := k.SubmitTurn(context.Background(), TurnRequest{
			SessionID:  "compact-backoff",
			InputItems: []InputItem{{Type: "text", Text: text}},
		}); err != nil {
			t.Fatalf("SubmitTurn(%q) returned error: %v", text, err)
		}
	}

	events, err := k.loadEvents()
	if err != nil {
		t.Fatalf("loadEvents returned error: %v", err)
	}
	statuses := []string{}
	var deferred ContextCompactionProjection
	for _, event := range events {
		if event.Data.ContextCompaction == nil {
			continue
		}
		statuses = append(statuses, event.Data.ContextCompaction.Status)
		if event.Data.ContextCompaction.Status == contextCompactionStatusDeferred {
			deferred = *event.Data.ContextCompaction
		}
	}
	if strings.Join(statuses, ",") != "running,failed,deferred,running,completed" {
		t.Fatalf("compaction statuses = %v, want running,failed,deferred,running,completed", statuses)
	}
	if deferred.BackoffRemainingTurns != 1 || deferred.PreviousFailureReason == "" {
		t.Fatalf("deferred compaction = %+v, want backoff evidence with previous failure", deferred)
	}
	if len(provider.compactionRequests) != 2 {
		t.Fatalf("compaction attempts = %d, want failed attempt plus post-backoff retry", len(provider.compactionRequests))
	}
}

func TestModelGatewayRecordsProviderBackedContextAccounting(t *testing.T) {
	provider := &compactionProvider{
		normalUsages: []*TokenUsage{
			{InputTokens: 12, OutputTokens: 2, TotalTokens: 14, CacheHitTokens: 7, CacheMissTokens: 5},
			{InputTokens: 24, OutputTokens: 3, TotalTokens: 27, CacheHitTokens: 18, CacheMissTokens: 6},
		},
	}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	first, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "context-accounting",
		InputItems: []InputItem{{Type: "text", Text: "first accounted turn"}},
	})
	if err != nil {
		t.Fatalf("first SubmitTurn returned error: %v", err)
	}
	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "context-accounting",
		InputItems: []InputItem{{Type: "text", Text: "second accounted turn"}},
	}); err != nil {
		t.Fatalf("second SubmitTurn returned error: %v", err)
	}

	events, err := k.loadEvents()
	if err != nil {
		t.Fatalf("loadEvents returned error: %v", err)
	}
	var accountings []ModelContextAccountingProjection
	for _, event := range events {
		if event.Type == "model.context.accounted" && event.Data.ModelContextAccounting != nil {
			accountings = append(accountings, *event.Data.ModelContextAccounting)
		}
	}
	if len(accountings) != 2 {
		t.Fatalf("accounting events = %+v, want 2", accountings)
	}
	if accountings[0].Usage == nil || accountings[0].Usage.InputTokens != 12 || accountings[0].Usage.CacheHitTokens != 7 || accountings[0].Usage.CacheMissTokens != 5 {
		t.Fatalf("first accounting usage = %+v, want provider usage/cache facts", accountings[0].Usage)
	}
	if accountings[0].ProcessedInputTokens != 5 || accountings[0].ProcessedInputTokenSource != "prompt_cache_miss_tokens" {
		t.Fatalf("first processed accounting = %+v, want provider cache miss source", accountings[0])
	}
	if len(accountings[0].HistoryTurnIDs) != 0 {
		t.Fatalf("first history turn ids = %+v, want none", accountings[0].HistoryTurnIDs)
	}
	if len(accountings[1].HistoryTurnIDs) != 1 || accountings[1].HistoryTurnIDs[0] != first.TurnID {
		t.Fatalf("second history turn ids = %+v, want first turn id %s", accountings[1].HistoryTurnIDs, first.TurnID)
	}
	if !reflect.DeepEqual(accountings[1].ModelInputKinds, []string{ModelInputKindConversationHistoryContext, ModelInputKindUserText}) {
		t.Fatalf("second model input kinds = %+v", accountings[1].ModelInputKinds)
	}
}

func TestModelGatewayAccountsToolRoundBoundaries(t *testing.T) {
	workspace := testTempDir(t)
	toolArgs, err := json.Marshal(map[string]string{
		"cwd":     workspace,
		"command": writeFileCommand("tool-accounting.txt", "tool accounting"),
	})
	if err != nil {
		t.Fatalf("marshal shell args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{
			{ToolCallID: "call_tool_accounting", Name: "shell_exec", Arguments: json.RawMessage(toolArgs)},
		},
		final: "tool accounting final",
		usages: []*TokenUsage{
			{InputTokens: 10, OutputTokens: 1, TotalTokens: 11, CacheMissTokens: 10},
			{InputTokens: 20, OutputTokens: 2, TotalTokens: 22, CacheMissTokens: 20},
		},
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

	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "tool-context-accounting",
		InputItems: []InputItem{{Type: "text", Text: "write and report"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}

	events, err := k.loadEvents()
	if err != nil {
		t.Fatalf("loadEvents returned error: %v", err)
	}
	var finalRound *ModelContextAccountingProjection
	for _, event := range events {
		if event.Type == "model.context.accounted" && event.Data.ModelContextAccounting != nil {
			accounting := *event.Data.ModelContextAccounting
			if accounting.ToolRoundCount > 0 {
				finalRound = &accounting
			}
		}
	}
	if finalRound == nil {
		t.Fatalf("events = %+v, want accounting for provider context with tool round", events)
	}
	if finalRound.ToolRoundCount != 1 || finalRound.ToolCallCount != 1 || finalRound.ToolResultCount != 1 {
		t.Fatalf("final round accounting = %+v, want one complete tool call/result pair", finalRound)
	}
}

func TestAutoCompactionUsesProviderBackedExchangeAccountingForRecentTail(t *testing.T) {
	provider := &compactionProvider{
		normalUsages: []*TokenUsage{
			{InputTokens: 20, OutputTokens: 1, TotalTokens: 21, CacheHitTokens: 0, CacheMissTokens: 8},
			{InputTokens: 30, OutputTokens: 1, TotalTokens: 31, CacheHitTokens: 18, CacheMissTokens: 8},
			{InputTokens: 45, OutputTokens: 1, TotalTokens: 46, CacheHitTokens: 23, CacheMissTokens: 30},
			{InputTokens: 90, OutputTokens: 1, TotalTokens: 91, CacheHitTokens: 80, CacheMissTokens: 8},
			{InputTokens: 35, OutputTokens: 1, TotalTokens: 36, CacheHitTokens: 25, CacheMissTokens: 10},
		},
	}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ContextPolicy: ContextPolicy{
			ContextWindowTokens: 100,
			AutoCompactRatio:    0.5,
			RecentTurnLimit:     1,
			RecentTailTokens:    40,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	for _, text := range []string{
		"first should compact",
		"second should compact",
		"third should stay by provider accounting",
		"fourth triggers compaction",
		"fifth reads compacted context",
	} {
		if _, err := k.SubmitTurn(context.Background(), TurnRequest{
			SessionID:  "provider-accounted-tail",
			InputItems: []InputItem{{Type: "text", Text: text}},
		}); err != nil {
			t.Fatalf("SubmitTurn(%q) returned error: %v", text, err)
		}
	}

	if len(provider.compactionRequests) == 0 {
		t.Fatalf("compaction requests = 0, want compaction")
	}
	source := modelUserText(provider.compactionRequests[0].InputItems)
	if !strings.Contains(source, "first should compact") || !strings.Contains(source, "second should compact") {
		t.Fatalf("compaction source = %q, want first two turns", source)
	}
	if strings.Contains(source, "third should stay by provider accounting") {
		t.Fatalf("compaction source = %q, must not compact provider-budgeted third turn", source)
	}
	fifthContext := modelUserText(provider.normalRequests[4].InputItems)
	if !strings.Contains(fifthContext, "summary of compacted earlier context") ||
		!strings.Contains(fifthContext, "third should stay by provider accounting") ||
		!strings.Contains(fifthContext, "fourth triggers compaction") {
		t.Fatalf("fifth context = %q, want summary plus provider-budgeted tail", fifthContext)
	}
}

func TestAutoCompactionRecordsUsageEconomicsAndCacheStability(t *testing.T) {
	provider := &compactionProvider{
		normalUsages: []*TokenUsage{
			{InputTokens: 20, OutputTokens: 1, TotalTokens: 21, CacheHitTokens: 0, CacheMissTokens: 20},
			{InputTokens: 40, OutputTokens: 1, TotalTokens: 41, CacheHitTokens: 20, CacheMissTokens: 20},
			{InputTokens: 90, OutputTokens: 1, TotalTokens: 91, CacheHitTokens: 60, CacheMissTokens: 30},
		},
	}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ContextPolicy: ContextPolicy{
			ContextWindowTokens: 100,
			AutoCompactRatio:    0.5,
			RecentTurnLimit:     1,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	for _, text := range []string{"first cache cold", "second cache warming", "third triggers economics"} {
		if _, err := k.SubmitTurn(context.Background(), TurnRequest{
			SessionID:  "compact-cache-economics",
			InputItems: []InputItem{{Type: "text", Text: text}},
		}); err != nil {
			t.Fatalf("SubmitTurn(%q) returned error: %v", text, err)
		}
	}

	events, err := k.loadEvents()
	if err != nil {
		t.Fatalf("loadEvents returned error: %v", err)
	}
	var completed *ContextCompactionProjection
	for _, event := range events {
		if event.Type == "context.compaction.completed" && event.Data.ContextCompaction != nil {
			projection := *event.Data.ContextCompaction
			completed = &projection
		}
	}
	if completed == nil {
		t.Fatalf("events = %+v, want completed compaction", events)
	}
	if completed.SourceUsage == nil || completed.SourceUsage.InputTokens != 90 || completed.SourceUsage.CacheHitTokens != 60 || completed.SourceUsage.CacheMissTokens != 30 {
		t.Fatalf("completed source usage = %+v, want triggering provider usage/cache facts", completed.SourceUsage)
	}
	if completed.CacheStability == nil {
		t.Fatalf("completed compaction = %+v, want cache stability metrics", completed)
	}
	if completed.CacheStability.Samples != 2 ||
		completed.CacheStability.CacheHitTokens != 20 ||
		completed.CacheStability.CacheMissTokens != 40 ||
		completed.CacheStability.LatestHitRatePermille != 500 ||
		completed.CacheStability.Trend != "warming" {
		t.Fatalf("cache stability = %+v, want warming cache facts for compacted region", completed.CacheStability)
	}
}

func TestCompactionSourcePreservesCompletedToolCallResultPairs(t *testing.T) {
	provider := &compactionToolPairProvider{}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ContextPolicy: ContextPolicy{
			ContextWindowTokens: 10,
			AutoCompactRatio:    0.5,
			RecentTurnLimit:     1,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	for _, text := range []string{"first turn uses a tool", "second turn triggers compaction"} {
		if _, err := k.SubmitTurn(context.Background(), TurnRequest{
			SessionID:  "compact-tool-pair",
			InputItems: []InputItem{{Type: "text", Text: text}},
		}); err != nil {
			t.Fatalf("SubmitTurn(%q) returned error: %v", text, err)
		}
	}

	if len(provider.compactionRequests) != 1 {
		t.Fatalf("compaction requests = %d, want 1", len(provider.compactionRequests))
	}
	source := modelUserText(provider.compactionRequests[0].InputItems)
	for _, want := range []string{"[tool call]", "shell_exec", "GENESIS_TOOL_PAIR_MARKER", "[tool result]", "permission_denied", "tool pair final"} {
		if !strings.Contains(source, want) {
			t.Fatalf("compaction source = %q, want %q", source, want)
		}
	}
}

func postSessionContextCompact(t *testing.T, k *Kernel, sessionID string, body string) (int, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/sessions/"+sessionID+"/context/compact", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+testRuntimeToken)
	recorder := httptest.NewRecorder()

	Handler(k).ServeHTTP(recorder, req)

	var decoded map[string]interface{}
	if strings.TrimSpace(recorder.Body.String()) != "" {
		if err := json.Unmarshal(recorder.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("decode compact response %q: %v", recorder.Body.String(), err)
		}
	}
	return recorder.Code, decoded
}
