package kernel

import (
	"context"
	"strings"
)

const (
	defaultAutoCompactRatio       = 0.8
	defaultRecentTurnLimit        = 2
	defaultSkillIndexChars        = 1200
	defaultCompactionBackoffTurns = 1
)

const (
	contextCompactionTriggerAuto     = "auto"
	contextCompactionStatusRunning   = "running"
	contextCompactionStatusCompleted = "completed"
	contextCompactionStatusFailed    = "failed"
	contextCompactionStatusDeferred  = "deferred"

	contextCompactionDeferredReasonStuckWindow = "auto_compaction_stuck_window"
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
	if policy.RetryBackoffTurns <= 0 {
		policy.RetryBackoffTurns = defaultCompactionBackoffTurns
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
	SourceUsage       *TokenUsage
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
		SourceUsage:       cloneTokenUsage(final.Usage),
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
	cacheStability := contextCacheStability(region, accounting)
	sourceUsage := cloneTokenUsage(command.SourceUsage)
	if sourceUsage == nil && command.SourceInputTokens > 0 {
		sourceUsage = &TokenUsage{InputTokens: command.SourceInputTokens}
	}
	if deferred, projection := contextCompactionBackoff(events, sessionID, trigger, command.SourceInputTokens, sourceUsage, cacheStability, k.contextPolicy); deferred {
		_ = k.appendContextCompactionEvent(sessionID, triggeringTurnID, projection)
		return
	}
	if deferred, projection := contextCompactionStuckGuard(events, sessionID, trigger, triggeringTurnID, command.SourceInputTokens, sourceUsage, cacheStability, k.contextPolicy); deferred {
		_ = k.appendContextCompactionEvent(sessionID, triggeringTurnID, projection)
		return
	}
	startedAt := k.clock()
	started := ContextCompactionProjection{
		Trigger:                  trigger,
		Status:                   contextCompactionStatusRunning,
		CompactedThroughTurnID:   region[len(region)-1].TurnID,
		CompactedTurnCount:       len(region),
		SourceInputTokens:        command.SourceInputTokens,
		SourceUsage:              sourceUsage,
		CacheStability:           cacheStability,
		RetryAfterCompletedTurns: k.contextPolicy.RetryBackoffTurns,
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
		Trigger:                  trigger,
		Status:                   contextCompactionStatusCompleted,
		Summary:                  strings.TrimSpace(response.Text),
		CompactedThroughTurnID:   region[len(region)-1].TurnID,
		CompactedTurnCount:       len(region),
		SourceInputTokens:        command.SourceInputTokens,
		SourceUsage:              sourceUsage,
		CacheStability:           cacheStability,
		RetryAfterCompletedTurns: k.contextPolicy.RetryBackoffTurns,
		Model:                    response.Model,
		Usage:                    response.Usage,
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

func contextCompactionBackoff(events []StoredEvent, sessionID string, trigger string, sourceInputTokens int, sourceUsage *TokenUsage, cacheStability *ContextCacheStabilityProjection, policy ContextPolicy) (bool, ContextCompactionProjection) {
	if policy.RetryBackoffTurns <= 0 {
		return false, ContextCompactionProjection{}
	}
	failure, ok := latestContextCompactionFailureAfterCompletion(events, sessionID)
	if !ok {
		return false, ContextCompactionProjection{}
	}
	turns := sameSessionCompletedConversationTurns(events, sessionID, "")
	completedAfterFailure := completedTurnsAfter(turns, failure.TurnID)
	if completedAfterFailure > policy.RetryBackoffTurns {
		return false, ContextCompactionProjection{}
	}
	remaining := policy.RetryBackoffTurns + 1 - completedAfterFailure
	if remaining < 1 {
		remaining = 1
	}
	projection := ContextCompactionProjection{
		Trigger:                  trigger,
		Status:                   contextCompactionStatusDeferred,
		SourceInputTokens:        sourceInputTokens,
		SourceUsage:              cloneTokenUsage(sourceUsage),
		CacheStability:           cacheStability,
		PreviousFailureReason:    strings.TrimSpace(failure.Projection.FailureReason),
		RetryAfterCompletedTurns: policy.RetryBackoffTurns,
		BackoffRemainingTurns:    remaining,
	}
	if projection.PreviousFailureReason == "" {
		projection.PreviousFailureReason = contextCompactionStatusFailed
	}
	return true, projection
}

func contextCompactionStuckGuard(events []StoredEvent, sessionID string, trigger string, triggeringTurnID string, sourceInputTokens int, sourceUsage *TokenUsage, cacheStability *ContextCacheStabilityProjection, policy ContextPolicy) (bool, ContextCompactionProjection) {
	if trigger != contextCompactionTriggerAuto {
		return false, ContextCompactionProjection{}
	}
	limit := policy.autoCompactLimit()
	if limit <= 0 || sourceInputTokens < limit {
		return false, ContextCompactionProjection{}
	}
	turns := sameSessionCompletedConversationTurns(events, sessionID, "")
	turnIndex := completedTurnIndexByID(turns)
	currentIndex, ok := turnIndex[strings.TrimSpace(triggeringTurnID)]
	if !ok {
		return false, ContextCompactionProjection{}
	}
	count := consecutiveCompletedAutoCompactionsBefore(events, sessionID, currentIndex, limit, turnIndex)
	if count < 2 && !previousTurnDeferredAutoCompactionStuck(events, sessionID, currentIndex, turnIndex) {
		return false, ContextCompactionProjection{}
	}
	if count < 2 {
		count = 2
	}
	return true, ContextCompactionProjection{
		Trigger:                         trigger,
		Status:                          contextCompactionStatusDeferred,
		DeferredReason:                  contextCompactionDeferredReasonStuckWindow,
		SourceInputTokens:               sourceInputTokens,
		SourceUsage:                     cloneTokenUsage(sourceUsage),
		CacheStability:                  cacheStability,
		ConsecutiveCompletedCompactions: count,
	}
}

func completedTurnIndexByID(turns []conversationHistoryTurn) map[string]int {
	index := make(map[string]int, len(turns))
	for i, turn := range turns {
		if turn.TurnID != "" {
			index[turn.TurnID] = i
		}
	}
	return index
}

func consecutiveCompletedAutoCompactionsBefore(events []StoredEvent, sessionID string, currentTurnIndex int, autoLimit int, turnIndex map[string]int) int {
	nextExpected := currentTurnIndex - 1
	count := 0
	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if event.SessionID != sessionID || event.Type != "context.compaction.completed" || event.Data.ContextCompaction == nil {
			continue
		}
		projection := event.Data.ContextCompaction
		if projection.Trigger != contextCompactionTriggerAuto || projection.SourceInputTokens < autoLimit {
			continue
		}
		index, ok := turnIndex[strings.TrimSpace(event.TurnID)]
		if !ok || index != nextExpected {
			return count
		}
		count++
		nextExpected--
	}
	return count
}

func previousTurnDeferredAutoCompactionStuck(events []StoredEvent, sessionID string, currentTurnIndex int, turnIndex map[string]int) bool {
	previousIndex := currentTurnIndex - 1
	if previousIndex < 0 {
		return false
	}
	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if event.SessionID != sessionID || event.Type != "context.compaction.deferred" || event.Data.ContextCompaction == nil {
			continue
		}
		if event.Data.ContextCompaction.Trigger != contextCompactionTriggerAuto ||
			event.Data.ContextCompaction.DeferredReason != contextCompactionDeferredReasonStuckWindow {
			continue
		}
		index, ok := turnIndex[strings.TrimSpace(event.TurnID)]
		return ok && index == previousIndex
	}
	return false
}

type contextCompactionFailureEvent struct {
	TurnID     string
	Projection ContextCompactionProjection
}

func latestContextCompactionFailureAfterCompletion(events []StoredEvent, sessionID string) (contextCompactionFailureEvent, bool) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return contextCompactionFailureEvent{}, false
	}
	lastCompletionIndex := -1
	for index, event := range events {
		if event.SessionID == sessionID && event.Type == "context.compaction.completed" {
			lastCompletionIndex = index
		}
	}
	var latest contextCompactionFailureEvent
	found := false
	for index := lastCompletionIndex + 1; index < len(events); index++ {
		event := events[index]
		if event.SessionID != sessionID || event.Type != "context.compaction.failed" || event.Data.ContextCompaction == nil {
			continue
		}
		latest = contextCompactionFailureEvent{
			TurnID:     strings.TrimSpace(event.TurnID),
			Projection: *event.Data.ContextCompaction,
		}
		found = true
	}
	return latest, found
}

