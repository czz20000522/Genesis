package kernel

import (
	"errors"
	"fmt"
	"strings"
)

var ErrWorkNotFound = errors.New("work not found")

func (k *Kernel) SubmitWork(req WorkSubmitRequest) (WorkProjection, error) {
	if err := validateWorkSubmitRequest(req); err != nil {
		return WorkProjection{}, err
	}
	k.workMu.Lock()
	defer k.workMu.Unlock()

	sessionID := strings.TrimSpace(req.SessionID)
	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if idempotencyKey != "" {
		work, ok, err := k.workByIdempotencyKey(sessionID, idempotencyKey)
		if err != nil {
			return WorkProjection{}, err
		}
		if ok {
			return work, nil
		}
	}
	now := k.clock()
	work := WorkProjection{
		WorkID:         newID("work", now),
		SessionID:      sessionID,
		Title:          strings.TrimSpace(req.Title),
		SourceRef:      strings.TrimSpace(req.SourceRef),
		IdempotencyKey: idempotencyKey,
		Status:         WorkStatusOpen,
		CreatedAt:      now,
	}
	event := StoredEvent{
		EventID:   newID("evt", now),
		SessionID: work.SessionID,
		WorkID:    work.WorkID,
		Type:      "work.submitted",
		CreatedAt: now,
		Data: EventData{
			Work: &work,
		},
	}
	if err := k.appendEvent(event); err != nil {
		return WorkProjection{}, err
	}
	return work, nil
}

func (k *Kernel) Work(workID string) (WorkProjection, error) {
	workID = strings.TrimSpace(workID)
	if workID == "" {
		return WorkProjection{}, errors.New("work id is required")
	}
	works, err := k.works()
	if err != nil {
		return WorkProjection{}, err
	}
	work, ok := works[workID]
	if !ok {
		return WorkProjection{}, ErrWorkNotFound
	}
	return work, nil
}

func (k *Kernel) CancelWork(workID string, req WorkCancelRequest) (WorkProjection, error) {
	workID = strings.TrimSpace(workID)
	if workID == "" {
		return WorkProjection{}, errors.New("work id is required")
	}
	if err := validateWorkCancelRequest(req); err != nil {
		return WorkProjection{}, err
	}
	k.workMu.Lock()
	defer k.workMu.Unlock()

	works, err := k.works()
	if err != nil {
		return WorkProjection{}, err
	}
	work, ok := works[workID]
	if !ok {
		return WorkProjection{}, ErrWorkNotFound
	}
	if work.Status == WorkStatusCanceled {
		return work, nil
	}
	now := k.clock()
	work.Status = WorkStatusCanceled
	work.CancelAuthority = strings.TrimSpace(req.CancelAuthority)
	work.CancelReason = strings.TrimSpace(req.CancelReason)
	work.CancelEvidenceRef = strings.TrimSpace(req.CancelEvidenceRef)
	work.CanceledAt = &now
	event := StoredEvent{
		EventID:   newID("evt", now),
		SessionID: work.SessionID,
		WorkID:    work.WorkID,
		Type:      "work.canceled",
		CreatedAt: now,
		Data: EventData{
			Work: &work,
		},
	}
	if err := k.appendEvent(event); err != nil {
		return WorkProjection{}, err
	}
	return work, nil
}

func validateWorkSubmitRequest(req WorkSubmitRequest) error {
	if strings.TrimSpace(req.SessionID) == "" {
		return errors.New("session_id is required")
	}
	if strings.TrimSpace(req.Title) == "" {
		return errors.New("title is required")
	}
	if strings.TrimSpace(req.SourceRef) == "" {
		return errors.New("source_ref is required")
	}
	if err := validateKernelControlToken("session_id", req.SessionID); err != nil {
		return err
	}
	if err := validateWorkRef("source_ref", req.SourceRef); err != nil {
		return err
	}
	if err := validateIdempotencyKey(req.IdempotencyKey); err != nil {
		return err
	}
	return nil
}

func validateWorkCancelRequest(req WorkCancelRequest) error {
	if strings.TrimSpace(req.CancelAuthority) == "" {
		return errors.New("cancel_authority is required")
	}
	if strings.TrimSpace(req.CancelReason) == "" {
		return errors.New("cancel_reason is required")
	}
	if strings.TrimSpace(req.CancelEvidenceRef) == "" {
		return errors.New("cancel_evidence_ref is required")
	}
	if err := validateWorkAuthority(req.CancelAuthority); err != nil {
		return err
	}
	if err := validateWorkRef("cancel_evidence_ref", req.CancelEvidenceRef); err != nil {
		return err
	}
	return nil
}

func validateWorkRef(field string, value string) error {
	return validateKernelRef(field, value)
}

func validateWorkAuthority(value string) error {
	return validateKernelAuthority("cancel_authority", value)
}

func (k *Kernel) workByIdempotencyKey(sessionID string, key string) (WorkProjection, bool, error) {
	works, err := k.works()
	if err != nil {
		return WorkProjection{}, false, err
	}
	for _, work := range works {
		if work.SessionID == sessionID && work.IdempotencyKey == key {
			return work, true, nil
		}
	}
	return WorkProjection{}, false, nil
}

func (k *Kernel) works() (map[string]WorkProjection, error) {
	events, err := k.loadEvents()
	if err != nil {
		return nil, err
	}
	works := map[string]WorkProjection{}
	for _, event := range events {
		if event.Data.Work == nil {
			continue
		}
		switch event.Type {
		case "work.submitted", "work.canceled":
			work := *event.Data.Work
			if work.WorkID == "" {
				work.WorkID = event.WorkID
			}
			if work.WorkID == "" {
				return nil, fmt.Errorf("%s event missing work id", event.Type)
			}
			current, exists := works[work.WorkID]
			merged, err := mergeWorkProjection(current, work, exists)
			if err != nil {
				return nil, err
			}
			works[work.WorkID] = merged
		}
	}
	return works, nil
}

func mergeWorkProjection(current WorkProjection, incoming WorkProjection, exists bool) (WorkProjection, error) {
	if !exists {
		return incoming, nil
	}
	if current.Status == WorkStatusCanceled {
		if incoming.Status == WorkStatusCanceled && !sameWorkCancelDecision(current, incoming) {
			return WorkProjection{}, fmt.Errorf("competing work cancel evidence for %s", current.WorkID)
		}
		return current, nil
	}
	if incoming.Status == WorkStatusCanceled {
		return incoming, nil
	}
	return incoming, nil
}

func sameWorkCancelDecision(left WorkProjection, right WorkProjection) bool {
	return left.CancelAuthority == right.CancelAuthority &&
		left.CancelReason == right.CancelReason &&
		left.CancelEvidenceRef == right.CancelEvidenceRef
}
