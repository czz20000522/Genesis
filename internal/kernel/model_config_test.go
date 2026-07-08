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

func TestResolveOpenAICompatibleConfigFromGenesisCarriesProviderAdapterBinding(t *testing.T) {
	root := writeModelsConfig(t, map[string]any{
		"model_gateway": map[string]any{
			"protocol":       "openai-chat-completions",
			"base_url":       "https://provider.example.com/api",
			"credential_ref": "secret://models/provider/default",
		},
		"active_model_profile_bindings": map[string]any{
			DefaultModelRole: "deepseek-flash",
		},
		"model_profiles": map[string]any{
			"cloud": map[string]any{
				"gateway": map[string]any{
					"deepseek-flash": map[string]any{
						"profile_id":                  "deepseek-flash",
						"model_id":                    "deepseek-v4-flash",
						"provider_adapter_id":         "deepseek",
						"provider_adapter_profile_id": "deepseek-v4-flash",
						"hidden_reasoning_policy":     "discard",
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
	if config.Adapter.AdapterID != "deepseek" || config.Adapter.ProfileID != "deepseek-v4-flash" {
		t.Fatalf("adapter binding = %+v, want DeepSeek profile binding", config.Adapter)
	}
	if config.Adapter.TransportProtocol != "openai-chat-completions" {
		t.Fatalf("adapter transport = %q, want openai-chat-completions", config.Adapter.TransportProtocol)
	}
	if config.Adapter.HiddenReasoningPolicy != "discard" {
		t.Fatalf("hidden reasoning policy = %q, want discard", config.Adapter.HiddenReasoningPolicy)
	}
}

func TestResolveOpenAICompatibleConfigFromGenesisRejectsAdapterPolicyWithoutAdapter(t *testing.T) {
	root := writeModelsConfig(t, map[string]any{
		"model_gateway": map[string]any{
			"protocol":       "openai-chat-completions",
			"base_url":       "https://provider.example.com/api",
			"credential_ref": "secret://models/provider/default",
		},
		"active_model_profile_bindings": map[string]any{
			DefaultModelRole: "provider-fast",
		},
		"model_profiles": map[string]any{
			"cloud": map[string]any{
				"gateway": map[string]any{
					"provider-fast": map[string]any{
						"profile_id":              "provider-fast",
						"model_id":                "provider-model-fast",
						"hidden_reasoning_policy": "discard",
					},
				},
			},
		},
	})

	_, err := ResolveOpenAICompatibleConfigFromGenesis(GenesisModelConfigRequest{
		ConfigRoot: root,
		SecretResolver: func(_ string, _ string) (string, error) {
			return "test-key", nil
		},
	})
	if !errors.Is(err, ErrGenesisModelProviderAdapterInvalid) {
		t.Fatalf("error = %v, want provider adapter invalid", err)
	}
	if ProviderConfigReason(err) != "provider_adapter_invalid" {
		t.Fatalf("reason = %q, want provider_adapter_invalid", ProviderConfigReason(err))
	}
}

func TestResolveProviderConfigFromGenesisSelectsCommandProviderRoute(t *testing.T) {
	workingDir := testTempDir(t)
	root := writeModelsConfig(t, map[string]any{
		"model_gateway": map[string]any{
			"protocol":            "provider_command",
			"command":             "fallback-provider",
			"args":                []any{"fallback"},
			"env":                 []any{"FALLBACK_ENV=1"},
			"working_dir":         testTempDir(t),
			"request_timeout_sec": 90,
			"routes": map[string]any{
				"command-primary": map[string]any{
					"command":             os.Args[0],
					"args":                []any{"-test.run=TestProviderCommandAdapterHelper", "--", "final"},
					"env":                 []any{"GENESIS_PROVIDER_COMMAND_HELPER=1"},
					"working_dir":         workingDir,
					"request_timeout_sec": 12,
				},
			},
		},
		"active_model_profile_bindings": map[string]any{
			DefaultModelRole: "command-profile",
		},
		"model_profiles": map[string]any{
			"local": map[string]any{
				"gateway": map[string]any{
					"command-profile": map[string]any{
						"profile_id":    "command-profile",
						"model_id":      "command-model",
						"gateway_route": "command-primary",
					},
				},
			},
		},
	})

	resolved, err := ResolveProviderConfigFromGenesis(GenesisModelConfigRequest{ConfigRoot: root})
	if err != nil {
		t.Fatalf("ResolveProviderConfigFromGenesis returned error: %v", err)
	}
	if resolved.Kind != "provider_command" {
		t.Fatalf("kind = %q, want provider_command", resolved.Kind)
	}
	if resolved.Command.Command != os.Args[0] {
		t.Fatalf("command = %q, want test binary", resolved.Command.Command)
	}
	if strings.Join(resolved.Command.Args, " ") != "-test.run=TestProviderCommandAdapterHelper -- final" {
		t.Fatalf("args = %v, want route args", resolved.Command.Args)
	}
	if strings.Join(resolved.Command.Env, " ") != "GENESIS_PROVIDER_COMMAND_HELPER=1" {
		t.Fatalf("env = %v, want route env", resolved.Command.Env)
	}
	if resolved.Command.WorkingDir != workingDir {
		t.Fatalf("working dir = %q, want route working dir", resolved.Command.WorkingDir)
	}
	if resolved.Command.Model != "command-model" {
		t.Fatalf("model = %q, want command-model", resolved.Command.Model)
	}
	if resolved.Command.RequestTimeout != 12*time.Second {
		t.Fatalf("timeout = %s, want 12s", resolved.Command.RequestTimeout)
	}
}

func TestResolveProviderConfigFromGenesisRejectsSecretCommandEnvironment(t *testing.T) {
	for _, env := range []string{
		"GENESIS_PROVIDER_API_KEY=sk-testsecret123",
		"Authorization=Bearer tokentest123456",
		"PASSWORD=plain-text-password",
		"GENESIS_PROVIDER_SECRET=secret://models/provider/default",
	} {
		t.Run(env, func(t *testing.T) {
			root := writeModelsConfig(t, map[string]any{
				"model_gateway": map[string]any{
					"protocol": "provider_command",
					"command":  os.Args[0],
					"env":      []any{env},
				},
				"active_model_profile_bindings": map[string]any{
					DefaultModelRole: "command-profile",
				},
				"model_profiles": map[string]any{
					"local": map[string]any{
						"gateway": map[string]any{
							"command-profile": map[string]any{
								"profile_id": "command-profile",
								"model_id":   "command-model",
							},
						},
					},
				},
			})

			_, err := ResolveProviderConfigFromGenesis(GenesisModelConfigRequest{ConfigRoot: root})
			if !errors.Is(err, ErrGenesisModelProviderCommandEnvRejected) {
				t.Fatalf("error = %v, want provider command env rejected", err)
			}
			if ProviderConfigReason(err) != "provider_command_env_secret_rejected" {
				t.Fatalf("reason = %q, want provider_command_env_secret_rejected", ProviderConfigReason(err))
			}
		})
	}
}

func TestResolveProviderConfigFromGenesisRejectsInvalidModelsJSON(t *testing.T) {
	root := testTempDir(t)
	if err := os.WriteFile(filepath.Join(root, "models.json"), []byte(`{"model_gateway": "sk-bad-config-secret"`), 0o644); err != nil {
		t.Fatalf("write invalid models.json: %v", err)
	}

	_, err := ResolveProviderConfigFromGenesis(GenesisModelConfigRequest{ConfigRoot: root})
	if !errors.Is(err, ErrGenesisModelConfigInvalid) {
		t.Fatalf("error = %v, want model config invalid", err)
	}
	if errors.Is(err, ErrGenesisModelConfigMissing) {
		t.Fatalf("error = %v, must not classify invalid config as missing", err)
	}
	if ProviderConfigReason(err) != "provider_config_invalid" {
		t.Fatalf("reason = %q, want provider_config_invalid", ProviderConfigReason(err))
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
	if status.Name != "openai-compatible" || status.Readiness != ReadinessNotReady || status.ReadinessReason != "provider_config_missing" {
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
	root := testTempDir(t)
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "models.json"), encoded, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return root
}
