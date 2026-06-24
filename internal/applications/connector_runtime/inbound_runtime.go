package connectorruntime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type ApplicationSessionMapper interface {
	Map(ExternalEvent) (ApplicationSessionMapping, error)
}

type DefaultApplicationSessionMapper struct{}

func (DefaultApplicationSessionMapper) Map(event ExternalEvent) (ApplicationSessionMapping, error) {
	if err := event.Validate(); err != nil {
		return ApplicationSessionMapping{}, err
	}
	appSessionID := stableOpaqueID("appsession",
		strings.TrimSpace(event.Connector),
		strings.TrimSpace(event.ThreadRef.Connector),
		strings.TrimSpace(event.ThreadRef.Kind),
		strings.TrimSpace(event.ThreadRef.ExternalID),
	)
	return ApplicationSessionMapping{
		ApplicationSessionID: appSessionID,
		KernelSessionID: stableOpaqueID("session",
			strings.TrimSpace(event.Connector),
			strings.TrimSpace(event.ThreadRef.Connector),
			strings.TrimSpace(event.ThreadRef.Kind),
			strings.TrimSpace(event.ThreadRef.ExternalID),
		),
	}, nil
}

func (r *Runtime) ProcessExternalEvent(ctx context.Context, event ExternalEvent) (ProcessResult, error) {
	event = uncheckedDirectExternalEvent(event)
	return r.processExternalEvent(ctx, event)
}

// ProcessSourceCommandEvent is only for events emitted by SourceCommandIntake
// after source frame and verification evidence validation.
func (r *Runtime) ProcessSourceCommandEvent(ctx context.Context, event ExternalEvent) (ProcessResult, error) {
	return r.processExternalEvent(ctx, event)
}

func (r *Runtime) processExternalEvent(ctx context.Context, event ExternalEvent) (ProcessResult, error) {
	if err := event.Validate(); err != nil {
		return ProcessResult{}, err
	}
	if r.InboundStore == nil {
		return ProcessResult{}, errors.New("connector runtime missing inbound store")
	}
	if r.Client == nil {
		return ProcessResult{}, errors.New("connector runtime missing kernel turn client")
	}
	mapper := r.SessionMapper
	if mapper == nil {
		mapper = DefaultApplicationSessionMapper{}
	}
	mapping, err := mapper.Map(event)
	if err != nil {
		return ProcessResult{}, err
	}
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}
	currentTime := now()
	requestContext := event.RequestContext(mapping)
	record := InboundSubmissionRecord{
		RequestID:            requestContext.RequestID,
		DedupeKey:            requestContext.DedupeKey,
		KernelIdempotencyKey: requestContext.KernelIdempotencyKey,
		Connector:            requestContext.Connector,
		EventType:            requestContext.EventType,
		ApplicationSessionID: requestContext.ApplicationSessionID,
		KernelSessionID:      requestContext.KernelSessionID,
		Status:               SubmissionStatusPending,
		CreatedAt:            currentTime,
		UpdatedAt:            currentTime,
	}
	record, reserved, err := r.InboundStore.Reserve(ctx, record)
	if err != nil {
		return ProcessResult{}, err
	}
	if !reserved {
		return ProcessResult{Record: record, Duplicate: true}, nil
	}

	req := TurnSubmitRequest{
		SessionID:      requestContext.KernelSessionID,
		IdempotencyKey: requestContext.KernelIdempotencyKey,
		InputItems: []TurnInputItem{{
			Type: "text",
			Text: FormatRequestContextForTurn(requestContext),
		}},
	}
	resp, err := r.Client.SubmitTurn(ctx, req)
	if err != nil {
		record.Status = SubmissionStatusFailed
		record.KernelError = err.Error()
		record.UpdatedAt = now()
		_ = r.InboundStore.Complete(ctx, record)
		return ProcessResult{Record: record}, err
	}
	record.TurnID = resp.TurnID
	if resp.SessionID != "" {
		record.KernelSessionID = resp.SessionID
	}
	if resp.Error != nil {
		record.Status = SubmissionStatusFailed
		record.KernelError = resp.Error.Code + ": " + resp.Error.Message
		record.UpdatedAt = now()
		_ = r.InboundStore.Complete(ctx, record)
		return ProcessResult{Record: record}, fmt.Errorf("kernel turn failed: %s", record.KernelError)
	}
	record.Status = SubmissionStatusSubmitted
	record.UpdatedAt = now()
	if err := r.InboundStore.Complete(ctx, record); err != nil {
		return ProcessResult{Record: record}, err
	}
	result := ProcessResult{Record: record, FinalText: resp.Final.Text}
	r.deliverFinalText(ctx, requestContext, record, &result)
	return result, nil
}

func uncheckedDirectExternalEvent(event ExternalEvent) ExternalEvent {
	event.SourceValidation = SourceValidationUnchecked
	return event
}

func (r *Runtime) deliverFinalText(ctx context.Context, requestContext RequestContext, record InboundSubmissionRecord, result *ProcessResult) {
	if result == nil || r.Store == nil || strings.TrimSpace(result.FinalText) == "" {
		return
	}
	command := AppCommand{
		CommandID: stableOpaqueID("cmd", requestContext.RequestID, record.TurnID, "final_reply"),
		Kind:      "send_message",
		TargetRef: ExternalThreadRef{
			Connector:  requestContext.ThreadRef.Connector,
			Kind:       requestContext.ThreadRef.Kind,
			ExternalID: requestContext.ThreadRef.ExternalID,
			Display:    requestContext.ThreadRef.Display,
		},
		Body:      result.FinalText,
		DedupeKey: stableOpaqueID("reply", requestContext.RequestID, record.TurnID),
		CreatedAt: record.UpdatedAt,
		Metadata: map[string]string{
			"source_request_id": requestContext.RequestID,
			"kernel_turn_id":    record.TurnID,
		},
	}
	item, duplicate, err := r.EnqueueCommand(ctx, command)
	if err != nil {
		result.DeliveryError = err.Error()
		return
	}
	result.OutboxItem = &item
	result.OutboxDuplicate = duplicate
	if duplicate {
		return
	}
	receipt, err := r.ExecuteOutboxItem(ctx, item.OutboxID)
	result.DeliveryReceipt = &receipt
	if err != nil {
		result.DeliveryError = err.Error()
	}
}
