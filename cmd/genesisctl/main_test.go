package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"genesis/internal/kernel"
	"genesis/internal/testsupport"
)

func TestProviderUseDeepSeekPresetDryRunDoesNotRequireAPIKey(t *testing.T) {
	configRoot := testsupport.ProjectTempDir(t, "genesisctl-provider-use-dry-run-config")
	credentialRoot := testsupport.ProjectTempDir(t, "genesisctl-provider-use-dry-run-credentials")
	var stdout bytes.Buffer

	err := run([]string{
		"provider", "use", "deepseek/deepseek-v4-flash",
		"-config-root", configRoot,
		"-credential-store-root", credentialRoot,
		"-dry-run",
	}, strings.NewReader(""), &stdout)
	if err != nil {
		t.Fatalf("provider use dry-run returned error: %v", err)
	}

	response := decodeProviderSetupResponse(t, stdout.Bytes())
	if !response["ok"].(bool) || !response["dry_run"].(bool) {
		t.Fatalf("response = %+v, want dry-run ok", response)
	}
	assertStringField(t, response, "provider_id", "deepseek")
	assertStringField(t, response, "model_id", "deepseek-v4-flash")
	assertStringField(t, response, "base_url", "https://api.deepseek.com")
	assertStringField(t, response, "profile_id", "deepseek-flash")
	assertStringField(t, response, "gateway_route", "deepseek")
	assertStringField(t, response, "credential_ref", "secret://models/deepseek/local")
	assertFloatField(t, response, "context_window_tokens", 1000000)
	if _, err := os.Stat(filepath.Join(configRoot, "models.json")); !os.IsNotExist(err) {
		t.Fatalf("models.json stat err = %v, want not exist", err)
	}
}

func TestProviderUseDeepSeekPresetWritesConfigWithoutPrintingSecret(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("real local credential setup uses Windows DPAPI")
	}
	configRoot := testsupport.ProjectTempDir(t, "genesisctl-provider-use-config")
	credentialRoot := testsupport.ProjectTempDir(t, "genesisctl-provider-use-credentials")
	secret := "sk-preset-secret"
	var stdout bytes.Buffer

	err := run([]string{
		"provider", "use", "deepseek/deepseek-v4-flash",
		"-config-root", configRoot,
		"-credential-store-root", credentialRoot,
		"-api-key-stdin",
	}, strings.NewReader(secret), &stdout)
	if err != nil {
		t.Fatalf("provider use returned error: %v", err)
	}
	if strings.Contains(stdout.String(), secret) {
		t.Fatalf("provider use output leaked secret: %s", stdout.String())
	}

	response := decodeProviderSetupResponse(t, stdout.Bytes())
	assertStringField(t, response, "provider_id", "deepseek")
	assertStringField(t, response, "model_id", "deepseek-v4-flash")
	configPayload, err := os.ReadFile(filepath.Join(configRoot, "models.json"))
	if err != nil {
		t.Fatalf("read models.json: %v", err)
	}
	for _, want := range []string{
		"https://api.deepseek.com",
		"deepseek-v4-flash",
		"\"provider_adapter_id\": \"deepseek\"",
		"\"provider_adapter_profile_id\": \"deepseek-v4-flash\"",
		"\"hidden_reasoning_policy\": \"discard\"",
		"secret://models/deepseek/local",
		"openai-chat-completions",
		"\"context_window_tokens\": 1000000",
	} {
		if !strings.Contains(string(configPayload), want) {
			t.Fatalf("models.json missing %q: %s", want, string(configPayload))
		}
	}
	if strings.Contains(string(configPayload), secret) {
		t.Fatalf("models.json leaked secret: %s", string(configPayload))
	}
	resolved, err := kernel.ResolveLocalCredentialSecret("secret://models/deepseek/local", credentialRoot)
	if err != nil {
		t.Fatalf("ResolveLocalCredentialSecret returned error: %v", err)
	}
	if resolved != secret {
		t.Fatalf("resolved secret = %q, want original secret", resolved)
	}
}

