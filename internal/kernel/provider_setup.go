package kernel

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type OpenAICompatibleProviderSetupRequest struct {
	ConfigRoot          string
	CredentialStoreRoot string
	ModelRole           string
	ProfileID           string
	GatewayRoute        string
	ProviderAdapterID   string
	AdapterProfileID    string
	HiddenReasoningMode string
	BaseURL             string
	ModelID             string
	ContextWindowTokens int
	CredentialRef       string
	APIKey              string
	RequestTimeout      time.Duration
	DryRun              bool
	Verify              bool
	SecretProtector     func([]byte) ([]byte, error)
	SecretResolver      func(ref string, storeRoot string) (string, error)
}

type OpenAICompatibleProviderSetupResult struct {
	ConfigPath     string `json:"config_path"`
	CredentialPath string `json:"credential_path"`
	CredentialRef  string `json:"credential_ref"`
	ModelRole      string `json:"model_role"`
	ProfileID      string `json:"profile_id"`
	GatewayRoute   string `json:"gateway_route"`
	DryRun         bool   `json:"dry_run"`
	Verified       bool   `json:"verified"`
}

type OpenAICompatibleProviderCredentialRotationRequest struct {
	ConfigRoot          string
	CredentialStoreRoot string
	ModelRole           string
	ProfileID           string
	APIKey              string
	DryRun              bool
	Verify              bool
	SecretProtector     func([]byte) ([]byte, error)
	SecretResolver      func(ref string, storeRoot string) (string, error)
}

func SetupOpenAICompatibleProvider(req OpenAICompatibleProviderSetupRequest) (OpenAICompatibleProviderSetupResult, error) {
	normalized, err := normalizeProviderSetup(req)
	if err != nil {
		return OpenAICompatibleProviderSetupResult{}, err
	}
	configRoot := resolveGenesisConfigRoot(normalized.ConfigRoot)
	configPath := filepath.Join(configRoot, "models.json")
	secretResult, err := WriteLocalCredentialSecret(LocalCredentialSecretWriteRequest{
		CredentialRef: normalized.CredentialRef,
		Secret:        normalized.APIKey,
		StoreRoot:     normalized.CredentialStoreRoot,
		Protector:     normalized.SecretProtector,
		DryRun:        normalized.DryRun,
	})
	if err != nil {
		return OpenAICompatibleProviderSetupResult{}, err
	}
	result := OpenAICompatibleProviderSetupResult{
		ConfigPath:     configPath,
		CredentialPath: secretResult.CredentialPath,
		CredentialRef:  secretResult.CredentialRef,
		ModelRole:      normalized.ModelRole,
		ProfileID:      normalized.ProfileID,
		GatewayRoute:   normalized.GatewayRoute,
		DryRun:         normalized.DryRun,
	}
	if normalized.DryRun {
		return result, nil
	}

	config, err := readGenesisModelsConfig(configPath)
	if err != nil {
		return OpenAICompatibleProviderSetupResult{}, err
	}
	upsertOpenAICompatibleProviderConfig(&config, normalized, secretResult.CredentialRef)
	if err := writeGenesisModelsConfig(configPath, config); err != nil {
		return OpenAICompatibleProviderSetupResult{}, err
	}
	if normalized.Verify {
		resolver := normalized.SecretResolver
		if resolver == nil {
			resolver = ResolveLocalCredentialSecret
		}
		resolved, err := ResolveOpenAICompatibleConfigFromGenesis(GenesisModelConfigRequest{
			ConfigRoot:          normalized.ConfigRoot,
			CredentialStoreRoot: normalized.CredentialStoreRoot,
			ModelRole:           normalized.ModelRole,
			ModelProfileID:      normalized.ProfileID,
			SecretResolver:      resolver,
		})
		if err != nil {
			return OpenAICompatibleProviderSetupResult{}, err
		}
		if resolved.BaseURL != normalized.BaseURL || resolved.Model != normalized.ModelID || strings.TrimSpace(resolved.APIKey) == "" {
			return OpenAICompatibleProviderSetupResult{}, errors.New("provider setup verification failed")
		}
		result.Verified = true
	}
	return result, nil
}

