package kernel

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrSessionModelInvalid                 = errors.New("session model binding invalid")
	ErrSessionModelUnselected              = errors.New("session model unselected")
	ErrSessionModelChangeBlockedActiveTurn = errors.New("session model change blocked by active turn")
)

type SessionModelBindingRequest struct {
	ProfileID string
}

type SessionModelBinding struct {
	ProfileID string `json:"profile_id"`
}

func (k *Kernel) BindSessionModel(sessionID string, request SessionModelBindingRequest) error {
	sessionID = strings.TrimSpace(sessionID)
	profileID := strings.TrimSpace(request.ProfileID)
	if sessionID == "" || profileID == "" || k.sessionProviderResolver == nil {
		return ErrSessionModelInvalid
	}
	finishControl, admitted := k.reserveActiveSessionControl(sessionID, "session_model_binding")
	if !admitted {
		return ErrSessionModelChangeBlockedActiveTurn
	}
	defer finishControl()
	events, err := k.loadEvents()
	if err != nil {
		return err
	}
	if sessionHasActiveTurn(events, sessionID) {
		return ErrSessionModelChangeBlockedActiveTurn
	}
	if _, err := k.sessionProviderForProfile(profileID); err != nil {
		return err
	}
	now := k.clock()
	return k.appendEvent(StoredEvent{
		EventID:   newID("evt", now),
		SessionID: sessionID,
		Type:      "session.model_bound",
		CreatedAt: now,
		Data: EventData{SessionModel: &SessionModelBinding{
			ProfileID: profileID,
		}},
	})
}

func (k *Kernel) sessionProviderForSession(sessionID string) (Provider, error) {
	if k.sessionProviderResolver == nil {
		return k.provider, nil
	}
	projection, err := k.Session(sessionID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil, ErrSessionModelUnselected
		}
		return nil, err
	}
	return k.sessionProviderForProfile(projection.ModelProfileID)
}

func (k *Kernel) sessionProviderForProfile(profileID string) (Provider, error) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return nil, ErrSessionModelUnselected
	}
	if k.sessionProviderResolver == nil {
		return nil, ErrSessionModelInvalid
	}
	provider, err := k.sessionProviderResolver(profileID)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, ErrSessionModelInvalid
	}
	if status := provider.Ready(); status.Readiness != ReadinessReady {
		return nil, fmt.Errorf("%w: %s", ErrProviderUnavailable, strings.TrimSpace(status.ReadinessReason))
	}
	return provider, nil
}

func (k *Kernel) sessionProviderForTurnEvents(events []StoredEvent, turnID string) (Provider, error) {
	if k.sessionProviderResolver == nil {
		return k.provider, nil
	}
	sessionID, profileID, found := sessionModelBindingForTurn(events, turnID)
	if !found || strings.TrimSpace(sessionID) == "" || strings.TrimSpace(profileID) == "" {
		return nil, ErrSessionModelUnselected
	}
	return k.sessionProviderForProfile(profileID)
}

func sessionModelBindingForTurn(events []StoredEvent, turnID string) (string, string, bool) {
	turnID = strings.TrimSpace(turnID)
	if turnID == "" {
		return "", "", false
	}
	sessionID := ""
	turnIndex := -1
	for index, event := range events {
		if event.Type == "turn.submitted" && event.TurnID == turnID {
			sessionID = strings.TrimSpace(event.SessionID)
			turnIndex = index
			break
		}
	}
	if sessionID == "" || turnIndex < 0 {
		return "", "", false
	}
	profileID := ""
	for _, event := range events[:turnIndex] {
		if event.SessionID != sessionID || event.Type != "session.model_bound" || event.Data.SessionModel == nil {
			continue
		}
		profileID = strings.TrimSpace(event.Data.SessionModel.ProfileID)
	}
	return sessionID, profileID, true
}
