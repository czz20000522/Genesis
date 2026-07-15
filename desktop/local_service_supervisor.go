package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	serviceKindKernel     = "kernel"
	windowsCreateNoWindow = 0x08000000

	serviceOwnershipOwned    = "owned"
	serviceOwnershipExternal = "external"

	sidecarExternalKernelConfigured = "external_kernel_configured"
	sidecarStopped                  = "sidecar_stopped"
	sidecarStarting                 = "sidecar_starting"
	sidecarStartFailed              = "sidecar_start_failed"
	sidecarStopFailed               = "sidecar_stop_failed"
	sidecarReadinessProbeFailed     = "kernel_readiness_probe_failed"
	sidecarKernelNotReady           = "kernel_not_ready"
	sidecarKernelAlreadyServing     = "kernel_already_serving"
)

func noConsoleSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true, CreationFlags: windowsCreateNoWindow}
}

type sidecarProcess interface {
	PID() int
	Stop(context.Context) error
}

type sidecarLauncher func(context.Context, sidecarLaunchRequest) (sidecarProcess, error)

type sidecarLaunchRequest struct {
	KernelBaseURL string
	RuntimeToken  string
	GenesisdPath  string
	WorkDir       string
	LogPath       string
}

type sidecarReadinessProbe func(context.Context, string, string) sidecarReadinessResult

type sidecarEndpointOccupiedProbe func(context.Context, string) bool

type sidecarReadinessResult struct {
	Ready  bool
	Reason string
}

type LocalServiceSupervisorConfig struct {
	KernelBaseURL    string
	RuntimeToken     string
	External         bool
	GenesisdPath     string
	WorkDir          string
	LogDir           string
	ReadinessTimeout time.Duration

	launcher              sidecarLauncher
	readinessProbe        sidecarReadinessProbe
	endpointOccupiedProbe sidecarEndpointOccupiedProbe
}

type LocalServiceSupervisor struct {
	cfg            LocalServiceSupervisorConfig
	status         SidecarStatus
	process        sidecarProcess
	startAttempted bool
	stopAttempted  bool
}

var desktopExecutablePath = os.Executable

func NewLocalServiceSupervisor(cfg LocalServiceSupervisorConfig) *LocalServiceSupervisor {
	cfg.KernelBaseURL = strings.TrimRight(strings.TrimSpace(cfg.KernelBaseURL), "/")
	cfg.RuntimeToken = strings.TrimSpace(cfg.RuntimeToken)
	cfg.GenesisdPath = strings.TrimSpace(cfg.GenesisdPath)
	cfg.WorkDir = strings.TrimSpace(cfg.WorkDir)
	cfg.LogDir = strings.TrimSpace(cfg.LogDir)
	if cfg.ReadinessTimeout <= 0 {
		cfg.ReadinessTimeout = 5 * time.Second
	}
	if cfg.launcher == nil {
		cfg.launcher = launchGenesisdSidecar
	}
	if cfg.readinessProbe == nil {
		cfg.readinessProbe = probeKernelReadiness
	}
	if cfg.endpointOccupiedProbe == nil {
		cfg.endpointOccupiedProbe = probeKernelEndpointOccupied
	}
	supervisor := &LocalServiceSupervisor{cfg: cfg}
	supervisor.status = supervisor.initialKernelStatus()
	return supervisor
}

func (s *LocalServiceSupervisor) KernelStatus() SidecarStatus {
	if s == nil {
		return SidecarStatus{}
	}
	return s.status
}

