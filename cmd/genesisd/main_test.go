package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"genesis/internal/kernel"
	"genesis/internal/testsupport"
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
	if status.Name != "provider_command" || status.Readiness != kernel.ReadinessReady {
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
	if status.Name != "provider_command" || status.Readiness != kernel.ReadinessReady {
		t.Fatalf("provider status = %+v, want ok provider_command", status)
	}
}

func TestBuildProviderCanPassExplicitCommandEnvironment(t *testing.T) {
	provider, err := buildProvider(providerBuildRequest{
		name:        "provider_command",
		command:     os.Args[0],
		commandArgs: []string{"-test.run=TestDaemonProviderCommandEnvHelper"},
		commandEnv:  []string{"GENESIS_DAEMON_PROVIDER_ENV_HELPER=1", "GENESIS_PROVIDER_COMMAND_SENTINEL=explicit"},
		model:       "command-model",
	})
	if err != nil {
		t.Fatalf("buildProvider returned error: %v", err)
	}
	resp, err := provider.Complete(context.Background(), kernel.ModelRequest{
		SessionID: "session",
		TurnID:    "turn",
		InputItems: []kernel.ModelInputItem{{
			Kind: "user_text",
			Text: "hello",
		}},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if resp.Text != "env explicit" {
		t.Fatalf("response text = %q, want env explicit", resp.Text)
	}
}

func TestBuildProviderBlocksSecretShapedCommandEnvironment(t *testing.T) {
	provider, err := buildProvider(providerBuildRequest{
		name:       "provider_command",
		command:    os.Args[0],
		commandEnv: []string{"GENESIS_PROVIDER_API_KEY=sk-testsecret123"},
		model:      "command-model",
	})
	if err != nil {
		t.Fatalf("buildProvider returned error: %v", err)
	}
	status := provider.Ready()
	if status.Readiness != kernel.ReadinessNotReady || status.ReadinessReason != "provider_command_env_secret_rejected" {
		t.Fatalf("provider status = %+v, want provider_command_env_secret_rejected", status)
	}
}

func TestDaemonProviderCommandEnvHelper(t *testing.T) {
	if os.Getenv("GENESIS_DAEMON_PROVIDER_ENV_HELPER") != "1" {
		return
	}
	if value := os.Getenv("GENESIS_PROVIDER_COMMAND_SENTINEL"); value != "explicit" {
		t.Fatalf("provider command env = %q, want explicit", value)
	}
	_, _ = os.Stdout.WriteString(`{"kind":"final","text":"env explicit"}` + "\n")
	os.Exit(0)
}

func writeDaemonModelsConfig(t *testing.T, payload map[string]any) string {
	t.Helper()
	root := testsupport.ProjectTempDir(t, "genesisd-models-config")
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "models.json"), encoded, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return root
}
