package connectorruntime

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestExternalEventFromFeishuMessageReceiveUsesStableSourceFields(t *testing.T) {
	raw := []byte(`{
		"event_id":"evt_123",
		"chat_id":"oc_123",
		"chat_type":"group",
		"message_id":"om_123",
		"sender_id":"ou_123",
		"message_type":"text",
		"content":"hello from Feishu",
		"timestamp":"1782269315000",
		"type":"im.message.receive_v1"
	}`)

	event, err := ExternalEventFromFeishuMessageReceiveJSON(raw)
	if err != nil {
		t.Fatalf("ExternalEventFromFeishuMessageReceiveJSON returned error: %v", err)
	}
	if event.Connector != "feishu" || event.EventType != "message.created" {
		t.Fatalf("event identity = %+v", event)
	}
	if event.ExternalEventID != "evt_123" || event.ThreadRef.ExternalID != "oc_123" || event.MessageRef.ExternalID != "om_123" || event.SenderRef.ExternalID != "ou_123" {
		t.Fatalf("event refs = %+v", event)
	}
	if event.Body != "hello from Feishu" {
		t.Fatalf("body = %q", event.Body)
	}
	if event.SourceValidation != SourceValidationUnchecked {
		t.Fatalf("source validation = %q", event.SourceValidation)
	}
	if !event.ReceivedAt.Equal(time.UnixMilli(1782269315000).UTC()) {
		t.Fatalf("received_at = %s", event.ReceivedAt)
	}
	if event.Metadata["external_event_type"] != "im.message.receive_v1" || event.Metadata["message_type"] != "text" || event.Metadata["chat_type"] != "group" {
		t.Fatalf("metadata = %+v", event.Metadata)
	}
}

func TestExternalEventFromFeishuMessageReceiveRejectsMalformedSourceBeforeKernel(t *testing.T) {
	_, err := ExternalEventFromFeishuMessageReceive(FeishuMessageReceiveEvent{
		EventID:   "evt_123",
		ChatID:    "oc_123",
		MessageID: "om_123",
		Content:   "missing sender",
		Timestamp: "1782269315000",
		Type:      DefaultFeishuMessageEventKey,
	})
	if err == nil {
		t.Fatal("Feishu source event should reject missing sender_id")
	}

	_, err = ExternalEventFromFeishuMessageReceive(FeishuMessageReceiveEvent{
		EventID:   "evt_123",
		ChatID:    "oc_123",
		MessageID: "om_123",
		SenderID:  "ou_123",
		Content:   "bad type",
		Timestamp: "1782269315000",
		Type:      "im.chat.updated_v1",
	})
	if err == nil {
		t.Fatal("Feishu source event should reject unexpected event type")
	}

	_, err = ExternalEventFromFeishuMessageReceive(FeishuMessageReceiveEvent{
		EventID:   "evt_123",
		ChatID:    "oc_123",
		MessageID: "om_123",
		SenderID:  "ou_123",
		Content:   "bad timestamp",
		Timestamp: "not-a-timestamp",
		Type:      DefaultFeishuMessageEventKey,
	})
	if err == nil {
		t.Fatal("Feishu source event should reject invalid timestamp")
	}
}

func TestFeishuEventSourceCommandUsesExplicitProfileAndBoundedRun(t *testing.T) {
	executable, args, err := (FeishuEventSourceConfig{
		Executable: os.Args[0],
		Profile:    "codex",
		MaxEvents:  1,
		Timeout:    "30s",
	}).Command()
	if err != nil {
		t.Fatalf("Command returned error: %v", err)
	}
	if strings.TrimSpace(executable) == "" {
		t.Fatal("executable should be resolved")
	}
	want := []string{
		"--profile", "codex",
		"event", "consume", DefaultFeishuMessageEventKey,
		"--as", "bot",
		"--max-events", "1",
		"--timeout", "30s",
	}
	if strings.Join(args, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestFeishuEventSourceCommandRejectsMissingProfile(t *testing.T) {
	_, _, err := (FeishuEventSourceConfig{Executable: os.Args[0]}).Command()
	if err == nil {
		t.Fatal("Command should reject missing explicit profile")
	}
}

func TestFeishuEventSourceDiagnosticsRedactsCredentialShapedStderr(t *testing.T) {
	var diagnostics bytes.Buffer
	ready := make(chan struct{})
	done := make(chan struct{})

	go drainFeishuEventStderr(strings.NewReader("Authorization: Bearer sk-secret\n[event] ready event_key=im.message.receive_v1\n"), &diagnostics, ready, done)
	<-ready
	<-done

	text := diagnostics.String()
	if strings.Contains(text, "Authorization") || strings.Contains(text, "sk-secret") {
		t.Fatalf("diagnostics leaked credential-shaped stderr: %q", text)
	}
	if !strings.Contains(text, "[event] ready event_key=im.message.receive_v1") {
		t.Fatalf("diagnostics should keep non-secret ready marker, got %q", text)
	}
}

func TestFeishuEventSourceOversizedStdoutReturnsScannerError(t *testing.T) {
	oversized := strings.Repeat("x", 1024*1024+1)
	err := processFeishuEventStdout(strings.NewReader(oversized), io.Discard, func(ExternalEvent) error {
		t.Fatal("oversized stdout should fail before event handling")
		return nil
	})
	if err == nil {
		t.Fatal("oversized source stdout should return scanner error")
	}
}
