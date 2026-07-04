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
	"time"
)

func TestModelInputItemsOnlyProjectUserInputWithoutMemoryRecall(t *testing.T) {
	items := modelInputItems([]InputItem{{Type: "text", Text: "你记得我的回答偏好吗？"}})

	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].Kind != ModelInputKindUserText || items[0].Text != "你记得我的回答偏好吗？" {
		t.Fatalf("user item = %+v", items[0])
	}
}

func TestKernelDoesNotAutoInjectApprovedMemoryIntoOpenAICompatibleProvider(t *testing.T) {
	var providerContent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		var req chatCompletionRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if len(req.Messages) != 1 {
			t.Fatalf("messages = %+v, want one user message", req.Messages)
		}
		providerContent = req.Messages[0].Content
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"served-model","choices":[{"message":{"role":"assistant","content":"provider answer"}}]}`))
	}))
	defer server.Close()

	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	k, err := New(Config{
		LedgerPath: ledgerPath,
		Provider: NewOpenAICompatibleProvider(OpenAICompatibleConfig{
			BaseURL: server.URL,
			APIKey:  "test-key",
			Model:   "test-model",
		}),
		RuntimeToken: testRuntimeToken,
		Clock: func() time.Time {
			return time.Date(2026, 6, 22, 1, 2, 3, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	candidate, err := k.CreateMemoryCandidate(MemoryCandidateRequest{
		SessionID: "provider-context-source",
		Text:      "memory-only preference: blue style",
		SourceRef: "turn:provider-context-source",
	})
	if err != nil {
		t.Fatalf("CreateMemoryCandidate returned error: %v", err)
	}
	if _, err := k.ApproveMemoryCandidate(candidate.CandidateID, testApprovalRequest("approval:provider-context-source")); err != nil {
		t.Fatalf("ApproveMemoryCandidate returned error: %v", err)
	}

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "provider-context-consumer",
		InputItems: []InputItem{{Type: "text", Text: "Do you remember my preference?"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "provider answer" {
		t.Fatalf("final text = %q, want provider answer", resp.Final.Text)
	}
	if strings.Contains(providerContent, "Approved memories:") || strings.Contains(providerContent, "memory-only preference: blue style") {
		t.Fatalf("provider content = %q, want approved memory omitted from automatic provider context", providerContent)
	}
	if !strings.Contains(providerContent, "Do you remember my preference?") {
		t.Fatalf("provider content = %q, want user text", providerContent)
	}

	stored, err := k.MemoryCandidate(candidate.CandidateID)
	if err != nil {
		t.Fatalf("MemoryCandidate returned error: %v", err)
	}
	if stored.Status != "approved" || stored.Text != "memory-only preference: blue style" {
		t.Fatalf("stored memory = %+v, want approved owner fact preserved", stored)
	}
}
