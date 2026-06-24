package connectorruntime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"genesis/internal/testsupport"
)

func TestSourceCommandFramesAcceptEventThenAdvanceCursor(t *testing.T) {
	ctx := context.Background()
	sourceStore, failureStore := newSourceCommandTestStores(t, "source-command-accept")
	frames := strings.Join([]string{
		`{"kind":"source.ready","source_id":"source_feishu_chat","connector":"feishu","adapter_ref":"feishu-source-adapter"}`,
		`{"kind":"source.event","source_id":"source_feishu_chat","event":{"connector":"feishu","external_event_id":"evt_1","event_type":"message.created","thread_ref":{"connector":"feishu","kind":"chat","external_id":"oc_1"},"sender_ref":{"connector":"feishu","kind":"user","external_id":"ou_1"},"message_ref":{"connector":"feishu","kind":"message","external_id":"om_1"},"body":"hello","source_validation":"unchecked"}}`,
		`{"kind":"source.cursor","source_id":"source_feishu_chat","after_event_id":"evt_1","cursor":{"source_id":"source_feishu_chat","cursor_kind":"external_event_id","cursor_value":"evt_1"}}`,
		`{"kind":"source.stopped","source_id":"source_feishu_chat"}`,
		"",
	}, "\n")

	var handled []ExternalEvent
	err := ConsumeSourceCommandFrames(ctx, strings.NewReader(frames), SourceCommandFrameConsumer{
		SourceStore:  sourceStore,
		FailureStore: failureStore,
	}, func(event ExternalEvent) error {
		handled = append(handled, event)
		return nil
	})
	if err != nil {
		t.Fatalf("ConsumeSourceCommandFrames returned error: %v", err)
	}
	if len(handled) != 1 || handled[0].ExternalEventID != "evt_1" {
		t.Fatalf("handled = %+v, want accepted source event", handled)
	}
	cursor, ok, err := sourceStore.GetSourceCursor(ctx, "source_feishu_chat", SourceCursorKindExternalEventID)
	if err != nil {
		t.Fatalf("GetSourceCursor returned error: %v", err)
	}
	if !ok || cursor.CursorValue != "evt_1" {
		t.Fatalf("cursor = %+v ok=%v, want accepted event cursor", cursor, ok)
	}
	runs, err := sourceStore.ListSourceRuns(ctx)
	if err != nil {
		t.Fatalf("ListSourceRuns returned error: %v", err)
	}
	if len(runs) != 1 || runs[0].Status != SourceRunStatusStopped || runs[0].Connector != "feishu" {
		t.Fatalf("runs = %+v, want stopped Feishu source run", runs)
	}
	failures, err := failureStore.ListSourceFailures(ctx)
	if err != nil {
		t.Fatalf("ListSourceFailures returned error: %v", err)
	}
	if len(failures) != 0 {
		t.Fatalf("failures = %+v, want no source failures", failures)
	}
}

func TestSourceCommandFramesRejectVerifiedEventWithoutEvidence(t *testing.T) {
	ctx := context.Background()
	sourceStore, failureStore := newSourceCommandTestStores(t, "source-command-verified-no-evidence")
	frames := `{"kind":"source.event","source_id":"source_feishu_chat","event":{"connector":"feishu","external_event_id":"evt_verified","event_type":"message.created","thread_ref":{"connector":"feishu","kind":"chat","external_id":"oc_1"},"sender_ref":{"connector":"feishu","kind":"user","external_id":"ou_1"},"message_ref":{"connector":"feishu","kind":"message","external_id":"om_1"},"body":"hello","source_validation":"verified"}}` + "\n"

	err := ConsumeSourceCommandFrames(ctx, strings.NewReader(frames), SourceCommandFrameConsumer{
		SourceStore:  sourceStore,
		FailureStore: failureStore,
	}, func(event ExternalEvent) error {
		t.Fatalf("verified event without evidence must not be handled: %+v", event)
		return nil
	})
	if err != nil {
		t.Fatalf("ConsumeSourceCommandFrames returned error: %v", err)
	}
	failures, err := failureStore.ListSourceFailures(ctx)
	if err != nil {
		t.Fatalf("ListSourceFailures returned error: %v", err)
	}
	if len(failures) != 1 || failures[0].Reason != "source_verification_failed" || failures[0].SourceValidation != SourceValidationRejected {
		t.Fatalf("failures = %+v, want rejected verification failure", failures)
	}
}

