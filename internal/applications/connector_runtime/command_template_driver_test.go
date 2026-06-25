package connectorruntime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestCommandTemplateDriverRendersConfiguredArgvWithoutCredentialPayload(t *testing.T) {
	runner := &recordingRunner{}
	driver := testFeishuCommandTemplateDriver("codex", runner)
	action := testConnectorSendAction()

	result, err := driver.Execute(context.Background(), action)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Status != DeliveryStatusSent {
		t.Fatalf("result = %+v", result)
	}
	wantArgs := []string{"--profile", "codex", "im", "+messages-send", "--as", "bot", "--chat-id", "oc_123", "--text", "hello", "--idempotency-key", "idem_1"}
	if strings.Join(runner.args, "\x00") != strings.Join(wantArgs, "\x00") {
		t.Fatalf("args = %#v, want %#v", runner.args, wantArgs)
	}
	if runner.name != "lark-cli" {
		t.Fatalf("executable = %q, want lark-cli", runner.name)
	}
	for key, value := range action.Payload {
		if strings.Contains(strings.ToLower(key), "credential") || strings.Contains(strings.ToLower(value), "secret") {
			t.Fatalf("action payload exposes credential-shaped data: %+v", action.Payload)
		}
	}
}

func TestCommandTemplateDriverRequiresExplicitProfile(t *testing.T) {
	driver := testFeishuCommandTemplateDriver("", &recordingRunner{})

	result, err := driver.Execute(context.Background(), testConnectorSendAction())
	if err == nil {
		t.Fatal("Execute should reject missing explicit profile")
	}
	if result.Reason != "missing_explicit_profile" {
		t.Fatalf("reason = %q, want missing_explicit_profile", result.Reason)
	}
}

func TestCommandTemplateDriverRequiresTemplateToBindExplicitProfile(t *testing.T) {
	driver := testFeishuCommandTemplateDriver("codex", &recordingRunner{})
	driver.Actions["send_message"] = CommandTemplateAction{
		Argv: []string{
			"im", "+messages-send",
			"--chat-id", "${target.external_id}",
			"--text", "${payload.body}",
		},
	}

	result, err := driver.Execute(context.Background(), testConnectorSendAction())
	if err == nil {
		t.Fatal("Execute should reject templates without explicit profile binding")
	}
	if result.Reason != "invalid_command_template" {
		t.Fatalf("reason = %q, want invalid_command_template", result.Reason)
	}
}

func TestCommandTemplateDriverRejectsUnknownTemplateVariable(t *testing.T) {
	driver := testFeishuCommandTemplateDriver("codex", &recordingRunner{})
	driver.Actions["send_message"] = CommandTemplateAction{
		Argv: []string{"--text", "${external.foo}"},
	}

	result, err := driver.Execute(context.Background(), testConnectorSendAction())
	if err == nil {
		t.Fatal("Execute should reject unknown template variable")
	}
	if result.Reason != "invalid_command_template" {
		t.Fatalf("reason = %q, want invalid_command_template", result.Reason)
	}
}

func TestCommandTemplateDriverRejectsMissingPayloadAsInvalidActionPayload(t *testing.T) {
	driver := testFeishuCommandTemplateDriver("codex", &recordingRunner{})
	action := testConnectorSendAction()
	delete(action.Payload, "body")

	result, err := driver.Execute(context.Background(), action)
	if err == nil {
		t.Fatal("Execute should reject missing action payload")
	}
	if result.Reason != "invalid_action_payload" {
		t.Fatalf("reason = %q, want invalid_action_payload", result.Reason)
	}
}

func TestCommandTemplateDriverRejectsShellStringTemplate(t *testing.T) {
	driver := testFeishuCommandTemplateDriver("codex", &recordingRunner{})
	driver.Actions["send_message"] = CommandTemplateAction{
		Argv: []string{`im +messages-send --chat-id ${target.external_id} --text ${payload.body}`},
	}

	result, err := driver.Execute(context.Background(), testConnectorSendAction())
	if err == nil {
		t.Fatal("Execute should reject shell string template")
	}
	if result.Reason != "invalid_command_template" {
		t.Fatalf("reason = %q, want invalid_command_template", result.Reason)
	}
}

func TestCommandTemplateDriverRejectsShellExecutable(t *testing.T) {
	driver := testFeishuCommandTemplateDriver("codex", &recordingRunner{})
	driver.Executable = "cmd.exe"

	result, err := driver.Execute(context.Background(), testConnectorSendAction())
	if err == nil {
		t.Fatal("Execute should reject shell executable")
	}
	if result.Reason != "invalid_command_template" {
		t.Fatalf("reason = %q, want invalid_command_template", result.Reason)
	}
}

func TestCommandTemplateDriverRejectsExecutableShellString(t *testing.T) {
	driver := testFeishuCommandTemplateDriver("codex", &recordingRunner{})
	driver.Executable = "lark-cli --profile codex"

	result, err := driver.Execute(context.Background(), testConnectorSendAction())
	if err == nil {
		t.Fatal("Execute should reject executable shell string")
	}
	if result.Reason != "invalid_command_template" {
		t.Fatalf("reason = %q, want invalid_command_template", result.Reason)
	}
}

