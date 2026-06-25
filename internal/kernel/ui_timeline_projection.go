package kernel

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrTimelineDetailNotFound = errors.New("timeline detail not found")

func (k *Kernel) UITimeline(sessionID string) (UITimelineResponse, error) {
	session, err := k.Session(sessionID)
	if err != nil {
		return UITimelineResponse{}, err
	}
	now := timelineNow(k)
	turns := map[string]*timelineTurnBuilder{}
	turnOrder := []string{}
	for _, event := range session.Events {
		turn := ensureTimelineTurn(turns, &turnOrder, session.SessionID, event)
		switch event.Type {
		case "turn.submitted":
			if len(event.Data.InputItems) == 0 {
				continue
			}
			turn.appendMessage("user_message", redactEvidenceText(inputItemsText(event.Data.InputItems)), event.CreatedAt)
		case "tool.call":
			if event.Data.ToolCall == nil {
				continue
			}
			turn.addToolCall(event.EventID, event.Data.ToolCall.Tool, event.CreatedAt)
		case "tool.result":
			if event.Data.ToolResult == nil {
				continue
			}
			turn.applyToolResult(*event.Data.ToolResult, event.CreatedAt)
		case "job.started", "job.output", "job.completed", "job.failed", "job.cancelled":
			if event.Data.Job == nil {
				continue
			}
			turn.applyJob(event.Type, *event.Data.Job, event.CreatedAt)
		case "model.final":
			if event.Data.Final == nil {
				continue
			}
			turn.markTerminal("completed", event.CreatedAt)
			turn.appendMessage("assistant_message", redactEvidenceText(event.Data.Final.Text), event.CreatedAt)
		case "assistant.interrupted":
			text := "turn interrupted"
			if event.Data.TurnInterruption != nil && strings.TrimSpace(event.Data.TurnInterruption.Reason) != "" {
				text = event.Data.TurnInterruption.Reason
			}
			turn.markTerminal("interrupted", event.CreatedAt)
			turn.appendProcessingNotice("notice", "interrupted", redactEvidenceText(text), event.CreatedAt)
		case "context.compaction.started":
			turn.appendCompactionNotice("running", "正在压缩上下文", event.CreatedAt)
		case "context.compaction.completed":
			turn.appendCompactionNotice("completed", "上下文已压缩", event.CreatedAt)
		case "context.compaction.failed":
			turn.appendCompactionNotice("failed", "上下文压缩失败，将在后续消息重试", event.CreatedAt)
		case "turn.failed":
			if event.Data.TurnError == nil {
				continue
			}
			text := event.Data.TurnError.Code
			if event.Data.TurnError.Message != "" {
				text = event.Data.TurnError.Message
			}
			turn.markTerminal("failed", event.CreatedAt)
			turn.appendProcessingNotice("notice", "failed", redactEvidenceText(text), event.CreatedAt)
		}
	}
	items := make([]UITimelineItem, 0, len(turnOrder))
	for _, key := range turnOrder {
		items = append(items, turns[key].finalize(now))
	}
	return UITimelineResponse{
		SessionID: session.SessionID,
		Status:    "ok",
		Items:     items,
	}, nil
}

func (k *Kernel) UITimelineDetail(sessionID string, detailRef string) (UITimelineDetailResponse, error) {
	detailRef = strings.TrimSpace(detailRef)
	if detailRef == "" {
		return UITimelineDetailResponse{}, errors.New("timeline detail ref is required")
	}
	timeline, err := k.UITimeline(sessionID)
	if err != nil {
		return UITimelineDetailResponse{}, err
	}
	item, ok := findUITimelineDetailItem(timeline.Items, detailRef)
	if !ok {
		return UITimelineDetailResponse{}, ErrTimelineDetailNotFound
	}
	return UITimelineDetailResponse{
		SessionID: timeline.SessionID,
		Status:    "ok",
		DetailRef: detailRef,
		Item:      item,
	}, nil
}

func findUITimelineDetailItem(items []UITimelineItem, detailRef string) (UITimelineItem, bool) {
	for _, item := range items {
		if item.ItemID == detailRef || item.DetailRef == detailRef {
			return item, true
		}
		if nested, ok := findUITimelineDetailItem(item.Children, detailRef); ok {
			return nested, true
		}
	}
	return UITimelineItem{}, false
}

