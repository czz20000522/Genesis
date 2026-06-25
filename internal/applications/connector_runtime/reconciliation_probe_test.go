package connectorruntime

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestReconciliationProbeRecordsReadOnlyEvidenceWithoutResolvingOutbox(t *testing.T) {
	store := newTestOutboxStore(t)
	runtime := testRuntime(store, nil)
	runtime.ReconciliationProbes = map[string]ConnectorReconciliationProbe{
		"feishu": &fakeReconciliationProbe{
			result: ReconciliationProbeResult{
				ObservedStatus:    ReconciliationObservedSent,
				Reason:            "external_confirmed",
				ExternalActionRef: "om_confirmed",
			},
		},
	}
	item := seedRecoveryRequiredOutboxItem(t, store)

	evidence, err := runtime.ProbeRecoveryRequiredOutboxItem(context.Background(), item.OutboxID, ReconciliationLookup{
		Kind:  ReconciliationLookupExternalActionRef,
		Value: "om_partial",
	})
	if err != nil {
		t.Fatalf("ProbeRecoveryRequiredOutboxItem returned error: %v", err)
	}
	if evidence.OutboxID != item.OutboxID || evidence.ObservedStatus != ReconciliationObservedSent || evidence.ExternalActionRef != "om_confirmed" {
		t.Fatalf("evidence = %+v, want sent reconciliation evidence", evidence)
	}
	unchanged, err := store.GetOutboxItem(context.Background(), item.OutboxID)
	if err != nil {
		t.Fatalf("GetOutboxItem returned error: %v", err)
	}
	if unchanged.Status != OutboxStatusRecoveryRequired {
		t.Fatalf("outbox status = %q, want recovery_required after read-only probe", unchanged.Status)
	}
	receipts, err := store.ListReceipts(context.Background(), item.OutboxID)
	if err != nil {
		t.Fatalf("ListReceipts returned error: %v", err)
	}
	if len(receipts) != 1 || receipts[0].Status != DeliveryStatusAmbiguous {
		t.Fatalf("receipts = %+v, want original ambiguous receipt only", receipts)
	}
	evidenceList, err := store.ListReconciliationEvidence(context.Background(), item.OutboxID)
	if err != nil {
		t.Fatalf("ListReconciliationEvidence returned error: %v", err)
	}
	if len(evidenceList) != 1 || evidenceList[0].ProbeID != evidence.ProbeID {
		t.Fatalf("stored evidence = %+v", evidenceList)
	}
}

func TestReconciliationProbeRequiresExactLookupHandle(t *testing.T) {
	store := newTestOutboxStore(t)
	probe := &fakeReconciliationProbe{}
	runtime := testRuntime(store, nil)
	runtime.ReconciliationProbes = map[string]ConnectorReconciliationProbe{"feishu": probe}
	item := seedRecoveryRequiredOutboxItem(t, store)

	if _, err := runtime.ProbeRecoveryRequiredOutboxItem(context.Background(), item.OutboxID, ReconciliationLookup{}); err == nil {
		t.Fatal("ProbeRecoveryRequiredOutboxItem should reject missing exact lookup")
	}
	if probe.calls != 0 {
		t.Fatalf("probe calls = %d, want 0 without exact lookup", probe.calls)
	}
	evidenceList, err := store.ListReconciliationEvidence(context.Background(), item.OutboxID)
	if err != nil {
		t.Fatalf("ListReconciliationEvidence returned error: %v", err)
	}
	if len(evidenceList) != 1 || evidenceList[0].Reason != ReconciliationReasonMissingHandle || evidenceList[0].ObservedStatus != ReconciliationObservedUnavailable {
		t.Fatalf("evidence = %+v, want missing_handle failure evidence", evidenceList)
	}
}

func TestReconciliationProbeRejectsFuzzyLookup(t *testing.T) {
	store := newTestOutboxStore(t)
	probe := &fakeReconciliationProbe{}
	runtime := testRuntime(store, nil)
	runtime.ReconciliationProbes = map[string]ConnectorReconciliationProbe{"feishu": probe}
	item := seedRecoveryRequiredOutboxItem(t, store)

	if _, err := runtime.ProbeRecoveryRequiredOutboxItem(context.Background(), item.OutboxID, ReconciliationLookup{
		Kind:  "message_body",
		Value: "reply text",
	}); err == nil {
		t.Fatal("ProbeRecoveryRequiredOutboxItem should reject fuzzy lookup")
	}
	if probe.calls != 0 {
		t.Fatalf("probe calls = %d, want 0 for fuzzy lookup", probe.calls)
	}
	evidenceList, err := store.ListReconciliationEvidence(context.Background(), item.OutboxID)
	if err != nil {
		t.Fatalf("ListReconciliationEvidence returned error: %v", err)
	}
	if len(evidenceList) != 1 || evidenceList[0].Reason != ReconciliationReasonMissingHandle {
		t.Fatalf("evidence = %+v, want missing_handle failure evidence", evidenceList)
	}
}

