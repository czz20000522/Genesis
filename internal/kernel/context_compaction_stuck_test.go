package kernel

import (
	"context"
	"path/filepath"
	"testing"
)

func TestAutoCompactionDefersWhenSuccessfulCompactionsStayAboveLimit(t *testing.T) {
	provider := &compactionProvider{
		normalUsages: []*TokenUsage{
			{InputTokens: 90, OutputTokens: 1, TotalTokens: 91, CacheHitTokens: 0, CacheMissTokens: 90},
			{InputTokens: 90, OutputTokens: 1, TotalTokens: 91, CacheHitTokens: 10, CacheMissTokens: 80},
			{InputTokens: 90, OutputTokens: 1, TotalTokens: 91, CacheHitTokens: 5, CacheMissTokens: 85},
			{InputTokens: 90, OutputTokens: 1, TotalTokens: 91, CacheHitTokens: 4, CacheMissTokens: 86},
			{InputTokens: 90, OutputTokens: 1, TotalTokens: 91, CacheHitTokens: 3, CacheMissTokens: 87},
			{InputTokens: 90, OutputTokens: 1, TotalTokens: 91, CacheHitTokens: 2, CacheMissTokens: 88},
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

	for _, text := range []string{
		"stuck compact turn 1",
		"stuck compact turn 2",
		"stuck compact turn 3",
		"stuck compact turn 4",
		"stuck compact turn 5",
		"stuck compact turn 6",
	} {
		if _, err := k.SubmitTurn(context.Background(), TurnRequest{
			SessionID:  "compact-stuck-window",
			InputItems: []InputItem{{Type: "text", Text: text}},
		}); err != nil {
			t.Fatalf("SubmitTurn(%q) returned error: %v", text, err)
		}
	}

	events, err := k.loadEvents()
	if err != nil {
		t.Fatalf("loadEvents returned error: %v", err)
	}
	completed, deferred := contextCompactionStatuses(events, "compact-stuck-window")
	if completed != 2 {
		t.Fatalf("completed compactions = %d, want 2 before stuck deferral; statuses=%v", completed, contextCompactionStatusList(events, "compact-stuck-window"))
	}
	if deferred < 1 {
		t.Fatalf("deferred compactions = %d, want stuck deferral; statuses=%v", deferred, contextCompactionStatusList(events, "compact-stuck-window"))
	}
	if len(provider.compactionRequests) != 2 {
		t.Fatalf("compaction requests = %d, want no summarizer calls after stuck deferral", len(provider.compactionRequests))
	}
	var stuck ContextCompactionProjection
	for _, event := range events {
		if event.SessionID == "compact-stuck-window" && event.Data.ContextCompaction != nil && event.Data.ContextCompaction.Status == contextCompactionStatusDeferred {
			stuck = *event.Data.ContextCompaction
			break
		}
	}
	if stuck.DeferredReason != contextCompactionDeferredReasonStuckWindow {
		t.Fatalf("deferred compaction = %+v, want stuck-window deferred reason", stuck)
	}
	if stuck.ConsecutiveCompletedCompactions < 2 {
		t.Fatalf("deferred compaction = %+v, want consecutive completed count", stuck)
	}
	timeline, err := k.UITimeline("compact-stuck-window")
	if err != nil {
		t.Fatalf("UITimeline returned error: %v", err)
	}
	if !timelineAnyItem(timeline.Items, func(item UITimelineItem) bool {
		return item.Kind == "compaction_notice" && item.Phase == RuntimePhaseWaiting && item.WaitReason == WaitReasonBudgetPause
	}) {
		t.Fatalf("timeline items = %+v, want deferred compaction notice", timeline.Items)
	}
}

func TestAutoCompactionDoesNotPauseHealthySeparatedCompactions(t *testing.T) {
	provider := &compactionProvider{
		normalUsages: []*TokenUsage{
			{InputTokens: 40, OutputTokens: 1, TotalTokens: 41, CacheHitTokens: 0, CacheMissTokens: 40},
			{InputTokens: 90, OutputTokens: 1, TotalTokens: 91, CacheHitTokens: 45, CacheMissTokens: 45},
			{InputTokens: 30, OutputTokens: 1, TotalTokens: 31, CacheHitTokens: 20, CacheMissTokens: 10},
			{InputTokens: 90, OutputTokens: 1, TotalTokens: 91, CacheHitTokens: 60, CacheMissTokens: 30},
			{InputTokens: 30, OutputTokens: 1, TotalTokens: 31, CacheHitTokens: 25, CacheMissTokens: 5},
			{InputTokens: 90, OutputTokens: 1, TotalTokens: 91, CacheHitTokens: 65, CacheMissTokens: 25},
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

	for _, text := range []string{
		"healthy compact turn 1 below limit",
		"healthy compact turn 2 triggers",
		"healthy compact turn 3 recovered",
		"healthy compact turn 4 triggers",
		"healthy compact turn 5 recovered",
		"healthy compact turn 6 triggers",
	} {
		if _, err := k.SubmitTurn(context.Background(), TurnRequest{
			SessionID:  "compact-healthy-window",
			InputItems: []InputItem{{Type: "text", Text: text}},
		}); err != nil {
			t.Fatalf("SubmitTurn(%q) returned error: %v", text, err)
		}
	}

	events, err := k.loadEvents()
	if err != nil {
		t.Fatalf("loadEvents returned error: %v", err)
	}
	completed, deferred := contextCompactionStatuses(events, "compact-healthy-window")
	if deferred != 0 {
		t.Fatalf("deferred compactions = %d, want no stuck pause for separated healthy compactions; statuses=%v", deferred, contextCompactionStatusList(events, "compact-healthy-window"))
	}
	if completed == 0 || len(provider.compactionRequests) == 0 {
		t.Fatalf("completed compactions = %d requests=%d, want healthy compaction still active", completed, len(provider.compactionRequests))
	}
}

func contextCompactionStatuses(events []StoredEvent, sessionID string) (completed int, deferred int) {
	for _, event := range events {
		if event.SessionID != sessionID || event.Data.ContextCompaction == nil {
			continue
		}
		switch event.Data.ContextCompaction.Status {
		case contextCompactionStatusCompleted:
			completed++
		case contextCompactionStatusDeferred:
			deferred++
		}
	}
	return completed, deferred
}

func contextCompactionStatusList(events []StoredEvent, sessionID string) []string {
	statuses := []string{}
	for _, event := range events {
		if event.SessionID == sessionID && event.Data.ContextCompaction != nil {
			statuses = append(statuses, event.Data.ContextCompaction.Status)
		}
	}
	return statuses
}
