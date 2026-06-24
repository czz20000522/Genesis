package connectorruntime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"genesis/internal/testsupport"
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
		"source_validation: unchecked",
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
	if result.OutboxItem != nil || result.DeliveryReceipt != nil || result.DeliveryError != "" {
		t.Fatalf("inbound-only runtime should not produce outbound delivery evidence: %+v", result)
	}
}

func TestProcessExternalEventDowngradesDirectVerifiedClaim(t *testing.T) {
	store := newTestInboundStore(t)
	client := &fakeTurnClient{
		response: TurnSubmitResponse{SessionID: "kernel-session", TurnID: "turn-1"},
	}
	runtime := testInboundRuntime(store, client)
	event := testExternalEvent("om_direct_verified_claim")
	event.SourceValidation = SourceValidationVerified

	if _, err := runtime.ProcessExternalEvent(context.Background(), event); err != nil {
		t.Fatalf("ProcessExternalEvent returned error: %v", err)
	}
	if client.calls != 1 {
		t.Fatalf("kernel calls = %d, want 1", client.calls)
	}
	input := client.requests[0].InputItems[0].Text
	if strings.Contains(input, "source_validation: verified") {
		t.Fatalf("direct external event self-claimed verified in kernel input:\n%s", input)
	}
	if !strings.Contains(input, "source_validation: unchecked") {
		t.Fatalf("kernel input = %q, want unchecked source validation", input)
	}
}

func TestProcessSourceCommandEventPreservesVerifiedAfterBoundaryValidation(t *testing.T) {
	store := newTestInboundStore(t)
	client := &fakeTurnClient{
		response: TurnSubmitResponse{SessionID: "kernel-session", TurnID: "turn-1"},
	}
	runtime := testInboundRuntime(store, client)
	event := testExternalEvent("om_source_verified")
	event.SourceValidation = SourceValidationVerified

	if _, err := runtime.ProcessSourceCommandEvent(context.Background(), event); err != nil {
		t.Fatalf("ProcessSourceCommandEvent returned error: %v", err)
	}
	if client.calls != 1 {
		t.Fatalf("kernel calls = %d, want 1", client.calls)
	}
	input := client.requests[0].InputItems[0].Text
	if !strings.Contains(input, "source_validation: verified") {
		t.Fatalf("source command event did not preserve verified validation:\n%s", input)
	}
}