func (s *LocalServiceSupervisor) StartKernel(ctx context.Context) SidecarStatus {
	if s == nil {
		return SidecarStatus{}
	}
	if s.cfg.External {
		s.status = s.initialKernelStatus()
		return s.status
	}
	s.startAttempted = true
	if s.process != nil {
		return s.status
	}
	preflightCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	if s.cfg.endpointOccupiedProbe(preflightCtx, s.cfg.KernelBaseURL) {
		s.status = s.unownedStatus("not_ready", sidecarKernelAlreadyServing)
		return s.status
	}

	logPath, err := s.prepareLogPath()
	if err != nil {
		s.status = s.ownedStatus("not_ready", sidecarStartFailed, 0, "", "")
		return s.status
	}
	startedAt := time.Now().UTC().Format(time.RFC3339Nano)
	req := sidecarLaunchRequest{
		KernelBaseURL: s.cfg.KernelBaseURL,
		RuntimeToken:  s.cfg.RuntimeToken,
		GenesisdPath:  s.cfg.GenesisdPath,
		WorkDir:       s.cfg.WorkDir,
		LogPath:       logPath,
	}
	proc, err := s.cfg.launcher(ctx, req)
	if err != nil {
		s.status = s.ownedStatus("not_ready", sidecarStartFailed, 0, startedAt, logPath)
		return s.status
	}
	s.process = proc
	s.status = s.ownedStatus("not_ready", sidecarStarting, proc.PID(), startedAt, logPath)

	probeCtx, cancel := context.WithTimeout(ctx, s.cfg.ReadinessTimeout)
	defer cancel()
	result := s.cfg.readinessProbe(probeCtx, s.cfg.KernelBaseURL, s.cfg.RuntimeToken)
	if result.Ready {
		s.status = s.ownedStatus("ready", "", proc.PID(), startedAt, logPath)
		return s.status
	}
	reason := strings.TrimSpace(result.Reason)
	if reason == "" {
		reason = sidecarReadinessProbeFailed
	}
	s.status = s.ownedStatus("not_ready", reason, proc.PID(), startedAt, logPath)
	return s.status
}

func (s *LocalServiceSupervisor) StopOwned(ctx context.Context) SidecarStatus {
	if s == nil {
		return SidecarStatus{}
	}
	if s.cfg.External {
		s.status = s.initialKernelStatus()
		return s.status
	}
	s.stopAttempted = true
	if s.process != nil {
		_ = s.process.Stop(ctx)
		s.process = nil
	}
	s.status.Readiness = "not_ready"
	s.status.Reason = sidecarStopped
	return s.status
}

func (s *LocalServiceSupervisor) RestartOwned(ctx context.Context) SidecarStatus {
	if s == nil {
		return SidecarStatus{}
	}
	if s.cfg.External {
		s.status = s.initialKernelStatus()
		return s.status
	}
	if s.process != nil {
		pid := s.process.PID()
		startedAt := s.status.StartedAt
		logPath := s.status.LogPath
		if err := s.process.Stop(ctx); err != nil {
			s.status = s.ownedStatus("not_ready", sidecarStopFailed, pid, startedAt, logPath)
			return s.status
		}
		s.process = nil
	}
	return s.StartKernel(ctx)
}

func (s *LocalServiceSupervisor) initialKernelStatus() SidecarStatus {
	if s.cfg.External {
		return SidecarStatus{
			ServiceID: "kernel",
			Kind:      serviceKindKernel,
			Ownership: serviceOwnershipExternal,
			Readiness: "not_ready",
			Reason:    sidecarExternalKernelConfigured,
		}
	}
	return s.ownedStatus("not_ready", sidecarStarting, 0, "", "")
}

func (s *LocalServiceSupervisor) ownedStatus(readiness, reason string, pid int, startedAt, logPath string) SidecarStatus {
	return SidecarStatus{
		ServiceID: serviceKindKernel,
		Kind:      serviceKindKernel,
		Ownership: serviceOwnershipOwned,
		Readiness: readiness,
		Reason:    reason,
		PID:       pid,
		StartedAt: startedAt,
		LogPath:   logPath,
	}
}

func (s *LocalServiceSupervisor) unownedStatus(readiness, reason string) SidecarStatus {
	return SidecarStatus{
		ServiceID: serviceKindKernel,
		Kind:      serviceKindKernel,
		Ownership: serviceOwnershipUnowned,
		Readiness: readiness,
		Reason:    reason,
	}
}

func (s *LocalServiceSupervisor) prepareLogPath() (string, error) {
	dir := s.cfg.LogDir
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "genesis-desktop", "sidecars")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "genesisd.log"), nil
}

type localSidecarProcess struct {
	cmd      *exec.Cmd
	waitDone chan error
	stopOnce sync.Once
}

