package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultKernelBaseURL = "http://127.0.0.1:8765"
	sidecarNotWired      = "sidecar_not_wired"
)

type App struct {
	ctx    context.Context
	config DesktopConfig
	client *KernelHTTPClient
}

type DesktopConfig struct {
	KernelBaseURL string        `json:"kernel_base_url"`
	RuntimeToken  string        `json:"runtime_token,omitempty"`
	Sidecar       SidecarStatus `json:"sidecar"`
}

type SidecarStatus struct {
	Readiness string `json:"readiness"`
	Reason    string `json:"reason,omitempty"`
}

func NewApp() *App {
	cfg := loadDesktopConfig()
	return &App{
		config: cfg,
		client: NewKernelHTTPClient(cfg.KernelBaseURL, cfg.RuntimeToken, nil),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) DesktopConfig() DesktopConfig {
	return a.config
}

func (a *App) KernelReady() (map[string]any, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return a.client.Get(ctx, "/ready", false)
}

func loadDesktopConfig() DesktopConfig {
	baseURL := strings.TrimSpace(os.Getenv("GENESIS_KERNEL_BASE_URL"))
	if baseURL == "" {
		baseURL = defaultKernelBaseURL
	}
	return DesktopConfig{
		KernelBaseURL: strings.TrimRight(baseURL, "/"),
		RuntimeToken:  strings.TrimSpace(os.Getenv("GENESIS_RUNTIME_TOKEN")),
		Sidecar: SidecarStatus{
			Readiness: "not_ready",
			Reason:    sidecarNotWired,
		},
	}
}

type KernelHTTPClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func NewKernelHTTPClient(baseURL string, token string, client *http.Client) *KernelHTTPClient {
	if client == nil {
		client = http.DefaultClient
	}
	return &KernelHTTPClient{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(token),
		client:  client,
	}
}

func (c *KernelHTTPClient) Get(ctx context.Context, path string, auth bool) (map[string]any, error) {
	return c.RequestJSON(ctx, http.MethodGet, path, auth)
}

func (c *KernelHTTPClient) RequestJSON(ctx context.Context, method string, path string, auth bool) (map[string]any, error) {
	if c.baseURL == "" {
		return nil, errors.New("kernel base URL is required")
	}
	u, err := url.JoinPath(c.baseURL, strings.TrimLeft(path, "/"))
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return nil, err
	}
	if auth && c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("kernel HTTP %d", resp.StatusCode)
	}
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload, nil
}
