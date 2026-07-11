// Package localconfig owns the user-editable Genesis Home model configuration.
// It deliberately contains no provider execution or ledger behavior so both
// the desktop and the kernel can consume the same local configuration truth.
package localconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const DefaultModelRole = "coordinator"

var (
	ErrConfigMissing  = errors.New("genesis model config missing")
	ErrConfigInvalid  = errors.New("genesis model config invalid")
	ErrProfileMissing = errors.New("genesis model profile missing")
)

var credentialRefPattern = regexp.MustCompile(`^secret://[a-z0-9][a-z0-9._/-]{0,190}$`)

type ModelsConfig struct {
	ModelGateway               ModelGateway                    `json:"model_gateway"`
	ActiveModelProfileBindings map[string]string               `json:"active_model_profile_bindings"`
	ModelProfiles              ModelProfiles                   `json:"model_profiles"`
	ProviderModelCatalogs      map[string]ProviderModelCatalog `json:"provider_model_catalogs,omitempty"`
	ParentWorkerRuntime        ParentWorkerRuntime             `json:"parent_worker_runtime,omitempty"`
}

type ModelGateway struct {
	BaseURL               string                  `json:"base_url"`
	CredentialRef         string                  `json:"credential_ref"`
	Protocol              string                  `json:"protocol"`
	Command               string                  `json:"command"`
	Args                  []string                `json:"args"`
	Env                   []string                `json:"env"`
	WorkingDir            string                  `json:"working_dir"`
	RequestTimeoutSec     float64                 `json:"request_timeout_sec"`
	AllowUnboundedRequest bool                    `json:"allow_unbounded_request"`
	Routes                map[string]GatewayRoute `json:"routes"`
}

type GatewayRoute struct {
	BaseURL               string   `json:"base_url"`
	CredentialRef         string   `json:"credential_ref"`
	Protocol              string   `json:"protocol"`
	Command               string   `json:"command"`
	Args                  []string `json:"args"`
	Env                   []string `json:"env"`
	WorkingDir            string   `json:"working_dir"`
	RequestTimeoutSec     float64  `json:"request_timeout_sec"`
	AllowUnboundedRequest bool     `json:"allow_unbounded_request"`
}

type ModelProfiles struct {
	Cloud ProfileBranch `json:"cloud"`
	Local ProfileBranch `json:"local"`
}

type ProfileBranch struct {
	Gateway map[string]GatewayProfile `json:"gateway"`
}

type GatewayProfile struct {
	ProfileID                string `json:"profile_id"`
	ModelID                  string `json:"model_id"`
	GatewayRoute             string `json:"gateway_route"`
	ContextWindowTokens      int    `json:"context_window_tokens,omitempty"`
	ProviderAdapterID        string `json:"provider_adapter_id,omitempty"`
	ProviderAdapterProfileID string `json:"provider_adapter_profile_id,omitempty"`
}

type ProviderModelCatalog struct {
	Route       string   `json:"route"`
	Protocol    string   `json:"protocol"`
	Models      []string `json:"models"`
	RefreshedAt string   `json:"refreshed_at"`
	Source      string   `json:"source"`
}

type ParentWorkerRuntime struct {
	Parents     map[string]ParentBinding     `json:"parents"`
	WorkerRoles map[string]WorkerRoleBinding `json:"worker_roles"`
}

type ParentBinding struct {
	ParentID           string   `json:"parent_id"`
	ProfileID          string   `json:"profile_id"`
	AllowedWorkerRoles []string `json:"allowed_worker_roles"`
	DefaultWorkerRole  string   `json:"default_worker_role"`
	CanCreateWorkers   bool     `json:"can_create_workers"`
}

type WorkerRoleBinding struct {
	RoleID           string   `json:"role_id"`
	ProfileID        string   `json:"profile_id"`
	ToolSet          []string `json:"tool_set"`
	ContextPolicyRef string   `json:"context_policy_ref"`
	MaxParallel      int      `json:"max_parallel"`
	LeafOnly         bool     `json:"leaf_only"`
}

type SnapshotRequest struct {
	ConfigRoot          string
	CredentialStoreRoot string
}

type SnapshotResult struct {
	Profiles     []ProfileProjection `json:"profiles"`
	RoleBindings map[string]string   `json:"role_bindings"`
}

type ProfileProjection struct {
	ProfileID         string   `json:"profile_id"`
	ModelID           string   `json:"model_id"`
	GatewayRoute      string   `json:"gateway_route"`
	Protocol          string   `json:"protocol"`
	ProviderAdapterID string   `json:"provider_adapter_id,omitempty"`
	Roles             []string `json:"roles"`
	CredentialPresent bool     `json:"credential_present"`
}

