package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
		stderrText := strings.TrimSpace(redactEvidenceText(stderr.String()))
		if stderrText != "" {
			return ModelResponse{}, fmt.Errorf("provider command failed: %w: %s", err, stderrText)
		}
		return ModelResponse{}, fmt.Errorf("provider command failed: %w", err)
	}
	captured := stdout.Capture()
	if captured.Truncated {
		return ModelResponse{}, fmt.Errorf("provider command stdout exceeded %d bytes", maxProviderCommandOutputBytes)
	}
	var response providerCommandResponse
	if err := json.Unmarshal([]byte(captured.Text), &response); err != nil {
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
