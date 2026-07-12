package localconfig

import (
	"errors"
	"net"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	ProviderTemplateDeepSeek   = "deepseek"
	ProviderTemplateOpenAI     = "openai"
	ProviderTemplateOpenCode   = "opencode-go"
	ProviderTemplateLocalLlama = "local-llama-cpp"
	ProviderTemplateAdvanced   = "openai-compatible"
)

type ProviderTemplate struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Protocol           string `json:"protocol"`
	BaseURL            string `json:"base_url,omitempty"`
	CredentialRef      string `json:"-"`
	AdapterID          string `json:"adapter_id,omitempty"`
	RequiresCredential bool   `json:"requires_credential"`
	SupportsDiscovery  bool   `json:"supports_discovery"`
	Advanced           bool   `json:"advanced"`
	DefaultModelID     string `json:"default_model_id,omitempty"`
}

type ProviderTemplateRouteImportRequest struct {
	ConfigRoot          string
	CredentialStoreRoot string
	TemplateID          string
	APIKey              string
	BaseURL             string
	ExplicitModelID     string
	Protector           func([]byte) ([]byte, error)
}

type ProviderTemplateRouteImportResult struct {
	RouteID           string `json:"route_id"`
	TemplateID        string `json:"template_id"`
	CredentialPresent bool   `json:"credential_present"`
}

type ProviderTemplateModelsRequest struct {
	ConfigRoot string
	RouteID    string
	Models     []string
}

type ProviderTemplateModelsResult struct {
	RouteID    string   `json:"route_id"`
	ProfileIDs []string `json:"profile_ids"`
}

type LocalLlamaProfileImportRequest struct {
	ConfigRoot  string
	ModelID     string
	BaseURL     string
	Command     string
	AdapterPath string
	WorkingDir  string
}

func ProviderTemplates() []ProviderTemplate {
	return []ProviderTemplate{
		{ID: ProviderTemplateDeepSeek, Name: "DeepSeek", Protocol: openAIChatCompletionsProtocol, BaseURL: "https://api.deepseek.com", CredentialRef: "secret://models/deepseek/local", AdapterID: "deepseek", RequiresCredential: true, SupportsDiscovery: true, DefaultModelID: "deepseek-v4-flash"},
		{ID: ProviderTemplateOpenAI, Name: "OpenAI", Protocol: openAIChatCompletionsProtocol, BaseURL: "https://api.openai.com/v1", CredentialRef: "secret://models/openai/default", AdapterID: "openai", RequiresCredential: true, SupportsDiscovery: true},
		{ID: ProviderTemplateOpenCode, Name: "OpenCode Go", Protocol: openAIChatCompletionsProtocol, BaseURL: "https://opencode.ai/zen/go/v1", CredentialRef: "secret://models/opencode/go", AdapterID: "opencode", RequiresCredential: true, SupportsDiscovery: true},
		{ID: ProviderTemplateLocalLlama, Name: "本地 llama.cpp", Protocol: "provider_command", AdapterID: "llama.cpp"},
		{ID: ProviderTemplateAdvanced, Name: "OpenAI 兼容", Protocol: openAIChatCompletionsProtocol, CredentialRef: "secret://models/openai-compatible/default", AdapterID: "openai-compatible", RequiresCredential: true, SupportsDiscovery: true, Advanced: true},
	}
}

func ProviderTemplateByID(id string) (ProviderTemplate, bool) {
	id = strings.TrimSpace(id)
	for _, template := range ProviderTemplates() {
		if template.ID == id {
			return template, true
		}
	}
	return ProviderTemplate{}, false
}

func ImportProviderTemplateRoute(req ProviderTemplateRouteImportRequest) (ProviderTemplateRouteImportResult, error) {
	template, ok := ProviderTemplateByID(req.TemplateID)
	if !ok || template.ID == ProviderTemplateLocalLlama {
		return ProviderTemplateRouteImportResult{}, errors.New("provider_template_unsupported")
	}
	baseURL := strings.TrimSpace(template.BaseURL)
	if template.Advanced {
		baseURL = strings.TrimSpace(req.BaseURL)
	}
	if baseURL == "" {
		return ProviderTemplateRouteImportResult{}, errors.New("provider_base_url_missing")
	}
	if !safeProviderBaseURL(baseURL) {
		return ProviderTemplateRouteImportResult{}, errors.New("provider_base_url_insecure")
	}
	apiKey := strings.TrimSpace(req.APIKey)
	if template.RequiresCredential && apiKey == "" {
		return ProviderTemplateRouteImportResult{}, ErrCredentialMissing
	}
	secret, err := WriteCredentialSecret(CredentialSecretWriteRequest{
		CredentialRef: template.CredentialRef,
		Secret:        apiKey,
		StoreRoot:     req.CredentialStoreRoot,
		Protector:     req.Protector,
	})
	if err != nil {
		return ProviderTemplateRouteImportResult{}, err
	}
	path := ConfigPath(req.ConfigRoot)
	config, err := ReadModels(path)
	if errors.Is(err, ErrConfigMissing) {
		config = ModelsConfig{}
	} else if err != nil {
		return ProviderTemplateRouteImportResult{}, err
	}
	if config.ModelGateway.Routes == nil {
		config.ModelGateway.Routes = map[string]GatewayRoute{}
	}
	config.ModelGateway.Routes[template.ID] = GatewayRoute{BaseURL: baseURL, CredentialRef: secret.CredentialRef, Protocol: template.Protocol, RequestTimeoutSec: (60 * time.Second).Seconds()}
	if config.ModelGateway.Protocol == "" {
		config.ModelGateway.Protocol = template.Protocol
	}
	if err := WriteModels(path, config); err != nil {
		return ProviderTemplateRouteImportResult{}, err
	}
	return ProviderTemplateRouteImportResult{RouteID: template.ID, TemplateID: template.ID, CredentialPresent: true}, nil
}

