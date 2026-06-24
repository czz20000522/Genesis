package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"genesis/internal/applications/connector_runtime"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	return runWithIO(ctx, args, os.Stdin, os.Stdout, os.Stderr)
}

func runWithIO(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: genesis-ingress <console-once|feishu-once|feishu-listen|feishu-probe> [flags]")
	}
	switch args[0] {
	case "console-once":
		return runOnce(ctx, args[1:], "console", true, stdin, stdout)
	case "feishu-once":
		return runOnce(ctx, args[1:], "feishu", false, stdin, stdout)
	case "feishu-listen":
		return runFeishuListen(ctx, args[1:], stdin, stdout, stderr)
	case "feishu-probe":
		return runFeishuProbe(ctx, args[1:], stdout)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runOnce(ctx context.Context, args []string, defaultChannel string, printFinal bool, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet(defaultChannel+"-once", flag.ContinueOnError)
	flags := registerCommonFlags(fs, defaultChannel)
	if err := fs.Parse(args); err != nil {
		return err
	}
	msg, runtime, err := buildRuntime(flags, stdin)
	if err != nil {
		return err
	}
	result, err := runtime.ProcessExternalEvent(ctx, msg)
	if printFinal && result.FinalText != "" {
		fmt.Fprintln(stdout, result.FinalText)
	}
	if err != nil {
		return err
	}
	_ = json.NewEncoder(stdout).Encode(result)
	return nil
}

func runFeishuListen(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("feishu-listen", flag.ContinueOnError)
	flags := registerCommonFlags(fs, "feishu")
	profile := fs.String("profile", "", "explicit lark-cli profile passed to the Feishu connector adapter")
	profileReadiness := fs.String("profile-readiness", "ok", "connector-local Feishu profile readiness posture: ok, missing_profile, profile_expired, permission_denied, or refresh_required")
	stdinJSONL := fs.Bool("stdin-jsonl", false, "read ExternalEvent NDJSON from stdin")
	larkCLI := fs.String("lark-cli", os.Getenv("GENESIS_FEISHU_CLI_EXECUTABLE"), "direct lark-cli executable passed to the Feishu connector adapter")
	deliveryCommand := fs.String("delivery-command", envOrDefault("GENESIS_FEISHU_CONNECTOR_COMMAND", "genesis-feishu-connector-adapter"), "direct Feishu connector adapter executable for final delivery")
	sourceCommand := fs.String("source-command", "", "direct source adapter executable that emits source_command NDJSON frames")
	sourceID := fs.String("source-id", "feishu.im.message.receive", "stable connector source id; do not include profile or credential material")
	sourceAdapterRef := fs.String("source-adapter-ref", "feishu-source-adapter", "connector-local source adapter reference")
	sourceAttempts := fs.Int("source-attempts", 1, "maximum source_command attempts after recoverable runtime failures")
	sourceBackoff := fs.Duration("source-backoff", time.Second, "backoff between recoverable source_command attempts")
	deliverFinal := fs.Bool("deliver-final", false, "enqueue and deliver kernel final_text back through the connector outbox")
	outboxPath := fs.String("outbox-state", envOrDefault("GENESIS_CONNECTOR_OUTBOX_STATE", filepath.Join(".genesis_ingress", "outbox.json")), "connector outbox state file")
	sourceFailurePath := fs.String("source-state", envOrDefault("GENESIS_CONNECTOR_SOURCE_STATE", filepath.Join(".genesis_ingress", "source_failures.json")), "connector source failure state file")
	sourceLifecyclePath := fs.String("source-lifecycle-state", envOrDefault("GENESIS_CONNECTOR_SOURCE_LIFECYCLE_STATE", filepath.Join(".genesis_ingress", "source_lifecycle.json")), "connector source lifecycle state file")
	var ignoreSenderIDs stringListFlag
	fs.Var(&ignoreSenderIDs, "ignore-sender-id", "external sender id to ignore before kernel submission; repeatable")
	var sourceCommandArgs stringListFlag
	fs.Var(&sourceCommandArgs, "source-command-arg", "argument passed to the source adapter executable; repeatable")
	var deliveryCommandArgs stringListFlag
	fs.Var(&deliveryCommandArgs, "delivery-command-arg", "argument passed to the Feishu connector adapter executable before generated profile flags; repeatable")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *sourceAttempts < 1 {
		return fmt.Errorf("feishu-listen --source-attempts must be at least 1")
	}
	if *sourceBackoff < 0 {
		return fmt.Errorf("feishu-listen --source-backoff must not be negative")
	}
	_, runtime, err := buildRuntime(flags, stdin)
	if err != nil {
		return err
	}
	if *deliverFinal {
		finalDeliveryBlockedReason, err := feishuProfileReadinessBlockReason(*profile, *profileReadiness)
		if err != nil {
			return err
		}
		if finalDeliveryBlockedReason != "" {
			return fmt.Errorf("feishu-listen final delivery blocked by profile readiness: %s", finalDeliveryBlockedReason)
		}
		finalDeliveryArgs := feishuConnectorCommandArgs(append([]string(nil), deliveryCommandArgs...), *profile, *larkCLI)
		if err := configureFeishuFinalDelivery(runtime, *outboxPath, *deliveryCommand, finalDeliveryArgs); err != nil {
			return err
		}
	}
	if *stdinJSONL {
		return processExternalEventJSONL(ctx, runtime, stdin, stdout, stderr)
	}
	sourceFailureStore, err := connectorruntime.NewFileSourceFailureStore(*sourceFailurePath)
	if err != nil {
		return err
	}
	sourceLifecycleStore, err := connectorruntime.NewFileSourceLifecycleStore(*sourceLifecyclePath)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(stdout)
	sourceReadinessBlockReason := ""
	sourceReadinessBlockDescription := ""
	sourceArgs := append([]string(nil), sourceCommandArgs...)
	if strings.TrimSpace(*sourceCommand) != "" {
		sourceReadinessBlockReason, err = feishuProfileReadinessBlockReason(*profile, *profileReadiness)
		if err != nil {
			return err
		}
		if sourceReadinessBlockReason == "" {
			sourceArgs = feishuSourceCommandArgs(sourceArgs, *profile, *larkCLI, *sourceID)
		} else {
			sourceReadinessBlockDescription = sourceReadinessBlockReason
		}
	}
	adapter := connectorruntime.SourceCommandAdapter{
		Executable:                *sourceCommand,
		Args:                      sourceArgs,
		SourceID:                  *sourceID,
		Connector:                 "feishu",
		AdapterRef:                *sourceAdapterRef,
		SourceStore:               sourceLifecycleStore,
		FailureStore:              sourceFailureStore,
		IgnoreSenderIDs:           append([]string(nil), ignoreSenderIDs...),
		ReadinessBlockReasonCode:  sourceReadinessBlockReason,
		ReadinessBlockDescription: sourceReadinessBlockDescription,
	}
	intake := connectorruntime.SourceCommandIntake{
		Adapter: adapter,
		Retry: connectorruntime.SourceCommandRetryPolicy{
			MaxAttempts: *sourceAttempts,
			Backoff:     *sourceBackoff,
		},
	}
	return intake.Run(ctx, func(event connectorruntime.ExternalEvent) error {
		result, err := runtime.ProcessSourceCommandEvent(ctx, event)
		if encodeErr := encoder.Encode(result); encodeErr != nil {
			return encodeErr
		}
		return err
	})
}

func runFeishuProbe(_ context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("feishu-probe", flag.ContinueOnError)
	profile := fs.String("profile", "", "explicit lark-cli profile used by the Feishu connector probe")
	profileReadiness := fs.String("profile-readiness", "ok", "connector-local Feishu profile readiness posture: ok, missing_profile, profile_expired, permission_denied, or refresh_required")
	larkCLI := fs.String("lark-cli", os.Getenv("GENESIS_FEISHU_CLI_EXECUTABLE"), "direct lark-cli executable passed to the Feishu connector adapter")
	deliveryCommand := fs.String("delivery-command", envOrDefault("GENESIS_FEISHU_CONNECTOR_COMMAND", "genesis-feishu-connector-adapter"), "direct Feishu connector adapter executable to validate")
	sourceCommand := fs.String("source-command", "", "direct source adapter executable to validate")
	var sourceCommandArgs stringListFlag
	fs.Var(&sourceCommandArgs, "source-command-arg", "argument passed to the source adapter executable; repeatable")
	var deliveryCommandArgs stringListFlag
	fs.Var(&deliveryCommandArgs, "delivery-command-arg", "argument passed to the Feishu connector adapter executable before generated profile flags; repeatable")
	if err := fs.Parse(args); err != nil {
		return err
	}
	finalDeliveryBlockedReason, err := feishuProfileReadinessBlockReason(*profile, *profileReadiness)
	if err != nil {
		return err
	}
	sourceBlockedReason := ""
	sourceArgs := append([]string(nil), sourceCommandArgs...)
	if strings.TrimSpace(*sourceCommand) != "" {
		sourceBlockedReason = finalDeliveryBlockedReason
		if sourceBlockedReason == "" {
			sourceArgs = feishuSourceCommandArgs(sourceArgs, *profile, *larkCLI, "feishu.im.message.receive")
		}
	}
	finalDeliveryArgs := []string(nil)
	if finalDeliveryBlockedReason == "" {
		finalDeliveryArgs = feishuConnectorCommandArgs(append([]string(nil), deliveryCommandArgs...), *profile, *larkCLI)
	}
	report := connectorruntime.ProbeFeishuAdapter(connectorruntime.FeishuAdapterProbeConfig{
		SourceCommand:              *sourceCommand,
		SourceCommandArgs:          sourceArgs,
		SourceBlockedReason:        sourceBlockedReason,
		FinalDeliveryCommand:       *deliveryCommand,
		FinalDeliveryCommandArgs:   finalDeliveryArgs,
		FinalDeliveryBlockedReason: finalDeliveryBlockedReason,
	})
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return err
	}
	if !report.Ready {
		return fmt.Errorf("Feishu connector probe failed")
	}
	return nil
}

