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

func TestModelInputItemsInjectsApprovedMemoryContextBeforeProvider(t *testing.T) {
	items := modelInputItems(
		[]InputItem{{Type: "text", Text: "你记得我的回答偏好吗？"}},
		[]MemoryRecall{
			{Text: "我偏好中文回答", Source: "turn:memory-source"},
			{Text: "  "},
		},
	)

	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].Kind != ModelInputKindApprovedMemoryContext || items[0].Text != "Approved memories:\n- 我偏好中文回答" {
		t.Fatalf("memory context item = %+v", items[0])
	}
	if items[1].Kind != ModelInputKindUserText || items[1].Text != "你记得我的回答偏好吗？" {
		t.Fatalf("user item = %+v", items[1])
	}
}

func TestKernelBuildsApprovedMemoryContextBeforeOpenAICompatibleProvider(t *testing.T) {
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

	ledgerPath := filepath.Join(testTempDir(t), "events.jsonl")
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
		Text:      "prefer concise answers",
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
		InputItems: []InputItem{{Type: "text", Text: "Do you remember prefer concise answers?"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "provider answer" {
		t.Fatalf("final text = %q, want provider answer", resp.Final.Text)
	}
	if !strings.Contains(providerContent, "Approved memories:\n- prefer concise answers") {
		t.Fatalf("provider content = %q, want approved memory context", providerContent)
	}
	if !strings.Contains(providerContent, "Do you remember prefer concise answers?") {
		t.Fatalf("provider content = %q, want user text", providerContent)
	}

	projection, err := k.Session("provider-context-consumer")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Turns) != 1 || len(projection.Turns[0].RecalledMemories) != 1 {
		t.Fatalf("projection turns = %+v, want recalled memory", projection.Turns)
	}
	if projection.Turns[0].RecalledMemories[0].Source != "turn:provider-context-source" {
		t.Fatalf("recall source = %q, want turn:provider-context-source", projection.Turns[0].RecalledMemories[0].Source)
	}
}
