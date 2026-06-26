package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"genesis/internal/kernel"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "genesisctl: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("command is required: provider-setup or provider")
	}
	switch args[0] {
	case "provider-setup":
		return runProviderSetup(args[1:], stdin, stdout)
	case "provider":
		return runProvider(args[1:], stdin, stdout)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runProvider(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("provider command is required: use, rotate-key, or verify")
	}
	switch args[0] {
	case "use":
		return runProviderUse(args[1:], stdin, stdout)
	case "rotate-key":
		return runProviderRotateKey(args[1:], stdin, stdout)
	case "verify":
		return runProviderVerify(args[1:], stdout)
	default:
		return fmt.Errorf("unknown provider command %q", args[0])
	}
}

func runProviderUse(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("provider preset is required")
	}
	preset, ok := providerSetupPresetByRef(args[0])
	if !ok {
		return fmt.Errorf("unknown provider preset %q", args[0])
	}
	fs := flag.NewFlagSet("provider use", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configRoot := fs.String("config-root", os.Getenv("GENESIS_CONFIG_ROOT"), "Genesis config root containing models.json")
	credentialStoreRoot := fs.String("credential-store-root", os.Getenv("GENESIS_CREDENTIAL_STORE_ROOT"), "Genesis credential store root")
	modelRole := fs.String("model-role", envOrDefault("GENESIS_MODEL_ROLE", kernel.DefaultModelRole), "Genesis model role binding to write")
	apiKeyEnv := fs.String("api-key-env", envOrDefault("GENESIS_PROVIDER_API_KEY_ENV", preset.APIKeyEnv), "environment variable containing provider API key")
	apiKeyStdin := fs.Bool("api-key-stdin", false, "read provider API key from stdin")
	dryRun := fs.Bool("dry-run", false, "validate and print target paths without writing files")
	verify := fs.Bool("verify", true, "verify generated config and credential after writing")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	apiKey, err := readAPIKey(*apiKeyEnv, *apiKeyStdin, *dryRun, stdin)
	if err != nil {
		return err
	}
	result, err := kernel.SetupOpenAICompatibleProvider(kernel.OpenAICompatibleProviderSetupRequest{
		ConfigRoot:          *configRoot,
		CredentialStoreRoot: *credentialStoreRoot,
		ModelRole:           *modelRole,
		ProfileID:           preset.ProfileID,
		GatewayRoute:        preset.GatewayRoute,
		ProviderAdapterID:   preset.AdapterID,
		AdapterProfileID:    preset.AdapterProfileID,
		HiddenReasoningMode: preset.HiddenReasoningMode,
		BaseURL:             preset.BaseURL,
		ModelID:             preset.ModelID,
		ContextWindowTokens: preset.ContextWindowTokens,
		CredentialRef:       preset.CredentialRef,
		APIKey:              apiKey,
		RequestTimeout:      preset.RequestTimeout,
		DryRun:              *dryRun,
		Verify:              *verify && !*dryRun,
	})
	if err != nil {
		return err
	}
	return writeProviderSetupResponse(stdout, result, preset)
}

