package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	connectorruntime "genesis/internal/applications/connector_runtime"
)

func TestFeishuConnectorAdapterManifestReportsStableContract(t *testing.T) {
	var stdout bytes.Buffer
	runner := &recordingRunner{}

	if err := run(context.Background(), []string{"--manifest"}, strings.NewReader("not an action"), &stdout, runner); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	var manifest struct {
		Connector        string   `json:"connector"`
		SupportedActions []string `json:"supported_actions"`
		RequiredProfile  bool     `json:"required_profile"`
		EnvAllowlist     []string `json:"env_allowlist"`
		ProbeModes       []string `json:"probe_modes"`
		ResultStatuses   []string `json:"result_statuses"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &manifest); err != nil {
		t.Fatalf("decode manifest: %v\n%s", err, stdout.String())
	}
	if manifest.Connector != "feishu" || !manifest.RequiredProfile {
		t.Fatalf("manifest = %+v, want Feishu required-profile contract", manifest)
	}
	if !stringSliceContains(manifest.SupportedActions, "send_message") {
		t.Fatalf("supported actions = %+v, want send_message", manifest.SupportedActions)
	}
	if !stringSliceContains(manifest.EnvAllowlist, "PATH") || stringSliceContains(manifest.EnvAllowlist, "AUTHORIZATION") {
		t.Fatalf("env allowlist = %+v, want bounded non-secret execution basics", manifest.EnvAllowlist)
	}
	if !stringSliceContains(manifest.ProbeModes, "readiness") || !stringSliceContains(manifest.ResultStatuses, connectorruntime.DeliveryStatusSent) {
		t.Fatalf("manifest = %+v, want readiness probe and result status schema", manifest)
	}
	if runner.calls != 0 {
		t.Fatalf("runner calls = %d, want 0 for manifest", runner.calls)
	}
	if strings.Contains(stdout.String(), "messages-send") || strings.Contains(stdout.String(), "+messages-send") {
		t.Fatalf("manifest leaked lark-cli message argv: %s", stdout.String())
	}
}

func TestFeishuConnectorAdapterProbeDoesNotSendMessage(t *testing.T) {
	var stdout bytes.Buffer
	runner := &recordingRunner{}

	if err := run(context.Background(), []string{"--probe", "--profile", "genesis", "--lark-cli", os.Args[0]}, strings.NewReader("not an action"), &stdout, runner); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	var report struct {
		Connector        string   `json:"connector"`
		Ready            bool     `json:"ready"`
		Status           string   `json:"status"`
		Profile          string   `json:"profile"`
		Executable       string   `json:"executable"`
		SupportedActions []string `json:"supported_actions"`
		SendsMessage     bool     `json:"sends_message"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode probe: %v\n%s", err, stdout.String())
	}
	if !report.Ready || report.Status != connectorruntime.ProbeStatusOK || report.Profile != "genesis" || report.Executable == "" {
		t.Fatalf("probe = %+v, want ready adapter report", report)
	}
	if !stringSliceContains(report.SupportedActions, "send_message") || report.SendsMessage {
		t.Fatalf("probe = %+v, want supported action without send side effect", report)
	}
	if runner.calls != 0 {
		t.Fatalf("runner calls = %d, want 0 for probe", runner.calls)
	}
}

func TestFeishuConnectorAdapterProbeClassifiesProfileReadiness(t *testing.T) {
	var stdout bytes.Buffer
	runner := &recordingRunner{}

	if err := run(context.Background(), []string{"--probe", "--profile", "genesis", "--profile-readiness", connectorruntime.SourceReadinessReasonPermissionDenied, "--lark-cli", os.Args[0]}, strings.NewReader("not an action"), &stdout, runner); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	var report struct {
		Ready  bool   `json:"ready"`
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode probe: %v\n%s", err, stdout.String())
	}
	if report.Ready || report.Status != connectorruntime.ProbeStatusFailed || report.Reason != connectorruntime.SourceReadinessReasonPermissionDenied {
		t.Fatalf("probe = %+v, want permission_denied readiness failure", report)
	}
	if runner.calls != 0 {
		t.Fatalf("runner calls = %d, want 0 for blocked probe", runner.calls)
	}
}

