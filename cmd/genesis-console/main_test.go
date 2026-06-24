package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"genesis/internal/applications/connector_runtime"
	"genesis/internal/testsupport"
)

func TestConsoleInspectReadsConnectorStateAndKernelProjection(t *testing.T) {
	ctx := context.Background()
	dir := testsupport.ProjectTempDir(t, "genesis-console-inspect")
	inboundPath := filepath.Join(dir, "inbound.json")
	outboxPath := filepath.Join(dir, "outbox.json")

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

func TestConsoleInspectFiltersConnectorStatusAndSession(t *testing.T) {
	ctx := context.Background()
	dir := testsupport.ProjectTempDir(t, "genesis-console-filters")
	inboundPath := filepath.Join(dir, "inbound.json")
	outboxPath := filepath.Join(dir, "outbox.json")

	inboundStore, err := connectorruntime.NewFileInboundStore(inboundPath)
	if err != nil {
		t.Fatalf("NewFileInboundStore returned error: %v", err)
	}
	keepInbound := connectorruntime.InboundSubmissionRecord{
		RequestID:            "req_keep",
		DedupeKey:            "dedupe_keep",
		KernelIdempotencyKey: "turn_keep",
		Connector:            "feishu",
		EventType:            "message.created",
		ApplicationSessionID: "app_keep",
		KernelSessionID:      "kernel_keep",
		TurnID:               "turn_keep",
		Status:               connectorruntime.SubmissionStatusSubmitted,
		CreatedAt:            time.Date(2026, 6, 24, 12, 1, 0, 0, time.UTC),
		UpdatedAt:            time.Date(2026, 6, 24, 12, 1, 1, 0, time.UTC),
	}
	dropInbound := keepInbound
	dropInbound.RequestID = "req_drop"
	dropInbound.DedupeKey = "dedupe_drop"
	dropInbound.KernelIdempotencyKey = "turn_drop"
	dropInbound.Connector = "email"
	dropInbound.ApplicationSessionID = "app_drop"
	dropInbound.KernelSessionID = "kernel_drop"
	dropInbound.TurnID = "turn_drop"
	dropInbound.Status = connectorruntime.SubmissionStatusFailed
	for _, record := range []connectorruntime.InboundSubmissionRecord{keepInbound, dropInbound} {
		if _, reserved, err := inboundStore.Reserve(ctx, record); err != nil || !reserved {
			t.Fatalf("Reserve(%s) returned reserved=%v err=%v", record.RequestID, reserved, err)
		}
		if err := inboundStore.Complete(ctx, record); err != nil {
			t.Fatalf("Complete(%s) returned error: %v", record.RequestID, err)
		}
	}

	outboxStore, err := connectorruntime.NewFileOutboxStore(outboxPath)
	if err != nil {
		t.Fatalf("NewFileOutboxStore returned error: %v", err)
	}
	keepItem, _, err := outboxStore.EnqueueCommand(ctx, connectorruntime.AppCommand{
		CommandID: "cmd_keep",
		Kind:      "send_message",
		TargetRef: connectorruntime.ExternalThreadRef{
			Connector:  "feishu",
			Kind:       "chat",
			ExternalID: "oc_keep",
		},
		Body:      "keep",
		DedupeKey: "reply_keep",
	}, time.Date(2026, 6, 24, 12, 1, 2, 0, time.UTC))
	if err != nil {
		t.Fatalf("enqueue keep item: %v", err)
	}
	keepItem.Status = connectorruntime.OutboxStatusDeadLetter
	if err := outboxStore.RecordDelivery(ctx, keepItem, connectorruntime.DeliveryReceipt{
		ReceiptID:  "receipt_keep",
		OutboxID:   keepItem.OutboxID,
		Connector:  "feishu",
		Status:     connectorruntime.DeliveryStatusDeadLettered,
		Attempt:    1,
		RecordedAt: time.Date(2026, 6, 24, 12, 1, 3, 0, time.UTC),
	}); err != nil {
		t.Fatalf("record keep receipt: %v", err)
	}
	dropItem, _, err := outboxStore.EnqueueCommand(ctx, connectorruntime.AppCommand{
		CommandID: "cmd_drop",
		Kind:      "send_message",
		TargetRef: connectorruntime.ExternalThreadRef{
			Connector:  "email",
			Kind:       "thread",
			ExternalID: "mail_drop",
		},
		Body:      "drop",
		DedupeKey: "reply_drop",
	}, time.Date(2026, 6, 24, 12, 1, 4, 0, time.UTC))
	if err != nil {
		t.Fatalf("enqueue drop item: %v", err)
	}
	dropItem.Status = connectorruntime.OutboxStatusSent
	if err := outboxStore.RecordDelivery(ctx, dropItem, connectorruntime.DeliveryReceipt{
		ReceiptID:  "receipt_drop",
		OutboxID:   dropItem.OutboxID,
		Connector:  "email",
		Status:     connectorruntime.DeliveryStatusSent,
		Attempt:    1,
		RecordedAt: time.Date(2026, 6, 24, 12, 1, 5, 0, time.UTC),
	}); err != nil {
		t.Fatalf("record drop receipt: %v", err)
	}

	var requestedSessions []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedSessions = append(requestedSessions, strings.TrimPrefix(r.URL.Path, "/sessions/"))
		_ = json.NewEncoder(w).Encode(map[string]string{"session_id": "kernel_keep"})
	}))
	t.Cleanup(server.Close)

	var stdout bytes.Buffer
	if err := run(ctx, []string{
		"inspect",
		"--inbound-state", inboundPath,
		"--outbox-state", outboxPath,
		"--kernel-url", server.URL,
		"--runtime-token", "token",
		"--connector", "feishu",
		"--inbound-status", connectorruntime.SubmissionStatusSubmitted,
		"--outbox-status", connectorruntime.OutboxStatusDeadLetter,
		"--kernel-session-id", "kernel_keep",
	}, &stdout, io.Discard); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	var got InspectionReport
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode report: %v\n%s", err, stdout.String())
	}
	if len(got.Inbound) != 1 || got.Inbound[0].RequestID != "req_keep" {
		t.Fatalf("filtered inbound = %+v", got.Inbound)
	}
	if len(got.Outbox) != 1 || got.Outbox[0].OutboxID != keepItem.OutboxID {
		t.Fatalf("filtered outbox = %+v", got.Outbox)
	}
	if len(got.Receipts) != 1 || len(got.Receipts[keepItem.OutboxID]) != 1 {
		t.Fatalf("filtered receipts = %+v", got.Receipts)
	}
	if len(requestedSessions) != 1 || requestedSessions[0] != "kernel_keep" {
		t.Fatalf("requested sessions = %+v, want kernel_keep only", requestedSessions)
	}
}

