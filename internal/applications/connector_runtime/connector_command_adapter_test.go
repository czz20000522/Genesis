package connectorruntime

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestConnectorCommandAdapterSendsTypedActionAndReadsTypedResult(t *testing.T) {
	capturePath := t.TempDir() + "/captured-action.json"
	adapter := testConnectorCommandAdapter("sent", capturePath)

	result, err := adapter.Execute(context.Background(), testConnectorSendAction())
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Status != DeliveryStatusSent || result.ExternalActionRef != "om_789" {
		t.Fatalf("result = %+v, want sent om_789", result)
	}

	var captured ConnectorAction
	content, err := os.ReadFile(capturePath)
	if err != nil {
		t.Fatalf("read captured action: %v", err)
	}
	if err := json.Unmarshal(content, &captured); err != nil {
		t.Fatalf("decode captured action: %v", err)
	}
	if captured.Connector != "feishu" || captured.ActionKind != "send_message" || captured.Payload["body"] != "hello" {
		t.Fatalf("captured action = %+v", captured)
	}
}

func TestConnectorCommandAdapterRejectsMalformedJSON(t *testing.T) {
	adapter := testConnectorCommandAdapter("malformed-json", "")

	result, err := adapter.Execute(context.Background(), testConnectorSendAction())
	if err == nil {
		t.Fatal("Execute should reject malformed adapter JSON")
	}
	if result.Status != DeliveryStatusFailed || result.Reason != "connector_command_invalid_result" {
		t.Fatalf("result = %+v, want failed connector_command_invalid_result", result)
	}
}

func TestConnectorCommandAdapterRejectsUnsupportedStatus(t *testing.T) {
	adapter := testConnectorCommandAdapter("bad-status", "")

	result, err := adapter.Execute(context.Background(), testConnectorSendAction())
	if err == nil {
		t.Fatal("Execute should reject unsupported adapter result status")
	}
	if result.Status != DeliveryStatusFailed || result.Reason != "connector_command_invalid_result" {
		t.Fatalf("result = %+v, want failed connector_command_invalid_result", result)
	}
}

func TestConnectorCommandAdapterRedactsStderrAndDoesNotPersistRawOutput(t *testing.T) {
	adapter := testConnectorCommandAdapter("stderr-redaction", "")

	result, err := adapter.Execute(context.Background(), testConnectorSendAction())
	if err == nil {
		t.Fatal("Execute should return adapter process error")
	}
	if result.Status != DeliveryStatusFailed || result.Reason != "connector_command_failed" {
		t.Fatalf("result = %+v, want failed connector_command_failed", result)
	}
	for _, leaked := range []string{"Authorization", "Bearer", "sk-secret", "rate limited"} {
		if strings.Contains(result.Reason, leaked) || strings.Contains(result.ExternalActionRef, leaked) {
			t.Fatalf("connector command result leaked raw stderr/stdout: %+v", result)
		}
	}
}

func TestConnectorCommandAdapterRejectsSecretShapedExternalActionRef(t *testing.T) {
	adapter := testConnectorCommandAdapter("unsafe-ref", "")

	result, err := adapter.Execute(context.Background(), testConnectorSendAction())
	if err == nil {
		t.Fatal("Execute should reject secret-shaped external action ref")
	}
	if result.Status != DeliveryStatusFailed || result.Reason != "connector_command_invalid_result" {
		t.Fatalf("result = %+v, want failed connector_command_invalid_result", result)
	}
}

func TestConnectorCommandAdapterRejectsMismatchedActionConnector(t *testing.T) {
	adapter := testConnectorCommandAdapter("sent", "")
	action := testConnectorSendAction()
	action.TargetRef.Connector = "wechat"

	result, err := adapter.Execute(context.Background(), action)
	if err == nil {
		t.Fatal("Execute should reject connector mismatch before invoking adapter process")
	}
	if result.Status != DeliveryStatusFailed || result.Reason != "invalid_connector_action" {
		t.Fatalf("result = %+v, want failed invalid_connector_action", result)
	}
}

func TestConnectorCommandAdapterRejectsCredentialShapedEnv(t *testing.T) {
	adapter := testConnectorCommandAdapter("sent", "")
	adapter.Env = append(adapter.Env, "FEISHU_TOKEN=secret")

	result, err := adapter.Execute(context.Background(), testConnectorSendAction())
	if err == nil {
		t.Fatal("Execute should reject credential-shaped env")
	}
	if result.Status != DeliveryStatusFailed || result.Reason != "connector_command_env_rejected" {
		t.Fatalf("result = %+v, want failed connector_command_env_rejected", result)
	}
}

