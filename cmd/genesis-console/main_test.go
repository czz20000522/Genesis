package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"genesis/internal/applications/connector_runtime"
)

func TestConsoleInspectReadsConnectorStateAndKernelProjection(t *testing.T) {
	ctx := context.Background()
	inboundPath := filepath.Join(t.TempDir(), "inbound.json")
	outboxPath := filepath.Join(t.TempDir(), "outbox.json")

	inboundStore, err := connectorruntime.NewFileInboundStore(inboundPath)
	if err != nil {
		t.Fatalf("NewFileInboundStore returned error: %v", err)
	}
	record := connectorruntime.InboundSubmissionRecord{
		RequestID:            "req_1",
		DedupeKey:            "dedupe_1",
		KernelIdempotencyKey: "turn_1",
		Connector:            "feishu",
		EventType:            "message.created",
		ApplicationSessionID: "app_session_1",
		KernelSessionID:      "kernel_session_1",
		TurnID:               "turn_1",
		Status:               connectorruntime.SubmissionStatusSubmitted,
		CreatedAt:            time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC),
		UpdatedAt:            time.Date(2026, 6, 24, 12, 0, 1, 0, time.UTC),
	}
	if _, reserved, err := inboundStore.Reserve(ctx, record); err != nil || !reserved {
		t.Fatalf("Reserve returned reserved=%v err=%v", reserved, err)
	}
	if err := inboundStore.Complete(ctx, record); err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}

	outboxStore, err := connectorruntime.NewFileOutboxStore(outboxPath)
	if err != nil {
		t.Fatalf("NewFileOutboxStore returned error: %v", err)
	}
	item, _, err := outboxStore.EnqueueCommand(ctx, connectorruntime.AppCommand{
		CommandID: "cmd_1",
		Kind:      "send_message",
		TargetRef: connectorruntime.ExternalThreadRef{
			Connector:  "feishu",
			Kind:       "chat",
			ExternalID: "oc_123",
		},
		Body:      "reply",
		DedupeKey: "reply_1",
	}, time.Date(2026, 6, 24, 12, 0, 2, 0, time.UTC))
	if err != nil {
		t.Fatalf("EnqueueCommand returned error: %v", err)
	}
	item.Status = connectorruntime.OutboxStatusSent
	item.AttemptCount = 1
	receipt := connectorruntime.DeliveryReceipt{
		ReceiptID:         "receipt_1",
		OutboxID:          item.OutboxID,
		Connector:         "feishu",
		ExternalActionRef: "om_123",
		Status:            connectorruntime.DeliveryStatusSent,
		Attempt:           1,
		RecordedAt:        time.Date(2026, 6, 24, 12, 0, 3, 0, time.UTC),
	}
	if err := outboxStore.RecordDelivery(ctx, item, receipt); err != nil {
		t.Fatalf("RecordDelivery returned error: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sessions/kernel_session_1" {
			t.Fatalf("path = %q, want /sessions/kernel_session_1", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Fatalf("authorization = %q, want bearer token", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"session_id": "kernel_session_1",
			"turns":      []map[string]string{{"turn_id": "turn_1", "status": "completed"}},
		})
	}))
	t.Cleanup(server.Close)

	var stdout bytes.Buffer
	if err := run(ctx, []string{
		"inspect",
		"--inbound-state", inboundPath,
		"--outbox-state", outboxPath,
		"--kernel-url", server.URL,
		"--runtime-token", "token",
	}, &stdout, io.Discard); err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	var got InspectionReport
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode report: %v\n%s", err, stdout.String())
	}
	if len(got.Inbound) != 1 || got.Inbound[0].KernelSessionID != "kernel_session_1" {
		t.Fatalf("inbound = %+v", got.Inbound)
	}
	if len(got.Outbox) != 1 || got.Outbox[0].Status != connectorruntime.OutboxStatusSent {
		t.Fatalf("outbox = %+v", got.Outbox)
	}
	if len(got.Receipts[item.OutboxID]) != 1 || got.Receipts[item.OutboxID][0].ExternalActionRef != "om_123" {
		t.Fatalf("receipts = %+v", got.Receipts)
	}
	if string(got.KernelSessions["kernel_session_1"]) == "" {
		t.Fatalf("kernel session projection missing: %+v", got.KernelSessions)
	}
}

func TestConsoleRequeueOutboxMutatesOnlyConnectorState(t *testing.T) {
	ctx := context.Background()
	outboxPath := filepath.Join(t.TempDir(), "outbox.json")
	outboxStore, err := connectorruntime.NewFileOutboxStore(outboxPath)
	if err != nil {
		t.Fatalf("NewFileOutboxStore returned error: %v", err)
	}
	item, _, err := outboxStore.EnqueueCommand(ctx, connectorruntime.AppCommand{
		CommandID: "cmd_1",
		Kind:      "send_message",
		TargetRef: connectorruntime.ExternalThreadRef{
			Connector:  "feishu",
			Kind:       "chat",
			ExternalID: "oc_123",
		},
		Body:      "reply",
		DedupeKey: "reply_1",
	}, time.Date(2026, 6, 24, 12, 0, 2, 0, time.UTC))
	if err != nil {
		t.Fatalf("EnqueueCommand returned error: %v", err)
	}
	item.Status = connectorruntime.OutboxStatusDeadLetter
	if err := outboxStore.RecordDelivery(ctx, item, connectorruntime.DeliveryReceipt{
		ReceiptID:  "receipt_1",
		OutboxID:   item.OutboxID,
		Connector:  "feishu",
		Status:     connectorruntime.DeliveryStatusDeadLettered,
		Reason:     "invalid_target",
		Attempt:    1,
		RecordedAt: time.Date(2026, 6, 24, 12, 0, 3, 0, time.UTC),
	}); err != nil {
		t.Fatalf("RecordDelivery returned error: %v", err)
	}

	var stdout bytes.Buffer
	if err := run(ctx, []string{
		"requeue-outbox",
		"--outbox-state", outboxPath,
		"--outbox-id", item.OutboxID,
		"--reason", "operator_requeued",
	}, &stdout, io.Discard); err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	var got RecoveryResult
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode recovery result: %v\n%s", err, stdout.String())
	}
	if got.Item.Status != connectorruntime.OutboxStatusQueued {
		t.Fatalf("item status = %q, want queued", got.Item.Status)
	}
	if got.Receipt.Status != connectorruntime.DeliveryStatusRetrying || got.Receipt.Reason != "operator_requeued" {
		t.Fatalf("receipt = %+v", got.Receipt)
	}

	reloaded, err := connectorruntime.NewFileOutboxStore(outboxPath)
	if err != nil {
		t.Fatalf("reload outbox store: %v", err)
	}
	receipts, err := reloaded.ListReceipts(ctx, item.OutboxID)
	if err != nil {
		t.Fatalf("ListReceipts returned error: %v", err)
	}
	if len(receipts) != 2 {
		t.Fatalf("receipt history count = %d, want 2", len(receipts))
	}
}
