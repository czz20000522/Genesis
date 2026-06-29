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
	feishucli "genesis/internal/applications/feishu_cli"
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

func execute(ctx context.Context, args []string, stdin io.Reader, runner connectorruntime.CommandRunner) any {
	fs := flag.NewFlagSet("genesis-feishu-connector-adapter", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	profile := fs.String("profile", "", "explicit lark-cli profile for Feishu outbound delivery")
	profileReadiness := fs.String("profile-readiness", "ok", "connector-local Feishu profile readiness posture: ok, missing_profile, profile_expired, permission_denied, or refresh_required")
	larkCLI := fs.String("lark-cli", os.Getenv("GENESIS_FEISHU_CLI_EXECUTABLE"), "direct lark-cli executable")
	identity := fs.String("as", "bot", "Feishu send identity: bot or user")
	manifest := fs.Bool("manifest", false, "emit the Feishu connector adapter manifest without sending")
	probe := fs.Bool("probe", false, "emit a Feishu connector adapter readiness report without sending")
	if err := fs.Parse(args); err != nil {
		return failed("invalid_adapter_config")
	}
	if *manifest {
		return feishuConnectorAdapterManifest()
	}
	if *probe {
		return feishuConnectorAdapterProbe(*profile, *profileReadiness, *larkCLI)
	}
	if *identity != "bot" && *identity != "user" {
		return failed("invalid_adapter_config")
	}
	if reason := feishuProfileReadinessReason(*profile, *profileReadiness); reason != "" {
		return failed(reason)
	}
	var action connectorruntime.ConnectorAction
	if err := json.NewDecoder(stdin).Decode(&action); err != nil {
		return failed("invalid_connector_action")
	}
	if err := validateFeishuSendMessageAction(action); err != nil {
		return failed(err.Error())
	}
	executable := feishucli.SelectExecutable(*larkCLI, feishucli.InstalledOfficialExecutable())
	output, err := runner.Run(ctx, executable,
		"--profile", strings.TrimSpace(*profile),
		"im", "+messages-send",
		"--as", *identity,
		"--chat-id", strings.TrimSpace(action.TargetRef.ExternalID),
		"--text", strings.TrimSpace(action.Payload["body"]),
		"--idempotency-key", strings.TrimSpace(action.IdempotencyKey),
	)
	boundedOutput, outputTruncated := connectorruntime.BoundConnectorCommandOutput(output)
	if outputTruncated || connectorruntime.IsConnectorCommandOutputExceeded(err) {
		return failed("external_command_output_exceeded")
	}
	if err != nil {
		return failed("external_command_failed")
	}
	return connectorruntime.ConnectorActionResult{
		Status:            connectorruntime.DeliveryStatusSent,
		ExternalActionRef: firstSafeExternalActionRef(boundedOutput, "data.message_id", "message_id"),
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

type feishuAdapterManifest struct {
	Connector        string   `json:"connector"`
	SupportedActions []string `json:"supported_actions"`
	RequiredProfile  bool     `json:"required_profile"`
	EnvAllowlist     []string `json:"env_allowlist"`
	ProbeModes       []string `json:"probe_modes"`
	ResultStatuses   []string `json:"result_statuses"`
	ResultRefs       []string `json:"external_action_ref_json_paths"`
}

type feishuAdapterProbe struct {
	Connector        string   `json:"connector"`
	Ready            bool     `json:"ready"`
	Status           string   `json:"status"`
	Reason           string   `json:"reason,omitempty"`
	Profile          string   `json:"profile,omitempty"`
	Executable       string   `json:"executable,omitempty"`
	SupportedActions []string `json:"supported_actions"`
	SendsMessage     bool     `json:"sends_message"`
}

func feishuConnectorAdapterManifest() feishuAdapterManifest {
	return feishuAdapterManifest{
		Connector:        "feishu",
		SupportedActions: []string{"send_message"},
		RequiredProfile:  true,
		EnvAllowlist:     connectorruntime.ConnectorCommandEnvironmentAllowlist(),
		ProbeModes:       []string{"manifest", "readiness"},
		ResultStatuses: []string{
			connectorruntime.DeliveryStatusSent,
			connectorruntime.DeliveryStatusFailed,
		},
		ResultRefs: []string{"data.message_id", "message_id"},
	}
}

func feishuConnectorAdapterProbe(profile string, profileReadiness string, larkCLI string) feishuAdapterProbe {
	report := feishuAdapterProbe{
		Connector:        "feishu",
		Status:           connectorruntime.ProbeStatusFailed,
		Profile:          strings.TrimSpace(profile),
		SupportedActions: []string{"send_message"},
		SendsMessage:     false,
	}
	if reason := feishuProfileReadinessReason(profile, profileReadiness); reason != "" {
		report.Reason = reason
		return report
	}
	executable := feishucli.SelectExecutable(larkCLI, feishucli.InstalledOfficialExecutable())
	resolved, err := connectorruntime.ResolveDirectCommandExecutable(executable)
	if err != nil {
		report.Reason = "external_command_missing"
		return report
	}
	report.Ready = true
	report.Status = connectorruntime.ProbeStatusOK
	report.Executable = resolved
	return report
}

func feishuProfileReadinessReason(profile string, readiness string) string {
	if strings.TrimSpace(profile) == "" {
		return connectorruntime.SourceReadinessReasonMissingProfile
	}
	readiness = strings.TrimSpace(readiness)
	if readiness == "" || readiness == "ok" {
		return ""
	}
	if !connectorruntime.ValidSourceReadinessReasonCode(readiness) {
		return "invalid_adapter_config"
	}
	return readiness
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
