package kernel

import (
	"context"
	"net/http"
	"strings"
	"time"
)

const (
	defaultProviderVerifyTimeout = 10 * time.Second
	providerVerifyProbeText      = "Reply with exactly: GENESIS_PROVIDER_VERIFY_OK"
)

type ProviderLiveVerifyRequest struct {
	ConfigRoot          string
	CredentialStoreRoot string
	ModelRole           string
	ModelProfileID      string
	Timeout             time.Duration
	SecretResolver      func(ref string, storeRoot string) (string, error)
	HTTPClient          *http.Client
}

type ProviderLiveVerifyResult struct {
	Readiness       string         `json:"readiness"`
	ReadinessReason string         `json:"readiness_reason,omitempty"`
	Provider        ProviderStatus `json:"provider"`
	ModelRole       string         `json:"model_role,omitempty"`
	ProfileID       string         `json:"profile_id,omitempty"`
	Model           string         `json:"model,omitempty"`
	Usage           *TokenUsage    `json:"usage,omitempty"`
}

func VerifyProviderLive(req ProviderLiveVerifyRequest) ProviderLiveVerifyResult {
	modelRole := strings.TrimSpace(req.ModelRole)
	if modelRole == "" {
		modelRole = DefaultModelRole
	}
	selected, selectedErr := loadSelectedGatewayConfig(GenesisModelConfigRequest{
		ConfigRoot:     req.ConfigRoot,
		ModelRole:      modelRole,
		ModelProfileID: req.ModelProfileID,
	})
	profileID := strings.TrimSpace(req.ModelProfileID)
	if profileID == "" && selectedErr == nil {
		profileID = selected.profile.ProfileID
	}
	resolved, err := ResolveProviderConfigFromGenesis(GenesisModelConfigRequest{
		ConfigRoot:          req.ConfigRoot,
		CredentialStoreRoot: req.CredentialStoreRoot,
		ModelRole:           modelRole,
		ModelProfileID:      req.ModelProfileID,
		SecretResolver:      req.SecretResolver,
	})
	if err != nil {
		return ProviderLiveVerifyResult{
			Readiness:       ReadinessNotReady,
			ReadinessReason: ProviderConfigReason(err),
			Provider:        ProviderStatus{Name: "genesis-config", Readiness: ReadinessNotReady, ReadinessReason: ProviderConfigReason(err)},
			ModelRole:       modelRole,
			ProfileID:       profileID,
		}
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = defaultProviderVerifyTimeout
	}
	var provider Provider
	var configuredModel string
	switch resolved.Kind {
	case "openai-compatible":
		config := resolved.OpenAICompatible
		config.RequestTimeout = timeout
		config.HTTPClient = req.HTTPClient
		provider = NewOpenAICompatibleProvider(config)
		configuredModel = config.Model
	case "provider_command":
		config := resolved.Command
		config.RequestTimeout = timeout
		provider = NewCommandProvider(config)
		configuredModel = config.Model
	default:
		return ProviderLiveVerifyResult{
			Readiness:       ReadinessNotReady,
			ReadinessReason: "provider_protocol_unsupported",
			Provider:        ProviderStatus{Name: resolved.Kind, Readiness: ReadinessNotReady, ReadinessReason: "provider_protocol_unsupported"},
			ModelRole:       modelRole,
			ProfileID:       profileID,
		}
	}
	status := provider.Ready()
	if status.Readiness != ReadinessReady {
		return ProviderLiveVerifyResult{
			Readiness:       ReadinessNotReady,
			ReadinessReason: firstNonEmpty(status.ReadinessReason, "provider_not_ready"),
			Provider:        status,
			ModelRole:       modelRole,
			ProfileID:       profileID,
			Model:           configuredModel,
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	resp, err := provider.Complete(ctx, ModelRequest{
		SessionID:  "provider-verify",
		TurnID:     "provider-verify",
		InputItems: []ModelInputItem{{Kind: ModelInputKindUserText, Text: providerVerifyProbeText}},
	})
	if err != nil {
		failure := providerFailureFromError(err)
		reason := firstNonEmpty(failure.ReasonCode, "provider_live_verify_failed")
		return ProviderLiveVerifyResult{
			Readiness:       ReadinessNotReady,
			ReadinessReason: reason,
			Provider:        ProviderStatus{Name: provider.Name(), Readiness: ReadinessNotReady, ReadinessReason: reason},
			ModelRole:       modelRole,
			ProfileID:       profileID,
			Model:           configuredModel,
		}
	}
	model := strings.TrimSpace(resp.Model)
	if model == "" {
		model = configuredModel
	}
	return ProviderLiveVerifyResult{
		Readiness: ReadinessReady,
		Provider:  ProviderStatus{Name: provider.Name(), Readiness: ReadinessReady},
		ModelRole: modelRole,
		ProfileID: profileID,
		Model:     model,
		Usage:     resp.Usage,
	}
}