func TestProviderUseUnknownPresetFailsClosed(t *testing.T) {
	configRoot := testsupport.ProjectTempDir(t, "genesisctl-provider-use-unknown-config")
	credentialRoot := testsupport.ProjectTempDir(t, "genesisctl-provider-use-unknown-credentials")
	var stdout bytes.Buffer

	err := run([]string{
		"provider", "use", "unknown/provider",
		"-config-root", configRoot,
		"-credential-store-root", credentialRoot,
		"-dry-run",
	}, strings.NewReader(""), &stdout)
	if err == nil {
		t.Fatal("provider use unknown preset returned nil error")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on failure", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(configRoot, "models.json")); !os.IsNotExist(err) {
		t.Fatalf("models.json stat err = %v, want not exist", err)
	}
}

func TestProviderVerifyReportsMissingCredentialAsJSON(t *testing.T) {
	configRoot := testsupport.ProjectTempDir(t, "genesisctl-provider-verify-config")
	credentialRoot := testsupport.ProjectTempDir(t, "genesisctl-provider-verify-credentials")
	if err := os.MkdirAll(configRoot, 0o755); err != nil {
		t.Fatalf("mkdir config root: %v", err)
	}
	config := `{
  "model_gateway": {
    "protocol": "openai-chat-completions",
    "base_url": "https://provider.example.com",
    "credential_ref": "secret://models/provider/missing"
  },
  "active_model_profile_bindings": {
    "foreground.coordinator": "verify-profile"
  },
  "model_profiles": {
    "cloud": {
      "gateway": {
        "verify-profile": {
          "profile_id": "verify-profile",
          "model_id": "verify-model"
        }
      }
    }
  }
}`
	if err := os.WriteFile(filepath.Join(configRoot, "models.json"), []byte(config), 0o644); err != nil {
		t.Fatalf("write models.json: %v", err)
	}
	var stdout bytes.Buffer

	err := run([]string{
		"provider", "verify",
		"-config-root", configRoot,
		"-credential-store-root", credentialRoot,
		"-timeout-sec", "1",
	}, strings.NewReader(""), &stdout)
	if err != nil {
		t.Fatalf("provider verify returned error: %v", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("decode provider verify response: %v\n%s", err, stdout.String())
	}
	assertStringField(t, response, "readiness", "not_ready")
	assertStringField(t, response, "readiness_reason", "provider_credential_missing")
	if strings.Contains(stdout.String(), "secret://models/provider/missing") || strings.Contains(stdout.String(), "Authorization") {
		t.Fatalf("provider verify output leaked credential detail: %s", stdout.String())
	}
}

func TestProviderUseSCNetPresetDryRun(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{
		"provider", "use", "scnet/DeepSeek-R1-Distill-Qwen-7B",
		"-config-root", testsupport.ProjectTempDir(t, "genesisctl-provider-use-scnet-config"),
		"-credential-store-root", testsupport.ProjectTempDir(t, "genesisctl-provider-use-scnet-credentials"),
		"-dry-run",
	}, strings.NewReader(""), &stdout)
	if err != nil {
		t.Fatalf("provider use scnet dry-run returned error: %v", err)
	}

	response := decodeProviderSetupResponse(t, stdout.Bytes())
	assertStringField(t, response, "provider_id", "scnet")
	assertStringField(t, response, "model_id", "DeepSeek-R1-Distill-Qwen-7B")
	assertStringField(t, response, "base_url", "https://api.scnet.cn/api/llm/v1")
	assertStringField(t, response, "profile_id", "scnet-deepseek-r1-distill-qwen-7b")
	assertStringField(t, response, "gateway_route", "scnet")
	assertStringField(t, response, "credential_ref", "secret://models/scnet/local")
	assertStringField(t, response, "provider_adapter_id", "scnet")
	assertStringField(t, response, "provider_adapter_profile_id", "DeepSeek-R1-Distill-Qwen-7B")
	if _, ok := response["context_window_tokens"]; ok {
		t.Fatalf("response unexpectedly included unapproved SCNet context metadata: %+v", response)
	}
}

