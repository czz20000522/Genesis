package messageingress

import "strings"

type SessionMapper interface {
	Map(ChannelMessage) (string, error)
}

type DefaultSessionMapper struct{}

func (DefaultSessionMapper) Map(msg ChannelMessage) (string, error) {
	if err := msg.Validate(); err != nil {
		return "", err
	}
	return StableOpaqueID("chan",
		strings.TrimSpace(msg.Channel),
		strings.TrimSpace(msg.Adapter),
		strings.TrimSpace(msg.ThreadID),
	), nil
}

func KernelIdempotencyKey(msg ChannelMessage) string {
	return StableOpaqueID("inmsg",
		strings.TrimSpace(msg.Channel),
		strings.TrimSpace(msg.Adapter),
		strings.TrimSpace(msg.MessageID),
	)
}
