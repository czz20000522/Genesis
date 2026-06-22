package kernel

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type Provider interface {
	Name() string
	Ready() ProviderStatus
	Complete(ctx context.Context, req ModelRequest) (ModelResponse, error)
}

var ErrProviderUnavailable = errors.New("provider unavailable")

type ModelRequest struct {
	SessionID    string
	TurnID       string
	InputItems   []ModelInputItem
	ToolManifest []ToolSpec
	ToolRounds   []ModelToolRound
}

type ProviderContextProjection struct {
	SessionID    string
	TurnID       string
	InputItems   []ModelInputItem
	ToolManifest []ToolSpec
	ToolRounds   []ModelToolRound
}

func (p ProviderContextProjection) ModelRequest() ModelRequest {
	return ModelRequest{
		SessionID:    p.SessionID,
		TurnID:       p.TurnID,
		InputItems:   cloneModelInputItems(p.InputItems),
		ToolManifest: cloneToolSpecs(p.ToolManifest),
		ToolRounds:   cloneModelToolRounds(p.ToolRounds),
	}
}

const (
	ModelInputKindUserText                   = "user_text"
	ModelInputKindApprovedMemoryContext      = "approved_memory_context"
	ModelInputKindSkillCatalogContext        = "skill_catalog_context"
	ModelInputKindConversationHistoryContext = "conversation_history_context"
)

type ModelInputItem struct {
	Kind string
	Text string
}

type ModelResponse struct {
	Text      string
	Model     string
	ToolCalls []ModelToolCall
	Usage     *TokenUsage
}

type FakeProvider struct{}

func (FakeProvider) Name() string {
	return "fake"
}

func (p FakeProvider) Ready() ProviderStatus {
	return ProviderStatus{
		Name:   p.Name(),
		Status: "ok",
	}
}

func (FakeProvider) Complete(_ context.Context, req ModelRequest) (ModelResponse, error) {
	var parts []string
	for _, item := range req.InputItems {
		if item.Text != "" {
			parts = append(parts, item.Text)
		}
	}
	text := strings.TrimSpace(strings.Join(parts, "\n"))
	if text == "" {
		text = "empty turn"
	}
	return ModelResponse{
		Text:  fmt.Sprintf("fake: %s", text),
		Model: "fake-model",
	}, nil
}

type BlockedProvider struct {
	name   string
	reason string
}

func NewBlockedProvider(name string, reason string) BlockedProvider {
	if strings.TrimSpace(name) == "" {
		name = "provider"
	}
	if strings.TrimSpace(reason) == "" {
		reason = "provider_blocked"
	}
	return BlockedProvider{
		name:   strings.TrimSpace(name),
		reason: strings.TrimSpace(reason),
	}
}

func (p BlockedProvider) Name() string {
	return p.name
}

func (p BlockedProvider) Ready() ProviderStatus {
	return ProviderStatus{
		Name:   p.Name(),
		Status: "blocked",
		Reason: p.reason,
	}
}

func (p BlockedProvider) Complete(_ context.Context, _ ModelRequest) (ModelResponse, error) {
	return ModelResponse{}, fmt.Errorf("%w: %s", ErrProviderUnavailable, p.reason)
}
