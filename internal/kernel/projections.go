package kernel

import (
	"encoding/json"
	"errors"
	"strings"
)

func jobOutputPreview(job JobProjection) string {
	switch {
	case strings.TrimSpace(job.Stdout) != "":
		return boundedTimelinePreview(job.Stdout)
	case strings.TrimSpace(job.Stderr) != "":
		return boundedTimelinePreview(job.Stderr)
	case strings.TrimSpace(job.FailureReason) != "":
		return boundedTimelinePreview(job.FailureReason)
	case strings.TrimSpace(job.CancelReason) != "":
		return boundedTimelinePreview(job.CancelReason)
	default:
		return boundedTimelinePreview(job.Receipt)
	}
}

func (k *Kernel) ContextInspection(turnID string) (ContextInspectionResponse, error) {
	turnID = strings.TrimSpace(turnID)
	if turnID == "" {
		return ContextInspectionResponse{}, errors.New("turn id is required")
	}
	events, err := k.loadEvents()
	if err != nil {
		return ContextInspectionResponse{}, err
	}
	for _, event := range events {
		if event.TurnID != turnID || event.Type != "turn.submitted" {
			continue
		}
		if len(event.Data.ToolManifest) == 0 && event.Data.RuntimeContext == nil {
			return ContextInspectionResponse{
				TurnID:            turnID,
				SessionID:         event.SessionID,
				Readiness:         ReadinessNotReady,
				ReadinessReason:   "snapshot_unavailable",
				InputItems:        cloneProjectionInputItems(event.Data.InputItems),
				ModelInputKinds:   cloneStringSlice(event.Data.ModelInputKinds),
				ToolManifest:      cloneToolSpecs(nil),
				SkillCatalog:      cloneSkillCatalogItems(nil),
				SourceSnapshots:   cloneSourceSnapshotDescriptors(event.Data.SourceSnapshots),
				RecalledMemories:  cloneMemoryRecalls(event.Data.RecalledMemories),
				HydratedContexts:  cloneContextHydrationProjections(event.Data.HydratedContexts),
				UnavailableReason: "turn context snapshot was not recorded for this turn",
			}, nil
		}
		return ContextInspectionResponse{
			TurnID:           turnID,
			SessionID:        event.SessionID,
			Readiness:        ReadinessReady,
			InputItems:       cloneProjectionInputItems(event.Data.InputItems),
			ModelInputKinds:  cloneStringSlice(event.Data.ModelInputKinds),
			ToolManifest:     cloneToolSpecs(event.Data.ToolManifest),
			SkillCatalog:     cloneSkillCatalogItems(event.Data.SkillCatalog),
			SourceSnapshots:  cloneSourceSnapshotDescriptors(event.Data.SourceSnapshots),
			RecalledMemories: cloneMemoryRecalls(event.Data.RecalledMemories),
			HydratedContexts: cloneContextHydrationProjections(event.Data.HydratedContexts),
			Runtime:          cloneContextRuntimeSnapshot(event.Data.RuntimeContext),
		}, nil
	}
	return ContextInspectionResponse{}, ErrTurnNotFound
}

func (k *Kernel) AuditReplay(turnID string) (AuditReplayResponse, error) {
	turnID = strings.TrimSpace(turnID)
	if turnID == "" {
		return AuditReplayResponse{}, errors.New("turn id is required")
	}
	events, err := k.loadEvents()
	if err != nil {
		return AuditReplayResponse{}, err
	}
	items := []AuditReplayItem{}
	sessionID := ""
	for _, event := range events {
		if event.TurnID != turnID {
			continue
		}
		if sessionID == "" {
			sessionID = event.SessionID
		}
		items = append(items, auditReplayItem(event))
	}
	if len(items) == 0 {
		return AuditReplayResponse{}, ErrTurnNotFound
	}
	return AuditReplayResponse{
		TurnID:    turnID,
		SessionID: sessionID,
		Readiness: ReadinessReady,
		Items:     items,
	}, nil
}

func inputItemsText(items []InputItem) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		if item.Type == "text" && strings.TrimSpace(item.Text) != "" {
			parts = append(parts, item.Text)
		}
	}
	return strings.Join(parts, "\n")
}

type toolPreview struct {
	Text                string
	Source              string
	Truncated           bool
	OutputTruncation    string
	StdoutOriginalBytes int
	StderrOriginalBytes int
	StdoutOmittedBytes  int
	StderrOmittedBytes  int
	OriginalBytes       int
	ReturnedBytes       int
	ElapsedMs           int64
	FullAvailable       bool
}

