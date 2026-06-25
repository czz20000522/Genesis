package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	providerCommandProtocol              = "genesis.provider_command"
	providerCommandResponseKindFinal     = "final"
	providerCommandResponseKindToolCalls = "tool_calls"
	maxProviderCommandOutputBytes        = 64 * 1024
	defaultProviderCommandTimeout        = 60 * time.Second
)

var (
	ErrProviderCommandEnvRejected = errors.New("provider command environment rejected")
	providerCommandEnvNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

type ProviderCommandConfig struct {
	Command        string
	Args           []string
	Env            []string
	WorkingDir     string
	Model          string
	RequestTimeout time.Duration
}

type CommandProvider struct {
	command        string
	args           []string
	env            []string
	workingDir     string
	model          string
	requestTimeout time.Duration
}

func NewCommandProvider(config ProviderCommandConfig) *CommandProvider {
	timeout := config.RequestTimeout
	if timeout <= 0 {
		timeout = defaultProviderCommandTimeout
	}
	return &CommandProvider{
		command:        strings.TrimSpace(config.Command),
		args:           append([]string(nil), config.Args...),
		env:            append([]string(nil), config.Env...),
		workingDir:     strings.TrimSpace(config.WorkingDir),
		model:          strings.TrimSpace(config.Model),
		requestTimeout: timeout,
	}
}

func (p *CommandProvider) Name() string {
	return "provider_command"
}

func (p *CommandProvider) Ready() ProviderStatus {
	if err := validateProviderCommandEnv(p.env); err != nil {
		return ProviderStatus{Name: p.Name(), Status: "blocked", Reason: "provider_command_env_secret_rejected"}
	}
	if p.command == "" {
		return ProviderStatus{Name: p.Name(), Status: "blocked", Reason: "provider_command_missing"}
	}
	if !providerCommandExists(p.command) {
		return ProviderStatus{Name: p.Name(), Status: "blocked", Reason: "provider_command_not_found"}
	}
	if p.workingDir != "" {
		info, err := os.Stat(p.workingDir)
		if err != nil || !info.IsDir() {
			return ProviderStatus{Name: p.Name(), Status: "blocked", Reason: "provider_command_working_dir_invalid"}
		}
	}
	return ProviderStatus{Name: p.Name(), Status: "ok"}
}

func (p *CommandProvider) Complete(ctx context.Context, req ModelRequest) (ModelResponse, error) {
	if status := p.Ready(); status.Status != "ok" {
		return ModelResponse{}, fmt.Errorf("%w: %s", ErrProviderUnavailable, status.Reason)
	}
	requestPayload := providerCommandRequest{
		Protocol:     providerCommandProtocol,
		SessionID:    req.SessionID,
		TurnID:       req.TurnID,
		Model:        p.model,
		InputItems:   req.InputItems,
		ToolManifest: req.ToolManifest,
		ToolRounds:   providerCommandModelToolRounds(req.ToolRounds),
	}
	encoded, err := json.Marshal(requestPayload)
	if err != nil {
		return ModelResponse{}, err
	}

	runCtx, cancel := context.WithTimeout(ctx, p.requestTimeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, p.command, p.args...)
	cmd.Stdin = bytes.NewReader(encoded)
	cmd.Dir = p.workingDir
	cmd.Env = providerCommandEnvironment(p.env)
	var stdout cappedBuffer
	var stderr cappedBuffer
	stdout.limit = maxProviderCommandOutputBytes
	stderr.limit = maxProviderCommandOutputBytes
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			return ModelResponse{}, fmt.Errorf("provider command timed out: %w", runCtx.Err())
		}
		stderrText := strings.TrimSpace(externalBoundaryDiagnosticText(stderr.String()))
		if stderrText != "" {
			return ModelResponse{}, fmt.Errorf("provider command failed: %w: %s", err, stderrText)
		}
		return ModelResponse{}, fmt.Errorf("provider command failed: %w", err)
	}
	captured := stdout.Capture()
	if captured.Truncated {
		return ModelResponse{}, fmt.Errorf("provider command stdout exceeded %d bytes", maxProviderCommandOutputBytes)
	}
	response, err := decodeProviderCommandResponse([]byte(captured.Text))
	if err != nil {
		return ModelResponse{}, fmt.Errorf("decode provider command response: %w", err)
	}
	return response.toModelResponse(p.model)
}

func providerCommandEnvironment(env []string) []string {
	if len(env) == 0 {
		return []string{}
	}
	return append([]string(nil), env...)
}

func validateProviderCommandEnv(env []string) error {
	for _, raw := range env {
		entry := strings.TrimSpace(raw)
		name, value, ok := strings.Cut(entry, "=")
		if entry == "" || !ok || !providerCommandEnvNamePattern.MatchString(name) {
			return fmt.Errorf("%w: invalid provider command environment entry", ErrProviderCommandEnvRejected)
		}
		if providerCommandEnvNameLooksSecret(name) || providerCommandEnvValueLooksSecret(value) {
			return fmt.Errorf("%w: credential-shaped provider command environment entry", ErrProviderCommandEnvRejected)
		}
	}
	return nil
}

func providerCommandEnvNameLooksSecret(name string) bool {
	for _, token := range strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return r < 'a' || r > 'z'
	}) {
		switch token {
		case "apikey", "key", "token", "secret", "password", "passwd", "authorization", "credential":
			return true
		}
	}
	return false
}

