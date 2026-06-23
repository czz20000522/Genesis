package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"genesis/internal/applications/connector_runtime"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: genesis-ingress <console-once|feishu-once> [flags]")
	}
	switch args[0] {
	case "console-once":
		return runOnce(ctx, args[1:], "console", true)
	case "feishu-once":
		return runOnce(ctx, args[1:], "feishu", false)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runOnce(ctx context.Context, args []string, defaultChannel string, printFinal bool) error {
	fs := flag.NewFlagSet(defaultChannel+"-once", flag.ContinueOnError)
	flags := registerCommonFlags(fs, defaultChannel)
	if err := fs.Parse(args); err != nil {
		return err
	}
	msg, runtime, err := buildRuntime(flags)
	if err != nil {
		return err
	}
	result, err := runtime.ProcessExternalEvent(ctx, msg)
	if printFinal && result.FinalText != "" {
		fmt.Println(result.FinalText)
	}
	if err != nil {
		return err
	}
	_ = json.NewEncoder(os.Stdout).Encode(result)
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

func buildRuntime(flags commonFlags) (connectorruntime.ExternalEvent, *connectorruntime.Runtime, error) {
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
		if err := json.NewDecoder(os.Stdin).Decode(&event); err != nil {
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
