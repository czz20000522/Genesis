package kernel

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type Kernel struct {
	ledger       Ledger
	provider     Provider
	runtimeToken string
	toolPolicy   ToolPolicy
	clock        func() time.Time
	operationMu  sync.Mutex
}

func New(config Config) (*Kernel, error) {
	if strings.TrimSpace(config.LedgerPath) == "" {
		return nil, errors.New("ledger path is required")
	}
	provider := config.Provider
	if provider == nil {
		provider = FakeProvider{}
	}
	clock := config.Clock
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	return &Kernel{
		ledger:       NewJSONLLedger(config.LedgerPath),
		provider:     provider,
		runtimeToken: strings.TrimSpace(config.RuntimeToken),
		toolPolicy:   normalizedToolPolicy(config.ToolPolicy),
		clock:        clock,
	}, nil
}

func (k *Kernel) Ready() ReadyResponse {
	providerStatus := k.provider.Ready()
	runtimeAuth := ReadyCheck{Status: "ok"}
	if k.runtimeToken == "" {
		runtimeAuth = ReadyCheck{Status: "blocked", Reason: "runtime_token_missing"}
	}
	ledgerStatus := k.ledger.Ready()
	status := "ok"
	if providerStatus.Status != "ok" || runtimeAuth.Status != "ok" || ledgerStatus.Status != "ok" {
		status = "blocked"
	}
	return ReadyResponse{
		Status:      status,
		Provider:    providerStatus,
		RuntimeAuth: runtimeAuth,
		Ledger:      ledgerStatus,
		LedgerPath:  k.ledger.Path(),
	}
}

func (k *Kernel) SubmitTurn(ctx context.Context, req TurnRequest) (TurnResponse, error) {
	if err := validateTurnRequest(req); err != nil {
		return TurnResponse{}, err
	}
	ingressRisks, err := scanTurnIngressSecurity(req.InputItems)
	if err != nil {
		return TurnResponse{}, err
	}
	now := k.clock()
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		sessionID = newID("sess", now)
	}
	turnID := newID("turn", now)
	recalledMemories, err := k.recallMemories(req.InputItems)
	if err != nil {
		return TurnResponse{}, err
	}

	submitted := StoredEvent{
		EventID:   newID("evt", now),
		SessionID: sessionID,
		TurnID:    turnID,
		Type:      "turn.submitted",
		CreatedAt: now,
		Data: EventData{
			InputItems:       req.InputItems,
			IngressRisks:     ingressRisks,
			RecalledMemories: recalledMemories,
		},
	}
	if err := k.appendEvent(submitted); err != nil {
		return TurnResponse{}, err
	}

	modelResp, err := k.provider.Complete(ctx, ModelRequest{
		SessionID:  sessionID,
		TurnID:     turnID,
		InputItems: modelInputItems(req.InputItems, recalledMemories),
	})
	if err != nil {
		failedAt := k.clock()
		failure := turnFailureFromProviderError(err)
		failed := StoredEvent{
			EventID:   newID("evt", failedAt),
			SessionID: sessionID,
			TurnID:    turnID,
			Type:      "turn.failed",
			CreatedAt: failedAt,
			Data: EventData{
				TurnError: &failure,
			},
		}
		if appendErr := k.appendEvent(failed); appendErr != nil {
			return TurnResponse{}, appendErr
		}
		return TurnResponse{}, fmt.Errorf("provider complete: %w", err)
	}

	completedAt := k.clock()
	final := FinalMessage{Text: modelResp.Text, Model: modelResp.Model}
	completed := StoredEvent{
		EventID:   newID("evt", completedAt),
		SessionID: sessionID,
		TurnID:    turnID,
		Type:      "model.final",
		CreatedAt: completedAt,
		Data: EventData{
			Final: &final,
		},
	}
	if err := k.appendEvent(completed); err != nil {
		return TurnResponse{}, err
	}

	return TurnResponse{
		SessionID: sessionID,
		TurnID:    turnID,
		Events: []Event{
			toEvent(submitted),
			toEvent(completed),
		},
		Final: final,
	}, nil
}

