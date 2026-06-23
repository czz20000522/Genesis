package connectorruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type ConsoleAdapter struct {
	Writer io.Writer
}

func (a ConsoleAdapter) Execute(_ context.Context, action ConnectorAction) (ConnectorActionResult, error) {
	writer := a.Writer
	if writer == nil {
		writer = os.Stdout
	}
	body := strings.TrimSpace(action.Payload["body"])
	if _, err := fmt.Fprintln(writer, body); err != nil {
		return ConnectorActionResult{Status: DeliveryStatusFailed, Reason: err.Error()}, err
	}
	return ConnectorActionResult{
		ExternalActionRef: action.OutboxID,
		Status:            DeliveryStatusSent,
	}, nil
}

type CommandRunner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

type OSCommandRunner struct{}

func (OSCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

type FeishuAdapter struct {
	CLIPath string
	Profile string
	Runner  CommandRunner
}

func (a FeishuAdapter) Execute(ctx context.Context, action ConnectorAction) (ConnectorActionResult, error) {
	if action.ActionKind != "send_message" {
		return ConnectorActionResult{Status: DeliveryStatusFailed, Reason: "unsupported_action_kind"}, fmt.Errorf("unsupported feishu action kind %q", action.ActionKind)
	}
	if len(action.TargetRef.Metadata) != 0 || hasUnexpectedPayloadKeys(action.Payload, "body") {
		return ConnectorActionResult{Status: DeliveryStatusFailed, Reason: "invalid_action_payload"}, fmt.Errorf("feishu action contains unsupported metadata or payload fields")
	}
	target := strings.TrimSpace(action.TargetRef.ExternalID)
	body := strings.TrimSpace(action.Payload["body"])
	if target == "" || body == "" {
		return ConnectorActionResult{Status: DeliveryStatusFailed, Reason: "invalid_action_payload"}, fmt.Errorf("feishu send requires target and body")
	}
	cli := strings.TrimSpace(a.CLIPath)
	if cli == "" {
		cli = "lark-cli"
	}
	args := []string{"im", "send", "--chat", target, "--text", body}
	if strings.TrimSpace(a.Profile) != "" {
		args = append(args, "--profile", strings.TrimSpace(a.Profile))
	}
	runner := a.Runner
	if runner == nil {
		runner = OSCommandRunner{}
	}
	output, err := runner.Run(ctx, cli, args...)
	if err != nil {
		return ConnectorActionResult{Status: DeliveryStatusFailed, Reason: "external_command_failed"}, err
	}
	return ConnectorActionResult{
		ExternalActionRef: parseFeishuMessageID(output),
		Status:            DeliveryStatusSent,
	}, nil
}

func hasUnexpectedPayloadKeys(payload map[string]string, allowed ...string) bool {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		allowedSet[key] = struct{}{}
	}
	for key := range payload {
		if _, ok := allowedSet[key]; !ok {
			return true
		}
	}
	return false
}

func parseFeishuMessageID(output []byte) string {
	if len(output) == 0 {
		return ""
	}
	if len(output) > 4096 {
		output = output[:4096]
	}
	var payload map[string]any
	if err := json.Unmarshal(output, &payload); err != nil {
		return ""
	}
	if id, ok := stringField(payload, "message_id"); ok {
		return id
	}
	if data, ok := payload["data"].(map[string]any); ok {
		if id, ok := stringField(data, "message_id"); ok {
			return id
		}
	}
	return ""
}

func stringField(payload map[string]any, key string) (string, bool) {
	value, ok := payload[key].(string)
	if !ok {
		return "", false
	}
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 256 {
		return "", false
	}
	return value, true
}
