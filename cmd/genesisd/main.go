package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"genesis/internal/kernel"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8765", "HTTP listen address")
	ledgerPath := flag.String("ledger", defaultLedgerPath(), "append-only event ledger path")
	providerName := flag.String("provider", envOrDefault("GENESIS_PROVIDER", "fake"), "provider name: fake or openai-compatible")
	providerBaseURL := flag.String("provider-base-url", os.Getenv("GENESIS_PROVIDER_BASE_URL"), "OpenAI-compatible provider base URL")
	providerModel := flag.String("provider-model", os.Getenv("GENESIS_PROVIDER_MODEL"), "OpenAI-compatible provider model")
	providerAPIKeyEnv := flag.String("provider-api-key-env", envOrDefault("GENESIS_PROVIDER_API_KEY_ENV", "GENESIS_PROVIDER_API_KEY"), "environment variable containing provider API key")
	flag.Parse()

	provider, err := buildProvider(*providerName, *providerBaseURL, *providerModel, *providerAPIKeyEnv)
	if err != nil {
		log.Fatalf("create provider: %v", err)
	}
	k, err := kernel.New(kernel.Config{
		LedgerPath: *ledgerPath,
		Provider:   provider,
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

func buildProvider(name string, baseURL string, model string, apiKeyEnv string) (kernel.Provider, error) {
	switch name {
	case "", "fake":
		return kernel.FakeProvider{}, nil
	case "openai-compatible":
		return kernel.NewOpenAICompatibleProvider(kernel.OpenAICompatibleConfig{
			BaseURL: baseURL,
			APIKey:  os.Getenv(apiKeyEnv),
			Model:   model,
		}), nil
	default:
		return nil, fmt.Errorf("unknown provider %q", name)
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
