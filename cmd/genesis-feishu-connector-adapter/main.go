package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	connectorruntime "genesis/internal/applications/connector_runtime"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, connectorruntime.OSCommandRunner{}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer, runner connectorruntime.CommandRunner) error {
	result := execute(ctx, args, stdin, runner)
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		return err
	}
	return nil
}

func execute(ctx context.Context, args []string, stdin io.Reader, runner connectorruntime.CommandRunner) connectorruntime.ConnectorActionResult {
	fs := flag.NewFlagSet("genesis-feishu-connector-adapter", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	profile := fs.String("profile", "", "explicit lark-cli profile for Feishu outbound delivery")
	larkCLI := fs.String("lark-cli", os.Getenv("GENESIS_FEISHU_CLI_EXECUTABLE"), "direct lark-cli executable")
	identity := fs.String("as", "bot", "Feishu send identity: bot or user")
	if err := fs.Parse(args); err != nil {
		return failed("invalid_adapter_config")
	}
	if strings.TrimSpace(*profile) == "" {
		return failed("missing_profile")
	}
	if *identity != "bot" && *identity != "user" {
		return failed("invalid_adapter_config")
	}
	var action connectorruntime.ConnectorAction
	if err := json.NewDecoder(stdin).Decode(&action); err != nil {
		return failed("invalid_connector_action")
	}
	if err := validateFeishuSendMessageAction(action); err != nil {
		return failed(err.Error())
	}
	executable := connectorruntime.SelectFeishuCLIExecutable(*larkCLI, connectorruntime.InstalledOfficialLarkCLIExecutable())
	output, err := runner.Run(ctx, executable,
		"--profile", strings.TrimSpace(*profile),
		"im", "+messages-send",
		"--as", *identity,
		"--chat-id", strings.TrimSpace(action.TargetRef.ExternalID),
		"--text", strings.TrimSpace(action.Payload["body"]),
		"--idempotency-key", strings.TrimSpace(action.IdempotencyKey),
	)
	if err != nil {
		return failed("external_command_failed")
	}
	return connectorruntime.ConnectorActionResult{
		Status:            connectorruntime.DeliveryStatusSent,
		ExternalActionRef: firstSafeExternalActionRef(output, "data.message_id", "message_id"),
	}
}

func validateFeishuSendMessageAction(action connectorruntime.ConnectorAction) error {
	switch {
	case strings.TrimSpace(action.Connector) != "feishu":
		return errors.New("invalid_connector_action")
	case strings.TrimSpace(action.ActionKind) != "send_message":
		return errors.New("unsupported_action_kind")
	case strings.TrimSpace(action.TargetRef.Connector) != "feishu":
		return errors.New("invalid_connector_action")
	case strings.TrimSpace(action.TargetRef.Kind) != "chat":
		return errors.New("invalid_connector_action")
	case strings.TrimSpace(action.TargetRef.ExternalID) == "":
		return errors.New("invalid_connector_action")
	case strings.TrimSpace(action.Payload["body"]) == "":
		return errors.New("invalid_connector_action")
	case strings.TrimSpace(action.IdempotencyKey) == "":
		return errors.New("invalid_connector_action")
	case len(action.TargetRef.Metadata) != 0:
		return errors.New("invalid_connector_action")
	default:
		return nil
	}
}

func failed(reason string) connectorruntime.ConnectorActionResult {
	if strings.TrimSpace(reason) == "" {
		reason = "adapter_failed"
	}
	return connectorruntime.ConnectorActionResult{
		Status: connectorruntime.DeliveryStatusFailed,
		Reason: reason,
	}
}

func firstSafeExternalActionRef(output []byte, paths ...string) string {
	if len(output) == 0 {
		return ""
	}
	if len(output) > 4096 {
		output = output[:4096]
	}
	var payload any
	if err := json.Unmarshal(output, &payload); err != nil {
		return ""
	}
	for _, path := range paths {
		value, ok := valueAtPath(payload, path)
		if !ok {
			continue
		}
		text, ok := value.(string)
		if !ok {
			continue
		}
		text = strings.TrimSpace(text)
		if safeExternalActionRef(text) {
			return text
		}
	}
	return ""
}

func valueAtPath(payload any, path string) (any, bool) {
	current := payload
	for _, part := range strings.Split(path, ".") {
		if part == "" {
			return nil, false
		}
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func safeExternalActionRef(ref string) bool {
	if ref == "" || len(ref) > 256 || credentialShaped(ref) {
		return false
	}
	for _, r := range ref {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '-' || r == '.' || r == ':':
		default:
			return false
		}
	}
	return true
}

func credentialShaped(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{"authorization", "bearer ", "credential", "secret", "api_key", "apikey", "password", "token", "sk-", "xoxb-", "xoxp-"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
