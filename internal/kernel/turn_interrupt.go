package kernel

import (
	"context"
	"errors"
	"strings"
)

var ErrTurnInterrupted = errors.New("turn interrupted")
var ErrNoActiveTurn = errors.New("no active turn")
var ErrSessionActive = errors.New("session has active work")

const (
	activeSessionKindTurn              = "turn"
	activeSessionKindContextCompaction = "context_compaction"
)

func (k *Kernel) InterruptSession(sessionID string, req TurnInterruptRequest) (TurnInterruptionProjection, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return TurnInterruptionProjection{}, errors.New("session id is required")
	}
	reason := strings.TrimSpace(req.Reason)
	k.activeTurnMu.Lock()
	active := k.activeTurns[sessionID]
	if active == nil || active.kind != activeSessionKindTurn {
		k.activeTurnMu.Unlock()
		return TurnInterruptionProjection{}, ErrNoActiveTurn
	}
	active.reason = reason
	cancel := active.cancel
	projection := TurnInterruptionProjection{
		SessionID:       active.sessionID,
		TurnID:          active.turnID,
		Phase:           RuntimePhaseEnded,
		TerminalOutcome: TerminalOutcomeInterrupted,
		TerminalCause:   TerminalCauseUserCancelled,
		Reason:          reason,
		InterruptedAt:   k.clock(),
	}
	k.activeTurnMu.Unlock()
	cancel()
	return projection, nil
}

func (k *Kernel) beginActiveTurn(ctx context.Context, sessionID string, turnID string) (context.Context, func()) {
	runCtx, finish, _ := k.tryBeginActiveTurn(ctx, sessionID, turnID)
	return runCtx, finish
}

func (k *Kernel) tryBeginActiveTurn(ctx context.Context, sessionID string, turnID string) (context.Context, func(), bool) {
	if ctx == nil {
		ctx = context.Background()
	}
	runCtx, cancel := context.WithCancel(ctx)
	active := &activeTurn{
		sessionID: strings.TrimSpace(sessionID),
		turnID:    strings.TrimSpace(turnID),
		kind:      activeSessionKindTurn,
		cancel:    cancel,
	}
	k.activeTurnMu.Lock()
	if k.activeTurns == nil {
		k.activeTurns = map[string]*activeTurn{}
	}
	if k.activeTurns[active.sessionID] != nil {
		k.activeTurnMu.Unlock()
		cancel()
		return ctx, func() {}, false
	}
	k.activeTurns[active.sessionID] = active
	k.activeTurnMu.Unlock()
	return runCtx, func() {
		k.activeTurnMu.Lock()
		if current := k.activeTurns[active.sessionID]; current == active {
			delete(k.activeTurns, active.sessionID)
		}
		k.activeTurnMu.Unlock()
		cancel()
	}, true
}

func (k *Kernel) reserveActiveSessionControl(sessionID string, kind string) (func(), bool) {
	active := &activeTurn{
		sessionID: strings.TrimSpace(sessionID),
		kind:      strings.TrimSpace(kind),
		cancel:    func() {},
	}
	if active.kind == "" {
		active.kind = "control"
	}
	k.activeTurnMu.Lock()
	if k.activeTurns == nil {
		k.activeTurns = map[string]*activeTurn{}
	}
	if active.sessionID == "" || k.activeTurns[active.sessionID] != nil {
		k.activeTurnMu.Unlock()
		return func() {}, false
	}
	k.activeTurns[active.sessionID] = active
	k.activeTurnMu.Unlock()
	return func() {
		k.activeTurnMu.Lock()
		if current := k.activeTurns[active.sessionID]; current == active {
			delete(k.activeTurns, active.sessionID)
		}
		k.activeTurnMu.Unlock()
	}, true
}

func (k *Kernel) completeInterruptedTurn(sessionID string, turnID string) (TurnResponse, error) {
	if err := k.appendTurnInterruption(sessionID, turnID); err != nil {
		return TurnResponse{}, err
	}
	events, err := k.TurnEvents(turnID)
	if err != nil {
		return TurnResponse{}, err
	}
	turnError := TurnError{
		Code:    "turn_interrupted",
		Message: "turn was interrupted",
	}
	return TurnResponse{
		SessionID: strings.TrimSpace(sessionID),
		TurnID:    strings.TrimSpace(turnID),
		Events:    events,
		Error:     &turnError,
	}, ErrTurnInterrupted
}

func (k *Kernel) appendTurnInterruption(sessionID string, turnID string) error {
	interruptedAt := k.clock()
	interruption := TurnInterruptionProjection{
		SessionID:       strings.TrimSpace(sessionID),
		TurnID:          strings.TrimSpace(turnID),
		Phase:           RuntimePhaseEnded,
		TerminalOutcome: TerminalOutcomeInterrupted,
		TerminalCause:   TerminalCauseUserCancelled,
		Reason:          k.activeTurnInterruptReason(sessionID, turnID),
		InterruptedAt:   interruptedAt,
	}
	return k.appendEvent(StoredEvent{
		EventID:   newID("evt", interruptedAt),
		SessionID: interruption.SessionID,
		TurnID:    interruption.TurnID,
		Type:      "assistant.interrupted",
		CreatedAt: interruptedAt,
		Data: EventData{
			TurnInterruption: &interruption,
		},
	})
}

func (k *Kernel) activeTurnInterruptReason(sessionID string, turnID string) string {
	sessionID = strings.TrimSpace(sessionID)
	turnID = strings.TrimSpace(turnID)
	k.activeTurnMu.Lock()
	defer k.activeTurnMu.Unlock()
	active := k.activeTurns[sessionID]
	if active == nil || active.kind != activeSessionKindTurn || active.turnID != turnID {
		return ""
	}
	return active.reason
}

func isTurnContextInterrupted(ctx context.Context, err error) bool {
	if ctx == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return true
	}
	return errors.Is(ctx.Err(), context.Canceled)
}
