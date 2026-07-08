package kernel

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVerifyProviderLiveAuthenticatesAgainstConfiguredUpstream(t *testing.T) {
	const apiKey = "sk-live-provider-secret"
	var sawAuth bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %q, want /chat/completions", r.URL.Path)
		}
		if r.Header.Get("Authorization") == "Bearer "+apiKey {
			sawAuth = true
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"verified-model","choices":[{"message":{"role":"assistant","content":"GENESIS_PROVIDER_VERIFY_OK"}}],"usage":{"prompt_tokens":4,"completion_tokens":2,"total_tokens":6}}`))
	}))
	defer server.Close()
	root := writeLiveVerifyModelsConfig(t, server.URL, "verified-model", "openai-chat-completions")

	result := VerifyProviderLive(ProviderLiveVerifyRequest{
		ConfigRoot: root,
		SecretResolver: func(ref string, _ string) (string, error) {
			if ref != "secret://models/provider/live" {
				t.Fatalf("credential ref = %q", ref)
			}
			return apiKey, nil
		},
		Timeout: time.Second,
	})

	if result.Readiness != ReadinessReady {
		t.Fatalf("result = %+v, want ready", result)
	}
	if !sawAuth {
		t.Fatal("provider verify did not authenticate upstream request")
	}
	if result.Model != "verified-model" || result.Provider.Name != "openai-compatible" {
		t.Fatalf("result = %+v, want verified model/provider metadata", result)
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if strings.Contains(string(resultJSON), apiKey) || strings.Contains(string(resultJSON), "Authorization") {
		t.Fatalf("provider verify result leaked secret material: %s", string(resultJSON))
	}
}

func TestVerifyProviderLiveMapsAuthFailureWithoutLeakingProviderBody(t *testing.T) {
	const apiKey = "sk-live-provider-secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "invalid key "+apiKey, http.StatusUnauthorized)
	}))
	defer server.Close()
	root := writeLiveVerifyModelsConfig(t, server.URL, "verified-model", "openai-chat-completions")

	result := VerifyProviderLive(ProviderLiveVerifyRequest{
		ConfigRoot: root,
		SecretResolver: func(string, string) (string, error) {
			return apiKey, nil
		},
		Timeout: time.Second,
	})

	if result.Readiness != ReadinessNotReady || result.ReadinessReason != "provider_auth_failed" {
		t.Fatalf("result = %+v, want provider_auth_failed", result)
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	for _, forbidden := range []string{apiKey, "invalid key", "Authorization"} {
		if strings.Contains(string(resultJSON), forbidden) {
			t.Fatalf("provider verify result leaked %q: %s", forbidden, string(resultJSON))
		}
	}
}

func TestVerifyProviderLiveReportsMissingCredentialWithoutNetworkProbe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("provider verify contacted upstream despite missing credential")
	}))
	defer server.Close()
	root := writeLiveVerifyModelsConfig(t, server.URL, "verified-model", "openai-chat-completions")

	result := VerifyProviderLive(ProviderLiveVerifyRequest{
		ConfigRoot: root,
		SecretResolver: func(string, string) (string, error) {
			return "", errors.New("missing sk-live-provider-secret")
		},
		Timeout: time.Second,
	})

	if result.Readiness != ReadinessNotReady || result.ReadinessReason != "provider_credential_missing" {
		t.Fatalf("result = %+v, want credential missing", result)
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if strings.Contains(string(resultJSON), "sk-live-provider-secret") {
		t.Fatalf("provider verify result leaked resolver error: %s", string(resultJSON))
	}
}

func TestVerifyProviderLiveReportsInvalidConfigDistinctFromMissing(t *testing.T) {
	root := testTempDir(t)
	if err := os.WriteFile(filepath.Join(root, "models.json"), []byte(`{"model_gateway": "sk-live-provider-secret"`), 0o644); err != nil {
		t.Fatalf("write invalid models.json: %v", err)
	}

	result := VerifyProviderLive(ProviderLiveVerifyRequest{ConfigRoot: root, Timeout: time.Second})

	if result.Readiness != ReadinessNotReady || result.ReadinessReason != "provider_config_invalid" {
		t.Fatalf("result = %+v, want provider_config_invalid", result)
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	for _, forbidden := range []string{"sk-live-provider-secret", "model_gateway"} {
		if strings.Contains(string(resultJSON), forbidden) {
			t.Fatalf("provider verify result leaked %q: %s", forbidden, string(resultJSON))
		}
	}
}

func TestVerifyProviderLiveRunsProviderCommandConfig(t *testing.T) {
	root := writeModelsConfig(t, map[string]any{
		"model_gateway": map[string]any{
			"protocol": "provider_command",
			"command":  os.Args[0],
			"args":     []string{"-test.run=TestProviderCommandAdapterHelper", "--", "verify-final"},
			"env":      []string{"GENESIS_PROVIDER_COMMAND_HELPER=1"},
		},
		"active_model_profile_bindings": map[string]any{
			DefaultModelRole: "live-profile",
		},
		"model_profiles": map[string]any{
			"local": map[string]any{
				"gateway": map[string]any{
					"live-profile": map[string]any{
						"profile_id": "live-profile",
						"model_id":   "command-model",
					},
				},
			},
		},
	})

	result := VerifyProviderLive(ProviderLiveVerifyRequest{ConfigRoot: root, Timeout: time.Second})

	if result.Readiness != ReadinessReady || result.Provider.Name != "provider_command" || result.Model != "command-model" {
		t.Fatalf("result = %+v, want ready provider_command", result)
	}
}

func writeLiveVerifyModelsConfig(t *testing.T, endpoint string, model string, protocol string) string {
	t.Helper()
	gateway := map[string]any{
		"protocol":       protocol,
		"credential_ref": "secret://models/provider/live",
	}
	if protocol == "provider_command" {
		gateway["command"] = endpoint
	} else {
		gateway["base_url"] = endpoint
	}
	return writeModelsConfig(t, map[string]any{
		"model_gateway": gateway,
		"active_model_profile_bindings": map[string]any{
			DefaultModelRole: "live-profile",
		},
		"model_profiles": map[string]any{
			"cloud": map[string]any{
				"gateway": map[string]any{
					"live-profile": map[string]any{
						"profile_id": "live-profile",
						"model_id":   model,
					},
				},
			},
		},
	})
}
