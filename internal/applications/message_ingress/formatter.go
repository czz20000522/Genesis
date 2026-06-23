package messageingress

import (
	"sort"
	"strings"
)

func FormatInboundInput(msg ChannelMessage) string {
	lines := []string{
		"Inbound external message",
		"source_channel: " + strings.TrimSpace(msg.Channel),
		"adapter: " + strings.TrimSpace(msg.Adapter),
		"thread_id: " + strings.TrimSpace(msg.ThreadID),
		"message_id: " + strings.TrimSpace(msg.MessageID),
		"sender_id: " + strings.TrimSpace(msg.UserID),
	}
	if len(msg.Metadata) > 0 {
		keys := make([]string, 0, len(msg.Metadata))
		for key := range msg.Metadata {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			value := strings.TrimSpace(msg.Metadata[key])
			if value == "" {
				continue
			}
			lines = append(lines, strings.TrimSpace(key)+": "+value)
		}
	}
	lines = append(lines, "", "text:", strings.TrimSpace(msg.Text))
	return strings.Join(lines, "\n")
}
