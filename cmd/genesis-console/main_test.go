package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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
	if len(got.OutboxSummary) != 1 {
		t.Fatalf("outbox summary = %+v", got.OutboxSummary)
	}
	summary := got.OutboxSummary[0]
	if summary.OutboxID != keepItem.OutboxID || summary.ReceiptCount != 1 {
		t.Fatalf("outbox summary identity = %+v", summary)
	}
	if summary.LastReceiptStatus != connectorruntime.DeliveryStatusDeadLettered || summary.RecommendedAction != OperatorActionReviewDeadLetter {
		t.Fatalf("outbox summary recovery fields = %+v", summary)
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
		RecordID:          "failure_keep",
		Connector:         "feishu",
		EventSource:       "feishu.message.receive",
		Reason:            "malformed_source_event",
		Detail:            "missing sender",
		DiagnosticExcerpt: "missing sender; source_bytes=42",
		SourceValidation:  connectorruntime.SourceValidationRejected,
		CreatedAt:         time.Date(2026, 6, 24, 12, 2, 0, 0, time.UTC),
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

func TestConsoleInspectIncludesSourceLifecycleState(t *testing.T) {
	ctx := context.Background()
	dir := testsupport.ProjectTempDir(t, "genesis-console-source-lifecycle")
	inboundPath := filepath.Join(dir, "inbound.json")
	outboxPath := filepath.Join(dir, "outbox.json")
	sourceFailurePath := filepath.Join(dir, "source-failures.json")
	sourceLifecyclePath := filepath.Join(dir, "source-lifecycle.json")
	store, err := connectorruntime.NewFileSourceLifecycleStore(sourceLifecyclePath)
	if err != nil {
		t.Fatalf("NewFileSourceLifecycleStore returned error: %v", err)
	}
	now := time.Date(2026, 6, 24, 16, 0, 0, 0, time.UTC)
	sourceRun := connectorruntime.SourceRun{
		SourceID:    "source_feishu_events",
		Connector:   "feishu",
		AdapterRef:  "feishu-source-adapter",
		Status:      connectorruntime.SourceRunStatusReady,
		StartedAt:   now,
		LastReadyAt: now.Add(time.Second),
		UpdatedAt:   now.Add(time.Second),
	}
	if err := store.UpsertSourceRun(ctx, sourceRun); err != nil {
		t.Fatalf("UpsertSourceRun returned error: %v", err)
	}
	if err := store.RecordSourceAttempt(ctx, connectorruntime.SourceAttempt{
		AttemptID:   "attempt_ready",
		SourceRunID: sourceRun.SourceID,
		StartedAt:   now,
		EndedAt:     now.Add(time.Second),
		Outcome:     connectorruntime.SourceAttemptOutcomeReady,
	}); err != nil {
		t.Fatalf("RecordSourceAttempt returned error: %v", err)
	}
	if err := store.SaveSourceCursor(ctx, connectorruntime.SourceCursor{
		SourceID:    sourceRun.SourceID,
		CursorKind:  connectorruntime.SourceCursorKindExternalEventID,
		CursorValue: "evt_123",
		WatermarkAt: now.Add(2 * time.Second),
		UpdatedAt:   now.Add(3 * time.Second),
	}); err != nil {
		t.Fatalf("SaveSourceCursor returned error: %v", err)
	}
	if err := store.RecordSourceVerification(ctx, connectorruntime.SourceVerificationEvidence{
		SourceEventRef:   "evt_123",
		SourceID:         sourceRun.SourceID,
		Connector:        "feishu",
		ValidationStatus: connectorruntime.SourceValidationVerified,
		EvidenceKind:     connectorruntime.SourceEvidenceKindTrustedLocalAdapterAttestation,
		EvidenceRef:      "evidence_123",
		CheckedAt:        now.Add(4 * time.Second),
		AdapterRef:       "feishu-source-adapter",
	}); err != nil {
		t.Fatalf("RecordSourceVerification returned error: %v", err)
	}

	var stdout bytes.Buffer
	if err := run(ctx, []string{
		"inspect",
		"--inbound-state", inboundPath,
		"--outbox-state", outboxPath,
		"--source-state", sourceFailurePath,
		"--source-lifecycle-state", sourceLifecyclePath,
		"--connector", "feishu",
	}, &stdout, io.Discard); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	var got InspectionReport
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode report: %v\n%s", err, stdout.String())
	}
	if len(got.SourceRuns) != 1 || got.SourceRuns[0].SourceID != sourceRun.SourceID || got.SourceRuns[0].Status != connectorruntime.SourceRunStatusReady {
		t.Fatalf("source runs = %+v, want ready Feishu source run", got.SourceRuns)
	}
	if len(got.SourceAttempts[sourceRun.SourceID]) != 1 || got.SourceAttempts[sourceRun.SourceID][0].Outcome != connectorruntime.SourceAttemptOutcomeReady {
		t.Fatalf("source attempts = %+v, want ready attempt", got.SourceAttempts)
	}
	if len(got.SourceCursors) != 1 || got.SourceCursors[0].CursorValue != "evt_123" {
		t.Fatalf("source cursors = %+v, want connector-local cursor", got.SourceCursors)
	}
	if len(got.SourceEvidence) != 1 || got.SourceEvidence[0].EvidenceRef != "evidence_123" {
		t.Fatalf("source verification evidence = %+v, want inspectable source evidence", got.SourceEvidence)
	}
}

