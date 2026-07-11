package kernel

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestReasoningMessagePersistsBeforeFinalAndProjectsAfterRestart(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	clock := func() time.Time { return time.Date(2026, 7, 10, 3, 4, 5, 0, time.UTC) }
	k, err := New(Config{
		LedgerPath:   ledgerPath,
		Provider:     reasoningMessageTestProvider{},
		RuntimeToken: testRuntimeToken,
		Clock:        clock,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	response, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "reasoning-message-session",
		InputItems: []InputItem{{Type: "text", Text: "explain the answer"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	reasoningIndex, finalIndex := -1, -1
	for index, event := range response.Events {
		switch event.Type {
		case "model.reasoning":
			data, ok := event.Data.(EventData)
			if !ok {
				t.Fatalf("reasoning event data = %T, want EventData", event.Data)
			}
			reasoningIndex = index
			if data.Reasoning == nil || data.Reasoning.Text != "inspect the request before answering" {
				t.Fatalf("reasoning event = %#v, want durable reasoning text", data.Reasoning)
			}
			if data.Reasoning.ReasoningID == "" || data.Reasoning.TurnID != response.TurnID || data.Reasoning.CreatedAt.IsZero() {
				t.Fatalf("reasoning event = %#v, want kernel identity and timing", data.Reasoning)
			}
		case "model.final":
			finalIndex = index
		}
	}
	if reasoningIndex < 0 || finalIndex != reasoningIndex+1 {
		t.Fatalf("reasoning/final ordering = %d/%d, want adjacent reasoning before final", reasoningIndex, finalIndex)
	}
	k.Close()

	restarted, err := New(Config{
		LedgerPath:   ledgerPath,
		Provider:     FakeProvider{},
		RuntimeToken: testRuntimeToken,
		Clock:        clock,
	})
	if err != nil {
		t.Fatalf("restart New returned error: %v", err)
	}
	t.Cleanup(restarted.Close)

	session, err := restarted.Session("reasoning-message-session")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(session.Turns) != 1 || len(session.Turns[0].ReasoningMessages) != 1 {
		t.Fatalf("session turns = %#v, want one projected reasoning message", session.Turns)
	}
	if got := session.Turns[0].ReasoningMessages[0]; got.Text != "inspect the request before answering" || got.TurnID != response.TurnID {
		t.Fatalf("reasoning projection = %#v, want persisted semantic message", got)
	}
	timeline, err := restarted.UITimeline("reasoning-message-session")
	if err != nil {
		t.Fatalf("UITimeline returned error: %v", err)
	}
	if !timelineAnyItem(timeline.Items, func(item UITimelineItem) bool {
		return item.Kind == "assistant_reasoning" && item.Text == "inspect the request before answering"
	}) {
		t.Fatalf("timeline = %#v, want assistant reasoning item", timeline.Items)
	}
	timelineReasoning, ok := timelineReasoningItem(timeline.Items)
	if !ok || timelineReasoning.ItemID != session.Turns[0].ReasoningMessages[0].ReasoningID || timelineReasoning.ReasoningID != session.Turns[0].ReasoningMessages[0].ReasoningID {
		t.Fatalf("timeline reasoning = %#v, want stable persisted reasoning identity", timelineReasoning)
	}
}

type reasoningMessageTestProvider struct{}

func (reasoningMessageTestProvider) Name() string { return "reasoning-message-test" }

func (reasoningMessageTestProvider) Ready() ProviderStatus {
	return ProviderStatus{Name: "reasoning-message-test", Readiness: ReadinessReady}
}

func (reasoningMessageTestProvider) Complete(_ context.Context, _ ModelRequest) (ModelResponse, error) {
	return ModelResponse{
		Reasoning: &ReasoningMessage{Text: "inspect the request before answering"},
		Text:      "final answer",
		Model:     "reasoning-message-test-model",
	}, nil
}

func timelineReasoningItem(items []UITimelineItem) (UITimelineItem, bool) {
	for _, item := range items {
		if item.Kind == "assistant_reasoning" {
			return item, true
		}
		if nested, ok := timelineReasoningItem(item.Children); ok {
			return nested, true
		}
	}
	return UITimelineItem{}, false
}
