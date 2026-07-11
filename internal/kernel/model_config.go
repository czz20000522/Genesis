package kernel

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"genesis/localconfig"
)

const DefaultModelRole = "coordinator"

const (
	modelGatewayProtocolChatCompletions = "openai-chat-completions"
	modelGatewayProtocolProviderCommand = "provider_command"
	defaultWorkerRoleMaxParallel        = 6
	defaultParentMaxChildren            = 24
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
	ErrGenesisParentBindingMissing            = errors.New("genesis parent binding missing")
	ErrGenesisWorkerRoleBindingMissing        = errors.New("genesis worker role binding missing")
	ErrGenesisWorkerRoleBindingInvalid        = errors.New("genesis worker role binding invalid")
)

type GenesisModelConfigRequest struct {
	ConfigRoot          string
	CredentialStoreRoot string
	ModelRole           string
	ModelProfileID      string
	SecretResolver      func(ref string, storeRoot string) (string, error)
}

type ParentWorkerRuntimeRequest struct {
	ConfigRoot string
	ParentID   string
}

type ParentWorkerRuntimeProjection struct {
	Parent      ParentBindingProjection       `json:"parent"`
	WorkerRoles []WorkerRoleBindingProjection `json:"worker_roles"`
}

type ParentBindingProjection struct {
	ParentID           string   `json:"parent_id"`
	ProfileID          string   `json:"profile_id"`
	ModelID            string   `json:"model_id"`
	ProviderRoute      string   `json:"provider_route,omitempty"`
	DefaultWorkerRole  string   `json:"default_worker_role,omitempty"`
	AllowedWorkerRoles []string `json:"allowed_worker_roles,omitempty"`
	CanCreateWorkers   bool     `json:"can_create_workers"`
	MaxChildren        int      `json:"max_children"`
}

type WorkerRoleBindingProjection struct {
	RoleID              string   `json:"role_id"`
	ProfileID           string   `json:"profile_id"`
	ModelID             string   `json:"model_id"`
	ProviderRoute       string   `json:"provider_route,omitempty"`
	ContextWindowTokens int      `json:"context_window_tokens,omitempty"`
	ToolSet             []string `json:"tool_set,omitempty"`
	ContextPolicyRef    string   `json:"context_policy_ref,omitempty"`
	MaxParallel         int      `json:"max_parallel"`
	LeafOnly            bool     `json:"leaf_only"`
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
	allowUnboundedRequest := route.AllowUnboundedRequest || gateway.AllowUnboundedRequest
	if allowUnboundedRequest && protocol != modelGatewayProtocolProviderCommand {
		return ResolvedProviderConfig{}, ErrGenesisModelConfigInvalid
	}
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
		adapter := providerAdapterBindingFromProfile(profile, protocol)
		if err := validateProviderAdapterBinding(adapter); err != nil {
			return ResolvedProviderConfig{}, ErrGenesisModelProviderAdapterInvalid
		}
		return ResolvedProviderConfig{
			Kind: "provider_command",
			Command: ProviderCommandConfig{
				Command:               command,
				Args:                  firstStringSlice(route.Args, gateway.Args),
				Env:                   env,
				WorkingDir:            firstNonEmpty(route.WorkingDir, gateway.WorkingDir),
				Model:                 profile.ModelID,
				Adapter:               adapter,
				RequestTimeout:        durationSeconds(timeout),
				AllowUnboundedRequest: allowUnboundedRequest,
			},
		}, nil
	default:
		return ResolvedProviderConfig{}, ErrGenesisModelProtocolUnsupported
	}
}

