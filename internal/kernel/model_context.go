package kernel

import (
	"encoding/json"
	"strings"
)

const (
	sourceSnapshotContextBytes = 4096
	sourceSnapshotLabelBytes   = 160
	stableProviderInstruction  = "You are Genesis, a local-first personal AI assistant. Follow the user's request, use available tools only when they help, and return a clear visible answer."
)

type conversationHistoryTurn struct {
	TurnID        string
	UserText      string
	ToolExchanges []conversationToolExchange
	AssistantText string
}

type conversationToolExchange struct {
	Tool          string
	Arguments     string
	ResultStatus  string
	ResultContent string
}

func modelInputItems(userItems []InputItem) []ModelInputItem {
	return modelInputItemsWithHistory(userItems, nil, 0, "")
}

func modelInputItemsWithHistory(userItems []InputItem, skills []SkillCatalogItemProjection, skillIndexBudget int, historyContext string) []ModelInputItem {
	return modelInputItemsWithHistoryAndHydration(userItems, skills, nil, nil, skillIndexBudget, historyContext, "")
}

func modelInputItemsWithHistoryAndHydration(userItems []InputItem, skills []SkillCatalogItemProjection, hydratedContexts []providerHydratedContextFragment, sourceSnapshots []SourceSnapshotDescriptor, skillIndexBudget int, historyContext string, observationContext string) []ModelInputItem {
	skillContext := skillIndexContext(skills, skillIndexBudget)
	sourceContext := sourceSnapshotContext(sourceSnapshots)
	withContext := make([]ModelInputItem, 0, len(userItems)+5+len(hydratedContexts))
	if strings.TrimSpace(historyContext) != "" {
		withContext = append(withContext, ModelInputItem{Kind: ModelInputKindConversationHistoryContext, Text: historyContext})
	}
	if skillContext != "" {
		withContext = append(withContext, ModelInputItem{Kind: ModelInputKindSkillIndexContext, Text: skillContext})
	}
	if context := strings.TrimSpace(observationContext); context != "" {
		withContext = append(withContext, ModelInputItem{Kind: ModelInputKindKernelObservationContext, Text: context})
	}
	withContext = appendHydratedContextItems(withContext, hydratedContexts)
	if sourceContext != "" {
		withContext = append(withContext, ModelInputItem{Kind: ModelInputKindSourceSnapshotContext, Text: sourceContext})
	}
	for _, item := range userItems {
		if item.Type == "text" && item.Text != "" {
			withContext = append(withContext, ModelInputItem{Kind: ModelInputKindUserText, Text: item.Text})
		}
	}
	return withContext
}

func sourceSnapshotContext(snapshots []SourceSnapshotDescriptor) string {
	if len(snapshots) == 0 {
		return ""
	}
	header := "Source snapshots available for this session. Use source_tree with source_snapshot_ref, then source_read with source_file_ref for selected files."
	lines := []string{header}
	used := len([]byte(header))
	omitted := 0
	for _, snapshot := range snapshots {
		ref := strings.TrimSpace(snapshot.SourceSnapshotRef)
		if ref == "" {
			continue
		}
		label := oneLine(snapshot.DisplayLabel)
		if label == "" {
			label = ref
		}
		label = utf8SafePrefix(label, sourceSnapshotLabelBytes)
		ops := strings.Join(snapshot.AvailableOperations, ",")
		if ops == "" {
			ops = ReferenceOperationSourceTree
		}
		line := "- " + ref + " (" + snapshot.SourceKind + ", " + snapshot.Purpose + ", label=" + label + ", operations=" + ops + ")"
		if used+1+len([]byte(line)) > sourceSnapshotContextBytes {
			omitted++
			continue
		}
		lines = append(lines, line)
		used += 1 + len([]byte(line))
	}
	if omitted > 0 {
		line := "- additional source snapshots omitted by context budget"
		if used+1+len([]byte(line)) <= sourceSnapshotContextBytes {
			lines = append(lines, line)
		}
	}
	if len(lines) == 1 {
		return ""
	}
	return strings.Join(lines, "\n")
}

func appendHydratedContextItems(items []ModelInputItem, hydratedContexts []providerHydratedContextFragment) []ModelInputItem {
	for _, context := range hydratedContexts {
		if context.InputKind != ModelInputKindHydratedContext {
			continue
		}
		if text := strings.TrimSpace(context.VisibleText); text != "" {
			items = append(items, ModelInputItem{Kind: ModelInputKindHydratedContext, Text: context.VisibleText})
		}
	}
	return items
}

func conversationHistoryContext(turns []conversationHistoryTurn) string {
	return conversationHistoryContextWithSummary("", turns)
}

func conversationHistoryContextWithSummary(summary string, turns []conversationHistoryTurn) string {
	summary = strings.TrimSpace(summary)
	if summary == "" && len(turns) == 0 {
		return ""
	}
	lines := make([]string, 0, len(turns)*2+3)
	lines = append(lines, "Same-session conversation history:")
	if summary != "" {
		lines = append(lines, "Compacted earlier conversation:")
		lines = append(lines, summary)
	}
	for _, turn := range turns {
		userText := strings.TrimSpace(turn.UserText)
		assistantText := strings.TrimSpace(turn.AssistantText)
		if userText != "" {
			lines = append(lines, "User: "+userText)
		}
		lines = appendToolExchangeLines(lines, turn.ToolExchanges)
		if assistantText != "" {
			lines = append(lines, "Assistant: "+assistantText)
		}
	}
	if len(lines) == 1 {
		return ""
	}
	return strings.Join(lines, "\n")
}

