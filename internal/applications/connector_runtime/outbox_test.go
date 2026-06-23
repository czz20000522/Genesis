package connectorruntime

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEnqueueAppCommandCreatesOneOutboxItemWithOpaqueID(t *testing.T) {
	store := newTestOutboxStore(t)
	runtime := testRuntime(store, nil)
	command := testSendMessageCommand()

	item, duplicate, err := runtime.EnqueueCommand(context.Background(), command)
	if err != nil {
		t.Fatalf("EnqueueCommand returned error: %v", err)
	}
	if duplicate {
		t.Fatal("first command should not be duplicate")
	}
	if item.OutboxID == "" || strings.Contains(item.OutboxID, command.TargetRef.ExternalID) {
		t.Fatalf("outbox id should be opaque, got %q", item.OutboxID)
	}
	if item.IdempotencyKey == "" || strings.Contains(item.IdempotencyKey, command.DedupeKey) {
		t.Fatalf("idempotency key should be opaque, got %q", item.IdempotencyKey)
	}
	if item.Status != OutboxStatusQueued {
		t.Fatalf("status = %q, want queued", item.Status)
	}
	if item.Connector != "feishu" || item.ActionKind != "send_message" {
		t.Fatalf("item = %+v", item)
	}
	if item.Payload["body"] != "hello from app command" {
		t.Fatalf("payload = %+v", item.Payload)
	}
}

