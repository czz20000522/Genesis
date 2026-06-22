package kernel

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func (k *Kernel) UITimeline(sessionID string) (UITimelineResponse, error) {
	session, err := k.Session(sessionID)
	if err != nil {
		return UITimelineResponse{}, err
	}
	items := make([]UITimelineItem, 0, len(session.Events))
	toolItemByEventID := map[string]int{}
	toolOrdinalByTurn := map[string]int{}
	messageOrdinalByTurn := map[string]int{}
	for _, event := range session.Events {
		switch event.Type {
		case "turn.submitted":
			if len(event.Data.InputItems) == 0 {
				continue
			}
			item := UITimelineItem{
				ItemID:    timelineItemID(event.TurnID, "user", messageOrdinalByTurn[event.TurnID]),
				TurnID:    event.TurnID,
				Kind:      "user_message",
				Text:      redactEvidenceText(inputItemsText(event.Data.InputItems)),
				CreatedAt: event.CreatedAt,
			}
			messageOrdinalByTurn[event.TurnID]++
			items = append(items, item)
		case "tool.call":
			if event.Data.ToolCall == nil {
				continue
			}
			ordinal := toolOrdinalByTurn[event.TurnID]
			item := UITimelineItem{
				ItemID:    timelineItemID(event.TurnID, "tool", ordinal),
				TurnID:    event.TurnID,
				Kind:      "tool",
				Status:    "running",
				Tool:      event.Data.ToolCall.Tool,
				CreatedAt: event.CreatedAt,
			}
			toolOrdinalByTurn[event.TurnID]++
			toolItemByEventID[event.EventID] = len(items)
			items = append(items, item)
		case "tool.result":
			if event.Data.ToolResult == nil {
				continue
			}
			idx, ok := toolItemByEventID[event.Data.ToolResult.ForEventID]
			if !ok {
				ordinal := toolOrdinalByTurn[event.TurnID]
				idx = len(items)
				items = append(items, UITimelineItem{
					ItemID:    timelineItemID(event.TurnID, "tool", ordinal),
					TurnID:    event.TurnID,
					Kind:      "tool",
					Tool:      event.Data.ToolResult.Tool,
					CreatedAt: event.CreatedAt,
				})
				toolOrdinalByTurn[event.TurnID]++
			}
			preview := toolResultPreview(event.Data.ToolResult.Content)
			items[idx].Status = event.Data.ToolResult.Status
			items[idx].Tool = event.Data.ToolResult.Tool
			items[idx].OutputPreview = preview.Text
			items[idx].OutputSource = preview.Source
			items[idx].OutputTruncated = preview.Truncated
			items[idx].FullOutputAvailable = preview.FullAvailable
			items[idx].UpdatedAt = event.CreatedAt
		case "model.final":
			if event.Data.Final == nil {
				continue
			}
			item := UITimelineItem{
				ItemID:    timelineItemID(event.TurnID, "assistant", messageOrdinalByTurn[event.TurnID]),
				TurnID:    event.TurnID,
				Kind:      "assistant_message",
				Text:      redactEvidenceText(event.Data.Final.Text),
				CreatedAt: event.CreatedAt,
			}
			messageOrdinalByTurn[event.TurnID]++
			items = append(items, item)
		case "turn.failed":
			if event.Data.TurnError == nil {
				continue
			}
			text := event.Data.TurnError.Code
			if event.Data.TurnError.Message != "" {
				text = event.Data.TurnError.Message
			}
			items = append(items, UITimelineItem{
				ItemID:    timelineItemID(event.TurnID, "notice", messageOrdinalByTurn[event.TurnID]),
				TurnID:    event.TurnID,
				Kind:      "notice",
				Status:    "failed",
				Text:      redactEvidenceText(text),
				CreatedAt: event.CreatedAt,
			})
			messageOrdinalByTurn[event.TurnID]++
		}
	}
	return UITimelineResponse{
		SessionID: session.SessionID,
		Status:    "ok",
		Items:     items,
	}, nil
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
				Status:            "snapshot_unavailable",
				InputItems:        redactInputItems(event.Data.InputItems),
				ModelInputKinds:   append([]string(nil), event.Data.ModelInputKinds...),
				RecalledMemories:  redactMemoryRecalls(event.Data.RecalledMemories),
				UnavailableReason: "turn context snapshot was not recorded for this turn",
			}, nil
		}
		return ContextInspectionResponse{
			TurnID:           turnID,
			SessionID:        event.SessionID,
			Status:           "ok",
			InputItems:       redactInputItems(event.Data.InputItems),
			ModelInputKinds:  append([]string(nil), event.Data.ModelInputKinds...),
			ToolManifest:     cloneToolSpecs(event.Data.ToolManifest),
			SkillCatalog:     cloneSkillCatalogItems(event.Data.SkillCatalog),
			RecalledMemories: redactMemoryRecalls(event.Data.RecalledMemories),
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
		Status:    "ok",
		Items:     items,
	}, nil
}

