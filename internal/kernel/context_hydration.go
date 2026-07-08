package kernel

import (
	"strings"
	"time"

	"genesis/internal/kernel/resource"
)

const (
	contextHydrationAdmissionAdmitted = "admitted"
	contextHydrationAdmissionRefused  = "refused"
)

func (k *Kernel) AdmitContextResource(req ContextHydrationAdmissionRequest) (ContextHydrationProjection, error) {
	now := k.clock()
	events, err := k.loadEvents()
	if err != nil {
		return ContextHydrationProjection{}, err
	}
	projection := k.evaluateContextHydrationAdmission(req, events, now)
	eventType := "context.hydration.refused"
	if projection.AdmissionResult == contextHydrationAdmissionAdmitted {
		eventType = "context.hydration.admitted"
	}
	eventTurnID := ""
	if projection.AdmissionResult == contextHydrationAdmissionAdmitted {
		eventTurnID = projection.TurnID
	}
	if err := k.appendEvent(StoredEvent{
		EventID:   newID("evt", now),
		SessionID: projection.SessionID,
		TurnID:    eventTurnID,
		Type:      eventType,
		CreatedAt: now,
		Data: EventData{
			ContextHydration: &projection,
		},
	}); err != nil {
		return ContextHydrationProjection{}, err
	}
	return cloneContextHydrationProjection(projection), nil
}

func (k *Kernel) evaluateContextHydrationAdmission(req ContextHydrationAdmissionRequest, events []StoredEvent, now time.Time) ContextHydrationProjection {
	projection := ContextHydrationProjection{
		SessionID:       strings.TrimSpace(req.SessionID),
		TurnID:          strings.TrimSpace(req.TurnID),
		AdmissionResult: contextHydrationAdmissionRefused,
		SourceOwner:     strings.TrimSpace(req.SourceOwner),
		ResourceRef:     strings.TrimSpace(req.ResourceRef),
		InputKind:       ModelInputKindHydratedContext,
		Reason:          strings.TrimSpace(req.Reason),
		DerivationRefs:  normalizedHydrationDerivationRefs(req.DerivationRefs),
		CreatedAt:       now,
	}
	if projection.SessionID == "" {
		projection.RefusalReasonClass = "invalid_session_id"
		return projection
	}
	if err := validateKernelControlToken("session_id", projection.SessionID); err != nil {
		projection.RefusalReasonClass = "invalid_session_id"
		return projection
	}
	if projection.TurnID != "" {
		projection.RefusalReasonClass = "scope_violation"
		return projection
	}
	if projection.SourceOwner == "" {
		projection.RefusalReasonClass = "invalid_source_owner"
		return projection
	}
	if err := validateKernelControlToken("source_owner", projection.SourceOwner); err != nil {
		projection.RefusalReasonClass = "invalid_source_owner"
		return projection
	}
	ref, err := resource.NormalizeRef(projection.ResourceRef)
	if err != nil {
		projection.RefusalReasonClass = "invalid_resource_ref"
		return projection
	}
	projection.ResourceRef = ref
	if req.MaxVisibleBytes < 0 {
		projection.RefusalReasonClass = "invalid_visible_byte_cap"
		return projection
	}
	metadata, err := k.resourceRegistry.Metadata(ref)
	if err != nil {
		projection.RefusalReasonClass = "resource_not_found"
		return projection
	}
	projection.ResourceRef = metadata.Ref
	projection.ResourceHash = metadata.ResourceHash
	projection.MimeType = metadata.MimeType
	projection.OriginalBytes = metadata.OriginalBytes
	if !metadata.TextReadable {
		projection.RefusalReasonClass = "unsupported_mime_type"
		return projection
	}
	limit := req.MaxVisibleBytes
	if limit == 0 {
		limit = resource.DefaultReadLimitBytes
	}
	if limit > resource.MaxReadLimitBytes {
		projection.RefusalReasonClass = "visible_byte_cap_too_large"
		return projection
	}
	if req.MaxVisibleBytes == 0 && metadata.OriginalBytes > resource.DefaultReadLimitBytes {
		projection.RefusalReasonClass = "max_visible_bytes_required"
		return projection
	}
	readReq, code, err := resource.NormalizeReadRequest(ref, nil, &limit)
	if err != nil {
		projection.RefusalReasonClass = code
		return projection
	}
	result, err := k.resourceRegistry.Read(readReq)
	if err != nil {
		projection.RefusalReasonClass = "resource_read_failed"
		return projection
	}
	projection.AdmissionResult = contextHydrationAdmissionAdmitted
	projection.HydrationID = newID("hydration", now)
	projection.VisibleBytes = result.ReturnedBytes
	projection.NextOffsetBytes = result.NextOffsetBytes
	projection.Truncated = result.Truncated
	projection.RefusalReasonClass = ""
	return projection
}

type providerHydratedContextFragment struct {
	InputKind   string
	VisibleText string
}

func (k *Kernel) providerHydratedContextFragments(hydratedContexts []ContextHydrationProjection) []providerHydratedContextFragment {
	out := make([]providerHydratedContextFragment, 0, len(hydratedContexts))
	for _, hydration := range hydratedContexts {
		if hydration.AdmissionResult != contextHydrationAdmissionAdmitted || hydration.InputKind != ModelInputKindHydratedContext || hydration.VisibleBytes <= 0 {
			continue
		}
		readReq, _, err := resource.NormalizeReadRequest(hydration.ResourceRef, nil, &hydration.VisibleBytes)
		if err != nil {
			continue
		}
		result, err := k.resourceRegistry.Read(readReq)
		if err != nil {
			continue
		}
		out = append(out, providerHydratedContextFragment{
			InputKind:   hydration.InputKind,
			VisibleText: result.Text,
		})
	}
	return out
}

func normalizedHydrationDerivationRefs(refs []string) []string {
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		out = append(out, ref)
	}
	return out
}

func pendingContextHydrationsForNewTurn(events []StoredEvent, sessionID string, turnID string) []ContextHydrationProjection {
	out := []ContextHydrationProjection{}
	for i, event := range events {
		if event.SessionID != sessionID || event.Type != "context.hydration.admitted" || event.Data.ContextHydration == nil {
			continue
		}
		hydration := *event.Data.ContextHydration
		if hydration.AdmissionResult != contextHydrationAdmissionAdmitted {
			continue
		}
		if hydration.TurnID != "" && hydration.TurnID != turnID {
			continue
		}
		if hydration.TurnID == "" && sessionHasSubmittedTurnAfter(events, sessionID, i) {
			continue
		}
		hydration.TurnID = turnID
		out = append(out, cloneContextHydrationProjection(hydration))
	}
	return out
}

func sessionHasSubmittedTurnAfter(events []StoredEvent, sessionID string, eventIndex int) bool {
	for i := eventIndex + 1; i < len(events); i++ {
		if events[i].SessionID == sessionID && events[i].Type == "turn.submitted" {
			return true
		}
	}
	return false
}

func cloneContextHydrationProjection(in ContextHydrationProjection) ContextHydrationProjection {
	out := in
	out.DerivationRefs = append([]string(nil), in.DerivationRefs...)
	if in.NextOffsetBytes != nil {
		next := *in.NextOffsetBytes
		out.NextOffsetBytes = &next
	}
	return out
}

func cloneContextHydrationProjections(items []ContextHydrationProjection) []ContextHydrationProjection {
	out := make([]ContextHydrationProjection, 0, len(items))
	for _, item := range items {
		out = append(out, cloneContextHydrationProjection(item))
	}
	return out
}