func RotateActiveOpenAICompatibleProviderCredential(req OpenAICompatibleProviderCredentialRotationRequest) (OpenAICompatibleProviderSetupResult, error) {
	modelRole := strings.TrimSpace(req.ModelRole)
	if modelRole == "" {
		modelRole = DefaultModelRole
	}
	if strings.TrimSpace(req.APIKey) == "" && !req.DryRun {
		return OpenAICompatibleProviderSetupResult{}, ErrLocalSecretMissing
	}
	selected, err := loadSelectedGatewayConfig(GenesisModelConfigRequest{
		ConfigRoot:     req.ConfigRoot,
		ModelRole:      modelRole,
		ModelProfileID: req.ProfileID,
	})
	if err != nil {
		return OpenAICompatibleProviderSetupResult{}, err
	}
	credentialRef := firstNonEmpty(selected.route.CredentialRef, selected.gateway.CredentialRef)
	if !isLocalSecretCredentialRef(credentialRef) {
		return OpenAICompatibleProviderSetupResult{}, ErrGenesisModelCredentialUnsupported
	}
	secretResult, err := WriteLocalCredentialSecret(LocalCredentialSecretWriteRequest{
		CredentialRef: credentialRef,
		Secret:        strings.TrimSpace(req.APIKey),
		StoreRoot:     req.CredentialStoreRoot,
		Protector:     req.SecretProtector,
		DryRun:        req.DryRun,
	})
	if err != nil {
		return OpenAICompatibleProviderSetupResult{}, err
	}
	result := OpenAICompatibleProviderSetupResult{
		ConfigPath:     filepath.Join(resolveGenesisConfigRoot(req.ConfigRoot), "models.json"),
		CredentialPath: secretResult.CredentialPath,
		CredentialRef:  secretResult.CredentialRef,
		ModelRole:      modelRole,
		ProfileID:      selected.profile.ProfileID,
		GatewayRoute:   selected.profile.GatewayRoute,
		DryRun:         req.DryRun,
	}
	if req.DryRun {
		return result, nil
	}
	if req.Verify {
		resolver := req.SecretResolver
		if resolver == nil {
			resolver = ResolveLocalCredentialSecret
		}
		resolved, err := ResolveProviderConfigFromGenesis(GenesisModelConfigRequest{
			ConfigRoot:          req.ConfigRoot,
			CredentialStoreRoot: req.CredentialStoreRoot,
			ModelRole:           modelRole,
			ModelProfileID:      selected.profile.ProfileID,
			SecretResolver:      resolver,
		})
		if err != nil {
			return OpenAICompatibleProviderSetupResult{}, err
		}
		if resolved.Kind != "openai-compatible" || strings.TrimSpace(resolved.OpenAICompatible.APIKey) == "" {
			return OpenAICompatibleProviderSetupResult{}, errors.New("provider credential rotation verification failed")
		}
		result.Verified = true
	}
	return result, nil
}

func normalizeProviderSetup(req OpenAICompatibleProviderSetupRequest) (OpenAICompatibleProviderSetupRequest, error) {
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
		return OpenAICompatibleProviderSetupRequest{}, errors.New("base_url is required")
	}
	req.ModelID = strings.TrimSpace(req.ModelID)
	if req.ModelID == "" {
		return OpenAICompatibleProviderSetupRequest{}, errors.New("model_id is required")
	}
	req.CredentialRef = normalizeLocalSecretRef(req.CredentialRef)
	if req.CredentialRef == "" {
		return OpenAICompatibleProviderSetupRequest{}, ErrLocalSecretRefInvalid
	}
	if strings.TrimSpace(req.APIKey) == "" && !req.DryRun {
		return OpenAICompatibleProviderSetupRequest{}, ErrLocalSecretMissing
	}
	if req.RequestTimeout < 0 {
		return OpenAICompatibleProviderSetupRequest{}, errors.New("request_timeout must not be negative")
	}
	req.APIKey = strings.TrimSpace(req.APIKey)
	return req, nil
}

func readGenesisModelsConfig(configPath string) (genesisModelsConfig, error) {
	payload, err := os.ReadFile(configPath)
	if errors.Is(err, os.ErrNotExist) {
		return genesisModelsConfig{}, nil
	}
	if err != nil {
		return genesisModelsConfig{}, fmt.Errorf("%w: %v", ErrGenesisModelConfigMissing, err)
	}
	var config genesisModelsConfig
	if err := json.Unmarshal(payload, &config); err != nil {
		return genesisModelsConfig{}, fmt.Errorf("%w: %v", ErrGenesisModelConfigMissing, err)
	}
	return config, nil
}

func upsertOpenAICompatibleProviderConfig(config *genesisModelsConfig, req OpenAICompatibleProviderSetupRequest, credentialRef string) {
	if config.ActiveModelProfileBindings == nil {
		config.ActiveModelProfileBindings = map[string]string{}
	}
	config.ActiveModelProfileBindings[req.ModelRole] = req.ProfileID

	if config.ModelGateway.Routes == nil {
		config.ModelGateway.Routes = map[string]genesisGatewayRoute{}
	}
	config.ModelGateway.Routes[req.GatewayRoute] = genesisGatewayRoute{
		BaseURL:           req.BaseURL,
		CredentialRef:     credentialRef,
		Protocol:          modelGatewayProtocolChatCompletions,
		RequestTimeoutSec: req.RequestTimeout.Seconds(),
	}
	if strings.TrimSpace(config.ModelGateway.Protocol) == "" {
		config.ModelGateway.Protocol = modelGatewayProtocolChatCompletions
	}

	if config.ModelProfiles.Cloud.Gateway == nil {
		config.ModelProfiles.Cloud.Gateway = map[string]genesisGatewayProfile{}
	}
	config.ModelProfiles.Cloud.Gateway[req.ProfileID] = genesisGatewayProfile{
		ProfileID:                req.ProfileID,
		ModelID:                  req.ModelID,
		GatewayRoute:             req.GatewayRoute,
		ContextWindowTokens:      req.ContextWindowTokens,
		ProviderAdapterID:        req.ProviderAdapterID,
		ProviderAdapterProfileID: req.AdapterProfileID,
		HiddenReasoningPolicy:    req.HiddenReasoningMode,
	}
}

func writeGenesisModelsConfig(configPath string, config genesisModelsConfig) error {
	encoded, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("%w: %v", ErrGenesisModelConfigMissing, err)
	}
	if err := os.WriteFile(configPath, append(encoded, '\n'), 0o644); err != nil {
		return fmt.Errorf("%w: %v", ErrGenesisModelConfigMissing, err)
	}
	return nil
}
