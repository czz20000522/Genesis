package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	serviceOwnershipUnowned          = "unowned"
	localModelDisabled               = "local_model_disabled"
	localModelConfigInvalid          = "local_model_config_invalid"
	localModelEndpointAlreadyServing = "local_model_endpoint_already_serving"
	localModelStartFailed            = "local_model_start_failed"
	localModelReadinessProbeFailed   = "local_model_readiness_probe_failed"
	localModelStopFailed             = "local_model_stop_failed"
	localModelStopped                = "local_model_stopped"
	localModelExited                 = "local_model_exited"
)

type localModelRuntimeConfig struct {
	Enabled          bool   `json:"enabled"`
	WSLDistribution  string `json:"wsl_distribution"`
	ServerPath       string `json:"server_path"`
	ModelPath        string `json:"model_path"`
	Host             string `json:"host"`
	Port             int    `json:"port"`
	HealthURL        string `json:"health_url"`
	ContextTokens    int    `json:"context_tokens"`
	GPUOffloadLayers string `json:"gpu_offload_layers"`
	CacheTypeK       string `json:"cache_type_k"`
	CacheTypeV       string `json:"cache_type_v"`
	Parallel         int    `json:"parallel"`
}

type localModelProcess interface {
	sidecarProcess
	Done() <-chan struct{}
}

type localModelLaunchRequest struct {
	Runtime localModelRuntimeConfig
	LogPath string
	PIDPath string
}

type localModelLauncher func(context.Context, localModelLaunchRequest) (localModelProcess, error)

type localModelReadinessProbe func(context.Context, string) sidecarReadinessResult

type LocalModelSupervisorConfig struct {
	Runtime          localModelRuntimeConfig
	LogDir           string
	ReadinessTimeout time.Duration

	launcher              localModelLauncher
	readinessProbe        localModelReadinessProbe
	endpointOccupiedProbe localModelReadinessProbe
}

type LocalModelSupervisor struct {
	mu       sync.Mutex
	cfg      LocalModelSupervisorConfig
	status   SidecarStatus
	process  localModelProcess
	stopping bool
}

func NewLocalModelSupervisor(cfg LocalModelSupervisorConfig) *LocalModelSupervisor {
	if cfg.ReadinessTimeout <= 0 {
		cfg.ReadinessTimeout = 90 * time.Second
	}
	if cfg.launcher == nil {
		cfg.launcher = launchWSLLocalModel
	}
	if cfg.readinessProbe == nil {
		cfg.readinessProbe = probeLocalModelReadiness
	}
	if cfg.endpointOccupiedProbe == nil {
		cfg.endpointOccupiedProbe = probeLocalModelReadinessOnce
	}
	supervisor := &LocalModelSupervisor{cfg: cfg}
	supervisor.status = supervisor.initialStatus()
	return supervisor
}