func TestSourceCommandFramesRejectUnsupportedSourceValidation(t *testing.T) {
	ctx := context.Background()
	sourceStore, failureStore := newSourceCommandTestStores(t, "source-command-bad-validation")
	frames := `{"kind":"source.event","source_id":"source_feishu_chat","event":{"connector":"feishu","external_event_id":"evt_bad_status","event_type":"message.created","thread_ref":{"connector":"feishu","kind":"chat","external_id":"oc_1"},"sender_ref":{"connector":"feishu","kind":"user","external_id":"ou_1"},"message_ref":{"connector":"feishu","kind":"message","external_id":"om_1"},"body":"hello","source_validation":"trusted"}}` + "\n"

	err := ConsumeSourceCommandFrames(ctx, strings.NewReader(frames), SourceCommandFrameConsumer{
		SourceStore:  sourceStore,
		FailureStore: failureStore,
	}, func(event ExternalEvent) error {
		t.Fatalf("event with unsupported source validation must not be handled: %+v", event)
		return nil
	})
	if err != nil {
		t.Fatalf("ConsumeSourceCommandFrames returned error: %v", err)
	}
	failures, err := failureStore.ListSourceFailures(ctx)
	if err != nil {
		t.Fatalf("ListSourceFailures returned error: %v", err)
	}
	if len(failures) != 1 || failures[0].Reason != "source_verification_failed" {
		t.Fatalf("failures = %+v, want source validation failure", failures)
	}
}

func TestSourceCommandFramesDoNotEmitRejectedEvent(t *testing.T) {
	ctx := context.Background()
	sourceStore, failureStore := newSourceCommandTestStores(t, "source-command-rejected-event")
	frames := `{"kind":"source.event","source_id":"source_feishu_chat","event":{"connector":"feishu","external_event_id":"evt_rejected","event_type":"message.created","thread_ref":{"connector":"feishu","kind":"chat","external_id":"oc_1"},"sender_ref":{"connector":"feishu","kind":"user","external_id":"ou_1"},"message_ref":{"connector":"feishu","kind":"message","external_id":"om_1"},"body":"hello","source_validation":"rejected"}}` + "\n"

	err := ConsumeSourceCommandFrames(ctx, strings.NewReader(frames), SourceCommandFrameConsumer{
		SourceStore:  sourceStore,
		FailureStore: failureStore,
	}, func(event ExternalEvent) error {
		t.Fatalf("rejected source event must not be handled: %+v", event)
		return nil
	})
	if err != nil {
		t.Fatalf("ConsumeSourceCommandFrames returned error: %v", err)
	}
	failures, err := failureStore.ListSourceFailures(ctx)
	if err != nil {
		t.Fatalf("ListSourceFailures returned error: %v", err)
	}
	if len(failures) != 1 || failures[0].Reason != "source_policy_rejected" {
		t.Fatalf("failures = %+v, want source policy rejection", failures)
	}
}