func TestConsoleSourceLifecycleControlsMutateOnlyConnectorState(t *testing.T) {
	ctx := context.Background()
	dir := testsupport.ProjectTempDir(t, "genesis-console-source-controls")
	inboundPath := filepath.Join(dir, "inbound.json")
	outboxPath := filepath.Join(dir, "outbox.json")
	sourceFailurePath := filepath.Join(dir, "source-failures.json")
	sourceLifecyclePath := filepath.Join(dir, "source-lifecycle.json")
	store, err := connectorruntime.NewFileSourceLifecycleStore(sourceLifecyclePath)
	if err != nil {
		t.Fatalf("NewFileSourceLifecycleStore returned error: %v", err)
	}
	startedAt := time.Date(2026, 6, 24, 17, 0, 0, 0, time.UTC)
	if err := store.UpsertSourceRun(ctx, connectorruntime.SourceRun{
		SourceID:          "source_feishu_events",
		Connector:         "feishu",
		AdapterRef:        "feishu-source-adapter",
		Status:            connectorruntime.SourceRunStatusBlocked,
		StartedAt:         startedAt,
		BlockedReasonCode: connectorruntime.SourceReadinessReasonMissingProfile,
		BlockedReason:     "profile missing",
		UpdatedAt:         startedAt,
	}); err != nil {
		t.Fatalf("UpsertSourceRun returned error: %v", err)
	}
	if err := store.SaveSourceCursor(ctx, connectorruntime.SourceCursor{
		SourceID:    "source_feishu_events",
		CursorKind:  connectorruntime.SourceCursorKindExternalEventID,
		CursorValue: "evt_latest",
		UpdatedAt:   startedAt,
	}); err != nil {
		t.Fatalf("SaveSourceCursor returned error: %v", err)
	}

	var clearStdout bytes.Buffer
	if err := run(ctx, []string{
		"source-clear-blocked",
		"--source-lifecycle-state", sourceLifecyclePath,
		"--source-id", "source_feishu_events",
		"--reason", "operator_profile_fixed",
	}, &clearStdout, io.Discard); err != nil {
		t.Fatalf("source-clear-blocked returned error: %v", err)
	}
	var clearResult SourceLifecycleControlResult
	if err := json.Unmarshal(clearStdout.Bytes(), &clearResult); err != nil {
		t.Fatalf("decode clear result: %v\n%s", err, clearStdout.String())
	}
	if clearResult.Run == nil || clearResult.Run.Status != connectorruntime.SourceRunStatusStopped || clearResult.OperatorAction.Action != connectorruntime.SourceOperatorActionClearBlocked {
		t.Fatalf("clear result = %+v", clearResult)
	}

	var restartStdout bytes.Buffer
	if err := run(ctx, []string{
		"source-request-restart",
		"--source-lifecycle-state", sourceLifecyclePath,
		"--source-id", "source_feishu_events",
		"--reason", "operator_requested_restart",
	}, &restartStdout, io.Discard); err != nil {
		t.Fatalf("source-request-restart returned error: %v", err)
	}
	var restartResult SourceLifecycleControlResult
	if err := json.Unmarshal(restartStdout.Bytes(), &restartResult); err != nil {
		t.Fatalf("decode restart result: %v\n%s", err, restartStdout.String())
	}
	if restartResult.OperatorAction.Action != connectorruntime.SourceOperatorActionRequestRestart {
		t.Fatalf("restart result = %+v", restartResult)
	}

	if err := run(ctx, []string{
		"source-reset-cursor",
		"--source-lifecycle-state", sourceLifecyclePath,
		"--source-id", "source_feishu_events",
		"--cursor-value", "evt_replay_from",
		"--reason", "operator_replay_cursor",
	}, io.Discard, io.Discard); err == nil {
		t.Fatal("source-reset-cursor should reject missing duplicate-risk confirmation")
	}

	var resetStdout bytes.Buffer
	if err := run(ctx, []string{
		"source-reset-cursor",
		"--source-lifecycle-state", sourceLifecyclePath,
		"--source-id", "source_feishu_events",
		"--cursor-value", "evt_replay_from",
		"--reason", "operator_replay_cursor",
		"--accept-duplicate-risk",
	}, &resetStdout, io.Discard); err != nil {
		t.Fatalf("source-reset-cursor returned error: %v", err)
	}
	var resetResult SourceLifecycleControlResult
	if err := json.Unmarshal(resetStdout.Bytes(), &resetResult); err != nil {
		t.Fatalf("decode reset result: %v\n%s", err, resetStdout.String())
	}
	if resetResult.Cursor == nil || resetResult.Cursor.CursorValue != "evt_replay_from" || resetResult.OperatorAction.Action != connectorruntime.SourceOperatorActionResetCursor {
		t.Fatalf("reset result = %+v", resetResult)
	}

	var inspectStdout bytes.Buffer
	if err := run(ctx, []string{
		"inspect",
		"--inbound-state", inboundPath,
		"--outbox-state", outboxPath,
		"--source-state", sourceFailurePath,
		"--source-lifecycle-state", sourceLifecyclePath,
		"--connector", "feishu",
	}, &inspectStdout, io.Discard); err != nil {
		t.Fatalf("inspect returned error: %v", err)
	}
	var report InspectionReport
	if err := json.Unmarshal(inspectStdout.Bytes(), &report); err != nil {
		t.Fatalf("decode inspect report: %v\n%s", err, inspectStdout.String())
	}
	actions := report.SourceActions["source_feishu_events"]
	if len(actions) != 3 {
		t.Fatalf("source operator action count = %d, want 3: %+v", len(actions), actions)
	}
	if len(report.SourceRuns) != 1 || report.SourceRuns[0].Status != connectorruntime.SourceRunStatusStopped {
		t.Fatalf("source runs = %+v, want stopped source after clear", report.SourceRuns)
	}
	if len(report.SourceCursors) != 1 || report.SourceCursors[0].CursorValue != "evt_replay_from" {
		t.Fatalf("source cursors = %+v, want reset cursor", report.SourceCursors)
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

func TestConsoleProbeOutboxRecordsEvidenceWithoutResolving(t *testing.T) {
	ctx := context.Background()
	outboxPath := filepath.Join(testsupport.ProjectTempDir(t, "genesis-console-probe"), "outbox.json")
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
	}, time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC))
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
		RecordedAt:        time.Date(2026, 6, 25, 10, 0, 1, 0, time.UTC),
	}); err != nil {
		t.Fatalf("RecordDelivery returned error: %v", err)
	}

	var stdout bytes.Buffer
	if err := run(ctx, []string{
		"probe-outbox",
		"--outbox-state", outboxPath,
		"--outbox-id", item.OutboxID,
		"--lookup-kind", connectorruntime.ReconciliationLookupExternalActionRef,
		"--lookup-value", "om_partial",
		"--probe-command", os.Args[0],
		"--probe-command-arg", "-test.run=TestReconciliationProbeCommandHelper",
		"--probe-command-arg", "--",
		"--probe-command-arg", connectorruntime.ReconciliationObservedSent,
		"--probe-command-arg", "external_confirmed",
		"--probe-command-arg", "om_confirmed",
	}, &stdout, io.Discard); err != nil {
		t.Fatalf("probe-outbox returned error: %v\n%s", err, stdout.String())
	}
	var evidence connectorruntime.ReconciliationEvidence
	if err := json.Unmarshal(stdout.Bytes(), &evidence); err != nil {
		t.Fatalf("decode evidence: %v\n%s", err, stdout.String())
	}
	if evidence.ObservedStatus != connectorruntime.ReconciliationObservedSent || evidence.ExternalActionRef != "om_confirmed" {
		t.Fatalf("evidence = %+v, want sent confirmation", evidence)
	}

	reloaded, err := connectorruntime.NewFileOutboxStore(outboxPath)
	if err != nil {
		t.Fatalf("reload outbox store: %v", err)
	}
	unchanged, err := reloaded.GetOutboxItem(ctx, item.OutboxID)
	if err != nil {
		t.Fatalf("GetOutboxItem returned error: %v", err)
	}
	if unchanged.Status != connectorruntime.OutboxStatusRecoveryRequired {
		t.Fatalf("outbox status = %q, want recovery_required after probe", unchanged.Status)
	}
	receipts, err := reloaded.ListReceipts(ctx, item.OutboxID)
	if err != nil {
		t.Fatalf("ListReceipts returned error: %v", err)
	}
	if len(receipts) != 1 || receipts[0].Status != connectorruntime.DeliveryStatusAmbiguous {
		t.Fatalf("receipts = %+v, want original ambiguous receipt only", receipts)
	}

	var inspectStdout bytes.Buffer
	if err := run(ctx, []string{
		"inspect",
		"--outbox-state", outboxPath,
		"--outbox-status", connectorruntime.OutboxStatusRecoveryRequired,
	}, &inspectStdout, io.Discard); err != nil {
		t.Fatalf("inspect returned error: %v", err)
	}
	var report InspectionReport
	if err := json.Unmarshal(inspectStdout.Bytes(), &report); err != nil {
		t.Fatalf("decode inspect report: %v\n%s", inspectStdout.String(), err)
	}
	if len(report.ReconciliationEvidence[item.OutboxID]) != 1 || report.ReconciliationEvidence[item.OutboxID][0].ExternalActionRef != "om_confirmed" {
		t.Fatalf("reconciliation evidence projection = %+v", report.ReconciliationEvidence)
	}
}

