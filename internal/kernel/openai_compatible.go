package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type OpenAICompatibleConfig struct {
	BaseURL        string
	APIKey         string
	Model          string
	RequestTimeout time.Duration
	HTTPClient     *http.Client
}

type OpenAICompatibleProvider struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewOpenAICompatibleProvider(config OpenAICompatibleConfig) *OpenAICompatibleProvider {
	client := config.HTTPClient
	if client == nil {
		timeout := config.RequestTimeout
		if timeout <= 0 {
			timeout = 60 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	}
	return &OpenAICompatibleProvider{
		baseURL:    strings.TrimRight(strings.TrimSpace(config.BaseURL), "/"),
		apiKey:     strings.TrimSpace(config.APIKey),
		model:      strings.TrimSpace(config.Model),
		httpClient: client,
	}
}

func (p *OpenAICompatibleProvider) Name() string {
	return "openai-compatible"
}

func (p *OpenAICompatibleProvider) Ready() ProviderStatus {
	if p.baseURL == "" {
		return ProviderStatus{Name: p.Name(), Status: "blocked", Reason: "provider_base_url_missing"}
	}
	if _, err := url.ParseRequestURI(p.baseURL); err != nil {
		return ProviderStatus{Name: p.Name(), Status: "blocked", Reason: "provider_base_url_invalid"}
	}
	if p.apiKey == "" {
		return ProviderStatus{Name: p.Name(), Status: "blocked", Reason: "provider_api_key_missing"}
	}
	if p.model == "" {
		return ProviderStatus{Name: p.Name(), Status: "blocked", Reason: "provider_model_missing"}
	}
	return ProviderStatus{Name: p.Name(), Status: "ok"}
}

func (p *OpenAICompatibleProvider) Complete(ctx context.Context, req ModelRequest) (ModelResponse, error) {
	if status := p.Ready(); status.Status != "ok" {
		return ModelResponse{}, fmt.Errorf("%w: %s", ErrProviderUnavailable, status.Reason)
	}

	payload := chatCompletionRequest{
		Model: p.model,
		Messages: []chatMessage{
			{Role: "user", Content: modelUserText(req.InputItems)},
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return ModelResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(encoded))
	if err != nil {
		return ModelResponse{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return ModelResponse{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return ModelResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ModelResponse{}, fmt.Errorf("provider returned status %d", resp.StatusCode)
	}
	var decoded chatCompletionResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return ModelResponse{}, err
	}
	if len(decoded.Choices) == 0 || strings.TrimSpace(decoded.Choices[0].Message.Content) == "" {
		return ModelResponse{}, errors.New("provider returned no assistant content")
	}
	model := decoded.Model
	if model == "" {
		model = p.model
	}
	return ModelResponse{
		Text:  decoded.Choices[0].Message.Content,
		Model: model,
	}, nil
}

func modelUserText(items []InputItem) string {
	var parts []string
	for _, item := range items {
		if item.Type == "text" && item.Text != "" {
			parts = append(parts, item.Text)
		}
	}
	return strings.Join(parts, "\n")
}

type chatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Model   string       `json:"model"`
	Choices []chatChoice `json:"choices"`
}

type chatChoice struct {
	Message chatMessage `json:"message"`
}
