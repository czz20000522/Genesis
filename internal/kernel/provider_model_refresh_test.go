package kernel

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRefreshProviderModelCatalogPersistsSortedModelsWithoutChangingBinding(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Fatalf("path = %q, want /models", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"model-b"},{"id":"model-a"},{"id":"model-b"},{"id":"  "}]}`))
	}))
	defer server.Close()
	root := writeModelsConfig(t, minimalModelsConfig(map[string]any{"base_url": server.URL}))
	now := time.Date(2026, 7, 8, 12, 30, 0, 0, time.UTC)

	result := RefreshProviderModelCatalog(ProviderModelRefreshRequest{
		ConfigRoot: root,
		SecretResolver: func(ref string, storeRoot string) (string, error) {
			if ref != "secret://models/provider/default" {
				t.Fatalf("credential ref = %q", ref)
			}
			return "sk-refresh-secret", nil
		},
		Clock: func() time.Time { return now },
	})

	if result.Readiness != ReadinessReady || result.ModelCount != 2 {
		t.Fatalf("result = %+v, want ready with two models", result)
	}
	if gotAuth != "Bearer sk-refresh-secret" {
		t.Fatalf("authorization = %q, want bearer key", gotAuth)
	}
	if !reflect.DeepEqual(result.Models, []string{"model-a", "model-b"}) {
		t.Fatalf("models = %+v, want sorted unique models", result.Models)
	}
	config := readRefreshModelsConfig(t, root)
	if config.ActiveModelProfileBindings[DefaultModelRole] != "provider-fast" {
		t.Fatalf("active binding changed: %+v", config.ActiveModelProfileBindings)
	}
	catalog := config.ProviderModelCatalogs["provider-fast"]
	if !reflect.DeepEqual(catalog.Models, []string{"model-a", "model-b"}) {
		t.Fatalf("catalog = %+v, want sorted persisted models", catalog)
	}
	if catalog.RefreshedAt != now.Format(time.RFC3339) || catalog.Protocol != modelGatewayProtocolChatCompletions {
		t.Fatalf("catalog metadata = %+v", catalog)
	}
	raw, err := os.ReadFile(filepath.Join(root, "models.json"))
	if err != nil {
		t.Fatalf("read models.json: %v", err)
	}
	for _, forbidden := range []string{"sk-refresh-secret", "Authorization"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("models.json leaked %q: %s", forbidden, string(raw))
		}
	}
	catalogJSON, err := json.Marshal(catalog)
	if err != nil {
		t.Fatalf("marshal catalog: %v", err)
	}
	if strings.Contains(string(catalogJSON), "secret://models/provider/default") {
		t.Fatalf("catalog leaked credential ref: %s", string(catalogJSON))
	}
}

func TestRefreshProviderModelCatalogFallsBackToV1Models(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path == "/models" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %q, want fallback /v1/models", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"fallback-model"}]}`))
	}))
	defer server.Close()
	root := writeModelsConfig(t, minimalModelsConfig(map[string]any{"base_url": server.URL}))

	result := RefreshProviderModelCatalog(ProviderModelRefreshRequest{
		ConfigRoot: root,
		SecretResolver: func(ref string, storeRoot string) (string, error) {
			return "sk-refresh-secret", nil
		},
	})

	if result.Readiness != ReadinessReady || !reflect.DeepEqual(result.Models, []string{"fallback-model"}) {
		t.Fatalf("result = %+v, want fallback model", result)
	}
	if !reflect.DeepEqual(paths, []string{"/models", "/v1/models"}) {
		t.Fatalf("paths = %+v, want /models then /v1/models", paths)
	}
}