func TestProcessExternalEventDeliversFinalTextThroughConnectorOutbox(t *testing.T) {
	inboundStore := newTestInboundStore(t)
	outboxStore := newTestOutboxStore(t)
	adapter := &fakeAdapter{
		result: ConnectorActionResult{Status: DeliveryStatusSent, ExternalActionRef: "om_123"},
	}
	client := &fakeTurnClient{
		response: TurnSubmitResponse{
			SessionID: "kernel-session",
			TurnID:    "turn-1",
			Final:     FinalAnswer{Text: "reply from kernel"},
		},
	}
	runtime := testInboundRuntime(inboundStore, client)
	runtime.Store = outboxStore
	runtime.Adapters = map[string]ConnectorAdapter{"feishu": adapter}

	result, err := runtime.ProcessExternalEvent(context.Background(), testExternalEvent("om_reply_source"))
	if err != nil {
		t.Fatalf("ProcessExternalEvent returned error: %v", err)
	}
	if result.Record.Status != SubmissionStatusSubmitted {
		t.Fatalf("submission status = %q, want submitted", result.Record.Status)
	}
	if result.OutboxItem == nil {
		t.Fatalf("expected connector outbox item in result: %+v", result)
	}
	if result.OutboxItem.Connector != "feishu" || result.OutboxItem.ActionKind != "send_message" {
		t.Fatalf("outbox item = %+v", result.OutboxItem)
	}
	if result.OutboxItem.Payload["body"] != "reply from kernel" {
		t.Fatalf("outbox payload = %+v", result.OutboxItem.Payload)
	}
	if result.DeliveryReceipt == nil || result.DeliveryReceipt.Status != DeliveryStatusSent {
		t.Fatalf("delivery receipt = %+v", result.DeliveryReceipt)
	}
	if result.DeliveryError != "" {
		t.Fatalf("delivery error = %q", result.DeliveryError)
	}
	if adapter.calls != 1 {
		t.Fatalf("adapter calls = %d, want 1", adapter.calls)
	}
	if len(adapter.actions) != 1 {
		t.Fatalf("adapter actions = %+v", adapter.actions)
	}
	action := adapter.actions[0]
	if action.TargetRef.ExternalID != "oc_raw_chat" {
		t.Fatalf("action target = %+v, want original connector-owned external chat id", action.TargetRef)
	}
	if action.Payload["body"] != "reply from kernel" {
		t.Fatalf("action payload = %+v", action.Payload)
	}
	items, err := outboxStore.ListOutbox(context.Background())
	if err != nil {
		t.Fatalf("ListOutbox returned error: %v", err)
	}
	if len(items) != 1 || items[0].Status != OutboxStatusSent {
		t.Fatalf("outbox items = %+v, want one sent item", items)
	}
	receipts, err := outboxStore.ListReceipts(context.Background(), items[0].OutboxID)
	if err != nil {
		t.Fatalf("ListReceipts returned error: %v", err)
	}
	if len(receipts) != 1 || receipts[0].ExternalActionRef != "om_123" {
		t.Fatalf("receipts = %+v", receipts)
	}
}

