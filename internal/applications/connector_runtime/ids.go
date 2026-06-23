package connectorruntime

import (
	"crypto/sha256"
	"encoding/base32"
	"strings"
)

func stableOpaqueID(prefix string, parts ...string) string {
	identity := strings.Join(parts, "\x00")
	sum := sha256.Sum256([]byte(identity))
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:])
	return prefix + "_" + strings.ToLower(encoded[:32])
}