func configureFeishuFinalDelivery(runtime *connectorruntime.Runtime, outboxPath string, executable string, args []string) error {
	store, err := connectorruntime.NewFileOutboxStore(outboxPath)
	if err != nil {
		return err
	}
	runtime.Store = store
	runtime.Adapters = map[string]connectorruntime.ConnectorAdapter{
		"feishu": connectorruntime.ConnectorCommandAdapter{
			Executable: executable,
			Args:       append([]string(nil), args...),
		},
	}
	return nil
}

func feishuConnectorCommandArgs(prefix []string, profile string, larkCLI string) []string {
	args := append([]string(nil), prefix...)
	args = append(args, "--profile", strings.TrimSpace(profile))
	if strings.TrimSpace(larkCLI) != "" {
		args = append(args, "--lark-cli", strings.TrimSpace(larkCLI))
	}
	return args
}

func feishuSourceCommandArgs(prefix []string, profile string, larkCLI string, sourceID string) []string {
	args := append([]string(nil), prefix...)
	args = append(args, "--profile", strings.TrimSpace(profile), "--source-id", strings.TrimSpace(sourceID))
	if strings.TrimSpace(larkCLI) != "" {
		args = append(args, "--lark-cli", strings.TrimSpace(larkCLI))
	}
	return args
}

