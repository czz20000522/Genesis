package kernel

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenAICompatibleReasoningContentIsResponseOnly(t *testing.T) {
	const (
		hiddenReasoning = "SECRET-CHAIN-OF-THOUGHT"
		firstAnswer     = "visible final answer"
		secondAnswer    = "second visible answer"
	)
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read provider request: %v", err)
		}
		requests = append(requests, string(body))
		if strings.Contains(string(body), hiddenReasoning) || strings.Contains(string(body), "reasoning_content") {
			t.Fatalf("provider request leaked hidden reasoning: %s", body)
		}

		w.Header().Set("Content-Type", "application/json")
		switch len(requests) {
		case 1:
			_, _ = w.Write([]byte(`{"model":"served-model","choices":[{"message":{"role":"assistant","content":"` + firstAnswer + `","reasoning_content":"` + hiddenReasoning + `"}}],"usage":{"prompt_tokens":7,"completion_tokens":5,"total_tokens":12}}`))
		case 2:
			if !strings.Contains(string(body), firstAnswer) {
				t.Fatalf("second provider request = %s, want visible first answer in history", body)
			}
			_, _ = w.Write([]byte(`{"model":"served-model","choices":[{"message":{"role":"assistant","content":"` + secondAnswer + `"}}]}`))
		default:
			t.Fatalf("unexpected provider request %d: %s", len(requests), body)
		}
	}))
	defer server.Close()

	k, err := New(Config{
		LedgerPath: filepath.Join(testTempDir(t), "events.jsonl"),
		Provider: NewOpenAICompatibleProvider(OpenAICompatibleConfig{
			BaseURL: server.URL,
			APIKey:  "test-key",
			Model:   "test-model",
			Adapter: ProviderAdapterBinding{
				AdapterID:             "deepseek",
				ProfileID:             "deepseek-v4-flash",
				TransportProtocol:     "openai-chat-completions",
				HiddenReasoningPolicy: "discard",
			},
		}),
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	first, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "reasoning-response-only",
		InputItems: []InputItem{{Type: "text", Text: "answer visibly"}},
	})
	if err != nil {
		t.Fatalf("first SubmitTurn returned error: %v", err)
	}
	if first.Final.Text != firstAnswer {
		t.Fatalf("first final = %q, want visible provider answer", first.Final.Text)
	}
	if first.Final.Usage == nil || first.Final.Usage.InputTokens != 7 || first.Final.Usage.OutputTokens != 5 || first.Final.Usage.TotalTokens != 12 {
		t.Fatalf("first usage = %+v, want normalized usage without reasoning replay", first.Final.Usage)
	}

	second, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "reasoning-response-only",
		InputItems: []InputItem{{Type: "text", Text: "continue"}},
	})
	if err != nil {
		t.Fatalf("second SubmitTurn returned error: %v", err)
	}
	if second.Final.Text != secondAnswer {
		t.Fatalf("second final = %q, want visible provider answer", second.Final.Text)
	}
	if len(requests) != 2 {
		t.Fatalf("provider request count = %d, want 2", len(requests))
	}

	assertProjectionOmitsHiddenReasoning(t, "openai replay request", requests[1], hiddenReasoning)

	storedEvents, err := k.loadEvents()
	if err != nil {
		t.Fatalf("load events: %v", err)
	}
	assertJSONProjectionOmitsHiddenReasoning(t, "stored events", storedEvents, hiddenReasoning)

	turnEvents, err := k.TurnEvents(first.TurnID)
	if err != nil {
		t.Fatalf("TurnEvents returned error: %v", err)
	}
	assertJSONProjectionOmitsHiddenReasoning(t, "turn events", turnEvents, hiddenReasoning)

	session, err := k.Session("reasoning-response-only")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	assertJSONProjectionOmitsHiddenReasoning(t, "session projection", session, hiddenReasoning)

	timeline, err := k.UITimeline("reasoning-response-only")
	if err != nil {
		t.Fatalf("UITimeline returned error: %v", err)
	}
	assertJSONProjectionOmitsHiddenReasoning(t, "UI timeline", timeline, hiddenReasoning)

	audit, err := k.AuditReplay(first.TurnID)
	if err != nil {
		t.Fatalf("AuditReplay returned error: %v", err)
	}
	assertJSONProjectionOmitsHiddenReasoning(t, "audit replay", audit, hiddenReasoning)

	contextInspection, err := k.ContextInspection(first.TurnID)
	if err != nil {
		t.Fatalf("ContextInspection returned error: %v", err)
	}
	assertJSONProjectionOmitsHiddenReasoning(t, "context inspection", contextInspection, hiddenReasoning)

	providerContext, err := k.ProviderContextProjection(second.TurnID)
	if err != nil {
		t.Fatalf("ProviderContextProjection returned error: %v", err)
	}
	contextJSON, err := json.Marshal(providerContext.ModelRequest())
	if err != nil {
		t.Fatalf("marshal provider context: %v", err)
	}
	if !strings.Contains(string(contextJSON), firstAnswer) {
		t.Fatalf("provider context = %s, want visible first answer", contextJSON)
	}
	assertProjectionOmitsHiddenReasoning(t, "provider context", string(contextJSON), hiddenReasoning)

	commandPayload := providerCommandRequest{
		Protocol:     providerCommandProtocol,
		SessionID:    providerContext.SessionID,
		TurnID:       providerContext.TurnID,
		Model:        "test-model",
		InputItems:   providerContext.ModelRequest().InputItems,
		ToolManifest: providerContext.ModelRequest().ToolManifest,
		ToolRounds:   providerCommandModelToolRounds(providerContext.ModelRequest().ToolRounds),
	}
	assertJSONProjectionOmitsHiddenReasoning(t, "provider command request", commandPayload, hiddenReasoning)
}

func TestOpenAICompatibleProviderRejectsVendorHiddenReasoningWithoutAdapterPolicy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"served-model","choices":[{"message":{"role":"assistant","content":"visible answer","reasoning_content":"vendor hidden reasoning"}}]}`))
	}))
	defer server.Close()

	provider := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	})
	_, err := provider.Complete(context.Background(), ModelRequest{
		InputItems: []ModelInputItem{{Kind: ModelInputKindUserText, Text: "answer visibly"}},
	})
	if err == nil {
		t.Fatal("Complete returned nil error for unsupported vendor hidden reasoning")
	}
	failure := providerFailureFromError(err)
	if failure.ReasonCode != "provider_vendor_field_unsupported" || failure.Retryable {
		t.Fatalf("failure = %+v, want nonretryable provider_vendor_field_unsupported", failure)
	}
}

func assertJSONProjectionOmitsHiddenReasoning(t *testing.T, label string, value interface{}, hiddenReasoning string) {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal %s: %v", label, err)
	}
	assertProjectionOmitsHiddenReasoning(t, label, string(encoded), hiddenReasoning)
}

func assertProjectionOmitsHiddenReasoning(t *testing.T, label string, text string, hiddenReasoning string) {
	t.Helper()
	for _, forbidden := range []string{hiddenReasoning, "reasoning_content", "ReasoningContent"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("%s leaked %q: %s", label, forbidden, text)
		}
	}
}
