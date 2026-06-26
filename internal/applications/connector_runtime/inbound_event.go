package connectorruntime

import (
	"errors"
	"sort"
	"strings"
)

func (e ExternalEvent) Validate() error {
	connector := strings.TrimSpace(e.Connector)
	switch {
	case connector == "":
		return errors.New("external event missing connector")
	case strings.TrimSpace(e.ExternalEventID) == "":
		return errors.New("external event missing external_event_id")
	case strings.TrimSpace(e.EventType) == "":
		return errors.New("external event missing event_type")
	case strings.TrimSpace(e.ThreadRef.Connector) == "":
		return errors.New("external event missing thread connector")
	case strings.TrimSpace(e.ThreadRef.Kind) == "":
		return errors.New("external event missing thread kind")
	case strings.TrimSpace(e.ThreadRef.ExternalID) == "":
		return errors.New("external event missing thread external id")
	case strings.TrimSpace(e.SenderRef.Connector) == "":
		return errors.New("external event missing sender connector")
	case strings.TrimSpace(e.SenderRef.Kind) == "":
		return errors.New("external event missing sender kind")
	case strings.TrimSpace(e.SenderRef.ExternalID) == "":
		return errors.New("external event missing sender external id")
	case strings.TrimSpace(e.MessageRef.Connector) == "":
		return errors.New("external event missing message connector")
	case strings.TrimSpace(e.MessageRef.Kind) == "":
		return errors.New("external event missing message kind")
	case strings.TrimSpace(e.MessageRef.ExternalID) == "":
		return errors.New("external event missing message external id")
	case strings.TrimSpace(e.Body) == "":
		return errors.New("external event missing body")
	case strings.TrimSpace(e.ThreadRef.Connector) != connector:
		return errors.New("external event thread connector mismatch")
	case strings.TrimSpace(e.SenderRef.Connector) != connector:
		return errors.New("external event sender connector mismatch")
	case strings.TrimSpace(e.MessageRef.Connector) != connector:
		return errors.New("external event message connector mismatch")
	default:
		return nil
	}
}

func (e ExternalEvent) DedupeKey() string {
	return stableOpaqueID("inbound", strings.TrimSpace(e.Connector), strings.TrimSpace(e.ExternalEventID))
}

func (e ExternalEvent) RequestContext(mapping ApplicationSessionMapping) RequestContext {
	sourceValidation := strings.TrimSpace(e.SourceValidation)
	if sourceValidation == "" {
		sourceValidation = SourceValidationUnchecked
	}
	return RequestContext{
		RequestID:            stableOpaqueID("req", strings.TrimSpace(e.Connector), strings.TrimSpace(e.ExternalEventID)),
		DedupeKey:            e.DedupeKey(),
		Connector:            strings.TrimSpace(e.Connector),
		EventType:            strings.TrimSpace(e.EventType),
		ThreadRef:            sanitizedThreadRef(e.ThreadRef),
		SenderRef:            sanitizedExternalRef(e.SenderRef),
		MessageRef:           sanitizedExternalRef(e.MessageRef),
		ResourceRefs:         append([]ExternalResourceRef(nil), e.ResourceRefs...),
		SourceValidation:     sourceValidation,
		ApplicationSessionID: mapping.ApplicationSessionID,
		KernelSessionID:      mapping.KernelSessionID,
		KernelIdempotencyKey: stableOpaqueID("turn", strings.TrimSpace(e.Connector), strings.TrimSpace(e.ExternalEventID)),
		Body:                 strings.TrimSpace(e.Body),
		ReceivedAt:           e.ReceivedAt,
	}
}

func FormatRequestContextForTurn(ctx RequestContext) string {
	lines := []string{
		"External application event",
		"connector: " + strings.TrimSpace(ctx.Connector),
		"event_type: " + strings.TrimSpace(ctx.EventType),
		"request_ref: " + strings.TrimSpace(ctx.RequestID),
		"thread_ref: " + opaqueExternalRefID("thread", ctx.ThreadRef.Connector, ctx.ThreadRef.Kind, ctx.ThreadRef.ExternalID),
		"thread_kind: " + strings.TrimSpace(ctx.ThreadRef.Kind),
		"message_ref: " + opaqueExternalRefID("message", ctx.MessageRef.Connector, ctx.MessageRef.Kind, ctx.MessageRef.ExternalID),
		"sender_ref: " + opaqueExternalRefID("sender", ctx.SenderRef.Connector, ctx.SenderRef.Kind, ctx.SenderRef.ExternalID),
		"sender_kind: " + strings.TrimSpace(ctx.SenderRef.Kind),
		"source_validation: " + strings.TrimSpace(ctx.SourceValidation),
	}
	if display := strings.TrimSpace(ctx.ThreadRef.Display); display != "" {
		lines = append(lines, "thread_display: "+display)
	}
	if display := strings.TrimSpace(ctx.SenderRef.Display); display != "" {
		lines = append(lines, "sender_display: "+display)
	}
	if len(ctx.ResourceRefs) > 0 {
		resourceIDs := make([]string, 0, len(ctx.ResourceRefs))
		for _, ref := range ctx.ResourceRefs {
			resourceIDs = append(resourceIDs, opaqueExternalRefID("resource", ref.Connector, ref.Kind, ref.ExternalID))
		}
		sort.Strings(resourceIDs)
		lines = append(lines, "resource_refs: "+strings.Join(resourceIDs, ","))
	}
	lines = append(lines, "", "text:", strings.TrimSpace(ctx.Body))
	return strings.Join(lines, "\n")
}

func sanitizedExternalRef(ref ExternalRef) ExternalRef {
	return ExternalRef{
		Connector:  strings.TrimSpace(ref.Connector),
		Kind:       strings.TrimSpace(ref.Kind),
		ExternalID: strings.TrimSpace(ref.ExternalID),
		Display:    strings.TrimSpace(ref.Display),
	}
}

func opaqueExternalRefID(prefix string, parts ...string) string {
	trimmed := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed = append(trimmed, strings.TrimSpace(part))
	}
	return stableOpaqueID(prefix, trimmed...)
}
