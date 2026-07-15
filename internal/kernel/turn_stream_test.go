package kernel

import (
	"context"
	"path/filepath"
	"testing"
)

func TestSubmitTurnStreamEmitsDeltasButPersistsOnlyFinalMessage(t *testing.T) {
	provider := &streamingTextProvider{chunks: []string{"你", "好"}}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.sqlite"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer k.Close()

	var deltas []string
	resp, err := k.SubmitTurnStream(context.Background(), TurnRequest{
		SessionID:  "stream-session",
		InputItems: []InputItem{{Type: "text", Text: "hello"}},
	}, func(event TurnStreamEvent) error {
		if event.Type == "assistant_delta" {
			deltas = append(deltas, event.Delta)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("SubmitTurnStream returned error: %v", err)
	}
	if got := joinStrings(deltas); got != "你好" {
		t.Fatalf("stream deltas = %q, want 你好", got)
	}
	if resp.Final.Text != "你好" {
		t.Fatalf("final text = %q, want 你好", resp.Final.Text)
	}

	events, err := k.TurnEvents(resp.TurnID)
	if err != nil {
		t.Fatalf("TurnEvents returned error: %v", err)
	}
	finalCount := 0
	for _, event := range events {
		if event.Type == "assistant_delta" {
			t.Fatalf("stream delta persisted as ledger event: %+v", event)
		}
		if event.Type == "model.final" {
			finalCount++
			data, ok := event.Data.(EventData)
			if !ok || data.Final == nil || data.Final.Text != "你好" {
				t.Fatalf("final event data = %+v, want complete text", event.Data)
			}
		}
	}
	if finalCount != 1 {
		t.Fatalf("model.final count = %d, want 1", finalCount)
	}
}

func TestSubmitTurnStreamDoesNotRetryAfterVisibleDelta(t *testing.T) {
	provider := &streamingTextProvider{
		chunks: []string{"partial"},
		err:    newProviderStatusError(500, "stream failed after visible delta", 0),
	}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.sqlite"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer k.Close()

	_, err = k.SubmitTurnStream(context.Background(), TurnRequest{
		SessionID:  "stream-failure-session",
		InputItems: []InputItem{{Type: "text", Text: "hello"}},
	}, func(TurnStreamEvent) error { return nil })
	if err == nil {
		t.Fatal("SubmitTurnStream returned nil error, want stream failure")
	}
	if provider.calls != 1 {
		t.Fatalf("stream provider calls = %d, want no retry after visible delta", provider.calls)
	}
	projection, err := k.Session("stream-failure-session")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Turns) != 1 {
		t.Fatalf("turns = %+v, want one durable failed turn", projection.Turns)
	}
	timeline, err := k.UITimeline("stream-failure-session")
	if err != nil {
		t.Fatalf("UITimeline returned error: %v", err)
	}
	turn := requireSingleTimelineTurn(t, timeline, projection.Turns[0].TurnID)
	if user := requireTimelineChild(t, turn, "user_message"); user.Text != "hello" {
		t.Fatalf("user message = %+v, want original retryable input", user)
	}
	if processing := requireTimelineChild(t, turn, "processing_group"); processing.TerminalOutcome != TerminalOutcomeFailed {
		t.Fatalf("processing group = %+v, want durable failed evidence", processing)
	}
}

type streamingTextProvider struct {
	chunks []string
	err    error
	calls  int
}

func (p *streamingTextProvider) Name() string {
	return "streaming-text"
}

func (p *streamingTextProvider) Ready() ProviderStatus {
	return ProviderStatus{Name: p.Name(), Readiness: ReadinessReady}
}

func (p *streamingTextProvider) Complete(context.Context, ModelRequest) (ModelResponse, error) {
	p.calls++
	text := joinStrings(p.chunks)
	if text == "" {
		text = "complete"
	}
	return ModelResponse{Text: text, Model: "streaming-text-model"}, p.err
}

func (p *streamingTextProvider) StreamComplete(_ context.Context, _ ModelRequest, emit func(ModelStreamDelta) error) (ModelResponse, error) {
	p.calls++
	for _, chunk := range p.chunks {
		if err := emit(ModelStreamDelta{Text: chunk}); err != nil {
			return ModelResponse{}, err
		}
	}
	if p.err != nil {
		return ModelResponse{}, p.err
	}
	return ModelResponse{Text: joinStrings(p.chunks), Model: "streaming-text-model"}, nil
}

func joinStrings(items []string) string {
	out := ""
	for _, item := range items {
		out += item
	}
	return out
}