func feishuProfileReadinessBlockReason(profile string, readiness string) (string, error) {
	readiness = strings.TrimSpace(readiness)
	if readiness == "" || readiness == "ok" {
		if strings.TrimSpace(profile) == "" {
			return connectorruntime.SourceReadinessReasonMissingProfile, nil
		}
		return "", nil
	}
	if !connectorruntime.ValidSourceReadinessReasonCode(readiness) {
		return "", fmt.Errorf("Feishu profile readiness must be ok or a known source readiness reason")
	}
	return readiness, nil
}

func processExternalEventJSONL(ctx context.Context, runtime *connectorruntime.Runtime, reader io.Reader, stdout io.Writer, stderr io.Writer) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	encoder := json.NewEncoder(stdout)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event connectorruntime.ExternalEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return fmt.Errorf("decode external event line %d: %w", lineNumber, err)
		}
		result, err := runtime.ProcessExternalEvent(ctx, event)
		if encodeErr := encoder.Encode(result); encodeErr != nil {
			return encodeErr
		}
		if err != nil {
			fmt.Fprintf(stderr, "external event line %d failed: %v\n", lineNumber, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

type commonFlags struct {
	kernelURL    *string
	runtimeToken *string
	statePath    *string
	connector    *string
	messageID    *string
	threadID     *string
	userID       *string
	text         *string
	stdinJSON    *bool
	chatID       *string
	senderName   *string
}

func registerCommonFlags(fs *flag.FlagSet, defaultChannel string) commonFlags {
	return commonFlags{
		kernelURL:    fs.String("kernel-url", envOrDefault("GENESIS_KERNEL_URL", "http://127.0.0.1:8765"), "Genesis Kernel HTTP URL"),
		runtimeToken: fs.String("runtime-token", os.Getenv("GENESIS_RUNTIME_TOKEN"), "Genesis runtime bearer token"),
		statePath:    fs.String("state", envOrDefault("GENESIS_INGRESS_STATE", filepath.Join(".genesis_ingress", "state.json")), "application-local ingress state file"),
		connector:    fs.String("connector", defaultChannel, "connector name"),
		messageID:    fs.String("message-id", "", "external message id"),
		threadID:     fs.String("thread-id", "", "external thread/chat id"),
		userID:       fs.String("user-id", "", "external user id"),
		text:         fs.String("text", "", "message text"),
		stdinJSON:    fs.Bool("stdin-json", false, "read ExternalEvent JSON from stdin"),
		chatID:       fs.String("chat-id", "", "external chat id to expose as inbound reply reference"),
		senderName:   fs.String("sender-display", "", "external sender display name"),
	}
}

func buildRuntime(flags commonFlags, stdin io.Reader) (connectorruntime.ExternalEvent, *connectorruntime.Runtime, error) {
	threadID := *flags.threadID
	if *flags.chatID != "" {
		threadID = *flags.chatID
	}
	event := connectorruntime.ExternalEvent{
		Connector:       *flags.connector,
		ExternalEventID: *flags.messageID,
		EventType:       "message.created",
		ThreadRef: connectorruntime.ExternalThreadRef{
			Connector:  *flags.connector,
			Kind:       threadKind(*flags.connector),
			ExternalID: threadID,
		},
		SenderRef: connectorruntime.ExternalRef{
			Connector:  *flags.connector,
			Kind:       "user",
			ExternalID: *flags.userID,
			Display:    *flags.senderName,
		},
		MessageRef: connectorruntime.ExternalRef{
			Connector:  *flags.connector,
			Kind:       "message",
			ExternalID: *flags.messageID,
		},
		Body:             *flags.text,
		SourceValidation: connectorruntime.SourceValidationUnchecked,
	}
	if *flags.stdinJSON {
		if err := json.NewDecoder(stdin).Decode(&event); err != nil {
			return connectorruntime.ExternalEvent{}, nil, err
		}
	}
	store, err := connectorruntime.NewFileInboundStore(*flags.statePath)
	if err != nil {
		return connectorruntime.ExternalEvent{}, nil, err
	}
	runtime := &connectorruntime.Runtime{
		InboundStore: store,
		Client: connectorruntime.HTTPKernelClient{
			BaseURL:      *flags.kernelURL,
			RuntimeToken: *flags.runtimeToken,
		},
		SessionMapper: connectorruntime.DefaultApplicationSessionMapper{},
	}
	return event, runtime, nil
}

func envOrDefault(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func threadKind(connector string) string {
	if connector == "feishu" {
		return "chat"
	}
	return "conversation"
}

type stringListFlag []string

func (f *stringListFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *stringListFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value != "" {
		*f = append(*f, value)
	}
	return nil
}
