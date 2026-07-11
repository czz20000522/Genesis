package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"genesis/internal/applications/connector_runtime"
	feishucli "genesis/internal/applications/feishu_cli"
	"genesis/localconfig"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	return runWithConfig(ctx, args, os.Stdin, os.Stdout, os.Stderr, true)
}

func runWithIO(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	return runWithConfig(ctx, args, stdin, stdout, stderr, false)
}

func runWithConfig(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer, requireConnectorBinding bool) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: genesis-ingress <console-once|feishu-once|feishu-listen|feishu-probe> [flags]")
	}
	switch args[0] {
	case "console-once":
		return runOnce(ctx, args[1:], "console", true, stdin, stdout)
	case "feishu-once":
		return runOnce(ctx, args[1:], "feishu", false, stdin, stdout)
	case "feishu-listen":
		return runFeishuListen(ctx, args[1:], stdin, stdout, stderr, requireConnectorBinding)
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

func runFeishuListen(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer, requireConnectorBinding bool) error {
	fs := flag.NewFlagSet("feishu-listen", flag.ContinueOnError)
	flags := registerCommonFlags(fs, "feishu")
	configRoot := fs.String("config-root", "", "Genesis Home config directory containing runtime-settings.json")
	profile := fs.String("profile", "", "explicit lark-cli profile passed to the Feishu connector adapter")
	profileReadiness := fs.String("profile-readiness", "ok", "connector-local Feishu profile readiness posture: ok, missing_profile, profile_expired, permission_denied, or refresh_required")
	profileProbeCommand := fs.String("profile-probe-command", "", "direct profile readiness probe executable; emits typed readiness JSON and does not start source or delivery adapters")
	profileProbeTimeout := fs.Duration("profile-probe-timeout", 0, "bounded timeout for the profile readiness probe command")
	stdinJSONL := fs.Bool("stdin-jsonl", false, "read ExternalEvent NDJSON from stdin")
	larkCLI := fs.String("lark-cli", os.Getenv("GENESIS_FEISHU_CLI_EXECUTABLE"), "direct lark-cli executable passed to the Feishu connector adapter")
	identity := fs.String("as", "bot", "Feishu event source identity: bot, user, or auto")
	deliveryCommand := fs.String("delivery-command", envOrDefault("GENESIS_FEISHU_CONNECTOR_COMMAND", "genesis-feishu-connector-adapter"), "direct Feishu connector adapter executable for final delivery")
	sourceCommand := fs.String("source-command", "", "direct source adapter executable that emits source_command NDJSON frames")
	sourceID := fs.String("source-id", "feishu.im.message.receive", "stable connector source id; do not include profile or credential material")
	sourceAdapterRef := fs.String("source-adapter-ref", "feishu-source-adapter", "connector-local source adapter reference")
	sourceAttempts := fs.Int("source-attempts", 1, "maximum source_command attempts after recoverable runtime failures")
	sourceBackoff := fs.Duration("source-backoff", time.Second, "backoff between recoverable source_command attempts")
	sourceIdleTimeout := fs.Duration("source-idle-timeout", 0, "maximum time without a source frame before killing and retrying the source command; 0 disables idle timeout")
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
	var profileProbeCommandArgs stringListFlag
	fs.Var(&profileProbeCommandArgs, "profile-probe-command-arg", "argument passed to the profile readiness probe executable before generated profile flags; repeatable")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *sourceAttempts < 1 {
		return fmt.Errorf("feishu-listen --source-attempts must be at least 1")
	}
	if *sourceBackoff < 0 {
		return fmt.Errorf("feishu-listen --source-backoff must not be negative")
	}
	if *sourceIdleTimeout < 0 {
		return fmt.Errorf("feishu-listen --source-idle-timeout must not be negative")
	}
	binding := feishuListenerBinding{Profile: *profile, Command: *larkCLI, Identity: *identity}
	if requireConnectorBinding && !*stdinJSONL {
		var err error
		binding, err = readFeishuListenerBinding(*configRoot)
		if err != nil {
			return err
		}
		*profile, *larkCLI, *identity = binding.Profile, binding.Command, binding.Identity
		*profileProbeCommand, profileProbeCommandArgs = defaultFeishuProfileProbeCommand(true, *profileProbeCommand, profileProbeCommandArgs, *sourceCommand)
	}
	_, runtime, err := buildRuntime(flags, stdin)
	if err != nil {
		return err
	}
	profileBlockedReason := ""
	if *deliverFinal || strings.TrimSpace(*sourceCommand) != "" {
		profileBlockedReason, err = feishuProfileReadinessBlockReason(ctx, *profile, *profileReadiness, *profileProbeCommand, profileProbeCommandArgs, *profileProbeTimeout)
		if err != nil {
			return err
		}
	}
	if *deliverFinal {
		finalDeliveryBlockedReason := profileBlockedReason
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
		sourceReadinessBlockReason = profileBlockedReason
		if sourceReadinessBlockReason == "" {
			sourceArgs = feishuSourceCommandArgs(sourceArgs, *profile, *larkCLI, *sourceID, *identity)
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
		RestrictToAllowedThreads:  binding.RestrictToAllowedThreads,
		AllowedThreadIDs:          append([]string(nil), binding.AllowedThreadIDs...),
		ReadinessBlockReasonCode:  sourceReadinessBlockReason,
		ReadinessBlockDescription: sourceReadinessBlockDescription,
		IdleTimeout:               *sourceIdleTimeout,
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

func runFeishuProbe(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("feishu-probe", flag.ContinueOnError)
	profile := fs.String("profile", "", "explicit lark-cli profile used by the Feishu connector probe")
	profileReadiness := fs.String("profile-readiness", "ok", "connector-local Feishu profile readiness posture: ok, missing_profile, profile_expired, permission_denied, or refresh_required")
	profileProbeCommand := fs.String("profile-probe-command", "", "direct profile readiness probe executable; emits typed readiness JSON and does not start source or delivery adapters")
	profileProbeTimeout := fs.Duration("profile-probe-timeout", 0, "bounded timeout for the profile readiness probe command")
	larkCLI := fs.String("lark-cli", os.Getenv("GENESIS_FEISHU_CLI_EXECUTABLE"), "direct lark-cli executable passed to the Feishu connector adapter")
	deliveryCommand := fs.String("delivery-command", envOrDefault("GENESIS_FEISHU_CONNECTOR_COMMAND", "genesis-feishu-connector-adapter"), "direct Feishu connector adapter executable to validate")
	sourceCommand := fs.String("source-command", "", "direct source adapter executable to validate")
	var sourceCommandArgs stringListFlag
	fs.Var(&sourceCommandArgs, "source-command-arg", "argument passed to the source adapter executable; repeatable")
	var deliveryCommandArgs stringListFlag
	fs.Var(&deliveryCommandArgs, "delivery-command-arg", "argument passed to the Feishu connector adapter executable before generated profile flags; repeatable")
	var profileProbeCommandArgs stringListFlag
	fs.Var(&profileProbeCommandArgs, "profile-probe-command-arg", "argument passed to the profile readiness probe executable before generated profile flags; repeatable")
	if err := fs.Parse(args); err != nil {
		return err
	}
	finalDeliveryBlockedReason, err := feishuProfileReadinessBlockReason(ctx, *profile, *profileReadiness, *profileProbeCommand, profileProbeCommandArgs, *profileProbeTimeout)
	if err != nil {
		return err
	}
	sourceBlockedReason := ""
	sourceArgs := append([]string(nil), sourceCommandArgs...)
	if strings.TrimSpace(*sourceCommand) != "" {
		sourceBlockedReason = finalDeliveryBlockedReason
		if sourceBlockedReason == "" {
			sourceArgs = feishuSourceCommandArgs(sourceArgs, *profile, *larkCLI, "feishu.im.message.receive", "bot")
		}
	}
	finalDeliveryArgs := []string(nil)
	if finalDeliveryBlockedReason == "" {
		finalDeliveryArgs = feishuConnectorCommandArgs(append([]string(nil), deliveryCommandArgs...), *profile, *larkCLI)
	}
	report := feishucli.ProbeAdapter(feishucli.AdapterProbeConfig{
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

func feishuSourceCommandArgs(prefix []string, profile string, larkCLI string, sourceID string, identity string) []string {
	args := append([]string(nil), prefix...)
	args = append(args, "--profile", strings.TrimSpace(profile), "--source-id", strings.TrimSpace(sourceID))
	if strings.TrimSpace(larkCLI) != "" {
		args = append(args, "--lark-cli", strings.TrimSpace(larkCLI))
	}
	if strings.TrimSpace(identity) != "" {
		args = append(args, "--as", strings.TrimSpace(identity))
	}
	return args
}

type feishuListenerBinding struct {
	Profile                  string
	Command                  string
	Identity                 string
	RestrictToAllowedThreads bool
	AllowedThreadIDs         []string
}

func readFeishuListenerBinding(configRoot string) (feishuListenerBinding, error) {
	settings, err := localconfig.ReadRuntimeSettings(localconfig.RuntimeSettingsPath(configRoot))
	if err != nil {
		return feishuListenerBinding{}, fmt.Errorf("read Feishu listener binding: %w", err)
	}
	if !settings.Feishu.Listener.Enabled {
		return feishuListenerBinding{}, errors.New("Feishu listener is disabled by runtime-settings.json")
	}
	profile := strings.TrimSpace(settings.Feishu.LarkCLI.Profile)
	if profile == "" {
		return feishuListenerBinding{}, errors.New("Feishu listener binding requires lark_cli.profile")
	}
	identity := strings.TrimSpace(settings.Feishu.LarkCLI.Identity)
	if identity == "" {
		identity = "bot"
	}
	allowedThreadIDs := nonEmptyStringValues(settings.Feishu.AllowedChatIDs)
	restricted := !settings.Feishu.AllowUnboundChats
	if restricted && len(allowedThreadIDs) == 0 {
		return feishuListenerBinding{}, errors.New("Feishu listener binding requires allowed_chat_ids when allow_unbound_chats is false")
	}
	return feishuListenerBinding{
		Profile:                  profile,
		Command:                  strings.TrimSpace(settings.Feishu.LarkCLI.Command),
		Identity:                 identity,
		RestrictToAllowedThreads: restricted,
		AllowedThreadIDs:         allowedThreadIDs,
	}, nil
}

func nonEmptyStringValues(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func defaultFeishuProfileProbeCommand(boundListener bool, configuredCommand string, configuredArgs []string, sourceCommand string) (string, []string) {
	if !boundListener || strings.TrimSpace(configuredCommand) != "" || strings.TrimSpace(sourceCommand) == "" {
		return strings.TrimSpace(configuredCommand), append([]string(nil), configuredArgs...)
	}
	return strings.TrimSpace(sourceCommand), []string{"--profile-probe"}
}

func feishuProfileReadinessBlockReason(ctx context.Context, profile string, readiness string, probeCommand string, probeArgs []string, probeTimeout time.Duration) (string, error) {
	blockReason, err := connectorruntime.ResolveProfileReadiness(ctx, profile, readiness, connectorruntime.ProfileReadinessCommandProbe{
		Executable: probeCommand,
		Args:       append([]string(nil), probeArgs...),
		Timeout:    probeTimeout,
	})
	if err != nil {
		return blockReason, fmt.Errorf("Feishu profile readiness probe failed: %w", err)
	}
	return blockReason, nil
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
