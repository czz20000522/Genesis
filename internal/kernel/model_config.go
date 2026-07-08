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

const DefaultModelRole = "coordinator"

const (
	modelGatewayProtocolChatCompletions = "openai-chat-completions"
	modelGatewayProtocolProviderCommand = "provider_command"
)

var (
	ErrGenesisModelConfigMissing              = errors.New("genesis model config missing")
	ErrGenesisModelConfigInvalid              = errors.New("genesis model config invalid")
	ErrGenesisModelProfileMissing             = errors.New("genesis model profile missing")
	ErrGenesisModelGatewayMissing             = errors.New("genesis model gateway missing")
	ErrGenesisModelGatewayRouteMissing        = errors.New("genesis model gateway route missing")
	ErrGenesisModelProtocolUnsupported        = errors.New("genesis model gateway protocol unsupported")
	ErrGenesisModelProviderAdapterInvalid     = errors.New("genesis model provider adapter invalid")
	ErrGenesisModelCredentialMissing          = errors.New("genesis model credential missing")
	ErrGenesisModelCredentialUnsupported      = errors.New("genesis model credential unsupported")
	ErrGenesisModelProviderCommandEnvRejected = errors.New("genesis provider command environment rejected")
)

type GenesisModelConfigRequest struct {
	ConfigRoot          string
	CredentialStoreRoot string
	ModelRole           string
	ModelProfileID      string
	SecretResolver      func(ref string, storeRoot string) (string, error)
}

type ResolvedProviderConfig struct {
	Kind             string
	OpenAICompatible OpenAICompatibleConfig
	Command          ProviderCommandConfig
}

func ResolveOpenAICompatibleConfigFromGenesis(req GenesisModelConfigRequest) (OpenAICompatibleConfig, error) {
	resolved, err := ResolveProviderConfigFromGenesis(req)
	if err != nil {
		return OpenAICompatibleConfig{}, err
	}
	if resolved.Kind != "openai-compatible" {
		return OpenAICompatibleConfig{}, ErrGenesisModelProtocolUnsupported
	}
	return resolved.OpenAICompatible, nil
}

func ResolveProviderConfigFromGenesis(req GenesisModelConfigRequest) (ResolvedProviderConfig, error) {
	selected, err := loadSelectedGatewayConfig(req)
	if err != nil {
		return ResolvedProviderConfig{}, err
	}
	gateway := selected.gateway
	profile := selected.profile
	route := selected.route
	protocol := firstNonEmpty(route.Protocol, gateway.Protocol)
	timeout := firstPositiveFloat(route.RequestTimeoutSec, gateway.RequestTimeoutSec)
	switch protocol {
	case modelGatewayProtocolChatCompletions:
		baseURL := firstNonEmpty(route.BaseURL, gateway.BaseURL)
		credentialRef := firstNonEmpty(route.CredentialRef, gateway.CredentialRef)
		if strings.TrimSpace(baseURL) == "" || strings.TrimSpace(profile.ModelID) == "" {
			return ResolvedProviderConfig{}, ErrGenesisModelGatewayMissing
		}
		if !isLocalSecretCredentialRef(credentialRef) {
			return ResolvedProviderConfig{}, ErrGenesisModelCredentialUnsupported
		}
		resolver := req.SecretResolver
		if resolver == nil {
			resolver = ResolveLocalCredentialSecret
		}
		apiKey, err := resolver(credentialRef, req.CredentialStoreRoot)
		if err != nil {
			return ResolvedProviderConfig{}, fmt.Errorf("%w: %v", ErrGenesisModelCredentialMissing, err)
		}
		if strings.TrimSpace(apiKey) == "" {
			return ResolvedProviderConfig{}, ErrGenesisModelCredentialMissing
		}
		adapter := providerAdapterBindingFromProfile(profile, protocol)
		if err := validateProviderAdapterBinding(adapter); err != nil {
			return ResolvedProviderConfig{}, err
		}
		return ResolvedProviderConfig{
			Kind: "openai-compatible",
			OpenAICompatible: OpenAICompatibleConfig{
				BaseURL:        baseURL,
				APIKey:         apiKey,
				Model:          profile.ModelID,
				Adapter:        adapter,
				RequestTimeout: durationSeconds(timeout),
			},
		}, nil
	case modelGatewayProtocolProviderCommand:
		command := firstNonEmpty(route.Command, gateway.Command)
		if strings.TrimSpace(command) == "" || strings.TrimSpace(profile.ModelID) == "" {
			return ResolvedProviderConfig{}, ErrGenesisModelGatewayMissing
		}
		env := firstStringSlice(route.Env, gateway.Env)
		if err := validateProviderCommandEnv(env); err != nil {
			return ResolvedProviderConfig{}, fmt.Errorf("%w: %v", ErrGenesisModelProviderCommandEnvRejected, err)
		}
		return ResolvedProviderConfig{
			Kind: "provider_command",
			Command: ProviderCommandConfig{
				Command:        command,
				Args:           firstStringSlice(route.Args, gateway.Args),
				Env:            env,
				WorkingDir:     firstNonEmpty(route.WorkingDir, gateway.WorkingDir),
				Model:          profile.ModelID,
				RequestTimeout: durationSeconds(timeout),
			},
		}, nil
	default:
		return ResolvedProviderConfig{}, ErrGenesisModelProtocolUnsupported
	}
}