func providerCommandEnvValueLooksSecret(value string) bool {
	text := strings.TrimSpace(value)
	if text == "" {
		return false
	}
	lower := strings.ToLower(text)
	if strings.HasPrefix(lower, "secret://") || strings.Contains(lower, "authorization") {
		return true
	}
	return containsCredentialShapedText(text)
}

func providerCommandExists(command string) bool {
	if filepath.IsAbs(command) || strings.ContainsAny(command, `/\`) {
		info, err := os.Stat(command)
		return err == nil && !info.IsDir()
	}
	_, err := exec.LookPath(command)
	return err == nil
}

type providerCommandRequest struct {
	Protocol     string                     `json:"protocol"`
	SessionID    string                     `json:"session_id"`
	TurnID       string                     `json:"turn_id"`
	Model        string                     `json:"model,omitempty"`
	InputItems   []ModelInputItem           `json:"input_items"`
	ToolManifest []ToolSpec                 `json:"tool_manifest,omitempty"`
	ToolRounds   []providerCommandToolRound `json:"tool_rounds,omitempty"`
}

type providerCommandToolRound struct {
	Calls   []ModelToolCall   `json:"calls,omitempty"`
	Results []ModelToolResult `json:"results,omitempty"`
}

func providerCommandModelToolRounds(rounds []ModelToolRound) []providerCommandToolRound {
	if len(rounds) == 0 {
		return nil
	}
	projected := make([]providerCommandToolRound, 0, len(rounds))
	for _, round := range rounds {
		next := providerCommandToolRound{
			Calls:   make([]ModelToolCall, 0, len(round.Calls)),
			Results: make([]ModelToolResult, 0, len(round.Results)),
		}
		for _, call := range round.Calls {
			next.Calls = append(next.Calls, ModelToolCall{
				ToolCallID: strings.TrimSpace(call.ToolCallID),
				Name:       strings.TrimSpace(call.Name),
				Arguments:  append(json.RawMessage(nil), call.Arguments...),
			})
		}
		for _, result := range round.Results {
			next.Results = append(next.Results, ModelToolResult{
				ToolCallID: strings.TrimSpace(result.ToolCallID),
				Name:       strings.TrimSpace(result.Name),
				Content:    result.Content,
			})
		}
		projected = append(projected, next)
	}
	return projected
}

type providerCommandResponse struct {
	Kind      string          `json:"kind"`
	Model     string          `json:"model,omitempty"`
	Text      string          `json:"text,omitempty"`
	ToolCalls []ModelToolCall `json:"tool_calls,omitempty"`
	Usage     *TokenUsage     `json:"usage,omitempty"`
}

type providerCommandResponsePayload struct {
	Kind      string                           `json:"kind"`
	Model     string                           `json:"model,omitempty"`
	Text      string                           `json:"text,omitempty"`
	ToolCalls []providerCommandToolCallPayload `json:"tool_calls,omitempty"`
	Usage     *TokenUsage                      `json:"usage,omitempty"`
}

type providerCommandToolCallPayload struct {
	ToolCallID   string           `json:"tool_call_id"`
	Name         string           `json:"name"`
	Arguments    *json.RawMessage `json:"arguments,omitempty"`
	RawArguments *string          `json:"raw_arguments,omitempty"`
}

func decodeProviderCommandResponse(data []byte) (providerCommandResponse, error) {
	var payload providerCommandResponsePayload
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return providerCommandResponse{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return providerCommandResponse{}, errors.New("provider command response contains trailing data")
	}
	toolCalls := make([]ModelToolCall, 0, len(payload.ToolCalls))
	for _, call := range payload.ToolCalls {
		if call.Arguments != nil && call.RawArguments != nil {
			return providerCommandResponse{}, errors.New("provider command tool call cannot set both arguments and raw_arguments")
		}
		next := ModelToolCall{
			ToolCallID: strings.TrimSpace(call.ToolCallID),
			Name:       strings.TrimSpace(call.Name),
		}
		switch {
		case call.RawArguments != nil:
			next.Arguments = json.RawMessage(*call.RawArguments)
		case call.Arguments != nil:
			next.Arguments = append(json.RawMessage(nil), (*call.Arguments)...)
		}
		toolCalls = append(toolCalls, next)
	}
	return providerCommandResponse{
		Kind:      payload.Kind,
		Model:     payload.Model,
		Text:      payload.Text,
		ToolCalls: toolCalls,
		Usage:     payload.Usage,
	}, nil
}

func (r providerCommandResponse) toModelResponse(defaultModel string) (ModelResponse, error) {
	model := strings.TrimSpace(r.Model)
	if model == "" {
		model = strings.TrimSpace(defaultModel)
	}
	switch strings.TrimSpace(r.Kind) {
	case providerCommandResponseKindFinal:
		if strings.TrimSpace(r.Text) == "" {
			return ModelResponse{}, errors.New("provider command final response missing text")
		}
		return ModelResponse{
			Text:  r.Text,
			Model: model,
			Usage: r.Usage,
		}, nil
	case providerCommandResponseKindToolCalls:
		if len(r.ToolCalls) == 0 {
			return ModelResponse{}, errors.New("provider command tool_calls response missing calls")
		}
		for _, call := range r.ToolCalls {
			if strings.TrimSpace(call.Name) == "" {
				return ModelResponse{}, errors.New("provider command tool call missing name")
			}
		}
		return ModelResponse{
			Model:     model,
			ToolCalls: r.ToolCalls,
			Usage:     r.Usage,
		}, nil
	default:
		return ModelResponse{}, fmt.Errorf("provider command returned unknown kind %q", r.Kind)
	}
}