func (s *LocalModelSupervisor) Status() SidecarStatus {
	if s == nil {
		return SidecarStatus{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func (s *LocalModelSupervisor) Start(ctx context.Context) SidecarStatus {
	if s == nil {
		return SidecarStatus{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.process != nil {
		return s.status
	}
	if !s.cfg.Runtime.Enabled {
		s.status = s.initialStatus()
		return s.status
	}
	if err := validateLocalModelRuntime(s.cfg.Runtime); err != nil {
		s.status = s.unownedStatus("not_ready", localModelConfigInvalid)
		return s.status
	}
	endpointCtx, cancelEndpointProbe := context.WithTimeout(ctx, time.Second)
	endpoint := s.cfg.endpointOccupiedProbe(endpointCtx, s.cfg.Runtime.HealthURL)
	cancelEndpointProbe()
	if endpoint.Ready {
		s.status = s.unownedStatus("ready", localModelEndpointAlreadyServing)
		return s.status
	}
	logPath, err := s.prepareLogPath()
	if err != nil {
		s.status = s.unownedStatus("not_ready", localModelStartFailed)
		return s.status
	}
	startedAt := time.Now().UTC().Format(time.RFC3339Nano)
	process, err := s.cfg.launcher(ctx, localModelLaunchRequest{Runtime: s.cfg.Runtime, LogPath: logPath, PIDPath: fmt.Sprintf("/tmp/genesis-local-model-%d.pid", time.Now().UnixNano())})
	if err != nil {
		s.status = s.unownedStatus("not_ready", localModelStartFailed)
		return s.status
	}
	s.process = process
	s.status = s.ownedStatus("not_ready", sidecarStarting, process.PID(), startedAt, logPath)
	go s.watch(process)

	probeCtx, cancel := context.WithTimeout(ctx, s.cfg.ReadinessTimeout)
	defer cancel()
	result := s.cfg.readinessProbe(probeCtx, s.cfg.Runtime.HealthURL)
	if result.Ready {
		s.status = s.ownedStatus("ready", "", process.PID(), startedAt, logPath)
		return s.status
	}
	s.status = s.ownedStatus("not_ready", localModelReadinessProbeFailed, process.PID(), startedAt, logPath)
	return s.status
}

func (s *LocalModelSupervisor) StopOwned(ctx context.Context) SidecarStatus {
	if s == nil {
		return SidecarStatus{}
	}
	s.mu.Lock()
	process := s.process
	if process == nil {
		status := s.status
		s.mu.Unlock()
		return status
	}
	if s.stopping {
		status := s.status
		s.mu.Unlock()
		return status
	}
	s.stopping = true
	s.mu.Unlock()
	if err := process.Stop(ctx); err != nil {
		s.mu.Lock()
		s.stopping = false
		if s.process == process {
			s.status = s.ownedStatus("not_ready", localModelStopFailed, process.PID(), s.status.StartedAt, s.status.LogPath)
		}
		status := s.status
		s.mu.Unlock()
		return status
	}
	s.mu.Lock()
	s.stopping = false
	if s.process == process {
		s.process = nil
		s.status = s.ownedStatus("not_ready", localModelStopped, 0, "", "")
	}
	status := s.status
	s.mu.Unlock()
	return status
}

func (s *LocalModelSupervisor) watch(process localModelProcess) {
	<-process.Done()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.process != process {
		return
	}
	s.process = nil
	s.status = s.ownedStatus("not_ready", localModelExited, 0, "", "")
}

func (s *LocalModelSupervisor) initialStatus() SidecarStatus {
	if !s.cfg.Runtime.Enabled {
		return s.unownedStatus("not_ready", localModelDisabled)
	}
	return s.unownedStatus("not_ready", localModelStopped)
}

func (s *LocalModelSupervisor) ownedStatus(readiness string, reason string, pid int, startedAt string, logPath string) SidecarStatus {
	return SidecarStatus{ServiceID: "local_model", Kind: "local_model", Ownership: serviceOwnershipOwned, Readiness: readiness, Reason: reason, PID: pid, StartedAt: startedAt, LogPath: logPath}
}

func (s *LocalModelSupervisor) unownedStatus(readiness string, reason string) SidecarStatus {
	return SidecarStatus{ServiceID: "local_model", Kind: "local_model", Ownership: serviceOwnershipUnowned, Readiness: readiness, Reason: reason}
}

func (s *LocalModelSupervisor) prepareLogPath() (string, error) {
	dir := s.cfg.LogDir
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "genesis-desktop", "local-model")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "llama-server.log"), nil
}

func validateLocalModelRuntime(cfg localModelRuntimeConfig) error {
	if !cfg.Enabled {
		return nil
	}
	if strings.TrimSpace(cfg.WSLDistribution) == "" || strings.TrimSpace(cfg.ServerPath) == "" || strings.TrimSpace(cfg.ModelPath) == "" || strings.TrimSpace(cfg.HealthURL) == "" {
		return errors.New("local model runtime is incomplete")
	}
	if cfg.Port <= 0 || cfg.ContextTokens <= 0 || cfg.Parallel <= 0 || strings.TrimSpace(cfg.GPUOffloadLayers) == "" || strings.TrimSpace(cfg.CacheTypeK) == "" || strings.TrimSpace(cfg.CacheTypeV) == "" {
		return errors.New("local model runtime values are invalid")
	}
	return nil
}

type wslLocalModelProcess struct {
	cmd          *exec.Cmd
	waitDone     chan struct{}
	stopMu       sync.Mutex
	distribution string
	pidPath      string
	serverPath   string
}

func (p *wslLocalModelProcess) PID() int {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

func (p *wslLocalModelProcess) Done() <-chan struct{} {
	if p == nil {
		return nil
	}
	return p.waitDone
}

func (p *wslLocalModelProcess) Stop(ctx context.Context) error {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	p.stopMu.Lock()
	defer p.stopMu.Unlock()
	select {
	case <-p.waitDone:
		return nil
	default:
	}
	_ = p.stopOwnedLinuxServer(ctx)
	if err := killLocalProcessTree(ctx, p.cmd); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.waitDone:
		return nil
	case <-time.After(3 * time.Second):
		return errors.New("local model process did not exit after kill")
	}
}

func launchWSLLocalModel(_ context.Context, req localModelLaunchRequest) (localModelProcess, error) {
	if runtime.GOOS != "windows" {
		return nil, fmt.Errorf("local model launcher requires Windows WSL")
	}
	logFile, err := os.OpenFile(req.LogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	args := wslLocalModelArgs(req)
	cmd := exec.Command("wsl.exe", args...)
	cmd.SysProcAttr = noConsoleSysProcAttr()
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return nil, err
	}
	process := &wslLocalModelProcess{cmd: cmd, waitDone: make(chan struct{}), distribution: req.Runtime.WSLDistribution, pidPath: req.PIDPath, serverPath: req.Runtime.ServerPath}
	go func() {
		_ = cmd.Wait()
		_ = logFile.Close()
		close(process.waitDone)
	}()
	return process, nil
}

func wslLocalModelArgs(req localModelLaunchRequest) []string {
	host := strings.TrimSpace(req.Runtime.Host)
	if host == "" {
		host = "0.0.0.0"
	}
	return []string{"-d", req.Runtime.WSLDistribution, "--exec", "/bin/sh", "-c", `pid_path="$1"; shift; printf '%s\n' "$$" > "$pid_path"; exec "$@"`, "genesis-local-model", req.PIDPath, req.Runtime.ServerPath,
		"-m", req.Runtime.ModelPath,
		"-c", strconv.Itoa(req.Runtime.ContextTokens),
		"-ngl", req.Runtime.GPUOffloadLayers,
		"--host", host,
		"--port", strconv.Itoa(req.Runtime.Port),
		"--cache-type-k", req.Runtime.CacheTypeK,
		"--cache-type-v", req.Runtime.CacheTypeV,
		"--parallel", strconv.Itoa(req.Runtime.Parallel),
	}
}

func (p *wslLocalModelProcess) stopOwnedLinuxServer(ctx context.Context) error {
	if p == nil || strings.TrimSpace(p.distribution) == "" || strings.TrimSpace(p.pidPath) == "" || strings.TrimSpace(p.serverPath) == "" {
		return nil
	}
	script := `pid="$(cat "$1" 2>/dev/null)"; case "$pid" in ''|*[!0-9]*) exit 0;; esac; command="$(tr '\000' ' ' < "/proc/$pid/cmdline" 2>/dev/null)"; case "$command" in *"$2"*) kill "$pid"; rm -f "$1";; esac`
	command := exec.CommandContext(ctx, "wsl.exe", "-d", p.distribution, "--exec", "/bin/sh", "-c", script, "genesis-local-model-stop", p.pidPath, p.serverPath)
	command.SysProcAttr = noConsoleSysProcAttr()
	return command.Run()
}

func probeLocalModelReadiness(ctx context.Context, healthURL string) sidecarReadinessResult {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		result := probeLocalModelReadinessOnce(ctx, healthURL)
		if result.Ready {
			return result
		}
		select {
		case <-ctx.Done():
			return result
		case <-ticker.C:
		}
	}
}

func probeLocalModelReadinessOnce(ctx context.Context, healthURL string) sidecarReadinessResult {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(healthURL), nil)
	if err != nil {
		return sidecarReadinessResult{Reason: localModelReadinessProbeFailed}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return sidecarReadinessResult{Reason: localModelReadinessProbeFailed}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return sidecarReadinessResult{Reason: localModelReadinessProbeFailed}
	}
	return sidecarReadinessResult{Ready: true}
}

func localModelRuntimeConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".genesis", "config", "desktop-local-model.json")
}

func loadLocalModelRuntimeConfig() localModelRuntimeConfig {
	path := localModelRuntimeConfigPath()
	if path == "" {
		return localModelRuntimeConfig{}
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return localModelRuntimeConfig{}
	}
	var cfg localModelRuntimeConfig
	if json.Unmarshal(payload, &cfg) != nil {
		return localModelRuntimeConfig{Enabled: true}
	}
	return cfg
}
