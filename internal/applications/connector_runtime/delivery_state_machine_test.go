package connectorruntime

import (
	"context"
	"testing"
	"time"
)

func TestListEligibleOutboxItemsWaitsForNextAttemptAt(t *testing.T) {
	store := newTestOutboxStore(t)
	now := time.Date(2026, 6, 23, 14, 0, 0, 0, time.UTC)
	nextAttempt := now.Add(2 * time.Minute)
	adapter := &fakeAdapter{
		result: ConnectorActionResult{
			Status:        DeliveryStatusRetrying,
			Reason:        "rate_limited",
			NextAttemptAt: nextAttempt,
		},
	}
	runtime := testRuntime(store, map[string]ConnectorAdapter{"feishu": adapter})
	runtime.Now = func() time.Time { return now }
	item, _, err := runtime.EnqueueCommand(context.Background(), testSendMessageCommand())
	if err != nil {
		t.Fatalf("EnqueueCommand returned error: %v", err)
	}
	if _, err := runtime.ExecuteOutboxItem(context.Background(), item.OutboxID); err != nil {
		t.Fatalf("ExecuteOutboxItem returned error: %v", err)
	}

	items, err := runtime.ListEligibleOutboxItems(context.Background())
	if err != nil {
		t.Fatalf("ListEligibleOutboxItems returned error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("eligible items before next_attempt_at = %+v, want none", items)
	}

	runtime.Now = func() time.Time { return nextAttempt }
	items, err = runtime.ListEligibleOutboxItems(context.Background())
	if err != nil {
		t.Fatalf("ListEligibleOutboxItems after next_attempt_at returned error: %v", err)
	}
	if len(items) != 1 || items[0].OutboxID != item.OutboxID {
		t.Fatalf("eligible items after next_attempt_at = %+v, want %q", items, item.OutboxID)
	}
}

func TestExecuteOutboxItemDoesNotRunRetryBeforeNextAttemptAt(t *testing.T) {
	store := newTestOutboxStore(t)
	now := time.Date(2026, 6, 23, 15, 0, 0, 0, time.UTC)
	nextAttempt := now.Add(5 * time.Minute)
	adapter := &fakeAdapter{
		result: ConnectorActionResult{
			Status:        DeliveryStatusRetrying,
			Reason:        "rate_limited",
			NextAttemptAt: nextAttempt,
		},
	}
	runtime := testRuntime(store, map[string]ConnectorAdapter{"feishu": adapter})
	runtime.Now = func() time.Time { return now }
	item, _, err := runtime.EnqueueCommand(context.Background(), testSendMessageCommand())
	if err != nil {
		t.Fatalf("EnqueueCommand returned error: %v", err)
	}
	if _, err := runtime.ExecuteOutboxItem(context.Background(), item.OutboxID); err != nil {
		t.Fatalf("first ExecuteOutboxItem returned error: %v", err)
	}
	if adapter.calls != 1 {
		t.Fatalf("adapter calls after first attempt = %d, want 1", adapter.calls)
	}

	receipt, err := runtime.ExecuteOutboxItem(context.Background(), item.OutboxID)
	if err == nil {
		t.Fatal("second ExecuteOutboxItem should reject premature retry")
	}
	if adapter.calls != 1 {
		t.Fatalf("adapter calls after premature retry = %d, want still 1", adapter.calls)
	}
	if receipt.Status != DeliveryStatusRetrying || receipt.Reason != "retry_not_eligible" {
		t.Fatalf("premature retry receipt = %+v", receipt)
	}
}

func TestClaimNextOutboxItemPreventsConcurrentWorkers(t *testing.T) {
	store := newTestOutboxStore(t)
	now := time.Date(2026, 6, 23, 16, 0, 0, 0, time.UTC)
	runtime := testRuntime(store, nil)
	runtime.Now = func() time.Time { return now }
	item, _, err := runtime.EnqueueCommand(context.Background(), testSendMessageCommand())
	if err != nil {
		t.Fatalf("EnqueueCommand returned error: %v", err)
	}

	first, claimed, err := runtime.ClaimNextOutboxItem(context.Background(), "worker-a", time.Minute)
	if err != nil {
		t.Fatalf("first ClaimNextOutboxItem returned error: %v", err)
	}
	if !claimed || first.OutboxID != item.OutboxID {
		t.Fatalf("first claim = %+v claimed=%v, want outbox %q", first, claimed, item.OutboxID)
	}
	if first.LeaseID == "" || first.LeaseOwner != "worker-a" || !first.LeaseExpiresAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("first lease fields = %+v", first)
	}

	second, claimed, err := runtime.ClaimNextOutboxItem(context.Background(), "worker-b", time.Minute)
	if err != nil {
		t.Fatalf("second ClaimNextOutboxItem returned error: %v", err)
	}
	if claimed {
		t.Fatalf("second worker claimed leased item: %+v", second)
	}
}

