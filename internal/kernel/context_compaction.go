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
	if policy.RecentTailTokens < 0 {
		policy.RecentTailTokens = 0
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

type ContextCompactionCommand struct {
	SessionID         string
	TriggeringTurnID  string
	Trigger           string
	SourceInputTokens int
}

func (k *Kernel) maybeSubmitAutoContextCompaction(ctx context.Context, sessionID string, triggeringTurnID string, final FinalMessage) {
	limit := k.contextPolicy.autoCompactLimit()
	if limit <= 0 || final.Usage == nil || final.Usage.InputTokens < limit {
		return
	}
	k.runContextCompaction(ctx, ContextCompactionCommand{
		SessionID:         sessionID,
		TriggeringTurnID:  triggeringTurnID,
		Trigger:           contextCompactionTriggerAuto,
		SourceInputTokens: final.Usage.InputTokens,
	})
}

func (k *Kernel) runContextCompaction(ctx context.Context, command ContextCompactionCommand) {
	sessionID := strings.TrimSpace(command.SessionID)
	triggeringTurnID := strings.TrimSpace(command.TriggeringTurnID)
	trigger := strings.TrimSpace(command.Trigger)
	if trigger == "" {
		trigger = contextCompactionTriggerAuto
	}
	events, err := k.loadEvents()
	if err != nil {
		return
	}
	latest := latestSessionContextCompaction(events, sessionID, "")
	turns := sameSessionCompletedConversationTurns(events, sessionID, "")
	eligible := turnsAfterCompactedTurn(turns, latest.CompactedThroughTurnID)
	accounting := modelContextAccountingByTurn(events, sessionID)
	region := compactionRegion(eligible, k.contextPolicy, accounting)
	if len(region) == 0 {
		return
	}
	startedAt := k.clock()
	started := ContextCompactionProjection{
		Trigger:                trigger,
		Status:                 contextCompactionStatusRunning,
		CompactedThroughTurnID: region[len(region)-1].TurnID,
		CompactedTurnCount:     len(region),
		SourceInputTokens:      command.SourceInputTokens,
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
		Trigger:                trigger,
		Status:                 contextCompactionStatusCompleted,
		Summary:                strings.TrimSpace(response.Text),
		CompactedThroughTurnID: region[len(region)-1].TurnID,
		CompactedTurnCount:     len(region),
		SourceInputTokens:      command.SourceInputTokens,
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

func compactionRegion(turns []conversationHistoryTurn, policy ContextPolicy, accountingByTurn map[string]ModelContextAccountingProjection) []conversationHistoryTurn {
	if len(turns) == 0 {
		return nil
	}
	floor := policy.RecentTurnLimit
	if floor <= 0 {
		floor = defaultRecentTurnLimit
	}
	if len(turns) <= floor {
		return nil
	}
	regionEnd := len(turns) - floor
	if policy.RecentTailTokens > 0 {
		regionEnd = compactionRegionEndByProviderAccounting(turns, floor, policy.RecentTailTokens, accountingByTurn)
	}
	if regionEnd <= 0 {
		return nil
	}
	return turns[:regionEnd]
}

func compactionRegionEndByProviderAccounting(turns []conversationHistoryTurn, floor int, tokenBudget int, accountingByTurn map[string]ModelContextAccountingProjection) int {
	regionEnd := len(turns) - floor
	if tokenBudget <= 0 || len(accountingByTurn) == 0 {
		return regionEnd
	}
	used := 0
	kept := 0
	for index := len(turns) - 1; index >= 0; index-- {
		accounting, ok := accountingByTurn[turns[index].TurnID]
		if !ok || accounting.ProcessedInputTokens <= 0 {
			if kept >= floor {
				break
			}
			return regionEnd
		}
		if kept >= floor && used+accounting.ProcessedInputTokens > tokenBudget {
			break
		}
		used += accounting.ProcessedInputTokens
		regionEnd = index
		kept++
	}
	if len(turns)-regionEnd < floor {
		regionEnd = len(turns) - floor
	}
	return regionEnd
}

func modelContextAccountingByTurn(events []StoredEvent, sessionID string) map[string]ModelContextAccountingProjection {
	sessionID = strings.TrimSpace(sessionID)
	accountingByTurn := map[string]ModelContextAccountingProjection{}
	if sessionID == "" {
		return accountingByTurn
	}
	for _, event := range events {
		if event.SessionID != sessionID || event.Type != "model.context.accounted" || event.Data.ModelContextAccounting == nil {
			continue
		}
		accountingByTurn[event.TurnID] = *event.Data.ModelContextAccounting
	}
	return accountingByTurn
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
