package kernel

import (
	"fmt"
	"strings"
)

type kernelObservation struct {
	EventID   string
	EventType string
	Job       JobProjection
}

func pendingKernelObservations(events []StoredEvent, sessionID string) []kernelObservation {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil
	}
	delivered := deliveredKernelObservationIDs(events, sessionID)
	observations := []kernelObservation{}
	for _, event := range events {
		if event.SessionID != sessionID || delivered[event.EventID] {
			continue
		}
		switch event.Type {
		case "job.completed", "job.failed", "job.cancelled":
			if event.Data.Job == nil {
				continue
			}
			job := *event.Data.Job
			if strings.TrimSpace(job.JobID) == "" {
				job.JobID = event.JobID
			}
			observations = append(observations, kernelObservation{
				EventID:   event.EventID,
				EventType: event.Type,
				Job:       job,
			})
		}
	}
	return observations
}

func deliveredKernelObservationIDs(events []StoredEvent, sessionID string) map[string]bool {
	delivered := map[string]bool{}
	for _, event := range events {
		if event.SessionID != sessionID || event.Type != "kernel.observation.delivered" || event.Data.KernelObservationDelivery == nil {
			continue
		}
		for _, eventID := range event.Data.KernelObservationDelivery.ObservationEventIDs {
			eventID = strings.TrimSpace(eventID)
			if eventID != "" {
				delivered[eventID] = true
			}
		}
	}
	return delivered
}

func kernelObservationEventIDs(observations []kernelObservation) []string {
	ids := make([]string, 0, len(observations))
	for _, observation := range observations {
		eventID := strings.TrimSpace(observation.EventID)
		if eventID != "" {
			ids = append(ids, eventID)
		}
	}
	return ids
}

func kernelObservationContext(observations []kernelObservation) string {
	if len(observations) == 0 {
		return ""
	}
	lines := []string{"Kernel observations:"}
	for _, observation := range observations {
		job := observation.Job
		lines = append(lines, fmt.Sprintf(
			"- %s event_id=%s job_id=%s tool=%s status=%s",
			strings.TrimSpace(observation.EventType),
			strings.TrimSpace(observation.EventID),
			strings.TrimSpace(job.JobID),
			strings.TrimSpace(job.Tool),
			strings.TrimSpace(job.Status),
		))
		if receipt := strings.TrimSpace(job.Receipt); receipt != "" {
			lines = append(lines, "  receipt: "+receipt)
		}
		if failure := strings.TrimSpace(job.FailureReason); failure != "" {
			lines = append(lines, "  failure_reason: "+failure)
		}
	}
	return strings.Join(lines, "\n")
}

func (k *Kernel) appendKernelObservationDelivered(sessionID string, turnID string, observationEventIDs []string) error {
	cleaned := make([]string, 0, len(observationEventIDs))
	for _, eventID := range observationEventIDs {
		eventID = strings.TrimSpace(eventID)
		if eventID != "" {
			cleaned = append(cleaned, eventID)
		}
	}
	if len(cleaned) == 0 {
		return nil
	}
	now := k.clock()
	return k.appendEvent(StoredEvent{
		EventID:   newID("evt", now),
		SessionID: strings.TrimSpace(sessionID),
		TurnID:    strings.TrimSpace(turnID),
		Type:      "kernel.observation.delivered",
		CreatedAt: now,
		Data: EventData{
			KernelObservationDelivery: &KernelObservationDeliveryProjection{
				ObservationEventIDs: cleaned,
				ModelInputKind:      ModelInputKindKernelObservationContext,
			},
		},
	})
}
