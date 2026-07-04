package main

import (
	"bufio"
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
)

type App struct {
	ctx        context.Context
	config     DesktopConfig
	client     *KernelHTTPClient
	supervisor *LocalServiceSupervisor
}

type DesktopConfig struct {
	KernelBaseURL string        `json:"kernel_base_url"`
	RuntimeToken  string        `json:"runtime_token,omitempty"`
	Sidecar       SidecarStatus `json:"sidecar"`
}

type SidecarStatus struct {
	ServiceID string `json:"service_id"`
	Kind      string `json:"kind"`
	Ownership string `json:"ownership"`
	Readiness string `json:"readiness"`
	Reason    string `json:"reason,omitempty"`
	PID       int    `json:"pid,omitempty"`
	StartedAt string `json:"started_at,omitempty"`
	LogPath   string `json:"log_path,omitempty"`
}

func NewApp() *App {
	cfg := loadDesktopConfig()
	supervisor := NewLocalServiceSupervisor(LocalServiceSupervisorConfig{
		KernelBaseURL: cfg.KernelBaseURL,
		RuntimeToken:  cfg.RuntimeToken,
		External:      cfg.Sidecar.Ownership == serviceOwnershipExternal,
		GenesisdPath:  strings.TrimSpace(os.Getenv("GENESIS_DESKTOP_GENESISD_PATH")),
	})
	return &App{
		config:     cfg,
		client:     NewKernelHTTPClient(cfg.KernelBaseURL, cfg.RuntimeToken, nil),
		supervisor: supervisor,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if a.supervisor != nil {
		a.config.Sidecar = a.supervisor.StartKernel(ctx)
	}
}

func (a *App) shutdown(ctx context.Context) {
	if a.supervisor != nil {
		a.config.Sidecar = a.supervisor.StopOwned(ctx)
	}
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

func (a *App) ListSessions() (map[string]any, error) {
	ctx, cancel := a.requestContext()
	defer cancel()
	return a.client.Get(ctx, "/sessions", true)
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

func (a *App) SubmitTurnStream(sessionID string, text string, idempotencyKey string) (map[string]any, error) {
	ctx, cancel := a.requestContext()
	defer cancel()
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	body, _ := json.Marshal(map[string]any{
		"session_id":      strings.TrimSpace(sessionID),
		"idempotency_key": idempotencyKey,
		"input_items":     []map[string]string{{"type": "text", "text": text}},
	})
	eventName := desktopTurnStreamEventName(idempotencyKey)
	final, err := a.client.StreamJSONLines(ctx, "/turn/stream", true, body, func(payload map[string]any) error {
		if a.ctx != nil {
			wailsruntime.EventsEmit(a.ctx, eventName, payload)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"stream_id": eventName,
		"response":  final,
	}, nil
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
	external := baseURL != ""
	if baseURL == "" {
		baseURL = defaultKernelBaseURL
	}
	token := strings.TrimSpace(os.Getenv("GENESIS_RUNTIME_TOKEN"))
	supervisor := NewLocalServiceSupervisor(LocalServiceSupervisorConfig{
		KernelBaseURL: baseURL,
		RuntimeToken:  token,
		External:      external,
		GenesisdPath:  strings.TrimSpace(os.Getenv("GENESIS_DESKTOP_GENESISD_PATH")),
	})
	return DesktopConfig{
		KernelBaseURL: strings.TrimRight(baseURL, "/"),
		RuntimeToken:  token,
		Sidecar:       supervisor.KernelStatus(),
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

func (c *KernelHTTPClient) StreamJSONLines(ctx context.Context, path string, auth bool, body json.RawMessage, emit func(map[string]any) error) (map[string]any, error) {
	if c.baseURL == "" {
		return nil, errors.New("kernel base URL is required")
	}
	u, err := url.JoinPath(c.baseURL, strings.TrimLeft(path, "/"))
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	if auth && c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/x-ndjson")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("kernel HTTP %d", resp.StatusCode)
	}
	var final map[string]any
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			return nil, err
		}
		if emit != nil {
			if err := emit(payload); err != nil {
				return nil, err
			}
		}
		switch payload["type"] {
		case "turn_completed":
			if response, ok := payload["response"].(map[string]any); ok {
				final = response
			}
		case "turn_failed":
			return nil, desktopTurnStreamFailure(payload)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if final == nil {
		return nil, errors.New("stream ended before turn_completed")
	}
	return final, nil
}

func (c *KernelHTTPClient) PostMultipart(ctx context.Context, path string, auth bool, contentType string, body io.Reader) (map[string]any, error) {
	return c.do(ctx, http.MethodPost, path, auth, contentType, body)
}

func desktopTurnStreamEventName(idempotencyKey string) string {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = "anonymous"
	}
	return "genesis:turn-stream:" + idempotencyKey
}

func desktopTurnStreamFailure(payload map[string]any) error {
	errPayload, _ := payload["error"].(map[string]any)
	code, _ := errPayload["code"].(string)
	message, _ := errPayload["message"].(string)
	text := strings.TrimSpace(strings.Join([]string{strings.TrimSpace(code), strings.TrimSpace(message)}, ": "))
	text = strings.Trim(text, ": ")
	if text == "" {
		text = "turn failed"
	}
	return errors.New(text)
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
