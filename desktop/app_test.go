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
	"sync"
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

func TestCreateTaskWorkspaceUsesPersistentRootAndSessionID(t *testing.T) {
	root := filepath.Join(desktopTestTempDir(t), "Genesis")
	workspace, err := createTaskWorkspace(root, "desktop-task-1")
	if err != nil {
		t.Fatalf("create task workspace: %v", err)
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("absolute root: %v", err)
	}
	want := filepath.Join(absoluteRoot, "desktop-task-1")
	if workspace != want {
		t.Fatalf("workspace = %q, want %q", workspace, want)
	}
	if info, err := os.Stat(workspace); err != nil || !info.IsDir() {
		t.Fatalf("workspace directory = %v, %v; want existing directory", info, err)
	}
}

func TestCreateTaskWorkspaceRejectsSessionPathTraversal(t *testing.T) {
	_, err := createTaskWorkspace(desktopTestTempDir(t), "../outside")
	if err == nil {
		t.Fatal("create task workspace accepted path traversal session id")
	}
}

type fakeSidecarProcess struct {
	pid       int
	stopCalls int
	stopErr   error
}

func (p *fakeSidecarProcess) PID() int {
	return p.pid
}

func (p *fakeSidecarProcess) Stop(context.Context) error {
	p.stopCalls++
	return p.stopErr
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
	app.localModel = NewLocalModelSupervisor(LocalModelSupervisorConfig{
		Runtime: localModelRuntimeConfig{Enabled: false},
	})

	app.startup(context.Background())
	if !supervisor.startAttempted {
		t.Fatal("startup did not ask local service supervisor to start owned services")
	}

	app.shutdown(context.Background())
	if !supervisor.stopAttempted {
		t.Fatal("shutdown did not ask local service supervisor to stop owned services")
	}
}

