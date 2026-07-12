package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProviderProfilesProjectsSafeConfiguredMetadata(t *testing.T) {
	root := desktopTestTempDir(t)
	writeDesktopProviderModelsConfig(t, root)
	app := &App{providerControl: desktopProviderControlConfig{ConfigRoot: root, CredentialStoreRoot: filepath.Join(root, "credentials")}}

	profiles, err := app.ProviderProfiles()
	if err != nil {
		t.Fatalf("ProviderProfiles returned error: %v", err)
	}
	if len(profiles.Profiles) != 2 {
		t.Fatalf("profiles = %+v", profiles.Profiles)
	}
	for _, profile := range profiles.Profiles {
		if profile.ProfileID == "" || profile.ModelID == "" || profile.Protocol == "" {
			t.Fatalf("profile must contain safe identity metadata: %+v", profile)
		}
	}
}

func TestProviderProfilesTreatsMissingConfigAsAnEmptyFirstRunState(t *testing.T) {
	app := &App{providerControl: desktopProviderControlConfig{ConfigRoot: desktopTestTempDir(t)}}

	result, err := app.ProviderProfiles()
	if err != nil {
		t.Fatalf("ProviderProfiles returned error: %v", err)
	}
	if len(result.Profiles) != 0 || len(result.RoleBindings) != 0 {
		t.Fatalf("result = %+v", result)
	}
}

func TestSetupDeepSeekFlashCreatesSafeProfileWithoutReturningSecret(t *testing.T) {
	root := desktopTestTempDir(t)
	app := &App{providerControl: desktopProviderControlConfig{
		ConfigRoot:          root,
		CredentialStoreRoot: filepath.Join(root, "credentials"),
		secretProtector:     func(data []byte) ([]byte, error) { return append([]byte("sealed:"), data...), nil },
	}}

	result, err := app.SetupDeepSeekFlash("secret-key")
	if err != nil {
		t.Fatalf("SetupDeepSeekFlash returned error: %v", err)
	}
	if result.ProfileID != "deepseek-flash" || !result.CredentialPresent {
		t.Fatalf("result = %+v", result)
	}
	if strings.Contains(fmt.Sprintf("%+v", result), "secret-key") {
		t.Fatalf("result leaked secret: %+v", result)
	}
}

