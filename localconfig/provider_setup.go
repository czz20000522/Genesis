package localconfig

import (
	"errors"
	"strings"
	"time"
)

const openAIChatCompletionsProtocol = "openai-chat-completions"

type OpenAICompatiblePreset struct {
	ProviderID          string
	ModelID             string
	ProfileID           string
	GatewayRoute        string
	AdapterID           string
	AdapterProfileID    string
	BaseURL             string
	CredentialRef       string
	APIKeyEnv           string
	ContextWindowTokens int
	RequestTimeout      time.Duration
}

func DeepSeekFlashPreset() OpenAICompatiblePreset {
	return OpenAICompatiblePreset{
		ProviderID:          "deepseek",
		ModelID:             "deepseek-v4-flash",
		ProfileID:           "deepseek-flash",
		GatewayRoute:        "deepseek",
		AdapterID:           "deepseek",
		AdapterProfileID:    "deepseek-v4-flash",
		BaseURL:             "https://api.deepseek.com",
		CredentialRef:       "secret://models/deepseek/local",
		APIKeyEnv:           "DEEPSEEK_API_KEY",
		ContextWindowTokens: 1000000,
		RequestTimeout:      60 * time.Second,
	}
}

type OpenAICompatibleProfileSetupRequest struct {
	ConfigRoot          string
	CredentialStoreRoot string
	ModelRole           string
	ProfileID           string
	GatewayRoute        string
	ProviderAdapterID   string
	AdapterProfileID    string
	BaseURL             string
	ModelID             string
	ContextWindowTokens int
	CredentialRef       string
	APIKey              string
	RequestTimeout      time.Duration
	BindRole            bool
	DryRun              bool
	Protector           func([]byte) ([]byte, error)
}

type OpenAICompatibleProfileSetupResult struct {
	ConfigPath     string
	CredentialPath string
	CredentialRef  string
	ModelRole      string
	ProfileID      string
	GatewayRoute   string
	DryRun         bool
}

type DeepSeekFlashSetupRequest struct {
	ConfigRoot          string
	CredentialStoreRoot string
	APIKey              string
	Protector           func([]byte) ([]byte, error)
}

type DeepSeekFlashSetupResult struct {
	ProfileID         string `json:"profile_id"`
	GatewayRoute      string `json:"gateway_route"`
	CredentialPresent bool   `json:"credential_present"`
}

func SetupDeepSeekFlash(req DeepSeekFlashSetupRequest) (DeepSeekFlashSetupResult, error) {
	preset := DeepSeekFlashPreset()
	result, err := SetupOpenAICompatibleProfile(OpenAICompatibleProfileSetupRequest{
		ConfigRoot:          req.ConfigRoot,
		CredentialStoreRoot: req.CredentialStoreRoot,
		ProfileID:           preset.ProfileID,
		GatewayRoute:        preset.GatewayRoute,
		ProviderAdapterID:   preset.AdapterID,
		AdapterProfileID:    preset.AdapterProfileID,
		BaseURL:             preset.BaseURL,
		ModelID:             preset.ModelID,
		ContextWindowTokens: preset.ContextWindowTokens,
		CredentialRef:       preset.CredentialRef,
		APIKey:              req.APIKey,
		RequestTimeout:      preset.RequestTimeout,
		Protector:           req.Protector,
	})
	if err != nil {
		return DeepSeekFlashSetupResult{}, err
	}
	return DeepSeekFlashSetupResult{
		ProfileID:         result.ProfileID,
		GatewayRoute:      result.GatewayRoute,
		CredentialPresent: !result.DryRun,
	}, nil
}