func TestCommandTemplateDriverRejectsScriptExecutablePath(t *testing.T) {
	for _, executable := range []string{"lark-cli.cmd", "lark-cli.bat", "lark-cli.ps1", "lark-cli.sh"} {
		t.Run(executable, func(t *testing.T) {
			driver := testFeishuCommandTemplateDriver("codex", &recordingRunner{})
			driver.Executable = executable

			result, err := driver.Execute(context.Background(), testConnectorSendAction())
			if err == nil {
				t.Fatal("Execute should reject script executable path")
			}
			if result.Reason != "invalid_command_template" {
				t.Fatalf("reason = %q, want invalid_command_template", result.Reason)
			}
		})
	}
}

func TestCommandTemplateDriverRejectsCredentialShapedTemplateVariable(t *testing.T) {
	driver := testFeishuCommandTemplateDriver("codex", &recordingRunner{})
	driver.Actions["send_message"] = CommandTemplateAction{
		Argv: []string{"--token", "${credential.token}"},
	}

	result, err := driver.Execute(context.Background(), testConnectorSendAction())
	if err == nil {
		t.Fatal("Execute should reject credential-shaped template variable")
	}
	if result.Reason != "invalid_command_template" {
		t.Fatalf("reason = %q, want invalid_command_template", result.Reason)
	}
}

func TestCommandTemplateDriverParsesExternalActionRefFromConfiguredJSONPath(t *testing.T) {
	runner := &recordingRunner{
		output: []byte(`{"data":{"message_id":"om_456"},"debug":"Authorization: Bearer sk-secret"}`),
	}
	driver := testFeishuCommandTemplateDriver("genesis", runner)

	result, err := driver.Execute(context.Background(), testConnectorSendAction())
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.ExternalActionRef != "om_456" {
		t.Fatalf("external action ref = %q, want parsed message id", result.ExternalActionRef)
	}
	if strings.Contains(result.ExternalActionRef, "Authorization") || strings.Contains(result.Reason, "Authorization") {
		t.Fatalf("result leaked raw CLI output: %+v", result)
	}
}

func TestCommandTemplateDriverDropsSecretShapedExternalActionRef(t *testing.T) {
	runner := &recordingRunner{
		output: []byte(`{"data":{"message_id":"Authorization: Bearer sk-secret"}}`),
	}
	driver := testFeishuCommandTemplateDriver("genesis", runner)

	result, err := driver.Execute(context.Background(), testConnectorSendAction())
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.ExternalActionRef != "" {
		t.Fatalf("external action ref should be dropped, got %q", result.ExternalActionRef)
	}
}

func TestCommandTemplateDriverDropsMalformedExternalActionRef(t *testing.T) {
	runner := &recordingRunner{
		output: []byte("{\"data\":{\"message_id\":\"om_456\\nraw-debug\"}}"),
	}
	driver := testFeishuCommandTemplateDriver("genesis", runner)

	result, err := driver.Execute(context.Background(), testConnectorSendAction())
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.ExternalActionRef != "" {
		t.Fatalf("external action ref should be dropped, got %q", result.ExternalActionRef)
	}
}

func TestCommandTemplateDriverMapsFailureWithoutRawOutput(t *testing.T) {
	runner := &recordingRunner{
		output: []byte("Authorization: Bearer sk-secret\nrate limit exceeded"),
		err:    errors.New("exit status 1"),
	}
	driver := testFeishuCommandTemplateDriver("genesis", runner)

	result, err := driver.Execute(context.Background(), testConnectorSendAction())
	if err == nil {
		t.Fatal("Execute should return command error")
	}
	if result.Reason != "external_command_failed" {
		t.Fatalf("reason = %q, want external_command_failed", result.Reason)
	}
	if strings.Contains(result.Reason, "Authorization") || strings.Contains(result.ExternalActionRef, "Authorization") {
		t.Fatalf("result leaked raw CLI output: %+v", result)
	}
}

func TestCommandTemplateDriverRejectsOversizedRunnerOutputBeforeParsing(t *testing.T) {
	runner := &recordingRunner{
		output: []byte(`{"message_id":"om_oversized"}` + strings.Repeat("x", maxConnectorCommandOutputBytes+1)),
	}
	driver := testFeishuCommandTemplateDriver("genesis", runner)

	result, err := driver.Execute(context.Background(), testConnectorSendAction())
	if err == nil {
		t.Fatal("Execute should reject oversized runner output")
	}
	if result.Status != DeliveryStatusFailed || result.Reason != "external_command_output_exceeded" {
		t.Fatalf("result = %+v, want failed external_command_output_exceeded", result)
	}
}

func TestOSCommandRunnerBoundsOversizedStderrOnFailure(t *testing.T) {
	env := append(connectorCommandEnvironment(os.Environ()),
		"GENESIS_COMMAND_TEMPLATE_DRIVER_HELPER=oversized-stderr-failure",
	)
	runner := OSCommandRunner{Env: env}

	output, err := runner.Run(context.Background(), os.Args[0], "-test.run=TestCommandTemplateDriverOutputHelper")
	if !errors.Is(err, errConnectorCommandOutputExceeded) {
		t.Fatalf("Run error = %v, want errConnectorCommandOutputExceeded", err)
	}
	if len(output) > maxConnectorCommandOutputBytes {
		t.Fatalf("captured output = %d bytes, want <= %d", len(output), maxConnectorCommandOutputBytes)
	}
}

