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
	"time"

	"genesis/internal/applications/connector_runtime"
)

type InspectionReport struct {
	Inbound        []connectorruntime.InboundSubmissionRecord               `json:"inbound"`
	Outbox         []connectorruntime.ConnectorOutboxItem                   `json:"outbox"`
	OutboxSummary  []OutboxInspectionSummary                                `json:"outbox_summary"`
	SourceFailures []connectorruntime.SourceFailureRecord                   `json:"source_failures,omitempty"`
	SourceRuns     []connectorruntime.SourceRun                             `json:"source_runs,omitempty"`
	SourceAttempts map[string][]connectorruntime.SourceAttempt              `json:"source_attempts,omitempty"`
	SourceCursors  []connectorruntime.SourceCursor                          `json:"source_cursors,omitempty"`
	SourceEvidence []connectorruntime.SourceVerificationEvidence            `json:"source_verification_evidence,omitempty"`
	SourceActions  map[string][]connectorruntime.SourceOperatorActionRecord `json:"source_operator_actions,omitempty"`
	Receipts       map[string][]connectorruntime.DeliveryReceipt            `json:"receipts"`
	KernelSessions map[string]json.RawMessage                               `json:"kernel_sessions,omitempty"`
	KernelErrors   map[string]string                                        `json:"kernel_errors,omitempty"`
}

const (
	OperatorActionNone                      = "none"
	OperatorActionDeliver                   = "deliver"
	OperatorActionAwaitRetry                = "await_retry"
	OperatorActionReviewDeadLetter          = "review_dead_letter"
	OperatorActionReconcileRecoveryRequired = "reconcile_recovery_required"
)

type OutboxInspectionSummary struct {
	OutboxID          string `json:"outbox_id"`
	Connector         string `json:"connector"`
	Status            string `json:"status"`
	AttemptCount      int    `json:"attempt_count"`
	ReceiptCount      int    `json:"receipt_count"`
	LastReceiptID     string `json:"last_receipt_id,omitempty"`
	LastReceiptStatus string `json:"last_receipt_status,omitempty"`
	LastReason        string `json:"last_reason,omitempty"`
	ExternalActionRef string `json:"external_action_ref,omitempty"`
	RecommendedAction string `json:"recommended_action"`
}

type InspectFilters struct {
	Connector       string
	InboundStatus   string
	OutboxStatus    string
	KernelSessionID string
}

type RecoveryResult struct {
	Item    connectorruntime.ConnectorOutboxItem `json:"item"`
	Receipt connectorruntime.DeliveryReceipt     `json:"receipt"`
}

