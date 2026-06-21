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
		return errors.New("command is required: provider-setup")
	}
	switch args[0] {
	case "provider-setup":
		return runProviderSetup(args[1:], stdin, stdout)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
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
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(response)
}

type providerSetupResponse struct {
	OK             bool   `json:"ok"`
	ConfigPath     string `json:"config_path"`
	CredentialPath string `json:"credential_path"`
	CredentialRef  string `json:"credential_ref"`
	ModelRole      string `json:"model_role"`
	ProfileID      string `json:"profile_id"`
	GatewayRoute   string `json:"gateway_route"`
	DryRun         bool   `json:"dry_run"`
	Verified       bool   `json:"verified"`
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
