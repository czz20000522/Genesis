package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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
	allowLabFakeProvider := flag.Bool("allow-lab-fake-provider", envBoolOrDefault("GENESIS_ALLOW_LAB_FAKE_PROVIDER", false), "allow lab-only fake provider readiness")
	providerBaseURL := flag.String("provider-base-url", os.Getenv("GENESIS_PROVIDER_BASE_URL"), "OpenAI-compatible provider base URL")
	providerModel := flag.String("provider-model", os.Getenv("GENESIS_PROVIDER_MODEL"), "OpenAI-compatible provider model")
	providerAPIKeyEnv := flag.String("provider-api-key-env", envOrDefault("GENESIS_PROVIDER_API_KEY_ENV", "GENESIS_PROVIDER_API_KEY"), "environment variable containing provider API key")
	providerCommand := flag.String("provider-command", os.Getenv("GENESIS_PROVIDER_COMMAND"), "provider command adapter executable")
	providerCommandArgs := stringListFlag(splitPathList(os.Getenv("GENESIS_PROVIDER_COMMAND_ARGS")))
	flag.Var(&providerCommandArgs, "provider-command-arg", "provider command adapter argument; repeatable")
	providerCommandEnv := stringListFlag(splitPathList(os.Getenv("GENESIS_PROVIDER_COMMAND_ENV")))
	flag.Var(&providerCommandEnv, "provider-command-env", "provider command environment entry NAME=VALUE; repeatable")
	configRoot := flag.String("config-root", os.Getenv("GENESIS_CONFIG_ROOT"), "Genesis config root containing models.json")
	credentialStoreRoot := flag.String("credential-store-root", os.Getenv("GENESIS_CREDENTIAL_STORE_ROOT"), "Genesis credential store root")
	modelRole := flag.String("model-role", envOrDefault("GENESIS_MODEL_ROLE", kernel.DefaultModelRole), "Genesis model role binding to resolve")
	modelProfileID := flag.String("model-profile-id", os.Getenv("GENESIS_MODEL_PROFILE_ID"), "Genesis model profile id override")
	contextWindowTokens := flag.Int("context-window-tokens", envIntOrDefault("GENESIS_CONTEXT_WINDOW_TOKENS", 0), "model context window in tokens; 0 disables auto compaction")
	autoCompactRatio := flag.Float64("auto-compact-ratio", envFloatOrDefault("GENESIS_AUTO_COMPACT_RATIO", 0), "auto compact when input tokens reach this fraction of context window; default 0.8 when context window is set")
	compactRecentTurns := flag.Int("compact-recent-turns", envIntOrDefault("GENESIS_COMPACT_RECENT_TURNS", 0), "recent completed turns to keep verbatim after compaction; default 2 when compaction is enabled")
	compactRecentTailTokens := flag.Int("compact-recent-tail-tokens", envIntOrDefault("GENESIS_COMPACT_RECENT_TAIL_TOKENS", 0), "provider-backed processed input token budget for recent completed turns kept verbatim after compaction; 0 keeps compact-recent-turns only")
	skillIndexChars := flag.Int("skill-index-chars", envIntOrDefault("GENESIS_SKILL_INDEX_CHARS", 0), "maximum characters of model-visible external skill index; 0 uses kernel default, negative disables")
	modelToolRoundBudget := flag.Int("model-tool-round-budget", envIntOrDefault("GENESIS_MODEL_TOOL_ROUND_BUDGET", 0), "model tool-round execution budget per turn; 0 uses kernel default")
	modelToolRoundCeiling := flag.Int("model-tool-round-ceiling", envIntOrDefault("GENESIS_MODEL_TOOL_ROUND_CEILING", 0), "maximum allowed model tool-round execution budget; 0 uses kernel default ceiling")
	sourceMaxFileCount := flag.Int("source-max-file-count", envIntOrDefault("GENESIS_SOURCE_MAX_FILE_COUNT", 0), "maximum files in a source snapshot archive; 0 uses kernel default")
	sourceMaxPerFileBytes := flag.Int("source-max-per-file-bytes", envIntOrDefault("GENESIS_SOURCE_MAX_PER_FILE_BYTES", 0), "maximum uncompressed bytes per source snapshot file; 0 uses kernel default")
	sourceMaxTotalBytes := flag.Int("source-max-total-bytes", envIntOrDefault("GENESIS_SOURCE_MAX_TOTAL_BYTES", 0), "maximum total uncompressed bytes in a source snapshot archive; 0 uses kernel default")
	sourceDefaultTreeEntries := flag.Int("source-default-tree-entries", envIntOrDefault("GENESIS_SOURCE_DEFAULT_TREE_ENTRIES", 0), "default source_tree entry limit; 0 uses kernel default")
	sourceMaxTreeEntries := flag.Int("source-max-tree-entries", envIntOrDefault("GENESIS_SOURCE_MAX_TREE_ENTRIES", 0), "maximum source_tree entry limit; 0 uses kernel default")
	sourceDefaultReadBytes := flag.Int("source-default-read-bytes", envIntOrDefault("GENESIS_SOURCE_DEFAULT_READ_BYTES", 0), "default source_read byte limit; 0 uses kernel default")
	sourceMaxReadBytes := flag.Int("source-max-read-bytes", envIntOrDefault("GENESIS_SOURCE_MAX_READ_BYTES", 0), "maximum source_read byte limit; 0 uses kernel default")
	skillRoots := pathListFlag(nil)
	disableDefaultSkillRoots := flag.Bool("disable-default-skill-roots", envBoolOrDefault("GENESIS_DISABLE_DEFAULT_SKILL_ROOTS", false), "smoke/dev escape hatch: scan only explicit --skill-root entries and do not include default global skill roots")
	flag.Var(&skillRoots, "skill-root", "external skill root to scan for SKILL.md metadata; repeatable")
	flag.Parse()
	effectiveSkills := effectiveSkillRoots(defaultSkillRoots(), skillRoots.Values(), *disableDefaultSkillRoots)

	provider, err := buildProvider(providerBuildRequest{
		name:                 *providerName,
		baseURL:              *providerBaseURL,
		model:                *providerModel,
		apiKeyEnv:            *providerAPIKeyEnv,
		command:              *providerCommand,
		commandArgs:          providerCommandArgs.Values(),
		commandEnv:           providerCommandEnv.Values(),
		configRoot:           *configRoot,
		credentialStoreRoot:  *credentialStoreRoot,
		modelRole:            *modelRole,
		modelProfileID:       *modelProfileID,
		allowLabFakeProvider: *allowLabFakeProvider,
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
		ContextPolicy: kernel.ContextPolicy{
			ContextWindowTokens: *contextWindowTokens,
			AutoCompactRatio:    *autoCompactRatio,
			RecentTurnLimit:     *compactRecentTurns,
			RecentTailTokens:    *compactRecentTailTokens,
			SkillIndexChars:     *skillIndexChars,
		},
		BudgetPolicy: kernel.BudgetPolicy{
			ModelToolRoundBudget:  *modelToolRoundBudget,
			ModelToolRoundCeiling: *modelToolRoundCeiling,
		},
		SourceSnapshotPolicy: kernel.SourceSnapshotPolicy{
			MaxFileCount:                *sourceMaxFileCount,
			MaxPerFileUncompressedBytes: int64(*sourceMaxPerFileBytes),
			MaxTotalUncompressedBytes:   int64(*sourceMaxTotalBytes),
			DefaultTreeEntries:          *sourceDefaultTreeEntries,
			MaxTreeEntries:              *sourceMaxTreeEntries,
			DefaultReadBytes:            *sourceDefaultReadBytes,
			MaxReadBytes:                *sourceMaxReadBytes,
		},
		SkillRoots: effectiveSkills,
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
	name                 string
	baseURL              string
	model                string
	apiKeyEnv            string
	command              string
	commandArgs          []string
	commandEnv           []string
	configRoot           string
	credentialStoreRoot  string
	modelRole            string
	modelProfileID       string
	allowLabFakeProvider bool
}

func buildProvider(req providerBuildRequest) (kernel.Provider, error) {
	switch req.name {
	case "":
		return buildProvider(providerBuildRequest{
			name:                "genesis-config",
			configRoot:          req.configRoot,
			credentialStoreRoot: req.credentialStoreRoot,
			modelRole:           req.modelRole,
			modelProfileID:      req.modelProfileID,
		})
	case "fake":
		if req.allowLabFakeProvider {
			return kernel.FakeProvider{}, nil
		}
		return kernel.NewBlockedProvider("fake", "provider_fake_lab_only"), nil
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
			Env:     req.commandEnv,
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

func envIntOrDefault(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envFloatOrDefault(name string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBoolOrDefault(name string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
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

func effectiveSkillRoots(defaults []string, explicit []string, disableDefaults bool) []string {
	roots := append([]string(nil), explicit...)
	if !disableDefaults {
		roots = append(roots, defaults...)
	}
	return dedupeSkillRoots(roots)
}

func defaultSkillRoots() []string {
	if roots := splitPathList(os.Getenv("GENESIS_SKILL_ROOTS")); len(roots) > 0 {
		return dedupeSkillRoots(roots)
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}
	return dedupeSkillRoots([]string{
		filepath.Join(home, ".genesis", "skills"),
		filepath.Join(home, ".agents", "skills"),
	})
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

func dedupeSkillRoots(roots []string) []string {
	var out []string
	seen := make(map[string]struct{}, len(roots))
	for _, root := range roots {
		key := filepath.Clean(root)
		if filepath.Separator == '\\' {
			key = strings.ToLower(key)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, root)
	}
	return out
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