func runProviderRotateKey(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("provider rotate-key", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configRoot := fs.String("config-root", os.Getenv("GENESIS_CONFIG_ROOT"), "Genesis config root containing models.json")
	credentialStoreRoot := fs.String("credential-store-root", os.Getenv("GENESIS_CREDENTIAL_STORE_ROOT"), "Genesis credential store root")
	modelRole := fs.String("model-role", envOrDefault("GENESIS_MODEL_ROLE", kernel.DefaultModelRole), "Genesis model role binding to use")
	profileID := fs.String("profile-id", os.Getenv("GENESIS_MODEL_PROFILE_ID"), "Genesis model profile id override")
	apiKeyEnv := fs.String("api-key-env", envOrDefault("GENESIS_PROVIDER_API_KEY_ENV", "GENESIS_PROVIDER_API_KEY"), "environment variable containing provider API key")
	apiKeyStdin := fs.Bool("api-key-stdin", false, "read provider API key from stdin")
	repairProfileMetadata := fs.String("repair-profile-metadata", "", "repair active profile adapter metadata from a known provider preset, e.g. deepseek/deepseek-v4-flash")
	dryRun := fs.Bool("dry-run", false, "validate and print target paths without writing files")
	verify := fs.Bool("verify", true, "verify generated credential after writing")
	if err := fs.Parse(args); err != nil {
		return err
	}
	var preset providerSetupPreset
	var repair *kernel.OpenAICompatibleProviderProfileMetadataRepair
	if strings.TrimSpace(*repairProfileMetadata) != "" {
		var ok bool
		preset, ok = providerSetupPresetByRef(*repairProfileMetadata)
		if !ok {
			return fmt.Errorf("unknown provider preset for profile metadata repair %q", *repairProfileMetadata)
		}
		repair = &kernel.OpenAICompatibleProviderProfileMetadataRepair{
			ProfileID:             preset.ProfileID,
			ModelID:               preset.ModelID,
			GatewayRoute:          preset.GatewayRoute,
			ProviderAdapterID:     preset.AdapterID,
			AdapterProfileID:      preset.AdapterProfileID,
			HiddenReasoningPolicy: preset.HiddenReasoningMode,
			ContextWindowTokens:   preset.ContextWindowTokens,
		}
	}
	apiKey, err := readAPIKey(*apiKeyEnv, *apiKeyStdin, *dryRun, stdin)
	if err != nil {
		return err
	}
	result, err := kernel.RotateActiveOpenAICompatibleProviderCredential(kernel.OpenAICompatibleProviderCredentialRotationRequest{
		ConfigRoot:            *configRoot,
		CredentialStoreRoot:   *credentialStoreRoot,
		ModelRole:             *modelRole,
		ProfileID:             *profileID,
		APIKey:                apiKey,
		RepairProfileMetadata: repair,
		DryRun:                *dryRun,
		Verify:                *verify && !*dryRun,
	})
	if err != nil {
		return err
	}
	return writeProviderSetupResponse(stdout, result, preset)
}

func runProviderVerify(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("provider verify", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configRoot := fs.String("config-root", os.Getenv("GENESIS_CONFIG_ROOT"), "Genesis config root containing models.json")
	credentialStoreRoot := fs.String("credential-store-root", os.Getenv("GENESIS_CREDENTIAL_STORE_ROOT"), "Genesis credential store root")
	modelRole := fs.String("model-role", envOrDefault("GENESIS_MODEL_ROLE", kernel.DefaultModelRole), "Genesis model role binding to verify")
	profileID := fs.String("profile-id", os.Getenv("GENESIS_MODEL_PROFILE_ID"), "Genesis model profile id override")
	timeoutSec := fs.Float64("timeout-sec", 10, "provider verification timeout seconds")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *timeoutSec <= 0 {
		return errors.New("timeout-sec must be greater than zero")
	}
	result := kernel.VerifyProviderLive(kernel.ProviderLiveVerifyRequest{
		ConfigRoot:          *configRoot,
		CredentialStoreRoot: *credentialStoreRoot,
		ModelRole:           *modelRole,
		ModelProfileID:      *profileID,
		Timeout:             time.Duration(*timeoutSec * float64(time.Second)),
	})
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func runProviderSetup(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("provider-setup", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	configRoot := fs.String("config-root", os.Getenv("GENESIS_CONFIG_ROOT"), "Genesis config root containing models.json")
	credentialStoreRoot := fs.String("credential-store-root", os.Getenv("GENESIS_CREDENTIAL_STORE_ROOT"), "Genesis credential store root")
	modelRole := fs.String("model-role", envOrDefault("GENESIS_MODEL_ROLE", kernel.DefaultModelRole), "Genesis model role binding to write")
	profileID := fs.String("profile-id", envOrDefault("GENESIS_MODEL_PROFILE_ID", "default"), "Genesis model profile id to write")
	gatewayRoute := fs.String("gateway-route", envOrDefault("GENESIS_GATEWAY_ROUTE", "default"), "Genesis model gateway route to write")
	baseURL := fs.String("base-url", os.Getenv("GENESIS_PROVIDER_BASE_URL"), "OpenAI-compatible provider base URL")
	modelID := fs.String("model", os.Getenv("GENESIS_PROVIDER_MODEL"), "OpenAI-compatible provider model id")
	contextWindowTokens := fs.Int("context-window-tokens", 0, "model context window metadata in tokens")
	credentialRef := fs.String("credential-ref", envOrDefault("GENESIS_PROVIDER_CREDENTIAL_REF", "secret://models/provider/default"), "Genesis local secret credential ref")
	apiKeyEnv := fs.String("api-key-env", envOrDefault("GENESIS_PROVIDER_API_KEY_ENV", "GENESIS_PROVIDER_API_KEY"), "environment variable containing provider API key")
	apiKeyStdin := fs.Bool("api-key-stdin", false, "read provider API key from stdin")
	timeoutSec := fs.Float64("request-timeout-sec", 60, "provider request timeout seconds")
	dryRun := fs.Bool("dry-run", false, "validate and print target paths without writing files")
	verify := fs.Bool("verify", true, "verify generated config and credential after writing")
	if err := fs.Parse(args); err != nil {
		return err
	}
	apiKey, err := readAPIKey(*apiKeyEnv, *apiKeyStdin, *dryRun, stdin)
	if err != nil {
		return err
	}
	result, err := kernel.SetupOpenAICompatibleProvider(kernel.OpenAICompatibleProviderSetupRequest{
		ConfigRoot:          *configRoot,
		CredentialStoreRoot: *credentialStoreRoot,
		ModelRole:           *modelRole,
		ProfileID:           *profileID,
		GatewayRoute:        *gatewayRoute,
		BaseURL:             *baseURL,
		ModelID:             *modelID,
		ContextWindowTokens: *contextWindowTokens,
		CredentialRef:       *credentialRef,
		APIKey:              apiKey,
		RequestTimeout:      time.Duration(*timeoutSec * float64(time.Second)),
		DryRun:              *dryRun,
		Verify:              *verify && !*dryRun,
	})
	if err != nil {
		return err
	}
	response := providerSetupResponse{
		OK:             true,
		ConfigPath:     result.ConfigPath,
		CredentialPath: result.CredentialPath,
		CredentialRef:  result.CredentialRef,
		ModelRole:      result.ModelRole,
		ProfileID:      result.ProfileID,
		GatewayRoute:   result.GatewayRoute,
		DryRun:         result.DryRun,
		Verified:       result.Verified,
	}
	return encodeProviderSetupResponse(stdout, response)
}

func writeProviderSetupResponse(stdout io.Writer, result kernel.OpenAICompatibleProviderSetupResult, preset providerSetupPreset) error {
	response := providerSetupResponse{
		OK:                  true,
		ConfigPath:          result.ConfigPath,
		CredentialPath:      result.CredentialPath,
		CredentialRef:       result.CredentialRef,
		ModelRole:           result.ModelRole,
		ProfileID:           result.ProfileID,
		GatewayRoute:        result.GatewayRoute,
		DryRun:              result.DryRun,
		Verified:            result.Verified,
		ProviderID:          preset.ProviderID,
		ModelID:             preset.ModelID,
		ProviderAdapterID:   preset.AdapterID,
		AdapterProfileID:    preset.AdapterProfileID,
		BaseURL:             preset.BaseURL,
		ContextWindowTokens: preset.ContextWindowTokens,
	}
	return encodeProviderSetupResponse(stdout, response)
}

func encodeProviderSetupResponse(stdout io.Writer, response providerSetupResponse) error {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(response)
}

type providerSetupResponse struct {
	OK                  bool   `json:"ok"`
	ConfigPath          string `json:"config_path"`
	CredentialPath      string `json:"credential_path"`
	CredentialRef       string `json:"credential_ref"`
	ModelRole           string `json:"model_role"`
	ProfileID           string `json:"profile_id"`
	GatewayRoute        string `json:"gateway_route"`
	DryRun              bool   `json:"dry_run"`
	Verified            bool   `json:"verified"`
	ProviderID          string `json:"provider_id,omitempty"`
	ModelID             string `json:"model_id,omitempty"`
	ProviderAdapterID   string `json:"provider_adapter_id,omitempty"`
	AdapterProfileID    string `json:"provider_adapter_profile_id,omitempty"`
	BaseURL             string `json:"base_url,omitempty"`
	ContextWindowTokens int    `json:"context_window_tokens,omitempty"`
}

func readAPIKey(envName string, fromStdin bool, dryRun bool, stdin io.Reader) (string, error) {
	if dryRun {
		return "", nil
	}
	if fromStdin {
		payload, err := io.ReadAll(stdin)
		if err != nil {
			return "", fmt.Errorf("read API key from stdin: %w", err)
		}
		key := strings.TrimSpace(string(payload))
		if key == "" {
			return "", errors.New("API key from stdin is empty")
		}
		return key, nil
	}
	name := strings.TrimSpace(envName)
	if name == "" {
		return "", errors.New("api-key-env is required when api-key-stdin is false")
	}
	key := strings.TrimSpace(os.Getenv(name))
	if key == "" {
		return "", fmt.Errorf("environment variable %s is empty", name)
	}
	return key, nil
}

func envOrDefault(name string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
