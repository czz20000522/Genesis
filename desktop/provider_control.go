package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"genesis/localconfig"
)

const (
	providerActivationOwnedKernelRestarted          = "owned_kernel_restarted"
	providerActivationExternalKernelRestartRequired = "external_kernel_restart_required"
	providerActivationRestartFailed                 = "owned_kernel_restart_failed"
	providerActivationBlockedActiveTurn             = "kernel_restart_blocked_active_turn"
	providerActivationInProgressReason              = "provider_activation_in_progress"
)

type desktopProviderControlConfig struct {
	ConfigRoot          string
	CredentialStoreRoot string
	secretProtector     func([]byte) ([]byte, error)
}

type ProviderProfileProjection struct {
	ProfileID         string   `json:"profile_id"`
	ModelID           string   `json:"model_id"`
	GatewayRoute      string   `json:"gateway_route"`
	Protocol          string   `json:"protocol"`
	ProviderAdapterID string   `json:"provider_adapter_id,omitempty"`
	Roles             []string `json:"roles"`
	CredentialPresent bool     `json:"credential_present"`
}

type ProviderProfilesProjection struct {
	Profiles     []ProviderProfileProjection `json:"profiles"`
	RoleBindings map[string]string           `json:"role_bindings"`
}

type ProviderRoleBindingProjection struct {
	ModelRole         string `json:"model_role"`
	ProfileID         string `json:"profile_id"`
	PreviousProfileID string `json:"previous_profile_id,omitempty"`
}

type ProviderActivationProjection struct {
	Status  string                        `json:"status"`
	Binding ProviderRoleBindingProjection `json:"binding"`
	Sidecar SidecarStatus                 `json:"sidecar"`
}

type ProviderCredentialRotationProjection struct {
	ProfileID         string `json:"profile_id"`
	CredentialPresent bool   `json:"credential_present"`
}

type FirstRunDeepSeekProjection struct {
	ProfileID         string `json:"profile_id"`
	CredentialPresent bool   `json:"credential_present"`
}

type ProviderImportProjection struct {
	RouteID         string   `json:"route_id"`
	ProfileIDs      []string `json:"profile_ids"`
	DiscoveryReason string   `json:"discovery_reason,omitempty"`
}

func (a *App) ImportProviderTemplate(templateID string, apiKey string, baseURL string, modelID string) (ProviderImportProjection, error) {
	if a == nil || a.client == nil {
		return ProviderImportProjection{}, errors.New("desktop app is unavailable")
	}
	route, err := localconfig.ImportProviderTemplateRoute(localconfig.ProviderTemplateRouteImportRequest{
		ConfigRoot: a.providerControl.ConfigRoot, CredentialStoreRoot: a.providerControl.CredentialStoreRoot,
		TemplateID: strings.TrimSpace(templateID), APIKey: strings.TrimSpace(apiKey), BaseURL: strings.TrimSpace(baseURL), ExplicitModelID: strings.TrimSpace(modelID), Protector: a.providerControl.secretProtector,
	})
	if err != nil {
		return ProviderImportProjection{}, err
	}
	ctx, cancel := a.requestContext()
	defer cancel()
	payload, err := a.client.RequestJSON(ctx, http.MethodPost, "/providers/"+url.PathEscape(route.RouteID)+"/models/discover", true, []byte("{}"))
	if err != nil {
		return ProviderImportProjection{RouteID: route.RouteID, DiscoveryReason: "provider_models_request_failed"}, nil
	}
	if strings.TrimSpace(stringFromMap(payload, "readiness")) != "ready" {
		return ProviderImportProjection{RouteID: route.RouteID, DiscoveryReason: strings.TrimSpace(stringFromMap(payload, "readiness_reason"))}, nil
	}
	models := stringSliceFromMap(payload, "models")
	if strings.TrimSpace(modelID) != "" {
		models = []string{strings.TrimSpace(modelID)}
	}
	materialized, err := localconfig.MaterializeProviderTemplateModels(localconfig.ProviderTemplateModelsRequest{ConfigRoot: a.providerControl.ConfigRoot, RouteID: route.RouteID, Models: models})
	if err != nil {
		return ProviderImportProjection{}, err
	}
	return ProviderImportProjection{RouteID: materialized.RouteID, ProfileIDs: materialized.ProfileIDs}, nil
}

type ProviderVerificationProjection struct {
	Readiness       string `json:"readiness"`
	ReadinessReason string `json:"readiness_reason,omitempty"`
	ModelRole       string `json:"model_role,omitempty"`
	ProfileID       string `json:"profile_id,omitempty"`
	Model           string `json:"model,omitempty"`
}

func (a *App) ProviderProfiles() (ProviderProfilesProjection, error) {
	if a == nil {
		return ProviderProfilesProjection{}, errors.New("desktop app is unavailable")
	}
	snapshot, err := localconfig.Snapshot(localconfig.SnapshotRequest{
		ConfigRoot:          a.providerControl.ConfigRoot,
		CredentialStoreRoot: a.providerControl.CredentialStoreRoot,
	})
	if errors.Is(err, localconfig.ErrConfigMissing) {
		return ProviderProfilesProjection{Profiles: []ProviderProfileProjection{}, RoleBindings: map[string]string{}}, nil
	}
	if err != nil {
		return ProviderProfilesProjection{}, err
	}
	result := ProviderProfilesProjection{RoleBindings: snapshot.RoleBindings, Profiles: make([]ProviderProfileProjection, 0, len(snapshot.Profiles))}
	for _, profile := range snapshot.Profiles {
		result.Profiles = append(result.Profiles, ProviderProfileProjection{
			ProfileID:         profile.ProfileID,
			ModelID:           profile.ModelID,
			GatewayRoute:      profile.GatewayRoute,
			Protocol:          profile.Protocol,
			ProviderAdapterID: profile.ProviderAdapterID,
			Roles:             append([]string(nil), profile.Roles...),
			CredentialPresent: profile.CredentialPresent,
		})
	}
	return result, nil
}

