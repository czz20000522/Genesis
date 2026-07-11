package kernel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"genesis/internal/testsupport"
)

func TestHTTPProviderVerifyUsesConfiguredReadOnlyVerifier(t *testing.T) {
	k := newTestKernelWithRuntimeToken(t, filepath.Join(testsupport.ProjectTempDir(t, "provider-verify"), "events.jsonl"), "test-token")
	defer k.Close()
	called := ProviderVerificationRequest{}
	k.providerVerifier = func(req ProviderVerificationRequest) ProviderLiveVerifyResult {
		called = req
		return ProviderLiveVerifyResult{
			Readiness: ReadinessReady,
			Provider:  ProviderStatus{Name: "openai-compatible", Readiness: ReadinessReady},
			ModelRole: req.ModelRole,
			ProfileID: req.ProfileID,
			Model:     "glm-5-2",
		}
	}
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	request, err := http.NewRequest(http.MethodPost, server.URL+"/providers/verify", strings.NewReader(`{"model_role":"coder","profile_id":"cloud-glm"}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer test-token")
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("provider verify request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
	if called.ModelRole != "coder" || called.ProfileID != "cloud-glm" {
		t.Fatalf("verification request = %+v", called)
	}
	var payload ProviderLiveVerifyResult
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Readiness != ReadinessReady || payload.Model != "glm-5-2" {
		t.Fatalf("response = %+v", payload)
	}
}