func TestConsoleProbeOutboxRequiresExactLookup(t *testing.T) {
	ctx := context.Background()
	outboxPath := filepath.Join(testsupport.ProjectTempDir(t, "genesis-console-probe-missing-handle"), "outbox.json")
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
	}, time.Date(2026, 6, 25, 11, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("EnqueueCommand returned error: %v", err)
	}
	item.Status = connectorruntime.OutboxStatusRecoveryRequired
	if err := outboxStore.RecordDelivery(ctx, item, connectorruntime.DeliveryReceipt{
		ReceiptID:  "receipt_1",
		OutboxID:   item.OutboxID,
		Connector:  "feishu",
		Status:     connectorruntime.DeliveryStatusAmbiguous,
		Reason:     "external_result_unknown",
		Attempt:    1,
		RecordedAt: time.Date(2026, 6, 25, 11, 0, 1, 0, time.UTC),
	}); err != nil {
		t.Fatalf("RecordDelivery returned error: %v", err)
	}

	if err := run(ctx, []string{
		"probe-outbox",
		"--outbox-state", outboxPath,
		"--outbox-id", item.OutboxID,
		"--lookup-kind", "message_body",
		"--lookup-value", "reply",
		"--probe-command", os.Args[0],
		"--probe-command-arg", "-test.run=TestReconciliationProbeCommandHelper",
	}, io.Discard, io.Discard); err == nil {
		t.Fatal("probe-outbox should reject fuzzy lookup")
	}
	reloaded, err := connectorruntime.NewFileOutboxStore(outboxPath)
	if err != nil {
		t.Fatalf("reload outbox store: %v", err)
	}
	evidence, err := reloaded.ListReconciliationEvidence(ctx, item.OutboxID)
	if err != nil {
		t.Fatalf("ListReconciliationEvidence returned error: %v", err)
	}
	if len(evidence) != 1 || evidence[0].Reason != connectorruntime.ReconciliationReasonMissingHandle {
		t.Fatalf("evidence = %+v, want missing_handle failure evidence", evidence)
	}
}

