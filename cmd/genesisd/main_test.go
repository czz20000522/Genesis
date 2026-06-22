package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"genesis/internal/kernel"
)

func TestBuildProviderFromGenesisConfigCanSelectCommandProvider(t *testing.T) {
	configRoot := writeDaemonModelsConfig(t, map[string]any{
		"model_gateway": map[string]any{
			"protocol": "provider_command",
			"command":  os.Args[0],
		},
		"active_model_profile_bindings": map[string]any{
			kernel.DefaultModelRole: "command-profile",
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

	provider, err := buildProvider(providerBuildRequest{
		name:       "genesis-config",
		configRoot: configRoot,
	})
	if err != nil {
		t.Fatalf("buildProvider returned error: %v", err)
	}
	status := provider.Ready()
	if status.Name != "provider_command" || status.Status != "ok" {
		t.Fatalf("provider status = %+v, want ok provider_command", status)
	}
	if _, ok := provider.(*kernel.CommandProvider); !ok {
		t.Fatalf("provider type = %T, want *kernel.CommandProvider", provider)
	}
}

func TestBuildProviderCanSelectCommandProviderDirectly(t *testing.T) {
	provider, err := buildProvider(providerBuildRequest{
		name:    "provider_command",
		command: os.Args[0],
		model:   "command-model",
	})
	if err != nil {
		t.Fatalf("buildProvider returned error: %v", err)
	}
	status := provider.Ready()
	if status.Name != "provider_command" || status.Status != "ok" {
		t.Fatalf("provider status = %+v, want ok provider_command", status)
	}
}

func writeDaemonModelsConfig(t *testing.T, payload map[string]any) string {
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
