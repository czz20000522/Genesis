package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func desktopTestTempDir(t *testing.T) string {
	t.Helper()
	name := strings.NewReplacer("\\", "_", "/", "_", ":", "_", " ", "_").Replace(t.Name())
	dir := filepath.Join("..", ".test-tmp", "desktop", name)
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove test temp dir: %v", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir test temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})
	return dir
}

type fakeSidecarProcess struct {
	pid       int
	stopCalls int
}

func (p *fakeSidecarProcess) PID() int {
	return p.pid
}

func (p *fakeSidecarProcess) Stop(context.Context) error {
	p.stopCalls++
	return nil
}

func TestLocalServiceSupervisorStartsOwnedKernelProcess(t *testing.T) {
	t.Setenv("GENESIS_KERNEL_BASE_URL", "")
	t.Setenv("GENESIS_RUNTIME_TOKEN", "")

	proc := &fakeSidecarProcess{pid: 4321}
	launched := false
	supervisor := NewLocalServiceSupervisor(LocalServiceSupervisorConfig{
		KernelBaseURL: defaultKernelBaseURL,
		LogDir:        desktopTestTempDir(t),
		launcher: func(_ context.Context, req sidecarLaunchRequest) (sidecarProcess, error) {
			launched = true
			if req.LogPath == "" {
				t.Fatal("launcher did not receive a log path")
			}
			return proc, nil
		},
		readinessProbe: func(context.Context, string, string) sidecarReadinessResult {
			return sidecarReadinessResult{Ready: true}
		},
	})

	status := supervisor.StartKernel(context.Background())

	if !launched {
		t.Fatal("owned supervisor did not launch genesisd")
	}
	if status.ServiceID != "kernel" || status.Kind != "kernel" || status.Ownership != "owned" {
		t.Fatalf("sidecar identity = %+v, want owned kernel service", status)
	}
	if status.Readiness != "ready" || status.Reason != "" {
		t.Fatalf("sidecar readiness = %+v, want ready owned service", status)
	}
	if status.PID != proc.pid || status.StartedAt == "" || status.LogPath == "" {
		t.Fatalf("sidecar process metadata = %+v, want pid, started_at, log_path", status)
	}
}

func TestLocalServiceSupervisorProjectsExternalKernelWithoutOwnership(t *testing.T) {
	t.Setenv("GENESIS_KERNEL_BASE_URL", "http://127.0.0.1:9999")
	t.Setenv("GENESIS_RUNTIME_TOKEN", "token")

	cfg := loadDesktopConfig()

	if cfg.KernelBaseURL != "http://127.0.0.1:9999" {
		t.Fatalf("kernel base url = %q", cfg.KernelBaseURL)
	}
	if cfg.Sidecar.Ownership != "external" || cfg.Sidecar.Reason != sidecarExternalKernelConfigured {
		t.Fatalf("sidecar = %+v, want external kernel projection", cfg.Sidecar)
	}
}

func TestDesktopStartupAndShutdownRouteThroughLocalServiceSupervisor(t *testing.T) {
	app := NewApp()
	supervisor := NewLocalServiceSupervisor(LocalServiceSupervisorConfig{
		KernelBaseURL: defaultKernelBaseURL,
		LogDir:        desktopTestTempDir(t),
		launcher: func(context.Context, sidecarLaunchRequest) (sidecarProcess, error) {
			return &fakeSidecarProcess{pid: 1234}, nil
		},
		readinessProbe: func(context.Context, string, string) sidecarReadinessResult {
			return sidecarReadinessResult{Ready: true}
		},
	})
	app.supervisor = supervisor

	app.startup(context.Background())
	if !supervisor.startAttempted {
		t.Fatal("startup did not ask local service supervisor to start owned services")
	}

	app.shutdown(context.Background())
	if !supervisor.stopAttempted {
		t.Fatal("shutdown did not ask local service supervisor to stop owned services")
	}
}

func TestLocalServiceSupervisorDoesNotOwnExternalKernel(t *testing.T) {
	proc := &fakeSidecarProcess{pid: 9876}
	supervisor := NewLocalServiceSupervisor(LocalServiceSupervisorConfig{
		KernelBaseURL: "http://127.0.0.1:9999",
		External:      true,
		launcher: func(context.Context, sidecarLaunchRequest) (sidecarProcess, error) {
			t.Fatal("external kernel must not launch a sidecar")
			return proc, nil
		},
	})

	started := supervisor.StartKernel(context.Background())
	stopped := supervisor.StopOwned(context.Background())

	if started.Ownership != "external" || stopped.Ownership != "external" {
		t.Fatalf("statuses = %+v %+v, want external ownership", started, stopped)
	}
	if proc.stopCalls != 0 {
		t.Fatalf("external shutdown stopped process %d times", proc.stopCalls)
	}
}

func TestLocalServiceSupervisorReportsStructuredStartFailure(t *testing.T) {
	supervisor := NewLocalServiceSupervisor(LocalServiceSupervisorConfig{
		KernelBaseURL: defaultKernelBaseURL,
		LogDir:        desktopTestTempDir(t),
		launcher: func(context.Context, sidecarLaunchRequest) (sidecarProcess, error) {
			return nil, errors.New("boom")
		},
	})

	status := supervisor.StartKernel(context.Background())

	if status.Readiness != "not_ready" || status.Reason != sidecarStartFailed {
		t.Fatalf("status = %+v, want structured start failure", status)
	}
	if status.LogPath == "" {
		t.Fatalf("status = %+v, want diagnostic log path", status)
	}
}

