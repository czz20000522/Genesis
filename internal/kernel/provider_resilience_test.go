package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestOpenAICompatibleProviderRetriesTransientStatusBeforeTurnFailure(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			http.Error(w, "temporary overload", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"deepseek-test","choices":[{"message":{"role":"assistant","content":"recovered visible answer"}}]}`))
	}))
	defer server.Close()

	k, err := New(Config{
		LedgerPath: filepath.Join(testTempDir(t), "events.jsonl"),
		Provider: NewOpenAICompatibleProvider(OpenAICompatibleConfig{
			BaseURL: server.URL,
			APIKey:  "sk-test",
			Model:   "deepseek-test",
		}),
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "provider-retry-transient",
		InputItems: []InputItem{{Type: "text", Text: "answer after transient"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "recovered visible answer" {
		t.Fatalf("final text = %q, want recovered visible answer", resp.Final.Text)
	}
	if requests != 2 {
		t.Fatalf("provider requests = %d, want retry then success", requests)
	}
	assertSessionHasEventType(t, k, "provider-retry-transient", "model.provider_attempt")
}

func TestOpenAICompatibleProviderFailsFastOnAuthStatus(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Error(w, "bad key", http.StatusUnauthorized)
	}))
	defer server.Close()

	k, err := New(Config{
		LedgerPath: filepath.Join(testTempDir(t), "events.jsonl"),
		Provider: NewOpenAICompatibleProvider(OpenAICompatibleConfig{
			BaseURL: server.URL,
			APIKey:  "sk-test",
			Model:   "deepseek-test",
		}),
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	_, err = k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "provider-auth-fail-fast",
		InputItems: []InputItem{{Type: "text", Text: "auth should fail"}},
	})
	if err == nil {
		t.Fatal("SubmitTurn returned nil error for auth failure")
	}
	if requests != 1 {
		t.Fatalf("provider requests = %d, want fail-fast single attempt", requests)
	}
	projection, err := k.Session("provider-auth-fail-fast")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Turns) != 1 || projection.Turns[0].Error == nil || projection.Turns[0].Error.Code != "provider_auth_failed" {
		t.Fatalf("turn projection = %+v, want provider_auth_failed", projection.Turns)
	}
}

func TestSubmitTurnRepairsEmptyVisibleFinalBeforeCompleting(t *testing.T) {
	provider := &scriptedResilienceProvider{
		responses: []ModelResponse{
			{Model: "empty-final-model"},
			{Model: "empty-final-model", Text: "visible reply"},
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

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "empty-final-repair",
		InputItems: []InputItem{{Type: "text", Text: "answer visibly"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "visible reply" {
		t.Fatalf("final text = %q, want visible reply", resp.Final.Text)
	}
	requests := provider.Requests()
	if len(requests) != 2 {
		t.Fatalf("provider requests = %d, want repair retry", len(requests))
	}
	if !modelRequestContains(requests[1], "visible final answer") {
		t.Fatalf("second request = %+v, want visible-answer repair context", requests[1].InputItems)
	}
	assertSessionHasEventType(t, k, "empty-final-repair", "model.provider_repair")
}

func TestSubmitTurnStopsAfterRepeatedEmptyVisibleFinals(t *testing.T) {
	provider := &scriptedResilienceProvider{
		responses: []ModelResponse{
			{Model: "empty-final-model"},
			{Model: "empty-final-model"},
			{Model: "empty-final-model"},
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

	_, err = k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "empty-final-bounded",
		InputItems: []InputItem{{Type: "text", Text: "answer visibly"}},
	})
	if err == nil {
		t.Fatal("SubmitTurn returned nil error for repeated empty finals")
	}
	if len(provider.Requests()) != 3 {
		t.Fatalf("provider requests = %d, want bounded three attempts", len(provider.Requests()))
	}
	projection, err := k.Session("empty-final-bounded")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Turns) != 1 || projection.Turns[0].Error == nil || projection.Turns[0].Error.Code != "provider_visible_final_required" {
		t.Fatalf("turn projection = %+v, want provider_visible_final_required", projection.Turns)
	}
}

func TestProviderCommandAdapterShapeFailureDoesNotRetry(t *testing.T) {
	provider := NewCommandProvider(ProviderCommandConfig{
		Command:        os.Args[0],
		Args:           []string{"-test.run=TestProviderCommandAdapterHelper", "--", "bad-json"},
		Model:          "command-model",
		RequestTimeout: 5 * time.Second,
		Env:            []string{"GENESIS_PROVIDER_COMMAND_HELPER=1"},
	})
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	_, err = k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "provider-command-no-retry",
		InputItems: []InputItem{{Type: "text", Text: "command provider should fail once"}},
	})
	if err == nil {
		t.Fatal("SubmitTurn returned nil error for provider command shape failure")
	}
	projection, err := k.Session("provider-command-no-retry")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	var attempts []ProviderAttemptProjection
	for _, event := range projection.Events {
		if event.Type == "model.provider_attempt" && event.Data.ProviderAttempt != nil {
			attempts = append(attempts, *event.Data.ProviderAttempt)
		}
	}
	if len(attempts) != 1 {
		t.Fatalf("provider attempt events = %+v, want one non-retry failure", attempts)
	}
	if attempts[0].Status != "failed" || attempts[0].Retryable {
		t.Fatalf("provider command attempt = %+v, want failed non-retryable", attempts[0])
	}
}

type scriptedResilienceProvider struct {
	mu        sync.Mutex
	responses []ModelResponse
	errors    []error
	requests  []ModelRequest
}

func (p *scriptedResilienceProvider) Name() string {
	return "scripted-resilience"
}

func (p *scriptedResilienceProvider) Ready() ProviderStatus {
	return ProviderStatus{Name: p.Name(), Status: "ok"}
}

func (p *scriptedResilienceProvider) Complete(_ context.Context, req ModelRequest) (ModelResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests = append(p.requests, req)
	idx := len(p.requests) - 1
	if idx < len(p.errors) && p.errors[idx] != nil {
		return ModelResponse{}, p.errors[idx]
	}
	if idx < len(p.responses) {
		return p.responses[idx], nil
	}
	return ModelResponse{}, errors.New("scripted provider exhausted")
}

func (p *scriptedResilienceProvider) Requests() []ModelRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]ModelRequest(nil), p.requests...)
}

func modelRequestContains(req ModelRequest, needle string) bool {
	payload, _ := json.Marshal(req)
	return strings.Contains(string(payload), needle)
}

func assertSessionHasEventType(t *testing.T, k *Kernel, sessionID string, eventType string) {
	t.Helper()
	projection, err := k.Session(sessionID)
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	for _, event := range projection.Events {
		if event.Type == eventType {
			return
		}
	}
	t.Fatalf("session %s missing event type %s; events = %+v", sessionID, eventType, projection.Events)
}