func (a *App) ApplyProviderRole(modelRole string, profileID string) (ProviderActivationProjection, error) {
	if a == nil {
		return ProviderActivationProjection{}, errors.New("desktop app is unavailable")
	}
	if err := a.beginProviderActivation(); err != nil {
		return ProviderActivationProjection{}, err
	}
	defer a.endProviderActivation()
	binding, err := localconfig.BindRole(localconfig.BindRoleRequest{
		ConfigRoot: a.providerControl.ConfigRoot,
		ModelRole:  strings.TrimSpace(modelRole),
		ProfileID:  strings.TrimSpace(profileID),
	})
	if err != nil {
		return ProviderActivationProjection{}, err
	}
	result := ProviderActivationProjection{Binding: ProviderRoleBindingProjection{
		ModelRole:         binding.ModelRole,
		ProfileID:         binding.ProfileID,
		PreviousProfileID: binding.PreviousProfileID,
	}}
	if a.supervisor == nil || a.supervisor.KernelStatus().Ownership == serviceOwnershipExternal {
		result.Status = providerActivationExternalKernelRestartRequired
		if a.supervisor != nil {
			result.Sidecar = a.supervisor.KernelStatus()
		}
		return result, nil
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	result.Sidecar = a.supervisor.RestartOwned(ctx)
	a.config.Sidecar = result.Sidecar
	if result.Sidecar.Readiness != "ready" {
		result.Status = providerActivationRestartFailed
		return result, nil
	}
	result.Status = providerActivationOwnedKernelRestarted
	return result, nil
}

func (a *App) RotateProviderCredential(profileID string, secret string) (ProviderCredentialRotationProjection, error) {
	if a == nil {
		return ProviderCredentialRotationProjection{}, errors.New("desktop app is unavailable")
	}
	result, err := localconfig.RotateProfileCredential(localconfig.RotateProfileCredentialRequest{
		ConfigRoot:          a.providerControl.ConfigRoot,
		CredentialStoreRoot: a.providerControl.CredentialStoreRoot,
		ProfileID:           strings.TrimSpace(profileID),
		Secret:              secret,
		Protector:           a.providerControl.secretProtector,
	})
	if err != nil {
		return ProviderCredentialRotationProjection{}, err
	}
	return ProviderCredentialRotationProjection{ProfileID: result.ProfileID, CredentialPresent: true}, nil
}

func (a *App) SetupDeepSeekFlash(apiKey string) (FirstRunDeepSeekProjection, error) {
	if a == nil {
		return FirstRunDeepSeekProjection{}, errors.New("desktop app is unavailable")
	}
	result, err := localconfig.SetupDeepSeekFlash(localconfig.DeepSeekFlashSetupRequest{
		ConfigRoot:          a.providerControl.ConfigRoot,
		CredentialStoreRoot: a.providerControl.CredentialStoreRoot,
		APIKey:              strings.TrimSpace(apiKey),
		Protector:           a.providerControl.secretProtector,
	})
	if err != nil {
		return FirstRunDeepSeekProjection{}, err
	}
	return FirstRunDeepSeekProjection{ProfileID: result.ProfileID, CredentialPresent: result.CredentialPresent}, nil
}

func (a *App) VerifyProvider(modelRole string, profileID string) (ProviderVerificationProjection, error) {
	if a == nil || a.client == nil {
		return ProviderVerificationProjection{}, errors.New("desktop kernel client is unavailable")
	}
	ctx, cancel := a.turnRequestContext()
	defer cancel()
	body, _ := json.Marshal(map[string]string{
		"model_role": strings.TrimSpace(modelRole),
		"profile_id": strings.TrimSpace(profileID),
	})
	payload, err := a.client.RequestJSON(ctx, "POST", "/providers/verify", true, body)
	if err != nil {
		return ProviderVerificationProjection{}, err
	}
	return ProviderVerificationProjection{
		Readiness:       strings.TrimSpace(stringFromMap(payload, "readiness")),
		ReadinessReason: strings.TrimSpace(stringFromMap(payload, "readiness_reason")),
		ModelRole:       strings.TrimSpace(stringFromMap(payload, "model_role")),
		ProfileID:       strings.TrimSpace(stringFromMap(payload, "profile_id")),
		Model:           strings.TrimSpace(stringFromMap(payload, "model")),
	}, nil
}

func stringFromMap(payload map[string]any, key string) string {
	value, _ := payload[key].(string)
	return value
}

func stringSliceFromMap(payload map[string]any, key string) []string {
	items, _ := payload[key].([]any)
	result := make([]string, 0, len(items))
	for _, item := range items {
		if value, ok := item.(string); ok && strings.TrimSpace(value) != "" {
			result = append(result, strings.TrimSpace(value))
		}
	}
	return result
}

func (a *App) beginProviderActivation() error {
	a.desktopTurnMu.Lock()
	defer a.desktopTurnMu.Unlock()
	if a.activeDesktopTurns > 0 {
		return errors.New(providerActivationBlockedActiveTurn)
	}
	if a.providerActivationInProgress {
		return errors.New(providerActivationInProgressReason)
	}
	a.providerActivationInProgress = true
	return nil
}

func (a *App) endProviderActivation() {
	a.desktopTurnMu.Lock()
	defer a.desktopTurnMu.Unlock()
	a.providerActivationInProgress = false
}