func timelineItemID(turnID string, kind string, ordinal int) string {
	return fmt.Sprintf("%s:%s:%d", turnID, kind, ordinal)
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
	Text          string
	Source        string
	Truncated     bool
	FullAvailable bool
}

func toolResultPreview(content string) toolPreview {
	var payload struct {
		Status          string `json:"status"`
		Stdout          string `json:"stdout"`
		Stderr          string `json:"stderr"`
		StdoutTruncated bool   `json:"stdout_truncated"`
		StderrTruncated bool   `json:"stderr_truncated"`
		Error           struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	preview := toolPreview{FullAvailable: strings.TrimSpace(content) != ""}
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		preview.Text = boundedTimelinePreview(content)
		preview.Source = "content"
		return preview
	}
	preview.Truncated = payload.StdoutTruncated || payload.StderrTruncated
	switch {
	case strings.TrimSpace(payload.Stdout) != "":
		preview.Text = boundedTimelinePreview(payload.Stdout)
		preview.Source = "stdout"
	case strings.TrimSpace(payload.Stderr) != "":
		preview.Text = boundedTimelinePreview(payload.Stderr)
		preview.Source = "stderr"
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
	text = redactEvidenceText(strings.TrimSpace(text))
	runes := []rune(text)
	if len(runes) <= 240 {
		return text
	}
	return string(runes[:240])
}

func redactInputItems(items []InputItem) []InputItem {
	out := make([]InputItem, 0, len(items))
	for _, item := range items {
		next := item
		next.Text = redactEvidenceText(next.Text)
		out = append(out, next)
	}
	return out
}

func redactMemoryRecalls(items []MemoryRecall) []MemoryRecall {
	out := make([]MemoryRecall, 0, len(items))
	for _, item := range items {
		next := item
		next.Text = redactEvidenceText(next.Text)
		out = append(out, next)
	}
	return out
}

func redactSessionProjection(projection SessionProjection) SessionProjection {
	for i := range projection.Turns {
		projection.Turns[i].InputItems = redactInputItems(projection.Turns[i].InputItems)
		projection.Turns[i].RecalledMemories = redactMemoryRecalls(projection.Turns[i].RecalledMemories)
		projection.Turns[i].FinalMessage = redactFinalMessage(projection.Turns[i].FinalMessage)
		if projection.Turns[i].Error != nil {
			copied := redactTurnError(*projection.Turns[i].Error)
			projection.Turns[i].Error = &copied
		}
	}
	for i := range projection.Operations {
		projection.Operations[i] = redactOperationEvidence(projection.Operations[i])
	}
	for i := range projection.Works {
		projection.Works[i] = redactWorkProjection(projection.Works[i])
	}
	for i := range projection.MemoryCandidates {
		projection.MemoryCandidates[i] = redactMemoryCandidateProjection(projection.MemoryCandidates[i])
	}
	for i := range projection.Events {
		projection.Events[i].Data = inspectionEventData(projection.Events[i].Data)
	}
	return projection
}

func redactFinalMessage(message FinalMessage) FinalMessage {
	message.Text = redactEvidenceText(message.Text)
	return message
}

func redactTurnError(turnError TurnError) TurnError {
	turnError.Message = redactEvidenceText(turnError.Message)
	return turnError
}

func redactWorkProjection(work WorkProjection) WorkProjection {
	work.Title = redactEvidenceText(work.Title)
	work.SourceRef = redactEvidenceText(work.SourceRef)
	work.CancelReason = redactEvidenceText(work.CancelReason)
	work.CancelEvidenceRef = redactEvidenceText(work.CancelEvidenceRef)
	return work
}

func redactMemoryCandidateProjection(candidate MemoryCandidateProjection) MemoryCandidateProjection {
	candidate.Text = redactEvidenceText(candidate.Text)
	candidate.SourceRef = redactEvidenceText(candidate.SourceRef)
	candidate.ApprovalReason = redactEvidenceText(candidate.ApprovalReason)
	candidate.ApprovalEvidenceRef = redactEvidenceText(candidate.ApprovalEvidenceRef)
	candidate.RejectionReason = redactEvidenceText(candidate.RejectionReason)
	candidate.RejectionEvidenceRef = redactEvidenceText(candidate.RejectionEvidenceRef)
	candidate.SupersessionReason = redactEvidenceText(candidate.SupersessionReason)
	candidate.SupersessionEvidenceRef = redactEvidenceText(candidate.SupersessionEvidenceRef)
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
	return append([]SkillCatalogItemProjection(nil), items...)
}

func cloneContextRuntimeSnapshot(snapshot *ContextRuntimeSnapshot) *ContextRuntimeSnapshot {
	if snapshot == nil {
		return nil
	}
	copied := *snapshot
	copied.Provider = safeProviderStatusForInspection(copied.Provider)
	return &copied
}

func toInspectionEvent(event StoredEvent) Event {
	return Event{
		EventID:     event.EventID,
		SessionID:   event.SessionID,
		TurnID:      event.TurnID,
		OperationID: event.OperationID,
		WorkID:      event.WorkID,
		CandidateID: event.CandidateID,
		Type:        event.Type,
		CreatedAt:   event.CreatedAt,
		Data:        inspectionEventData(event.Data),
	}
}

func inspectionEventData(data EventData) EventData {
	next := data
	next.InputItems = redactInputItems(data.InputItems)
	next.RecalledMemories = redactMemoryRecalls(data.RecalledMemories)
	if data.ToolCall != nil {
		copied := *data.ToolCall
		copied.ProviderToolCallID = redactProviderToolCallID(copied.ProviderToolCallID)
		copied.Arguments = redactEvidenceText(copied.Arguments)
		next.ToolCall = &copied
	}
	if data.ToolResult != nil {
		copied := *data.ToolResult
		copied.ProviderToolCallID = redactProviderToolCallID(copied.ProviderToolCallID)
		copied.Content = redactEvidenceText(copied.Content)
		next.ToolResult = &copied
	}
	if data.Final != nil {
		copied := redactFinalMessage(*data.Final)
		next.Final = &copied
	}
	if data.TurnError != nil {
		copied := redactTurnError(*data.TurnError)
		next.TurnError = &copied
	}
	if data.Operation != nil {
		copied := *data.Operation
		copied.CWD = redactEvidenceText(copied.CWD)
		copied.Command = redactEvidenceText(copied.Command)
		copied.Stdout = redactEvidenceText(copied.Stdout)
		copied.Stderr = redactEvidenceText(copied.Stderr)
		copied.BlockedReason = redactEvidenceText(copied.BlockedReason)
		copied.InfrastructureReason = redactEvidenceText(copied.InfrastructureReason)
		next.Operation = &copied
	}
	if data.Work != nil {
		copied := redactWorkProjection(*data.Work)
		next.Work = &copied
	}
	if data.MemoryCandidate != nil {
		copied := redactMemoryCandidateProjection(*data.MemoryCandidate)
		next.MemoryCandidate = &copied
	}
	if data.ReplacementMemoryCandidate != nil {
		copied := redactMemoryCandidateProjection(*data.ReplacementMemoryCandidate)
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
	case "operation.running", "operation.completed", "operation.failed", "operation.blocked":
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
	case "model.final":
		if data.Final != nil {
			item.Usage = data.Final.Usage
		}
	case "turn.failed":
		if data.TurnError != nil {
			item.ErrorCode = data.TurnError.Code
			item.ErrorMessage = data.TurnError.Message
		}
	}
	return item
}
