package connectorruntime

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type ConnectorAdapter interface {
	Execute(context.Context, ConnectorAction) (ConnectorActionResult, error)
}

type Runtime struct {
	Store                OutboxStore
	InboundStore         InboundStore
	Client               TurnClient
	SessionMapper        ApplicationSessionMapper
	Adapters             map[string]ConnectorAdapter
	ReconciliationProbes map[string]ConnectorReconciliationProbe
	Now                  func() time.Time
}

func (r *Runtime) EnqueueCommand(ctx context.Context, command AppCommand) (ConnectorOutboxItem, bool, error) {
	if r.Store == nil {
		return ConnectorOutboxItem{}, false, errors.New("connector runtime missing outbox store")
	}
	now := time.Now()
	if r.Now != nil {
		now = r.Now()
	}
	return r.Store.EnqueueCommand(ctx, command, now)
}

func (r *Runtime) ListEligibleOutboxItems(ctx context.Context) ([]ConnectorOutboxItem, error) {
	if r.Store == nil {
		return nil, errors.New("connector runtime missing outbox store")
	}
	now := time.Now()
	if r.Now != nil {
		now = r.Now()
	}
	items, err := r.Store.ListOutbox(ctx)
	if err != nil {
		return nil, err
	}
	eligible := make([]ConnectorOutboxItem, 0, len(items))
	for _, item := range items {
		if deliveryEligible(item, now) {
			eligible = append(eligible, item)
		}
	}
	return eligible, nil
}

func (r *Runtime) ClaimNextOutboxItem(ctx context.Context, owner string, leaseDuration time.Duration) (ConnectorOutboxItem, bool, error) {
	if r.Store == nil {
		return ConnectorOutboxItem{}, false, errors.New("connector runtime missing outbox store")
	}
	now := time.Now()
	if r.Now != nil {
		now = r.Now()
	}
	return r.Store.ClaimNextOutboxItem(ctx, now, owner, leaseDuration)
}

func (r *Runtime) ExecuteOutboxItem(ctx context.Context, outboxID string) (DeliveryReceipt, error) {
	if r.Store == nil {
		return DeliveryReceipt{}, errors.New("connector runtime missing outbox store")
	}
	item, err := r.Store.GetOutboxItem(ctx, outboxID)
	if err != nil {
		return DeliveryReceipt{}, err
	}
	if item.Status == OutboxStatusSent || item.Status == OutboxStatusDeadLetter || item.Status == OutboxStatusRecoveryRequired {
		receipt := r.receiptFor(item, ConnectorActionResult{
			Status: DeliveryStatusDuplicateSuppressed,
			Reason: "outbox_terminal_" + item.Status,
		})
		if recordErr := r.Store.RecordDelivery(ctx, item, receipt); recordErr != nil {
			return receipt, recordErr
		}
		return receipt, nil
	}
	now := time.Now()
	if r.Now != nil {
		now = r.Now()
	}
	if item.Status == OutboxStatusRetrying && item.NextAttemptAt.After(now) {
		receipt := r.nonAttemptReceiptFor(item, ConnectorActionResult{
			Status:        DeliveryStatusRetrying,
			Reason:        "retry_not_eligible",
			NextAttemptAt: item.NextAttemptAt,
		})
		if recordErr := r.Store.RecordDelivery(ctx, item, receipt); recordErr != nil {
			return receipt, recordErr
		}
		return receipt, errors.New("connector retry is not eligible yet")
	}
	if deliveryLeaseActive(item, now) {
		receipt := r.nonAttemptReceiptFor(item, ConnectorActionResult{
			Status:        DeliveryStatusRetrying,
			Reason:        "delivery_lease_active",
			NextAttemptAt: item.LeaseExpiresAt,
		})
		if recordErr := r.Store.RecordDelivery(ctx, item, receipt); recordErr != nil {
			return receipt, recordErr
		}
		return receipt, errors.New("connector delivery lease is active")
	}
	item, claimed, err := r.Store.ClaimOutboxItem(ctx, outboxID, now, defaultDeliveryLeaseOwner, defaultDeliveryLeaseTTL)
	if err != nil {
		return DeliveryReceipt{}, err
	}
	if !claimed {
		receipt := r.nonAttemptReceiptFor(item, ConnectorActionResult{
			Status: DeliveryStatusRetrying,
			Reason: "delivery_not_claimed",
		})
		if recordErr := r.Store.RecordDelivery(ctx, item, receipt); recordErr != nil {
			return receipt, recordErr
		}
		return receipt, errors.New("connector delivery was not claimed")
	}
	return r.executeLeasedOutboxItem(ctx, item, now)
}

func (r *Runtime) ExecuteClaimedOutboxItem(ctx context.Context, claimed ConnectorOutboxItem) (DeliveryReceipt, error) {
	if r.Store == nil {
		return DeliveryReceipt{}, errors.New("connector runtime missing outbox store")
	}
	if claimed.OutboxID == "" {
		return DeliveryReceipt{}, errors.New("outbox id is required")
	}
	item, err := r.Store.GetOutboxItem(ctx, claimed.OutboxID)
	if err != nil {
		return DeliveryReceipt{}, err
	}
	now := time.Now()
	if r.Now != nil {
		now = r.Now()
	}
	if claimed.LeaseID == "" || item.LeaseID != claimed.LeaseID || !deliveryLeaseActive(item, now) {
		receipt := r.nonAttemptReceiptFor(item, ConnectorActionResult{
			Status: DeliveryStatusRetrying,
			Reason: "delivery_lease_invalid",
		})
		if recordErr := r.Store.RecordDelivery(ctx, item, receipt); recordErr != nil {
			return receipt, recordErr
		}
		return receipt, errors.New("connector delivery lease is invalid")
	}
	return r.executeLeasedOutboxItem(ctx, item, now)
}