func TestCommandTemplateDriverMapsUnsafeExecutableToInvalidTemplate(t *testing.T) {
	runner := &recordingRunner{
		err: fmt.Errorf("%w: lark-cli.cmd", errUnsafeCommandExecutable),
	}
	driver := testFeishuCommandTemplateDriver("genesis", runner)

	result, err := driver.Execute(context.Background(), testConnectorSendAction())
	if err == nil {
		t.Fatal("Execute should return unsafe executable error")
	}
	if result.Reason != "invalid_command_template" {
		t.Fatalf("reason = %q, want invalid_command_template", result.Reason)
	}
}

func TestCommandTemplateDriverRejectsUnexpectedActionPayloadAndTargetMetadata(t *testing.T) {
	driver := testFeishuCommandTemplateDriver("genesis", &recordingRunner{})
	action := testConnectorSendAction()
	action.TargetRef.Metadata = map[string]string{"api_key": "sk-secret"}
	action.Payload["api_key"] = "sk-secret"

	result, err := driver.Execute(context.Background(), action)
	if err == nil {
		t.Fatal("Execute should reject unexpected payload and target metadata")
	}
	if result.Reason != "invalid_action_payload" {
		t.Fatalf("reason = %q, want invalid_action_payload", result.Reason)
	}
}

func TestConnectorCommandEnvironmentDropsSecretEnvironment(t *testing.T) {
	env := connectorCommandEnvironment([]string{
		"PATH=C:\\tools",
		"Path=C:\\shadow",
		"GENESIS_PROVIDER_API_KEY=sk-secret",
		"OPENAI_API_KEY=sk-secret",
		"AUTHORIZATION=Bearer secret",
		"USERPROFILE=C:\\Users\\Tomczz",
	})
	joined := strings.Join(env, "\n")
	for _, forbidden := range []string{"GENESIS_PROVIDER_API_KEY", "OPENAI_API_KEY", "AUTHORIZATION", "sk-secret", "Bearer secret"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("connector command environment leaked %q: %v", forbidden, env)
		}
	}
	if !strings.Contains(joined, "PATH=C:\\tools") || !strings.Contains(joined, "USERPROFILE=C:\\Users\\Tomczz") {
		t.Fatalf("connector command environment dropped required non-secret entries: %v", env)
	}
	if strings.Contains(joined, "Path=C:\\shadow") {
		t.Fatalf("connector command environment should keep only the first case-insensitive key: %v", env)
	}
}

func TestUnsafeResolvedCommandExecutableRejectsScriptWrappers(t *testing.T) {
	executables := []string{"/usr/local/bin/lark-cli.sh"}
	if runtime.GOOS == "windows" {
		executables = append(executables,
			"C:\\Users\\Tomczz\\AppData\\Roaming\\npm\\lark-cli",
			"C:\\Users\\Tomczz\\AppData\\Roaming\\npm\\lark-cli.cmd",
			"C:\\Users\\Tomczz\\AppData\\Roaming\\npm\\lark-cli.ps1",
		)
	}
	for _, executable := range executables {
		t.Run(executable, func(t *testing.T) {
			if !unsafeResolvedCommandExecutable(executable) {
				t.Fatalf("expected %q to be rejected as a script wrapper", executable)
			}
		})
	}
}

func TestCommandTemplateDriverOutputHelper(t *testing.T) {
	switch os.Getenv("GENESIS_COMMAND_TEMPLATE_DRIVER_HELPER") {
	case "":
		return
	case "oversized-stderr-failure":
		_, _ = os.Stderr.WriteString(strings.Repeat("Authorization: Bearer sk-secret\n", maxConnectorCommandOutputBytes/8))
		os.Exit(42)
	default:
		t.Fatalf("unknown command template driver helper mode %q", os.Getenv("GENESIS_COMMAND_TEMPLATE_DRIVER_HELPER"))
	}
}

func testFeishuCommandTemplateDriver(profile string, runner CommandRunner) CommandTemplateDriver {
	return NewFeishuSendMessageCommandTemplateDriver(profile, "lark-cli", runner)
}

func testConnectorSendAction() ConnectorAction {
	return ConnectorAction{
		OutboxID:       "outbox_1",
		Connector:      "feishu",
		ActionKind:     "send_message",
		TargetRef:      ExternalThreadRef{Connector: "feishu", Kind: "chat", ExternalID: "oc_123"},
		Payload:        map[string]string{"body": "hello"},
		IdempotencyKey: "idem_1",
		Attempt:        1,
	}
}

type recordingRunner struct {
	name   string
	args   []string
	output []byte
	err    error
}

func (r *recordingRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.name = name
	r.args = append([]string(nil), args...)
	if r.output == nil {
		r.output = []byte(`{"message_id":"om_123"}`)
	}
	return r.output, r.err
}