func completedTurnsAfter(turns []conversationHistoryTurn, turnID string) int {
	turnID = strings.TrimSpace(turnID)
	if turnID == "" {
		return 0
	}
	for index, turn := range turns {
		if turn.TurnID == turnID {
			return len(turns) - index - 1
		}
	}
	return 0
}

func (k *Kernel) appendContextCompactionFailed(sessionID string, turnID string, started ContextCompactionProjection, reason string) {
	now := k.clock()
	failed := started
	failed.Status = contextCompactionStatusFailed
	failed.Summary = ""
	failed.FailureReason = strings.TrimSpace(reason)
	if failed.RetryAfterCompletedTurns <= 0 {
		failed.RetryAfterCompletedTurns = k.contextPolicy.RetryBackoffTurns
	}
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

func (k *Kernel) appendContextCompactionEvent(sessionID string, turnID string, projection ContextCompactionProjection) error {
	now := k.clock()
	return k.appendEvent(StoredEvent{
		EventID:   newID("evt", now),
		SessionID: strings.TrimSpace(sessionID),
		TurnID:    strings.TrimSpace(turnID),
		Type:      "context.compaction." + strings.TrimSpace(projection.Status),
		CreatedAt: now,
		Data: EventData{
			ContextCompaction: &projection,
		},
	})
}

func contextCacheStability(region []conversationHistoryTurn, accountingByTurn map[string]ModelContextAccountingProjection) *ContextCacheStabilityProjection {
	if len(region) == 0 || len(accountingByTurn) == 0 {
		return nil
	}
	var samples []TokenUsage
	for _, turn := range region {
		accounting, ok := accountingByTurn[turn.TurnID]
		if !ok || accounting.Usage == nil {
			continue
		}
		if accounting.Usage.CacheHitTokens == 0 && accounting.Usage.CacheMissTokens == 0 {
			continue
		}
		samples = append(samples, *accounting.Usage)
	}
	if len(samples) == 0 {
		return nil
	}
	projection := ContextCacheStabilityProjection{Samples: len(samples)}
	for _, usage := range samples {
		projection.CacheHitTokens += usage.CacheHitTokens
		projection.CacheMissTokens += usage.CacheMissTokens
	}
	first := samples[0]
	latest := samples[len(samples)-1]
	projection.FirstHitRatePermille = cacheHitRatePermille(first.CacheHitTokens, first.CacheMissTokens)
	projection.LatestHitRatePermille = cacheHitRatePermille(latest.CacheHitTokens, latest.CacheMissTokens)
	projection.HitRatePermille = cacheHitRatePermille(projection.CacheHitTokens, projection.CacheMissTokens)
	projection.LatestCacheHitTokens = latest.CacheHitTokens
	projection.LatestCacheMissTokens = latest.CacheMissTokens
	projection.Trend = cacheTrend(projection.FirstHitRatePermille, projection.LatestHitRatePermille, len(samples))
	return &projection
}

func cacheHitRatePermille(hitTokens int, missTokens int) int {
	total := hitTokens + missTokens
	if total <= 0 {
		return 0
	}
	return (hitTokens * 1000) / total
}

func cacheTrend(firstPermille int, latestPermille int, samples int) string {
	if samples < 2 {
		return "unknown"
	}
	const threshold = 50
	switch {
	case latestPermille-firstPermille >= threshold:
		return "warming"
	case firstPermille-latestPermille >= threshold:
		return "cooling"
	default:
		return "stable"
	}
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
		for _, exchange := range turn.ToolExchanges {
			tool := strings.TrimSpace(exchange.Tool)
			if tool == "" {
				tool = "unknown"
			}
			callLines := []string{"[tool call]", "tool: " + tool}
			if arguments := strings.TrimSpace(exchange.Arguments); arguments != "" {
				callLines = append(callLines, "arguments: "+redactEvidenceText(arguments))
			}
			lines = append(lines, strings.Join(callLines, "\n"))
			resultLines := []string{"[tool result]"}
			if status := strings.TrimSpace(exchange.ResultStatus); status != "" {
				resultLines = append(resultLines, "status: "+status)
			}
			if content := strings.TrimSpace(exchange.ResultContent); content != "" {
				resultLines = append(resultLines, redactEvidenceText(content))
			}
			lines = append(lines, strings.Join(resultLines, "\n"))
		}
		if assistantText != "" {
			lines = append(lines, "[assistant]\n"+assistantText)
		}
	}
	return strings.Join(lines, "\n\n")
}