func toolResultPreview(content string) toolPreview {
	var payload struct {
		Status              string `json:"status"`
		Text                string `json:"text"`
		Stdout              string `json:"stdout"`
		Stderr              string `json:"stderr"`
		Truncated           bool   `json:"truncated"`
		StdoutTruncated     bool   `json:"stdout_truncated"`
		StderrTruncated     bool   `json:"stderr_truncated"`
		StdoutOriginalBytes int    `json:"stdout_original_bytes"`
		StderrOriginalBytes int    `json:"stderr_original_bytes"`
		StdoutOmittedBytes  int    `json:"stdout_omitted_bytes"`
		StderrOmittedBytes  int    `json:"stderr_omitted_bytes"`
		OutputTruncation    string `json:"output_truncation"`
		OriginalBytes       int    `json:"original_bytes"`
		ReturnedBytes       int    `json:"returned_bytes"`
		ElapsedMs           int64  `json:"elapsed_ms"`
		Error               struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	preview := toolPreview{FullAvailable: strings.TrimSpace(content) != ""}
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		preview.Text = boundedTimelinePreview(content)
		preview.Source = "content"
		return preview
	}
	preview.Truncated = payload.Truncated || payload.StdoutTruncated || payload.StderrTruncated
	preview.OutputTruncation = strings.TrimSpace(payload.OutputTruncation)
	preview.StdoutOriginalBytes = payload.StdoutOriginalBytes
	preview.StderrOriginalBytes = payload.StderrOriginalBytes
	preview.StdoutOmittedBytes = payload.StdoutOmittedBytes
	preview.StderrOmittedBytes = payload.StderrOmittedBytes
	preview.OriginalBytes = payload.OriginalBytes
	preview.ReturnedBytes = payload.ReturnedBytes
	preview.ElapsedMs = payload.ElapsedMs
	switch {
	case strings.TrimSpace(payload.Stdout) != "":
		preview.Text = boundedTimelinePreview(payload.Stdout)
		preview.Source = "stdout"
	case strings.TrimSpace(payload.Stderr) != "":
		preview.Text = boundedTimelinePreview(payload.Stderr)
		preview.Source = "stderr"
	case strings.TrimSpace(payload.Text) != "":
		preview.Text = boundedTimelinePreview(payload.Text)
		preview.Source = "text"
	case strings.TrimSpace(payload.Error.Message) != "":
		preview.Text = boundedTimelinePreview(payload.Error.Message)
		preview.Source = "error"
	default:
		preview.Text = boundedTimelinePreview(payload.Status)
		preview.Source = "status"
	}
	return preview
}

func boundedTimelinePreview(text string) string {
	text = strings.TrimSpace(text)
	runes := []rune(text)
	if len(runes) <= 240 {
		return text
	}
	return string(runes[:240])
}

