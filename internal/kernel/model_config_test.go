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
}

func TestResolveOpenAICompatibleConfigFromGenesisRejectsPartialAdapterBinding(t *testing.T) {
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
						"profile_id":                  "provider-fast",
						"model_id":                    "provider-model-fast",
						"provider_adapter_profile_id": "deepseek-v4-flash",
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
						"profile_id":                  "command-profile",
						"model_id":                    "command-model",
						"gateway_route":               "command-primary",
						"provider_adapter_id":         "llama.cpp",
						"provider_adapter_profile_id": "command-model-profile",
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
	if resolved.Command.Adapter.AdapterID != "llama.cpp" || resolved.Command.Adapter.ProfileID != "command-model-profile" || resolved.Command.Adapter.TransportProtocol != "provider_command" {
		t.Fatalf("adapter binding = %+v, want llama.cpp command profile binding", resolved.Command.Adapter)
	}
	if resolved.Command.RequestTimeout != 12*time.Second {
		t.Fatalf("timeout = %s, want 12s", resolved.Command.RequestTimeout)
	}
}

func TestResolveProviderConfigFromGenesisAllowsExplicitUnboundedCommandRoute(t *testing.T) {
	root := writeModelsConfig(t, map[string]any{
		"model_gateway": map[string]any{
			"protocol": "provider_command",
			"routes": map[string]any{
				"local-command": map[string]any{
					"protocol":                "provider_command",
					"command":                 os.Args[0],
					"request_timeout_sec":     0,
					"allow_unbounded_request": true,
				},
			},
		},
		"active_model_profile_bindings": map[string]any{DefaultModelRole: "local-command-profile"},
		"model_profiles": map[string]any{
			"local": map[string]any{
				"gateway": map[string]any{
					"local-command-profile": map[string]any{
						"profile_id":    "local-command-profile",
						"model_id":      "command-model",
						"gateway_route": "local-command",
					},
				},
			},
		},
	})

	resolved, err := ResolveProviderConfigFromGenesis(GenesisModelConfigRequest{ConfigRoot: root})
	if err != nil {
		t.Fatalf("ResolveProviderConfigFromGenesis returned error: %v", err)
	}
	if !resolved.Command.AllowUnboundedRequest {
		t.Fatalf("AllowUnboundedRequest = false, want true")
	}
	if resolved.Command.RequestTimeout != 0 {
		t.Fatalf("RequestTimeout = %s, want no command deadline", resolved.Command.RequestTimeout)
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

func TestResolveParentWorkerRuntimeFromGenesisProjectsRoleBindings(t *testing.T) {
	root := writeModelsConfig(t, map[string]any{
		"model_gateway": map[string]any{
			"protocol":       "openai-chat-completions",
			"base_url":       "https://provider.example.com/api",
			"credential_ref": "secret://models/provider/default",
			"routes": map[string]any{
				"local-qwen": map[string]any{
					"base_url":       "http://127.0.0.1:8080/v1",
					"credential_ref": "secret://models/local/qwen",
				},
			},
		},
		"active_model_profile_bindings": map[string]any{
			DefaultModelRole: "parent-profile",
		},
		"model_profiles": map[string]any{
			"cloud": map[string]any{
				"gateway": map[string]any{
					"parent-profile": map[string]any{
						"profile_id":    "parent-profile",
						"model_id":      "frontier-parent",
						"gateway_route": "cloud-parent",
					},
				},
			},
			"local": map[string]any{
				"gateway": map[string]any{
					"local-worker-profile": map[string]any{
						"profile_id":            "local-worker-profile",
						"model_id":              "qwen-agentworld",
						"gateway_route":         "local-qwen",
						"context_window_tokens": 262144,
					},
				},
			},
		},
		"parent_worker_runtime": map[string]any{
			"parents": map[string]any{
				DefaultModelRole: map[string]any{
					"allowed_worker_roles": []any{"local-small-worker"},
					"default_worker_role":  "local-small-worker",
					"can_create_workers":   true,
				},
			},
			"worker_roles": map[string]any{
				"local-small-worker": map[string]any{
					"profile_id":         "local-worker-profile",
					"tool_set":           []any{"resource_read", "source_read", "resource_read"},
					"context_policy_ref": "context:diff-plus-issue",
					"max_parallel":       1,
					"leaf_only":          true,
				},
			},
		},
	})

	projection, err := ResolveParentWorkerRuntimeFromGenesis(ParentWorkerRuntimeRequest{ConfigRoot: root})
	if err != nil {
		t.Fatalf("ResolveParentWorkerRuntimeFromGenesis returned error: %v", err)
	}
	if projection.Parent.ParentID != DefaultModelRole {
		t.Fatalf("parent id = %q", projection.Parent.ParentID)
	}
	if projection.Parent.ProfileID != "parent-profile" || projection.Parent.ModelID != "frontier-parent" {
		t.Fatalf("parent projection = %+v", projection.Parent)
	}
	if !projection.Parent.CanCreateWorkers || projection.Parent.DefaultWorkerRole != "local-small-worker" {
		t.Fatalf("parent worker controls = %+v", projection.Parent)
	}
	if len(projection.WorkerRoles) != 1 {
		t.Fatalf("worker roles = %d, want 1", len(projection.WorkerRoles))
	}
	worker := projection.WorkerRoles[0]
	if worker.RoleID != "local-small-worker" || worker.ProfileID != "local-worker-profile" || worker.ModelID != "qwen-agentworld" {
		t.Fatalf("worker projection = %+v", worker)
	}
	if worker.ProviderRoute != "local-qwen" || worker.ContextWindowTokens != 262144 {
		t.Fatalf("worker provider/context = %+v", worker)
	}
	if strings.Join(worker.ToolSet, ",") != "resource_read,source_read" {
		t.Fatalf("tool set = %v, want sorted unique preset tools", worker.ToolSet)
	}
	if worker.ContextPolicyRef != "context:diff-plus-issue" || worker.MaxParallel != 1 || !worker.LeafOnly {
		t.Fatalf("worker controls = %+v", worker)
	}
	encoded, err := json.Marshal(projection)
	if err != nil {
		t.Fatalf("marshal projection: %v", err)
	}
	for _, forbidden := range []string{"secret://", "credential", "sandbox", "permission"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("projection leaked %q: %s", forbidden, string(encoded))
		}
	}
}

func TestResolveParentWorkerRuntimeFromGenesisRejectsInvalidBindings(t *testing.T) {
	for _, tc := range []struct {
		name    string
		runtime map[string]any
		want    error
	}{
		{
			name: "unknown allowed worker role",
			runtime: map[string]any{
				"parents": map[string]any{
					DefaultModelRole: map[string]any{
						"allowed_worker_roles": []any{"missing-worker"},
						"can_create_workers":   true,
					},
				},
			},
			want: ErrGenesisWorkerRoleBindingMissing,
		},
		{
			name: "unknown worker tool",
			runtime: map[string]any{
				"parents": map[string]any{
					DefaultModelRole: map[string]any{
						"allowed_worker_roles": []any{"local-worker"},
						"can_create_workers":   true,
					},
				},
				"worker_roles": map[string]any{
					"local-worker": map[string]any{
						"profile_id": "worker-profile",
						"tool_set":   []any{"unknown_tool"},
						"leaf_only":  true,
					},
				},
			},
			want: ErrGenesisWorkerRoleBindingInvalid,
		},
		{
			name: "worker profile missing",
			runtime: map[string]any{
				"parents": map[string]any{
					DefaultModelRole: map[string]any{
						"allowed_worker_roles": []any{"local-worker"},
						"can_create_workers":   true,
					},
				},
				"worker_roles": map[string]any{
					"local-worker": map[string]any{
						"profile_id": "missing-profile",
						"leaf_only":  true,
					},
				},
			},
			want: ErrGenesisModelProfileMissing,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := writeModelsConfig(t, map[string]any{
				"model_gateway": map[string]any{
					"protocol":       "openai-chat-completions",
					"base_url":       "https://provider.example.com/api",
					"credential_ref": "secret://models/provider/default",
				},
				"active_model_profile_bindings": map[string]any{
					DefaultModelRole: "parent-profile",
				},
				"model_profiles": map[string]any{
					"cloud": map[string]any{
						"gateway": map[string]any{
							"parent-profile": map[string]any{
								"profile_id": "parent-profile",
								"model_id":   "frontier-parent",
							},
							"worker-profile": map[string]any{
								"profile_id": "worker-profile",
								"model_id":   "worker-model",
							},
						},
					},
				},
				"parent_worker_runtime": tc.runtime,
			})

			_, err := ResolveParentWorkerRuntimeFromGenesis(ParentWorkerRuntimeRequest{ConfigRoot: root})
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
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