func ResolveParentWorkerRuntimeFromGenesis(req ParentWorkerRuntimeRequest) (ParentWorkerRuntimeProjection, error) {
	configPath := filepath.Join(resolveGenesisConfigRoot(req.ConfigRoot), "models.json")
	config, err := readGenesisModelsConfig(configPath)
	if err != nil {
		return ParentWorkerRuntimeProjection{}, err
	}
	parentID := strings.TrimSpace(req.ParentID)
	if parentID == "" {
		parentID = DefaultModelRole
	}
	parent, ok := config.ParentWorkerRuntime.Parents[parentID]
	if !ok {
		return ParentWorkerRuntimeProjection{}, ErrGenesisParentBindingMissing
	}
	parent.ParentID = firstNonEmpty(parent.ParentID, parentID)
	parent.ProfileID = firstNonEmpty(parent.ProfileID, config.ActiveModelProfileBindings[parent.ParentID])
	parentProfile, err := gatewayProfileByID(config, parent.ProfileID)
	if err != nil {
		return ParentWorkerRuntimeProjection{}, err
	}
	allowedRoles := normalizeStringSet(parent.AllowedWorkerRoles)
	if len(allowedRoles) == 0 {
		return ParentWorkerRuntimeProjection{}, ErrGenesisWorkerRoleBindingMissing
	}
	defaultWorkerRole := strings.TrimSpace(parent.DefaultWorkerRole)
	if defaultWorkerRole != "" && !stringSliceContains(allowedRoles, defaultWorkerRole) {
		return ParentWorkerRuntimeProjection{}, ErrGenesisWorkerRoleBindingMissing
	}

	workers := make([]WorkerRoleBindingProjection, 0, len(allowedRoles))
	for _, roleID := range allowedRoles {
		worker, ok := config.ParentWorkerRuntime.WorkerRoles[roleID]
		if !ok {
			return ParentWorkerRuntimeProjection{}, ErrGenesisWorkerRoleBindingMissing
		}
		projection, err := projectWorkerRoleBinding(config, roleID, worker)
		if err != nil {
			return ParentWorkerRuntimeProjection{}, err
		}
		workers = append(workers, projection)
	}
	return ParentWorkerRuntimeProjection{
		Parent: ParentBindingProjection{
			ParentID:           parent.ParentID,
			ProfileID:          parent.ProfileID,
			ModelID:            strings.TrimSpace(parentProfile.ModelID),
			ProviderRoute:      strings.TrimSpace(parentProfile.GatewayRoute),
			DefaultWorkerRole:  defaultWorkerRole,
			AllowedWorkerRoles: allowedRoles,
			CanCreateWorkers:   parent.CanCreateWorkers,
			MaxChildren:        normalizedMaxChildren(parent.MaxChildren),
		},
		WorkerRoles: workers,
	}, nil
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
	return localconfig.ResolveConfigRoot(configRoot)
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
	return gatewayProfileByID(config, profileID)
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

func gatewayProfileByID(config genesisModelsConfig, profileID string) (genesisGatewayProfile, error) {
	profileID = strings.TrimSpace(profileID)
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

func projectWorkerRoleBinding(config genesisModelsConfig, roleID string, worker genesisWorkerRoleBinding) (WorkerRoleBindingProjection, error) {
	roleID = firstNonEmpty(worker.RoleID, roleID)
	if strings.TrimSpace(roleID) == "" || !worker.LeafOnly {
		return WorkerRoleBindingProjection{}, ErrGenesisWorkerRoleBindingInvalid
	}
	profile, err := gatewayProfileByID(config, worker.ProfileID)
	if err != nil {
		return WorkerRoleBindingProjection{}, err
	}
	toolSet, err := normalizeWorkerToolSet(worker.ToolSet)
	if err != nil {
		return WorkerRoleBindingProjection{}, err
	}
	return WorkerRoleBindingProjection{
		RoleID:              roleID,
		ProfileID:           firstNonEmpty(worker.ProfileID, profile.ProfileID),
		ModelID:             strings.TrimSpace(profile.ModelID),
		ProviderRoute:       strings.TrimSpace(profile.GatewayRoute),
		ContextWindowTokens: profile.ContextWindowTokens,
		ToolSet:             toolSet,
		ContextPolicyRef:    strings.TrimSpace(worker.ContextPolicyRef),
		MaxParallel:         normalizedMaxParallel(worker.MaxParallel),
		LeafOnly:            true,
	}, nil
}

func normalizeWorkerToolSet(tools []string) ([]string, error) {
	grant := normalizeCapabilityGrant(CapabilityGrant{ToolNames: tools})
	known := defaultKernelToolNameSet()
	for _, tool := range grant.ToolNames {
		if tool == "delegate_worker" {
			return nil, fmt.Errorf("%w: worker_role_must_be_leaf", ErrGenesisWorkerRoleBindingInvalid)
		}
		if _, ok := known[tool]; !ok {
			return nil, fmt.Errorf("%w: capability_grant_unknown_tool: %s", ErrGenesisWorkerRoleBindingInvalid, tool)
		}
	}
	return grant.ToolNames, nil
}

func defaultKernelToolNameSet() map[string]struct{} {
	tools := map[string]struct{}{}
	for _, tool := range defaultKernelTools() {
		name := strings.TrimSpace(tool.Spec.Name)
		if name != "" {
			tools[name] = struct{}{}
		}
	}
	return tools
}

func normalizeStringSet(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func stringSliceContains(values []string, needle string) bool {
	needle = strings.TrimSpace(needle)
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func normalizedMaxParallel(value int) int {
	if value <= 0 {
		return defaultWorkerRoleMaxParallel
	}
	return value
}

func normalizedMaxChildren(value int) int {
	if value <= 0 {
		return defaultParentMaxChildren
	}
	return value
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

type genesisModelsConfig = localconfig.ModelsConfig
type genesisModelGateway = localconfig.ModelGateway
type genesisGatewayRoute = localconfig.GatewayRoute

type selectedGatewayConfig struct {
	gateway genesisModelGateway
	profile genesisGatewayProfile
	route   genesisGatewayRoute
}

type genesisModelProfiles = localconfig.ModelProfiles
type genesisGatewayProfileBranch = localconfig.ProfileBranch
type genesisGatewayProfile = localconfig.GatewayProfile
type genesisProviderModelCatalog = localconfig.ProviderModelCatalog
type genesisParentWorkerRuntime = localconfig.ParentWorkerRuntime
type genesisParentBinding = localconfig.ParentBinding
type genesisWorkerRoleBinding = localconfig.WorkerRoleBinding