type timelineTurnBuilder struct {
	item                  UITimelineItem
	startedAt             time.Time
	terminalAt            time.Time
	terminalStatus        string
	messageOrdinal        int
	processingIndex       int
	processingInitialized bool
	toolOrdinal           int
	toolGroupByCallEvent  map[string]int
	toolGroupByJobID      map[string]int
	seenJobIDs            map[string]bool
	pendingAction         bool
}

func ensureTimelineTurn(turns map[string]*timelineTurnBuilder, order *[]string, sessionID string, event EventProjection) *timelineTurnBuilder {
	key := strings.TrimSpace(event.TurnID)
	if key == "" {
		key = "session:" + sessionID
	}
	if existing := turns[key]; existing != nil {
		if existing.startedAt.IsZero() || event.CreatedAt.Before(existing.startedAt) {
			existing.startedAt = event.CreatedAt
			existing.item.CreatedAt = event.CreatedAt
		}
		return existing
	}
	turnID := event.TurnID
	startedAt := event.CreatedAt
	itemID := timelineItemID(key, "turn", 0)
	builder := &timelineTurnBuilder{
		item: UITimelineItem{
			ItemID:    itemID,
			TurnID:    turnID,
			Kind:      "turn",
			Status:    "running",
			CreatedAt: startedAt,
		},
		startedAt:            startedAt,
		processingIndex:      -1,
		toolGroupByCallEvent: map[string]int{},
		toolGroupByJobID:     map[string]int{},
		seenJobIDs:           map[string]bool{},
	}
	turns[key] = builder
	*order = append(*order, key)
	return builder
}

func (b *timelineTurnBuilder) appendMessage(kind string, text string, createdAt time.Time) {
	b.item.Children = append(b.item.Children, UITimelineItem{
		ItemID:    timelineItemID(b.item.ItemID, kind, b.messageOrdinal),
		TurnID:    b.item.TurnID,
		Kind:      kind,
		Text:      text,
		CreatedAt: createdAt,
	})
	b.messageOrdinal++
}

func (b *timelineTurnBuilder) processingGroup() *UITimelineItem {
	if b.processingInitialized {
		return &b.item.Children[b.processingIndex]
	}
	createdAt := b.startedAt
	if createdAt.IsZero() {
		createdAt = b.item.CreatedAt
	}
	b.item.Children = append(b.item.Children, UITimelineItem{
		ItemID:          timelineItemID(b.item.ItemID, "processing", 0),
		TurnID:          b.item.TurnID,
		Kind:            "processing_group",
		Status:          "running",
		DetailRef:       timelineItemID(b.item.ItemID, "processing_detail", 0),
		DetailAvailable: true,
		DefaultOpen:     true,
		CreatedAt:       createdAt,
	})
	b.processingIndex = len(b.item.Children) - 1
	b.processingInitialized = true
	return &b.item.Children[b.processingIndex]
}

func (b *timelineTurnBuilder) addToolCall(callEventID string, tool string, createdAt time.Time) {
	processing := b.processingGroup()
	ordinal := b.toolOrdinal
	group := UITimelineItem{
		ItemID:          timelineItemID(b.item.ItemID, "tool_group", ordinal),
		TurnID:          b.item.TurnID,
		Kind:            "tool_group",
		Status:          "running",
		Tool:            tool,
		DetailRef:       timelineItemID(b.item.ItemID, "tool_detail", ordinal),
		DetailAvailable: true,
		CreatedAt:       createdAt,
		Children: []UITimelineItem{{
			ItemID:          timelineItemID(b.item.ItemID, "operation_detail", ordinal),
			TurnID:          b.item.TurnID,
			Kind:            "operation_detail",
			Status:          "running",
			Tool:            tool,
			DetailAvailable: true,
			CreatedAt:       createdAt,
		}},
	}
	processing.Children = append(processing.Children, group)
	b.toolGroupByCallEvent[strings.TrimSpace(callEventID)] = len(processing.Children) - 1
	b.toolOrdinal++
	processing.ToolCount = b.toolOrdinal
}