func TestVerifyProviderUsesTheKernelOwnedAdapterPath(t *testing.T) {
	var requestPath string
	var requestBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"readiness":"ready","model_role":"coder","profile_id":"cloud-glm","model":"glm-5-2"}`))
	}))
	defer server.Close()
	app := &App{ctx: context.Background(), client: NewKernelHTTPClient(server.URL, "token", server.Client())}

	result, err := app.VerifyProvider("coder", "cloud-glm")
	if err != nil {
		t.Fatalf("VerifyProvider returned error: %v", err)
	}
	if requestPath != "/providers/verify" || requestBody["model_role"] != "coder" || requestBody["profile_id"] != "cloud-glm" {
		t.Fatalf("request = path %q body %+v", requestPath, requestBody)
	}
	if result.Readiness != "ready" || result.Model != "glm-5-2" {
		t.Fatalf("result = %+v", result)
	}
}

func TestApplyProviderRoleRestartsOnlyOwnedSidecar(t *testing.T) {
	root := desktopTestTempDir(t)
	writeDesktopProviderModelsConfig(t, root)
	processes := make([]*fakeSidecarProcess, 0, 2)
	supervisor := NewLocalServiceSupervisor(LocalServiceSupervisorConfig{
		KernelBaseURL: defaultKernelBaseURL,
		LogDir:        root,
		launcher: func(context.Context, sidecarLaunchRequest) (sidecarProcess, error) {
			process := &fakeSidecarProcess{pid: 100 + len(processes)}
			processes = append(processes, process)
			return process, nil
		},
		readinessProbe: func(context.Context, string, string) sidecarReadinessResult {
			return sidecarReadinessResult{Ready: true}
		},
	})
	supervisor.StartKernel(context.Background())
	app := &App{
		ctx:             context.Background(),
		supervisor:      supervisor,
		providerControl: desktopProviderControlConfig{ConfigRoot: root, CredentialStoreRoot: filepath.Join(root, "credentials")},
	}

	result, err := app.ApplyProviderRole("coordinator", "cloud-glm")
	if err != nil {
		t.Fatalf("ApplyProviderRole returned error: %v", err)
	}
	if result.Status != providerActivationOwnedKernelRestarted || result.Binding.ProfileID != "cloud-glm" {
		t.Fatalf("result = %+v", result)
	}
	if len(processes) != 2 || processes[0].stopCalls != 1 {
		t.Fatalf("processes = %+v, want first owned sidecar stopped and exactly one replacement", processes)
	}
}

func TestApplyProviderRoleExternalLeavesKernelUntouched(t *testing.T) {
	root := desktopTestTempDir(t)
	writeDesktopProviderModelsConfig(t, root)
	supervisor := NewLocalServiceSupervisor(LocalServiceSupervisorConfig{KernelBaseURL: "http://127.0.0.1:9999", External: true})
	app := &App{
		ctx:             context.Background(),
		supervisor:      supervisor,
		providerControl: desktopProviderControlConfig{ConfigRoot: root, CredentialStoreRoot: filepath.Join(root, "credentials")},
	}

	result, err := app.ApplyProviderRole("coordinator", "cloud-glm")
	if err != nil {
		t.Fatalf("ApplyProviderRole returned error: %v", err)
	}
	if result.Status != providerActivationExternalKernelRestartRequired {
		t.Fatalf("result = %+v", result)
	}
}

func TestApplyProviderRoleRefusesAnActiveTurn(t *testing.T) {
	root := desktopTestTempDir(t)
	writeDesktopProviderModelsConfig(t, root)
	app := &App{providerControl: desktopProviderControlConfig{ConfigRoot: root}}
	app.desktopTurnMu.Lock()
	app.activeDesktopTurns = 1
	app.desktopTurnMu.Unlock()

	_, err := app.ApplyProviderRole("coordinator", "cloud-glm")
	if err == nil || err.Error() != providerActivationBlockedActiveTurn {
		t.Fatalf("ApplyProviderRole error = %v, want %q", err, providerActivationBlockedActiveTurn)
	}
}

func TestRotateProviderCredentialDoesNotExposeTheSecret(t *testing.T) {
	root := desktopTestTempDir(t)
	writeDesktopProviderModelsConfig(t, root)
	app := &App{providerControl: desktopProviderControlConfig{
		ConfigRoot:          root,
		CredentialStoreRoot: filepath.Join(root, "credentials"),
		secretProtector:     func(data []byte) ([]byte, error) { return append([]byte("sealed:"), data...), nil },
	}}

	result, err := app.RotateProviderCredential("cloud-glm", "long-lived-secret")
	if err != nil {
		t.Fatalf("RotateProviderCredential returned error: %v", err)
	}
	if result.ProfileID != "cloud-glm" || !result.CredentialPresent {
		t.Fatalf("result = %+v", result)
	}
	profiles, err := app.ProviderProfiles()
	if err != nil {
		t.Fatalf("ProviderProfiles returned error: %v", err)
	}
	for _, profile := range profiles.Profiles {
		if profile.ProfileID == "cloud-glm" && !profile.CredentialPresent {
			t.Fatal("rotated profile must project credential presence")
		}
	}
}

func writeDesktopProviderModelsConfig(t *testing.T, root string) {
	t.Helper()
	payload := `{
  "model_gateway": {"routes": {
    "local": {"protocol": "provider_command"},
    "cloud": {"protocol": "openai-chat-completions", "credential_ref": "secret://models/cloud/glm"}
  }},
  "active_model_profile_bindings": {"coordinator": "local-qwen"},
  "model_profiles": {
    "local": {"gateway": {"local-qwen": {"profile_id": "local-qwen", "model_id": "qwen-agentworld", "gateway_route": "local", "provider_adapter_id": "llama-cpp"}}},
    "cloud": {"gateway": {"cloud-glm": {"profile_id": "cloud-glm", "model_id": "glm-5-2", "gateway_route": "cloud", "provider_adapter_id": "zai-glm"}}}
  }
}`
	if err := os.WriteFile(filepath.Join(root, "models.json"), []byte(payload), 0o600); err != nil {
		t.Fatalf("write models config: %v", err)
	}
}
