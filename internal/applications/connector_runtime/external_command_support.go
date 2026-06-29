package connectorruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func SafeCLIProbeExcerpt(output []byte) string {
	const limit = 1024
	truncated := false
	if len(output) > limit {
		output = output[:limit]
		truncated = true
	}
	lines := strings.Split(string(output), "\n")
	for i, line := range lines {
		if isCredentialShapedExternalValue(line) {
			lines[i] = "[redacted credential-shaped CLI output]"
		}
	}
	text := strings.Join(lines, "\n")
	if truncated {
		text += "\n[truncated]"
	}
	return text
}

func sourcePayloadHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func ignoreSenderIDSet(values []string) map[string]struct{} {
	ignored := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			ignored[value] = struct{}{}
		}
	}
	return ignored
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