func TestLocalServiceSupervisorShutdownOnlyStopsOwnedProcessOnce(t *testing.T) {
	proc := &fakeSidecarProcess{pid: 2468}
	supervisor := NewLocalServiceSupervisor(LocalServiceSupervisorConfig{
		KernelBaseURL: defaultKernelBaseURL,
		LogDir:        desktopTestTempDir(t),
		launcher: func(context.Context, sidecarLaunchRequest) (sidecarProcess, error) {
			return proc, nil
		},
		readinessProbe: func(context.Context, string, string) sidecarReadinessResult {
			return sidecarReadinessResult{Ready: true}
		},
	})

	supervisor.StartKernel(context.Background())
	first := supervisor.StopOwned(context.Background())
	second := supervisor.StopOwned(context.Background())

	if proc.stopCalls != 1 {
		t.Fatalf("stop calls = %d, want exactly one owned process stop", proc.stopCalls)
	}
	if first.Readiness != "not_ready" || second.Readiness != "not_ready" || second.Reason != sidecarStopped {
		t.Fatalf("shutdown statuses = %+v %+v, want idempotent stopped state", first, second)
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

func TestTypedListSessionsBridgeReadsKernelSessionIndex(t *testing.T) {
	var gotPath string
	var gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"session_id":"s1","title":"first"}]}`))
	}))
	defer server.Close()

	app := NewApp()
	app.client = NewKernelHTTPClient(server.URL, "token", server.Client())
	payload, err := app.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions returned error: %v", err)
	}

	if gotPath != "/sessions" || gotMethod != http.MethodGet {
		t.Fatalf("request = %s %s, want GET /sessions", gotMethod, gotPath)
	}
	items, ok := payload["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("payload = %+v, want one session item", payload)
	}
}

func TestKernelHTTPClientStreamsTurnEventsFromKernelPrimitive(t *testing.T) {
	var gotPath string
	var gotMethod string
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte(`{"type":"assistant_delta","delta":"你"}` + "\n"))
		_, _ = w.Write([]byte(`{"type":"assistant_delta","delta":"好"}` + "\n"))
		_, _ = w.Write([]byte(`{"type":"turn_completed","response":{"session_id":"s1","turn_id":"t1","final":{"text":"你好","model":"m"}}}` + "\n"))
	}))
	defer server.Close()

	client := NewKernelHTTPClient(server.URL, "token", server.Client())
	var deltas []string
	final, err := client.StreamJSONLines(context.Background(), "/turn/stream", true, json.RawMessage(`{"session_id":"s1"}`), func(payload map[string]any) error {
		if payload["type"] == "assistant_delta" {
			deltas = append(deltas, payload["delta"].(string))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("StreamJSONLines returned error: %v", err)
	}

	if gotPath != "/turn/stream" || gotMethod != http.MethodPost || gotAuth != "Bearer token" {
		t.Fatalf("request = %s %s auth %q, want POST /turn/stream with token", gotMethod, gotPath, gotAuth)
	}
	if strings.Join(deltas, "") != "你好" {
		t.Fatalf("deltas = %v, want 你好", deltas)
	}
	if final["turn_id"] != "t1" {
		t.Fatalf("final = %+v, want turn response", final)
	}
}

func TestKernelHTTPClientAcceptsPausedTurnStreamTerminalEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte(`{"type":"turn_paused","response":{"session_id":"s1","turn_id":"t1","pause":{"wait_reason":"budget_pause"}}}` + "\n"))
	}))
	defer server.Close()

	client := NewKernelHTTPClient(server.URL, "token", server.Client())
	final, err := client.StreamJSONLines(context.Background(), "/turn/stream", true, json.RawMessage(`{"session_id":"s1"}`), nil)
	if err != nil {
		t.Fatalf("StreamJSONLines returned error: %v", err)
	}
	pause, ok := final["pause"].(map[string]any)
	if !ok || pause["wait_reason"] != "budget_pause" {
		t.Fatalf("final = %+v, want paused turn response", final)
	}
}

func TestUploadMaterialBridgePostsMultipartThroughGoChokePoint(t *testing.T) {
	source := filepath.Join(desktopTestTempDir(t), "package.zip")
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
			"NewSQLiteLedger",
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

func TestFrontendDoesNotManageLocalProcesses(t *testing.T) {
	forbidden := []string{
		"child_process",
		"process.kill",
		"spawn(",
		"exec(",
		"GENESIS_KERNEL_BASE_URL",
	}
	err := filepath.WalkDir(filepath.Join("frontend", "src"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		switch filepath.Ext(path) {
		case ".ts", ".vue":
		default:
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(body)
		for _, needle := range forbidden {
			if strings.Contains(text, needle) {
				t.Fatalf("%s contains process-management surface %q", path, needle)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk frontend source: %v", err)
	}
}

func TestFrontendAssetDirPrefersPackagedExecutableLayout(t *testing.T) {
	root := desktopTestTempDir(t)
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
