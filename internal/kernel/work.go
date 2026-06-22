package kernel

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	WorkStatusOpen     = "open"
	WorkStatusCanceled = "canceled"
)

var ErrWorkNotFound = errors.New("work not found")

var (
	workRefPattern       = regexp.MustCompile(`^(turn|review|work|operation|memory|event):[A-Za-z0-9][A-Za-z0-9._:/=-]{0,190}$`)
	workAuthorityPattern = regexp.MustCompile(`^(runtime|operator|user|daemon|system):[A-Za-z0-9][A-Za-z0-9._:/=-]{0,190}$`)
)

func (k *Kernel) SubmitWork(req WorkSubmitRequest) (WorkProjection, error) {
	if err := validateWorkSubmitRequest(req); err != nil {
		return WorkProjection{}, err
	}
	now := k.clock()
	work := WorkProjection{
		WorkID:    newID("work", now),
		SessionID: strings.TrimSpace(req.SessionID),
		Title:     strings.TrimSpace(req.Title),
		SourceRef: strings.TrimSpace(req.SourceRef),
		Status:    WorkStatusOpen,
		CreatedAt: now,
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
	if err := validateWorkTextNotSecret("title", req.Title); err != nil {
		return err
	}
	if err := validateWorkRef("source_ref", req.SourceRef); err != nil {
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
	if err := validateWorkTextNotSecret("cancel_reason", req.CancelReason); err != nil {
		return err
	}
	if err := validateWorkRef("cancel_evidence_ref", req.CancelEvidenceRef); err != nil {
		return err
	}
	return nil
}

func validateWorkRef(field string, value string) error {
	value = strings.TrimSpace(value)
	if !workRefPattern.MatchString(value) {
		return fmt.Errorf("%s must be a kernel ref", field)
	}
	return validateWorkTextNotSecret(field, value)
}

func validateWorkAuthority(value string) error {
	value = strings.TrimSpace(value)
	if !workAuthorityPattern.MatchString(value) {
		return errors.New("cancel_authority must be a kernel authority ref")
	}
	return validateWorkTextNotSecret("cancel_authority", value)
}

func validateWorkTextNotSecret(field string, value string) error {
	if redactEvidenceText(value) != value {
		return fmt.Errorf("%s must not contain secret-shaped content", field)
	}
	return nil
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
			works[work.WorkID] = work
		}
	}
	return works, nil
}
