package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
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

func TestCreateProjectWorkspaceUsesNamedGenesisDirectory(t *testing.T) {
	root := filepath.Join(desktopTestTempDir(t), "Genesis")
	workspace, existing, err := createProjectWorkspace(root, "alpha")
	if err != nil {
		t.Fatalf("create project workspace: %v", err)
	}
	want, err := filepath.Abs(filepath.Join(root, "alpha"))
	if err != nil {
		t.Fatalf("absolute project root: %v", err)
	}
	if workspace != want {
		t.Fatalf("project workspace = %q, want %q", workspace, want)
	}
	if existing {
		t.Fatal("new project workspace reported existing")
	}
	if _, existing, err := createProjectWorkspace(root, "alpha"); err != nil || !existing {
		t.Fatalf("existing project workspace = %q, %v, want existing root", workspace, err)
	}
	if _, _, err := createProjectWorkspace(root, "../outside"); err == nil {
		t.Fatal("create project workspace accepted path traversal name")
	}
}

func TestDesktopCloseBehaviorDefaultsToExitAndPersistsTraySelection(t *testing.T) {
	dir := desktopTestTempDir(t)
	previous := desktopUserConfigDir
	desktopUserConfigDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { desktopUserConfigDir = previous })

	if got := loadDesktopCloseBehavior(); got != closeBehaviorExit {
		t.Fatalf("default close behavior = %q, want %q", got, closeBehaviorExit)
	}
	if err := saveDesktopCloseBehavior(closeBehaviorTray); err != nil {
		t.Fatalf("save tray close behavior: %v", err)
	}
	if got := loadDesktopCloseBehavior(); got != closeBehaviorTray {
		t.Fatalf("persisted close behavior = %q, want %q", got, closeBehaviorTray)
	}
	if _, err := normalizedCloseBehavior("unexpected"); err == nil {
		t.Fatal("accepted unknown close behavior")
	}
}

func TestDesktopCatalogPersistsProjectAndSessionMetadataUnderGenesisHome(t *testing.T) {
	dir := desktopTestTempDir(t)
	previous := desktopCatalogHomeDir
	desktopCatalogHomeDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { desktopCatalogHomeDir = previous })
	catalog := DesktopCatalogProjection{
		Projects: []DesktopProjectCatalogProjection{{ProjectID: "project-a", Name: "Alpha", Root: "D:\\work\\alpha"}},
		Sessions: []DesktopSessionCatalogProjection{{SessionID: "session-a", Kind: "project", ProjectID: "project-a", Root: "D:\\work\\alpha"}},
	}
	if err := saveDesktopCatalog(catalog); err != nil {
		t.Fatalf("save desktop catalog: %v", err)
	}
	loaded, err := loadDesktopCatalog()
	if err != nil {
		t.Fatalf("load desktop catalog: %v", err)
	}
	if !slices.Equal(loaded.Projects, catalog.Projects) || !slices.Equal(loaded.Sessions, catalog.Sessions) {
		t.Fatalf("catalog = %+v, want %+v", loaded, catalog)
	}
	path, err := desktopCatalogPath()
	if err != nil {
		t.Fatalf("catalog path: %v", err)
	}
	if !strings.Contains(filepath.ToSlash(path), "/.genesis/desktop/catalog.json") {
		t.Fatalf("catalog path = %q, want Genesis Home desktop catalog", path)
	}
}

