package kernel

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalConversationPlacesStableSkillPrefixBeforeCurrentUser(t *testing.T) {
	messages := modelConversationMessagesFromStoredEvents(nil, "session-layered", "turn-layered", sameSessionHistoryProjection{}, []ModelInputItem{
		{Kind: ModelInputKindSkillIndexContext, Text: "External skill index (metadata only):\n- repo-read: inspect a repository"},
		{Kind: ModelInputKindUserText, Text: "Read this repository and explain what it does."},
	})

	if len(messages) != 2 {
		t.Fatalf("messages = %#v, want stable system prefix and current user message", messages)
	}
	if messages[0].Role != "system" || !strings.Contains(messages[0].Text, "repo-read") {
		t.Fatalf("prefix = %#v, want system skill prefix", messages[0])
	}
	if messages[1].Role != "user" || messages[1].Text != "Read this repository and explain what it does." {
		t.Fatalf("current message = %#v, want distinct final user message", messages[1])
	}
}

func TestContextInspectionProjectsPersistedStablePrefixFingerprint(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	k.provider = &prefixFingerprintAccountingProvider{identity: "prefix-accounting\nadapter-a\nprofile\nprotocol\nmodel"}

	response, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "prefix-fingerprint-session",
		InputItems: []InputItem{{Type: "text", Text: "inspect the prefix fingerprint"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	inspection, err := k.ContextInspection(response.TurnID)
	if err != nil {
		t.Fatalf("ContextInspection returned error: %v", err)
	}
	if inspection.PrefixFingerprint == "" {
		t.Fatal("prefix fingerprint is empty")
	}

	session, err := k.Session("prefix-fingerprint-session")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	for _, event := range session.Events {
		if event.Type != "model.context.accounted" || event.Data.ModelContextAccounting == nil {
			continue
		}
		if event.Data.ModelContextAccounting.PrefixFingerprint != inspection.PrefixFingerprint {
			t.Fatalf("accounting fingerprint = %q, inspection fingerprint = %q", event.Data.ModelContextAccounting.PrefixFingerprint, inspection.PrefixFingerprint)
		}
		return
	}
	t.Fatal("model context accounting event was not recorded")
}

func TestContextInspectionExplainsPrefixChanges(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	provider := &prefixFingerprintAccountingProvider{identity: "prefix-accounting\nadapter-a\nprofile\nprotocol\nmodel"}
	k.provider = provider

	first, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "prefix-change-session",
		InputItems: []InputItem{{Type: "text", Text: "first"}},
	})
	if err != nil {
		t.Fatalf("first SubmitTurn returned error: %v", err)
	}
	firstInspection, err := k.ContextInspection(first.TurnID)
	if err != nil {
		t.Fatalf("first ContextInspection returned error: %v", err)
	}
	if got := strings.Join(contextInspectionPrefixChangeReasons(t, firstInspection), ","); got != "initial" {
		t.Fatalf("first prefix change reasons = %q, want initial", got)
	}

	provider.identity = "prefix-accounting\nadapter-b\nprofile\nprotocol\nmodel"
	second, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "prefix-change-session",
		InputItems: []InputItem{{Type: "text", Text: "second"}},
	})
	if err != nil {
		t.Fatalf("second SubmitTurn returned error: %v", err)
	}
	secondInspection, err := k.ContextInspection(second.TurnID)
	if err != nil {
		t.Fatalf("second ContextInspection returned error: %v", err)
	}
	if got := strings.Join(contextInspectionPrefixChangeReasons(t, secondInspection), ","); got != "adapter_binding" {
		t.Fatalf("second prefix change reasons = %q, want adapter_binding", got)
	}
}

func contextInspectionPrefixChangeReasons(t *testing.T, inspection ContextInspectionResponse) []string {
	t.Helper()
	encoded, err := json.Marshal(inspection)
	if err != nil {
		t.Fatalf("marshal context inspection: %v", err)
	}
	var payload struct {
		PrefixChangeReasons []string `json:"prefix_change_reasons"`
	}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("decode context inspection: %v", err)
	}
	return payload.PrefixChangeReasons
}

type prefixFingerprintAccountingProvider struct {
	identity string
}

func (*prefixFingerprintAccountingProvider) Name() string { return "prefix-accounting" }

func (p *prefixFingerprintAccountingProvider) PrefixIdentity() string {
	return p.identity
}

func (*prefixFingerprintAccountingProvider) Ready() ProviderStatus {
	return ProviderStatus{Name: "prefix-accounting", Readiness: ReadinessReady}
}

func (*prefixFingerprintAccountingProvider) Complete(context.Context, ModelRequest) (ModelResponse, error) {
	return ModelResponse{
		Text:  "ok",
		Model: "prefix-accounting-model",
		Usage: &TokenUsage{InputTokens: 3, OutputTokens: 1, TotalTokens: 4},
	}, nil
}
