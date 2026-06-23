package connectorruntime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProcessExternalEventSubmitsOpaqueKernelTurn(t *testing.T) {
	store := newTestInboundStore(t)
	client := &fakeTurnClient{
		response: TurnSubmitResponse{
			SessionID: "kernel-session",
			TurnID:    "turn-1",
			Final:     FinalAnswer{Text: "kernel final is local diagnostic only"},
		},
	}
	runtime := testInboundRuntime(store, client)

	result, err := runtime.ProcessExternalEvent(context.Background(), testExternalEvent("om_raw_message"))
	if err != nil {
		t.Fatalf("ProcessExternalEvent returned error: %v", err)
	}
	if result.Duplicate {
		t.Fatal("first external event should not be marked duplicate")
	}
	if client.calls != 1 {
		t.Fatalf("kernel calls = %d, want 1", client.calls)
	}
	req := client.requests[0]
	for _, value := range []string{req.SessionID, req.IdempotencyKey, result.Record.RequestID, result.Record.ApplicationSessionID, result.Record.KernelSessionID} {
		if value == "" || strings.Contains(value, "oc_raw_chat") || strings.Contains(value, "om_raw_message") || strings.Contains(value, "ou_raw_user") || strings.Contains(value, ":") {
			t.Fatalf("system id should be opaque and grammar-safe, got %q", value)
		}
	}
	if len(req.InputItems) != 1 || req.InputItems[0].Type != "text" {
		t.Fatalf("unexpected turn input: %+v", req.InputItems)
	}
	input := req.InputItems[0].Text
	for _, want := range []string{
		"External application event",
		"connector: feishu",
		"event_type: message.created",
		"source_validation: verified",
		"thread_kind: chat",
		"sender_display: Tom",
		"text:\nhello",
	} {
		if !strings.Contains(input, want) {
			t.Fatalf("turn input missing %q in:\n%s", want, input)
		}
	}
	for _, forbidden := range []string{
		"oc_raw_chat",
		"om_raw_message",
		"ou_raw_user",
		"credential",
		"api_key",
		"provider_context",
		"permission_mode",
		"sandbox_profile",
		"approval_policy",
	} {
		if strings.Contains(input, forbidden) {
			t.Fatalf("turn input contains forbidden external/control value %q in:\n%s", forbidden, input)
		}
	}
	if result.Record.Status != SubmissionStatusSubmitted {
		t.Fatalf("submission status = %q", result.Record.Status)
	}
	if result.FinalText != "kernel final is local diagnostic only" {
		t.Fatalf("transient final text = %q", result.FinalText)
	}
}

func TestProcessExternalEventDuplicateDoesNotSubmitAnotherTurn(t *testing.T) {
	store := newTestInboundStore(t)
	client := &fakeTurnClient{
		response: TurnSubmitResponse{SessionID: "kernel-session", TurnID: "turn-1", Final: FinalAnswer{Text: "first"}},
	}
	runtime := testInboundRuntime(store, client)

	if _, err := runtime.ProcessExternalEvent(context.Background(), testExternalEvent("om_raw_message")); err != nil {
		t.Fatalf("first ProcessExternalEvent returned error: %v", err)
	}
	result, err := runtime.ProcessExternalEvent(context.Background(), testExternalEvent("om_raw_message"))
	if err != nil {
		t.Fatalf("duplicate ProcessExternalEvent returned error: %v", err)
	}
	if !result.Duplicate {
		t.Fatal("duplicate external event should be marked duplicate")
	}
	if client.calls != 1 {
		t.Fatalf("kernel calls = %d, want 1", client.calls)
	}
	if result.Record.TurnID != "turn-1" {
		t.Fatalf("duplicate record turn id = %q", result.Record.TurnID)
	}
}

func TestInvalidExternalEventRejectedBeforeKernelCall(t *testing.T) {
	store := newTestInboundStore(t)
	client := &fakeTurnClient{}
	runtime := testInboundRuntime(store, client)

	_, err := runtime.ProcessExternalEvent(context.Background(), ExternalEvent{
		Connector:        "feishu",
		ExternalEventID:  "om_raw_message",
		EventType:        "message.created",
		ThreadRef:        ExternalThreadRef{Connector: "feishu", Kind: "chat", ExternalID: "oc_raw_chat"},
		SenderRef:        ExternalRef{Connector: "feishu", Kind: "user", ExternalID: "ou_raw_user"},
		MessageRef:       ExternalRef{Connector: "feishu", Kind: "message", ExternalID: "om_raw_message"},
		SourceValidation: SourceValidationVerified,
	})
	if err == nil {
		t.Fatal("ProcessExternalEvent should reject missing body")
	}
	if client.calls != 0 {
		t.Fatalf("kernel calls = %d, want 0", client.calls)
	}
}

