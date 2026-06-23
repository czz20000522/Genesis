package messageingress

import (
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"strings"
)

func (m ChannelMessage) Validate() error {
	switch {
	case strings.TrimSpace(m.Channel) == "":
		return errors.New("channel message missing channel")
	case strings.TrimSpace(m.Adapter) == "":
		return errors.New("channel message missing adapter")
	case strings.TrimSpace(m.MessageID) == "":
		return errors.New("channel message missing message_id")
	case strings.TrimSpace(m.ThreadID) == "":
		return errors.New("channel message missing thread_id")
	case strings.TrimSpace(m.UserID) == "":
		return errors.New("channel message missing user_id")
	case strings.TrimSpace(m.Text) == "":
		return errors.New("channel message missing text")
	default:
		return nil
	}
}

func (m ChannelMessage) RawDedupeKey() string {
	return strings.TrimSpace(m.Channel) + ":" + strings.TrimSpace(m.Adapter) + ":" + strings.TrimSpace(m.MessageID)
}

func StableOpaqueID(prefix string, parts ...string) string {
	identity := strings.Join(parts, "\x00")
	sum := sha256.Sum256([]byte(identity))
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:])
	return prefix + "_" + strings.ToLower(encoded[:32])
}