func (k *Kernel) Session(sessionID string) (SessionProjection, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return SessionProjection{}, errors.New("session id is required")
	}
	events, err := k.loadEvents()
	if err != nil {
		return SessionProjection{}, err
	}
	projection := SessionProjection{
		SessionID:        sessionID,
		Turns:            []TurnProjection{},
		Operations:       []OperationProjection{},
		MemoryCandidates: []MemoryCandidateProjection{},
		Events:           []EventProjection{},
	}
	turnByID := map[string]int{}
	candidateByID := map[string]int{}
	for _, event := range events {
		if event.SessionID != sessionID {
			continue
		}
		projection.Events = append(projection.Events, EventProjection{
			EventID:     event.EventID,
			TurnID:      event.TurnID,
			OperationID: event.OperationID,
			CandidateID: event.CandidateID,
			Type:        event.Type,
			CreatedAt:   event.CreatedAt,
		})
		switch event.Type {
		case "turn.submitted":
			turnByID[event.TurnID] = len(projection.Turns)
			projection.Turns = append(projection.Turns, TurnProjection{
				TurnID:           event.TurnID,
				Status:           "running",
				InputItems:       event.Data.InputItems,
				IngressRisks:     event.Data.IngressRisks,
				RecalledMemories: event.Data.RecalledMemories,
				StartedAt:        event.CreatedAt,
			})
		case "model.final":
			idx, ok := turnByID[event.TurnID]
			if !ok {
				continue
			}
			projection.Turns[idx].Status = "completed"
			if event.Data.Final != nil {
				projection.Turns[idx].FinalMessage = *event.Data.Final
			}
			projection.Turns[idx].CompletedAt = event.CreatedAt
		case "turn.failed":
			idx, ok := turnByID[event.TurnID]
			if !ok {
				continue
			}
			projection.Turns[idx].Status = "failed"
			if event.Data.TurnError != nil {
				projection.Turns[idx].Error = event.Data.TurnError
			}
			projection.Turns[idx].CompletedAt = event.CreatedAt
		case "operation.running", "operation.completed", "operation.failed", "operation.blocked":
			if event.Data.Operation != nil {
				operation := *event.Data.Operation
				replaced := false
				for i := range projection.Operations {
					if projection.Operations[i].OperationID == operation.OperationID {
						projection.Operations[i] = operation
						replaced = true
						break
					}
				}
				if !replaced {
					projection.Operations = append(projection.Operations, operation)
				}
			}
		case "memory.candidate.created", "memory.candidate.approved":
			if event.Data.MemoryCandidate == nil {
				continue
			}
			candidate := *event.Data.MemoryCandidate
			idx, ok := candidateByID[candidate.CandidateID]
			if ok {
				projection.MemoryCandidates[idx] = candidate
				continue
			}
			candidateByID[candidate.CandidateID] = len(projection.MemoryCandidates)
			projection.MemoryCandidates = append(projection.MemoryCandidates, candidate)
		}
	}
	if len(projection.Events) == 0 {
		return SessionProjection{}, ErrSessionNotFound
	}
	return projection, nil
}

var ErrSessionNotFound = errors.New("session not found")
var ErrLedgerUnavailable = errors.New("ledger unavailable")

func (k *Kernel) appendEvent(event StoredEvent) error {
	if err := k.ensureLedgerReady(); err != nil {
		return err
	}
	if err := k.ledger.Append(event); err != nil {
		return wrapLedgerUnavailable(err)
	}
	return nil
}

func (k *Kernel) loadEvents() ([]StoredEvent, error) {
	events, err := k.ledger.Load()
	if err != nil {
		return nil, wrapLedgerUnavailable(err)
	}
	return events, nil
}

func (k *Kernel) ensureLedgerReady() error {
	check := k.ledger.Ready()
	if check.Status == "ok" {
		return nil
	}
	switch check.Reason {
	case "ledger_corrupt":
		return wrapLedgerUnavailable(ErrLedgerCorrupt)
	case "ledger_unreadable":
		return wrapLedgerUnavailable(ErrLedgerUnreadable)
	default:
		return wrapLedgerUnavailable(ErrLedgerUnwritable)
	}
}

func wrapLedgerUnavailable(err error) error {
	if errors.Is(err, ErrLedgerUnavailable) {
		return err
	}
	return fmt.Errorf("%w: %w", ErrLedgerUnavailable, err)
}

func ledgerErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrLedgerCorrupt):
		return "ledger_corrupt"
	case errors.Is(err, ErrLedgerUnreadable):
		return "ledger_unreadable"
	case errors.Is(err, ErrLedgerUnwritable):
		return "ledger_unwritable"
	default:
		return "ledger_unavailable"
	}
}

func validateTurnRequest(req TurnRequest) error {
	if len(req.InputItems) == 0 {
		return errors.New("input_items is required")
	}
	for i, item := range req.InputItems {
		if item.Type != "text" {
			return fmt.Errorf("input_items[%d].type must be text", i)
		}
		if strings.TrimSpace(item.Text) == "" {
			return fmt.Errorf("input_items[%d].text is required", i)
		}
	}
	return nil
}

func turnFailureFromProviderError(err error) TurnError {
	code := "provider_error"
	if errors.Is(err, ErrProviderUnavailable) {
		code = "provider_unavailable"
	}
	return TurnError{
		Code:    code,
		Message: err.Error(),
	}
}

func toEvent(event StoredEvent) Event {
	return Event{
		EventID:     event.EventID,
		SessionID:   event.SessionID,
		TurnID:      event.TurnID,
		OperationID: event.OperationID,
		CandidateID: event.CandidateID,
		Type:        event.Type,
		CreatedAt:   event.CreatedAt,
		Data:        event.Data,
	}
}