func TestInvalidExternalEventRejectsMismatchedConnectorRefs(t *testing.T) {
	store := newTestInboundStore(t)
	client := &fakeTurnClient{}
	runtime := testInboundRuntime(store, client)
	event := testExternalEvent("om_raw_message")
	event.ThreadRef.Connector = "wechat"

	_, err := runtime.ProcessExternalEvent(context.Background(), event)
	if err == nil {
		t.Fatal("ProcessExternalEvent should reject mismatched connector refs")
	}
	if client.calls != 0 {
		t.Fatalf("kernel calls = %d, want 0", client.calls)
	}
}

func TestFileInboundStorePersistsDuplicateAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "connector-inbound-state.json")
	store, err := NewFileInboundStore(path)
	if err != nil {
		t.Fatalf("NewFileInboundStore returned error: %v", err)
	}
	runtime := testInboundRuntime(store, &fakeTurnClient{
		response: TurnSubmitResponse{SessionID: "kernel-session", TurnID: "turn-1", Final: FinalAnswer{Text: "reply"}},
	})
	if _, err := runtime.ProcessExternalEvent(context.Background(), testExternalEvent("om_raw_message")); err != nil {
		t.Fatalf("ProcessExternalEvent returned error: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if strings.Contains(string(content), "kernel_final_text") || strings.Contains(string(content), "reply") {
		t.Fatalf("inbound store should not persist kernel final answer, got:\n%s", string(content))
	}

	reopened, err := NewFileInboundStore(path)
	if err != nil {
		t.Fatalf("reopen store returned error: %v", err)
	}
	client := &fakeTurnClient{
		response: TurnSubmitResponse{SessionID: "kernel-session", TurnID: "turn-2", Final: FinalAnswer{Text: "second"}},
	}
	secondRuntime := testInboundRuntime(reopened, client)
	result, err := secondRuntime.ProcessExternalEvent(context.Background(), testExternalEvent("om_raw_message"))
	if err != nil {
		t.Fatalf("duplicate ProcessExternalEvent returned error: %v", err)
	}
	if !result.Duplicate {
		t.Fatal("reopened store should preserve duplicate state")
	}
	if client.calls != 0 {
		t.Fatalf("kernel calls after reopen = %d, want 0", client.calls)
	}
}

func newTestInboundStore(t *testing.T) *FileInboundStore {
	t.Helper()
	store, err := NewFileInboundStore(filepath.Join(t.TempDir(), "connector-inbound-state.json"))
	if err != nil {
		t.Fatalf("NewFileInboundStore returned error: %v", err)
	}
	return store
}

func testInboundRuntime(store InboundStore, client TurnClient) *Runtime {
	return &Runtime{
		InboundStore:  store,
		Client:        client,
		SessionMapper: DefaultApplicationSessionMapper{},
		Now: func() time.Time {
			return time.Date(2026, 6, 23, 10, 0, 0, 0, time.UTC)
		},
	}
}

func testExternalEvent(eventID string) ExternalEvent {
	return ExternalEvent{
		Connector:       "feishu",
		ExternalEventID: eventID,
		EventType:       "message.created",
		ThreadRef: ExternalThreadRef{
			Connector:  "feishu",
			Kind:       "chat",
			ExternalID: "oc_raw_chat",
			Display:    "Genesis test chat",
			Metadata:   map[string]string{"credential": "sk-thread-secret"},
		},
		SenderRef: ExternalRef{
			Connector:  "feishu",
			Kind:       "user",
			ExternalID: "ou_raw_user",
			Display:    "Tom",
			Metadata:   map[string]string{"api_key": "sk-user-secret"},
		},
		MessageRef:       ExternalRef{Connector: "feishu", Kind: "message", ExternalID: eventID},
		Body:             "hello",
		ReceivedAt:       time.Date(2026, 6, 23, 9, 0, 0, 0, time.UTC),
		SourceValidation: SourceValidationVerified,
		Metadata: map[string]string{
			"api_key": "sk-event-secret",
		},
	}
}

type fakeTurnClient struct {
	response TurnSubmitResponse
	err      error
	calls    int
	requests []TurnSubmitRequest
}

func (f *fakeTurnClient) SubmitTurn(_ context.Context, req TurnSubmitRequest) (TurnSubmitResponse, error) {
	f.calls++
	f.requests = append(f.requests, req)
	return f.response, f.err
}