func TestReconciliationProbeUnsupportedActionRecordsEvidence(t *testing.T) {
	store := newTestOutboxStore(t)
	runtime := testRuntime(store, nil)
	item := seedRecoveryRequiredOutboxItem(t, store)

	if _, err := runtime.ProbeRecoveryRequiredOutboxItem(context.Background(), item.OutboxID, ReconciliationLookup{
		Kind:  ReconciliationLookupExternalActionRef,
		Value: "om_partial",
	}); err == nil {
		t.Fatal("ProbeRecoveryRequiredOutboxItem should reject unsupported connector/action probe")
	}
	evidenceList, err := store.ListReconciliationEvidence(context.Background(), item.OutboxID)
	if err != nil {
		t.Fatalf("ListReconciliationEvidence returned error: %v", err)
	}
	if len(evidenceList) != 1 || evidenceList[0].Reason != ReconciliationReasonUnsupportedAction || evidenceList[0].ObservedStatus != ReconciliationObservedUnavailable {
		t.Fatalf("evidence = %+v, want unsupported_action evidence", evidenceList)
	}
}

func TestReconciliationProbeAmbiguousDoesNotResolveOutbox(t *testing.T) {
	store := newTestOutboxStore(t)
	runtime := testRuntime(store, nil)
	runtime.ReconciliationProbes = map[string]ConnectorReconciliationProbe{
		"feishu": &fakeReconciliationProbe{result: ReconciliationProbeResult{
			ObservedStatus: ReconciliationObservedAmbiguous,
			Reason:         ReconciliationReasonAmbiguous,
		}},
	}
	item := seedRecoveryRequiredOutboxItem(t, store)

	evidence, err := runtime.ProbeRecoveryRequiredOutboxItem(context.Background(), item.OutboxID, ReconciliationLookup{
		Kind:  ReconciliationLookupExternalActionRef,
		Value: "om_partial",
	})
	if err != nil {
		t.Fatalf("ProbeRecoveryRequiredOutboxItem returned error: %v", err)
	}
	if evidence.ObservedStatus != ReconciliationObservedAmbiguous || evidence.Reason != ReconciliationReasonAmbiguous {
		t.Fatalf("evidence = %+v, want ambiguous evidence", evidence)
	}
	unchanged, err := store.GetOutboxItem(context.Background(), item.OutboxID)
	if err != nil {
		t.Fatalf("GetOutboxItem returned error: %v", err)
	}
	if unchanged.Status != OutboxStatusRecoveryRequired {
		t.Fatalf("outbox status = %q, want recovery_required after ambiguous probe", unchanged.Status)
	}
}

func TestReconciliationProbeExternalUnavailableRecordsEvidence(t *testing.T) {
	store := newTestOutboxStore(t)
	runtime := testRuntime(store, nil)
	runtime.ReconciliationProbes = map[string]ConnectorReconciliationProbe{
		"feishu": &fakeReconciliationProbe{
			result: ReconciliationProbeResult{
				ObservedStatus: ReconciliationObservedUnavailable,
				Reason:         ReconciliationReasonExternalUnavailable,
			},
			err: errors.New("external probe unavailable"),
		},
	}
	item := seedRecoveryRequiredOutboxItem(t, store)

	if _, err := runtime.ProbeRecoveryRequiredOutboxItem(context.Background(), item.OutboxID, ReconciliationLookup{
		Kind:  ReconciliationLookupExternalActionRef,
		Value: "om_partial",
	}); err == nil {
		t.Fatal("ProbeRecoveryRequiredOutboxItem should return external probe error")
	}
	evidenceList, err := store.ListReconciliationEvidence(context.Background(), item.OutboxID)
	if err != nil {
		t.Fatalf("ListReconciliationEvidence returned error: %v", err)
	}
	if len(evidenceList) != 1 || evidenceList[0].ObservedStatus != ReconciliationObservedUnavailable || evidenceList[0].Reason != ReconciliationReasonExternalUnavailable {
		t.Fatalf("evidence = %+v, want external_unavailable evidence", evidenceList)
	}
}

func seedRecoveryRequiredOutboxItem(t *testing.T, store *FileOutboxStore) ConnectorOutboxItem {
	t.Helper()
	item, _, err := store.EnqueueCommand(context.Background(), testSendMessageCommand(), time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("EnqueueCommand returned error: %v", err)
	}
	item.Status = OutboxStatusRecoveryRequired
	item.AttemptCount = 1
	if err := store.RecordDelivery(context.Background(), item, DeliveryReceipt{
		ReceiptID:         "receipt_ambiguous",
		OutboxID:          item.OutboxID,
		Connector:         item.Connector,
		Status:            DeliveryStatusAmbiguous,
		Reason:            "external_result_unknown",
		ExternalActionRef: "om_partial",
		Attempt:           1,
		RecordedAt:        time.Date(2026, 6, 25, 10, 0, 1, 0, time.UTC),
	}); err != nil {
		t.Fatalf("RecordDelivery returned error: %v", err)
	}
	return item
}

type fakeReconciliationProbe struct {
	result ReconciliationProbeResult
	err    error
	calls  int
}

func (p *fakeReconciliationProbe) Probe(ctx context.Context, item ConnectorOutboxItem, lookup ReconciliationLookup) (ReconciliationProbeResult, error) {
	p.calls++
	return p.result, p.err
}