type BindRoleRequest struct {
	ConfigRoot string
	ModelRole  string
	ProfileID  string
}

type BindRoleResult struct {
	ModelRole         string `json:"model_role"`
	ProfileID         string `json:"profile_id"`
	PreviousProfileID string `json:"previous_profile_id,omitempty"`
}

type RotateProfileCredentialRequest struct {
	ConfigRoot          string
	CredentialStoreRoot string
	ProfileID           string
	Secret              string
	DryRun              bool
	Protector           func([]byte) ([]byte, error)
}

type RotateProfileCredentialResult struct {
	ProfileID      string `json:"profile_id"`
	GatewayRoute   string `json:"gateway_route"`
	CredentialRef  string `json:"credential_ref"`
	CredentialPath string `json:"credential_path"`
	DryRun         bool   `json:"dry_run"`
}

func Snapshot(req SnapshotRequest) (SnapshotResult, error) {
	config, err := ReadModels(ConfigPath(req.ConfigRoot))
	if err != nil {
		return SnapshotResult{}, err
	}
	if len(config.ModelGateway.Routes) == 0 && strings.TrimSpace(config.ModelGateway.Protocol) == "" {
		return SnapshotResult{}, ErrConfigMissing
	}
	result := SnapshotResult{RoleBindings: cloneBindings(config.ActiveModelProfileBindings)}
	for _, profile := range AllProfiles(config) {
		route := config.ModelGateway.Routes[strings.TrimSpace(profile.GatewayRoute)]
		protocol := firstNonEmpty(route.Protocol, config.ModelGateway.Protocol)
		credentialRef := firstNonEmpty(route.CredentialRef, config.ModelGateway.CredentialRef)
		roles := rolesForProfile(config.ActiveModelProfileBindings, profile.ProfileID)
		result.Profiles = append(result.Profiles, ProfileProjection{
			ProfileID:         profile.ProfileID,
			ModelID:           strings.TrimSpace(profile.ModelID),
			GatewayRoute:      strings.TrimSpace(profile.GatewayRoute),
			Protocol:          protocol,
			ProviderAdapterID: strings.TrimSpace(profile.ProviderAdapterID),
			Roles:             roles,
			CredentialPresent: CredentialExists(credentialRef, req.CredentialStoreRoot),
		})
	}
	sort.Slice(result.Profiles, func(i, j int) bool { return result.Profiles[i].ProfileID < result.Profiles[j].ProfileID })
	return result, nil
}

func BindRole(req BindRoleRequest) (BindRoleResult, error) {
	configPath := ConfigPath(req.ConfigRoot)
	config, err := ReadModels(configPath)
	if err != nil {
		return BindRoleResult{}, err
	}
	role := strings.TrimSpace(req.ModelRole)
	if role == "" {
		role = DefaultModelRole
	}
	profileID := strings.TrimSpace(req.ProfileID)
	if profileID == "" {
		return BindRoleResult{}, ErrProfileMissing
	}
	if _, ok := ProfileByID(config, profileID); !ok {
		return BindRoleResult{}, ErrProfileMissing
	}
	if config.ActiveModelProfileBindings == nil {
		config.ActiveModelProfileBindings = map[string]string{}
	}
	result := BindRoleResult{ModelRole: role, ProfileID: profileID, PreviousProfileID: strings.TrimSpace(config.ActiveModelProfileBindings[role])}
	config.ActiveModelProfileBindings[role] = profileID
	if err := WriteModels(configPath, config); err != nil {
		return BindRoleResult{}, err
	}
	return result, nil
}

func RotateProfileCredential(req RotateProfileCredentialRequest) (RotateProfileCredentialResult, error) {
	config, err := ReadModels(ConfigPath(req.ConfigRoot))
	if err != nil {
		return RotateProfileCredentialResult{}, err
	}
	profileID := strings.TrimSpace(req.ProfileID)
	profile, ok := ProfileByID(config, profileID)
	if !ok {
		return RotateProfileCredentialResult{}, ErrProfileMissing
	}
	route, ok := config.ModelGateway.Routes[strings.TrimSpace(profile.GatewayRoute)]
	if !ok {
		return RotateProfileCredentialResult{}, ErrConfigInvalid
	}
	credentialRef := firstNonEmpty(route.CredentialRef, config.ModelGateway.CredentialRef)
	if NormalizeCredentialRef(credentialRef) == "" {
		return RotateProfileCredentialResult{}, ErrCredentialRefInvalid
	}
	secret, err := WriteCredentialSecret(CredentialSecretWriteRequest{
		CredentialRef: credentialRef,
		Secret:        req.Secret,
		StoreRoot:     req.CredentialStoreRoot,
		Protector:     req.Protector,
		DryRun:        req.DryRun,
	})
	if err != nil {
		return RotateProfileCredentialResult{}, err
	}
	return RotateProfileCredentialResult{
		ProfileID:      profile.ProfileID,
		GatewayRoute:   profile.GatewayRoute,
		CredentialRef:  secret.CredentialRef,
		CredentialPath: secret.CredentialPath,
		DryRun:         secret.DryRun,
	}, nil
}

