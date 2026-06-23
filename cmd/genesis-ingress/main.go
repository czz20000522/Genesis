package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"genesis/internal/applications/message_ingress"
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
		return runOnce(ctx, args[1:], "console", "stdin", true)
	case "feishu-once":
		return runOnce(ctx, args[1:], "feishu", "feishu-inbound", false)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runOnce(ctx context.Context, args []string, defaultChannel, defaultAdapter string, printFinal bool) error {
	fs := flag.NewFlagSet(defaultChannel+"-once", flag.ContinueOnError)
	flags := registerCommonFlags(fs, defaultChannel, defaultAdapter)
	if err := fs.Parse(args); err != nil {
		return err
	}
	msg, runtime, err := buildRuntime(flags)
	if err != nil {
		return err
	}
	result, err := runtime.Process(ctx, msg)
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
	channel      *string
	adapter      *string
	messageID    *string
	threadID     *string
	userID       *string
	text         *string
	stdinJSON    *bool
	chatID       *string
	senderName   *string
}

func registerCommonFlags(fs *flag.FlagSet, defaultChannel, defaultAdapter string) commonFlags {
	return commonFlags{
		kernelURL:    fs.String("kernel-url", envOrDefault("GENESIS_KERNEL_URL", "http://127.0.0.1:8765"), "Genesis Kernel HTTP URL"),
		runtimeToken: fs.String("runtime-token", os.Getenv("GENESIS_RUNTIME_TOKEN"), "Genesis runtime bearer token"),
		statePath:    fs.String("state", envOrDefault("GENESIS_INGRESS_STATE", filepath.Join(".genesis_ingress", "state.json")), "application-local ingress state file"),
		channel:      fs.String("channel", defaultChannel, "channel name"),
		adapter:      fs.String("adapter", defaultAdapter, "adapter name"),
		messageID:    fs.String("message-id", "", "external message id"),
		threadID:     fs.String("thread-id", "", "external thread/chat id"),
		userID:       fs.String("user-id", "", "external user id"),
		text:         fs.String("text", "", "message text"),
		stdinJSON:    fs.Bool("stdin-json", false, "read ChannelMessage JSON from stdin"),
		chatID:       fs.String("chat-id", "", "external chat id to expose as inbound reply reference"),
		senderName:   fs.String("sender-display", "", "external sender display name"),
	}
}

func buildRuntime(flags commonFlags) (messageingress.ChannelMessage, *messageingress.Runtime, error) {
	msg := messageingress.ChannelMessage{
		Channel:   *flags.channel,
		Adapter:   *flags.adapter,
		MessageID: *flags.messageID,
		ThreadID:  *flags.threadID,
		UserID:    *flags.userID,
		Text:      *flags.text,
	}
	if *flags.stdinJSON {
		if err := json.NewDecoder(os.Stdin).Decode(&msg); err != nil {
			return messageingress.ChannelMessage{}, nil, err
		}
	}
	if *flags.chatID != "" || *flags.senderName != "" {
		if msg.Metadata == nil {
			msg.Metadata = make(map[string]string)
		}
		if *flags.chatID != "" {
			msg.Metadata["chat_id"] = *flags.chatID
		}
		if *flags.senderName != "" {
			msg.Metadata["sender_display"] = *flags.senderName
		}
	}
	store, err := messageingress.NewFileInboundStore(*flags.statePath)
	if err != nil {
		return messageingress.ChannelMessage{}, nil, err
	}
	runtime := &messageingress.Runtime{
		Store: store,
		Client: messageingress.HTTPKernelClient{
			BaseURL:      *flags.kernelURL,
			RuntimeToken: *flags.runtimeToken,
		},
		Mapper: messageingress.DefaultSessionMapper{},
	}
	return msg, runtime, nil
}

func envOrDefault(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