func (b *timelineTurnBuilder) applyToolResult(result ToolResultProjection, createdAt time.Time) {
	group := b.toolGroupForCall(result.ForEventID, result.Tool, createdAt)
	preview := toolResultPreview(result.Content)
	group.Status = result.Status
	group.Tool = result.Tool
	group.OutputPreview = preview.Text
	group.OutputSource = preview.Source
	group.OutputTruncated = preview.Truncated
	group.FullOutputAvailable = preview.FullAvailable
	group.UpdatedAt = createdAt
	detail := b.ensureOperationDetail(group, createdAt)
	detail.Status = result.Status
	detail.Tool = result.Tool
	detail.OutputPreview = preview.Text
	detail.OutputSource = preview.Source
	detail.OutputTruncated = preview.Truncated
	detail.FullOutputAvailable = preview.FullAvailable
	detail.UpdatedAt = createdAt
	if result.Status == "approval_required" {
		b.pendingAction = true
		b.appendUserActionRequest("approval_required", result.Tool, createdAt)
	}
}

func (b *timelineTurnBuilder) toolGroupForCall(callEventID string, tool string, createdAt time.Time) *UITimelineItem {
	processing := b.processingGroup()
	if idx, ok := b.toolGroupByCallEvent[strings.TrimSpace(callEventID)]; ok {
		return &processing.Children[idx]
	}
	b.addToolCall("", tool, createdAt)
	idx := len(processing.Children) - 1
	if callEventID = strings.TrimSpace(callEventID); callEventID != "" {
		b.toolGroupByCallEvent[callEventID] = idx
	}
	return &processing.Children[idx]
}

func (b *timelineTurnBuilder) applyJob(eventType string, job JobProjection, createdAt time.Time) {
	group := b.toolGroupForJob(job, createdAt)
	group.Status = job.Status
	group.Tool = job.Tool
	group.OutputPreview = jobOutputPreview(job)
	group.OutputSource = "job"
	group.OutputTruncated = job.StdoutTruncated || job.StderrTruncated
	group.FullOutputAvailable = strings.TrimSpace(job.Stdout) != "" || strings.TrimSpace(job.Stderr) != "" || strings.TrimSpace(job.Receipt) != "" || strings.TrimSpace(job.FailureReason) != ""
	group.UpdatedAt = createdAt
	detail := b.ensureOperationDetail(group, createdAt)
	detail.Status = job.Status
	detail.Tool = job.Tool
	detail.OutputPreview = group.OutputPreview
	detail.OutputSource = "job"
	detail.OutputTruncated = group.OutputTruncated
	detail.FullOutputAvailable = group.FullOutputAvailable
	detail.UpdatedAt = createdAt
}

func (b *timelineTurnBuilder) toolGroupForJob(job JobProjection, createdAt time.Time) *UITimelineItem {
	processing := b.processingGroup()
	if toolCallEventID := strings.TrimSpace(job.ToolCallEventID); toolCallEventID != "" {
		if idx, ok := b.toolGroupByCallEvent[toolCallEventID]; ok {
			if jobID := strings.TrimSpace(job.JobID); jobID != "" {
				b.toolGroupByJobID[jobID] = idx
			}
			b.noteJob(job.JobID)
			return &processing.Children[idx]
		}
	}
	if jobID := strings.TrimSpace(job.JobID); jobID != "" {
		if idx, ok := b.toolGroupByJobID[jobID]; ok {
			b.noteJob(job.JobID)
			return &processing.Children[idx]
		}
	}
	b.addToolCall(strings.TrimSpace(job.ToolCallEventID), job.Tool, createdAt)
	idx := len(processing.Children) - 1
	if jobID := strings.TrimSpace(job.JobID); jobID != "" {
		b.toolGroupByJobID[jobID] = idx
	}
	b.noteJob(job.JobID)
	return &processing.Children[idx]
}

func (b *timelineTurnBuilder) noteJob(jobID string) {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" || b.seenJobIDs[jobID] {
		return
	}
	b.seenJobIDs[jobID] = true
	b.processingGroup().JobCount = len(b.seenJobIDs)
}