func TestDesktopStartupLeavesLocalModelStoppedUntilManualStart(t *testing.T) {
	app := NewApp()
	app.supervisor = NewLocalServiceSupervisor(LocalServiceSupervisorConfig{
		KernelBaseURL: "http://127.0.0.1:9999",
		External:      true,
	})
	process := &fakeLocalModelProcess{fakeSidecarProcess: fakeSidecarProcess{pid: 2468}, done: make(chan struct{})}
	app.localModel = NewLocalModelSupervisor(LocalModelSupervisorConfig{
		Runtime: localModelRuntimeConfig{
			Enabled:          true,
			WSLDistribution:  "Ubuntu",
			ServerPath:       "/home/tomczz/tools/llama.cpp/llama-server",
			ModelPath:        "/home/tomczz/.genesis/models/qwen.gguf",
			HealthURL:        "http://127.0.0.1:8081/health",
			Port:             8081,
			ContextTokens:    262144,
			GPUOffloadLayers: "auto",
			CacheTypeK:       "q8_0",
			CacheTypeV:       "q8_0",
			Parallel:         2,
		},
		launcher: func(context.Context, localModelLaunchRequest) (localModelProcess, error) {
			return process, nil
		},
		readinessProbe: func(context.Context, string) sidecarReadinessResult {
			return sidecarReadinessResult{Ready: true}
		},
	})

	app.startup(context.Background())
	if status := app.LocalModelStatus(); status.Ownership != serviceOwnershipUnowned || status.Reason != localModelStopped {
		t.Fatalf("local model after startup = %+v, want stopped and unowned", status)
	}

	if status := app.StartLocalModel(); status.Ownership != serviceOwnershipOwned || status.PID != process.pid {
		t.Fatalf("local model after manual start = %+v, want the owned fake process", status)
	}

	app.shutdown(context.Background())
	if process.stopCalls != 1 {
		t.Fatalf("local model stop calls = %d, want exactly one owned-process stop", process.stopCalls)
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

type fakeLocalModelProcess struct {
	fakeSidecarProcess
	done chan struct{}
}

func localModelTestRuntime() localModelRuntimeConfig {
	return localModelRuntimeConfig{
		Enabled:          true,
		WSLDistribution:  "Ubuntu",
		ServerPath:       "/home/tomczz/tools/llama.cpp/llama-server",
		ModelPath:        "/home/tomczz/.genesis/models/qwen.gguf",
		HealthURL:        "http://127.0.0.1:8081/health",
		Port:             8081,
		ContextTokens:    262144,
		GPUOffloadLayers: "auto",
		CacheTypeK:       "q8_0",
		CacheTypeV:       "q8_0",
		Parallel:         2,
	}
}

func (p *fakeLocalModelProcess) Done() <-chan struct{} {
	return p.done
}

func TestLocalModelSupervisorStopsOnlyItsOwnedWSLProcess(t *testing.T) {
	proc := &fakeLocalModelProcess{fakeSidecarProcess: fakeSidecarProcess{pid: 5678}, done: make(chan struct{})}
	launched := false
	supervisor := NewLocalModelSupervisor(LocalModelSupervisorConfig{
		Runtime: localModelRuntimeConfig{
			Enabled:          true,
			WSLDistribution:  "Ubuntu",
			ServerPath:       "/home/tomczz/tools/llama.cpp/llama-server",
			ModelPath:        "/home/tomczz/.genesis/models/qwen.gguf",
			HealthURL:        "http://127.0.0.1:8081/health",
			Port:             8081,
			ContextTokens:    262144,
			GPUOffloadLayers: "auto",
			CacheTypeK:       "q8_0",
			CacheTypeV:       "q8_0",
			Parallel:         2,
		},
		launcher: func(context.Context, localModelLaunchRequest) (localModelProcess, error) {
			launched = true
			return proc, nil
		},
		readinessProbe: func(context.Context, string) sidecarReadinessResult {
			return sidecarReadinessResult{Ready: true}
		},
	})

	started := supervisor.Start(context.Background())
	if !launched || started.Ownership != serviceOwnershipOwned || started.Readiness != "ready" || started.PID != 5678 {
		t.Fatalf("started = %+v, launched = %t", started, launched)
	}
	stopped := supervisor.StopOwned(context.Background())
	if proc.stopCalls != 1 || stopped.Reason != localModelStopped {
		t.Fatalf("stopped = %+v, stop calls = %d", stopped, proc.stopCalls)
	}
	supervisor.StopOwned(context.Background())
	if proc.stopCalls != 1 {
		t.Fatalf("stop calls = %d, want exactly one owned stop", proc.stopCalls)
	}
}

func TestLocalModelSupervisorLeavesDisabledRuntimeUnowned(t *testing.T) {
	supervisor := NewLocalModelSupervisor(LocalModelSupervisorConfig{
		Runtime: localModelRuntimeConfig{Enabled: false},
		launcher: func(context.Context, localModelLaunchRequest) (localModelProcess, error) {
			t.Fatal("disabled local runtime must not launch")
			return nil, nil
		},
	})

	status := supervisor.Start(context.Background())
	if status.Ownership != serviceOwnershipUnowned || status.Reason != localModelDisabled {
		t.Fatalf("status = %+v, want disabled unowned model", status)
	}
}

func TestLocalModelSupervisorRetainsOwnedProcessWhenStopFails(t *testing.T) {
	proc := &fakeLocalModelProcess{
		fakeSidecarProcess: fakeSidecarProcess{pid: 6789, stopErr: errors.New("taskkill failed")},
		done:               make(chan struct{}),
	}
	supervisor := NewLocalModelSupervisor(LocalModelSupervisorConfig{
		Runtime: localModelTestRuntime(),
		launcher: func(context.Context, localModelLaunchRequest) (localModelProcess, error) {
			return proc, nil
		},
		readinessProbe: func(context.Context, string) sidecarReadinessResult {
			return sidecarReadinessResult{Ready: true}
		},
	})

	supervisor.Start(context.Background())
	failed := supervisor.StopOwned(context.Background())
	if failed.Ownership != serviceOwnershipOwned || failed.PID != proc.pid || failed.Reason != localModelStopFailed {
		t.Fatalf("failed stop = %+v, want retained owned process", failed)
	}
	proc.stopErr = nil
	stopped := supervisor.StopOwned(context.Background())
	if proc.stopCalls != 2 || stopped.Reason != localModelStopped {
		t.Fatalf("successful retry = %+v, stop calls = %d", stopped, proc.stopCalls)
	}
}

type blockingLocalModelProcess struct {
	pid         int
	done        chan struct{}
	stopStarted chan struct{}
	releaseStop chan struct{}
	mu          sync.Mutex
	stopCalls   int
}

func (p *blockingLocalModelProcess) PID() int {
	return p.pid
}

func (p *blockingLocalModelProcess) Done() <-chan struct{} {
	return p.done
}

func (p *blockingLocalModelProcess) Stop(context.Context) error {
	p.mu.Lock()
	p.stopCalls++
	p.mu.Unlock()
	close(p.stopStarted)
	<-p.releaseStop
	return nil
}

func (p *blockingLocalModelProcess) StopCalls() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stopCalls
}

func TestLocalModelSupervisorSerializesConcurrentOwnedStops(t *testing.T) {
	proc := &blockingLocalModelProcess{
		pid:         1357,
		done:        make(chan struct{}),
		stopStarted: make(chan struct{}),
		releaseStop: make(chan struct{}),
	}
	supervisor := NewLocalModelSupervisor(LocalModelSupervisorConfig{
		Runtime: localModelTestRuntime(),
		launcher: func(context.Context, localModelLaunchRequest) (localModelProcess, error) {
			return proc, nil
		},
		readinessProbe: func(context.Context, string) sidecarReadinessResult {
			return sidecarReadinessResult{Ready: true}
		},
	})
	supervisor.Start(context.Background())

	firstDone := make(chan SidecarStatus, 1)
	go func() {
		firstDone <- supervisor.StopOwned(context.Background())
	}()
	<-proc.stopStarted
	second := supervisor.StopOwned(context.Background())
	if proc.StopCalls() != 1 || second.PID != proc.pid {
		t.Fatalf("second stop = %+v, stop calls = %d, want no second process termination", second, proc.StopCalls())
	}
	close(proc.releaseStop)
	if first := <-firstDone; first.Reason != localModelStopped {
		t.Fatalf("first stop = %+v, want stopped state", first)
	}
}

func TestLocalModelSupervisorDoesNotClaimOwnershipWhenLaunchFails(t *testing.T) {
	supervisor := NewLocalModelSupervisor(LocalModelSupervisorConfig{
		Runtime: localModelRuntimeConfig{
			Enabled:          true,
			WSLDistribution:  "Ubuntu",
			ServerPath:       "/home/tomczz/tools/llama.cpp/llama-server",
			ModelPath:        "/home/tomczz/.genesis/models/qwen.gguf",
			HealthURL:        "http://127.0.0.1:8081/health",
			Port:             8081,
			ContextTokens:    262144,
			GPUOffloadLayers: "auto",
			CacheTypeK:       "q8_0",
			CacheTypeV:       "q8_0",
			Parallel:         2,
		},
		LogDir: desktopTestTempDir(t),
		launcher: func(context.Context, localModelLaunchRequest) (localModelProcess, error) {
			return nil, errors.New("wsl launch failed")
		},
	})

	status := supervisor.Start(context.Background())
	if status.Ownership != serviceOwnershipUnowned || status.Reason != localModelStartFailed {
		t.Fatalf("status = %+v, want unowned launch failure", status)
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

func TestTypedSearchSessionsBridgeReadsKernelSearchProjection(t *testing.T) {
	var gotPath string
	var gotMethod string
	var gotAuth string
	var gotQuery string
	var gotLimit string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotAuth = r.Header.Get("Authorization")
		gotQuery = r.URL.Query().Get("q")
		gotLimit = r.URL.Query().Get("limit")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"query":"basalt notes","items":[{"session_id":"s1","title":"first","match_fields":["title"],"snippet":"Basalt notes"}]}`))
	}))
	defer server.Close()

	app := NewApp()
	app.client = NewKernelHTTPClient(server.URL, "token", server.Client())
	payload, err := app.SearchSessions(" basalt notes ", 5)
	if err != nil {
		t.Fatalf("SearchSessions returned error: %v", err)
	}

	if gotPath != "/sessions/search" || gotMethod != http.MethodGet || gotAuth != "Bearer token" {
		t.Fatalf("request = %s %s auth %q, want GET /sessions/search with token", gotMethod, gotPath, gotAuth)
	}
	if gotQuery != "basalt notes" || gotLimit != "5" {
		t.Fatalf("query = %q limit = %q, want trimmed query and limit", gotQuery, gotLimit)
	}
	items, ok := payload["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("payload = %+v, want one search result", payload)
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

func TestEmbeddedFrontendAssetsContainIndex(t *testing.T) {
	file, err := assets.Open("frontend/dist/index.html")
	if err != nil {
		t.Fatalf("assets.Open(index.html) error = %v", err)
	}
	defer file.Close()
}

func TestSingleInstanceLockIsConfigured(t *testing.T) {
	lock := singleInstanceLock(NewApp())

	if lock == nil || lock.UniqueId == "" || lock.OnSecondInstanceLaunch == nil {
		t.Fatalf("singleInstanceLock() = %+v, want unique id and second-launch handler", lock)
	}
}
