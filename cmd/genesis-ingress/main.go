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
	profile := fs.String("profile", "", "explicit lark-cli profile used by the Feishu event source")
	stdinJSONL := fs.Bool("stdin-jsonl", false, "read ExternalEvent NDJSON from stdin")
	larkCLI := fs.String("lark-cli", os.Getenv("GENESIS_FEISHU_CLI_EXECUTABLE"), "direct lark-cli executable for Feishu event source")
	eventKey := fs.String("event-key", connectorruntime.DefaultFeishuMessageEventKey, "Feishu event key to consume")
	eventIdentity := fs.String("as", "bot", "Feishu event source identity: bot, user, or auto")
	maxEvents := fs.Int("max-events", 0, "stop Feishu event source after N events; 0 means unlimited")
	eventTimeout := fs.String("event-timeout", "", "stop Feishu event source after duration such as 30s or 10m")
	sourceAttempts := fs.Int("source-attempts", 1, "maximum Feishu event source attempts after recoverable runtime failures")
	sourceBackoff := fs.String("source-backoff", "1s", "backoff between Feishu event source retry attempts")
	deliverFinal := fs.Bool("deliver-final", false, "enqueue and deliver kernel final_text back through the connector outbox")
	outboxPath := fs.String("outbox-state", envOrDefault("GENESIS_CONNECTOR_OUTBOX_STATE", filepath.Join(".genesis_ingress", "outbox.json")), "connector outbox state file")
	sourceFailurePath := fs.String("source-state", envOrDefault("GENESIS_CONNECTOR_SOURCE_STATE", filepath.Join(".genesis_ingress", "source_failures.json")), "connector source failure state file")
	sourceSupervisorPath := fs.String("source-supervisor-state", envOrDefault("GENESIS_CONNECTOR_SOURCE_SUPERVISOR_STATE", filepath.Join(".genesis_ingress", "source_supervisor.json")), "connector source supervisor state file")
	var ignoreSenderIDs stringListFlag
	fs.Var(&ignoreSenderIDs, "ignore-sender-id", "external sender id to ignore before kernel submission; repeatable")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*profile) == "" {
		return fmt.Errorf("feishu-listen requires explicit --profile")
	}
	if *sourceAttempts < 1 {
		return fmt.Errorf("feishu-listen --source-attempts must be at least 1")
	}
	parsedSourceBackoff, err := time.ParseDuration(strings.TrimSpace(*sourceBackoff))
	if err != nil {
		return fmt.Errorf("parse --source-backoff: %w", err)
	}
	if parsedSourceBackoff < 0 {
		return fmt.Errorf("feishu-listen --source-backoff must be non-negative")
	}
	_, runtime, err := buildRuntime(flags, stdin)
	if err != nil {
		return err
	}
	if *deliverFinal {
		if err := configureFeishuFinalDelivery(runtime, *outboxPath, *profile, *larkCLI); err != nil {
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
	sourceSupervisorStore, err := connectorruntime.NewFileSourceSupervisorStore(*sourceSupervisorPath)
	if err != nil {
		return err
	}
	sourceConfig := connectorruntime.FeishuEventSourceConfig{
		Executable:      *larkCLI,
		Profile:         *profile,
		EventKey:        *eventKey,
		Identity:        *eventIdentity,
		MaxEvents:       *maxEvents,
		Timeout:         *eventTimeout,
		IgnoreSenderIDs: append([]string(nil), ignoreSenderIDs...),
		FailureStore:    sourceFailureStore,
		SourceStore:     sourceSupervisorStore,
	}
	encoder := json.NewEncoder(stdout)
	return connectorruntime.ConsumeFeishuEventSourceWithRetry(ctx, sourceConfig, connectorruntime.FeishuEventSourceRetryPolicy{
		MaxAttempts: *sourceAttempts,
		Backoff:     parsedSourceBackoff,
	}, stderr, func(event connectorruntime.ExternalEvent) error {
		result, err := runtime.ProcessExternalEvent(ctx, event)
		if encodeErr := encoder.Encode(result); encodeErr != nil {
			return encodeErr
		}
		return err
	})
}

func runFeishuProbe(_ context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("feishu-probe", flag.ContinueOnError)
	profile := fs.String("profile", "", "explicit lark-cli profile used by the Feishu connector probe")
	larkCLI := fs.String("lark-cli", os.Getenv("GENESIS_FEISHU_CLI_EXECUTABLE"), "direct lark-cli executable for Feishu connector probe")
	eventKey := fs.String("event-key", connectorruntime.DefaultFeishuMessageEventKey, "Feishu event key to validate")
	eventIdentity := fs.String("as", "bot", "Feishu event source identity: bot, user, or auto")
	if err := fs.Parse(args); err != nil {
		return err
	}
	report := connectorruntime.ProbeFeishuAdapter(connectorruntime.FeishuAdapterProbeConfig{
		Executable: *larkCLI,
		Profile:    *profile,
		EventKey:   *eventKey,
		Identity:   *eventIdentity,
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

func configureFeishuFinalDelivery(runtime *connectorruntime.Runtime, outboxPath string, profile string, executable string) error {
	store, err := connectorruntime.NewFileOutboxStore(outboxPath)
	if err != nil {
		return err
	}
	runtime.Store = store
	runtime.Adapters = map[string]connectorruntime.ConnectorAdapter{
		"feishu": connectorruntime.NewFeishuSendMessageCommandTemplateDriver(profile, executable, nil),
	}
	return nil
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