func (r *Runtime) RequeueOutboxItem(ctx context.Context, outboxID string, reason string) (ConnectorOutboxItem, DeliveryReceipt, error) {
	if r.Store == nil {
		return ConnectorOutboxItem{}, DeliveryReceipt{}, errors.New("connector runtime missing outbox store")
	}
	now := time.Now()
	if r.Now != nil {
		now = r.Now()
	}
	return r.Store.RequeueOutboxItem(ctx, outboxID, reason, now)
}

func (r *Runtime) ResolveRecoveryRequiredOutboxItem(ctx context.Context, outboxID string, outcome string, reason string, externalActionRef string) (ConnectorOutboxItem, DeliveryReceipt, error) {
	if r.Store == nil {
		return ConnectorOutboxItem{}, DeliveryReceipt{}, errors.New("connector runtime missing outbox store")
	}
	now := time.Now()
	if r.Now != nil {
		now = r.Now()
	}
	return r.Store.ResolveRecoveryRequiredOutboxItem(ctx, outboxID, outcome, reason, externalActionRef, now)
}

func (r *Runtime) executeLeasedOutboxItem(ctx context.Context, item ConnectorOutboxItem, now time.Time) (DeliveryReceipt, error) {
	adapter := r.Adapters[item.Connector]
	if adapter == nil {
		receipt := r.receiptFor(item, ConnectorActionResult{Status: DeliveryStatusFailed, Reason: "connector_adapter_missing"})
		if recordErr := r.recordResult(ctx, item, receipt); recordErr != nil {
			return receipt, recordErr
		}
		return receipt, errors.New("connector adapter missing")
	}
	action := ConnectorAction{
		OutboxID:       item.OutboxID,
		Connector:      item.Connector,
		ActionKind:     item.ActionKind,
		TargetRef:      item.TargetRef,
		Payload:        copyStringMap(item.Payload),
		IdempotencyKey: item.IdempotencyKey,
		Attempt:        item.AttemptCount + 1,
	}
	result, execErr := adapter.Execute(ctx, action)
	if result.Status == "" {
		if execErr != nil {
			result.Status = DeliveryStatusFailed
			result.Reason = execErr.Error()
		} else {
			result.Status = DeliveryStatusSent
		}
	}
	result = normalizeDeliveryResult(item, result, now)
	receipt := r.receiptFor(item, result)
	if recordErr := r.recordResult(ctx, item, receipt); recordErr != nil {
		return receipt, recordErr
	}
	if execErr != nil {
		return receipt, execErr
	}
	return receipt, nil
}

func (r *Runtime) nonAttemptReceiptFor(item ConnectorOutboxItem, result ConnectorActionResult) DeliveryReceipt {
	now := time.Now()
	if r.Now != nil {
		now = r.Now()
	}
	return DeliveryReceipt{
		ReceiptID:         stableOpaqueID("receipt", item.OutboxID, result.Status, result.ExternalActionRef, result.Reason, fmt.Sprint(item.AttemptCount), now.Format(time.RFC3339Nano)),
		OutboxID:          item.OutboxID,
		Connector:         item.Connector,
		ExternalActionRef: result.ExternalActionRef,
		Status:            result.Status,
		Reason:            result.Reason,
		Attempt:           item.AttemptCount,
		NextAttemptAt:     result.NextAttemptAt,
		RecordedAt:        now,
	}
}

func (r *Runtime) receiptFor(item ConnectorOutboxItem, result ConnectorActionResult) DeliveryReceipt {
	now := time.Now()
	if r.Now != nil {
		now = r.Now()
	}
	return DeliveryReceipt{
		ReceiptID:         stableOpaqueID("receipt", item.OutboxID, result.Status, result.ExternalActionRef, result.Reason, fmt.Sprint(item.AttemptCount+1), now.Format(time.RFC3339Nano)),
		OutboxID:          item.OutboxID,
		Connector:         item.Connector,
		ExternalActionRef: result.ExternalActionRef,
		Status:            result.Status,
		Reason:            result.Reason,
		Attempt:           item.AttemptCount + 1,
		NextAttemptAt:     result.NextAttemptAt,
		RecordedAt:        now,
	}
}

func (r *Runtime) recordResult(ctx context.Context, item ConnectorOutboxItem, receipt DeliveryReceipt) error {
	item.AttemptCount = receipt.Attempt
	item.LastReceiptID = receipt.ReceiptID
	item.LeaseID = ""
	item.LeaseOwner = ""
	item.LeaseExpiresAt = time.Time{}
	item.UpdatedAt = receipt.RecordedAt
	switch receipt.Status {
	case DeliveryStatusSent:
		item.Status = OutboxStatusSent
	case DeliveryStatusRetrying:
		item.Status = OutboxStatusRetrying
		item.NextAttemptAt = receipt.NextAttemptAt
	case DeliveryStatusDeadLettered:
		item.Status = OutboxStatusDeadLetter
		item.NextAttemptAt = time.Time{}
	case DeliveryStatusPartialSuccess, DeliveryStatusAmbiguous:
		item.Status = OutboxStatusRecoveryRequired
		item.NextAttemptAt = time.Time{}
	case DeliveryStatusDuplicateSuppressed:
		item.Status = OutboxStatusSent
	default:
		item.Status = OutboxStatusDeadLetter
		item.NextAttemptAt = time.Time{}
	}
	return r.Store.RecordDelivery(ctx, item, receipt)
}

func copyStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
