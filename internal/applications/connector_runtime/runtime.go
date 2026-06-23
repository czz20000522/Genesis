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
	Store    OutboxStore
	Adapters map[string]ConnectorAdapter
	Now      func() time.Time
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

func (r *Runtime) ExecuteOutboxItem(ctx context.Context, outboxID string) (DeliveryReceipt, error) {
	if r.Store == nil {
		return DeliveryReceipt{}, errors.New("connector runtime missing outbox store")
	}
	item, err := r.Store.GetOutboxItem(ctx, outboxID)
	if err != nil {
		return DeliveryReceipt{}, err
	}
	if item.Status == OutboxStatusSent || item.Status == OutboxStatusFailed || item.Status == OutboxStatusDeadLetter {
		receipt := r.receiptFor(item, ConnectorActionResult{
			Status: DeliveryStatusDuplicateSuppressed,
			Reason: "outbox_terminal_" + item.Status,
		})
		if recordErr := r.Store.RecordDelivery(ctx, item, receipt); recordErr != nil {
			return receipt, recordErr
		}
		return receipt, nil
	}
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
	receipt := r.receiptFor(item, result)
	if recordErr := r.recordResult(ctx, item, receipt); recordErr != nil {
		return receipt, recordErr
	}
	if execErr != nil {
		return receipt, execErr
	}
	return receipt, nil
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
		RecordedAt:        now,
	}
}

func (r *Runtime) recordResult(ctx context.Context, item ConnectorOutboxItem, receipt DeliveryReceipt) error {
	item.AttemptCount = receipt.Attempt
	item.LastReceiptID = receipt.ReceiptID
	item.UpdatedAt = receipt.RecordedAt
	switch receipt.Status {
	case DeliveryStatusSent:
		item.Status = OutboxStatusSent
	case DeliveryStatusRetrying:
		item.Status = OutboxStatusRetrying
	case DeliveryStatusDuplicateSuppressed:
		item.Status = OutboxStatusSent
	default:
		item.Status = OutboxStatusFailed
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
