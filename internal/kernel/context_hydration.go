package kernel

import (
	"strings"
	"time"

	"genesis/internal/kernel/resource"
)

const (
	contextHydrationStatusAccepted = "accepted"
	contextHydrationStatusRejected = "rejected"
)

func (k *Kernel) AdmitContextResource(req ContextHydrationAdmissionRequest) (ContextHydrationProjection, error) {
	now := k.clock()
	events, err := k.loadEvents()
	if err != nil {
		return ContextHydrationProjection{}, err
	}
	projection := k.evaluateContextHydrationAdmission(req, events, now)
	eventType := "context.hydration.rejected"
	if projection.Status == contextHydrationStatusAccepted {
		eventType = "context.hydration.accepted"
	}
	eventTurnID := ""
	if projection.Status == contextHydrationStatusAccepted {
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
		SessionID:      strings.TrimSpace(req.SessionID),
		TurnID:         strings.TrimSpace(req.TurnID),
		Status:         contextHydrationStatusRejected,
		SourceOwner:    strings.TrimSpace(req.SourceOwner),
		ResourceRef:    strings.TrimSpace(req.ResourceRef),
		InputKind:      ModelInputKindHydratedContext,
		Reason:         strings.TrimSpace(req.Reason),
		DerivationRefs: normalizedHydrationDerivationRefs(req.DerivationRefs),
		CreatedAt:      now,
	}
	if projection.SessionID == "" {
		projection.RejectedReason = "invalid_session_id"
		return projection
	}
	if err := validateKernelControlToken("session_id", projection.SessionID); err != nil {
		projection.RejectedReason = "invalid_session_id"
		return projection
	}
	if projection.TurnID != "" {
		if err := validateKernelControlToken("turn_id", projection.TurnID); err != nil {
			projection.RejectedReason = "invalid_turn_id"
			return projection
		}
		if !turnBelongsToSession(events, projection.TurnID, projection.SessionID) {
			projection.RejectedReason = "scope_mismatch"
			return projection
		}
	}
	if projection.SourceOwner == "" {
		projection.RejectedReason = "invalid_source_owner"
		return projection
	}
	if err := validateKernelControlToken("source_owner", projection.SourceOwner); err != nil {
		projection.RejectedReason = "invalid_source_owner"
		return projection
	}
	ref, err := resource.NormalizeRef(projection.ResourceRef)
	if err != nil {
		projection.RejectedReason = "invalid_resource_ref"
		return projection
	}
	projection.ResourceRef = ref
	if req.MaxVisibleBytes < 0 {
		projection.RejectedReason = "invalid_visible_byte_cap"
		return projection
	}
	metadata, err := k.resourceRegistry.Metadata(ref)
	if err != nil {
		projection.RejectedReason = "resource_not_found"
		return projection
	}
	projection.ResourceRef = metadata.Ref
	projection.ResourceHash = metadata.ResourceHash
	projection.MimeType = metadata.MimeType
	projection.OriginalBytes = metadata.OriginalBytes
	if !metadata.TextReadable {
		projection.RejectedReason = "unsupported_mime_type"
		return projection
	}
	limit := req.MaxVisibleBytes
	if limit == 0 {
		limit = resource.DefaultReadLimitBytes
	}
	if limit > resource.MaxReadLimitBytes {
		projection.RejectedReason = "visible_byte_cap_too_large"
		return projection
	}
	if req.MaxVisibleBytes == 0 && metadata.OriginalBytes > resource.DefaultReadLimitBytes {
		projection.RejectedReason = "max_visible_bytes_required"
		return projection
	}
	readReq, code, err := resource.NormalizeReadRequest(ref, nil, &limit)
	if err != nil {
		projection.RejectedReason = code
		return projection
	}
	result, err := k.resourceRegistry.Read(readReq)
	if err != nil {
		projection.RejectedReason = "resource_read_failed"
		return projection
	}
	projection.Status = contextHydrationStatusAccepted
	projection.HydrationID = newID("hydration", now)
	projection.VisibleText = result.Text
	projection.VisibleBytes = result.ReturnedBytes
	projection.Truncated = result.Truncated
	projection.RejectedReason = ""
	return projection
}

func turnBelongsToSession(events []StoredEvent, turnID string, sessionID string) bool {
	turnID = strings.TrimSpace(turnID)
	sessionID = strings.TrimSpace(sessionID)
	if turnID == "" || sessionID == "" {
		return false
	}
	for _, event := range events {
		if event.Type == "turn.submitted" && event.TurnID == turnID && event.SessionID == sessionID {
			return true
		}
	}
	return false
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
		if event.SessionID != sessionID || event.Type != "context.hydration.accepted" || event.Data.ContextHydration == nil {
			continue
		}
		hydration := *event.Data.ContextHydration
		if hydration.Status != contextHydrationStatusAccepted {
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
	return out
}

func cloneContextHydrationProjections(items []ContextHydrationProjection) []ContextHydrationProjection {
	out := make([]ContextHydrationProjection, 0, len(items))
	for _, item := range items {
		out = append(out, cloneContextHydrationProjection(item))
	}
	return out
}
