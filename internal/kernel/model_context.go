package kernel

import (
	"strings"
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

func modelInputItems(userItems []InputItem, memories []MemoryRecall) []ModelInputItem {
	return modelInputItemsWithHistory(userItems, memories, nil, 0, "")
}

func modelInputItemsWithHistory(userItems []InputItem, memories []MemoryRecall, skills []SkillCatalogItemProjection, skillIndexBudget int, historyContext string) []ModelInputItem {
	skillContext := skillIndexContext(skills, skillIndexBudget)
	memoryContext := approvedMemoryContext(memories)
	withContext := make([]ModelInputItem, 0, len(userItems)+3)
	if strings.TrimSpace(historyContext) != "" {
		withContext = append(withContext, ModelInputItem{Kind: ModelInputKindConversationHistoryContext, Text: historyContext})
	}
	if skillContext != "" {
		withContext = append(withContext, ModelInputItem{Kind: ModelInputKindSkillIndexContext, Text: skillContext})
	}
	if memoryContext != "" {
		withContext = append(withContext, ModelInputItem{Kind: ModelInputKindApprovedMemoryContext, Text: memoryContext})
	}
	for _, item := range userItems {
		if item.Type == "text" && item.Text != "" {
			withContext = append(withContext, ModelInputItem{Kind: ModelInputKindUserText, Text: item.Text})
		}
	}
	return withContext
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
			lines = append(lines, "Tool arguments: "+redactEvidenceText(arguments))
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
				lines = append(lines, redactEvidenceText(content))
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

func approvedMemoryContext(memories []MemoryRecall) string {
	var memoryLines []string
	for _, memory := range memories {
		text := strings.TrimSpace(memory.Text)
		if text != "" {
			memoryLines = append(memoryLines, "- "+text)
		}
	}
	if len(memoryLines) == 0 {
		return ""
	}
	return "Approved memories:\n" + strings.Join(memoryLines, "\n")
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