func (p *localSidecarProcess) PID() int {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

func (p *localSidecarProcess) Stop(ctx context.Context) error {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	var stopErr error
	p.stopOnce.Do(func() {
		stopErr = killLocalProcessTree(ctx, p.cmd)
		select {
		case <-ctx.Done():
			if stopErr == nil {
				stopErr = ctx.Err()
			}
		case <-p.waitDone:
		case <-time.After(3 * time.Second):
			if stopErr == nil {
				stopErr = errors.New("sidecar process did not exit after kill")
			}
		}
	})
	return stopErr
}

func launchGenesisdSidecar(_ context.Context, req sidecarLaunchRequest) (sidecarProcess, error) {
	logFile, err := os.OpenFile(req.LogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	exe, args, workDir, err := genesisdCommand(req)
	if err != nil {
		_ = logFile.Close()
		return nil, err
	}
	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = noConsoleSysProcAttr()
	cmd.Dir = workDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = os.Environ()
	if req.RuntimeToken != "" {
		cmd.Env = append(cmd.Env, "GENESIS_RUNTIME_TOKEN="+req.RuntimeToken)
	}
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return nil, err
	}
	proc := &localSidecarProcess{cmd: cmd, waitDone: make(chan error, 1)}
	go func() {
		proc.waitDone <- cmd.Wait()
		_ = logFile.Close()
	}()
	return proc, nil
}

func genesisdCommand(req sidecarLaunchRequest) (string, []string, string, error) {
	if req.GenesisdPath != "" {
		return req.GenesisdPath, nil, req.WorkDir, nil
	}
	if envPath := strings.TrimSpace(os.Getenv("GENESIS_DESKTOP_GENESISD_PATH")); envPath != "" {
		return envPath, nil, req.WorkDir, nil
	}
	if executable, err := desktopExecutablePath(); err == nil {
		runtimeDir := filepath.Join(filepath.Dir(executable), "kernel")
		if candidate := filepath.Join(runtimeDir, "genesisd.exe"); fileExists(candidate) {
			return candidate, nil, runtimeDir, nil
		}
	}
	root := req.WorkDir
	if root == "" {
		var err error
		root, err = findRepoRoot()
		if err != nil {
			return "", nil, "", err
		}
	}
	if candidate := filepath.Join(root, "genesisd.exe"); fileExists(candidate) {
		return candidate, nil, root, nil
	}
	if candidate := filepath.Join(root, "build", "bin", "genesisd.exe"); fileExists(candidate) {
		return candidate, nil, root, nil
	}
	return "go", []string{"run", "./cmd/genesisd"}, root, nil
}

func findRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := wd; ; dir = filepath.Dir(dir) {
		if dirExists(filepath.Join(dir, "cmd", "genesisd")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not locate repository root from %s", wd)
		}
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func probeKernelReadiness(ctx context.Context, baseURL, token string) sidecarReadinessResult {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		result := probeKernelReadinessOnce(ctx, baseURL, token)
		if result.Ready {
			return result
		}
		select {
		case <-ctx.Done():
			if result.Reason != "" {
				return result
			}
			return sidecarReadinessResult{Reason: sidecarReadinessProbeFailed}
		case <-ticker.C:
		}
	}
}

func probeKernelReadinessOnce(ctx context.Context, baseURL, token string) sidecarReadinessResult {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/ready", nil)
	if err != nil {
		return sidecarReadinessResult{Reason: sidecarReadinessProbeFailed}
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return sidecarReadinessResult{Reason: sidecarReadinessProbeFailed}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return sidecarReadinessResult{Reason: sidecarKernelNotReady}
	}
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return sidecarReadinessResult{Reason: sidecarKernelNotReady}
	}
	if asString(payload["readiness"]) == "ready" || asString(payload["status"]) == "ok" {
		return sidecarReadinessResult{Ready: true}
	}
	reason := asString(payload["reason"])
	if reason == "" {
		reason = sidecarKernelNotReady
	}
	return sidecarReadinessResult{Reason: reason}
}

func probeKernelEndpointOccupied(ctx context.Context, baseURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Host == "" {
		return false
	}
	host := parsed.Host
	if _, _, err := net.SplitHostPort(host); err != nil {
		port := "80"
		if parsed.Scheme == "https" {
			port = "443"
		}
		host = net.JoinHostPort(parsed.Hostname(), port)
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", host)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func asString(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func killLocalProcessTree(ctx context.Context, cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if runtime.GOOS == "windows" {
		kill := exec.CommandContext(ctx, "taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid))
		kill.SysProcAttr = noConsoleSysProcAttr()
		if err := kill.Run(); err == nil {
			return nil
		}
	}
	return cmd.Process.Kill()
}