func TestConsoleInspectIncludesFilteredSourceFailures(t *testing.T) {
	ctx := context.Background()
	dir := testsupport.ProjectTempDir(t, "genesis-console-source-failures")
	inboundPath := filepath.Join(dir, "inbound.json")
	outboxPath := filepath.Join(dir, "outbox.json")
	sourcePath := filepath.Join(dir, "source-failures.json")
	store, err := connectorruntime.NewFileSourceFailureStore(sourcePath)
	if err != nil {
		t.Fatalf("NewFileSourceFailureStore returned error: %v", err)
	}
	if err := store.RecordSourceFailure(ctx, connectorruntime.SourceFailureRecord{
		RecordID:         "failure_keep",
		Connector:        "feishu",
		EventSource:      connectorruntime.DefaultFeishuMessageEventKey,
		Reason:           "malformed_source_event",
		Detail:           "missing sender",
		RawExcerpt:       `{"event_id":"evt_bad"}`,
		SourceValidation: connectorruntime.SourceValidationRejected,
		CreatedAt:        time.Date(2026, 6, 24, 12, 2, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("record source failure: %v", err)
	}
	if err := store.RecordSourceFailure(ctx, connectorruntime.SourceFailureRecord{
		RecordID:         "failure_drop",
		Connector:        "email",
		EventSource:      "mail.poll",
		Reason:           "malformed_source_event",
		Detail:           "drop",
		SourceValidation: connectorruntime.SourceValidationRejected,
		CreatedAt:        time.Date(2026, 6, 24, 12, 2, 1, 0, time.UTC),
	}); err != nil {
		t.Fatalf("record drop source failure: %v", err)
	}

	var stdout bytes.Buffer
	if err := run(ctx, []string{
		"inspect",
		"--inbound-state", inboundPath,
		"--outbox-state", outboxPath,
		"--source-state", sourcePath,
		"--connector", "feishu",
	}, &stdout, io.Discard); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	var got InspectionReport
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode report: %v\n%s", err, stdout.String())
	}
	if len(got.SourceFailures) != 1 || got.SourceFailures[0].RecordID != "failure_keep" {
		t.Fatalf("source failures = %+v, want only feishu failure", got.SourceFailures)
	}
	if got.SourceFailures[0].SourceValidation != connectorruntime.SourceValidationRejected {
		t.Fatalf("source validation = %q", got.SourceFailures[0].SourceValidation)
	}
}

func TestConsoleRequeueOutboxMutatesOnlyConnectorState(t *testing.T) {
	ctx := context.Background()
	outboxPath := filepath.Join(testsupport.ProjectTempDir(t, "genesis-console-requeue"), "outbox.json")
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

func TestConsoleResolveOutboxMutatesOnlyConnectorState(t *testing.T) {
	ctx := context.Background()
	outboxPath := filepath.Join(testsupport.ProjectTempDir(t, "genesis-console-resolve"), "outbox.json")
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
	}, time.Date(2026, 6, 24, 12, 10, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("EnqueueCommand returned error: %v", err)
	}
	item.Status = connectorruntime.OutboxStatusRecoveryRequired
	item.AttemptCount = 1
	if err := outboxStore.RecordDelivery(ctx, item, connectorruntime.DeliveryReceipt{
		ReceiptID:         "receipt_1",
		OutboxID:          item.OutboxID,
		Connector:         "feishu",
		Status:            connectorruntime.DeliveryStatusAmbiguous,
		Reason:            "external_result_unknown",
		ExternalActionRef: "om_partial",
		Attempt:           1,
		RecordedAt:        time.Date(2026, 6, 24, 0, 10, 1, 0, time.UTC),
	}); err != nil {
		t.Fatalf("RecordDelivery returned error: %v", err)
	}

	var stdout bytes.Buffer
	if err := run(ctx, []string{
		"resolve-outbox",
		"--outbox-state", outboxPath,
		"--outbox-id", item.OutboxID,
		"--outcome", connectorruntime.DeliveryStatusSent,
		"--reason", "operator_confirmed_sent",
		"--external-action-ref", "om_confirmed",
	}, &stdout, io.Discard); err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	var got RecoveryResult
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode recovery result: %v\n%s", err, stdout.String())
	}
	if got.Item.Status != connectorruntime.OutboxStatusSent {
		t.Fatalf("item status = %q, want sent", got.Item.Status)
	}
	if got.Receipt.Status != connectorruntime.DeliveryStatusSent || got.Receipt.ExternalActionRef != "om_confirmed" {
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
	if len(receipts) != 2 || receipts[0].Status != connectorruntime.DeliveryStatusAmbiguous || receipts[1].Status != connectorruntime.DeliveryStatusSent {
		t.Fatalf("receipt history = %+v", receipts)
	}
}
