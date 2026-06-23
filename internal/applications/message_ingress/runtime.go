package messageingress

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Runtime struct {
	Store  InboundStore
	Client TurnClient
	Mapper SessionMapper
	Now    func() time.Time
}

func (r *Runtime) Process(ctx context.Context, msg ChannelMessage) (ProcessResult, error) {
	if err := msg.Validate(); err != nil {
		return ProcessResult{}, err
	}
	if r.Store == nil {
		return ProcessResult{}, errors.New("message ingress runtime missing inbound store")
	}
	if r.Client == nil {
		return ProcessResult{}, errors.New("message ingress runtime missing kernel turn client")
	}
	mapper := r.Mapper
	if mapper == nil {
		mapper = DefaultSessionMapper{}
	}
	sessionID, err := mapper.Map(msg)
	if err != nil {
		return ProcessResult{}, err
	}
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}
	currentTime := now()
	record := SubmissionRecord{
		RawKey:            msg.RawDedupeKey(),
		KernelIdempotency: KernelIdempotencyKey(msg),
		Channel:           strings.TrimSpace(msg.Channel),
		Adapter:           strings.TrimSpace(msg.Adapter),
		MessageID:         strings.TrimSpace(msg.MessageID),
		ThreadID:          strings.TrimSpace(msg.ThreadID),
		UserID:            strings.TrimSpace(msg.UserID),
		SessionID:         sessionID,
		Status:            SubmissionStatusPending,
		CreatedAt:         currentTime,
		UpdatedAt:         currentTime,
	}
	record, reserved, err := r.Store.Reserve(ctx, record)
	if err != nil {
		return ProcessResult{}, err
	}
	if !reserved {
		return ProcessResult{Record: record, Duplicate: true}, nil
	}

	req := TurnSubmitRequest{
		SessionID:      sessionID,
		IdempotencyKey: KernelIdempotencyKey(msg),
		InputItems: []TurnInputItem{{
			Type: "text",
			Text: FormatInboundInput(msg),
		}},
	}
	resp, err := r.Client.SubmitTurn(ctx, req)
	if err != nil {
		record.Status = SubmissionStatusFailed
		record.KernelError = err.Error()
		record.UpdatedAt = now()
		_ = r.Store.Complete(ctx, record)
		return ProcessResult{Record: record}, err
	}
	record.TurnID = resp.TurnID
	if resp.SessionID != "" {
		record.SessionID = resp.SessionID
	}
	if resp.Error != nil {
		record.Status = SubmissionStatusFailed
		record.KernelError = resp.Error.Code + ": " + resp.Error.Message
		record.UpdatedAt = now()
		_ = r.Store.Complete(ctx, record)
		return ProcessResult{Record: record}, fmt.Errorf("kernel turn failed: %s", record.KernelError)
	}
	record.Status = SubmissionStatusSubmitted
	record.UpdatedAt = now()
	if err := r.Store.Complete(ctx, record); err != nil {
		return ProcessResult{Record: record}, err
	}
	return ProcessResult{Record: record, FinalText: resp.Final.Text}, nil
}
