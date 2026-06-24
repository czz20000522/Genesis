package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	connectorruntime "genesis/internal/applications/connector_runtime"
)

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
