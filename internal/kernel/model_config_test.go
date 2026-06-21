package kernel

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestResolveOpenAICompatibleConfigFromGenesisSelectsRoleProfile(t *testing.T) {
	root := writeModelsConfig(t, map[string]any{
		"model_gateway": map[string]any{
			"base_url":            "https://gateway.example.com/api",
			"credential_ref":      "secret://models/gateway/default",
			"protocol":            "openai-chat-completions",
			"request_timeout_sec": 90,
			"routes": map[string]any{
				"provider-primary": map[string]any{
					"base_url":            "https://provider.example.com/api",
					"credential_ref":      "secret://models/provider/default",
					"request_timeout_sec": 45,
				},
			},
		},
		"active_model_profile_bindings": map[string]any{
			DefaultModelRole: "provider-fast",
		},
		"model_profiles": map[string]any{
			"cloud": map[string]any{
				"gateway": map[string]any{
					"provider-fast": map[string]any{
						"profile_id":    "provider-fast",
						"model_id":      "provider-model-fast",
						"gateway_route": "provider-primary",
					},
				},
			},
		},
	})

	config, err := ResolveOpenAICompatibleConfigFromGenesis(GenesisModelConfigRequest{
		ConfigRoot: root,
		SecretResolver: func(ref string, _ string) (string, error) {
			if ref != "secret://models/provider/default" {
				t.Fatalf("credential ref = %q, want provider route ref", ref)
			}
			return "test-key", nil
		},
	})
	if err != nil {
		t.Fatalf("ResolveOpenAICompatibleConfigFromGenesis returned error: %v", err)
	}
	if config.BaseURL != "https://provider.example.com/api" {
		t.Fatalf("base URL = %q", config.BaseURL)
	}
	if config.Model != "provider-model-fast" {
		t.Fatalf("model = %q", config.Model)
	}
	if config.APIKey != "test-key" {
		t.Fatalf("api key = %q", config.APIKey)
	}
	if config.RequestTimeout != 45*time.Second {
		t.Fatalf("timeout = %s, want 45s", config.RequestTimeout)
	}
}

func TestResolveOpenAICompatibleConfigFromGenesisRejectsUnsupportedProtocol(t *testing.T) {
	root := writeModelsConfig(t, minimalModelsConfig(map[string]any{
		"protocol": "openai-responses",
	}))

	_, err := ResolveOpenAICompatibleConfigFromGenesis(GenesisModelConfigRequest{
		ConfigRoot: root,
		SecretResolver: func(_ string, _ string) (string, error) {
			return "test-key", nil
		},
	})
	if !errors.Is(err, ErrGenesisModelProtocolUnsupported) {
		t.Fatalf("error = %v, want protocol unsupported", err)
	}
	if ProviderConfigReason(err) != "provider_protocol_unsupported" {
		t.Fatalf("reason = %q", ProviderConfigReason(err))
	}
}

func TestResolveOpenAICompatibleConfigFromGenesisRejectsMissingCredential(t *testing.T) {
	root := writeModelsConfig(t, minimalModelsConfig(nil))

	_, err := ResolveOpenAICompatibleConfigFromGenesis(GenesisModelConfigRequest{
		ConfigRoot: root,
		SecretResolver: func(_ string, _ string) (string, error) {
			return "", nil
		},
	})
	if !errors.Is(err, ErrGenesisModelCredentialMissing) {
		t.Fatalf("error = %v, want credential missing", err)
	}
	if ProviderConfigReason(err) != "provider_credential_missing" {
		t.Fatalf("reason = %q", ProviderConfigReason(err))
	}
}

func TestBlockedProviderReportsReadinessBlocker(t *testing.T) {
	provider := NewBlockedProvider("openai-compatible", "provider_config_missing")

	status := provider.Ready()
	if status.Name != "openai-compatible" || status.Status != "blocked" || status.Reason != "provider_config_missing" {
		t.Fatalf("status = %+v", status)
	}
	if _, err := provider.Complete(nil, ModelRequest{}); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("Complete error = %v, want provider unavailable", err)
	}
}

func TestLocalSecretPathMatchesGenesisCredentialStoreShape(t *testing.T) {
	path := localSecretPath("secret://models/deepseek/default", "C:/tmp/genesis-credentials")
	want := filepath.Clean("C:/tmp/genesis-credentials/models-deepseek-default-77e7f1953ec9617924531da2.json")
	if filepath.Clean(path) != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
}

func minimalModelsConfig(gatewayOverride map[string]any) map[string]any {
	gateway := map[string]any{
		"base_url":       "https://provider.example.com/api",
		"credential_ref": "secret://models/provider/default",
		"protocol":       "openai-chat-completions",
	}
	for key, value := range gatewayOverride {
		gateway[key] = value
	}
	return map[string]any{
		"model_gateway": gateway,
		"active_model_profile_bindings": map[string]any{
			DefaultModelRole: "provider-fast",
		},
		"model_profiles": map[string]any{
			"cloud": map[string]any{
				"gateway": map[string]any{
					"provider-fast": map[string]any{
						"profile_id": "provider-fast",
						"model_id":   "provider-model-fast",
					},
				},
			},
		},
	}
}

func writeModelsConfig(t *testing.T, payload map[string]any) string {
	t.Helper()
	root := t.TempDir()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "models.json"), encoded, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return root
}