func ConfigPath(root string) string {
	return filepath.Join(ResolveConfigRoot(root), "models.json")
}

func ResolveConfigRoot(root string) string {
	if text := strings.TrimSpace(root); text != "" {
		return filepath.Clean(expandHome(text))
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".genesis", "config")
	}
	return filepath.Join(home, ".genesis", "config")
}

func ReadModels(path string) (ModelsConfig, error) {
	payload, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ModelsConfig{}, ErrConfigMissing
	}
	if err != nil {
		return ModelsConfig{}, fmt.Errorf("%w: %v", ErrConfigInvalid, err)
	}
	var config ModelsConfig
	if err := json.Unmarshal(payload, &config); err != nil {
		return ModelsConfig{}, fmt.Errorf("%w: %v", ErrConfigInvalid, err)
	}
	return config, nil
}

func WriteModels(path string, config ModelsConfig) error {
	encoded, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("%w: %v", ErrConfigMissing, err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".models-*.json")
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConfigMissing, err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(append(encoded, '\n')); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("%w: %v", ErrConfigMissing, err)
	}
	if err := temporary.Chmod(0o644); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("%w: %v", ErrConfigMissing, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("%w: %v", ErrConfigMissing, err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("%w: %v", ErrConfigMissing, err)
	}
	return nil
}

func AllProfiles(config ModelsConfig) []GatewayProfile {
	profiles := make([]GatewayProfile, 0, len(config.ModelProfiles.Cloud.Gateway)+len(config.ModelProfiles.Local.Gateway))
	for _, branch := range []ProfileBranch{config.ModelProfiles.Cloud, config.ModelProfiles.Local} {
		for key, profile := range branch.Gateway {
			profile.ProfileID = firstNonEmpty(profile.ProfileID, key)
			if strings.TrimSpace(profile.ProfileID) != "" {
				profiles = append(profiles, profile)
			}
		}
	}
	return profiles
}

func ProfileByID(config ModelsConfig, profileID string) (GatewayProfile, bool) {
	profileID = strings.TrimSpace(profileID)
	for _, profile := range AllProfiles(config) {
		if profile.ProfileID == profileID {
			return profile, true
		}
	}
	return GatewayProfile{}, false
}

func CredentialExists(ref string, storeRoot string) bool {
	path := CredentialPath(ref, storeRoot)
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func CredentialPath(ref string, storeRoot string) string {
	return credentialPath(ref, storeRoot)
}

func NormalizeCredentialRef(value string) string {
	text := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"))
	if !strings.HasPrefix(text, "secret://") {
		return ""
	}
	parts := strings.Split(text[len("secret://"):], "/")
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		token := strings.TrimSpace(part)
		if token == "" {
			continue
		}
		if token == ".." {
			return ""
		}
		cleaned = append(cleaned, token)
	}
	if len(cleaned) == 0 {
		return ""
	}
	normalized := "secret://" + strings.Join(cleaned, "/")
	if !credentialRefPattern.MatchString(normalized) {
		return ""
	}
	return normalized
}

func cloneBindings(bindings map[string]string) map[string]string {
	if len(bindings) == 0 {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(bindings))
	for role, profileID := range bindings {
		if role = strings.TrimSpace(role); role != "" {
			cloned[role] = strings.TrimSpace(profileID)
		}
	}
	return cloned
}

func rolesForProfile(bindings map[string]string, profileID string) []string {
	roles := make([]string, 0)
	for role, bound := range bindings {
		if strings.TrimSpace(bound) == profileID {
			roles = append(roles, strings.TrimSpace(role))
		}
	}
	sort.Strings(roles)
	return roles
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
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

func refToken(value string) string {
	text := strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false
	for _, char := range text {
		allowed := (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' || char == '_' || char == '.'
		if allowed {
			builder.WriteRune(char)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-_.")
}