func TestBeforeCloseHidesOnlyForTrayBehavior(t *testing.T) {
	app := &App{closeBehavior: closeBehaviorExit}
	if app.beforeClose(context.Background()) {
		t.Fatal("exit behavior blocked window close")
	}
	app.closeBehavior = closeBehaviorTray
	if !app.beforeClose(context.Background()) {
		t.Fatal("tray behavior did not block window close")
	}
	app.requestExit()
	if app.beforeClose(context.Background()) {
		t.Fatal("requested exit was blocked by tray behavior")
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

func TestOwnedDesktopConfigGeneratesRuntimeToken(t *testing.T) {
	t.Setenv("GENESIS_KERNEL_BASE_URL", "")
	t.Setenv("GENESIS_RUNTIME_TOKEN", "")
	config := loadDesktopConfig()
	if len(config.RuntimeToken) != 64 {
		t.Fatalf("owned runtime token length = %d, want 64", len(config.RuntimeToken))
	}
}

func TestExternalDesktopConfigDoesNotInventRuntimeToken(t *testing.T) {
	t.Setenv("GENESIS_KERNEL_BASE_URL", "http://127.0.0.1:9999")
	t.Setenv("GENESIS_RUNTIME_TOKEN", "")
	if token := loadDesktopConfig().RuntimeToken; token != "" {
		t.Fatalf("external runtime token = %q, want empty", token)
	}
}

func TestGenesisdCommandUsesPrivateRuntime(t *testing.T) {
	dir := desktopTestTempDir(t)
	runtimeDir := filepath.Join(dir, "kernel")
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		t.Fatalf("create runtime directory: %v", err)
	}
	bundledKernel := filepath.Join(runtimeDir, "genesisd.exe")
	if err := os.WriteFile(bundledKernel, []byte("test"), 0o600); err != nil {
		t.Fatalf("write bundled kernel: %v", err)
	}
	previous := desktopExecutablePath
	desktopExecutablePath = func() (string, error) { return filepath.Join(dir, "genesis-desktop.exe"), nil }
	t.Cleanup(func() { desktopExecutablePath = previous })

	exe, args, workDir, err := genesisdCommand(sidecarLaunchRequest{})
	if err != nil {
		t.Fatalf("genesisdCommand returned error: %v", err)
	}
	if exe != bundledKernel || len(args) != 0 || workDir != runtimeDir {
		t.Fatalf("command = %q %v in %q, want private runtime kernel", exe, args, workDir)
	}
}

func TestNSISInstallerRemembersPathAndDoesNotRecursivelyDeleteIt(t *testing.T) {
	payload, err := os.ReadFile(filepath.Join("build", "windows", "installer", "project.nsi"))
	if err != nil {
		t.Fatalf("read installer project: %v", err)
	}
	text := string(payload)
	for _, required := range []string{
		"InstallDirRegKey HKLM \"Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\${UNINST_KEY_NAME}\" \"InstallLocation\"",
		"WriteRegStr HKLM \"${UNINST_KEY}\" \"InstallLocation\" \"$INSTDIR\"",
		"InstallDir \"D:\\software\\Genesis\"",
		"SetOutPath \"$INSTDIR\\kernel\"",
		"Delete \"$INSTDIR\\kernel\\genesisd.exe\"",
		"SetOutPath \"$INSTDIR\\kernel\\scripts\\providers\"",
		"File \"/oname=llama_cpp_provider_command.py\" \"..\\..\\bin\\scripts\\providers\\llama_cpp_provider_command.py\"",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("installer project missing %q", required)
		}
	}
	if strings.Contains(text, "RMDir /r $INSTDIR") {
		t.Fatal("installer recursively deletes the selected installation directory")
	}
	if strings.Contains(text, "RMDir \"$INSTDIR\"") {
		t.Fatal("installer must preserve the selected installation directory on uninstall")
	}
	runtimeSetOutPath := strings.Index(text, "SetOutPath \"$INSTDIR\\kernel\"")
	kernelFile := strings.Index(text, "File \"/oname=genesisd.exe\" \"..\\..\\bin\\genesisd.exe\"")
	if runtimeSetOutPath < 0 || kernelFile < runtimeSetOutPath {
		t.Fatal("installer does not place genesisd.exe in the private runtime directory")
	}
}

func TestDesktopReleaseCopiesLocalProviderAdapter(t *testing.T) {
	payload, err := os.ReadFile(filepath.Join("..", "scripts", "build_desktop_release.ps1"))
	if err != nil {
		t.Fatalf("read release script: %v", err)
	}
	if !strings.Contains(string(payload), "llama_cpp_provider_command.py") {
		t.Fatal("desktop release must package the local provider adapter")
	}
}

