package connectorruntime

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"genesis/internal/testsupport"
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

func TestFeishuEventSourceIgnoresConfiguredSenderIDs(t *testing.T) {
	var handled []ExternalEvent
	input := strings.Join([]string{
		`{"event_id":"evt_self","chat_id":"oc_123","chat_type":"group","message_id":"om_self","sender_id":"cli_self","message_type":"text","content":"self reply","timestamp":"1782269315000","type":"im.message.receive_v1"}`,
		`{"event_id":"evt_user","chat_id":"oc_123","chat_type":"group","message_id":"om_user","sender_id":"ou_user","message_type":"text","content":"user message","timestamp":"1782269316000","type":"im.message.receive_v1"}`,
		"",
	}, "\n")

	err := processFeishuEventStdout(context.Background(), strings.NewReader(input), io.Discard, ignoreSenderIDSet([]string{"cli_self"}), nil, func(event ExternalEvent) error {
		handled = append(handled, event)
		return nil
	})
	if err != nil {
		t.Fatalf("processFeishuEventStdout returned error: %v", err)
	}
	if len(handled) != 1 || handled[0].ExternalEventID != "evt_user" {
		t.Fatalf("handled events = %+v, want only user event", handled)
	}
}

func TestFeishuEventSourceRecordsMalformedSourceFailureBeforeKernel(t *testing.T) {
	ctx := context.Background()
	store, err := NewFileSourceFailureStore(filepath.Join(testsupport.ProjectTempDir(t, "feishu-source-failures"), "source-failures.json"))
	if err != nil {
		t.Fatalf("NewFileSourceFailureStore returned error: %v", err)
	}
	input := strings.Join([]string{
		`{"event_id":"evt_bad","chat_id":"oc_123","chat_type":"group","message_id":"om_bad","message_type":"text","content":"missing sender","timestamp":"1782269315000","type":"im.message.receive_v1"}`,
		`{"event_id":"evt_user","chat_id":"oc_123","chat_type":"group","message_id":"om_user","sender_id":"ou_user","message_type":"text","content":"user message","timestamp":"1782269316000","type":"im.message.receive_v1"}`,
		"",
	}, "\n")

	var handled []ExternalEvent
	err = processFeishuEventStdout(ctx, strings.NewReader(input), io.Discard, nil, store, func(event ExternalEvent) error {
		handled = append(handled, event)
		return nil
	})
	if err != nil {
		t.Fatalf("processFeishuEventStdout returned error: %v", err)
	}
	if len(handled) != 1 || handled[0].ExternalEventID != "evt_user" {
		t.Fatalf("handled events = %+v, want only valid user event", handled)
	}
	failures, err := store.ListSourceFailures(ctx)
	if err != nil {
		t.Fatalf("ListSourceFailures returned error: %v", err)
	}
	if len(failures) != 1 {
		t.Fatalf("source failures = %+v, want one malformed source record", failures)
	}
	failure := failures[0]
	if failure.Connector != "feishu" || failure.Reason != "malformed_source_event" || failure.SourceValidation != SourceValidationRejected {
		t.Fatalf("failure identity = %+v", failure)
	}
	if !strings.Contains(failure.Detail, "missing sender_id") {
		t.Fatalf("failure detail = %q, want missing sender evidence", failure.Detail)
	}
	if strings.TrimSpace(failure.RawExcerpt) == "" || !strings.Contains(failure.RawExcerpt, "evt_bad") {
		t.Fatalf("raw excerpt = %q, want bounded source excerpt", failure.RawExcerpt)
	}
}

func TestFeishuEventSourceFailsClosedWhenSourceFailureCannotBeRecorded(t *testing.T) {
	var handled []ExternalEvent
	err := processFeishuEventStdout(context.Background(), strings.NewReader(`{"event_id":"evt_bad","chat_id":"oc_123","message_id":"om_bad","content":"missing sender","timestamp":"1782269315000","type":"im.message.receive_v1"}`+"\n"), io.Discard, nil, failingSourceFailureStore{}, func(event ExternalEvent) error {
		handled = append(handled, event)
		return nil
	})
	if err == nil {
		t.Fatal("processFeishuEventStdout should fail when source failure evidence cannot be recorded")
	}
	if !strings.Contains(err.Error(), "record Feishu source failure") {
		t.Fatalf("error = %v, want source failure recording evidence", err)
	}
	if len(handled) != 0 {
		t.Fatalf("handled events = %+v, want no kernel-bound events after evidence write failure", handled)
	}
}

type failingSourceFailureStore struct{}

func (failingSourceFailureStore) RecordSourceFailure(context.Context, SourceFailureRecord) error {
	return errors.New("source store unavailable")
}

func (failingSourceFailureStore) ListSourceFailures(context.Context) ([]SourceFailureRecord, error) {
	return nil, nil
}

func TestFeishuEventSourceOversizedStdoutReturnsScannerError(t *testing.T) {
	oversized := strings.Repeat("x", 1024*1024+1)
	err := processFeishuEventStdout(context.Background(), strings.NewReader(oversized), io.Discard, nil, nil, func(ExternalEvent) error {
		t.Fatal("oversized stdout should fail before event handling")
		return nil
	})
	if err == nil {
		t.Fatal("oversized source stdout should return scanner error")
	}
}
