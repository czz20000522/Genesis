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

const kernelObservationContextBytes = 4096

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

func kernelObservationContext(observations []kernelObservation) (string, []string) {
	if len(observations) == 0 {
		return "", nil
	}
	header := "Kernel observations:"
	lines := []string{header}
	used := len([]byte(header))
	deliveredIDs := []string{}
	omitted := 0
	for _, observation := range observations {
		next := kernelObservationLines(observation)
		needed := 0
		for _, line := range next {
			needed += 1 + len([]byte(line))
		}
		if used+needed > kernelObservationContextBytes {
			omitted++
			continue
		}
		lines = append(lines, next...)
		used += needed
		if eventID := strings.TrimSpace(observation.EventID); eventID != "" {
			deliveredIDs = append(deliveredIDs, eventID)
		}
	}
	if omitted > 0 {
		line := "- additional kernel observations omitted by context budget"
		if used+1+len([]byte(line)) <= kernelObservationContextBytes {
			lines = append(lines, line)
		}
	}
	if len(lines) == 1 {
		return "", nil
	}
	return strings.Join(lines, "\n"), deliveredIDs
}

func kernelObservationLines(observation kernelObservation) []string {
	job := observation.Job
	lines := []string{
		fmt.Sprintf(
			"- %s job_id=%s tool=%s status=%s",
			strings.TrimSpace(observation.EventType),
			strings.TrimSpace(job.JobID),
			strings.TrimSpace(job.Tool),
			strings.TrimSpace(job.Status),
		),
	}
	if receipt := strings.TrimSpace(job.Receipt); receipt != "" {
		lines = append(lines, "  receipt: "+boundedTimelinePreview(receipt))
	}
	if failure := strings.TrimSpace(job.FailureReason); failure != "" {
		lines = append(lines, "  failure_reason: "+boundedTimelinePreview(failure))
	}
	if job.ExitCode != nil {
		lines = append(lines, fmt.Sprintf("  exit_code: %d", *job.ExitCode))
	}
	if stdout := strings.TrimSpace(job.Stdout); stdout != "" {
		lines = append(lines, "  stdout: "+boundedTimelinePreview(stdout))
	}
	if stderr := strings.TrimSpace(job.Stderr); stderr != "" {
		lines = append(lines, "  stderr: "+boundedTimelinePreview(stderr))
	}
	return lines
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
