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

const DefaultModelRole = "foreground.coordinator"

const (
	modelGatewayProtocolChatCompletions = "openai-chat-completions"
)

var (
	ErrGenesisModelConfigMissing         = errors.New("genesis model config missing")
	ErrGenesisModelProfileMissing        = errors.New("genesis model profile missing")
	ErrGenesisModelGatewayMissing        = errors.New("genesis model gateway missing")
	ErrGenesisModelGatewayRouteMissing   = errors.New("genesis model gateway route missing")
	ErrGenesisModelProtocolUnsupported   = errors.New("genesis model gateway protocol unsupported")
	ErrGenesisModelCredentialMissing     = errors.New("genesis model credential missing")
	ErrGenesisModelCredentialUnsupported = errors.New("genesis model credential unsupported")
)

type GenesisModelConfigRequest struct {
	ConfigRoot          string
	CredentialStoreRoot string
	ModelRole           string
	ModelProfileID      string
	SecretResolver      func(ref string, storeRoot string) (string, error)
}

func ResolveOpenAICompatibleConfigFromGenesis(req GenesisModelConfigRequest) (OpenAICompatibleConfig, error) {
	configPath := filepath.Join(resolveGenesisConfigRoot(req.ConfigRoot), "models.json")
	payload, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return OpenAICompatibleConfig{}, ErrGenesisModelConfigMissing
		}
		return OpenAICompatibleConfig{}, fmt.Errorf("%w: %v", ErrGenesisModelConfigMissing, err)
	}
	var config genesisModelsConfig
	if err := json.Unmarshal(payload, &config); err != nil {
		return OpenAICompatibleConfig{}, fmt.Errorf("%w: %v", ErrGenesisModelConfigMissing, err)
	}
	gateway := config.ModelGateway
	if strings.TrimSpace(gateway.BaseURL) == "" && len(gateway.Routes) == 0 {
		return OpenAICompatibleConfig{}, ErrGenesisModelGatewayMissing
	}

	profile, err := selectGatewayProfile(config, req)
	if err != nil {
		return OpenAICompatibleConfig{}, err
	}
	route, err := selectGatewayRoute(gateway, profile.GatewayRoute)
	if err != nil {
		return OpenAICompatibleConfig{}, err
	}
	baseURL := firstNonEmpty(route.BaseURL, gateway.BaseURL)
	credentialRef := firstNonEmpty(route.CredentialRef, gateway.CredentialRef)
	protocol := firstNonEmpty(route.Protocol, gateway.Protocol)
	if protocol != modelGatewayProtocolChatCompletions {
		return OpenAICompatibleConfig{}, ErrGenesisModelProtocolUnsupported
	}
	if strings.TrimSpace(baseURL) == "" || strings.TrimSpace(profile.ModelID) == "" {
		return OpenAICompatibleConfig{}, ErrGenesisModelGatewayMissing
	}
	if !isLocalSecretCredentialRef(credentialRef) {
		return OpenAICompatibleConfig{}, ErrGenesisModelCredentialUnsupported
	}
	resolver := req.SecretResolver
	if resolver == nil {
		resolver = ResolveLocalCredentialSecret
	}
	apiKey, err := resolver(credentialRef, req.CredentialStoreRoot)
	if err != nil {
		return OpenAICompatibleConfig{}, fmt.Errorf("%w: %v", ErrGenesisModelCredentialMissing, err)
	}
	if strings.TrimSpace(apiKey) == "" {
		return OpenAICompatibleConfig{}, ErrGenesisModelCredentialMissing
	}
	timeout := firstPositiveFloat(route.RequestTimeoutSec, gateway.RequestTimeoutSec)
	return OpenAICompatibleConfig{
		BaseURL:        baseURL,
		APIKey:         apiKey,
		Model:          profile.ModelID,
		RequestTimeout: durationSeconds(timeout),
	}, nil
}