func TestDuplicateAppCommandSuppressesDuplicateOutboxItem(t *testing.T) {
	store := newTestOutboxStore(t)
	runtime := testRuntime(store, nil)
	command := testSendMessageCommand()

	first, duplicate, err := runtime.EnqueueCommand(context.Background(), command)
	if err != nil {
		t.Fatalf("first EnqueueCommand returned error: %v", err)
	}
	if duplicate {
		t.Fatal("first command should not be duplicate")
	}
	second, duplicate, err := runtime.EnqueueCommand(context.Background(), command)
	if err != nil {
		t.Fatalf("second EnqueueCommand returned error: %v", err)
	}
	if !duplicate {
		t.Fatal("second command should be duplicate")
	}
	if second.OutboxID != first.OutboxID {
		t.Fatalf("duplicate outbox id = %q, want %q", second.OutboxID, first.OutboxID)
	}
	items, err := store.ListOutbox(context.Background())
	if err != nil {
		t.Fatalf("ListOutbox returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("outbox item count = %d, want 1", len(items))
	}
}

func TestAppCommandMetadataIsNotCopiedToConnectorActionPayload(t *testing.T) {
	store := newTestOutboxStore(t)
	runtime := testRuntime(store, nil)
	command := testSendMessageCommand()
	command.Metadata = map[string]string{
		"credential_ref": "cred_feishu",
		"secret_note":    "do not expose",
		"display":        "safe but not needed for action",
	}

	item, _, err := runtime.EnqueueCommand(context.Background(), command)
	if err != nil {
		t.Fatalf("EnqueueCommand returned error: %v", err)
	}
	if _, ok := item.Payload["body"]; !ok {
		t.Fatalf("payload should contain body: %+v", item.Payload)
	}
	if len(item.Payload) != 1 {
		t.Fatalf("payload should only contain execution body, got %+v", item.Payload)
	}
}

func TestExternalThreadRefMetadataIsNotCopiedToOutboxAction(t *testing.T) {
	store := newTestOutboxStore(t)
	runtime := testRuntime(store, nil)
	command := testSendMessageCommand()
	command.TargetRef.Metadata = map[string]string{
		"api_key": "sk-secret",
		"note":    "origin-only",
	}

	item, _, err := runtime.EnqueueCommand(context.Background(), command)
	if err != nil {
		t.Fatalf("EnqueueCommand returned error: %v", err)
	}
	if len(item.TargetRef.Metadata) != 0 {
		t.Fatalf("target metadata should not enter outbox action target: %+v", item.TargetRef.Metadata)
	}
}

func TestExecuteConnectorActionRecordsFailedDeliveryReceipt(t *testing.T) {
	store := newTestOutboxStore(t)
	adapter := &fakeAdapter{
		result: ConnectorActionResult{
			Status: DeliveryStatusRetrying,
			Reason: "rate_limited",
		},
		err: errors.New("rate limit"),
	}
	runtime := testRuntime(store, map[string]ConnectorAdapter{"feishu": adapter})
	item, _, err := runtime.EnqueueCommand(context.Background(), testSendMessageCommand())
	if err != nil {
		t.Fatalf("EnqueueCommand returned error: %v", err)
	}

	receipt, err := runtime.ExecuteOutboxItem(context.Background(), item.OutboxID)
	if err == nil {
		t.Fatal("ExecuteOutboxItem should return adapter error")
	}
	if receipt.Status != DeliveryStatusRetrying || receipt.Reason != "rate_limited" {
		t.Fatalf("receipt = %+v", receipt)
	}
	if receipt.OutboxID != item.OutboxID || receipt.Connector != "feishu" {
		t.Fatalf("receipt identity = %+v", receipt)
	}
	if adapter.calls != 1 {
		t.Fatalf("adapter calls = %d, want 1", adapter.calls)
	}
	updated, err := store.GetOutboxItem(context.Background(), item.OutboxID)
	if err != nil {
		t.Fatalf("GetOutboxItem returned error: %v", err)
	}
	if updated.Status != OutboxStatusRetrying {
		t.Fatalf("updated status = %q, want retrying", updated.Status)
	}
	receipts, err := store.ListReceipts(context.Background(), item.OutboxID)
	if err != nil {
		t.Fatalf("ListReceipts returned error: %v", err)
	}
	if len(receipts) != 1 || receipts[0].Status != DeliveryStatusRetrying {
		t.Fatalf("receipts = %+v", receipts)
	}
}

func TestExecuteOutboxItemSuppressesDuplicateSentDelivery(t *testing.T) {
	store := newTestOutboxStore(t)
	adapter := &fakeAdapter{
		result: ConnectorActionResult{
			Status:            DeliveryStatusSent,
			ExternalActionRef: "om_123",
		},
	}
	runtime := testRuntime(store, map[string]ConnectorAdapter{"feishu": adapter})
	item, _, err := runtime.EnqueueCommand(context.Background(), testSendMessageCommand())
	if err != nil {
		t.Fatalf("EnqueueCommand returned error: %v", err)
	}
	first, err := runtime.ExecuteOutboxItem(context.Background(), item.OutboxID)
	if err != nil {
		t.Fatalf("first ExecuteOutboxItem returned error: %v", err)
	}
	if first.Status != DeliveryStatusSent {
		t.Fatalf("first receipt = %+v", first)
	}
	second, err := runtime.ExecuteOutboxItem(context.Background(), item.OutboxID)
	if err != nil {
		t.Fatalf("second ExecuteOutboxItem returned error: %v", err)
	}
	if second.Status != DeliveryStatusDuplicateSuppressed {
		t.Fatalf("second receipt = %+v, want duplicate_suppressed", second)
	}
	if adapter.calls != 1 {
		t.Fatalf("adapter calls = %d, want 1", adapter.calls)
	}
	receipts, err := store.ListReceipts(context.Background(), item.OutboxID)
	if err != nil {
		t.Fatalf("ListReceipts returned error: %v", err)
	}
	if len(receipts) != 2 {
		t.Fatalf("receipt count = %d, want 2", len(receipts))
	}
}

func TestFeishuAdapterUsesRunnerWithoutPuttingCredentialInActionPayload(t *testing.T) {
	runner := &recordingRunner{}
	adapter := FeishuAdapter{
		CLIPath: "lark-cli",
		Profile: "genesis",
		Runner:  runner,
	}
	action := ConnectorAction{
		OutboxID:       "outbox_1",
		Connector:      "feishu",
		ActionKind:     "send_message",
		TargetRef:      ExternalThreadRef{Connector: "feishu", Kind: "chat", ExternalID: "oc_123"},
		Payload:        map[string]string{"body": "hello"},
		IdempotencyKey: "idem_1",
		Attempt:        1,
	}

	result, err := adapter.Execute(context.Background(), action)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Status != DeliveryStatusSent {
		t.Fatalf("result = %+v", result)
	}
	wantArgs := []string{"im", "send", "--chat", "oc_123", "--text", "hello", "--profile", "genesis"}
	if strings.Join(runner.args, "\x00") != strings.Join(wantArgs, "\x00") {
		t.Fatalf("args = %#v, want %#v", runner.args, wantArgs)
	}
	for key, value := range action.Payload {
		if strings.Contains(strings.ToLower(key), "credential") || strings.Contains(strings.ToLower(value), "secret") {
			t.Fatalf("action payload exposes credential-shaped data: %+v", action.Payload)
		}
	}
}

func TestFeishuAdapterDoesNotPersistRawCLIOutputInReceiptFields(t *testing.T) {
	runner := &recordingRunner{
		output: []byte(`{"data":{"message_id":"om_456"},"debug":"Authorization: Bearer sk-secret"}`),
	}
	adapter := FeishuAdapter{
		CLIPath: "lark-cli",
		Profile: "genesis",
		Runner:  runner,
	}
	action := ConnectorAction{
		OutboxID:       "outbox_1",
		Connector:      "feishu",
		ActionKind:     "send_message",
		TargetRef:      ExternalThreadRef{Connector: "feishu", Kind: "chat", ExternalID: "oc_123"},
		Payload:        map[string]string{"body": "hello"},
		IdempotencyKey: "idem_1",
		Attempt:        1,
	}

	result, err := adapter.Execute(context.Background(), action)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.ExternalActionRef != "om_456" {
		t.Fatalf("external action ref = %q, want parsed message id", result.ExternalActionRef)
	}
	if strings.Contains(result.ExternalActionRef, "Authorization") || strings.Contains(result.Reason, "Authorization") {
		t.Fatalf("result leaked raw CLI output: %+v", result)
	}
}

func TestFeishuAdapterMapsFailureToConnectorReasonWithoutRawOutput(t *testing.T) {
	runner := &recordingRunner{
		output: []byte("Authorization: Bearer sk-secret\nrate limit exceeded"),
		err:    errors.New("exit status 1"),
	}
	adapter := FeishuAdapter{
		CLIPath: "lark-cli",
		Profile: "genesis",
		Runner:  runner,
	}
	action := ConnectorAction{
		OutboxID:       "outbox_1",
		Connector:      "feishu",
		ActionKind:     "send_message",
		TargetRef:      ExternalThreadRef{Connector: "feishu", Kind: "chat", ExternalID: "oc_123"},
		Payload:        map[string]string{"body": "hello"},
		IdempotencyKey: "idem_1",
		Attempt:        1,
	}

	result, err := adapter.Execute(context.Background(), action)
	if err == nil {
		t.Fatal("Execute should return command error")
	}
	if result.Reason != "external_command_failed" {
		t.Fatalf("reason = %q, want external_command_failed", result.Reason)
	}
	if strings.Contains(result.Reason, "Authorization") || strings.Contains(result.ExternalActionRef, "Authorization") {
		t.Fatalf("result leaked raw CLI output: %+v", result)
	}
}

func TestFeishuAdapterRejectsUnexpectedActionPayloadAndTargetMetadata(t *testing.T) {
	adapter := FeishuAdapter{
		CLIPath: "lark-cli",
		Profile: "genesis",
		Runner:  &recordingRunner{},
	}
	action := ConnectorAction{
		OutboxID:       "outbox_1",
		Connector:      "feishu",
		ActionKind:     "send_message",
		TargetRef:      ExternalThreadRef{Connector: "feishu", Kind: "chat", ExternalID: "oc_123", Metadata: map[string]string{"api_key": "sk-secret"}},
		Payload:        map[string]string{"body": "hello", "api_key": "sk-secret"},
		IdempotencyKey: "idem_1",
		Attempt:        1,
	}

	result, err := adapter.Execute(context.Background(), action)
	if err == nil {
		t.Fatal("Execute should reject unexpected payload and target metadata")
	}
	if result.Reason != "invalid_action_payload" {
		t.Fatalf("reason = %q, want invalid_action_payload", result.Reason)
	}
}

func newTestOutboxStore(t *testing.T) *FileOutboxStore {
	t.Helper()
	store, err := NewFileOutboxStore(filepath.Join(t.TempDir(), "connector-outbox.json"))
	if err != nil {
		t.Fatalf("NewFileOutboxStore returned error: %v", err)
	}
	return store
}

func testRuntime(store OutboxStore, adapters map[string]ConnectorAdapter) *Runtime {
	return &Runtime{
		Store:    store,
		Adapters: adapters,
		Now: func() time.Time {
			return time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
		},
	}
}

func testSendMessageCommand() AppCommand {
	return AppCommand{
		CommandID: "cmd-raw-1",
		Kind:      "send_message",
		TargetRef: ExternalThreadRef{
			Connector:  "feishu",
			Kind:       "chat",
			ExternalID: "oc_123",
			Display:    "Genesis test chat",
		},
		Body:      "hello from app command",
		DedupeKey: "feishu:oc_123:cmd-raw-1",
		CreatedAt: time.Date(2026, 6, 23, 11, 0, 0, 0, time.UTC),
	}
}

type fakeAdapter struct {
	result ConnectorActionResult
	err    error
	calls  int
}

func (f *fakeAdapter) Execute(_ context.Context, _ ConnectorAction) (ConnectorActionResult, error) {
	f.calls++
	return f.result, f.err
}

type recordingRunner struct {
	name   string
	args   []string
	output []byte
	err    error
}

func (r *recordingRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.name = name
	r.args = append([]string(nil), args...)
	if r.output == nil {
		r.output = []byte(`{"message_id":"om_123"}`)
	}
	return r.output, r.err
}