func TestFeishuConnectorAdapterProfileReadinessBlocksActionBeforeCLI(t *testing.T) {
	var stdin bytes.Buffer
	if err := json.NewEncoder(&stdin).Encode(testSendAction()); err != nil {
		t.Fatalf("encode action: %v", err)
	}
	var stdout bytes.Buffer
	runner := &recordingRunner{}

	if err := run(context.Background(), []string{"--profile", "genesis", "--profile-readiness", connectorruntime.SourceReadinessReasonRefreshRequired}, &stdin, &stdout, runner); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	var result connectorruntime.ConnectorActionResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v\n%s", err, stdout.String())
	}
	if result.Status != connectorruntime.DeliveryStatusFailed || result.Reason != connectorruntime.SourceReadinessReasonRefreshRequired {
		t.Fatalf("result = %+v, want refresh_required failure", result)
	}
	if runner.calls != 0 {
		t.Fatalf("runner calls = %d, want 0 when profile readiness blocks", runner.calls)
	}
}

func TestFeishuConnectorAdapterSendsTypedActionThroughLarkCLI(t *testing.T) {
	runner := &recordingRunner{output: []byte(`{"data":{"message_id":"om_456"}}`)}
	var stdin bytes.Buffer
	if err := json.NewEncoder(&stdin).Encode(testSendAction()); err != nil {
		t.Fatalf("encode action: %v", err)
	}
	var stdout bytes.Buffer

	if err := run(context.Background(), []string{"--profile", "genesis", "--lark-cli", "lark-cli.exe"}, &stdin, &stdout, runner); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	var result connectorruntime.ConnectorActionResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v\n%s", err, stdout.String())
	}
	if result.Status != connectorruntime.DeliveryStatusSent || result.ExternalActionRef != "om_456" {
		t.Fatalf("result = %+v, want sent om_456", result)
	}
	wantArgs := []string{"--profile", "genesis", "im", "+messages-send", "--as", "bot", "--chat-id", "oc_123", "--text", "hello", "--idempotency-key", "idem_1"}
	if runner.name != "lark-cli.exe" || strings.Join(runner.args, "\x00") != strings.Join(wantArgs, "\x00") {
		t.Fatalf("command = %q %#v, want lark-cli args %#v", runner.name, runner.args, wantArgs)
	}
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestFeishuConnectorAdapterRequiresExplicitProfile(t *testing.T) {
	var stdin bytes.Buffer
	if err := json.NewEncoder(&stdin).Encode(testSendAction()); err != nil {
		t.Fatalf("encode action: %v", err)
	}
	var stdout bytes.Buffer

	if err := run(context.Background(), nil, &stdin, &stdout, &recordingRunner{}); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	var result connectorruntime.ConnectorActionResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v\n%s", err, stdout.String())
	}
	if result.Status != connectorruntime.DeliveryStatusFailed || result.Reason != "missing_profile" {
		t.Fatalf("result = %+v, want failed missing_profile", result)
	}
}

func TestFeishuConnectorAdapterRejectsMalformedActionWithoutRunningCLI(t *testing.T) {
	action := testSendAction()
	action.TargetRef.Metadata = map[string]string{"credential_ref": "cred_1"}
	var stdin bytes.Buffer
	if err := json.NewEncoder(&stdin).Encode(action); err != nil {
		t.Fatalf("encode action: %v", err)
	}
	runner := &recordingRunner{}
	var stdout bytes.Buffer

	if err := run(context.Background(), []string{"--profile", "genesis"}, &stdin, &stdout, runner); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	var result connectorruntime.ConnectorActionResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v\n%s", err, stdout.String())
	}
	if result.Status != connectorruntime.DeliveryStatusFailed || result.Reason != "invalid_connector_action" {
		t.Fatalf("result = %+v, want failed invalid_connector_action", result)
	}
	if runner.calls != 0 {
		t.Fatalf("runner calls = %d, want 0", runner.calls)
	}
}

