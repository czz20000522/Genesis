package kernel

import (
	"context"
	"errors"
	"strings"
)

const (
	contextCompactionAdmissionAdmitted = "admitted"
	contextCompactionAdmissionRefused  = "refused"
	contextCompactionRefusalActiveTurn = "active_turn_running"
)

type ContextCompactionControlRequest struct{}

type ContextCompactionControlResponse struct {
	SessionID       string `json:"session_id"`
	AdmissionResult string `json:"admission_result"`
	ReasonClass     string `json:"reason_class,omitempty"`
}

func (k *Kernel) CompactSessionContext(ctx context.Context, sessionID string) (ContextCompactionControlResponse, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ContextCompactionControlResponse{}, errors.New("session id is required")
	}
	finishActiveControl, admitted := k.reserveActiveSessionControl(sessionID, activeSessionKindContextCompaction)
	if !admitted {
		return ContextCompactionControlResponse{
			SessionID:       sessionID,
			AdmissionResult: contextCompactionAdmissionRefused,
			ReasonClass:     contextCompactionRefusalActiveTurn,
		}, nil
	}
	defer finishActiveControl()
	events, err := k.loadEvents()
	if err != nil {
		return ContextCompactionControlResponse{}, err
	}
	if sessionHasActiveTurn(events, sessionID) {
		return ContextCompactionControlResponse{
			SessionID:       sessionID,
			AdmissionResult: contextCompactionAdmissionRefused,
			ReasonClass:     contextCompactionRefusalActiveTurn,
		}, nil
	}
	provider, err := k.sessionProviderForSession(sessionID)
	if err != nil {
		return ContextCompactionControlResponse{}, err
	}
	k.runContextCompaction(ctx, ContextCompactionCommand{
		SessionID: sessionID,
		Trigger:   contextCompactionTriggerManual,
	}, provider)
	return ContextCompactionControlResponse{
		SessionID:       sessionID,
		AdmissionResult: contextCompactionAdmissionAdmitted,
	}, nil
}

func sessionHasActiveTurn(events []StoredEvent, sessionID string) bool {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return false
	}
	active := map[string]bool{}
	for _, event := range events {
		if event.SessionID != sessionID || strings.TrimSpace(event.TurnID) == "" {
			continue
		}
		switch event.Type {
		case "turn.submitted":
			active[event.TurnID] = true
		case "model.final", "turn.failed", "assistant.interrupted":
			delete(active, event.TurnID)
		case "turn.paused":
			active[event.TurnID] = true
		}
	}
	return len(active) > 0
}