func safeProviderBaseURL(raw string) bool {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(raw))
	if err != nil || parsed.Hostname() == "" {
		return false
	}
	if strings.EqualFold(parsed.Scheme, "https") {
		return true
	}
	if !strings.EqualFold(parsed.Scheme, "http") {
		return false
	}
	host := strings.Trim(strings.ToLower(parsed.Hostname()), "[]")
	if host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

func MaterializeProviderTemplateModels(req ProviderTemplateModelsRequest) (ProviderTemplateModelsResult, error) {
	template, ok := ProviderTemplateByID(req.RouteID)
	if !ok || template.ID == ProviderTemplateLocalLlama {
		return ProviderTemplateModelsResult{}, errors.New("provider_template_unsupported")
	}
	path := ConfigPath(req.ConfigRoot)
	config, err := ReadModels(path)
	if err != nil {
		return ProviderTemplateModelsResult{}, err
	}
	if _, ok := config.ModelGateway.Routes[template.ID]; !ok {
		return ProviderTemplateModelsResult{}, ErrConfigInvalid
	}
	models := normalizedTemplateModels(req.Models)
	if len(models) == 0 && template.DefaultModelID != "" {
		models = []string{template.DefaultModelID}
	}
	if len(models) == 0 {
		return ProviderTemplateModelsResult{}, errors.New("provider_models_empty")
	}
	if config.ModelProfiles.Cloud.Gateway == nil {
		config.ModelProfiles.Cloud.Gateway = map[string]GatewayProfile{}
	}
	profiles := make([]string, 0, len(models))
	for _, modelID := range models {
		profileID := template.ID + "-" + refToken(modelID)
		config.ModelProfiles.Cloud.Gateway[profileID] = GatewayProfile{ProfileID: profileID, ModelID: modelID, GatewayRoute: template.ID, ProviderAdapterID: template.AdapterID, ProviderAdapterProfileID: modelID}
		profiles = append(profiles, profileID)
	}
	if err := WriteModels(path, config); err != nil {
		return ProviderTemplateModelsResult{}, err
	}
	return ProviderTemplateModelsResult{RouteID: template.ID, ProfileIDs: profiles}, nil
}

func normalizedTemplateModels(models []string) []string {
	seen := map[string]bool{}
	for _, model := range models {
		if model = strings.TrimSpace(model); model != "" {
			seen[model] = true
		}
	}
	result := make([]string, 0, len(seen))
	for model := range seen {
		result = append(result, model)
	}
	sort.Strings(result)
	return result
}

func RepointLocalProviderAdapter(configRoot string, adapterPath string) error {
	adapterPath = strings.TrimSpace(adapterPath)
	if adapterPath == "" {
		return nil
	}
	path := ConfigPath(configRoot)
	config, err := ReadModels(path)
	if errors.Is(err, ErrConfigMissing) {
		return nil
	}
	if err != nil {
		return err
	}
	changed := false
	for routeID, route := range config.ModelGateway.Routes {
		if route.Protocol != "provider_command" {
			continue
		}
		for index, arg := range route.Args {
			if strings.EqualFold(filepath.Base(strings.TrimSpace(arg)), "llama_cpp_provider_command.py") && strings.TrimSpace(arg) != adapterPath {
				route.Args[index] = adapterPath
				config.ModelGateway.Routes[routeID] = route
				changed = true
			}
		}
	}
	if !changed {
		return nil
	}
	return WriteModels(path, config)
}

func ImportLocalLlamaProfile(req LocalLlamaProfileImportRequest) (ProviderTemplateModelsResult, error) {
	modelID := strings.TrimSpace(req.ModelID)
	baseURL := strings.TrimSpace(req.BaseURL)
	command := strings.TrimSpace(req.Command)
	adapterPath := strings.TrimSpace(req.AdapterPath)
	if modelID == "" || baseURL == "" || command == "" || adapterPath == "" {
		return ProviderTemplateModelsResult{}, errors.New("local_model_configuration_required")
	}
	path := ConfigPath(req.ConfigRoot)
	config, err := ReadModels(path)
	if errors.Is(err, ErrConfigMissing) {
		config = ModelsConfig{}
	} else if err != nil {
		return ProviderTemplateModelsResult{}, err
	}
	if config.ModelGateway.Routes == nil {
		config.ModelGateway.Routes = map[string]GatewayRoute{}
	}
	config.ModelGateway.Routes[ProviderTemplateLocalLlama] = GatewayRoute{Protocol: "provider_command", Command: command, Args: []string{"-3", adapterPath, "--base-url", baseURL, "--timeout-sec", "0"}, WorkingDir: strings.TrimSpace(req.WorkingDir), AllowUnboundedRequest: true}
	if config.ModelProfiles.Local.Gateway == nil {
		config.ModelProfiles.Local.Gateway = map[string]GatewayProfile{}
	}
	profileID := ProviderTemplateLocalLlama + "-" + refToken(modelID)
	config.ModelProfiles.Local.Gateway[profileID] = GatewayProfile{ProfileID: profileID, ModelID: modelID, GatewayRoute: ProviderTemplateLocalLlama, ProviderAdapterID: "llama.cpp", ProviderAdapterProfileID: refToken(modelID)}
	if err := WriteModels(path, config); err != nil {
		return ProviderTemplateModelsResult{}, err
	}
	return ProviderTemplateModelsResult{RouteID: ProviderTemplateLocalLlama, ProfileIDs: []string{profileID}}, nil
}