func TestProviderRotateKeyUsesActiveProfileWithoutProviderMetadata(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("real local credential setup uses Windows DPAPI")
	}
	configRoot := testsupport.ProjectTempDir(t, "genesisctl-provider-rotate-config")
	credentialRoot := testsupport.ProjectTempDir(t, "genesisctl-provider-rotate-credentials")
	var setupOut bytes.Buffer
	if err := run([]string{
		"provider", "use", "deepseek/deepseek-v4-flash",
		"-config-root", configRoot,
		"-credential-store-root", credentialRoot,
		"-api-key-stdin",
	}, strings.NewReader("sk-first-secret"), &setupOut); err != nil {
		t.Fatalf("initial provider use returned error: %v", err)
	}

	var rotateOut bytes.Buffer
	err := run([]string{
		"provider", "rotate-key",
		"-config-root", configRoot,
		"-credential-store-root", credentialRoot,
		"-api-key-stdin",
	}, strings.NewReader("sk-rotated-secret"), &rotateOut)
	if err != nil {
		t.Fatalf("provider rotate-key returned error: %v", err)
	}
	if strings.Contains(rotateOut.String(), "sk-rotated-secret") {
		t.Fatalf("rotate-key output leaked secret: %s", rotateOut.String())
	}
	response := decodeProviderSetupResponse(t, rotateOut.Bytes())
	assertStringField(t, response, "profile_id", "deepseek-flash")
	assertStringField(t, response, "gateway_route", "deepseek")
	assertStringField(t, response, "credential_ref", "secret://models/deepseek/local")
	resolved, err := kernel.ResolveLocalCredentialSecret("secret://models/deepseek/local", credentialRoot)
	if err != nil {
		t.Fatalf("ResolveLocalCredentialSecret returned error: %v", err)
	}
	if resolved != "sk-rotated-secret" {
		t.Fatalf("resolved secret = %q, want rotated secret", resolved)
	}
	configPayload, err := os.ReadFile(filepath.Join(configRoot, "models.json"))
	if err != nil {
		t.Fatalf("read models.json: %v", err)
	}
	if !strings.Contains(string(configPayload), "deepseek-v4-flash") {
		t.Fatalf("rotate-key changed provider metadata: %s", string(configPayload))
	}
}

func TestProviderUsePreservesExistingSharedModelConfig(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("real local credential setup uses Windows DPAPI")
	}
	configRoot := testsupport.ProjectTempDir(t, "genesisctl-provider-use-preserve-config")
	credentialRoot := testsupport.ProjectTempDir(t, "genesisctl-provider-use-preserve-credentials")
	initial := `{
  "model_gateway": {
    "protocol": "provider_command",
    "command": "existing-provider-command",
    "routes": {
      "existing-route": {
        "protocol": "provider_command",
        "command": "existing-route-command"
      }
    }
  },
  "active_model_profile_bindings": {
    "background.worker": "existing-profile"
  },
  "model_profiles": {
    "local": {
      "gateway": {
        "existing-profile": {
          "profile_id": "existing-profile",
          "model_id": "existing-model",
          "gateway_route": "existing-route"
        }
      }
    }
  }
}`
	if err := os.MkdirAll(configRoot, 0o755); err != nil {
		t.Fatalf("mkdir config root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configRoot, "models.json"), []byte(initial), 0o644); err != nil {
		t.Fatalf("write initial models.json: %v", err)
	}

	var stdout bytes.Buffer
	if err := run([]string{
		"provider", "use", "deepseek/deepseek-v4-flash",
		"-config-root", configRoot,
		"-credential-store-root", credentialRoot,
		"-api-key-stdin",
	}, strings.NewReader("sk-preserve-secret"), &stdout); err != nil {
		t.Fatalf("provider use returned error: %v", err)
	}
	configPayload, err := os.ReadFile(filepath.Join(configRoot, "models.json"))
	if err != nil {
		t.Fatalf("read models.json: %v", err)
	}
	for _, want := range []string{
		"existing-provider-command",
		"existing-route-command",
		"background.worker",
		"existing-profile",
		"deepseek-v4-flash",
	} {
		if !strings.Contains(string(configPayload), want) {
			t.Fatalf("models.json missing preserved value %q: %s", want, string(configPayload))
		}
	}
}