func SetupOpenAICompatibleProfile(req OpenAICompatibleProfileSetupRequest) (OpenAICompatibleProfileSetupResult, error) {
	normalized, err := normalizeOpenAICompatibleProfileSetup(req)
	if err != nil {
		return OpenAICompatibleProfileSetupResult{}, err
	}
	secret, err := WriteCredentialSecret(CredentialSecretWriteRequest{
		CredentialRef: normalized.CredentialRef,
		Secret:        normalized.APIKey,
		StoreRoot:     normalized.CredentialStoreRoot,
		Protector:     normalized.Protector,
		DryRun:        normalized.DryRun,
	})
	if err != nil {
		return OpenAICompatibleProfileSetupResult{}, err
	}
	result := OpenAICompatibleProfileSetupResult{
		ConfigPath:     ConfigPath(normalized.ConfigRoot),
		CredentialPath: secret.CredentialPath,
		CredentialRef:  secret.CredentialRef,
		ModelRole:      normalized.ModelRole,
		ProfileID:      normalized.ProfileID,
		GatewayRoute:   normalized.GatewayRoute,
		DryRun:         normalized.DryRun,
	}
	if normalized.DryRun {
		return result, nil
	}
	config, err := ReadModels(result.ConfigPath)
	if errors.Is(err, ErrConfigMissing) {
		config = ModelsConfig{}
	} else if err != nil {
		return OpenAICompatibleProfileSetupResult{}, err
	}
	upsertOpenAICompatibleProfile(&config, normalized, secret.CredentialRef)
	if err := WriteModels(result.ConfigPath, config); err != nil {
		return OpenAICompatibleProfileSetupResult{}, err
	}
	return result, nil
}

func normalizeOpenAICompatibleProfileSetup(req OpenAICompatibleProfileSetupRequest) (OpenAICompatibleProfileSetupRequest, error) {
	req.ModelRole = strings.TrimSpace(req.ModelRole)
	if req.ModelRole == "" {
		req.ModelRole = DefaultModelRole
	}
	req.ProfileID = strings.TrimSpace(req.ProfileID)
	if req.ProfileID == "" {
		req.ProfileID = "default"
	}
	req.GatewayRoute = strings.TrimSpace(req.GatewayRoute)
	if req.GatewayRoute == "" {
		req.GatewayRoute = "default"
	}
	req.BaseURL = strings.TrimSpace(req.BaseURL)
	if req.BaseURL == "" {
		return OpenAICompatibleProfileSetupRequest{}, errors.New("base_url is required")
	}
	req.ModelID = strings.TrimSpace(req.ModelID)
	if req.ModelID == "" {
		return OpenAICompatibleProfileSetupRequest{}, errors.New("model_id is required")
	}
	req.CredentialRef = NormalizeCredentialRef(req.CredentialRef)
	if req.CredentialRef == "" {
		return OpenAICompatibleProfileSetupRequest{}, ErrCredentialRefInvalid
	}
	req.APIKey = strings.TrimSpace(req.APIKey)
	if req.APIKey == "" && !req.DryRun {
		return OpenAICompatibleProfileSetupRequest{}, ErrCredentialMissing
	}
	if req.RequestTimeout < 0 {
		return OpenAICompatibleProfileSetupRequest{}, errors.New("request_timeout must not be negative")
	}
	return req, nil
}

func upsertOpenAICompatibleProfile(config *ModelsConfig, req OpenAICompatibleProfileSetupRequest, credentialRef string) {
	if req.BindRole {
		if config.ActiveModelProfileBindings == nil {
			config.ActiveModelProfileBindings = map[string]string{}
		}
		config.ActiveModelProfileBindings[req.ModelRole] = req.ProfileID
	}
	if config.ModelGateway.Routes == nil {
		config.ModelGateway.Routes = map[string]GatewayRoute{}
	}
	config.ModelGateway.Routes[req.GatewayRoute] = GatewayRoute{
		BaseURL:           req.BaseURL,
		CredentialRef:     credentialRef,
		Protocol:          openAIChatCompletionsProtocol,
		RequestTimeoutSec: req.RequestTimeout.Seconds(),
	}
	if strings.TrimSpace(config.ModelGateway.Protocol) == "" {
		config.ModelGateway.Protocol = openAIChatCompletionsProtocol
	}
	if config.ModelProfiles.Cloud.Gateway == nil {
		config.ModelProfiles.Cloud.Gateway = map[string]GatewayProfile{}
	}
	config.ModelProfiles.Cloud.Gateway[req.ProfileID] = GatewayProfile{
		ProfileID:                req.ProfileID,
		ModelID:                  req.ModelID,
		GatewayRoute:             req.GatewayRoute,
		ContextWindowTokens:      req.ContextWindowTokens,
		ProviderAdapterID:        strings.TrimSpace(req.ProviderAdapterID),
		ProviderAdapterProfileID: strings.TrimSpace(req.AdapterProfileID),
	}
}
