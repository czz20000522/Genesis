package kernel

import (
	"strings"
)

func modelInputItems(userItems []InputItem, memories []MemoryRecall) []InputItem {
	items := cloneInputItems(userItems)
	memoryContext := approvedMemoryContext(memories)
	if memoryContext == "" {
		return items
	}
	withContext := make([]InputItem, 0, len(items)+1)
	withContext = append(withContext, InputItem{Type: "text", Text: memoryContext})
	withContext = append(withContext, items...)
	return withContext
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
