package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"genesis/internal/applications/connector_runtime"
)

type InspectionReport struct {
	Inbound        []connectorruntime.InboundSubmissionRecord    `json:"inbound"`
	Outbox         []connectorruntime.ConnectorOutboxItem        `json:"outbox"`
	Receipts       map[string][]connectorruntime.DeliveryReceipt `json:"receipts"`
	KernelSessions map[string]json.RawMessage                    `json:"kernel_sessions,omitempty"`
	KernelErrors   map[string]string                             `json:"kernel_errors,omitempty"`
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: genesis-console <inspect> [flags]")
	}
	switch args[0] {
	case "inspect":
		return runInspect(ctx, args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runInspect(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	inboundPath := fs.String("inbound-state", envOrDefault("GENESIS_INGRESS_STATE", filepath.Join(".genesis_ingress", "state.json")), "connector inbound state file")
	outboxPath := fs.String("outbox-state", envOrDefault("GENESIS_CONNECTOR_OUTBOX_STATE", filepath.Join(".genesis_ingress", "outbox.json")), "connector outbox state file")
	kernelURL := fs.String("kernel-url", os.Getenv("GENESIS_KERNEL_URL"), "optional Genesis Kernel HTTP URL for session projections")
	runtimeToken := fs.String("runtime-token", os.Getenv("GENESIS_RUNTIME_TOKEN"), "Genesis runtime bearer token")
	if err := fs.Parse(args); err != nil {
		return err
	}

	report, err := inspectConnectorState(ctx, *inboundPath, *outboxPath, strings.TrimRight(*kernelURL, "/"), *runtimeToken)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return err
	}
	return nil
}

func inspectConnectorState(ctx context.Context, inboundPath string, outboxPath string, kernelURL string, runtimeToken string) (InspectionReport, error) {
	inboundStore, err := connectorruntime.NewFileInboundStore(inboundPath)
	if err != nil {
		return InspectionReport{}, err
	}
	outboxStore, err := connectorruntime.NewFileOutboxStore(outboxPath)
	if err != nil {
		return InspectionReport{}, err
	}
	inbound, err := inboundStore.ListInbound(ctx)
	if err != nil {
		return InspectionReport{}, err
	}
	outbox, err := outboxStore.ListOutbox(ctx)
	if err != nil {
		return InspectionReport{}, err
	}
	receipts := make(map[string][]connectorruntime.DeliveryReceipt, len(outbox))
	for _, item := range outbox {
		itemReceipts, err := outboxStore.ListReceipts(ctx, item.OutboxID)
		if err != nil {
			return InspectionReport{}, err
		}
		receipts[item.OutboxID] = itemReceipts
	}
	report := InspectionReport{
		Inbound:  inbound,
		Outbox:   outbox,
		Receipts: receipts,
	}
	if strings.TrimSpace(kernelURL) != "" {
		sessions, errorsBySession := fetchKernelSessionProjections(ctx, kernelURL, runtimeToken, uniqueKernelSessionIDs(inbound))
		if len(sessions) != 0 {
			report.KernelSessions = sessions
		}
		if len(errorsBySession) != 0 {
			report.KernelErrors = errorsBySession
		}
	}
	return report, nil
}

func uniqueKernelSessionIDs(inbound []connectorruntime.InboundSubmissionRecord) []string {
	seen := map[string]struct{}{}
	for _, record := range inbound {
		sessionID := strings.TrimSpace(record.KernelSessionID)
		if sessionID == "" {
			continue
		}
		seen[sessionID] = struct{}{}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func fetchKernelSessionProjections(ctx context.Context, kernelURL string, runtimeToken string, sessionIDs []string) (map[string]json.RawMessage, map[string]string) {
	sessions := map[string]json.RawMessage{}
	errorsBySession := map[string]string{}
	client := &http.Client{}
	for _, sessionID := range sessionIDs {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, kernelURL+"/sessions/"+sessionID, nil)
		if err != nil {
			errorsBySession[sessionID] = err.Error()
			continue
		}
		if strings.TrimSpace(runtimeToken) != "" {
			req.Header.Set("Authorization", "Bearer "+runtimeToken)
		}
		resp, err := client.Do(req)
		if err != nil {
			errorsBySession[sessionID] = err.Error()
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
		_ = resp.Body.Close()
		if readErr != nil {
			errorsBySession[sessionID] = readErr.Error()
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			errorsBySession[sessionID] = fmt.Sprintf("kernel session projection returned HTTP %d", resp.StatusCode)
			continue
		}
		if !json.Valid(body) {
			errorsBySession[sessionID] = "kernel session projection returned invalid JSON"
			continue
		}
		sessions[sessionID] = append(json.RawMessage(nil), body...)
	}
	return sessions, errorsBySession
}

func envOrDefault(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
