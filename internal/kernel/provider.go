package kernel

import (
	"context"
	"fmt"
	"strings"
)

type Provider interface {
	Name() string
	Ready() ProviderStatus
	Complete(ctx context.Context, req ModelRequest) (ModelResponse, error)
}

type ModelRequest struct {
	SessionID  string
	TurnID     string
	InputItems []InputItem
}

type ModelResponse struct {
	Text  string
	Model string
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
		if item.Type == "text" && item.Text != "" {
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
