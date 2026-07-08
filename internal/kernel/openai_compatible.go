package kernel

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type OpenAICompatibleConfig struct {
	BaseURL        string
	APIKey         string
	Model          string
	Adapter        ProviderAdapterBinding
	RequestTimeout time.Duration
	HTTPClient     *http.Client
}

type OpenAICompatibleProvider struct {
	baseURL    string
	apiKey     string
	model      string
	adapter    ProviderAdapterBinding
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
		adapter:    config.Adapter,
		httpClient: client,
	}
}

func (p *OpenAICompatibleProvider) Name() string {
	return "openai-compatible"
}

func (p *OpenAICompatibleProvider) Ready() ProviderStatus {
	if p.baseURL == "" {
		return ProviderStatus{Name: p.Name(), Readiness: ReadinessNotReady, ReadinessReason: "provider_base_url_missing"}
	}
	if _, err := url.ParseRequestURI(p.baseURL); err != nil {
		return ProviderStatus{Name: p.Name(), Readiness: ReadinessNotReady, ReadinessReason: "provider_base_url_invalid"}
	}
	if p.apiKey == "" {
		return ProviderStatus{Name: p.Name(), Readiness: ReadinessNotReady, ReadinessReason: "provider_api_key_missing"}
	}
	if p.model == "" {
		return ProviderStatus{Name: p.Name(), Readiness: ReadinessNotReady, ReadinessReason: "provider_model_missing"}
	}
	return ProviderStatus{Name: p.Name(), Readiness: ReadinessReady}
}

func (p *OpenAICompatibleProvider) Complete(ctx context.Context, req ModelRequest) (ModelResponse, error) {
	if status := p.Ready(); status.Readiness != ReadinessReady {
		return ModelResponse{}, fmt.Errorf("%w: %s", ErrProviderUnavailable, status.ReadinessReason)
	}

	payload := chatCompletionRequest{
		Model:    p.model,
		Messages: chatMessagesFromModelRequest(req),
		Tools:    chatToolsFromManifest(req.ToolManifest),
	}
	if len(payload.Tools) > 0 {
		payload.ToolChoice = "auto"
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
		if ctx.Err() != nil {
			return ModelResponse{}, err
		}
		return ModelResponse{}, newProviderTransportError(err)
	}
	defer resp.Body.Close()

	retryAfter := parseProviderRetryAfter(resp)
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return ModelResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ModelResponse{}, newProviderStatusError(resp.StatusCode, string(body), retryAfter)
	}
	var decoded chatCompletionResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return ModelResponse{}, err
	}
	if len(decoded.Choices) == 0 {
		return ModelResponse{}, errors.New("provider returned no choices")
	}
	message := decoded.Choices[0].Message
	model := decoded.Model
	if model == "" {
		model = p.model
	}
	if len(message.ToolCalls) > 0 {
		if err := p.handleVendorHiddenReasoning(message); err != nil {
			return ModelResponse{}, err
		}
		calls, err := modelToolCallsFromChat(message.ToolCalls)
		if err != nil {
			return ModelResponse{}, err
		}
		return ModelResponse{
			Model:     model,
			ToolCalls: calls,
			Usage:     tokenUsageFromChatUsage(decoded.Usage),
		}, nil
	}
	if err := p.handleVendorHiddenReasoning(message); err != nil {
		return ModelResponse{}, err
	}
	if strings.TrimSpace(message.Content) == "" {
		return ModelResponse{}, newProviderVisibleFinalRequiredError()
	}
	return ModelResponse{
		Text:  message.Content,
		Model: model,
		Usage: tokenUsageFromChatUsage(decoded.Usage),
	}, nil
}

func (p *OpenAICompatibleProvider) StreamComplete(ctx context.Context, req ModelRequest, emit func(ModelStreamDelta) error) (ModelResponse, error) {
	if status := p.Ready(); status.Readiness != ReadinessReady {
		return ModelResponse{}, fmt.Errorf("%w: %s", ErrProviderUnavailable, status.ReadinessReason)
	}

	payload := chatCompletionRequest{
		Model:         p.model,
		Messages:      chatMessagesFromModelRequest(req),
		Tools:         chatToolsFromManifest(req.ToolManifest),
		Stream:        true,
		StreamOptions: &chatCompletionStreamOptions{IncludeUsage: true},
	}
	if len(payload.Tools) > 0 {
		payload.ToolChoice = "auto"
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
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return ModelResponse{}, err
		}
		return ModelResponse{}, newProviderTransportError(err)
	}
	defer resp.Body.Close()

	retryAfter := parseProviderRetryAfter(resp)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
		if readErr != nil {
			return ModelResponse{}, readErr
		}
		return ModelResponse{}, newProviderStatusError(resp.StatusCode, string(body), retryAfter)
	}
	return p.decodeChatStream(resp.Body, emit)
}

