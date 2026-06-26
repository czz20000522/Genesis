package kernel

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSetupOpenAICompatibleProviderWritesConfigAndProtectedCredential(t *testing.T) {
	configRoot := testTempDir(t)
	credentialRoot := testTempDir(t)
	apiKey := "sk-setup-secret"

	result, err := SetupOpenAICompatibleProvider(OpenAICompatibleProviderSetupRequest{
		ConfigRoot:          configRoot,
		CredentialStoreRoot: credentialRoot,
		ModelRole:           DefaultModelRole,
		ProfileID:           "primary",
		GatewayRoute:        "main",
		BaseURL:             "https://provider.example.com/api",
		ModelID:             "provider-model",
		CredentialRef:       "secret://models/provider/primary",
		APIKey:              apiKey,
		RequestTimeout:      45 * time.Second,
		SecretProtector: func(secret []byte) ([]byte, error) {
			if string(secret) != apiKey {
				t.Fatalf("secret passed to protector = %q", string(secret))
			}
			return []byte("protected-provider-key"), nil
		},
		SecretResolver: func(ref string, storeRoot string) (string, error) {
			if ref != "secret://models/provider/primary" {
				t.Fatalf("resolver ref = %q", ref)
			}
			if storeRoot != credentialRoot {
				t.Fatalf("resolver store root = %q", storeRoot)
			}
			return apiKey, nil
		},
		Verify: true,
	})
	if err != nil {
		t.Fatalf("SetupOpenAICompatibleProvider returned error: %v", err)
	}
	if !result.Verified {
		t.Fatal("result.Verified = false, want true")
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if strings.Contains(string(resultJSON), apiKey) {
		t.Fatalf("setup result leaked API key: %s", string(resultJSON))
	}

	configPayload, err := os.ReadFile(filepath.Join(configRoot, "models.json"))
	if err != nil {
		t.Fatalf("read models.json: %v", err)
	}
	if strings.Contains(string(configPayload), apiKey) {
		t.Fatalf("models.json leaked API key: %s", string(configPayload))
	}
	if !strings.Contains(string(configPayload), "secret://models/provider/primary") {
		t.Fatalf("models.json missing credential ref: %s", string(configPayload))
	}

	credentialPayload, err := os.ReadFile(result.CredentialPath)
	if err != nil {
		t.Fatalf("read credential record: %v", err)
	}
	if strings.Contains(string(credentialPayload), apiKey) {
		t.Fatalf("credential record leaked API key: %s", string(credentialPayload))
	}
	var credentialRecord localSecretRecord
	if err := json.Unmarshal(credentialPayload, &credentialRecord); err != nil {
		t.Fatalf("decode credential record: %v", err)
	}
	if credentialRecord.RecordType != "local_credential_secret" || credentialRecord.CredentialRef != "secret://models/provider/primary" {
		t.Fatalf("credential record = %+v", credentialRecord)
	}
	if strings.TrimSpace(credentialRecord.ProtectedData) == "" {
		t.Fatalf("credential record missing protected payload: %s", string(credentialPayload))
	}

	resolved, err := ResolveOpenAICompatibleConfigFromGenesis(GenesisModelConfigRequest{
		ConfigRoot:          configRoot,
		CredentialStoreRoot: credentialRoot,
		ModelRole:           DefaultModelRole,
		SecretResolver: func(ref string, storeRoot string) (string, error) {
			if ref != "secret://models/provider/primary" || storeRoot != credentialRoot {
				t.Fatalf("unexpected resolver input: %q %q", ref, storeRoot)
			}
			return apiKey, nil
		},
	})
	if err != nil {
		t.Fatalf("ResolveOpenAICompatibleConfigFromGenesis returned error: %v", err)
	}
	if resolved.BaseURL != "https://provider.example.com/api" || resolved.Model != "provider-model" || resolved.APIKey != apiKey {
		t.Fatalf("resolved config = %+v", resolved)
	}
	if resolved.RequestTimeout != 45*time.Second {
		t.Fatalf("timeout = %s, want 45s", resolved.RequestTimeout)
	}
}

func TestSetupOpenAICompatibleProviderWritesAdapterProfileBinding(t *testing.T) {
	configRoot := testTempDir(t)
	credentialRoot := testTempDir(t)
	apiKey := "sk-scnet-secret"

	_, err := SetupOpenAICompatibleProvider(OpenAICompatibleProviderSetupRequest{
		ConfigRoot:          configRoot,
		CredentialStoreRoot: credentialRoot,
		ModelRole:           DefaultModelRole,
		ProfileID:           "scnet-deepseek-r1-distill-qwen-7b",
		GatewayRoute:        "scnet",
		ProviderAdapterID:   "scnet",
		AdapterProfileID:    "DeepSeek-R1-Distill-Qwen-7B",
		BaseURL:             "https://api.scnet.cn/api/llm/v1",
		ModelID:             "DeepSeek-R1-Distill-Qwen-7B",
		CredentialRef:       "secret://models/scnet/local",
		APIKey:              apiKey,
		RequestTimeout:      60 * time.Second,
		SecretProtector: func(secret []byte) ([]byte, error) {
			if string(secret) != apiKey {
				t.Fatalf("secret passed to protector = %q", string(secret))
			}
			return []byte("protected-scnet-key"), nil
		},
	})
	if err != nil {
		t.Fatalf("SetupOpenAICompatibleProvider returned error: %v", err)
	}

	configPayload, err := os.ReadFile(filepath.Join(configRoot, "models.json"))
	if err != nil {
		t.Fatalf("read models.json: %v", err)
	}
	for _, want := range []string{
		`"provider_adapter_id": "scnet"`,
		`"provider_adapter_profile_id": "DeepSeek-R1-Distill-Qwen-7B"`,
		`"base_url": "https://api.scnet.cn/api/llm/v1"`,
		`"credential_ref": "secret://models/scnet/local"`,
	} {
		if !strings.Contains(string(configPayload), want) {
			t.Fatalf("models.json missing %q: %s", want, string(configPayload))
		}
	}
	resolved, err := ResolveProviderConfigFromGenesis(GenesisModelConfigRequest{
		ConfigRoot:          configRoot,
		CredentialStoreRoot: credentialRoot,
		SecretResolver: func(ref string, storeRoot string) (string, error) {
			if ref != "secret://models/scnet/local" || storeRoot != credentialRoot {
				t.Fatalf("unexpected resolver input: %q %q", ref, storeRoot)
			}
			return apiKey, nil
		},
	})
	if err != nil {
		t.Fatalf("ResolveProviderConfigFromGenesis returned error: %v", err)
	}
	if resolved.Kind != "openai-compatible" {
		t.Fatalf("kind = %q, want openai-compatible", resolved.Kind)
	}
	if resolved.OpenAICompatible.Adapter.AdapterID != "scnet" || resolved.OpenAICompatible.Adapter.ProfileID != "DeepSeek-R1-Distill-Qwen-7B" {
		t.Fatalf("adapter binding = %+v, want SCNet profile binding", resolved.OpenAICompatible.Adapter)
	}
}

func TestSetupOpenAICompatibleProviderDryRunWritesNothing(t *testing.T) {
	configRoot := testTempDir(t)
	credentialRoot := testTempDir(t)

	result, err := SetupOpenAICompatibleProvider(OpenAICompatibleProviderSetupRequest{
		ConfigRoot:          configRoot,
		CredentialStoreRoot: credentialRoot,
		BaseURL:             "https://provider.example.com/api",
		ModelID:             "provider-model",
		CredentialRef:       "secret://models/provider/default",
		DryRun:              true,
	})
	if err != nil {
		t.Fatalf("dry-run setup returned error: %v", err)
	}
	if !result.DryRun {
		t.Fatal("result.DryRun = false, want true")
	}
	if _, err := os.Stat(filepath.Join(configRoot, "models.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("models.json stat err = %v, want not exist", err)
	}
	if _, err := os.Stat(result.CredentialPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("credential stat err = %v, want not exist", err)
	}
}

func TestCorruptSetupCredentialBlocksProviderConfig(t *testing.T) {
	configRoot := testTempDir(t)
	credentialRoot := testTempDir(t)

	result, err := SetupOpenAICompatibleProvider(OpenAICompatibleProviderSetupRequest{
		ConfigRoot:          configRoot,
		CredentialStoreRoot: credentialRoot,
		BaseURL:             "https://provider.example.com/api",
		ModelID:             "provider-model",
		CredentialRef:       "secret://models/provider/corrupt",
		APIKey:              "sk-corrupt-secret",
		SecretProtector: func([]byte) ([]byte, error) {
			return []byte("protected"), nil
		},
		SecretResolver: func(string, string) (string, error) {
			return "sk-corrupt-secret", nil
		},
		Verify: true,
	})
	if err != nil {
		t.Fatalf("setup returned error: %v", err)
	}
	if err := os.WriteFile(result.CredentialPath, []byte("{bad json\n"), 0o600); err != nil {
		t.Fatalf("write corrupt credential: %v", err)
	}
	_, err = ResolveOpenAICompatibleConfigFromGenesis(GenesisModelConfigRequest{
		ConfigRoot:          configRoot,
		CredentialStoreRoot: credentialRoot,
	})
	if !errors.Is(err, ErrGenesisModelCredentialMissing) {
		t.Fatalf("error = %v, want credential missing", err)
	}
	if ProviderConfigReason(err) != "provider_credential_missing" {
		t.Fatalf("reason = %q, want provider_credential_missing", ProviderConfigReason(err))
	}
}