func appendToolExchangeLines(lines []string, exchanges []conversationToolExchange) []string {
	for _, exchange := range exchanges {
		tool := strings.TrimSpace(exchange.Tool)
		if tool == "" {
			tool = "unknown"
		}
		lines = append(lines, "Tool call: "+tool)
		if arguments := strings.TrimSpace(exchange.Arguments); arguments != "" {
			lines = append(lines, "Tool arguments: "+arguments)
		}
		status := strings.TrimSpace(exchange.ResultStatus)
		content := strings.TrimSpace(exchange.ResultContent)
		if status != "" || content != "" {
			resultLine := "Tool result:"
			if status != "" {
				resultLine += " " + status
			}
			lines = append(lines, resultLine)
			if content != "" {
				lines = append(lines, content)
			}
		}
	}
	return lines
}

func modelInputKinds(items []ModelInputItem) []string {
	kinds := make([]string, 0, len(items))
	for _, item := range items {
		kind := strings.TrimSpace(item.Kind)
		if kind != "" {
			kinds = append(kinds, kind)
		}
	}
	if len(kinds) == 0 {
		return nil
	}
	return kinds
}

func modelUserTextWithoutHistory(items []ModelInputItem) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		if item.Kind != ModelInputKindConversationHistoryContext && item.Text != "" {
			parts = append(parts, item.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func stableSystemPrefix(items []ModelInputItem) string {
	instruction, skillIndex := stableSystemPrefixParts(items)
	parts := []string{instruction}
	if skillIndex != "" {
		parts = append(parts, skillIndex)
	}
	return strings.Join(parts, "\n\n")
}

func stableSystemPrefixParts(items []ModelInputItem) (string, string) {
	skillParts := []string{}
	for _, item := range items {
		if item.Kind == ModelInputKindSkillIndexContext && strings.TrimSpace(item.Text) != "" {
			skillParts = append(skillParts, item.Text)
		}
	}
	return stableProviderInstruction, strings.Join(skillParts, "\n\n")
}

func currentConversationMessages(items []ModelInputItem) []ModelConversationMessage {
	messages := []ModelConversationMessage{}
	for _, item := range items {
		if item.Kind == ModelInputKindConversationHistoryContext || item.Kind == ModelInputKindSkillIndexContext || item.Kind == ModelInputKindUserText || strings.TrimSpace(item.Text) == "" {
			continue
		}
		messages = append(messages, ModelConversationMessage{Role: "user", Text: "Context (" + item.Kind + "):\n" + item.Text})
	}
	for _, item := range items {
		if item.Kind == ModelInputKindUserText && strings.TrimSpace(item.Text) != "" {
			messages = append(messages, ModelConversationMessage{Role: "user", Text: item.Text})
		}
	}
	return messages
}

func skillIndexContext(skills []SkillCatalogItemProjection, budget int) string {
	if budget <= 0 || len(skills) == 0 {
		return ""
	}
	const header = "External skill index (metadata only; instructions are loaded only when explicitly needed):"
	used := len(header)
	lines := []string{header}
	for _, skill := range skills {
		name := strings.TrimSpace(skill.Name)
		if name == "" {
			continue
		}
		minimum := "- " + name
		line := minimum
		if description := oneLine(strings.TrimSpace(skill.Description)); description != "" {
			line = minimum + ": " + description
		}
		if used+1+len(line) > budget {
			if used+1+len(minimum) > budget {
				continue
			}
			line = minimum
		}
		lines = append(lines, line)
		used += 1 + len(line)
	}
	if len(lines) == 1 {
		return ""
	}
	return strings.Join(lines, "\n")
}

func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func cloneInputItems(items []InputItem) []InputItem {
	cloned := make([]InputItem, len(items))
	copy(cloned, items)
	return cloned
}

func cloneModelInputItems(items []ModelInputItem) []ModelInputItem {
	cloned := make([]ModelInputItem, len(items))
	copy(cloned, items)
	return cloned
}

func cloneModelConversationMessages(messages []ModelConversationMessage) []ModelConversationMessage {
	cloned := make([]ModelConversationMessage, 0, len(messages))
	for _, message := range messages {
		next := message
		next.ToolCalls = make([]ModelToolCall, 0, len(message.ToolCalls))
		for _, call := range message.ToolCalls {
			next.ToolCalls = append(next.ToolCalls, ModelToolCall{
				ToolCallID:      call.ToolCallID,
				ToolCallEventID: call.ToolCallEventID,
				Name:            call.Name,
				Arguments:       append(json.RawMessage(nil), call.Arguments...),
			})
		}
		cloned = append(cloned, next)
	}
	return cloned
}

func cloneModelToolCalls(calls []ModelToolCall) []ModelToolCall {
	cloned := make([]ModelToolCall, 0, len(calls))
	for _, call := range calls {
		cloned = append(cloned, ModelToolCall{
			ToolCallID:      call.ToolCallID,
			ToolCallEventID: call.ToolCallEventID,
			Name:            call.Name,
			Arguments:       append(json.RawMessage(nil), call.Arguments...),
		})
	}
	return cloned
}

func cloneModelToolRounds(rounds []ModelToolRound) []ModelToolRound {
	cloned := make([]ModelToolRound, 0, len(rounds))
	for _, round := range rounds {
		next := ModelToolRound{
			Calls:   make([]ModelToolCall, 0, len(round.Calls)),
			Results: make([]ModelToolResult, 0, len(round.Results)),
		}
		for _, call := range round.Calls {
			next.Calls = append(next.Calls, ModelToolCall{
				ToolCallID:      call.ToolCallID,
				ToolCallEventID: call.ToolCallEventID,
				Name:            call.Name,
				Arguments:       append([]byte(nil), call.Arguments...),
			})
		}
		next.Results = append(next.Results, round.Results...)
		cloned = append(cloned, next)
	}
	return cloned
}
