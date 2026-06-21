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
	flag.Parse()

	k, err := kernel.New(kernel.Config{
		LedgerPath: *ledgerPath,
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
