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
