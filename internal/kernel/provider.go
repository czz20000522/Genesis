package kernel

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Provider interface {
	Name() string
	Ready() ProviderStatus
	Complete(ctx context.Context, req ModelRequest) (ModelResponse, error)
}

// ProviderPrefixIdentityProvider exposes the non-secret provider configuration
// that affects how a stable Genesis prefix is projected to that provider.
// Providers that do not implement it fall back to their public provider name.
type ProviderPrefixIdentityProvider interface {
	PrefixIdentity() string
}

type StreamingProvider interface {
	StreamComplete(ctx context.Context, req ModelRequest, emit func(ModelStreamDelta) error) (ModelResponse, error)
}

type ModelStreamDelta struct {
	Text string
}

var ErrProviderUnavailable = errors.New("provider unavailable")

type ModelRequest struct {
	SessionID         string
	TurnID            string
	InputItems        []ModelInputItem
	Conversation      []ModelConversationMessage
	ToolManifest      []ToolSpec
	ToolRounds        []ModelToolRound
	PrefixFingerprint string
}

type ProviderContextProjection struct {
	SessionID                 string
	TurnID                    string
	InputItems                []ModelInputItem
	Conversation              []ModelConversationMessage
	ToolManifest              []ToolSpec
	ToolRounds                []ModelToolRound
	PrefixFingerprint         string
	PrefixComponents          PrefixFingerprintComponents
	KernelObservationEventIDs []string
	HistoryTurnIDs            []string
	CompactedThroughTurnID    string
}

func (p ProviderContextProjection) ModelRequest() ModelRequest {
	return ModelRequest{
		SessionID:         p.SessionID,
		TurnID:            p.TurnID,
		InputItems:        cloneModelInputItems(p.InputItems),
		Conversation:      cloneModelConversationMessages(p.Conversation),
		ToolManifest:      cloneToolSpecs(p.ToolManifest),
		ToolRounds:        cloneModelToolRounds(p.ToolRounds),
		PrefixFingerprint: p.PrefixFingerprint,
	}
}

const (
	ModelInputKindUserText                   = "user_text"
	ModelInputKindSkillIndexContext          = "skill_index_context"
	ModelInputKindConversationHistoryContext = "conversation_history_context"
	ModelInputKindKernelObservationContext   = "kernel_observation_context"
	ModelInputKindProviderRepairContext      = "provider_repair_context"
	ModelInputKindHydratedContext            = "hydrated_context"
	ModelInputKindSourceSnapshotContext      = "source_snapshot_context"
)

type ModelInputItem struct {
	Kind string
	Text string
}

type ModelConversationMessage struct {
	Role                    string          `json:"role"`
	Text                    string          `json:"text,omitempty"`
	ReasoningText           string          `json:"reasoning_text,omitempty"`
	ReasoningAdapterID      string          `json:"-"`
	ReasoningAdapterProfile string          `json:"-"`
	ToolCalls               []ModelToolCall `json:"tool_calls,omitempty"`
	ToolCallID              string          `json:"tool_call_id,omitempty"`
}

type ModelResponse struct {
	Reasoning *ReasoningMessage
	Text      string
	Model     string
	ToolCalls []ModelToolCall
	Usage     *TokenUsage
}

type ReasoningMessage struct {
	ReasoningID      string    `json:"reasoning_id"`
	TurnID           string    `json:"turn_id"`
	Text             string    `json:"text"`
	AdapterID        string    `json:"adapter_id,omitempty"`
	AdapterProfileID string    `json:"adapter_profile_id,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type ReasoningMessageProjection struct {
	ReasoningID string    `json:"reasoning_id"`
	TurnID      string    `json:"turn_id"`
	Text        string    `json:"text"`
	CreatedAt   time.Time `json:"created_at"`
}

type FakeProvider struct{}

func (FakeProvider) Name() string {
	return "fake"
}

func (p FakeProvider) Ready() ProviderStatus {
	return ProviderStatus{
		Name:      p.Name(),
		Readiness: ReadinessReady,
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
		Name:            p.Name(),
		Readiness:       ReadinessNotReady,
		ReadinessReason: p.reason,
	}
}

func (p BlockedProvider) Complete(_ context.Context, _ ModelRequest) (ModelResponse, error) {
	return ModelResponse{}, fmt.Errorf("%w: %s", ErrProviderUnavailable, p.reason)
}
