package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
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

type MaterialBridgeRequest struct {
	SessionID string `json:"session_id"`
	Purpose   string `json:"purpose"`
	FilePath  string `json:"file_path"`
}

type MaterialFileSelection struct {
	FilePath string `json:"file_path"`
	Filename string `json:"filename"`
}

func (a *App) Ready() (map[string]any, error) {
	ctx, cancel := a.requestContext()
	defer cancel()
	return a.client.Get(ctx, "/ready", false)
}

func (a *App) SubmitTurn(sessionID string, text string, idempotencyKey string) (map[string]any, error) {
	ctx, cancel := a.requestContext()
	defer cancel()
	body, _ := json.Marshal(map[string]any{
		"session_id":      strings.TrimSpace(sessionID),
		"idempotency_key": strings.TrimSpace(idempotencyKey),
		"input_items":     []map[string]string{{"type": "text", "text": text}},
	})
	return a.client.RequestJSON(ctx, http.MethodPost, "/turn", true, body)
}

func (a *App) ReadTimeline(sessionID string) (map[string]any, error) {
	ctx, cancel := a.requestContext()
	defer cancel()
	return a.client.Get(ctx, "/sessions/"+url.PathEscape(strings.TrimSpace(sessionID))+"/timeline", true)
}

func (a *App) ReadTimelineDetail(sessionID string, detailRef string) (map[string]any, error) {
	ctx, cancel := a.requestContext()
	defer cancel()
	return a.client.Get(ctx, "/sessions/"+url.PathEscape(strings.TrimSpace(sessionID))+"/timeline/details/"+url.PathEscape(strings.TrimSpace(detailRef)), true)
}

func (a *App) ReadSession(sessionID string) (map[string]any, error) {
	ctx, cancel := a.requestContext()
	defer cancel()
	return a.client.Get(ctx, "/sessions/"+url.PathEscape(strings.TrimSpace(sessionID)), true)
}

func (a *App) DecideApproval(approvalID string, decision string, reason string) (map[string]any, error) {
	ctx, cancel := a.requestContext()
	defer cancel()
	body, _ := json.Marshal(map[string]any{
		"decision":              strings.TrimSpace(decision),
		"decision_authority":    "desktop:operator",
		"decision_reason":       strings.TrimSpace(reason),
		"decision_evidence_ref": "approval:desktop-operator",
	})
	return a.client.RequestJSON(ctx, http.MethodPost, "/approvals/"+url.PathEscape(strings.TrimSpace(approvalID))+"/decision", true, body)
}

func (a *App) PickMaterialFile() (*MaterialFileSelection, error) {
	if a.ctx == nil {
		return nil, errors.New("desktop window is not ready")
	}
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "选择资料",
		Filters: []wailsruntime.FileFilter{{
			DisplayName: "Archives",
			Pattern:     "*.zip",
		}},
	})
	if err != nil || strings.TrimSpace(path) == "" {
		return nil, err
	}
	return &MaterialFileSelection{FilePath: path, Filename: filepath.Base(path)}, nil
}

func (a *App) UploadMaterial(req MaterialBridgeRequest) (map[string]any, error) {
	ctx, cancel := a.requestContext()
	defer cancel()
	sessionID := strings.TrimSpace(req.SessionID)
	filePath := strings.TrimSpace(req.FilePath)
	if sessionID == "" {
		return nil, errors.New("session_id is required")
	}
	if filePath == "" {
		return nil, errors.New("file_path is required")
	}
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return a.client.PostMultipartFile(ctx, "/materials/upload", true, sessionID, strings.TrimSpace(req.Purpose), filepath.Base(filePath), file)
}

func (a *App) EnableSessionDebug(sessionID string) (map[string]any, error) {
	ctx, cancel := a.requestContext()
	defer cancel()
	return a.client.RequestJSON(ctx, http.MethodPost, "/sessions/"+url.PathEscape(strings.TrimSpace(sessionID))+"/debug/enable", true, json.RawMessage(`{}`))
}

func (a *App) ExportSessionDebug(sessionID string) (map[string]any, error) {
	ctx, cancel := a.requestContext()
	defer cancel()
	return a.client.Get(ctx, "/sessions/"+url.PathEscape(strings.TrimSpace(sessionID))+"/debug", true)
}

func (a *App) CompactSessionContext(sessionID string) (map[string]any, error) {
	ctx, cancel := a.requestContext()
	defer cancel()
	return a.client.RequestJSON(ctx, http.MethodPost, "/sessions/"+url.PathEscape(strings.TrimSpace(sessionID))+"/context/compact", true, json.RawMessage(`{}`))
}

func (a *App) requestContext() (context.Context, context.CancelFunc) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, 30*time.Second)
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
	return c.RequestJSON(ctx, http.MethodGet, path, auth, nil)
}

func (c *KernelHTTPClient) RequestJSON(ctx context.Context, method string, path string, auth bool, body json.RawMessage) (map[string]any, error) {
	var reader io.Reader
	if len(body) > 0 && string(body) != "null" {
		reader = strings.NewReader(string(body))
	}
	return c.do(ctx, method, path, auth, "application/json", reader)
}

func (c *KernelHTTPClient) PostMultipart(ctx context.Context, path string, auth bool, contentType string, body io.Reader) (map[string]any, error) {
	return c.do(ctx, http.MethodPost, path, auth, contentType, body)
}

func (c *KernelHTTPClient) PostMultipartFile(ctx context.Context, path string, auth bool, sessionID string, purpose string, filename string, file io.Reader) (map[string]any, error) {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)
	go func() {
		var err error
		defer func() {
			if err != nil {
				_ = pw.CloseWithError(err)
				return
			}
			_ = pw.Close()
		}()
		if err = writer.WriteField("session_id", sessionID); err != nil {
			return
		}
		if err = writer.WriteField("purpose", purpose); err != nil {
			return
		}
		var part io.Writer
		if part, err = writer.CreateFormFile("file", filename); err != nil {
			return
		}
		if _, err = io.Copy(part, file); err != nil {
			return
		}
		err = writer.Close()
	}()
	return c.PostMultipart(ctx, path, auth, writer.FormDataContentType(), pr)
}

func (c *KernelHTTPClient) do(ctx context.Context, method string, path string, auth bool, contentType string, body io.Reader) (map[string]any, error) {
	if c.baseURL == "" {
		return nil, errors.New("kernel base URL is required")
	}
	u, err := url.JoinPath(c.baseURL, strings.TrimLeft(path, "/"))
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, err
	}
	if auth && c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if body != nil && contentType != "" {
		req.Header.Set("Content-Type", contentType)
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
