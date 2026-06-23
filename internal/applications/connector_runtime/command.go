package connectorruntime

import (
	"errors"
	"strings"
	"time"
)

func (c AppCommand) validate() error {
	switch {
	case strings.TrimSpace(c.Kind) == "":
		return errors.New("app command missing kind")
	case strings.TrimSpace(c.TargetRef.Connector) == "":
		return errors.New("app command missing target connector")
	case strings.TrimSpace(c.TargetRef.Kind) == "":
		return errors.New("app command missing target kind")
	case strings.TrimSpace(c.TargetRef.ExternalID) == "":
		return errors.New("app command missing target external id")
	case strings.TrimSpace(c.DedupeKey) == "":
		return errors.New("app command missing dedupe key")
	default:
		return nil
	}
}

func (c AppCommand) outboxDedupeKey() string {
	return stableOpaqueID("outidem",
		strings.TrimSpace(c.TargetRef.Connector),
		strings.TrimSpace(c.Kind),
		strings.TrimSpace(c.DedupeKey),
	)
}

func (c AppCommand) toOutboxItem(now time.Time) ConnectorOutboxItem {
	idempotencyKey := c.outboxDedupeKey()
	payload := map[string]string{}
	if strings.TrimSpace(c.Body) != "" {
		payload["body"] = strings.TrimSpace(c.Body)
	}
	commandID := strings.TrimSpace(c.CommandID)
	if commandID == "" {
		commandID = stableOpaqueID("cmd", c.TargetRef.Connector, c.Kind, c.DedupeKey)
	}
	return ConnectorOutboxItem{
		OutboxID:       stableOpaqueID("outbox", idempotencyKey),
		CommandID:      commandID,
		Connector:      strings.TrimSpace(c.TargetRef.Connector),
		ActionKind:     strings.TrimSpace(c.Kind),
		TargetRef:      sanitizedThreadRef(c.TargetRef),
		Payload:        payload,
		Status:         OutboxStatusQueued,
		AttemptCount:   0,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func sanitizedThreadRef(ref ExternalThreadRef) ExternalThreadRef {
	return ExternalThreadRef{
		Connector:  strings.TrimSpace(ref.Connector),
		Kind:       strings.TrimSpace(ref.Kind),
		ExternalID: strings.TrimSpace(ref.ExternalID),
		Display:    strings.TrimSpace(ref.Display),
	}
}