func ProviderConfigReason(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrGenesisModelConfigMissing):
		return "provider_config_missing"
	case errors.Is(err, ErrGenesisModelProfileMissing):
		return "provider_profile_missing"
	case errors.Is(err, ErrGenesisModelGatewayRouteMissing):
		return "provider_route_missing"
	case errors.Is(err, ErrGenesisModelProtocolUnsupported):
		return "provider_protocol_unsupported"
	case errors.Is(err, ErrGenesisModelCredentialUnsupported):
		return "provider_credential_unsupported"
	case errors.Is(err, ErrGenesisModelCredentialMissing):
		return "provider_credential_missing"
	case errors.Is(err, ErrGenesisModelGatewayMissing):
		return "provider_gateway_missing"
	default:
		return "provider_config_invalid"
	}
}

func resolveGenesisConfigRoot(configRoot string) string {
	if root := strings.TrimSpace(configRoot); root != "" {
		return filepath.Clean(expandHome(root))
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".genesis", "config")
	}
	return filepath.Join(home, ".genesis", "config")
}

func selectGatewayProfile(config genesisModelsConfig, req GenesisModelConfigRequest) (genesisGatewayProfile, error) {
	profileID := strings.TrimSpace(req.ModelProfileID)
	if profileID == "" {
		role := strings.TrimSpace(req.ModelRole)
		if role == "" {
			role = DefaultModelRole
		}
		profileID = strings.TrimSpace(config.ActiveModelProfileBindings[role])
	}
	if profileID == "" {
		return genesisGatewayProfile{}, ErrGenesisModelProfileMissing
	}
	for _, profiles := range []map[string]genesisGatewayProfile{
		config.ModelProfiles.Cloud.Gateway,
		config.ModelProfiles.Local.Gateway,
	} {
		for key, profile := range profiles {
			if strings.TrimSpace(profile.ProfileID) == "" {
				profile.ProfileID = key
			}
			if strings.TrimSpace(key) == profileID || strings.TrimSpace(profile.ProfileID) == profileID {
				return profile, nil
			}
		}
	}
	return genesisGatewayProfile{}, ErrGenesisModelProfileMissing
}

func selectGatewayRoute(gateway genesisModelGateway, routeName string) (genesisGatewayRoute, error) {
	name := strings.TrimSpace(routeName)
	if name == "" {
		return genesisGatewayRoute{}, nil
	}
	route, ok := gateway.Routes[name]
	if !ok {
		return genesisGatewayRoute{}, ErrGenesisModelGatewayRouteMissing
	}
	return route, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if text := strings.TrimSpace(value); text != "" {
			return text
		}
	}
	return ""
}

func firstPositiveFloat(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func durationSeconds(value float64) time.Duration {
	if value <= 0 {
		return 0
	}
	return time.Duration(value * float64(time.Second))
}

func expandHome(value string) string {
	text := strings.TrimSpace(value)
	if text == "~" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return home
		}
	}
	if strings.HasPrefix(text, "~/") || strings.HasPrefix(text, "~\\") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, text[2:])
		}
	}
	return text
}

type genesisModelsConfig struct {
	ModelGateway               genesisModelGateway  `json:"model_gateway"`
	ActiveModelProfileBindings map[string]string    `json:"active_model_profile_bindings"`
	ModelProfiles              genesisModelProfiles `json:"model_profiles"`
}

type genesisModelGateway struct {
	BaseURL           string                         `json:"base_url"`
	CredentialRef     string                         `json:"credential_ref"`
	Protocol          string                         `json:"protocol"`
	RequestTimeoutSec float64                        `json:"request_timeout_sec"`
	Routes            map[string]genesisGatewayRoute `json:"routes"`
}

type genesisGatewayRoute struct {
	BaseURL           string  `json:"base_url"`
	CredentialRef     string  `json:"credential_ref"`
	Protocol          string  `json:"protocol"`
	RequestTimeoutSec float64 `json:"request_timeout_sec"`
}

type genesisModelProfiles struct {
	Cloud genesisGatewayProfileBranch `json:"cloud"`
	Local genesisGatewayProfileBranch `json:"local"`
}

type genesisGatewayProfileBranch struct {
	Gateway map[string]genesisGatewayProfile `json:"gateway"`
}

type genesisGatewayProfile struct {
	ProfileID    string `json:"profile_id"`
	ModelID      string `json:"model_id"`
	GatewayRoute string `json:"gateway_route"`
}
