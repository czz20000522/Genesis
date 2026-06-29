package main

import (
	"context"
	"io"
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
	payload, err := client.RequestJSON(context.Background(), http.MethodGet, "/capabilities", true, nil)
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

func TestTypedSubmitTurnBridgePostsKernelTurn(t *testing.T) {
	var gotPath string
	var gotMethod string
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	app := NewApp()
	app.client = NewKernelHTTPClient(server.URL, "token", server.Client())
	payload, err := app.SubmitTurn("s1", "hello", "idem-1")
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}

	if gotPath != "/turn" || gotMethod != http.MethodPost {
		t.Fatalf("request = %s %s, want POST /turn", gotMethod, gotPath)
	}
	if !strings.Contains(gotBody, `"session_id":"s1"`) || !strings.Contains(gotBody, `"text":"hello"`) {
		t.Fatalf("body = %q", gotBody)
	}
	if payload["ok"] != true {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestUploadMaterialBridgePostsMultipartThroughGoChokePoint(t *testing.T) {
	source := filepath.Join(t.TempDir(), "package.zip")
	if err := os.WriteFile(source, []byte("zip"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	var gotSession string
	var gotPurpose string
	var gotFilename string
	var gotContent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		gotSession = r.FormValue("session_id")
		gotPurpose = r.FormValue("purpose")
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("FormFile: %v", err)
		}
		defer file.Close()
		gotFilename = header.Filename
		body, _ := io.ReadAll(file)
		gotContent = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"admission_result":"admitted"}`))
	}))
	defer server.Close()

	app := NewApp()
	app.client = NewKernelHTTPClient(server.URL, "token", server.Client())
	payload, err := app.UploadMaterial(MaterialBridgeRequest{
		SessionID: "session-1",
		Purpose:   "source_analysis",
		FilePath:  source,
	})
	if err != nil {
		t.Fatalf("UploadMaterial returned error: %v", err)
	}

	if gotSession != "session-1" || gotPurpose != "source_analysis" || gotFilename != "package.zip" || gotContent != "zip" {
		t.Fatalf("multipart = session %q purpose %q file %q content %q", gotSession, gotPurpose, gotFilename, gotContent)
	}
	if payload["admission_result"] != "admitted" {
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

func TestFrontendAssetDirPrefersPackagedExecutableLayout(t *testing.T) {
	root := t.TempDir()
	dist := filepath.Join(root, "frontend", "dist")
	if err := os.MkdirAll(dist, 0o755); err != nil {
		t.Fatalf("mkdir dist: %v", err)
	}
	exe := filepath.Join(root, "build", "bin", "genesis-desktop.exe")
	cwd := filepath.Join(root, "elsewhere")

	got := frontendAssetDir(exe, cwd)

	if got != dist {
		t.Fatalf("frontendAssetDir() = %q, want %q", got, dist)
	}
}

func TestSingleInstanceLockIsConfigured(t *testing.T) {
	lock := singleInstanceLock(NewApp())

	if lock == nil || lock.UniqueId == "" || lock.OnSecondInstanceLaunch == nil {
		t.Fatalf("singleInstanceLock() = %+v, want unique id and second-launch handler", lock)
	}
}