func cloneProjectionInputItems(items []InputItem) []InputItem {
	out := make([]InputItem, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	return out
}

func cloneMemoryRecalls(items []MemoryRecall) []MemoryRecall {
	out := make([]MemoryRecall, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	return out
}

func localSessionProjection(projection SessionProjection) SessionProjection {
	for i := range projection.Turns {
		projection.Turns[i].InputItems = cloneProjectionInputItems(projection.Turns[i].InputItems)
		projection.Turns[i].RecalledMemories = cloneMemoryRecalls(projection.Turns[i].RecalledMemories)
		projection.Turns[i].FinalMessage = cloneFinalMessage(projection.Turns[i].FinalMessage)
		if projection.Turns[i].Error != nil {
			copied := cloneTurnError(*projection.Turns[i].Error)
			projection.Turns[i].Error = &copied
		}
	}
	for i := range projection.Operations {
		projection.Operations[i] = localOperationProjection(projection.Operations[i])
	}
	for i := range projection.Jobs {
		projection.Jobs[i] = cloneJobProjection(projection.Jobs[i])
	}
	for i := range projection.Approvals {
		projection.Approvals[i] = cloneApprovalProjection(projection.Approvals[i])
	}
	for i := range projection.SandboxReadiness {
		projection.SandboxReadiness[i] = cloneSandboxReadinessProjection(projection.SandboxReadiness[i])
	}
	for i := range projection.Works {
		projection.Works[i] = cloneWorkProjection(projection.Works[i])
	}
	for i := range projection.MemoryCandidates {
		projection.MemoryCandidates[i] = cloneMemoryCandidateProjection(projection.MemoryCandidates[i])
	}
	for i := range projection.Events {
		projection.Events[i].Data = inspectionEventData(projection.Events[i].Data)
	}
	return projection
}

func cloneFinalMessage(message FinalMessage) FinalMessage {
	return message
}

func cloneTurnError(turnError TurnError) TurnError {
	return turnError
}

func cloneWorkProjection(work WorkProjection) WorkProjection {
	return work
}

func cloneJobProjection(job JobProjection) JobProjection {
	return job
}

func cloneApprovalProjection(approval ApprovalProjection) ApprovalProjection {
	return approval
}

func cloneSandboxReadinessProjection(readiness SandboxReadinessProjection) SandboxReadinessProjection {
	return readiness
}

func cloneMemoryCandidateProjection(candidate MemoryCandidateProjection) MemoryCandidateProjection {
	return candidate
}

func cloneToolSpecs(items []ToolSpec) []ToolSpec {
	out := make([]ToolSpec, 0, len(items))
	for _, item := range items {
		next := item
		if item.InputSchema != nil {
			next.InputSchema = make(map[string]interface{}, len(item.InputSchema))
			for key, value := range item.InputSchema {
				next.InputSchema[key] = value
			}
		}
		out = append(out, next)
	}
	return out
}

func cloneSkillCatalogItems(items []SkillCatalogItemProjection) []SkillCatalogItemProjection {
	out := make([]SkillCatalogItemProjection, 0, len(items))
	return append(out, items...)
}

func cloneSourceSnapshotDescriptors(items []SourceSnapshotDescriptor) []SourceSnapshotDescriptor {
	out := make([]SourceSnapshotDescriptor, 0, len(items))
	for _, item := range items {
		next := item
		next.AvailableOperations = append([]string(nil), item.AvailableOperations...)
		next.Diagnostics = append([]SourceDiagnostic(nil), item.Diagnostics...)
		out = append(out, next)
	}
	return out
}

func cloneStringSlice(items []string) []string {
	out := make([]string, 0, len(items))
	return append(out, items...)
}

func cloneContextRuntimeSnapshot(snapshot *ContextRuntimeSnapshot) *ContextRuntimeSnapshot {
	if snapshot == nil {
		return nil
	}
	copied := *snapshot
	copied.Provider = safeProviderStatusForInspection(copied.Provider)
	copied.Limits = append([]RuntimeLimitProjection(nil), snapshot.Limits...)
	return &copied
}

func toInspectionEvent(event StoredEvent) Event {
	return Event{
		EventID:            event.EventID,
		SessionID:          event.SessionID,
		TurnID:             event.TurnID,
		OperationID:        event.OperationID,
		JobID:              event.JobID,
		WorkID:             event.WorkID,
		CandidateID:        event.CandidateID,
		ApprovalID:         event.ApprovalID,
		SandboxReadinessID: event.SandboxReadinessID,
		Type:               event.Type,
		CreatedAt:          event.CreatedAt,
		Data:               inspectionEventData(event.Data),
	}
}

func inspectionEventData(data EventData) EventData {
	next := data
	next.InputItems = cloneProjectionInputItems(data.InputItems)
	next.RecalledMemories = cloneMemoryRecalls(data.RecalledMemories)
	next.HydratedContexts = cloneContextHydrationProjections(data.HydratedContexts)
	next.SourceSnapshots = cloneSourceSnapshotDescriptors(data.SourceSnapshots)
	if data.ContextHydration != nil {
		copied := cloneContextHydrationProjection(*data.ContextHydration)
		next.ContextHydration = &copied
	}
	if data.ToolCall != nil {
		copied := *data.ToolCall
		copied.ProviderToolCallID = redactProviderToolCallID(copied.ProviderToolCallID)
		next.ToolCall = &copied
	}
	if data.ToolResult != nil {
		copied := *data.ToolResult
		copied.ProviderToolCallID = redactProviderToolCallID(copied.ProviderToolCallID)
		next.ToolResult = &copied
	}
	if data.ProviderAttempt != nil {
		copied := *data.ProviderAttempt
		next.ProviderAttempt = &copied
	}
	if data.Final != nil {
		copied := cloneFinalMessage(*data.Final)
		next.Final = &copied
	}
	if data.TurnError != nil {
		copied := cloneTurnError(*data.TurnError)
		next.TurnError = &copied
	}
	if data.Operation != nil {
		copied := *data.Operation
		next.Operation = &copied
	}
	if data.Job != nil {
		copied := cloneJobProjection(*data.Job)
		next.Job = &copied
	}
	if data.Approval != nil {
		copied := cloneApprovalProjection(*data.Approval)
		next.Approval = &copied
	}
	if data.SandboxReadiness != nil {
		copied := cloneSandboxReadinessProjection(*data.SandboxReadiness)
		next.SandboxReadiness = &copied
	}
	if data.KernelObservationDelivery != nil {
		copied := KernelObservationDeliveryProjection{
			ObservationEventIDs: append([]string(nil), data.KernelObservationDelivery.ObservationEventIDs...),
			ModelInputKind:      data.KernelObservationDelivery.ModelInputKind,
		}
		next.KernelObservationDelivery = &copied
	}
	if data.Work != nil {
		copied := cloneWorkProjection(*data.Work)
		next.Work = &copied
	}
	if data.MemoryCandidate != nil {
		copied := cloneMemoryCandidateProjection(*data.MemoryCandidate)
		next.MemoryCandidate = &copied
	}
	if data.ReplacementMemoryCandidate != nil {
		copied := cloneMemoryCandidateProjection(*data.ReplacementMemoryCandidate)
		next.ReplacementMemoryCandidate = &copied
	}
	return next
}

func redactProviderToolCallID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	return safeInspectionToken(id, "provider_tool_call_id_unavailable")
}

func auditReplayItem(event StoredEvent) AuditReplayItem {
	item := AuditReplayItem{
		EventID:     event.EventID,
		EventType:   event.Type,
		TurnID:      event.TurnID,
		OperationID: event.OperationID,
		JobID:       event.JobID,
		CreatedAt:   event.CreatedAt,
	}
	data := inspectionEventData(event.Data)
	switch event.Type {
	case "turn.submitted":
		item.ModelInputKinds = append([]string(nil), data.ModelInputKinds...)
		item.ProviderContextKinds = append([]string(nil), data.ModelInputKinds...)
	case "tool.call":
		if data.ToolCall != nil {
			item.Tool = data.ToolCall.Tool
		}
	case "tool.result":
		if data.ToolResult != nil {
			item.Tool = data.ToolResult.Tool
			item.ToolStatus = data.ToolResult.Status
			preview := toolResultPreview(data.ToolResult.Content)
			item.OutputPreview = preview.Text
			item.OutputTruncated = preview.Truncated
		}
	case "operation.running", "operation.completed", "operation.failed", "operation.blocked", "operation.interrupted":
		if data.Operation != nil {
			item.Tool = data.Operation.Tool
			item.ToolStatus = data.Operation.Status
			item.OutputTruncated = data.Operation.StdoutTruncated || data.Operation.StderrTruncated
			item.OutputTruncation = data.Operation.OutputTruncation
			item.StdoutOriginalBytes = data.Operation.StdoutOriginalBytes
			item.StderrOriginalBytes = data.Operation.StderrOriginalBytes
			item.StdoutOmittedBytes = data.Operation.StdoutOmittedBytes
			item.StderrOmittedBytes = data.Operation.StderrOmittedBytes
			switch {
			case strings.TrimSpace(data.Operation.Stdout) != "":
				item.OutputPreview = boundedTimelinePreview(data.Operation.Stdout)
			case strings.TrimSpace(data.Operation.Stderr) != "":
				item.OutputPreview = boundedTimelinePreview(data.Operation.Stderr)
			}
		}
	case "job.started", "job.completed", "job.failed", "job.cancelled":
		if data.Job != nil {
			item.Tool = data.Job.Tool
			item.ToolStatus = data.Job.Status
			item.OutputPreview = jobOutputPreview(*data.Job)
		}
	case "approval.requested", "approval.approved", "approval.denied", "approval.expired":
		if data.Approval != nil {
			item.Tool = data.Approval.Tool
			item.ToolStatus = data.Approval.Status
			item.OutputPreview = boundedTimelinePreview(data.Approval.Effect.CommandPreview)
			if data.Approval.BlockedReason != "" {
				item.ErrorCode = data.Approval.BlockedReason
			}
		}
	case "sandbox.ready", "sandbox.unavailable":
		if data.SandboxReadiness != nil {
			item.ToolStatus = data.SandboxReadiness.Status
			item.OutputPreview = boundedTimelinePreview(data.SandboxReadiness.SandboxProfile)
			if data.SandboxReadiness.UnavailableReason != "" {
				item.ErrorCode = data.SandboxReadiness.UnavailableReason
			}
		}
	case "kernel.observation.delivered":
		if data.KernelObservationDelivery != nil {
			item.ToolStatus = "delivered"
			item.OutputPreview = boundedTimelinePreview(strings.Join(data.KernelObservationDelivery.ObservationEventIDs, "\n"))
		}
	case "model.provider_attempt", "model.provider_repair":
		if data.ProviderAttempt != nil {
			item.ToolStatus = data.ProviderAttempt.Status
			item.ErrorCode = data.ProviderAttempt.ReasonCode
			item.ErrorMessage = data.ProviderAttempt.Message
			item.OutputPreview = boundedTimelinePreview(data.ProviderAttempt.RepairKind)
		}
	case "model.final":
		if data.Final != nil {
			item.Usage = data.Final.Usage
		}
	case "assistant.interrupted":
		item.ToolStatus = "interrupted"
		if data.TurnInterruption != nil {
			item.OutputPreview = boundedTimelinePreview(data.TurnInterruption.Reason)
		}
	case "turn.failed":
		if data.TurnError != nil {
			item.ErrorCode = data.TurnError.Code
			item.ErrorMessage = data.TurnError.Message
		}
	}
	return item
}