func TestRefreshProviderModelCatalogFailurePreservesExistingCatalog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "sk-refresh-secret should not leak", http.StatusUnauthorized)
	}))
	defer server.Close()
	payload := minimalModelsConfig(map[string]any{"base_url": server.URL})
	payload["provider_model_catalogs"] = map[string]any{
		"provider-fast": map[string]any{
			"route":        "provider-fast",
			"protocol":     modelGatewayProtocolChatCompletions,
			"models":       []string{"old-model"},
			"refreshed_at": "2026-07-01T00:00:00Z",
			"source":       "models_endpoint",
		},
	}
	root := writeModelsConfig(t, payload)

	result := RefreshProviderModelCatalog(ProviderModelRefreshRequest{
		ConfigRoot: root,
		SecretResolver: func(ref string, storeRoot string) (string, error) {
			return "sk-refresh-secret", nil
		},
	})

	if result.Readiness != ReadinessNotReady || result.ReadinessReason != "provider_models_auth_failed" {
		t.Fatalf("result = %+v, want sanitized auth failure", result)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if strings.Contains(string(encoded), "sk-refresh-secret") {
		t.Fatalf("result leaked secret: %s", string(encoded))
	}
	config := readRefreshModelsConfig(t, root)
	catalog := config.ProviderModelCatalogs["provider-fast"]
	if !reflect.DeepEqual(catalog.Models, []string{"old-model"}) {
		t.Fatalf("catalog = %+v, want previous catalog preserved", catalog)
	}
}

func TestRefreshProviderModelCatalogClassifiesFailuresWithoutUpdatingCatalog(t *testing.T) {
	for _, tc := range []struct {
		name           string
		handler        http.HandlerFunc
		secretResolver func(string, string) (string, error)
		wantReason     string
	}{
		{
			name: "missing credential",
			secretResolver: func(string, string) (string, error) {
				return "", errors.New("missing secret detail")
			},
			wantReason: "provider_credential_missing",
		},
		{
			name: "endpoint missing",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.NotFound(w, r)
			},
			wantReason: "provider_models_endpoint_missing",
		},
		{
			name: "empty response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":[]}`))
			},
			wantReason: "provider_models_empty",
		},
		{
			name: "decode failure",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":`))
			},
			wantReason: "provider_models_decode_failed",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			baseURL := "https://provider.example.com/api"
			var server *httptest.Server
			if tc.handler != nil {
				server = httptest.NewServer(tc.handler)
				defer server.Close()
				baseURL = server.URL
			}
			root := writeModelsConfig(t, refreshConfigWithOldCatalog(baseURL))
			resolver := tc.secretResolver
			if resolver == nil {
				resolver = func(string, string) (string, error) {
					return "sk-refresh-secret", nil
				}
			}

			result := RefreshProviderModelCatalog(ProviderModelRefreshRequest{
				ConfigRoot:     root,
				SecretResolver: resolver,
			})

			if result.Readiness != ReadinessNotReady || result.ReadinessReason != tc.wantReason {
				t.Fatalf("result = %+v, want reason %q", result, tc.wantReason)
			}
			config := readRefreshModelsConfig(t, root)
			catalog := config.ProviderModelCatalogs["provider-fast"]
			if !reflect.DeepEqual(catalog.Models, []string{"old-model"}) {
				t.Fatalf("catalog = %+v, want old catalog preserved", catalog)
			}
		})
	}
}

func TestRefreshProviderModelCatalogRefusesProviderCommand(t *testing.T) {
	root := writeModelsConfig(t, minimalModelsConfig(map[string]any{
		"protocol": modelGatewayProtocolProviderCommand,
		"command":  "provider-command-that-must-not-run",
	}))

	result := RefreshProviderModelCatalog(ProviderModelRefreshRequest{ConfigRoot: root})

	if result.Readiness != ReadinessNotReady || result.ReadinessReason != "provider_model_refresh_unsupported" {
		t.Fatalf("result = %+v, want unsupported provider_command refresh", result)
	}
}

func refreshConfigWithOldCatalog(baseURL string) map[string]any {
	payload := minimalModelsConfig(map[string]any{"base_url": baseURL})
	payload["provider_model_catalogs"] = map[string]any{
		"provider-fast": map[string]any{
			"route":        "provider-fast",
			"protocol":     modelGatewayProtocolChatCompletions,
			"models":       []string{"old-model"},
			"refreshed_at": "2026-07-01T00:00:00Z",
			"source":       "models_endpoint",
		},
	}
	return payload
}

func readRefreshModelsConfig(t *testing.T, root string) genesisModelsConfig {
	t.Helper()
	payload, err := os.ReadFile(filepath.Join(root, "models.json"))
	if err != nil {
		t.Fatalf("read models.json: %v", err)
	}
	var config genesisModelsConfig
	if err := json.Unmarshal(payload, &config); err != nil {
		t.Fatalf("decode models.json: %v\n%s", err, string(payload))
	}
	return config
}