func TestRuntimeExecuteOutboxItemWithConnectorCommandRecordsReceipt(t *testing.T) {
	store := newTestOutboxStore(t)
	runtime := testRuntime(store, map[string]ConnectorAdapter{
		"feishu": testConnectorCommandAdapter("sent", ""),
	})
	item, _, err := runtime.EnqueueCommand(context.Background(), testSendMessageCommand())
	if err != nil {
		t.Fatalf("EnqueueCommand returned error: %v", err)
	}

	receipt, err := runtime.ExecuteOutboxItem(context.Background(), item.OutboxID)
	if err != nil {
		t.Fatalf("ExecuteOutboxItem returned error: %v", err)
	}
	if receipt.Status != DeliveryStatusSent || receipt.ExternalActionRef != "om_789" {
		t.Fatalf("receipt = %+v, want sent om_789", receipt)
	}
	if strings.Contains(receipt.Reason, "stdout") || strings.Contains(receipt.Reason, "stderr") {
		t.Fatalf("receipt should not persist raw command diagnostics: %+v", receipt)
	}
}

func TestRuntimeExecuteOutboxItemWithConnectorCommandFailureRecordsRedactedReceipt(t *testing.T) {
	store := newTestOutboxStore(t)
	runtime := testRuntime(store, map[string]ConnectorAdapter{
		"feishu": testConnectorCommandAdapter("stderr-redaction", ""),
	})
	item, _, err := runtime.EnqueueCommand(context.Background(), testSendMessageCommand())
	if err != nil {
		t.Fatalf("EnqueueCommand returned error: %v", err)
	}

	receipt, err := runtime.ExecuteOutboxItem(context.Background(), item.OutboxID)
	if err == nil {
		t.Fatal("ExecuteOutboxItem should return connector command failure")
	}
	if receipt.Status != DeliveryStatusDeadLettered || receipt.Reason != "connector_command_failed" {
		t.Fatalf("receipt = %+v, want dead_lettered connector_command_failed", receipt)
	}
	for _, leaked := range []string{"Authorization", "Bearer", "sk-secret", "rate limited"} {
		if strings.Contains(receipt.Reason, leaked) || strings.Contains(receipt.ExternalActionRef, leaked) {
			t.Fatalf("receipt leaked raw adapter diagnostics: %+v", receipt)
		}
	}
}

func testConnectorCommandAdapter(mode string, capturePath string) ConnectorCommandAdapter {
	env := append(connectorCommandEnvironment(os.Environ()),
		"GENESIS_CONNECTOR_COMMAND_HELPER="+mode,
	)
	if capturePath != "" {
		env = append(env, "GENESIS_CONNECTOR_COMMAND_CAPTURE="+capturePath)
	}
	return ConnectorCommandAdapter{
		Executable: os.Args[0],
		Args:       []string{"-test.run=TestConnectorCommandAdapterHelper"},
		Env:        env,
		Timeout:    2 * time.Second,
	}
}

func TestConnectorCommandAdapterHelper(t *testing.T) {
	mode := os.Getenv("GENESIS_CONNECTOR_COMMAND_HELPER")
	if mode == "" {
		return
	}
	var action ConnectorAction
	if err := json.NewDecoder(os.Stdin).Decode(&action); err != nil {
		t.Fatalf("decode action: %v", err)
	}
	if capturePath := os.Getenv("GENESIS_CONNECTOR_COMMAND_CAPTURE"); capturePath != "" {
		content, err := json.Marshal(action)
		if err != nil {
			t.Fatalf("marshal captured action: %v", err)
		}
		if err := os.WriteFile(capturePath, content, 0o644); err != nil {
			t.Fatalf("write captured action: %v", err)
		}
	}

	switch mode {
	case "sent":
		writeConnectorCommandHelperResult(t, ConnectorActionResult{
			Status:            DeliveryStatusSent,
			ExternalActionRef: "om_789",
		})
	case "malformed-json":
		_, _ = os.Stdout.WriteString("{not-json")
	case "bad-status":
		writeConnectorCommandHelperResult(t, ConnectorActionResult{
			Status:            "owned",
			ExternalActionRef: "om_789",
		})
	case "stderr-redaction":
		_, _ = os.Stderr.WriteString("Authorization: Bearer sk-secret\nrate limited\n")
		os.Exit(42)
	case "unsafe-ref":
		writeConnectorCommandHelperResult(t, ConnectorActionResult{
			Status:            DeliveryStatusSent,
			ExternalActionRef: "Authorization: Bearer sk-secret",
		})
	default:
		t.Fatalf("unknown connector command helper mode %q", mode)
	}
	os.Exit(0)
}

func writeConnectorCommandHelperResult(t *testing.T, result ConnectorActionResult) {
	t.Helper()
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		t.Fatalf("encode result: %v", err)
	}
}
