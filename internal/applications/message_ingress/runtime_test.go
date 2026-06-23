package messageingress

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProcessSubmitsOneInboundTurnWithoutOutboundDelivery(t *testing.T) {
	store := newTestStore(t)
	client := &fakeTurnClient{
		response: TurnSubmitResponse{
			SessionID: "kernel-session",
			TurnID:    "turn-1",
			Final:     FinalAnswer{Text: "kernel final is local diagnostic only"},
		},
	}
	runtime := testRuntime(store, client)

	result, err := runtime.Process(context.Background(), testMessage("msg-1"))
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if result.Duplicate {
		t.Fatal("first inbound delivery should not be marked duplicate")
	}
	if client.calls != 1 {
		t.Fatalf("kernel calls = %d, want 1", client.calls)
	}
	req := client.requests[0]
	if req.SessionID == "" || req.SessionID == "chat-1" || strings.Contains(req.SessionID, "chat") {
		t.Fatalf("session id should be opaque and non-empty, got %q", req.SessionID)
	}
	if req.IdempotencyKey == "" || strings.Contains(req.IdempotencyKey, "msg-1") || strings.Contains(req.IdempotencyKey, ":") {
		t.Fatalf("kernel idempotency key should be opaque and grammar-safe, got %q", req.IdempotencyKey)
	}
	if len(req.InputItems) != 1 || req.InputItems[0].Type != "text" {
		t.Fatalf("unexpected turn input: %+v", req.InputItems)
	}
	if !strings.Contains(req.InputItems[0].Text, "source_channel: feishu") {
		t.Fatalf("turn input missing source channel: %q", req.InputItems[0].Text)
	}
	if !strings.Contains(req.InputItems[0].Text, "text:\nhello") {
		t.Fatalf("turn input missing message text: %q", req.InputItems[0].Text)
	}
	if result.Record.Status != SubmissionStatusSubmitted {
		t.Fatalf("submission status = %q", result.Record.Status)
	}
	if result.FinalText != "kernel final is local diagnostic only" {
		t.Fatalf("transient final text = %q", result.FinalText)
	}
}

func TestProcessDuplicateDoesNotSubmitAnotherTurn(t *testing.T) {
	store := newTestStore(t)
	client := &fakeTurnClient{
		response: TurnSubmitResponse{SessionID: "kernel-session", TurnID: "turn-1", Final: FinalAnswer{Text: "first"}},
	}
	runtime := testRuntime(store, client)

	if _, err := runtime.Process(context.Background(), testMessage("msg-1")); err != nil {
		t.Fatalf("first Process returned error: %v", err)
	}
	result, err := runtime.Process(context.Background(), testMessage("msg-1"))
	if err != nil {
		t.Fatalf("duplicate Process returned error: %v", err)
	}
	if !result.Duplicate {
		t.Fatal("duplicate inbound delivery should be marked duplicate")
	}
	if client.calls != 1 {
		t.Fatalf("kernel calls = %d, want 1", client.calls)
	}
	if result.Record.TurnID != "turn-1" {
		t.Fatalf("duplicate record turn id = %q", result.Record.TurnID)
	}
}

func TestInvalidEnvelopeRejectedBeforeKernelCall(t *testing.T) {
	store := newTestStore(t)
	client := &fakeTurnClient{}
	runtime := testRuntime(store, client)

	_, err := runtime.Process(context.Background(), ChannelMessage{
		Channel:   "feishu",
		Adapter:   "feishu-inbound",
		MessageID: "msg-1",
		ThreadID:  "chat-1",
		UserID:    "user-1",
	})
	if err == nil {
		t.Fatal("Process should reject missing text")
	}
	if client.calls != 0 {
		t.Fatalf("kernel calls = %d, want 0", client.calls)
	}
}

func TestInboundContextIncludesReplyReferenceButNoAuthorityFields(t *testing.T) {
	msg := testMessage("msg-1")
	msg.Metadata = map[string]string{
		"chat_id":        "oc_123",
		"sender_display": "Tom",
	}
	input := FormatInboundInput(msg)
	for _, want := range []string{
		"source_channel: feishu",
		"adapter: feishu-inbound",
		"thread_id: chat-1",
		"message_id: msg-1",
		"sender_id: user-1",
		"chat_id: oc_123",
		"sender_display: Tom",
	} {
		if !strings.Contains(input, want) {
			t.Fatalf("inbound input missing %q in:\n%s", want, input)
		}
	}
	for _, forbidden := range []string{"permission_mode", "sandbox_profile", "approval_policy", "credential", "provider_context"} {
		if strings.Contains(input, forbidden) {
			t.Fatalf("inbound input contains authority field %q in:\n%s", forbidden, input)
		}
	}
}

func TestFileInboundStorePersistsDuplicateAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "message-ingress-state.json")
	store, err := NewFileInboundStore(path)
	if err != nil {
		t.Fatalf("NewFileInboundStore returned error: %v", err)
	}
	runtime := testRuntime(store, &fakeTurnClient{
		response: TurnSubmitResponse{SessionID: "kernel-session", TurnID: "turn-1", Final: FinalAnswer{Text: "reply"}},
	})
	if _, err := runtime.Process(context.Background(), testMessage("msg-1")); err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if strings.Contains(string(content), "kernel_final_text") || strings.Contains(string(content), "reply") {
		t.Fatalf("ingress store should not persist kernel final answer, got:\n%s", string(content))
	}

	reopened, err := NewFileInboundStore(path)
	if err != nil {
		t.Fatalf("reopen store returned error: %v", err)
	}
	client := &fakeTurnClient{
		response: TurnSubmitResponse{SessionID: "kernel-session", TurnID: "turn-2", Final: FinalAnswer{Text: "second"}},
	}
	secondRuntime := testRuntime(reopened, client)
	result, err := secondRuntime.Process(context.Background(), testMessage("msg-1"))
	if err != nil {
		t.Fatalf("duplicate Process returned error: %v", err)
	}
	if !result.Duplicate {
		t.Fatal("reopened store should preserve duplicate state")
	}
	if client.calls != 0 {
		t.Fatalf("kernel calls after reopen = %d, want 0", client.calls)
	}
}

func newTestStore(t *testing.T) *FileInboundStore {
	t.Helper()
	store, err := NewFileInboundStore(filepath.Join(t.TempDir(), "message-ingress-state.json"))
	if err != nil {
		t.Fatalf("NewFileInboundStore returned error: %v", err)
	}
	return store
}

func testRuntime(store InboundStore, client TurnClient) *Runtime {
	return &Runtime{
		Store:  store,
		Client: client,
		Mapper: DefaultSessionMapper{},
		Now: func() time.Time {
			return time.Date(2026, 6, 23, 10, 0, 0, 0, time.UTC)
		},
	}
}

func testMessage(messageID string) ChannelMessage {
	return ChannelMessage{
		Channel:    "feishu",
		Adapter:    "feishu-inbound",
		MessageID:  messageID,
		ThreadID:   "chat-1",
		UserID:     "user-1",
		Text:       "hello",
		ReceivedAt: time.Date(2026, 6, 23, 9, 0, 0, 0, time.UTC),
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