func TestExpiredDeliveryLeaseCanBeClaimedByAnotherWorker(t *testing.T) {
	store := newTestOutboxStore(t)
	now := time.Date(2026, 6, 23, 16, 15, 0, 0, time.UTC)
	runtime := testRuntime(store, nil)
	runtime.Now = func() time.Time { return now }
	item, _, err := runtime.EnqueueCommand(context.Background(), testSendMessageCommand())
	if err != nil {
		t.Fatalf("EnqueueCommand returned error: %v", err)
	}
	if _, claimed, err := runtime.ClaimNextOutboxItem(context.Background(), "worker-a", time.Minute); err != nil || !claimed {
		t.Fatalf("first ClaimNextOutboxItem returned claimed=%v err=%v", claimed, err)
	}

	runtime.Now = func() time.Time { return now.Add(2 * time.Minute) }
	second, claimed, err := runtime.ClaimNextOutboxItem(context.Background(), "worker-b", time.Minute)
	if err != nil {
		t.Fatalf("second ClaimNextOutboxItem returned error: %v", err)
	}
	if !claimed || second.OutboxID != item.OutboxID || second.LeaseOwner != "worker-b" {
		t.Fatalf("second claim = %+v claimed=%v, want worker-b claim for %q", second, claimed, item.OutboxID)
	}
}

func TestExecuteOutboxItemDoesNotRunWhenAnotherWorkerLeaseIsActive(t *testing.T) {
	store := newTestOutboxStore(t)
	now := time.Date(2026, 6, 23, 16, 30, 0, 0, time.UTC)
	adapter := &fakeAdapter{
		result: ConnectorActionResult{Status: DeliveryStatusSent, ExternalActionRef: "om_123"},
	}
	runtime := testRuntime(store, map[string]ConnectorAdapter{"feishu": adapter})
	runtime.Now = func() time.Time { return now }
	item, _, err := runtime.EnqueueCommand(context.Background(), testSendMessageCommand())
	if err != nil {
		t.Fatalf("EnqueueCommand returned error: %v", err)
	}
	if _, claimed, err := runtime.ClaimNextOutboxItem(context.Background(), "worker-a", time.Minute); err != nil || !claimed {
		t.Fatalf("ClaimNextOutboxItem returned claimed=%v err=%v", claimed, err)
	}

	receipt, err := runtime.ExecuteOutboxItem(context.Background(), item.OutboxID)
	if err == nil {
		t.Fatal("ExecuteOutboxItem should reject item leased by another worker")
	}
	if adapter.calls != 0 {
		t.Fatalf("adapter calls = %d, want 0", adapter.calls)
	}
	if receipt.Status != DeliveryStatusRetrying || receipt.Reason != "delivery_lease_active" {
		t.Fatalf("active lease receipt = %+v", receipt)
	}
}

func TestExecuteOutboxItemClaimsLeaseBeforeAdapterSideEffect(t *testing.T) {
	store := newTestOutboxStore(t)
	now := time.Date(2026, 6, 23, 16, 45, 0, 0, time.UTC)
	adapter := &leaseObservingAdapter{
		store: store,
		now:   now,
	}
	runtime := testRuntime(store, map[string]ConnectorAdapter{"feishu": adapter})
	runtime.Now = func() time.Time { return now }
	item, _, err := runtime.EnqueueCommand(context.Background(), testSendMessageCommand())
	if err != nil {
		t.Fatalf("EnqueueCommand returned error: %v", err)
	}

	receipt, err := runtime.ExecuteOutboxItem(context.Background(), item.OutboxID)
	if err != nil {
		t.Fatalf("ExecuteOutboxItem returned error: %v", err)
	}
	if receipt.Status != DeliveryStatusSent {
		t.Fatalf("receipt = %+v, want sent", receipt)
	}
	if !adapter.observedActiveLease {
		t.Fatal("adapter ran before the outbox item had an active delivery lease")
	}
	updated, err := store.GetOutboxItem(context.Background(), item.OutboxID)
	if err != nil {
		t.Fatalf("GetOutboxItem returned error: %v", err)
	}
	if updated.LeaseID != "" || !updated.LeaseExpiresAt.IsZero() {
		t.Fatalf("completed delivery should clear lease fields: %+v", updated)
	}
}