type SourceLifecycleControlResult struct {
	Run            *connectorruntime.SourceRun                 `json:"run,omitempty"`
	Cursor         *connectorruntime.SourceCursor              `json:"cursor,omitempty"`
	OperatorAction connectorruntime.SourceOperatorActionRecord `json:"operator_action"`
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
	case "requeue-outbox":
		return runRequeueOutbox(ctx, args[1:], stdout, stderr)
	case "resolve-outbox":
		return runResolveOutbox(ctx, args[1:], stdout, stderr)
	case "source-clear-blocked":
		return runSourceClearBlocked(ctx, args[1:], stdout, stderr)
	case "source-request-restart":
		return runSourceRequestRestart(ctx, args[1:], stdout, stderr)
	case "source-reset-cursor":
		return runSourceResetCursor(ctx, args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runInspect(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	inboundPath := fs.String("inbound-state", envOrDefault("GENESIS_INGRESS_STATE", filepath.Join(".genesis_ingress", "state.json")), "connector inbound state file")
	outboxPath := fs.String("outbox-state", envOrDefault("GENESIS_CONNECTOR_OUTBOX_STATE", filepath.Join(".genesis_ingress", "outbox.json")), "connector outbox state file")
	sourceFailurePath := fs.String("source-state", envOrDefault("GENESIS_CONNECTOR_SOURCE_STATE", filepath.Join(".genesis_ingress", "source_failures.json")), "connector source failure state file")
	sourceLifecyclePath := fs.String("source-lifecycle-state", envOrDefault("GENESIS_CONNECTOR_SOURCE_LIFECYCLE_STATE", filepath.Join(".genesis_ingress", "source_lifecycle.json")), "connector source lifecycle state file")
	kernelURL := fs.String("kernel-url", os.Getenv("GENESIS_KERNEL_URL"), "optional Genesis Kernel HTTP URL for session projections")
	runtimeToken := fs.String("runtime-token", os.Getenv("GENESIS_RUNTIME_TOKEN"), "Genesis runtime bearer token")
	connector := fs.String("connector", "", "filter connector records by connector name")
	inboundStatus := fs.String("inbound-status", "", "filter inbound records by connector-local status")
	outboxStatus := fs.String("outbox-status", "", "filter outbox records by connector-local status")
	kernelSessionID := fs.String("kernel-session-id", "", "filter inbound records and kernel projection by kernel session id")
	if err := fs.Parse(args); err != nil {
		return err
	}

	filters := InspectFilters{
		Connector:       strings.TrimSpace(*connector),
		InboundStatus:   strings.TrimSpace(*inboundStatus),
		OutboxStatus:    strings.TrimSpace(*outboxStatus),
		KernelSessionID: strings.TrimSpace(*kernelSessionID),
	}
	report, err := inspectConnectorState(ctx, *inboundPath, *outboxPath, *sourceFailurePath, *sourceLifecyclePath, strings.TrimRight(*kernelURL, "/"), *runtimeToken, filters)
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

func runRequeueOutbox(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("requeue-outbox", flag.ContinueOnError)
	outboxPath := fs.String("outbox-state", envOrDefault("GENESIS_CONNECTOR_OUTBOX_STATE", filepath.Join(".genesis_ingress", "outbox.json")), "connector outbox state file")
	outboxID := fs.String("outbox-id", "", "connector outbox id to requeue")
	reason := fs.String("reason", "operator_requeued", "safe connector-local recovery reason")
	if err := fs.Parse(args); err != nil {
		return err
	}
	outboxStore, err := connectorruntime.NewFileOutboxStore(*outboxPath)
	if err != nil {
		return err
	}
	runtime := connectorruntime.Runtime{Store: outboxStore}
	item, receipt, err := runtime.RequeueOutboxItem(ctx, strings.TrimSpace(*outboxID), strings.TrimSpace(*reason))
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(RecoveryResult{Item: item, Receipt: receipt}); err != nil {
		return err
	}
	return nil
}

func runResolveOutbox(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("resolve-outbox", flag.ContinueOnError)
	outboxPath := fs.String("outbox-state", envOrDefault("GENESIS_CONNECTOR_OUTBOX_STATE", filepath.Join(".genesis_ingress", "outbox.json")), "connector outbox state file")
	outboxID := fs.String("outbox-id", "", "connector recovery-required outbox id to resolve")
	outcome := fs.String("outcome", "", "operator reconciliation outcome: sent or dead_lettered")
	reason := fs.String("reason", "", "safe connector-local reconciliation reason")
	externalActionRef := fs.String("external-action-ref", "", "optional safe external action reference observed during reconciliation")
	if err := fs.Parse(args); err != nil {
		return err
	}
	outboxStore, err := connectorruntime.NewFileOutboxStore(*outboxPath)
	if err != nil {
		return err
	}
	runtime := connectorruntime.Runtime{Store: outboxStore}
	item, receipt, err := runtime.ResolveRecoveryRequiredOutboxItem(ctx, strings.TrimSpace(*outboxID), strings.TrimSpace(*outcome), strings.TrimSpace(*reason), strings.TrimSpace(*externalActionRef))
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(RecoveryResult{Item: item, Receipt: receipt}); err != nil {
		return err
	}
	return nil
}

func runSourceClearBlocked(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("source-clear-blocked", flag.ContinueOnError)
	sourceLifecyclePath := fs.String("source-lifecycle-state", envOrDefault("GENESIS_CONNECTOR_SOURCE_LIFECYCLE_STATE", filepath.Join(".genesis_ingress", "source_lifecycle.json")), "connector source lifecycle state file")
	sourceID := fs.String("source-id", "", "connector source id to clear from blocked state")
	reason := fs.String("reason", "operator_cleared_blocked", "safe connector-local source recovery reason")
	if err := fs.Parse(args); err != nil {
		return err
	}
	store, err := connectorruntime.NewFileSourceLifecycleStore(*sourceLifecyclePath)
	if err != nil {
		return err
	}
	run, action, err := store.ClearBlockedSourceRun(ctx, strings.TrimSpace(*sourceID), strings.TrimSpace(*reason), time.Now())
	if err != nil {
		return err
	}
	return encodeSourceLifecycleControlResult(stdout, SourceLifecycleControlResult{Run: &run, OperatorAction: action})
}

func runSourceRequestRestart(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("source-request-restart", flag.ContinueOnError)
	sourceLifecyclePath := fs.String("source-lifecycle-state", envOrDefault("GENESIS_CONNECTOR_SOURCE_LIFECYCLE_STATE", filepath.Join(".genesis_ingress", "source_lifecycle.json")), "connector source lifecycle state file")
	sourceID := fs.String("source-id", "", "connector source id to request restart for")
	reason := fs.String("reason", "operator_requested_restart", "safe connector-local source restart reason")
	if err := fs.Parse(args); err != nil {
		return err
	}
	store, err := connectorruntime.NewFileSourceLifecycleStore(*sourceLifecyclePath)
	if err != nil {
		return err
	}
	action, err := store.RequestSourceRestart(ctx, strings.TrimSpace(*sourceID), strings.TrimSpace(*reason), time.Now())
	if err != nil {
		return err
	}
	return encodeSourceLifecycleControlResult(stdout, SourceLifecycleControlResult{OperatorAction: action})
}

func runSourceResetCursor(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("source-reset-cursor", flag.ContinueOnError)
	sourceLifecyclePath := fs.String("source-lifecycle-state", envOrDefault("GENESIS_CONNECTOR_SOURCE_LIFECYCLE_STATE", filepath.Join(".genesis_ingress", "source_lifecycle.json")), "connector source lifecycle state file")
	sourceID := fs.String("source-id", "", "connector source id whose cursor should be reset")
	cursorKind := fs.String("cursor-kind", connectorruntime.SourceCursorKindExternalEventID, "connector-owned cursor kind")
	cursorValue := fs.String("cursor-value", "", "connector-owned cursor value to write")
	reason := fs.String("reason", "operator_reset_cursor", "safe connector-local source cursor reset reason")
	acceptDuplicateRisk := fs.Bool("accept-duplicate-risk", false, "confirm that resetting this cursor can replay already accepted events")
	if err := fs.Parse(args); err != nil {
		return err
	}
	store, err := connectorruntime.NewFileSourceLifecycleStore(*sourceLifecyclePath)
	if err != nil {
		return err
	}
	cursor, action, err := store.ResetSourceCursor(ctx, strings.TrimSpace(*sourceID), strings.TrimSpace(*cursorKind), strings.TrimSpace(*cursorValue), strings.TrimSpace(*reason), *acceptDuplicateRisk, time.Now())
	if err != nil {
		return err
	}
	return encodeSourceLifecycleControlResult(stdout, SourceLifecycleControlResult{Cursor: &cursor, OperatorAction: action})
}

func encodeSourceLifecycleControlResult(stdout io.Writer, result SourceLifecycleControlResult) error {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func inspectConnectorState(ctx context.Context, inboundPath string, outboxPath string, sourceFailurePath string, sourceLifecyclePath string, kernelURL string, runtimeToken string, filters InspectFilters) (InspectionReport, error) {
	inboundStore, err := connectorruntime.NewFileInboundStore(inboundPath)
	if err != nil {
		return InspectionReport{}, err
	}
	outboxStore, err := connectorruntime.NewFileOutboxStore(outboxPath)
	if err != nil {
		return InspectionReport{}, err
	}
	sourceFailureStore, err := connectorruntime.NewFileSourceFailureStore(sourceFailurePath)
	if err != nil {
		return InspectionReport{}, err
	}
	sourceLifecycleStore, err := connectorruntime.NewFileSourceLifecycleStore(sourceLifecyclePath)
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
	sourceFailures, err := sourceFailureStore.ListSourceFailures(ctx)
	if err != nil {
		return InspectionReport{}, err
	}
	sourceRuns, err := sourceLifecycleStore.ListSourceRuns(ctx)
	if err != nil {
		return InspectionReport{}, err
	}
	sourceCursors, err := sourceLifecycleStore.ListSourceCursors(ctx)
	if err != nil {
		return InspectionReport{}, err
	}
	sourceEvidence, err := sourceLifecycleStore.ListSourceVerifications(ctx)
	if err != nil {
		return InspectionReport{}, err
	}
	inbound = filterInbound(inbound, filters)
	outbox = filterOutbox(outbox, filters)
	sourceFailures = filterSourceFailures(sourceFailures, filters)
	sourceRuns = filterSourceRuns(sourceRuns, filters)
	sourceCursors = filterSourceCursors(sourceCursors, sourceRuns)
	sourceEvidence = filterSourceEvidence(sourceEvidence, sourceRuns)
	sourceAttempts := map[string][]connectorruntime.SourceAttempt{}
	sourceActions := map[string][]connectorruntime.SourceOperatorActionRecord{}
	for _, run := range sourceRuns {
		attempts, err := sourceLifecycleStore.ListSourceAttempts(ctx, run.SourceID)
		if err != nil {
			return InspectionReport{}, err
		}
		if len(attempts) != 0 {
			sourceAttempts[run.SourceID] = attempts
		}
		actions, err := sourceLifecycleStore.ListSourceOperatorActions(ctx, run.SourceID)
		if err != nil {
			return InspectionReport{}, err
		}
		if len(actions) != 0 {
			sourceActions[run.SourceID] = actions
		}
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
		Inbound:        inbound,
		Outbox:         outbox,
		OutboxSummary:  summarizeOutbox(outbox, receipts),
		SourceFailures: sourceFailures,
		SourceRuns:     sourceRuns,
		SourceAttempts: sourceAttempts,
		SourceCursors:  sourceCursors,
		SourceEvidence: sourceEvidence,
		SourceActions:  sourceActions,
		Receipts:       receipts,
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

func summarizeOutbox(items []connectorruntime.ConnectorOutboxItem, receipts map[string][]connectorruntime.DeliveryReceipt) []OutboxInspectionSummary {
	summaries := make([]OutboxInspectionSummary, 0, len(items))
	for _, item := range items {
		itemReceipts := receipts[item.OutboxID]
		summary := OutboxInspectionSummary{
			OutboxID:          item.OutboxID,
			Connector:         item.Connector,
			Status:            item.Status,
			AttemptCount:      item.AttemptCount,
			ReceiptCount:      len(itemReceipts),
			LastReceiptID:     item.LastReceiptID,
			RecommendedAction: recommendedOperatorAction(item),
		}
		if len(itemReceipts) != 0 {
			lastReceipt := itemReceipts[len(itemReceipts)-1]
			summary.LastReceiptID = lastReceipt.ReceiptID
			summary.LastReceiptStatus = lastReceipt.Status
			summary.LastReason = lastReceipt.Reason
			summary.ExternalActionRef = lastReceipt.ExternalActionRef
		}
		summaries = append(summaries, summary)
	}
	return summaries
}

func recommendedOperatorAction(item connectorruntime.ConnectorOutboxItem) string {
	switch item.Status {
	case connectorruntime.OutboxStatusSent:
		return OperatorActionNone
	case connectorruntime.OutboxStatusQueued:
		return OperatorActionDeliver
	case connectorruntime.OutboxStatusRetrying:
		if item.NextAttemptAt.IsZero() || !item.NextAttemptAt.After(time.Now()) {
			return OperatorActionDeliver
		}
		return OperatorActionAwaitRetry
	case connectorruntime.OutboxStatusDeadLetter:
		return OperatorActionReviewDeadLetter
	case connectorruntime.OutboxStatusRecoveryRequired:
		return OperatorActionReconcileRecoveryRequired
	default:
		return OperatorActionReviewDeadLetter
	}
}

func filterInbound(records []connectorruntime.InboundSubmissionRecord, filters InspectFilters) []connectorruntime.InboundSubmissionRecord {
	if filters.Connector == "" && filters.InboundStatus == "" && filters.KernelSessionID == "" {
		return records
	}
	filtered := make([]connectorruntime.InboundSubmissionRecord, 0, len(records))
	for _, record := range records {
		if filters.Connector != "" && record.Connector != filters.Connector {
			continue
		}
		if filters.InboundStatus != "" && record.Status != filters.InboundStatus {
			continue
		}
		if filters.KernelSessionID != "" && record.KernelSessionID != filters.KernelSessionID {
			continue
		}
		filtered = append(filtered, record)
	}
	return filtered
}

func filterOutbox(items []connectorruntime.ConnectorOutboxItem, filters InspectFilters) []connectorruntime.ConnectorOutboxItem {
	if filters.Connector == "" && filters.OutboxStatus == "" {
		return items
	}
	filtered := make([]connectorruntime.ConnectorOutboxItem, 0, len(items))
	for _, item := range items {
		if filters.Connector != "" && item.Connector != filters.Connector {
			continue
		}
		if filters.OutboxStatus != "" && item.Status != filters.OutboxStatus {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func filterSourceFailures(records []connectorruntime.SourceFailureRecord, filters InspectFilters) []connectorruntime.SourceFailureRecord {
	if filters.Connector == "" {
		return records
	}
	filtered := make([]connectorruntime.SourceFailureRecord, 0, len(records))
	for _, record := range records {
		if record.Connector != filters.Connector {
			continue
		}
		filtered = append(filtered, record)
	}
	return filtered
}

func filterSourceRuns(runs []connectorruntime.SourceRun, filters InspectFilters) []connectorruntime.SourceRun {
	if filters.Connector == "" {
		return runs
	}
	filtered := make([]connectorruntime.SourceRun, 0, len(runs))
	for _, run := range runs {
		if run.Connector == filters.Connector {
			filtered = append(filtered, run)
		}
	}
	return filtered
}

func filterSourceCursors(cursors []connectorruntime.SourceCursor, runs []connectorruntime.SourceRun) []connectorruntime.SourceCursor {
	if len(runs) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(runs))
	for _, run := range runs {
		allowed[run.SourceID] = struct{}{}
	}
	filtered := make([]connectorruntime.SourceCursor, 0, len(cursors))
	for _, cursor := range cursors {
		if _, ok := allowed[cursor.SourceID]; ok {
			filtered = append(filtered, cursor)
		}
	}
	return filtered
}

func filterSourceEvidence(evidence []connectorruntime.SourceVerificationEvidence, runs []connectorruntime.SourceRun) []connectorruntime.SourceVerificationEvidence {
	if len(runs) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(runs))
	for _, run := range runs {
		allowed[run.AdapterRef] = struct{}{}
	}
	filtered := make([]connectorruntime.SourceVerificationEvidence, 0, len(evidence))
	for _, item := range evidence {
		if _, ok := allowed[item.AdapterRef]; ok {
			filtered = append(filtered, item)
		}
	}
	return filtered
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
