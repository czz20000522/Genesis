package localconfig

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrCredentialRefInvalid  = errors.New("local secret ref invalid")
	ErrCredentialMissing     = errors.New("local secret missing")
	ErrCredentialUnreadable  = errors.New("local secret unreadable")
	ErrCredentialUnsupported = errors.New("local secret unsupported")
)

type CredentialSecretWriteRequest struct {
	CredentialRef string
	Secret        string
	StoreRoot     string
	Protector     func([]byte) ([]byte, error)
	DryRun        bool
}

type CredentialSecretWriteResult struct {
	CredentialRef  string `json:"credential_ref"`
	CredentialPath string `json:"credential_path"`
	DryRun         bool   `json:"dry_run"`
}

func ResolveCredentialSecret(ref string, storeRoot string) (string, error) {
	normalized := NormalizeCredentialRef(ref)
	if normalized == "" {
		return "", ErrCredentialRefInvalid
	}
	payload, err := os.ReadFile(CredentialPath(normalized, storeRoot))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrCredentialMissing
		}
		return "", fmt.Errorf("%w: %v", ErrCredentialUnreadable, err)
	}
	var record credentialSecretRecord
	if err := json.Unmarshal(payload, &record); err != nil {
		return "", fmt.Errorf("%w: %v", ErrCredentialUnreadable, err)
	}
	if record.RecordType != "local_credential_secret" || NormalizeCredentialRef(record.CredentialRef) != normalized {
		return "", ErrCredentialUnreadable
	}
	encrypted, err := base64.StdEncoding.DecodeString(strings.TrimSpace(record.ProtectedData))
	if err != nil || len(encrypted) == 0 {
		return "", ErrCredentialUnreadable
	}
	plain, err := dpapiUnprotect(encrypted)
	if err != nil {
		if errors.Is(err, ErrCredentialUnsupported) {
			return "", err
		}
		return "", fmt.Errorf("%w: %v", ErrCredentialUnreadable, err)
	}
	return strings.TrimSpace(string(plain)), nil
}

func WriteCredentialSecret(req CredentialSecretWriteRequest) (CredentialSecretWriteResult, error) {
	normalized := NormalizeCredentialRef(req.CredentialRef)
	if normalized == "" {
		return CredentialSecretWriteResult{}, ErrCredentialRefInvalid
	}
	secret := strings.TrimSpace(req.Secret)
	if secret == "" && !req.DryRun {
		return CredentialSecretWriteResult{}, ErrCredentialMissing
	}
	result := CredentialSecretWriteResult{CredentialRef: normalized, CredentialPath: CredentialPath(normalized, req.StoreRoot), DryRun: req.DryRun}
	if req.DryRun {
		return result, nil
	}
	protector := req.Protector
	if protector == nil {
		protector = dpapiProtect
	}
	protected, err := protector([]byte(secret))
	if err != nil {
		if errors.Is(err, ErrCredentialUnsupported) {
			return CredentialSecretWriteResult{}, err
		}
		return CredentialSecretWriteResult{}, fmt.Errorf("%w: %v", ErrCredentialUnreadable, err)
	}
	record := credentialSecretRecord{RecordType: "local_credential_secret", CredentialRef: normalized, ProtectedData: base64.StdEncoding.EncodeToString(protected)}
	encoded, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return CredentialSecretWriteResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(result.CredentialPath), 0o700); err != nil {
		return CredentialSecretWriteResult{}, fmt.Errorf("%w: %v", ErrCredentialUnreadable, err)
	}
	if err := os.WriteFile(result.CredentialPath, append(encoded, '\n'), 0o600); err != nil {
		return CredentialSecretWriteResult{}, fmt.Errorf("%w: %v", ErrCredentialUnreadable, err)
	}
	return result, nil
}

type credentialSecretRecord struct {
	RecordType    string `json:"record_type"`
	CredentialRef string `json:"credential_ref"`
	ProtectedData string `json:"protected_data_b64"`
}

func credentialPath(ref string, storeRoot string) string {
	root := strings.TrimSpace(storeRoot)
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			root = filepath.Join(".genesis", "credentials")
		} else {
			root = filepath.Join(home, ".genesis", "credentials")
		}
	}
	normalized := NormalizeCredentialRef(ref)
	if normalized == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(normalized))
	digest := hex.EncodeToString(sum[:])[:24]
	suffix := strings.ReplaceAll(strings.TrimPrefix(normalized, "secret://"), "/", "-")
	token := refToken(suffix)
	if token == "" {
		token = "credential"
	}
	return filepath.Join(filepath.Clean(expandHome(root)), token+"-"+digest+".json")
}
