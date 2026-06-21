package kernel

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const localSecretRefPrefix = "secret://"

var localSecretRefPattern = regexp.MustCompile(`^secret://[a-z0-9][a-z0-9._/-]{0,190}$`)

var (
	ErrLocalSecretRefInvalid  = errors.New("local secret ref invalid")
	ErrLocalSecretMissing     = errors.New("local secret missing")
	ErrLocalSecretUnreadable  = errors.New("local secret unreadable")
	ErrLocalSecretUnsupported = errors.New("local secret unsupported")
)

func ResolveLocalCredentialSecret(ref string, storeRoot string) (string, error) {
	normalized := normalizeLocalSecretRef(ref)
	if normalized == "" {
		return "", ErrLocalSecretRefInvalid
	}
	path := localSecretPath(normalized, storeRoot)
	payload, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrLocalSecretMissing
		}
		return "", fmt.Errorf("%w: %v", ErrLocalSecretUnreadable, err)
	}
	var record localSecretRecord
	if err := json.Unmarshal(payload, &record); err != nil {
		return "", fmt.Errorf("%w: %v", ErrLocalSecretUnreadable, err)
	}
	if record.RecordType != "local_credential_secret" {
		return "", ErrLocalSecretUnreadable
	}
	if normalizeLocalSecretRef(record.CredentialRef) != normalized {
		return "", ErrLocalSecretUnreadable
	}
	protected := strings.TrimSpace(record.ProtectedData)
	if protected == "" {
		return "", ErrLocalSecretUnreadable
	}
	encrypted, err := base64.StdEncoding.DecodeString(protected)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrLocalSecretUnreadable, err)
	}
	plain, err := dpapiUnprotect(encrypted)
	if err != nil {
		if errors.Is(err, ErrLocalSecretUnsupported) {
			return "", err
		}
		return "", fmt.Errorf("%w: %v", ErrLocalSecretUnreadable, err)
	}
	return strings.TrimSpace(string(plain)), nil
}

func isLocalSecretCredentialRef(value string) bool {
	return normalizeLocalSecretRef(value) != ""
}

func normalizeLocalSecretRef(value string) string {
	text := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"))
	if !strings.HasPrefix(text, localSecretRefPrefix) {
		return ""
	}
	parts := strings.Split(text[len(localSecretRefPrefix):], "/")
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		token := strings.TrimSpace(part)
		if token == "" {
			continue
		}
		if token == ".." {
			return ""
		}
		cleaned = append(cleaned, token)
	}
	if len(cleaned) == 0 {
		return ""
	}
	normalized := localSecretRefPrefix + strings.Join(cleaned, "/")
	if !localSecretRefPattern.MatchString(normalized) {
		return ""
	}
	return normalized
}

func localSecretPath(ref string, storeRoot string) string {
	root := strings.TrimSpace(storeRoot)
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			root = filepath.Join(".genesis", "credentials")
		} else {
			root = filepath.Join(home, ".genesis", "credentials")
		}
	}
	normalized := normalizeLocalSecretRef(ref)
	sum := sha256.Sum256([]byte(normalized))
	digest := hex.EncodeToString(sum[:])[:24]
	suffix := strings.ReplaceAll(strings.TrimPrefix(normalized, localSecretRefPrefix), "/", "-")
	token := refToken(suffix)
	if token == "" {
		token = "credential"
	}
	return filepath.Join(filepath.Clean(expandHome(root)), token+"-"+digest+".json")
}

func refToken(value string) string {
	text := strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false
	for _, char := range text {
		allowed := (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' || char == '_' || char == '.'
		if allowed {
			builder.WriteRune(char)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-_.")
}

type localSecretRecord struct {
	RecordType    string `json:"record_type"`
	CredentialRef string `json:"credential_ref"`
	ProtectedData string `json:"protected_data_b64"`
}