func TestReconciliationProbeCommandHelper(t *testing.T) {
	mode := reconciliationProbeHelperMode()
	if mode == "" {
		return
	}
	status, reason, externalRef := reconciliationProbeHelperArgs()
	var request connectorruntime.ReconciliationProbeRequest
	if err := json.Unmarshal([]byte(os.Args[len(os.Args)-1]), &request); err != nil {
		t.Fatalf("decode probe request: %v", err)
	}
	if request.Lookup.Kind == "" || request.Lookup.Value == "" {
		t.Fatalf("probe request missing exact lookup: %+v", request)
	}
	if err := json.NewEncoder(os.Stdout).Encode(connectorruntime.ReconciliationProbeResult{
		ObservedStatus:    status,
		Reason:            reason,
		ExternalActionRef: externalRef,
	}); err != nil {
		t.Fatalf("encode probe result: %v", err)
	}
	os.Exit(0)
}

func reconciliationProbeHelperMode() string {
	for _, arg := range os.Args {
		if arg == "--" {
			return "probe"
		}
	}
	return ""
}

func reconciliationProbeHelperArgs() (string, string, string) {
	for i, arg := range os.Args {
		if arg != "--" {
			continue
		}
		status := connectorruntime.ReconciliationObservedUnknown
		reason := ""
		externalRef := ""
		if i+1 < len(os.Args) {
			status = os.Args[i+1]
		}
		if i+2 < len(os.Args) {
			reason = os.Args[i+2]
		}
		if i+3 < len(os.Args) {
			externalRef = os.Args[i+3]
		}
		return status, reason, externalRef
	}
	return connectorruntime.ReconciliationObservedUnknown, "", ""
}
