package connectorruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
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

func TestSourceCommandFramesIdleTimeoutFailsWithoutEmittingEvent(t *testing.T) {
	ctx := context.Background()
	reader, writer := io.Pipe()
	defer writer.Close()
	err := ConsumeSourceCommandFrames(ctx, reader, SourceCommandFrameConsumer{
		IdleTimeout: 10 * time.Millisecond,
	}, func(event ExternalEvent) error {
		t.Fatalf("idle source must not emit event: %+v", event)
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "idle timeout") {
		t.Fatalf("ConsumeSourceCommandFrames error = %v, want idle timeout", err)
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

func TestSourceCommandFramesRejectVerifiedEventWithUncheckedEvidence(t *testing.T) {
	ctx := context.Background()
	sourceStore, failureStore := newSourceCommandTestStores(t, "source-command-verified-weak-evidence")
	frames := `{"kind":"source.event","source_id":"source_feishu_chat","connector":"feishu","adapter_ref":"feishu-source-adapter","event":{"connector":"feishu","external_event_id":"evt_verified","event_type":"message.created","thread_ref":{"connector":"feishu","kind":"chat","external_id":"oc_1"},"sender_ref":{"connector":"feishu","kind":"user","external_id":"ou_1"},"message_ref":{"connector":"feishu","kind":"message","external_id":"om_1"},"body":"hello","source_validation":"verified"},"verification_evidence":{"source_event_ref":"evt_verified","source_id":"source_feishu_chat","connector":"feishu","validation_status":"unchecked","adapter_ref":"feishu-source-adapter"}}` + "\n"

	err := ConsumeSourceCommandFrames(ctx, strings.NewReader(frames), SourceCommandFrameConsumer{
		ExpectedSourceID:   "source_feishu_chat",
		ExpectedConnector:  "feishu",
		ExpectedAdapterRef: "feishu-source-adapter",
		SourceStore:        sourceStore,
		FailureStore:       failureStore,
	}, func(event ExternalEvent) error {
		t.Fatalf("verified event with weak evidence must not be handled: %+v", event)
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
		t.Fatalf("failures = %+v, want weak evidence rejection", failures)
	}
}

func TestSourceCommandFramesAcceptVerifiedEventWithBoundEvidence(t *testing.T) {
	ctx := context.Background()
	sourceStore, failureStore := newSourceCommandTestStores(t, "source-command-verified-bound-evidence")
	frames := `{"kind":"source.event","source_id":"source_feishu_chat","connector":"feishu","adapter_ref":"feishu-source-adapter","event":{"connector":"feishu","external_event_id":"evt_verified","event_type":"message.created","thread_ref":{"connector":"feishu","kind":"chat","external_id":"oc_1"},"sender_ref":{"connector":"feishu","kind":"user","external_id":"ou_1"},"message_ref":{"connector":"feishu","kind":"message","external_id":"om_1"},"body":"hello","source_validation":"verified"},"verification_evidence":{"source_event_ref":"evt_verified","source_id":"source_feishu_chat","connector":"feishu","validation_status":"verified","evidence_kind":"trusted_local_adapter_attestation","evidence_ref":"evidence_1","adapter_ref":"feishu-source-adapter"}}` + "\n"

	var handled []ExternalEvent
	err := ConsumeSourceCommandFrames(ctx, strings.NewReader(frames), SourceCommandFrameConsumer{
		ExpectedSourceID:   "source_feishu_chat",
		ExpectedConnector:  "feishu",
		ExpectedAdapterRef: "feishu-source-adapter",
		SourceStore:        sourceStore,
		FailureStore:       failureStore,
	}, func(event ExternalEvent) error {
		handled = append(handled, event)
		return nil
	})
	if err != nil {
		t.Fatalf("ConsumeSourceCommandFrames returned error: %v", err)
	}
	if len(handled) != 1 || handled[0].ExternalEventID != "evt_verified" || handled[0].SourceValidation != SourceValidationVerified {
		t.Fatalf("handled = %+v, want one verified event", handled)
	}
	evidence, err := sourceStore.ListSourceVerifications(ctx)
	if err != nil {
		t.Fatalf("ListSourceVerifications returned error: %v", err)
	}
	if len(evidence) != 1 || evidence[0].SourceID != "source_feishu_chat" || evidence[0].Connector != "feishu" || evidence[0].EvidenceKind != SourceEvidenceKindTrustedLocalAdapterAttestation {
		t.Fatalf("evidence = %+v, want bound approved source evidence", evidence)
	}
}

func TestSourceCommandFramesRejectVerifiedEventWithUnapprovedEvidenceKind(t *testing.T) {
	ctx := context.Background()
	sourceStore, failureStore := newSourceCommandTestStores(t, "source-command-verified-bad-evidence-kind")
	frames := `{"kind":"source.event","source_id":"source_feishu_chat","connector":"feishu","adapter_ref":"feishu-source-adapter","event":{"connector":"feishu","external_event_id":"evt_verified","event_type":"message.created","thread_ref":{"connector":"feishu","kind":"chat","external_id":"oc_1"},"sender_ref":{"connector":"feishu","kind":"user","external_id":"ou_1"},"message_ref":{"connector":"feishu","kind":"message","external_id":"om_1"},"body":"hello","source_validation":"verified"},"verification_evidence":{"source_event_ref":"evt_verified","source_id":"source_feishu_chat","connector":"feishu","validation_status":"verified","evidence_kind":"unknown_adapter_claim","evidence_ref":"evidence_1","adapter_ref":"feishu-source-adapter"}}` + "\n"

	err := ConsumeSourceCommandFrames(ctx, strings.NewReader(frames), SourceCommandFrameConsumer{
		ExpectedSourceID:   "source_feishu_chat",
		ExpectedConnector:  "feishu",
		ExpectedAdapterRef: "feishu-source-adapter",
		SourceStore:        sourceStore,
		FailureStore:       failureStore,
	}, func(event ExternalEvent) error {
		t.Fatalf("verified event with unapproved evidence kind must not be handled: %+v", event)
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
		t.Fatalf("failures = %+v, want evidence kind rejection", failures)
	}
}

func TestSourceCommandFramesRejectEventConnectorMismatch(t *testing.T) {
	ctx := context.Background()
	sourceStore, failureStore := newSourceCommandTestStores(t, "source-command-connector-mismatch")
	frames := `{"kind":"source.event","source_id":"source_feishu_chat","connector":"feishu","adapter_ref":"feishu-source-adapter","event":{"connector":"email","external_event_id":"evt_wrong_connector","event_type":"message.created","thread_ref":{"connector":"email","kind":"inbox","external_id":"thread_1"},"sender_ref":{"connector":"email","kind":"user","external_id":"sender_1"},"message_ref":{"connector":"email","kind":"message","external_id":"msg_1"},"body":"hello","source_validation":"unchecked"}}` + "\n"

	err := ConsumeSourceCommandFrames(ctx, strings.NewReader(frames), SourceCommandFrameConsumer{
		ExpectedSourceID:   "source_feishu_chat",
		ExpectedConnector:  "feishu",
		ExpectedAdapterRef: "feishu-source-adapter",
		SourceStore:        sourceStore,
		FailureStore:       failureStore,
	}, func(event ExternalEvent) error {
		t.Fatalf("event from mismatched connector must not be handled: %+v", event)
		return nil
	})
	if err != nil {
		t.Fatalf("ConsumeSourceCommandFrames returned error: %v", err)
	}
	failures, err := failureStore.ListSourceFailures(ctx)
	if err != nil {
		t.Fatalf("ListSourceFailures returned error: %v", err)
	}
	if len(failures) != 1 || failures[0].Reason != "source_payload_malformed" {
		t.Fatalf("failures = %+v, want connector mismatch failure", failures)
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

func TestSourceCommandFrameFailureUsesExpectedSourceIdentity(t *testing.T) {
	ctx := context.Background()
	sourceStore, failureStore := newSourceCommandTestStores(t, "source-command-failure-expected-identity")
	frames := `{"kind":"source.failed","source_id":"sk-raw-secret","connector":"sk-connector-secret","event_source":"Bearer token","reason":"source_runtime_failed","detail":"raw payload {\"secret\":\"value\"}","payload_hash":"not-a-hash"}` + "\n"

	err := ConsumeSourceCommandFrames(ctx, strings.NewReader(frames), SourceCommandFrameConsumer{
		ExpectedSourceID:   "source_feishu_chat",
		ExpectedConnector:  "feishu",
		ExpectedAdapterRef: "feishu-source-adapter",
		SourceStore:        sourceStore,
		FailureStore:       failureStore,
	}, func(event ExternalEvent) error {
		t.Fatalf("source.failed frame must not emit event: %+v", event)
		return nil
	})
	if err != nil {
		t.Fatalf("ConsumeSourceCommandFrames returned error: %v", err)
	}
	failures, err := failureStore.ListSourceFailures(ctx)
	if err != nil {
		t.Fatalf("ListSourceFailures returned error: %v", err)
	}
	if len(failures) != 1 {
		t.Fatalf("failures = %+v, want one source failure", failures)
	}
	failure := failures[0]
	if failure.Connector != "feishu" || failure.EventSource != "feishu-source-adapter" || failure.SourceRunRef != "source_feishu_chat" {
		t.Fatalf("failure identity = connector %q event_source %q source %q, want expected source identity", failure.Connector, failure.EventSource, failure.SourceRunRef)
	}
	if failure.PayloadHash != "" {
		t.Fatalf("payload_hash = %q, want invalid adapter-supplied hash dropped", failure.PayloadHash)
	}
	if strings.Contains(failure.Detail, "secret") || strings.Contains(failure.DiagnosticExcerpt, "secret") {
		t.Fatalf("failure diagnostics were not redacted: %+v", failure)
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

func TestSourceCommandAdapterReadinessBlockDoesNotStartProcess(t *testing.T) {
	ctx := context.Background()
	sourceStore, failureStore := newSourceCommandTestStores(t, "source-command-readiness-block")
	startedPath := filepath.Join(testsupport.ProjectTempDir(t, "source-command-readiness-side-effect"), "started.txt")
	env := append(connectorCommandEnvironment(os.Environ()),
		"GENESIS_SOURCE_COMMAND_HELPER=record-start",
		"GENESIS_SOURCE_COMMAND_STARTED_FILE="+startedPath,
	)
	adapter := SourceCommandAdapter{
		Executable:                os.Args[0],
		Args:                      []string{"-test.run=TestSourceCommandAdapterHelper"},
		Env:                       env,
		SourceID:                  "source_feishu_chat",
		Connector:                 "feishu",
		AdapterRef:                "feishu-source-adapter",
		SourceStore:               sourceStore,
		FailureStore:              failureStore,
		ReadinessBlockReasonCode:  SourceReadinessReasonProfileExpired,
		ReadinessBlockDescription: "profile expired before source start",
	}
	err := adapter.Consume(ctx, func(event ExternalEvent) error {
		t.Fatalf("blocked source command must not emit event: %+v", event)
		return nil
	})
	if !errors.Is(err, ErrSourceCommandBlocked) {
		t.Fatalf("Consume error = %v, want source command blocked", err)
	}
	if _, err := os.Stat(startedPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source process side effect exists or stat failed: %v", err)
	}
	runs, err := sourceStore.ListSourceRuns(ctx)
	if err != nil {
		t.Fatalf("ListSourceRuns returned error: %v", err)
	}
	if len(runs) != 1 || runs[0].Status != SourceRunStatusBlocked || runs[0].BlockedReasonCode != SourceReadinessReasonProfileExpired {
		t.Fatalf("runs = %+v, want profile_expired blocked run", runs)
	}
	attempts, err := sourceStore.ListSourceAttempts(ctx, "source_feishu_chat")
	if err != nil {
		t.Fatalf("ListSourceAttempts returned error: %v", err)
	}
	if len(attempts) != 1 || attempts[0].Outcome != SourceAttemptOutcomeBlocked {
		t.Fatalf("attempts = %+v, want blocked attempt", attempts)
	}
}

func TestFileSourceLifecycleStoreListsVerificationEvidence(t *testing.T) {
	ctx := context.Background()
	sourceStore, _ := newSourceCommandTestStores(t, "source-command-verification-list")
	err := sourceStore.RecordSourceVerification(ctx, SourceVerificationEvidence{
		SourceEventRef:   "evt_verified",
		SourceID:         "source_feishu_chat",
		Connector:        "feishu",
		ValidationStatus: SourceValidationVerified,
		EvidenceKind:     SourceEvidenceKindTrustedLocalAdapterAttestation,
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
		emitReadyEventStoppedFrames(t)
	case "fail-once-then-ready-event-stopped":
		attemptFile := os.Getenv("GENESIS_SOURCE_COMMAND_ATTEMPT_FILE")
		if attemptFile == "" {
			t.Fatal("GENESIS_SOURCE_COMMAND_ATTEMPT_FILE is required")
		}
		attempt := 0
		if raw, err := os.ReadFile(attemptFile); err == nil && strings.TrimSpace(string(raw)) != "" {
			parsed, parseErr := strconv.Atoi(strings.TrimSpace(string(raw)))
			if parseErr != nil {
				t.Fatalf("parse attempt file: %v", parseErr)
			}
			attempt = parsed
		}
		attempt++
		if err := os.WriteFile(attemptFile, []byte(strconv.Itoa(attempt)), 0o600); err != nil {
			t.Fatalf("write attempt file: %v", err)
		}
		if attempt == 1 {
			fmt.Fprintln(os.Stderr, "transient source runtime failure")
			os.Exit(42)
		}
		emitReadyEventStoppedFrames(t)
	case "record-start":
		startedPath := os.Getenv("GENESIS_SOURCE_COMMAND_STARTED_FILE")
		if startedPath == "" {
			t.Fatal("GENESIS_SOURCE_COMMAND_STARTED_FILE is required")
		}
		if err := os.WriteFile(startedPath, []byte("started"), 0o600); err != nil {
			t.Fatalf("write started file: %v", err)
		}
		emitReadyEventStoppedFrames(t)
	default:
		t.Fatalf("unknown source command helper mode %q", mode)
	}
	os.Exit(0)
}

func emitReadyEventStoppedFrames(t *testing.T) {
	t.Helper()
	sourceID := envOrFallback("GENESIS_SOURCE_COMMAND_SOURCE_ID", "source_feishu_chat")
	connector := envOrFallback("GENESIS_SOURCE_COMMAND_CONNECTOR", "feishu")
	adapterRef := envOrFallback("GENESIS_SOURCE_COMMAND_ADAPTER_REF", "feishu-source-adapter")
	frames := []SourceCommandFrame{
		{Kind: SourceFrameKindReady, SourceID: sourceID, Connector: connector, AdapterRef: adapterRef},
		{
			Kind:       SourceFrameKindEvent,
			SourceID:   sourceID,
			Connector:  connector,
			AdapterRef: adapterRef,
			Event: &ExternalEvent{
				Connector:       connector,
				ExternalEventID: "evt_self",
				EventType:       "message.created",
				ThreadRef:       ExternalThreadRef{Connector: connector, Kind: "chat", ExternalID: "oc_1"},
				SenderRef:       ExternalRef{Connector: connector, Kind: "user", ExternalID: "bot_self"},
				MessageRef:      ExternalRef{Connector: connector, Kind: "message", ExternalID: "om_self"},
				Body:            "self message",
				ReceivedAt:      time.Date(2026, 6, 24, 16, 0, 0, 0, time.UTC),
			},
		},
		{
			Kind:       SourceFrameKindEvent,
			SourceID:   sourceID,
			Connector:  connector,
			AdapterRef: adapterRef,
			Event: &ExternalEvent{
				Connector:        connector,
				ExternalEventID:  "evt_1",
				EventType:        "message.created",
				ThreadRef:        ExternalThreadRef{Connector: connector, Kind: "chat", ExternalID: "oc_1"},
				SenderRef:        ExternalRef{Connector: connector, Kind: "user", ExternalID: "ou_1"},
				MessageRef:       ExternalRef{Connector: connector, Kind: "message", ExternalID: "om_1"},
				Body:             "hello",
				ReceivedAt:       time.Date(2026, 6, 24, 16, 0, 1, 0, time.UTC),
				SourceValidation: SourceValidationUnchecked,
			},
		},
		{Kind: SourceFrameKindStopped, SourceID: sourceID, Connector: connector, AdapterRef: adapterRef},
	}
	encoder := json.NewEncoder(os.Stdout)
	for _, frame := range frames {
		if err := encoder.Encode(frame); err != nil {
			t.Fatalf("encode frame: %v", err)
		}
	}
}

func envOrFallback(name string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func newSourceCommandTestStores(t *testing.T, name string) (*FileSourceLifecycleStore, *FileSourceFailureStore) {
	t.Helper()
	dir := testsupport.ProjectTempDir(t, name)
	sourceStore, err := NewFileSourceLifecycleStore(filepath.Join(dir, "source-lifecycle.json"))
	if err != nil {
		t.Fatalf("NewFileSourceLifecycleStore returned error: %v", err)
	}
	failureStore, err := NewFileSourceFailureStore(filepath.Join(dir, "source-failures.json"))
	if err != nil {
		t.Fatalf("NewFileSourceFailureStore returned error: %v", err)
	}
	return sourceStore, failureStore
}
