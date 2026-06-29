package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSidecarPlaceholderIsNotReady(t *testing.T) {
	t.Setenv("GENESIS_KERNEL_BASE_URL", "")
	t.Setenv("GENESIS_RUNTIME_TOKEN", "")

	cfg := loadDesktopConfig()

	if cfg.Sidecar.Readiness != "not_ready" || cfg.Sidecar.Reason != sidecarNotWired {
		t.Fatalf("sidecar = %+v, want structured not_ready placeholder", cfg.Sidecar)
	}
}

func TestKernelHTTPClientUsesKernelHTTPPrimitive(t *testing.T) {
	var gotPath string
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"readiness":"ready"}`))
	}))
	defer server.Close()

	client := NewKernelHTTPClient(server.URL, "test-token", server.Client())
	payload, err := client.RequestJSON(context.Background(), http.MethodGet, "/capabilities", true)
	if err != nil {
		t.Fatalf("RequestJSON returned error: %v", err)
	}

	if gotPath != "/capabilities" {
		t.Fatalf("path = %q, want /capabilities", gotPath)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("authorization = %q, want bearer runtime token", gotAuth)
	}
	if payload["readiness"] != "ready" {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestDesktopGoDoesNotImportKernelInternals(t *testing.T) {
	entries, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob Go files: %v", err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry, "_test.go") {
			continue
		}
		body, err := os.ReadFile(entry)
		if err != nil {
			t.Fatalf("read %s: %v", entry, err)
		}
		text := string(body)
		for _, forbidden := range []string{
			"genesis/internal/kernel",
			"NewJSONLLedger",
			"MemoryCandidate",
			"ToolResult",
			"ProviderContext",
		} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s contains forbidden kernel truth surface %q", entry, forbidden)
			}
		}
	}
}