func loadSelectedGatewayConfig(req GenesisModelConfigRequest) (selectedGatewayConfig, error) {
	configPath := filepath.Join(resolveGenesisConfigRoot(req.ConfigRoot), "models.json")
	payload, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return selectedGatewayConfig{}, ErrGenesisModelConfigMissing
		}
		return selectedGatewayConfig{}, fmt.Errorf("%w: %v", ErrGenesisModelConfigInvalid, err)
	}
	var config genesisModelsConfig
	if err := json.Unmarshal(payload, &config); err != nil {
		return selectedGatewayConfig{}, fmt.Errorf("%w: %v", ErrGenesisModelConfigInvalid, err)
	}
	gateway := config.ModelGateway
	if strings.TrimSpace(gateway.BaseURL) == "" && strings.TrimSpace(gateway.Command) == "" && len(gateway.Routes) == 0 {
		return selectedGatewayConfig{}, ErrGenesisModelGatewayMissing
	}

	profile, err := selectGatewayProfile(config, req)
	if err != nil {
		return selectedGatewayConfig{}, err
	}
	route, err := selectGatewayRoute(gateway, profile.GatewayRoute)
	if err != nil {
		return selectedGatewayConfig{}, err
	}
	return selectedGatewayConfig{
		gateway: gateway,
		profile: profile,
		route:   route,
	}, nil
}

func ProviderConfigReason(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrGenesisModelConfigInvalid):
		return "provider_config_invalid"
	case errors.Is(err, ErrGenesisModelConfigMissing):
		return "provider_config_missing"
	case errors.Is(err, ErrGenesisModelProfileMissing):
		return "provider_profile_missing"
	case errors.Is(err, ErrGenesisModelGatewayRouteMissing):
		return "provider_route_missing"
	case errors.Is(err, ErrGenesisModelProtocolUnsupported):
		return "provider_protocol_unsupported"
	case errors.Is(err, ErrGenesisModelProviderAdapterInvalid):
		return "provider_adapter_invalid"
	case errors.Is(err, ErrGenesisModelCredentialUnsupported):
		return "provider_credential_unsupported"
	case errors.Is(err, ErrGenesisModelCredentialMissing):
		return "provider_credential_missing"
	case errors.Is(err, ErrGenesisModelProviderCommandEnvRejected):
		return "provider_command_env_secret_rejected"
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

func firstStringSlice(values ...[]string) []string {
	for _, value := range values {
		if len(value) > 0 {
			return append([]string(nil), value...)
		}
	}
	return nil
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
	ModelGateway               genesisModelGateway                    `json:"model_gateway"`
	ActiveModelProfileBindings map[string]string                      `json:"active_model_profile_bindings"`
	ModelProfiles              genesisModelProfiles                   `json:"model_profiles"`
	ProviderModelCatalogs      map[string]genesisProviderModelCatalog `json:"provider_model_catalogs,omitempty"`
}

type genesisModelGateway struct {
	BaseURL           string                         `json:"base_url"`
	CredentialRef     string                         `json:"credential_ref"`
	Protocol          string                         `json:"protocol"`
	Command           string                         `json:"command"`
	Args              []string                       `json:"args"`
	Env               []string                       `json:"env"`
	WorkingDir        string                         `json:"working_dir"`
	RequestTimeoutSec float64                        `json:"request_timeout_sec"`
	Routes            map[string]genesisGatewayRoute `json:"routes"`
}

type genesisGatewayRoute struct {
	BaseURL           string   `json:"base_url"`
	CredentialRef     string   `json:"credential_ref"`
	Protocol          string   `json:"protocol"`
	Command           string   `json:"command"`
	Args              []string `json:"args"`
	Env               []string `json:"env"`
	WorkingDir        string   `json:"working_dir"`
	RequestTimeoutSec float64  `json:"request_timeout_sec"`
}

type selectedGatewayConfig struct {
	gateway genesisModelGateway
	profile genesisGatewayProfile
	route   genesisGatewayRoute
}

type genesisModelProfiles struct {
	Cloud genesisGatewayProfileBranch `json:"cloud"`
	Local genesisGatewayProfileBranch `json:"local"`
}

type genesisGatewayProfileBranch struct {
	Gateway map[string]genesisGatewayProfile `json:"gateway"`
}

type genesisGatewayProfile struct {
	ProfileID                string `json:"profile_id"`
	ModelID                  string `json:"model_id"`
	GatewayRoute             string `json:"gateway_route"`
	ContextWindowTokens      int    `json:"context_window_tokens,omitempty"`
	ProviderAdapterID        string `json:"provider_adapter_id,omitempty"`
	ProviderAdapterProfileID string `json:"provider_adapter_profile_id,omitempty"`
	HiddenReasoningPolicy    string `json:"hidden_reasoning_policy,omitempty"`
}

type genesisProviderModelCatalog struct {
	Route       string   `json:"route"`
	Protocol    string   `json:"protocol"`
	Models      []string `json:"models"`
	RefreshedAt string   `json:"refreshed_at"`
	Source      string   `json:"source"`
}