func TestFeishuConnectorAdapterRejectsUnsupportedActionWithoutRunningCLI(t *testing.T) {
	action := testSendAction()
	action.ActionKind = "send_card"
	var stdin bytes.Buffer
	if err := json.NewEncoder(&stdin).Encode(action); err != nil {
		t.Fatalf("encode action: %v", err)
	}
	runner := &recordingRunner{}
	var stdout bytes.Buffer

	if err := run(context.Background(), []string{"--profile", "genesis"}, &stdin, &stdout, runner); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	var result connectorruntime.ConnectorActionResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v\n%s", err, stdout.String())
	}
	if result.Status != connectorruntime.DeliveryStatusFailed || result.Reason != "unsupported_action_kind" {
		t.Fatalf("result = %+v, want failed unsupported_action_kind", result)
	}
	if runner.calls != 0 {
		t.Fatalf("runner calls = %d, want 0", runner.calls)
	}
}

func TestFeishuConnectorAdapterDoesNotExposeRawCLIOutputOnFailure(t *testing.T) {
	runner := &recordingRunner{
		output: []byte("Authorization: Bearer sk-secret\nrate limited\n"),
		err:    errors.New("exit status 1"),
	}
	var stdin bytes.Buffer
	if err := json.NewEncoder(&stdin).Encode(testSendAction()); err != nil {
		t.Fatalf("encode action: %v", err)
	}
	var stdout bytes.Buffer

	if err := run(context.Background(), []string{"--profile", "genesis"}, &stdin, &stdout, runner); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	var result connectorruntime.ConnectorActionResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v\n%s", err, stdout.String())
	}
	if result.Status != connectorruntime.DeliveryStatusFailed || result.Reason != "external_command_failed" {
		t.Fatalf("result = %+v, want failed external_command_failed", result)
	}
	for _, leaked := range []string{"Authorization", "Bearer", "sk-secret", "rate limited"} {
		if strings.Contains(result.Reason, leaked) || strings.Contains(result.ExternalActionRef, leaked) {
			t.Fatalf("result leaked raw CLI output: %+v", result)
		}
	}
}

func TestFeishuConnectorAdapterRejectsOversizedCLIOutput(t *testing.T) {
	runner := &recordingRunner{
		output: []byte(`{"data":{"message_id":"om_oversized"}}` + strings.Repeat("x", 128*1024)),
	}
	var stdin bytes.Buffer
	if err := json.NewEncoder(&stdin).Encode(testSendAction()); err != nil {
		t.Fatalf("encode action: %v", err)
	}
	var stdout bytes.Buffer

	if err := run(context.Background(), []string{"--profile", "genesis"}, &stdin, &stdout, runner); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	var result connectorruntime.ConnectorActionResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v\n%s", err, stdout.String())
	}
	if result.Status != connectorruntime.DeliveryStatusFailed || result.Reason != "external_command_output_exceeded" {
		t.Fatalf("result = %+v, want failed external_command_output_exceeded", result)
	}
}

func testSendAction() connectorruntime.ConnectorAction {
	return connectorruntime.ConnectorAction{
		OutboxID:       "outbox_1",
		Connector:      "feishu",
		ActionKind:     "send_message",
		TargetRef:      connectorruntime.ExternalThreadRef{Connector: "feishu", Kind: "chat", ExternalID: "oc_123"},
		Payload:        map[string]string{"body": "hello"},
		IdempotencyKey: "idem_1",
		Attempt:        1,
	}
}

type recordingRunner struct {
	calls  int
	name   string
	args   []string
	output []byte
	err    error
}

func (r *recordingRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls++
	r.name = name
	r.args = append([]string(nil), args...)
	return r.output, r.err
}
