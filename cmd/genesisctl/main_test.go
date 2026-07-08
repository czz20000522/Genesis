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

func TestCapabilityListDoctorAndRunUseUserCapabilityRoot(t *testing.T) {
	root := testsupport.ProjectTempDir(t, "genesisctl-capability-root")
	pkg := filepath.Join(root, "video-transcript")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatalf("mkdir capability package: %v", err)
	}
	runnerName := "runner"
	if runtime.GOOS == "windows" {
		runnerName += ".exe"
	}
	source, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatalf("read test executable: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkg, runnerName), source, 0o755); err != nil {
		t.Fatalf("write helper runner: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "SKILL.md"), []byte("# Video transcript\n"), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	manifest := `{
  "id": "video-transcript",
  "name": "视频字幕提取",
  "description": "从链接生成字幕文件。",
  "entrypoint": "` + runnerName + `",
  "skill": "SKILL.md",
  "inputs": ["url"],
  "outputs": ["txt"]
}`
	if err := os.WriteFile(filepath.Join(pkg, "genesis.capability.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	var listOut bytes.Buffer
	if err := run([]string{"capability", "list", "-root", root}, strings.NewReader(""), &listOut); err != nil {
		t.Fatalf("capability list returned error: %v", err)
	}
	if !strings.Contains(listOut.String(), `"id": "video-transcript"`) || !strings.Contains(listOut.String(), `"readiness": "ready"`) {
		t.Fatalf("list output = %s", listOut.String())
	}

	var doctorOut bytes.Buffer
	if err := run([]string{"capability", "doctor", "video-transcript", "-root", root}, strings.NewReader(""), &doctorOut); err != nil {
		t.Fatalf("capability doctor returned error: %v\n%s", err, doctorOut.String())
	}
	if !strings.Contains(doctorOut.String(), `"readiness": "ready"`) {
		t.Fatalf("doctor output = %s", doctorOut.String())
	}

	var runOut bytes.Buffer
	if err := run([]string{
		"capability", "run", "video-transcript",
		"-root", root,
		"--",
		"-test.run=TestCapabilityRunnerHelper",
		"--",
		"capability-helper",
		"https://example.com/video",
	}, strings.NewReader(""), &runOut); err != nil {
		t.Fatalf("capability run returned error: %v\n%s", err, runOut.String())
	}
	if strings.TrimSpace(runOut.String()) != "capability:https://example.com/video" {
		t.Fatalf("run output = %q", runOut.String())
	}
}

func TestCapabilityDoctorFailsClosedForUnsafeManifest(t *testing.T) {
	root := testsupport.ProjectTempDir(t, "genesisctl-capability-unsafe")
	pkg := filepath.Join(root, "bad")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatalf("mkdir capability package: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "genesis.capability.json"), []byte(`{"id":"bad","entrypoint":"../escape.ps1"}`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	var stdout bytes.Buffer
	err := run([]string{"capability", "doctor", "bad", "-root", root}, strings.NewReader(""), &stdout)
	if err == nil {
		t.Fatal("capability doctor should reject unsafe entrypoint")
	}
	if !strings.Contains(stdout.String(), `"reason": "entrypoint_unsafe"`) {
		t.Fatalf("doctor output = %s", stdout.String())
	}
}

func TestCapabilityRunnerHelper(t *testing.T) {
	args := os.Args
	for i, arg := range args {
		if arg == "capability-helper" {
			if i+1 >= len(args) {
				t.Fatal("capability-helper missing payload")
			}
			t.Log("capability helper executed")
			os.Stdout.WriteString("capability:" + args[i+1] + "\n")
			os.Exit(0)
		}
	}
}

func TestProviderCommandVerifyHelper(t *testing.T) {
	for _, arg := range os.Args {
		if arg == "provider-command-verify-helper" {
			os.Stdout.WriteString(`{"kind":"final","text":"GENESIS_PROVIDER_VERIFY_OK","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}` + "\n")
			os.Exit(0)
		}
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
    "coordinator": "verify-profile"
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

func TestProviderVerifyRunsProviderCommandConfig(t *testing.T) {
	configRoot := testsupport.ProjectTempDir(t, "genesisctl-provider-verify-command-config")
	if err := os.MkdirAll(configRoot, 0o755); err != nil {
		t.Fatalf("mkdir config root: %v", err)
	}
	configPayload, err := json.Marshal(map[string]any{
		"model_gateway": map[string]any{
			"protocol": "provider_command",
			"command":  os.Args[0],
			"args":     []string{"-test.run=TestProviderCommandVerifyHelper", "--", "provider-command-verify-helper"},
		},
		"active_model_profile_bindings": map[string]any{
			"coordinator": "verify-command-profile",
		},
		"model_profiles": map[string]any{
			"local": map[string]any{
				"gateway": map[string]any{
					"verify-command-profile": map[string]any{
						"profile_id": "verify-command-profile",
						"model_id":   "command-model",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal models config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configRoot, "models.json"), configPayload, 0o644); err != nil {
		t.Fatalf("write models.json: %v", err)
	}
	var stdout bytes.Buffer

	err = run([]string{
		"provider", "verify",
		"-config-root", configRoot,
		"-timeout-sec", "1",
	}, strings.NewReader(""), &stdout)
	if err != nil {
		t.Fatalf("provider verify returned error: %v", err)
	}

	response := decodeProviderSetupResponse(t, stdout.Bytes())
	assertStringField(t, response, "readiness", "ready")
	assertStringField(t, response, "model", "command-model")
	provider := response["provider"].(map[string]interface{})
	assertStringField(t, provider, "name", "provider_command")
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

func TestProviderRotateKeyCanRepairKnownPresetMetadataInDryRun(t *testing.T) {
	configRoot := testsupport.ProjectTempDir(t, "genesisctl-provider-rotate-repair-config")
	credentialRoot := testsupport.ProjectTempDir(t, "genesisctl-provider-rotate-repair-credentials")
	writePreAdapterProviderConfigForCLI(t, configRoot, "deepseek-flash", "deepseek-v4-flash", "deepseek", "secret://models/deepseek/local")
	var stdout bytes.Buffer

	err := run([]string{
		"provider", "rotate-key",
		"-config-root", configRoot,
		"-credential-store-root", credentialRoot,
		"-repair-profile-metadata", "deepseek/deepseek-v4-flash",
		"-dry-run",
	}, strings.NewReader(""), &stdout)
	if err != nil {
		t.Fatalf("provider rotate-key repair dry-run returned error: %v", err)
	}
	response := decodeProviderSetupResponse(t, stdout.Bytes())
	assertStringField(t, response, "profile_id", "deepseek-flash")
	assertStringField(t, response, "provider_adapter_id", "deepseek")
	assertStringField(t, response, "provider_adapter_profile_id", "deepseek-v4-flash")
	assertFloatField(t, response, "context_window_tokens", 1000000)

	configPayload, err := os.ReadFile(filepath.Join(configRoot, "models.json"))
	if err != nil {
		t.Fatalf("read models.json: %v", err)
	}
	if strings.Contains(string(configPayload), "provider_adapter_id") {
		t.Fatalf("dry-run mutated provider metadata: %s", string(configPayload))
	}
}

func TestProviderRotateKeyRejectsUnknownRepairPresetBeforeReadingSecret(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{
		"provider", "rotate-key",
		"-config-root", testsupport.ProjectTempDir(t, "genesisctl-provider-rotate-unknown-config"),
		"-credential-store-root", testsupport.ProjectTempDir(t, "genesisctl-provider-rotate-unknown-credentials"),
		"-repair-profile-metadata", "unknown/provider",
	}, strings.NewReader(""), &stdout)
	if err == nil || !strings.Contains(err.Error(), "unknown provider preset") {
		t.Fatalf("error = %v, want unknown repair preset refusal", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on failure", stdout.String())
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

func writePreAdapterProviderConfigForCLI(t *testing.T, configRoot string, profileID string, modelID string, routeName string, credentialRef string) {
	t.Helper()
	if err := os.MkdirAll(configRoot, 0o755); err != nil {
		t.Fatalf("mkdir config root: %v", err)
	}
	config := `{
  "model_gateway": {
    "protocol": "openai-chat-completions",
    "routes": {
      "` + routeName + `": {
        "protocol": "openai-chat-completions",
        "base_url": "https://api.deepseek.com",
        "credential_ref": "` + credentialRef + `",
        "request_timeout_sec": 60
      }
    }
  },
  "active_model_profile_bindings": {
    "coordinator": "` + profileID + `"
  },
  "model_profiles": {
    "cloud": {
      "gateway": {
        "` + profileID + `": {
          "profile_id": "` + profileID + `",
          "model_id": "` + modelID + `",
          "gateway_route": "` + routeName + `"
        }
      }
    }
  }
}`
	if err := os.WriteFile(filepath.Join(configRoot, "models.json"), []byte(config), 0o644); err != nil {
		t.Fatalf("write pre-adapter models.json: %v", err)
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