func (b *timelineTurnBuilder) ensureOperationDetail(group *UITimelineItem, createdAt time.Time) *UITimelineItem {
	for i := range group.Children {
		if group.Children[i].Kind == "operation_detail" {
			return &group.Children[i]
		}
	}
	group.Children = append(group.Children, UITimelineItem{
		ItemID:          timelineItemID(group.ItemID, "operation_detail", len(group.Children)),
		TurnID:          b.item.TurnID,
		Kind:            "operation_detail",
		Status:          group.Status,
		Tool:            group.Tool,
		DetailAvailable: true,
		CreatedAt:       createdAt,
	})
	return &group.Children[len(group.Children)-1]
}

func (b *timelineTurnBuilder) appendUserActionRequest(status string, tool string, createdAt time.Time) {
	for _, child := range b.item.Children {
		if child.Kind == "user_action_request" && child.Status == status && child.Tool == tool {
			return
		}
	}
	b.item.Children = append(b.item.Children, UITimelineItem{
		ItemID:          timelineItemID(b.item.ItemID, "user_action", b.messageOrdinal),
		TurnID:          b.item.TurnID,
		Kind:            "user_action_request",
		Status:          status,
		Tool:            tool,
		Text:            "需要用户批准",
		DetailAvailable: true,
		CreatedAt:       createdAt,
	})
	b.messageOrdinal++
}

func (b *timelineTurnBuilder) appendCompactionNotice(status string, text string, createdAt time.Time) {
	processing := b.processingGroup()
	processing.CompactionCount++
	b.appendProcessingNotice("compaction_notice", status, text, createdAt)
}

func (b *timelineTurnBuilder) appendProcessingNotice(kind string, status string, text string, createdAt time.Time) {
	processing := b.processingGroup()
	processing.Children = append(processing.Children, UITimelineItem{
		ItemID:    timelineItemID(b.item.ItemID, kind, len(processing.Children)),
		TurnID:    b.item.TurnID,
		Kind:      kind,
		Status:    status,
		Text:      text,
		CreatedAt: createdAt,
	})
}

func (b *timelineTurnBuilder) markTerminal(status string, at time.Time) {
	if b.terminalAt.IsZero() || at.After(b.terminalAt) {
		b.terminalAt = at
		b.terminalStatus = status
	}
}

func (b *timelineTurnBuilder) finalize(now time.Time) UITimelineItem {
	processing := b.processingGroup()
	end := now
	settled := !b.terminalAt.IsZero()
	status := "running"
	prefix := "正在处理 "
	if settled {
		end = b.terminalAt
		status = b.terminalStatus
		if status == "" {
			status = "completed"
		}
		prefix = "已处理 "
		processing.DefaultOpen = false
	} else if b.pendingAction {
		status = "waiting_for_user"
		processing.DefaultOpen = true
	}
	if end.Before(b.startedAt) {
		end = b.startedAt
	}
	duration := end.Sub(b.startedAt)
	processing.Status = status
	processing.Text = prefix + formatTimelineDuration(duration)
	processing.DurationMs = duration.Milliseconds()
	if processing.UpdatedAt.IsZero() || end.After(processing.UpdatedAt) {
		processing.UpdatedAt = end
	}
	b.item.Status = status
	if b.item.UpdatedAt.IsZero() || end.After(b.item.UpdatedAt) {
		b.item.UpdatedAt = end
	}
	return normalizeUITimelineItemArrays(b.item)
}

func normalizeUITimelineItemArrays(item UITimelineItem) UITimelineItem {
	if item.Children == nil {
		item.Children = []UITimelineItem{}
	}
	for i := range item.Children {
		item.Children[i] = normalizeUITimelineItemArrays(item.Children[i])
	}
	return item
}

func timelineNow(k *Kernel) time.Time {
	if k != nil && k.clock != nil {
		return k.clock()
	}
	return time.Now().UTC()
}

func timelineItemID(turnID string, kind string, ordinal int) string {
	return fmt.Sprintf("%s:%s:%d", turnID, kind, ordinal)
}

func formatTimelineDuration(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}
	totalSeconds := int64(duration / time.Second)
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60
	switch {
	case hours > 0 && minutes > 0:
		return fmt.Sprintf("%dh %dm", hours, minutes)
	case hours > 0:
		return fmt.Sprintf("%dh", hours)
	case minutes > 0:
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}