func TestExecuteClaimedOutboxItemUsesExistingLease(t *testing.T) {
	store := newTestOutboxStore(t)
	now := time.Date(2026, 6, 23, 16, 50, 0, 0, time.UTC)
	adapter := &fakeAdapter{
		result: ConnectorActionResult{Status: DeliveryStatusSent, ExternalActionRef: "om_123"},
	}
	runtime := testRuntime(store, map[string]ConnectorAdapter{"feishu": adapter})
	runtime.Now = func() time.Time { return now }
	item, _, err := runtime.EnqueueCommand(context.Background(), testSendMessageCommand())
	if err != nil {
		t.Fatalf("EnqueueCommand returned error: %v", err)
	}
	claimed, ok, err := runtime.ClaimNextOutboxItem(context.Background(), "worker-a", time.Minute)
	if err != nil {
		t.Fatalf("ClaimNextOutboxItem returned error: %v", err)
	}
	if !ok || claimed.OutboxID != item.OutboxID || claimed.LeaseID == "" {
		t.Fatalf("claimed item = %+v ok=%v", claimed, ok)
	}

	receipt, err := runtime.ExecuteClaimedOutboxItem(context.Background(), claimed)
	if err != nil {
		t.Fatalf("ExecuteClaimedOutboxItem returned error: %v", err)
	}
	if receipt.Status != DeliveryStatusSent {
		t.Fatalf("receipt = %+v, want sent", receipt)
	}
	if adapter.calls != 1 {
		t.Fatalf("adapter calls = %d, want 1", adapter.calls)
	}
}

func TestRetryableFailureSchedulesNextAttemptAt(t *testing.T) {
	store := newTestOutboxStore(t)
	now := time.Date(2026, 6, 23, 17, 0, 0, 0, time.UTC)
	adapter := &fakeAdapter{
		result: ConnectorActionResult{Status: DeliveryStatusRetrying, Reason: "rate_limited"},
	}
	runtime := testRuntime(store, map[string]ConnectorAdapter{"feishu": adapter})
	runtime.Now = func() time.Time { return now }
	item, _, err := runtime.EnqueueCommand(context.Background(), testSendMessageCommand())
	if err != nil {
		t.Fatalf("EnqueueCommand returned error: %v", err)
	}

	receipt, err := runtime.ExecuteOutboxItem(context.Background(), item.OutboxID)
	if err != nil {
		t.Fatalf("ExecuteOutboxItem returned error: %v", err)
	}
	if receipt.Status != DeliveryStatusRetrying || !receipt.NextAttemptAt.After(now) {
		t.Fatalf("retry receipt = %+v, want retrying with future next_attempt_at", receipt)
	}
	updated, err := store.GetOutboxItem(context.Background(), item.OutboxID)
	if err != nil {
		t.Fatalf("GetOutboxItem returned error: %v", err)
	}
	if updated.Status != OutboxStatusRetrying || !updated.NextAttemptAt.Equal(receipt.NextAttemptAt) {
		t.Fatalf("updated outbox = %+v, receipt = %+v", updated, receipt)
	}
}

func TestRetryableFailureExhaustionDeadLettersOutboxItem(t *testing.T) {
	store := newTestOutboxStore(t)
	now := time.Date(2026, 6, 23, 17, 30, 0, 0, time.UTC)
	adapter := &fakeAdapter{
		result: ConnectorActionResult{Status: DeliveryStatusRetrying, Reason: "rate_limited"},
	}
	runtime := testRuntime(store, map[string]ConnectorAdapter{"feishu": adapter})
	runtime.Now = func() time.Time { return now }
	item, _, err := runtime.EnqueueCommand(context.Background(), testSendMessageCommand())
	if err != nil {
		t.Fatalf("EnqueueCommand returned error: %v", err)
	}

	var receipt DeliveryReceipt
	for attempt := 1; attempt <= 3; attempt++ {
		receipt, err = runtime.ExecuteOutboxItem(context.Background(), item.OutboxID)
		if err != nil {
			t.Fatalf("attempt %d ExecuteOutboxItem returned error: %v", attempt, err)
		}
		if receipt.Status == DeliveryStatusRetrying {
			runtime.Now = func() time.Time { return receipt.NextAttemptAt }
		}
	}
	if receipt.Status != DeliveryStatusDeadLettered {
		t.Fatalf("final receipt = %+v, want dead_lettered", receipt)
	}
	updated, err := store.GetOutboxItem(context.Background(), item.OutboxID)
	if err != nil {
		t.Fatalf("GetOutboxItem returned error: %v", err)
	}
	if updated.Status != OutboxStatusDeadLetter {
		t.Fatalf("updated status = %q, want dead_lettered", updated.Status)
	}
	if adapter.calls != 3 {
		t.Fatalf("adapter calls = %d, want 3", adapter.calls)
	}
}