func TestSourceCommandFramesRejectMalformedFrameWithoutExternalEvent(t *testing.T) {
	ctx := context.Background()
	sourceStore, failureStore := newSourceCommandTestStores(t, "source-command-malformed")
	frames := "{not-json}\n"

	err := ConsumeSourceCommandFrames(ctx, strings.NewReader(frames), SourceCommandFrameConsumer{
		SourceStore:  sourceStore,
		FailureStore: failureStore,
	}, func(event ExternalEvent) error {
		t.Fatalf("malformed frame must not be handled: %+v", event)
		return nil
	})
	if err != nil {
		t.Fatalf("ConsumeSourceCommandFrames returned error: %v", err)
	}
	failures, err := failureStore.ListSourceFailures(ctx)
	if err != nil {
		t.Fatalf("ListSourceFailures returned error: %v", err)
	}
	if len(failures) != 1 || failures[0].Reason != "malformed_source_frame" || failures[0].PayloadHash == "" || failures[0].PayloadSizeBytes == 0 {
		t.Fatalf("failures = %+v, want redacted malformed frame failure with payload metadata", failures)
	}
}

func TestSourceCommandFramesDoNotAdvanceCursorBeforeAcceptedEvent(t *testing.T) {
	ctx := context.Background()
	sourceStore, failureStore := newSourceCommandTestStores(t, "source-command-cursor-before-accept")
	frames := `{"kind":"source.cursor","source_id":"source_feishu_chat","after_event_id":"evt_missing","cursor":{"source_id":"source_feishu_chat","cursor_kind":"external_event_id","cursor_value":"evt_missing"}}` + "\n"

	err := ConsumeSourceCommandFrames(ctx, strings.NewReader(frames), SourceCommandFrameConsumer{
		SourceStore:  sourceStore,
		FailureStore: failureStore,
	}, func(event ExternalEvent) error {
		t.Fatalf("cursor-only frame must not emit event: %+v", event)
		return nil
	})
	if err != nil {
		t.Fatalf("ConsumeSourceCommandFrames returned error: %v", err)
	}
	if _, ok, err := sourceStore.GetSourceCursor(ctx, "source_feishu_chat", SourceCursorKindExternalEventID); err != nil {
		t.Fatalf("GetSourceCursor returned error: %v", err)
	} else if ok {
		t.Fatal("cursor advanced before accepted event")
	}
	failures, err := failureStore.ListSourceFailures(ctx)
	if err != nil {
		t.Fatalf("ListSourceFailures returned error: %v", err)
	}
	if len(failures) != 1 || failures[0].Reason != "source_cursor_failed" {
		t.Fatalf("failures = %+v, want source cursor failure", failures)
	}
}

func TestSourceCommandAdapterRunsTypedSourceProcess(t *testing.T) {
	ctx := context.Background()
	sourceStore, failureStore := newSourceCommandTestStores(t, "source-command-process")
	env := append(connectorCommandEnvironment(os.Environ()), "GENESIS_SOURCE_COMMAND_HELPER=ready-event-stopped")
	adapter := SourceCommandAdapter{
		Executable:      os.Args[0],
		Args:            []string{"-test.run=TestSourceCommandAdapterHelper"},
		Env:             env,
		SourceID:        "source_feishu_chat",
		Connector:       "feishu",
		AdapterRef:      "feishu-source-adapter",
		SourceStore:     sourceStore,
		FailureStore:    failureStore,
		IgnoreSenderIDs: []string{"bot_self"},
	}
	var handled []ExternalEvent
	err := adapter.Consume(ctx, func(event ExternalEvent) error {
		handled = append(handled, event)
		return nil
	})
	if err != nil {
		t.Fatalf("Consume returned error: %v", err)
	}
	if len(handled) != 1 || handled[0].ExternalEventID != "evt_1" {
		t.Fatalf("handled = %+v, want one non-ignored event", handled)
	}
	attempts, err := sourceStore.ListSourceAttempts(ctx, "source_feishu_chat")
	if err != nil {
		t.Fatalf("ListSourceAttempts returned error: %v", err)
	}
	if len(attempts) != 1 || attempts[0].Outcome != SourceAttemptOutcomeStopped {
		t.Fatalf("attempts = %+v, want stopped source command attempt", attempts)
	}
}

