package kernel

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestApprovedMemoryProviderContextUsesExplicitEgressBoundaryWithoutMutatingTruth(t *testing.T) {
	secretMemory := "prefer concise answers; GENESIS_PROVIDER_API_KEY=sk-memory-secret; Authorization: Bearer tokentest123456"
	provider := &recordingTextProvider{text: "memory context observed"}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.jsonl"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	candidate, err := k.CreateMemoryCandidate(MemoryCandidateRequest{
		SessionID: "memory-context-source",
		Text:      secretMemory,
		SourceRef: "turn:memory-context-source",
	})
	if err != nil {
		t.Fatalf("CreateMemoryCandidate returned error: %v", err)
	}
	if _, err := k.ApproveMemoryCandidate(candidate.CandidateID, testApprovalRequest("approval:memory-context-source")); err != nil {
		t.Fatalf("ApproveMemoryCandidate returned error: %v", err)
	}

	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "memory-context-consumer",
		InputItems: []InputItem{{Type: "text", Text: "prefer concise answers"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}

	stored, err := k.MemoryCandidate(candidate.CandidateID)
	if err != nil {
		t.Fatalf("MemoryCandidate returned error: %v", err)
	}
	if stored.Text != secretMemory {
		t.Fatalf("stored memory text = %q, want owner truth preserved", stored.Text)
	}

	requests := provider.Requests()
	if len(requests) != 1 {
		t.Fatalf("provider requests = %d, want one request", len(requests))
	}
	encodedRequest, _ := json.Marshal(requests[0])
	requestText := string(encodedRequest)
	for _, forbidden := range []string{"sk-memory-secret", "tokentest123456"} {
		if strings.Contains(requestText, forbidden) {
			t.Fatalf("provider request leaked %q: %s", forbidden, requestText)
		}
	}
	if strings.Contains(requestText, "Approved memories:") || strings.Contains(requestText, "[REDACTED]") {
		t.Fatalf("provider request = %s, want sensitive memory omitted at egress boundary without lossy replacement", requestText)
	}

	projection, err := k.Session("memory-context-consumer")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Turns) != 1 || len(projection.Turns[0].RecalledMemories) != 1 {
		t.Fatalf("projection turns = %+v, want one recalled memory", projection.Turns)
	}
	projectedRecall := projection.Turns[0].RecalledMemories[0].Text
	if projectedRecall != secretMemory {
		t.Fatalf("session recall projection = %q, want owner truth preserved", projectedRecall)
	}
	sessionJSON, err := json.Marshal(projection)
	if err != nil {
		t.Fatalf("marshal session projection: %v", err)
	}
	if strings.Contains(string(sessionJSON), "[REDACTED]") {
		t.Fatalf("session projection should not use lossy redaction for memory truth: %s", string(sessionJSON))
	}
}