func (p *OpenAICompatibleProvider) decodeChatStream(body io.Reader, emit func(ModelStreamDelta) error) (ModelResponse, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var content strings.Builder
	model := p.model
	toolCalls := map[int]*chatToolCall{}
	var usage *TokenUsage
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk chatCompletionStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return ModelResponse{}, err
		}
		if strings.TrimSpace(chunk.Model) != "" {
			model = chunk.Model
		}
		if chunk.Usage != nil {
			usage = tokenUsageFromChatUsage(chunk.Usage)
		}
		for _, choice := range chunk.Choices {
			if err := p.handleVendorHiddenReasoning(choice.Delta); err != nil {
				return ModelResponse{}, err
			}
			if choice.Delta.Content != "" {
				content.WriteString(choice.Delta.Content)
				if emit != nil {
					if err := emit(ModelStreamDelta{Text: choice.Delta.Content}); err != nil {
						return ModelResponse{}, err
					}
				}
			}
			mergeStreamToolCalls(toolCalls, choice.Delta.ToolCalls)
		}
	}
	if err := scanner.Err(); err != nil {
		return ModelResponse{}, newProviderTransportError(err)
	}
	if len(toolCalls) > 0 {
		calls, err := modelToolCallsFromChat(orderedStreamToolCalls(toolCalls))
		if err != nil {
			return ModelResponse{}, err
		}
		return ModelResponse{Model: model, ToolCalls: calls, Usage: usage}, nil
	}
	text := content.String()
	if strings.TrimSpace(text) == "" {
		return ModelResponse{}, newProviderVisibleFinalRequiredError()
	}
	return ModelResponse{Text: text, Model: model, Usage: usage}, nil
}

func (p *OpenAICompatibleProvider) handleVendorHiddenReasoning(message chatMessage) error {
	if strings.TrimSpace(message.ReasoningContent) == "" {
		return nil
	}
	if p.adapter.allowsHiddenReasoningDiscard() {
		return nil
	}
	return newProviderVendorFieldUnsupportedError()
}

func parseProviderRetryAfter(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}
	value := strings.TrimSpace(resp.Header.Get("Retry-After"))
	if value == "" {
		return 0
	}
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds < 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func chatMessagesFromModelRequest(req ModelRequest) []chatMessage {
	messages := []chatMessage{
		{Role: "user", Content: modelUserText(req.InputItems)},
	}
	for _, round := range req.ToolRounds {
		if len(round.Calls) == 0 {
			continue
		}
		messages = append(messages, chatMessage{
			Role:      "assistant",
			ToolCalls: chatToolCallsFromModel(round.Calls),
		})
		for _, result := range round.Results {
			messages = append(messages, chatMessage{
				Role:       "tool",
				ToolCallID: providerToolResultID(result),
				Content:    result.Content,
			})
		}
	}
	return messages
}

func modelUserText(items []ModelInputItem) string {
	var parts []string
	for _, item := range items {
		if item.Text != "" {
			parts = append(parts, item.Text)
		}
	}
	return strings.Join(parts, "\n")
}

type chatCompletionRequest struct {
	Model         string                       `json:"model"`
	Messages      []chatMessage                `json:"messages"`
	Tools         []chatTool                   `json:"tools,omitempty"`
	ToolChoice    string                       `json:"tool_choice,omitempty"`
	Stream        bool                         `json:"stream,omitempty"`
	StreamOptions *chatCompletionStreamOptions `json:"stream_options,omitempty"`
}

type chatCompletionStreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

type chatMessage struct {
	Role             string         `json:"role"`
	Content          string         `json:"content,omitempty"`
	ToolCallID       string         `json:"tool_call_id,omitempty"`
	ToolCalls        []chatToolCall `json:"tool_calls,omitempty"`
	ReasoningContent string         `json:"reasoning_content,omitempty"`
}

type chatCompletionResponse struct {
	Model   string       `json:"model"`
	Choices []chatChoice `json:"choices"`
	Usage   *chatUsage   `json:"usage,omitempty"`
}

type chatCompletionStreamChunk struct {
	Model   string             `json:"model"`
	Choices []chatStreamChoice `json:"choices"`
	Usage   *chatUsage         `json:"usage,omitempty"`
}

type chatStreamChoice struct {
	Delta chatMessage `json:"delta"`
}

type chatChoice struct {
	Message chatMessage `json:"message"`
}

type chatTool struct {
	Type     string           `json:"type"`
	Function chatToolFunction `json:"function"`
}

type chatToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type chatToolCall struct {
	Index    int                  `json:"index,omitempty"`
	ID       string               `json:"id"`
	Type     string               `json:"type"`
	Function chatToolCallFunction `json:"function"`
}

type chatToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatUsage struct {
	PromptTokens          int                 `json:"prompt_tokens,omitempty"`
	CompletionTokens      int                 `json:"completion_tokens,omitempty"`
	TotalTokens           int                 `json:"total_tokens,omitempty"`
	InputTokens           int                 `json:"input_tokens,omitempty"`
	OutputTokens          int                 `json:"output_tokens,omitempty"`
	PromptCacheHitTokens  int                 `json:"prompt_cache_hit_tokens,omitempty"`
	PromptCacheMissTokens int                 `json:"prompt_cache_miss_tokens,omitempty"`
	PromptTokensDetails   *promptTokenDetails `json:"prompt_tokens_details,omitempty"`
}

type promptTokenDetails struct {
	CachedTokens int `json:"cached_tokens,omitempty"`
}

func tokenUsageFromChatUsage(usage *chatUsage) *TokenUsage {
	if usage == nil {
		return nil
	}
	inputTokens := usage.PromptTokens
	if inputTokens == 0 {
		inputTokens = usage.InputTokens
	}
	outputTokens := usage.CompletionTokens
	if outputTokens == 0 {
		outputTokens = usage.OutputTokens
	}
	totalTokens := usage.TotalTokens
	if totalTokens == 0 && (inputTokens != 0 || outputTokens != 0) {
		totalTokens = inputTokens + outputTokens
	}
	cacheHitTokens := usage.PromptCacheHitTokens
	if cacheHitTokens == 0 && usage.PromptTokensDetails != nil {
		cacheHitTokens = usage.PromptTokensDetails.CachedTokens
	}
	cacheMissTokens := usage.PromptCacheMissTokens
	if cacheMissTokens == 0 && cacheHitTokens > 0 && inputTokens > cacheHitTokens {
		cacheMissTokens = inputTokens - cacheHitTokens
	}
	return &TokenUsage{
		InputTokens:     inputTokens,
		OutputTokens:    outputTokens,
		TotalTokens:     totalTokens,
		CacheHitTokens:  cacheHitTokens,
		CacheMissTokens: cacheMissTokens,
	}
}

func chatToolsFromManifest(tools []ToolSpec) []chatTool {
	converted := make([]chatTool, 0, len(tools))
	for _, tool := range tools {
		name := strings.TrimSpace(tool.Name)
		if name == "" {
			continue
		}
		converted = append(converted, chatTool{
			Type: "function",
			Function: chatToolFunction{
				Name:        name,
				Description: strings.TrimSpace(tool.Description),
				Parameters:  tool.InputSchema,
			},
		})
	}
	return converted
}

func chatToolCallsFromModel(calls []ModelToolCall) []chatToolCall {
	converted := make([]chatToolCall, 0, len(calls))
	for _, call := range calls {
		args := strings.TrimSpace(string(call.Arguments))
		if args == "" {
			args = "{}"
		}
		converted = append(converted, chatToolCall{
			ID:   providerToolCallIDForReplay(call),
			Type: "function",
			Function: chatToolCallFunction{
				Name:      strings.TrimSpace(call.Name),
				Arguments: args,
			},
		})
	}
	return converted
}

func modelToolCallsFromChat(calls []chatToolCall) ([]ModelToolCall, error) {
	converted := make([]ModelToolCall, 0, len(calls))
	for _, call := range calls {
		if strings.TrimSpace(call.Type) != "function" {
			return nil, fmt.Errorf("unsupported provider tool call type %q", call.Type)
		}
		args := strings.TrimSpace(call.Function.Arguments)
		if args == "" {
			args = "{}"
		}
		raw := json.RawMessage(args)
		converted = append(converted, ModelToolCall{
			ToolCallID: strings.TrimSpace(call.ID),
			Name:       strings.TrimSpace(call.Function.Name),
			Arguments:  raw,
		})
	}
	return converted, nil
}

func mergeStreamToolCalls(calls map[int]*chatToolCall, deltas []chatToolCall) {
	for _, delta := range deltas {
		call := calls[delta.Index]
		if call == nil {
			call = &chatToolCall{}
			calls[delta.Index] = call
		}
		if delta.ID != "" {
			call.ID = delta.ID
		}
		if delta.Type != "" {
			call.Type = delta.Type
		}
		if delta.Function.Name != "" {
			call.Function.Name += delta.Function.Name
		}
		if delta.Function.Arguments != "" {
			call.Function.Arguments += delta.Function.Arguments
		}
	}
}

func orderedStreamToolCalls(calls map[int]*chatToolCall) []chatToolCall {
	indexes := make([]int, 0, len(calls))
	for index := range calls {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	ordered := make([]chatToolCall, 0, len(indexes))
	for _, index := range indexes {
		call := *calls[index]
		if strings.TrimSpace(call.ID) == "" {
			call.ID = fmt.Sprintf("call_%d", index)
		}
		if strings.TrimSpace(call.Type) == "" {
			call.Type = "function"
		}
		ordered = append(ordered, call)
	}
	return ordered
}

func providerToolCallIDForReplay(call ModelToolCall) string {
	return strings.TrimSpace(call.ToolCallID)
}

func providerToolResultID(result ModelToolResult) string {
	return strings.TrimSpace(result.ToolCallID)
}
