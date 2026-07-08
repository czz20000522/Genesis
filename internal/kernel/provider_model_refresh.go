package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const defaultProviderModelRefreshTimeout = 10 * time.Second

type ProviderModelRefreshRequest struct {
	ConfigRoot          string
	CredentialStoreRoot string
	ModelRole           string
	ModelProfileID      string
	Timeout             time.Duration
	SecretResolver      func(ref string, storeRoot string) (string, error)
	HTTPClient          *http.Client
	Clock               func() time.Time
}

type ProviderModelRefreshResult struct {
	Readiness       string         `json:"readiness"`
	ReadinessReason string         `json:"readiness_reason,omitempty"`
	ModelRole       string         `json:"model_role,omitempty"`
	ProfileID       string         `json:"profile_id,omitempty"`
	CatalogID       string         `json:"catalog_id,omitempty"`
	ModelCount      int            `json:"model_count"`
	Models          []string       `json:"models,omitempty"`
	RefreshedAt     string         `json:"refreshed_at,omitempty"`
	Provider        ProviderStatus `json:"provider"`
}

type providerModelFetchFailure struct {
	reason string
}

func (e providerModelFetchFailure) Error() string {
	return e.reason
}

func RefreshProviderModelCatalog(req ProviderModelRefreshRequest) ProviderModelRefreshResult {
	modelRole := strings.TrimSpace(req.ModelRole)
	if modelRole == "" {
		modelRole = DefaultModelRole
	}
	selected, err := loadSelectedGatewayConfig(GenesisModelConfigRequest{
		ConfigRoot:     req.ConfigRoot,
		ModelRole:      modelRole,
		ModelProfileID: req.ModelProfileID,
	})
	if err != nil {
		return providerModelRefreshFailure(modelRole, "", "", ProviderConfigReason(err))
	}
	profileID := strings.TrimSpace(firstNonEmpty(req.ModelProfileID, selected.profile.ProfileID))
	catalogID := providerModelCatalogID(selected.profile)

	resolved, err := ResolveProviderConfigFromGenesis(GenesisModelConfigRequest{
		ConfigRoot:          req.ConfigRoot,
		CredentialStoreRoot: req.CredentialStoreRoot,
		ModelRole:           modelRole,
		ModelProfileID:      req.ModelProfileID,
		SecretResolver:      req.SecretResolver,
	})
	if err != nil {
		return providerModelRefreshFailure(modelRole, profileID, catalogID, ProviderConfigReason(err))
	}
	if resolved.Kind != "openai-compatible" {
		return providerModelRefreshFailure(modelRole, profileID, catalogID, "provider_model_refresh_unsupported")
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = defaultProviderModelRefreshTimeout
	}
	client := req.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	models, err := fetchProviderModelIDs(ctx, client, resolved.OpenAICompatible.BaseURL, resolved.OpenAICompatible.APIKey)
	if err != nil {
		return providerModelRefreshFailure(modelRole, profileID, catalogID, providerModelRefreshReason(err))
	}
	if len(models) == 0 {
		return providerModelRefreshFailure(modelRole, profileID, catalogID, "provider_models_empty")
	}

	now := time.Now().UTC()
	if req.Clock != nil {
		now = req.Clock().UTC()
	}
	configPath := filepath.Join(resolveGenesisConfigRoot(req.ConfigRoot), "models.json")
	config, err := readGenesisModelsConfig(configPath)
	if err != nil {
		return providerModelRefreshFailure(modelRole, profileID, catalogID, ProviderConfigReason(err))
	}
	if config.ProviderModelCatalogs == nil {
		config.ProviderModelCatalogs = map[string]genesisProviderModelCatalog{}
	}
	refreshedAt := now.Format(time.RFC3339)
	config.ProviderModelCatalogs[catalogID] = genesisProviderModelCatalog{
		Route:       catalogID,
		Protocol:    modelGatewayProtocolChatCompletions,
		Models:      append([]string(nil), models...),
		RefreshedAt: refreshedAt,
		Source:      "models_endpoint",
	}
	if err := writeGenesisModelsConfig(configPath, config); err != nil {
		return providerModelRefreshFailure(modelRole, profileID, catalogID, "provider_config_unwritable")
	}
	return ProviderModelRefreshResult{
		Readiness:   ReadinessReady,
		ModelRole:   modelRole,
		ProfileID:   profileID,
		CatalogID:   catalogID,
		ModelCount:  len(models),
		Models:      append([]string(nil), models...),
		RefreshedAt: refreshedAt,
		Provider:    ProviderStatus{Name: "openai-compatible", Readiness: ReadinessReady},
	}
}