func TestNonRetryableFailureDeadLettersOutboxItem(t *testing.T) {
	store := newTestOutboxStore(t)
	adapter := &fakeAdapter{
		result: ConnectorActionResult{Status: DeliveryStatusFailed, Reason: "invalid_target"},
	}
	runtime := testRuntime(store, map[string]ConnectorAdapter{"feishu": adapter})
	item, _, err := runtime.EnqueueCommand(context.Background(), testSendMessageCommand())
	if err != nil {
		t.Fatalf("EnqueueCommand returned error: %v", err)
	}

	receipt, err := runtime.ExecuteOutboxItem(context.Background(), item.OutboxID)
	if err != nil {
		t.Fatalf("ExecuteOutboxItem returned error: %v", err)
	}
	if receipt.Status != DeliveryStatusDeadLettered || receipt.Reason != "invalid_target" {
		t.Fatalf("receipt = %+v, want dead_lettered invalid_target", receipt)
	}
	updated, err := store.GetOutboxItem(context.Background(), item.OutboxID)
	if err != nil {
		t.Fatalf("GetOutboxItem returned error: %v", err)
	}
	if updated.Status != OutboxStatusDeadLetter {
		t.Fatalf("updated status = %q, want dead_lettered", updated.Status)
	}
}

func TestExecuteOutboxItemSuppressesDuplicateDeadLetterDelivery(t *testing.T) {
	store := newTestOutboxStore(t)
	adapter := &fakeAdapter{
		result: ConnectorActionResult{Status: DeliveryStatusFailed, Reason: "invalid_target"},
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
	if first.Status != DeliveryStatusDeadLettered {
		t.Fatalf("first receipt = %+v, want dead_lettered", first)
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
}

func TestPartialSuccessRequiresRecoveryAndBlocksBlindRetry(t *testing.T) {
	store := newTestOutboxStore(t)
	adapter := &fakeAdapter{
		result: ConnectorActionResult{
			Status:            DeliveryStatusPartialSuccess,
			Reason:            "receipt_persist_unknown",
			ExternalActionRef: "om_123",
		},
	}
	runtime := testRuntime(store, map[string]ConnectorAdapter{"feishu": adapter})
	item, _, err := runtime.EnqueueCommand(context.Background(), testSendMessageCommand())
	if err != nil {
		t.Fatalf("EnqueueCommand returned error: %v", err)
	}

	receipt, err := runtime.ExecuteOutboxItem(context.Background(), item.OutboxID)
	if err != nil {
		t.Fatalf("ExecuteOutboxItem returned error: %v", err)
	}
	if receipt.Status != DeliveryStatusPartialSuccess {
		t.Fatalf("receipt = %+v, want partial_success", receipt)
	}
	updated, err := store.GetOutboxItem(context.Background(), item.OutboxID)
	if err != nil {
		t.Fatalf("GetOutboxItem returned error: %v", err)
	}
	if updated.Status != OutboxStatusRecoveryRequired {
		t.Fatalf("updated status = %q, want recovery_required", updated.Status)
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
}

func TestOperatorRequeueDeadLetteredItemPreservesReceiptHistory(t *testing.T) {
	store := newTestOutboxStore(t)
	adapter := &fakeAdapter{
		result: ConnectorActionResult{
			Status: DeliveryStatusFailed,
			Reason: "invalid_target",
		},
	}
	runtime := testRuntime(store, map[string]ConnectorAdapter{"feishu": adapter})
	item, _, err := runtime.EnqueueCommand(context.Background(), testSendMessageCommand())
	if err != nil {
		t.Fatalf("EnqueueCommand returned error: %v", err)
	}
	if _, err := runtime.ExecuteOutboxItem(context.Background(), item.OutboxID); err != nil {
		t.Fatalf("ExecuteOutboxItem returned error: %v", err)
	}
	updated, err := store.GetOutboxItem(context.Background(), item.OutboxID)
	if err != nil {
		t.Fatalf("GetOutboxItem returned error: %v", err)
	}
	if updated.Status != OutboxStatusDeadLetter {
		t.Fatalf("updated status = %q, want dead_lettered", updated.Status)
	}

	requeued, receipt, err := runtime.RequeueOutboxItem(context.Background(), item.OutboxID, "operator_requeued")
	if err != nil {
		t.Fatalf("RequeueOutboxItem returned error: %v", err)
	}
	if requeued.Status != OutboxStatusQueued || !requeued.NextAttemptAt.IsZero() || requeued.LeaseID != "" {
		t.Fatalf("requeued item = %+v, want queued with cleared scheduling/lease", requeued)
	}
	if receipt.Status != DeliveryStatusRetrying || receipt.Reason != "operator_requeued" || receipt.Attempt != 1 {
		t.Fatalf("operator receipt = %+v", receipt)
	}
	if adapter.calls != 1 {
		t.Fatalf("operator requeue should not execute adapter, calls = %d", adapter.calls)
	}
	receipts, err := store.ListReceipts(context.Background(), item.OutboxID)
	if err != nil {
		t.Fatalf("ListReceipts returned error: %v", err)
	}
	if len(receipts) != 2 || receipts[0].Status != DeliveryStatusDeadLettered || receipts[1].Status != DeliveryStatusRetrying {
		t.Fatalf("receipts = %+v, want original dead_lettered plus operator recovery receipt", receipts)
	}
}

func TestOperatorRequeueRejectsQueuedRecoveryRequiredAndUnsafeReason(t *testing.T) {
	store := newTestOutboxStore(t)
	runtime := testRuntime(store, nil)
	item, _, err := runtime.EnqueueCommand(context.Background(), testSendMessageCommand())
	if err != nil {
		t.Fatalf("EnqueueCommand returned error: %v", err)
	}
	if _, _, err := runtime.RequeueOutboxItem(context.Background(), item.OutboxID, "operator_requeued"); err == nil {
		t.Fatal("queued outbox item should not be operator-requeued")
	}

	item.Status = OutboxStatusRecoveryRequired
	if err := store.RecordDelivery(context.Background(), item, DeliveryReceipt{
		ReceiptID:  "receipt_partial",
		OutboxID:   item.OutboxID,
		Connector:  item.Connector,
		Status:     DeliveryStatusPartialSuccess,
		Reason:     "receipt_persist_unknown",
		Attempt:    1,
		RecordedAt: time.Date(2026, 6, 23, 18, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("RecordDelivery returned error: %v", err)
	}
	if _, _, err := runtime.RequeueOutboxItem(context.Background(), item.OutboxID, "operator_requeued"); err == nil {
		t.Fatal("recovery_required outbox item should require reconciliation, not requeue")
	}

	item.Status = OutboxStatusDeadLetter
	if err := store.RecordDelivery(context.Background(), item, DeliveryReceipt{
		ReceiptID:  "receipt_deadletter",
		OutboxID:   item.OutboxID,
		Connector:  item.Connector,
		Status:     DeliveryStatusDeadLettered,
		Reason:     "invalid_target",
		Attempt:    1,
		RecordedAt: time.Date(2026, 6, 23, 18, 1, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("RecordDelivery dead-letter returned error: %v", err)
	}
	if _, _, err := runtime.RequeueOutboxItem(context.Background(), item.OutboxID, "Authorization: Bearer sk-secret"); err == nil {
		t.Fatal("operator requeue should reject credential-shaped reason")
	}
}

type leaseObservingAdapter struct {
	store               *FileOutboxStore
	now                 time.Time
	observedActiveLease bool
}

func (a *leaseObservingAdapter) Execute(ctx context.Context, action ConnectorAction) (ConnectorActionResult, error) {
	item, err := a.store.GetOutboxItem(ctx, action.OutboxID)
	if err == nil {
		a.observedActiveLease = deliveryLeaseActive(item, a.now)
	}
	return ConnectorActionResult{Status: DeliveryStatusSent, ExternalActionRef: "om_123"}, nil
}
