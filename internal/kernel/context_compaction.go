package kernel

import (
	"context"
	"strings"
)

const (
	defaultAutoCompactRatio = 0.8
	defaultRecentTurnLimit  = 2
	defaultSkillIndexChars  = 1200
)

const (
	contextCompactionTriggerAuto     = "auto"
	contextCompactionStatusRunning   = "running"
	contextCompactionStatusCompleted = "completed"
	contextCompactionStatusFailed    = "failed"
)

const contextCompactionPrompt = `You are compacting the earlier part of an assistant conversation to save context.
The assistant will keep only your summary plus recent turns. Preserve concrete user preferences, facts, decisions, constraints, file paths, commands, errors, and pending next steps. Do not invent facts. Be concise.`

func normalizedContextPolicy(policy ContextPolicy) ContextPolicy {
	if policy.SkillIndexChars == 0 {
		policy.SkillIndexChars = defaultSkillIndexChars
	}
	if policy.SkillIndexChars < 0 {
		policy.SkillIndexChars = 0
	}
	if policy.ContextWindowTokens <= 0 {
		policy.ContextWindowTokens = 0
		return policy
	}
	if policy.AutoCompactRatio <= 0 {
		policy.AutoCompactRatio = defaultAutoCompactRatio
	}
	if policy.AutoCompactRatio > 1 {
		policy.AutoCompactRatio = 1
	}
	if policy.RecentTurnLimit <= 0 {
		policy.RecentTurnLimit = defaultRecentTurnLimit
	}
	return policy
}

func (p ContextPolicy) autoCompactLimit() int {
	if p.ContextWindowTokens <= 0 || p.AutoCompactRatio <= 0 {
		return 0
	}
	limit := int(float64(p.ContextWindowTokens) * p.AutoCompactRatio)
	if limit <= 0 {
		return 1
	}
	return limit
}

func (k *Kernel) maybeCompactSession(ctx context.Context, sessionID string, triggeringTurnID string, final FinalMessage) {
	limit := k.contextPolicy.autoCompactLimit()
	if limit <= 0 || final.Usage == nil || final.Usage.InputTokens < limit {
		return
	}
	events, err := k.loadEvents()
	if err != nil {
		return
	}
	latest := latestSessionContextCompaction(events, sessionID, "")
	turns := sameSessionCompletedConversationTurns(events, sessionID, "")
	eligible := turnsAfterCompactedTurn(turns, latest.CompactedThroughTurnID)
	if len(eligible) <= k.contextPolicy.RecentTurnLimit {
		return
	}
	region := eligible[:len(eligible)-k.contextPolicy.RecentTurnLimit]
	if len(region) == 0 {
		return
	}
	startedAt := k.clock()
	started := ContextCompactionProjection{
		Trigger:                contextCompactionTriggerAuto,
		Status:                 contextCompactionStatusRunning,
		CompactedThroughTurnID: region[len(region)-1].TurnID,
		CompactedTurnCount:     len(region),
		SourceInputTokens:      final.Usage.InputTokens,
	}
	if err := k.appendEvent(StoredEvent{
		EventID:   newID("evt", startedAt),
		SessionID: strings.TrimSpace(sessionID),
		TurnID:    strings.TrimSpace(triggeringTurnID),
		Type:      "context.compaction.started",
		CreatedAt: startedAt,
		Data: EventData{
			ContextCompaction: &started,
		},
	}); err != nil {
		return
	}
	source := compactionSourceTranscript(latest.Summary, region)
	response, err := k.summarizeContext(ctx, source)
	if err != nil {
		k.appendContextCompactionFailed(sessionID, triggeringTurnID, started, err.Error())
		return
	}
	if strings.TrimSpace(response.Text) == "" {
		k.appendContextCompactionFailed(sessionID, triggeringTurnID, started, "summarizer returned empty output")
		return
	}
	now := k.clock()
	compacted := ContextCompactionProjection{
		Trigger:                contextCompactionTriggerAuto,
		Status:                 contextCompactionStatusCompleted,
		Summary:                strings.TrimSpace(response.Text),
		CompactedThroughTurnID: region[len(region)-1].TurnID,
		CompactedTurnCount:     len(region),
		SourceInputTokens:      final.Usage.InputTokens,
		Model:                  response.Model,
		Usage:                  response.Usage,
	}
	if err := k.appendEvent(StoredEvent{
		EventID:   newID("evt", now),
		SessionID: strings.TrimSpace(sessionID),
		TurnID:    strings.TrimSpace(triggeringTurnID),
		Type:      "context.compaction.completed",
		CreatedAt: now,
		Data: EventData{
			ContextCompaction: &compacted,
		},
	}); err != nil {
		k.appendContextCompactionFailed(sessionID, triggeringTurnID, started, err.Error())
	}
}

func (k *Kernel) appendContextCompactionFailed(sessionID string, turnID string, started ContextCompactionProjection, reason string) {
	now := k.clock()
	failed := started
	failed.Status = contextCompactionStatusFailed
	failed.Summary = ""
	failed.FailureReason = strings.TrimSpace(reason)
	_ = k.appendEvent(StoredEvent{
		EventID:   newID("evt", now),
		SessionID: strings.TrimSpace(sessionID),
		TurnID:    strings.TrimSpace(turnID),
		Type:      "context.compaction.failed",
		CreatedAt: now,
		Data: EventData{
			ContextCompaction: &failed,
		},
	})
}

func (k *Kernel) summarizeContext(ctx context.Context, transcript string) (ModelResponse, error) {
	return k.provider.Complete(ctx, ModelRequest{
		InputItems: []ModelInputItem{
			{Kind: "context_compaction_source", Text: contextCompactionPrompt + "\n\nTranscript:\n" + strings.TrimSpace(transcript)},
		},
	})
}

func latestSessionContextCompaction(events []StoredEvent, sessionID string, beforeTurnID string) ContextCompactionProjection {
	sessionID = strings.TrimSpace(sessionID)
	beforeTurnID = strings.TrimSpace(beforeTurnID)
	var latest ContextCompactionProjection
	for _, event := range events {
		if event.SessionID != sessionID {
			continue
		}
		if beforeTurnID != "" && event.TurnID == beforeTurnID && event.Type == "turn.submitted" {
			break
		}
		if event.Type != "context.compaction.completed" || event.Data.ContextCompaction == nil {
			continue
		}
		latest = *event.Data.ContextCompaction
	}
	return latest
}

func turnsAfterCompactedTurn(turns []conversationHistoryTurn, compactedThroughTurnID string) []conversationHistoryTurn {
	compactedThroughTurnID = strings.TrimSpace(compactedThroughTurnID)
	if compactedThroughTurnID == "" {
		return turns
	}
	for index, turn := range turns {
		if turn.TurnID == compactedThroughTurnID {
			return turns[index+1:]
		}
	}
	return turns
}

func compactionSourceTranscript(previousSummary string, turns []conversationHistoryTurn) string {
	var lines []string
	if summary := strings.TrimSpace(previousSummary); summary != "" {
		lines = append(lines, "Previous summary:", summary, "")
	}
	for _, turn := range turns {
		userText := strings.TrimSpace(turn.UserText)
		assistantText := strings.TrimSpace(turn.AssistantText)
		if userText != "" {
			lines = append(lines, "[user]\n"+userText)
		}
		if assistantText != "" {
			lines = append(lines, "[assistant]\n"+assistantText)
		}
	}
	return strings.Join(lines, "\n\n")
}
