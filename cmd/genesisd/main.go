package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"genesis/internal/kernel"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8765", "HTTP listen address")
	ledgerPath := flag.String("ledger", defaultLedgerPath(), "append-only event ledger path")
	runtimeToken := flag.String("runtime-token", os.Getenv("GENESIS_RUNTIME_TOKEN"), "runtime bearer token for protected routes")
	permissionMode := flag.String("permission-mode", envOrDefault("GENESIS_PERMISSION_MODE", kernel.PermissionModePlan), "tool permission mode: plan, default, or yolo")
	workspaceRoot := flag.String("workspace-root", os.Getenv("GENESIS_WORKSPACE_ROOT"), "workspace root for default tool permission mode")
	providerName := flag.String("provider", envOrDefault("GENESIS_PROVIDER", "genesis-config"), "provider name: genesis-config, fake, provider_command, or openai-compatible")
	providerBaseURL := flag.String("provider-base-url", os.Getenv("GENESIS_PROVIDER_BASE_URL"), "OpenAI-compatible provider base URL")
	providerModel := flag.String("provider-model", os.Getenv("GENESIS_PROVIDER_MODEL"), "OpenAI-compatible provider model")
	providerAPIKeyEnv := flag.String("provider-api-key-env", envOrDefault("GENESIS_PROVIDER_API_KEY_ENV", "GENESIS_PROVIDER_API_KEY"), "environment variable containing provider API key")
	providerCommand := flag.String("provider-command", os.Getenv("GENESIS_PROVIDER_COMMAND"), "provider command adapter executable")
	providerCommandArgs := stringListFlag(splitPathList(os.Getenv("GENESIS_PROVIDER_COMMAND_ARGS")))
	flag.Var(&providerCommandArgs, "provider-command-arg", "provider command adapter argument; repeatable")
	configRoot := flag.String("config-root", os.Getenv("GENESIS_CONFIG_ROOT"), "Genesis config root containing models.json")
	credentialStoreRoot := flag.String("credential-store-root", os.Getenv("GENESIS_CREDENTIAL_STORE_ROOT"), "Genesis credential store root")
	modelRole := flag.String("model-role", envOrDefault("GENESIS_MODEL_ROLE", kernel.DefaultModelRole), "Genesis model role binding to resolve")
	modelProfileID := flag.String("model-profile-id", os.Getenv("GENESIS_MODEL_PROFILE_ID"), "Genesis model profile id override")
	skillRoots := pathListFlag(defaultSkillRoots())
	flag.Var(&skillRoots, "skill-root", "external skill root to scan for SKILL.md metadata; repeatable")
	flag.Parse()

	provider, err := buildProvider(providerBuildRequest{
		name:                *providerName,
		baseURL:             *providerBaseURL,
		model:               *providerModel,
		apiKeyEnv:           *providerAPIKeyEnv,
		command:             *providerCommand,
		commandArgs:         providerCommandArgs.Values(),
		configRoot:          *configRoot,
		credentialStoreRoot: *credentialStoreRoot,
		modelRole:           *modelRole,
		modelProfileID:      *modelProfileID,
	})
	if err != nil {
		log.Fatalf("create provider: %v", err)
	}
	k, err := kernel.New(kernel.Config{
		LedgerPath:   *ledgerPath,
		Provider:     provider,
		RuntimeToken: *runtimeToken,
		ToolPolicy: kernel.ToolPolicy{
			PermissionMode: *permissionMode,
			WorkspaceRoot:  *workspaceRoot,
		},
		SkillRoots: skillRoots.Values(),
	})
	if err != nil {
		log.Fatalf("create kernel: %v", err)
	}

	server := &http.Server{
		Addr:              *addr,
		Handler:           kernel.Handler(k),
		ReadHeaderTimeout: 5 * time.Second,
	}
	fmt.Printf("genesisd listening on http://%s\n", *addr)
	fmt.Printf("ledger: %s\n", *ledgerPath)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve: %v", err)
	}
}

type providerBuildRequest struct {
	name                string
	baseURL             string
	model               string
	apiKeyEnv           string
	command             string
	commandArgs         []string
	configRoot          string
	credentialStoreRoot string
	modelRole           string
	modelProfileID      string
}

func buildProvider(req providerBuildRequest) (kernel.Provider, error) {
	switch req.name {
	case "", "fake":
		return kernel.FakeProvider{}, nil
	case "genesis-config":
		config, err := kernel.ResolveProviderConfigFromGenesis(kernel.GenesisModelConfigRequest{
			ConfigRoot:          req.configRoot,
			CredentialStoreRoot: req.credentialStoreRoot,
			ModelRole:           req.modelRole,
			ModelProfileID:      req.modelProfileID,
		})
		if err != nil {
			return kernel.NewBlockedProvider("provider", kernel.ProviderConfigReason(err)), nil
		}
		switch config.Kind {
		case "provider_command":
			return kernel.NewCommandProvider(config.Command), nil
		case "openai-compatible":
			return kernel.NewOpenAICompatibleProvider(config.OpenAICompatible), nil
		default:
			return kernel.NewBlockedProvider("provider", "provider_config_invalid"), nil
		}
	case "provider_command":
		return kernel.NewCommandProvider(kernel.ProviderCommandConfig{
			Command: req.command,
			Args:    req.commandArgs,
			Model:   req.model,
		}), nil
	case "openai-compatible":
		return kernel.NewOpenAICompatibleProvider(kernel.OpenAICompatibleConfig{
			BaseURL: req.baseURL,
			APIKey:  os.Getenv(req.apiKeyEnv),
			Model:   req.model,
		}), nil
	default:
		return nil, fmt.Errorf("unknown provider %q", req.name)
	}
}

func defaultLedgerPath() string {
	if path := os.Getenv("GENESIS_LEDGER_PATH"); path != "" {
		return path
	}
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		return filepath.Join(".genesis", "events.jsonl")
	}
	return filepath.Join(dir, "Genesis", "kernel", "events.jsonl")
}

func envOrDefault(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

type pathListFlag []string

func (f *pathListFlag) String() string {
	if f == nil {
		return ""
	}
	return strings.Join(*f, string(os.PathListSeparator))
}

func (f *pathListFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	*f = append(*f, value)
	return nil
}

func (f pathListFlag) Values() []string {
	return append([]string(nil), f...)
}

func defaultSkillRoots() []string {
	if roots := splitPathList(os.Getenv("GENESIS_SKILL_ROOTS")); len(roots) > 0 {
		return roots
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}
	return []string{filepath.Join(home, ".agents", "skills")}
}

func splitPathList(value string) []string {
	var roots []string
	for _, root := range filepath.SplitList(value) {
		root = strings.TrimSpace(root)
		if root != "" {
			roots = append(roots, root)
		}
	}
	return roots
}

type stringListFlag []string

func (f *stringListFlag) String() string {
	if f == nil {
		return ""
	}
	return strings.Join(*f, " ")
}

func (f *stringListFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func (f stringListFlag) Values() []string {
	return append([]string(nil), f...)
}