func TestNewAppRepairsOnlyExistingLocalProviderAdapterPath(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "genesis-desktop.exe")
	adapter := filepath.Join(dir, "kernel", "scripts", "providers", "llama_cpp_provider_command.py")
	if err := os.MkdirAll(filepath.Dir(adapter), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(adapter, []byte("adapter"), 0o644); err != nil {
		t.Fatal(err)
	}
	configRoot := filepath.Join(dir, "config")
	if err := os.MkdirAll(configRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configRoot, "models.json"), []byte(`{"model_gateway":{"routes":{"local":{"protocol":"provider_command","command":"python.exe","args":["C:\\old\\llama_cpp_provider_command.py","--base-url","http://127.0.0.1:8081/v1"]}}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GENESIS_CONFIG_ROOT", configRoot)
	previous := desktopExecutablePath
	desktopExecutablePath = func() (string, error) { return executable, nil }
	defer func() { desktopExecutablePath = previous }()
	_ = NewApp()
	payload, err := os.ReadFile(filepath.Join(configRoot, "models.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), strings.ReplaceAll(adapter, `\`, `\\`)) {
		t.Fatalf("config did not use packaged adapter: %s", payload)
	}
}

func TestDesktopUpdateLauncherHidesTheHelperProcess(t *testing.T) {
	payload, err := os.ReadFile("update_launch_windows.go")
	if err != nil {
		t.Fatalf("read Windows update launcher: %v", err)
	}
	text := string(payload)
	if !strings.Contains(text, "noConsoleSysProcAttr()") {
		t.Fatal("update installer launcher must suppress its console window")
	}
}

func TestDesktopProcessLaunchersDoNotCreateConsoleWindows(t *testing.T) {
	for _, name := range []string{"local_service_supervisor.go", "local_model_supervisor.go", "update_launch_windows.go"} {
		payload, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if !strings.Contains(string(payload), "noConsoleSysProcAttr()") {
			t.Fatalf("%s must suppress console windows", name)
		}
	}
	payload, err := os.ReadFile("local_service_supervisor.go")
	if err != nil {
		t.Fatalf("read local service supervisor: %v", err)
	}
	if !strings.Contains(string(payload), "CreationFlags: windowsCreateNoWindow") {
		t.Fatal("desktop process attributes must use CREATE_NO_WINDOW")
	}
}

func TestWSLLocalModelLaunchUsesOwnedPIDFile(t *testing.T) {
	args := wslLocalModelArgs(localModelLaunchRequest{
		Runtime: localModelRuntimeConfig{
			WSLDistribution:  "Ubuntu",
			ServerPath:       "/home/tomczz/tools/llama-server",
			ModelPath:        "/home/tomczz/.genesis/models/qwen.gguf",
			Host:             "0.0.0.0",
			Port:             8081,
			ContextTokens:    262144,
			GPUOffloadLayers: "auto",
			CacheTypeK:       "q8_0",
			CacheTypeV:       "q8_0",
			Parallel:         2,
		},
		PIDPath: "/tmp/genesis-local-model-test.pid",
	})
	joined := strings.Join(args, " ")
	for _, required := range []string{"--exec /bin/sh -c", "/tmp/genesis-local-model-test.pid", "/home/tomczz/tools/llama-server", "--port 8081"} {
		if !strings.Contains(joined, required) {
			t.Fatalf("WSL launch args missing %q: %q", required, joined)
		}
	}
}

func TestDesktopReleaseBuildWritesInstallerChecksum(t *testing.T) {
	payload, err := os.ReadFile(filepath.Join("..", "scripts", "build_desktop_release.ps1"))
	if err != nil {
		t.Fatalf("read desktop release script: %v", err)
	}
	text := string(payload)
	for _, required := range []string{"Get-FileHash", "genesis-desktop-amd64-installer.exe.sha256", "Set-Content", "Program Files (x86)\\NSIS", "makensis.exe"} {
		if !strings.Contains(text, required) {
			t.Fatalf("desktop release script missing %q", required)
		}
	}
}

func TestDesktopVersionMatchesInstallerProductVersion(t *testing.T) {
	payload, err := os.ReadFile("wails.json")
	if err != nil {
		t.Fatalf("read wails config: %v", err)
	}
	var config struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(payload, &config); err != nil {
		t.Fatalf("decode wails config: %v", err)
	}
	if desktopVersion != config.Info.ProductVersion {
		t.Fatalf("desktop version = %q, installer product version = %q", desktopVersion, config.Info.ProductVersion)
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

func TestLocalServiceSupervisorRefusesReadyUnownedKernel(t *testing.T) {
	launched := false
	supervisor := NewLocalServiceSupervisor(LocalServiceSupervisorConfig{
		KernelBaseURL: defaultKernelBaseURL,
		LogDir:        desktopTestTempDir(t),
		launcher: func(context.Context, sidecarLaunchRequest) (sidecarProcess, error) {
			launched = true
			return &fakeSidecarProcess{pid: 9876}, nil
		},
		endpointOccupiedProbe: func(context.Context, string) bool {
			return true
		},
	})

	status := supervisor.StartKernel(context.Background())

	if launched {
		t.Fatal("occupied kernel endpoint launched a sidecar")
	}
	if status.Ownership != serviceOwnershipUnowned || status.Readiness != "not_ready" || status.Reason != sidecarKernelAlreadyServing {
		t.Fatalf("status = %+v, want unowned occupied-kernel status", status)
	}
}

func TestProbeKernelEndpointOccupiedDetectsListeningPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	endpoint := "http://" + listener.Addr().String()
	if !probeKernelEndpointOccupied(context.Background(), endpoint) {
		t.Fatal("listening endpoint was not detected as occupied")
	}
	if err := listener.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if probeKernelEndpointOccupied(ctx, endpoint) {
		t.Fatal("closed endpoint was detected as occupied")
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

func TestLocalServiceSupervisorRetainsOwnedProcessWhenStopFails(t *testing.T) {
	proc := &fakeSidecarProcess{pid: 2468, stopErr: errors.New("taskkill failed")}
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
	failed := supervisor.StopOwned(context.Background())
	if failed.Ownership != serviceOwnershipOwned || failed.PID != proc.pid || failed.Reason != sidecarStopFailed {
		t.Fatalf("failed stop = %+v, want retained owned process", failed)
	}
	proc.stopErr = nil
	stopped := supervisor.StopOwned(context.Background())
	if proc.stopCalls != 2 || stopped.Reason != sidecarStopped {
		t.Fatalf("successful retry = %+v, stop calls = %d", stopped, proc.stopCalls)
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

func TestLocalModelSupervisorLeavesReadyEndpointUnowned(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	runtime := localModelTestRuntime()
	runtime.HealthURL = server.URL
	launched := false
	supervisor := NewLocalModelSupervisor(LocalModelSupervisorConfig{
		Runtime: runtime,
		launcher: func(context.Context, localModelLaunchRequest) (localModelProcess, error) {
			launched = true
			return nil, errors.New("external endpoint must prevent launch")
		},
	})

	status := supervisor.Start(context.Background())
	if launched || status.Ownership != serviceOwnershipUnowned || status.Readiness != "ready" || status.Reason != localModelEndpointAlreadyServing {
		t.Fatalf("status = %+v, launched = %t; want unowned serving endpoint", status, launched)
	}
	if stopped := supervisor.StopOwned(context.Background()); stopped != status {
		t.Fatalf("StopOwned() = %+v, want unchanged unowned endpoint", stopped)
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

func TestProbeLocalModelReadinessWaitsForDelayedServer(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if result := probeLocalModelReadiness(ctx, server.URL); !result.Ready || attempts < 2 {
		t.Fatalf("readiness result = %+v after %d attempts, want ready after retry", result, attempts)
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

func TestKernelHTTPClientStreamsTerminalResponseBeyondScannerLimit(t *testing.T) {
	text := strings.Repeat("x", 4*1024*1024+1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = fmt.Fprintf(w, `{"type":"turn_completed","response":{"session_id":"s1","turn_id":"t1","final":{"text":%q}}}`+"\n", text)
	}))
	defer server.Close()

	client := NewKernelHTTPClient(server.URL, "token", server.Client())
	final, err := client.StreamJSONLines(context.Background(), "/turn/stream", true, json.RawMessage(`{"session_id":"s1"}`), nil)
	if err != nil {
		t.Fatalf("StreamJSONLines returned error: %v", err)
	}
	responseFinal, ok := final["final"].(map[string]any)
	if !ok || responseFinal["text"] != text {
		t.Fatalf("final response did not preserve the complete long text")
	}
}

func TestTurnRequestContextHasNoOuterDeadline(t *testing.T) {
	app := &App{ctx: context.Background()}
	ctx, cancel := app.turnRequestContext()
	defer cancel()
	if _, ok := ctx.Deadline(); ok {
		t.Fatal("turn request context must not impose an outer deadline")
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

func TestUploadMaterialBridgePackagesSelectedDirectory(t *testing.T) {
	root := filepath.Join(desktopTestTempDir(t), "repo")
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatalf("create source directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# repo\n"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "main.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("create metadata directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "config"), []byte("private metadata"), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("API_KEY=private"), 0o600); err != nil {
		t.Fatalf("write environment secret: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "id_ed25519"), []byte("private key"), 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "credentials.json"), []byte(`{"token":"private"}`), 0o600); err != nil {
		t.Fatalf("write credentials: %v", err)
	}

	var gotFilename string
	var archiveBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(4 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("FormFile: %v", err)
		}
		defer file.Close()
		gotFilename = header.Filename
		archiveBody, _ = io.ReadAll(file)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"admission_result":"admitted"}`))
	}))
	defer server.Close()

	app := NewApp()
	app.client = NewKernelHTTPClient(server.URL, "token", server.Client())
	if _, err := app.UploadMaterial(MaterialBridgeRequest{SessionID: "session-directory", Purpose: "source_analysis", FilePath: root}); err != nil {
		t.Fatalf("UploadMaterial directory returned error: %v", err)
	}
	if gotFilename != "repo.zip" {
		t.Fatalf("uploaded filename = %q, want repo.zip", gotFilename)
	}
	archive, err := zip.NewReader(strings.NewReader(string(archiveBody)), int64(len(archiveBody)))
	if err != nil {
		t.Fatalf("uploaded directory is not a zip archive: %v", err)
	}
	entries := make([]string, 0, len(archive.File))
	for _, file := range archive.File {
		entries = append(entries, file.Name)
	}
	if !slices.Contains(entries, "README.md") || !slices.Contains(entries, "src/main.go") {
		t.Fatalf("archive entries = %v, want project source files", entries)
	}
	for _, excluded := range []string{".env", ".git/config", "id_ed25519", "credentials.json"} {
		if slices.Contains(entries, excluded) {
			t.Fatalf("archive entries = %v, must exclude %s", entries, excluded)
		}
	}
}

func TestArchiveMaterialDirectoryRejectsOversizedSourceFile(t *testing.T) {
	root := filepath.Join(desktopTestTempDir(t), "repo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("create source directory: %v", err)
	}
	oversized := filepath.Join(root, "large.bin")
	file, err := os.Create(oversized)
	if err != nil {
		t.Fatalf("create oversized source: %v", err)
	}
	if err := file.Truncate(desktopMaterialMaxFileBytes + 1); err != nil {
		_ = file.Close()
		t.Fatalf("size oversized source: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close oversized source: %v", err)
	}
	if _, err := archiveMaterialDirectory(root); err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Fatalf("archiveMaterialDirectory error = %v, want oversized file refusal", err)
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