func providerModelCatalogID(profile genesisGatewayProfile) string {
	return firstNonEmpty(profile.GatewayRoute, profile.ProfileID)
}

func providerModelRefreshFailure(modelRole string, profileID string, catalogID string, reason string) ProviderModelRefreshResult {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "provider_models_request_failed"
	}
	return ProviderModelRefreshResult{
		Readiness:       ReadinessNotReady,
		ReadinessReason: reason,
		ModelRole:       modelRole,
		ProfileID:       strings.TrimSpace(profileID),
		CatalogID:       strings.TrimSpace(catalogID),
		Provider:        ProviderStatus{Name: "provider-model-refresh", Readiness: ReadinessNotReady, ReadinessReason: reason},
	}
}

func fetchProviderModelIDs(ctx context.Context, client *http.Client, baseURL string, apiKey string) ([]string, error) {
	candidates, err := providerModelListURLs(baseURL)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, endpoint := range candidates {
		models, err := fetchProviderModelIDsFromURL(ctx, client, endpoint, apiKey)
		if err == nil {
			return models, nil
		}
		lastErr = err
		if providerModelRefreshReason(err) != "provider_models_endpoint_missing" {
			break
		}
	}
	return nil, lastErr
}

func fetchProviderModelIDsFromURL(ctx context.Context, client *http.Client, endpoint string, apiKey string) ([]string, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, providerModelFetchFailure{reason: "provider_models_request_failed"}
	}
	httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	httpReq.Header.Set("Accept", "application/json")
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, providerModelFetchFailure{reason: "provider_models_request_failed"}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return nil, providerModelFetchFailure{reason: "provider_models_request_failed"}
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, providerModelFetchFailure{reason: "provider_models_auth_failed"}
	}
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
		return nil, providerModelFetchFailure{reason: "provider_models_endpoint_missing"}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, providerModelFetchFailure{reason: "provider_models_request_failed"}
	}
	var decoded struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, providerModelFetchFailure{reason: "provider_models_decode_failed"}
	}
	return normalizedProviderModelIDs(decoded.Data), nil
}

func normalizedProviderModelIDs(data []struct {
	ID string `json:"id"`
}) []string {
	seen := map[string]bool{}
	for _, item := range data {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		seen[id] = true
	}
	models := make([]string, 0, len(seen))
	for id := range seen {
		models = append(models, id)
	}
	sort.Strings(models)
	return models
}

func providerModelListURLs(baseURL string) ([]string, error) {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return nil, providerModelFetchFailure{reason: "provider_base_url_missing"}
	}
	if strings.HasSuffix(base, "/models") {
		return []string{base}, nil
	}
	if endsWithProviderModelVersionSegment(base) {
		return []string{base + "/models"}, nil
	}
	return []string{base + "/models", base + "/v1/models"}, nil
}

func endsWithProviderModelVersionSegment(raw string) bool {
	last := raw
	if i := strings.LastIndex(raw, "/"); i >= 0 {
		last = raw[i+1:]
	}
	if len(last) < 2 || last[0] != 'v' {
		return false
	}
	for _, r := range last[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func providerModelRefreshReason(err error) string {
	var failure providerModelFetchFailure
	if errors.As(err, &failure) {
		return failure.reason
	}
	return "provider_models_request_failed"
}