func TestProcessExternalEventDuplicateDoesNotDeliverFinalAgain(t *testing.T) {
	inboundStore := newTestInboundStore(t)
	outboxStore := newTestOutboxStore(t)
	adapter := &fakeAdapter{
		result: ConnectorActionResult{Status: DeliveryStatusSent, ExternalActionRef: "om_123"},
	}
	client := &fakeTurnClient{
		response: TurnSubmitResponse{SessionID: "kernel-session", TurnID: "turn-1", Final: FinalAnswer{Text: "reply once"}},
	}
	runtime := testInboundRuntime(inboundStore, client)
	runtime.Store = outboxStore
	runtime.Adapters = map[string]ConnectorAdapter{"feishu": adapter}

	if _, err := runtime.ProcessExternalEvent(context.Background(), testExternalEvent("om_duplicate_delivery")); err != nil {
		t.Fatalf("first ProcessExternalEvent returned error: %v", err)
	}
	result, err := runtime.ProcessExternalEvent(context.Background(), testExternalEvent("om_duplicate_delivery"))
	if err != nil {
		t.Fatalf("duplicate ProcessExternalEvent returned error: %v", err)
	}
	if !result.Duplicate {
		t.Fatal("duplicate external event should be marked duplicate")
	}
	if result.OutboxItem != nil || result.DeliveryReceipt != nil {
		t.Fatalf("duplicate inbound event should not deliver again: %+v", result)
	}
	if client.calls != 1 {
		t.Fatalf("kernel calls = %d, want 1", client.calls)
	}
	if adapter.calls != 1 {
		t.Fatalf("adapter calls = %d, want 1", adapter.calls)
	}
	items, err := outboxStore.ListOutbox(context.Background())
	if err != nil {
		t.Fatalf("ListOutbox returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("outbox item count = %d, want 1", len(items))
	}
}

func TestProcessExternalEventDeliveryFailureDoesNotFailKernelSubmission(t *testing.T) {
	inboundStore := newTestInboundStore(t)
	outboxStore := newTestOutboxStore(t)
	adapter := &fakeAdapter{
		result: ConnectorActionResult{Status: DeliveryStatusRetrying, Reason: "rate_limited"},
		err:    errors.New("rate limit"),
	}
	client := &fakeTurnClient{
		response: TurnSubmitResponse{SessionID: "kernel-session", TurnID: "turn-1", Final: FinalAnswer{Text: "reply later"}},
	}
	runtime := testInboundRuntime(inboundStore, client)
	runtime.Store = outboxStore
	runtime.Adapters = map[string]ConnectorAdapter{"feishu": adapter}

	result, err := runtime.ProcessExternalEvent(context.Background(), testExternalEvent("om_delivery_retry"))
	if err != nil {
		t.Fatalf("connector delivery failure must not fail inbound kernel submission: %v", err)
	}
	if result.Record.Status != SubmissionStatusSubmitted || result.Record.KernelError != "" {
		t.Fatalf("inbound record = %+v, want submitted without kernel error", result.Record)
	}
	if result.DeliveryReceipt == nil || result.DeliveryReceipt.Status != DeliveryStatusRetrying {
		t.Fatalf("delivery receipt = %+v, want retrying", result.DeliveryReceipt)
	}
	if result.DeliveryError == "" {
		t.Fatalf("expected connector-local delivery error in result: %+v", result)
	}
	updated, err := outboxStore.GetOutboxItem(context.Background(), result.OutboxItem.OutboxID)
	if err != nil {
		t.Fatalf("GetOutboxItem returned error: %v", err)
	}
	if updated.Status != OutboxStatusRetrying {
		t.Fatalf("outbox status = %q, want retrying", updated.Status)
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
		SourceValidation: SourceValidationUnchecked,
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
	path := filepath.Join(testsupport.ProjectTempDir(t, "connector-inbound-reopen"), "connector-inbound-state.json")
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

func TestFileInboundStoreConcurrentInstancesPreserveIndependentReservations(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(testsupport.ProjectTempDir(t, "connector-inbound-concurrent"), "connector-inbound-state.json")
	first, err := NewFileInboundStore(path)
	if err != nil {
		t.Fatalf("first NewFileInboundStore returned error: %v", err)
	}
	second, err := NewFileInboundStore(path)
	if err != nil {
		t.Fatalf("second NewFileInboundStore returned error: %v", err)
	}

	firstRecord := testInboundRecord("dedupe_first", "turn_first")
	if _, reserved, err := first.Reserve(ctx, firstRecord); err != nil || !reserved {
		t.Fatalf("first Reserve returned reserved=%v err=%v", reserved, err)
	}
	secondRecord := testInboundRecord("dedupe_second", "turn_second")
	if _, reserved, err := second.Reserve(ctx, secondRecord); err != nil || !reserved {
		t.Fatalf("second Reserve returned reserved=%v err=%v", reserved, err)
	}

	reloaded, err := NewFileInboundStore(path)
	if err != nil {
		t.Fatalf("reload NewFileInboundStore returned error: %v", err)
	}
	records, err := reloaded.ListInbound(ctx)
	if err != nil {
		t.Fatalf("ListInbound returned error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records after two independent writers = %+v, want both writes preserved", records)
	}
}

func newTestInboundStore(t *testing.T) *FileInboundStore {
	t.Helper()
	store, err := NewFileInboundStore(filepath.Join(testsupport.ProjectTempDir(t, "connector-inbound"), "connector-inbound-state.json"))
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

func testInboundRecord(dedupeKey string, turnID string) InboundSubmissionRecord {
	return InboundSubmissionRecord{
		RequestID:            stableOpaqueID("request", dedupeKey),
		DedupeKey:            dedupeKey,
		KernelIdempotencyKey: turnID,
		Connector:            "feishu",
		EventType:            "message.created",
		ApplicationSessionID: "app_" + turnID,
		KernelSessionID:      "kernel_" + turnID,
		TurnID:               turnID,
		Status:               SubmissionStatusPending,
		CreatedAt:            time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC),
		UpdatedAt:            time.Date(2026, 6, 24, 12, 0, 1, 0, time.UTC),
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
		SourceValidation: SourceValidationUnchecked,
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