func TestProviderSetupCommandDryRunDoesNotRequireAPIKey(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{
		"provider-setup",
		"-config-root", testsupport.ProjectTempDir(t, "genesisctl-dry-run-config"),
		"-credential-store-root", testsupport.ProjectTempDir(t, "genesisctl-dry-run-credentials"),
		"-base-url", "https://provider.example.com/api",
		"-model", "provider-model",
		"-dry-run",
	}, strings.NewReader(""), &stdout)
	if err != nil {
		t.Fatalf("dry-run command returned error: %v", err)
	}
	var response providerSetupResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.OK || !response.DryRun || response.Verified {
		t.Fatalf("response = %+v, want dry-run ok without verify", response)
	}
}

func TestProviderSetupCommandWritesCredentialWithoutPrintingSecret(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("real local credential setup uses Windows DPAPI")
	}
	configRoot := testsupport.ProjectTempDir(t, "genesisctl-config")
	credentialRoot := testsupport.ProjectTempDir(t, "genesisctl-credentials")
	secret := "sk-command-secret"
	t.Setenv("GENESIS_PROVIDER_API_KEY", secret)

	var stdout bytes.Buffer
	err := run([]string{
		"provider-setup",
		"-config-root", configRoot,
		"-credential-store-root", credentialRoot,
		"-profile-id", "command-profile",
		"-gateway-route", "command-route",
		"-base-url", "https://provider.example.com/api",
		"-model", "provider-command-model",
		"-credential-ref", "secret://models/provider/command",
	}, strings.NewReader(""), &stdout)
	if err != nil {
		t.Fatalf("provider-setup command returned error: %v", err)
	}
	if strings.Contains(stdout.String(), secret) {
		t.Fatalf("command output leaked secret: %s", stdout.String())
	}
	var response providerSetupResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.OK || !response.Verified {
		t.Fatalf("response = %+v, want verified ok", response)
	}
	configPayload, err := os.ReadFile(filepath.Join(configRoot, "models.json"))
	if err != nil {
		t.Fatalf("read models.json: %v", err)
	}
	if strings.Contains(string(configPayload), secret) {
		t.Fatalf("models.json leaked secret: %s", string(configPayload))
	}
	credentialPayload, err := os.ReadFile(response.CredentialPath)
	if err != nil {
		t.Fatalf("read credential record: %v", err)
	}
	if strings.Contains(string(credentialPayload), secret) {
		t.Fatalf("credential record leaked secret: %s", string(credentialPayload))
	}
	resolved, err := kernel.ResolveLocalCredentialSecret(response.CredentialRef, credentialRoot)
	if err != nil {
		t.Fatalf("ResolveLocalCredentialSecret returned error: %v", err)
	}
	if resolved != secret {
		t.Fatalf("resolved secret = %q, want original secret", resolved)
	}
}

func decodeProviderSetupResponse(t *testing.T, payload []byte) map[string]any {
	t.Helper()
	var response map[string]any
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatalf("decode response: %v\npayload: %s", err, string(payload))
	}
	return response
}

func assertStringField(t *testing.T, response map[string]any, field string, want string) {
	t.Helper()
	got, ok := response[field].(string)
	if !ok || got != want {
		t.Fatalf("response[%s] = %#v, want %q in %+v", field, response[field], want, response)
	}
}

func assertFloatField(t *testing.T, response map[string]any, field string, want float64) {
	t.Helper()
	got, ok := response[field].(float64)
	if !ok || got != want {
		t.Fatalf("response[%s] = %#v, want %.0f in %+v", field, response[field], want, response)
	}
}