func TestFileSourceSupervisorStoreListsVerificationEvidence(t *testing.T) {
	ctx := context.Background()
	sourceStore, _ := newSourceCommandTestStores(t, "source-command-verification-list")
	err := sourceStore.RecordSourceVerification(ctx, SourceVerificationEvidence{
		SourceEventRef:   "evt_verified",
		ValidationStatus: SourceValidationVerified,
		EvidenceKind:     "trusted_adapter_assertion",
		EvidenceRef:      "evidence_1",
		AdapterRef:       "feishu-source-adapter",
	})
	if err != nil {
		t.Fatalf("RecordSourceVerification returned error: %v", err)
	}
	evidence, err := sourceStore.ListSourceVerifications(ctx)
	if err != nil {
		t.Fatalf("ListSourceVerifications returned error: %v", err)
	}
	if len(evidence) != 1 || evidence[0].SourceEventRef != "evt_verified" || evidence[0].EvidenceRef != "evidence_1" {
		t.Fatalf("evidence = %+v, want inspectable verification evidence", evidence)
	}
}

func TestSourceCommandAdapterHelper(t *testing.T) {
	mode := os.Getenv("GENESIS_SOURCE_COMMAND_HELPER")
	if mode == "" {
		return
	}
	switch mode {
	case "ready-event-stopped":
		frames := []SourceCommandFrame{
			{Kind: SourceFrameKindReady, SourceID: "source_feishu_chat", Connector: "feishu", AdapterRef: "feishu-source-adapter"},
			{
				Kind:     SourceFrameKindEvent,
				SourceID: "source_feishu_chat",
				Event: &ExternalEvent{
					Connector:       "feishu",
					ExternalEventID: "evt_self",
					EventType:       "message.created",
					ThreadRef:       ExternalThreadRef{Connector: "feishu", Kind: "chat", ExternalID: "oc_1"},
					SenderRef:       ExternalRef{Connector: "feishu", Kind: "user", ExternalID: "bot_self"},
					MessageRef:      ExternalRef{Connector: "feishu", Kind: "message", ExternalID: "om_self"},
					Body:            "self message",
					ReceivedAt:      time.Date(2026, 6, 24, 16, 0, 0, 0, time.UTC),
				},
			},
			{
				Kind:     SourceFrameKindEvent,
				SourceID: "source_feishu_chat",
				Event: &ExternalEvent{
					Connector:        "feishu",
					ExternalEventID:  "evt_1",
					EventType:        "message.created",
					ThreadRef:        ExternalThreadRef{Connector: "feishu", Kind: "chat", ExternalID: "oc_1"},
					SenderRef:        ExternalRef{Connector: "feishu", Kind: "user", ExternalID: "ou_1"},
					MessageRef:       ExternalRef{Connector: "feishu", Kind: "message", ExternalID: "om_1"},
					Body:             "hello",
					ReceivedAt:       time.Date(2026, 6, 24, 16, 0, 1, 0, time.UTC),
					SourceValidation: SourceValidationUnchecked,
				},
			},
			{Kind: SourceFrameKindStopped, SourceID: "source_feishu_chat"},
		}
		encoder := json.NewEncoder(os.Stdout)
		for _, frame := range frames {
			if err := encoder.Encode(frame); err != nil {
				t.Fatalf("encode frame: %v", err)
			}
		}
	default:
		t.Fatalf("unknown source command helper mode %q", mode)
	}
	os.Exit(0)
}

func newSourceCommandTestStores(t *testing.T, name string) (*FileSourceSupervisorStore, *FileSourceFailureStore) {
	t.Helper()
	dir := testsupport.ProjectTempDir(t, name)
	sourceStore, err := NewFileSourceSupervisorStore(filepath.Join(dir, "source-supervisor.json"))
	if err != nil {
		t.Fatalf("NewFileSourceSupervisorStore returned error: %v", err)
	}
	failureStore, err := NewFileSourceFailureStore(filepath.Join(dir, "source-failures.json"))
	if err != nil {
		t.Fatalf("NewFileSourceFailureStore returned error: %v", err)
	}
	return sourceStore, failureStore
}
