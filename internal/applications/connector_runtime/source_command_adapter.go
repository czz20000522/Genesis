package connectorruntime

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	SourceFrameKindReady   = "source.ready"
	SourceFrameKindEvent   = "source.event"
	SourceFrameKindCursor  = "source.cursor"
	SourceFrameKindFailed  = "source.failed"
	SourceFrameKindStopped = "source.stopped"
)

type SourceCommandFrameConsumer struct {
	ExpectedSourceID  string
	ExpectedConnector string
	SourceStore       SourceSupervisorStore
	FailureStore      SourceFailureStore
	IgnoreSenderIDs   []string
	Now               func() time.Time
}

type SourceCommandAdapter struct {
	Executable string
	Args       []string
	Env        []string
	WorkingDir string
	SourceID   string
	Connector  string
	AdapterRef string

	SourceStore     SourceSupervisorStore
	FailureStore    SourceFailureStore
	IgnoreSenderIDs []string
	Now             func() time.Time
}

type SourceCommandFrame struct {
	Kind                 string                      `json:"kind"`
	SourceID             string                      `json:"source_id,omitempty"`
	Connector            string                      `json:"connector,omitempty"`
	AdapterRef           string                      `json:"adapter_ref,omitempty"`
	EventSource          string                      `json:"event_source,omitempty"`
	Event                *ExternalEvent              `json:"event,omitempty"`
	Cursor               *SourceCommandCursorFrame   `json:"cursor,omitempty"`
	VerificationEvidence *SourceVerificationEvidence `json:"verification_evidence,omitempty"`
	AfterEventID         string                      `json:"after_event_id,omitempty"`
	Reason               string                      `json:"reason,omitempty"`
	Detail               string                      `json:"detail,omitempty"`
	PayloadHash          string                      `json:"payload_hash,omitempty"`
	PayloadSizeBytes     int                         `json:"payload_size_bytes,omitempty"`
}

type SourceCommandCursorFrame struct {
	SourceID     string    `json:"source_id,omitempty"`
	CursorKind   string    `json:"cursor_kind"`
	CursorValue  string    `json:"cursor_value"`
	WatermarkAt  time.Time `json:"watermark_at,omitempty"`
	UpdatedAt    time.Time `json:"updated_at,omitempty"`
	AfterEventID string    `json:"after_event_id,omitempty"`
}

func (a SourceCommandAdapter) Consume(ctx context.Context, handle func(ExternalEvent) error) error {
	if handle == nil {
		return errors.New("source command event handler is required")
	}
	sourceID := strings.TrimSpace(a.SourceID)
	connector := strings.TrimSpace(a.Connector)
	adapterRef := strings.TrimSpace(a.AdapterRef)
	switch {
	case sourceID == "":
		return errors.New("source command source_id is required")
	case connector == "":
		return errors.New("source command connector is required")
	case adapterRef == "":
		return errors.New("source command adapter_ref is required")
	}
	startedAt := sourceCommandNow(SourceCommandFrameConsumer{Now: a.Now})
	if err := a.recordRun(ctx, SourceRunStatusStarting, "", startedAt, time.Time{}); err != nil {
		return err
	}
	executable, err := a.resolveExecutable()
	if err != nil {
		endedAt := sourceCommandNow(SourceCommandFrameConsumer{Now: a.Now})
		if recordErr := a.recordRun(ctx, SourceRunStatusBlocked, err.Error(), startedAt, time.Time{}); recordErr != nil {
			return recordErr
		}
		if recordErr := a.recordAttempt(ctx, startedAt, endedAt, SourceAttemptOutcomeBlocked, ""); recordErr != nil {
			return recordErr
		}
		return err
	}
	env, err := a.environment()
	if err != nil {
		endedAt := sourceCommandNow(SourceCommandFrameConsumer{Now: a.Now})
		if recordErr := a.recordRun(ctx, SourceRunStatusBlocked, err.Error(), startedAt, time.Time{}); recordErr != nil {
			return recordErr
		}
		if recordErr := a.recordAttempt(ctx, startedAt, endedAt, SourceAttemptOutcomeBlocked, ""); recordErr != nil {
			return recordErr
		}
		return err
	}

	cmd := exec.CommandContext(ctx, executable, a.Args...)
	cmd.Dir = strings.TrimSpace(a.WorkingDir)
	cmd.Env = env
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr connectorCommandCappedBuffer
	stderr.limit = maxConnectorCommandOutputBytes
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		endedAt := sourceCommandNow(SourceCommandFrameConsumer{Now: a.Now})
		if recordErr := a.recordRun(ctx, SourceRunStatusBlocked, err.Error(), startedAt, time.Time{}); recordErr != nil {
			return recordErr
		}
		if recordErr := a.recordAttempt(ctx, startedAt, endedAt, SourceAttemptOutcomeBlocked, ""); recordErr != nil {
			return recordErr
		}
		return err
	}
	consumer := SourceCommandFrameConsumer{
		ExpectedSourceID:  sourceID,
		ExpectedConnector: connector,
		SourceStore:       a.SourceStore,
		FailureStore:      a.FailureStore,
		IgnoreSenderIDs:   append([]string(nil), a.IgnoreSenderIDs...),
		Now:               a.Now,
	}
	consumeErr := ConsumeSourceCommandFrames(ctx, stdout, consumer, handle)
	endedAt := sourceCommandNow(SourceCommandFrameConsumer{Now: a.Now})
	if consumeErr != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		if recordErr := a.recordRun(ctx, SourceRunStatusDegraded, consumeErr.Error(), startedAt, endedAt); recordErr != nil {
			return recordErr
		}
		if recordErr := a.recordAttempt(ctx, startedAt, endedAt, SourceAttemptOutcomeFailed, ""); recordErr != nil {
			return recordErr
		}
		return consumeErr
	}
	waitErr := cmd.Wait()
	if waitErr != nil {
		detail := waitErr.Error()
		if stderr.String() != "" {
			detail = detail + ": " + SafeCLIProbeExcerpt([]byte(stderr.String()))
		}
		if err := recordSourceCommandFrameFailure(ctx, consumer, SourceCommandFrame{
			SourceID:    sourceID,
			Connector:   connector,
			AdapterRef:  adapterRef,
			EventSource: adapterRef,
			Reason:      "source_runtime_failed",
			Detail:      detail,
		}, ""); err != nil {
			return err
		}
		if recordErr := a.recordRun(ctx, SourceRunStatusDegraded, detail, startedAt, endedAt); recordErr != nil {
			return recordErr
		}
		if recordErr := a.recordAttempt(ctx, startedAt, endedAt, SourceAttemptOutcomeFailed, ""); recordErr != nil {
			return recordErr
		}
		return errors.New("source command failed")
	}
	if err := a.recordAttempt(ctx, startedAt, endedAt, SourceAttemptOutcomeStopped, ""); err != nil {
		return err
	}
	if err := a.recordRun(ctx, SourceRunStatusStopped, "", startedAt, endedAt); err != nil {
		return err
	}
	return nil
}

func ConsumeSourceCommandFrames(ctx context.Context, reader io.Reader, consumer SourceCommandFrameConsumer, handle func(ExternalEvent) error) error {
	if handle == nil {
		return errors.New("source command event handler is required")
	}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	acceptedEvents := map[string]struct{}{}
	ignoredSenderIDs := ignoreSenderIDSet(consumer.IgnoreSenderIDs)
	lineNumber := 0
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		frame, err := decodeSourceCommandFrame(line)
		if err != nil {
			if recordErr := recordSourceCommandFrameFailure(ctx, consumer, SourceCommandFrame{
				Kind:        SourceFrameKindFailed,
				Connector:   "source_command",
				EventSource: "source_command",
				Reason:      "malformed_source_frame",
				Detail:      fmt.Sprintf("line %d: %s", lineNumber, err.Error()),
			}, line); recordErr != nil {
				return recordErr
			}
			continue
		}
		switch frame.Kind {
		case SourceFrameKindReady:
			if err := consumeSourceReadyFrame(ctx, consumer, frame); err != nil {
				return err
			}
		case SourceFrameKindEvent:
			accepted, err := consumeSourceEventFrame(ctx, consumer, frame, ignoredSenderIDs, handle)
			if err != nil {
				return err
			}
			if accepted != "" {
				acceptedEvents[accepted] = struct{}{}
			}
		case SourceFrameKindCursor:
			if err := consumeSourceCursorFrame(ctx, consumer, frame, acceptedEvents); err != nil {
				return err
			}
		case SourceFrameKindFailed:
			if err := recordSourceCommandFrameFailure(ctx, consumer, frame, ""); err != nil {
				return err
			}
		case SourceFrameKindStopped:
			if err := consumeSourceStoppedFrame(ctx, consumer, frame); err != nil {
				return err
			}
		default:
			if err := recordSourceCommandFrameFailure(ctx, consumer, SourceCommandFrame{
				SourceID:    frame.SourceID,
				Connector:   frame.Connector,
				EventSource: frame.EventSource,
				Reason:      "malformed_source_frame",
				Detail:      "unsupported source frame kind",
			}, ""); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

func (a SourceCommandAdapter) resolveExecutable() (string, error) {
	executable := strings.TrimSpace(a.Executable)
	if executable == "" || invalidCommandTemplateExecutable(executable) {
		return "", errors.New("source command executable must be a direct executable")
	}
	resolved, err := resolveCommandExecutable(executable)
	if err != nil {
		return "", err
	}
	if unsafeResolvedCommandExecutable(resolved) {
		return "", fmt.Errorf("%w: %q is not a direct binary", errUnsafeCommandExecutable, resolved)
	}
	return resolved, nil
}

func (a SourceCommandAdapter) environment() ([]string, error) {
	env := a.Env
	if env == nil {
		env = connectorCommandEnvironment(os.Environ())
	}
	if err := validateConnectorCommandEnv(env); err != nil {
		return nil, err
	}
	return append([]string(nil), env...), nil
}

func (a SourceCommandAdapter) recordRun(ctx context.Context, status string, blockedReason string, startedAt time.Time, boundaryAt time.Time) error {
	if a.SourceStore == nil {
		return nil
	}
	run := SourceRun{
		SourceID:      strings.TrimSpace(a.SourceID),
		Connector:     strings.TrimSpace(a.Connector),
		AdapterRef:    strings.TrimSpace(a.AdapterRef),
		Status:        status,
		StartedAt:     startedAt,
		BlockedReason: blockedReason,
		UpdatedAt:     sourceCommandNow(SourceCommandFrameConsumer{Now: a.Now}),
	}
	if status == SourceRunStatusReady {
		run.LastReadyAt = boundaryAt
	}
	if status == SourceRunStatusStopped {
		run.StoppedAt = boundaryAt
	}
	return a.SourceStore.UpsertSourceRun(ctx, run)
}

func (a SourceCommandAdapter) recordAttempt(ctx context.Context, startedAt time.Time, endedAt time.Time, outcome string, failureRef string) error {
	if a.SourceStore == nil {
		return nil
	}
	return a.SourceStore.RecordSourceAttempt(ctx, SourceAttempt{
		AttemptID:   stableOpaqueID("source_attempt", strings.TrimSpace(a.SourceID), outcome, startedAt.Format(time.RFC3339Nano), endedAt.Format(time.RFC3339Nano), failureRef),
		SourceRunID: strings.TrimSpace(a.SourceID),
		StartedAt:   startedAt,
		EndedAt:     endedAt,
		Outcome:     outcome,
		FailureRef:  failureRef,
	})
}

func decodeSourceCommandFrame(line string) (SourceCommandFrame, error) {
	decoder := json.NewDecoder(strings.NewReader(line))
	decoder.DisallowUnknownFields()
	var frame SourceCommandFrame
	if err := decoder.Decode(&frame); err != nil {
		return SourceCommandFrame{}, fmt.Errorf("decode source frame: %w", err)
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return SourceCommandFrame{}, errors.New("source frame contained multiple JSON values")
	}
	frame.Kind = strings.TrimSpace(frame.Kind)
	frame.SourceID = strings.TrimSpace(frame.SourceID)
	frame.Connector = strings.TrimSpace(frame.Connector)
	frame.AdapterRef = strings.TrimSpace(frame.AdapterRef)
	frame.EventSource = strings.TrimSpace(frame.EventSource)
	frame.AfterEventID = strings.TrimSpace(frame.AfterEventID)
	frame.Reason = strings.TrimSpace(frame.Reason)
	frame.Detail = strings.TrimSpace(frame.Detail)
	frame.PayloadHash = strings.TrimSpace(frame.PayloadHash)
	return frame, nil
}

func consumeSourceReadyFrame(ctx context.Context, consumer SourceCommandFrameConsumer, frame SourceCommandFrame) error {
	if err := validateExpectedSourceFrame(consumer, frame); err != nil {
		return recordSourceCommandFrameFailure(ctx, consumer, sourceFrameValidationFailure(frame, "malformed_source_frame", err), "")
	}
	if err := validateSourceRunFrame(frame); err != nil {
		return recordSourceCommandFrameFailure(ctx, consumer, sourceFrameValidationFailure(frame, "source_ready_failed", err), "")
	}
	if consumer.SourceStore == nil {
		return nil
	}
	now := sourceCommandNow(consumer)
	return consumer.SourceStore.UpsertSourceRun(ctx, SourceRun{
		SourceID:    frame.SourceID,
		Connector:   frame.Connector,
		AdapterRef:  frame.AdapterRef,
		Status:      SourceRunStatusReady,
		StartedAt:   now,
		LastReadyAt: now,
		UpdatedAt:   now,
	})
}

func consumeSourceEventFrame(ctx context.Context, consumer SourceCommandFrameConsumer, frame SourceCommandFrame, ignoredSenderIDs map[string]struct{}, handle func(ExternalEvent) error) (string, error) {
	if err := validateExpectedSourceFrame(consumer, frame); err != nil {
		return "", recordSourceCommandFrameFailure(ctx, consumer, sourceFrameValidationFailure(frame, "malformed_source_frame", err), "")
	}
	if strings.TrimSpace(frame.SourceID) == "" {
		return "", recordSourceCommandFrameFailure(ctx, consumer, sourceFrameValidationFailure(frame, "malformed_source_frame", errors.New("source event frame missing source_id")), "")
	}
	if frame.Event == nil {
		return "", recordSourceCommandFrameFailure(ctx, consumer, sourceFrameValidationFailure(frame, "malformed_source_frame", errors.New("source event frame missing event")), "")
	}
	event := *frame.Event
	if strings.TrimSpace(event.SourceValidation) == "" {
		event.SourceValidation = SourceValidationUnchecked
	}
	if err := event.Validate(); err != nil {
		return "", recordSourceCommandFrameFailure(ctx, consumer, sourceFrameValidationFailure(frame, "source_payload_malformed", err), "")
	}
	if !validSourceValidationStatus(event.SourceValidation) {
		return "", recordSourceCommandFrameFailure(ctx, consumer, sourceFrameValidationFailure(frame, "source_verification_failed", errors.New("source event validation status is invalid")), "")
	}
	if event.SourceValidation == SourceValidationRejected {
		return "", recordSourceCommandFrameFailure(ctx, consumer, sourceFrameValidationFailure(frame, "source_policy_rejected", errors.New("source event is rejected")), "")
	}
	if event.SourceValidation == SourceValidationVerified {
		if frame.VerificationEvidence == nil {
			return "", recordSourceCommandFrameFailure(ctx, consumer, sourceFrameValidationFailure(frame, "source_verification_failed", errors.New("verified source event missing verification evidence")), "")
		}
		evidence := *frame.VerificationEvidence
		if strings.TrimSpace(evidence.SourceEventRef) != event.ExternalEventID {
			return "", recordSourceCommandFrameFailure(ctx, consumer, sourceFrameValidationFailure(frame, "source_verification_failed", errors.New("verification evidence event ref mismatch")), "")
		}
		if consumer.SourceStore != nil {
			if err := consumer.SourceStore.RecordSourceVerification(ctx, evidence); err != nil {
				return "", err
			}
		}
	}
	if _, ignored := ignoredSenderIDs[event.SenderRef.ExternalID]; ignored {
		return "", nil
	}
	if err := handle(event); err != nil {
		return "", err
	}
	if frame.Cursor != nil {
		cursor, err := sourceCursorFromFrame(frame.SourceID, *frame.Cursor)
		if err != nil {
			return "", recordSourceCommandFrameFailure(ctx, consumer, sourceFrameValidationFailure(frame, "source_cursor_failed", err), "")
		}
		afterEventID := strings.TrimSpace(firstNonEmpty(frame.AfterEventID, frame.Cursor.AfterEventID, event.ExternalEventID))
		if afterEventID != event.ExternalEventID {
			return "", recordSourceCommandFrameFailure(ctx, consumer, sourceFrameValidationFailure(frame, "source_cursor_failed", errors.New("source cursor after_event_id does not match accepted event")), "")
		}
		if consumer.SourceStore != nil {
			if err := consumer.SourceStore.SaveSourceCursor(ctx, cursor); err != nil {
				return "", err
			}
		}
	}
	return event.ExternalEventID, nil
}

func consumeSourceCursorFrame(ctx context.Context, consumer SourceCommandFrameConsumer, frame SourceCommandFrame, acceptedEvents map[string]struct{}) error {
	if err := validateExpectedSourceFrame(consumer, frame); err != nil {
		return recordSourceCommandFrameFailure(ctx, consumer, sourceFrameValidationFailure(frame, "source_cursor_failed", err), "")
	}
	if strings.TrimSpace(frame.SourceID) == "" || frame.Cursor == nil {
		return recordSourceCommandFrameFailure(ctx, consumer, sourceFrameValidationFailure(frame, "source_cursor_failed", errors.New("source cursor frame missing source_id or cursor")), "")
	}
	afterEventID := strings.TrimSpace(firstNonEmpty(frame.AfterEventID, frame.Cursor.AfterEventID))
	if afterEventID == "" {
		return recordSourceCommandFrameFailure(ctx, consumer, sourceFrameValidationFailure(frame, "source_cursor_failed", errors.New("source cursor frame missing after_event_id")), "")
	}
	if _, ok := acceptedEvents[afterEventID]; !ok {
		return recordSourceCommandFrameFailure(ctx, consumer, sourceFrameValidationFailure(frame, "source_cursor_failed", errors.New("source cursor referenced unaccepted event")), "")
	}
	cursor, err := sourceCursorFromFrame(frame.SourceID, *frame.Cursor)
	if err != nil {
		return recordSourceCommandFrameFailure(ctx, consumer, sourceFrameValidationFailure(frame, "source_cursor_failed", err), "")
	}
	if consumer.SourceStore == nil {
		return nil
	}
	return consumer.SourceStore.SaveSourceCursor(ctx, cursor)
}

func consumeSourceStoppedFrame(ctx context.Context, consumer SourceCommandFrameConsumer, frame SourceCommandFrame) error {
	if err := validateExpectedSourceFrame(consumer, frame); err != nil {
		return recordSourceCommandFrameFailure(ctx, consumer, sourceFrameValidationFailure(frame, "malformed_source_frame", err), "")
	}
	if strings.TrimSpace(frame.SourceID) == "" {
		return recordSourceCommandFrameFailure(ctx, consumer, sourceFrameValidationFailure(frame, "malformed_source_frame", errors.New("source stopped frame missing source_id")), "")
	}
	if consumer.SourceStore == nil {
		return nil
	}
	now := sourceCommandNow(consumer)
	run := SourceRun{
		SourceID:      frame.SourceID,
		Connector:     strings.TrimSpace(frame.Connector),
		AdapterRef:    strings.TrimSpace(frame.AdapterRef),
		Status:        SourceRunStatusStopped,
		StartedAt:     now,
		StoppedAt:     now,
		BlockedReason: safeSourceFailureDiagnostic(frame.Detail),
		UpdatedAt:     now,
	}
	existingRuns, err := consumer.SourceStore.ListSourceRuns(ctx)
	if err != nil {
		return err
	}
	for _, existing := range existingRuns {
		if existing.SourceID == frame.SourceID {
			if run.Connector == "" {
				run.Connector = existing.Connector
			}
			if run.AdapterRef == "" {
				run.AdapterRef = existing.AdapterRef
			}
			run.StartedAt = existing.StartedAt
			break
		}
	}
	if run.Connector == "" {
		run.Connector = "source_command"
	}
	if run.AdapterRef == "" {
		run.AdapterRef = "source_command"
	}
	return consumer.SourceStore.UpsertSourceRun(ctx, run)
}

func validateExpectedSourceFrame(consumer SourceCommandFrameConsumer, frame SourceCommandFrame) error {
	expectedSourceID := strings.TrimSpace(consumer.ExpectedSourceID)
	expectedConnector := strings.TrimSpace(consumer.ExpectedConnector)
	if expectedSourceID != "" && strings.TrimSpace(frame.SourceID) != "" && strings.TrimSpace(frame.SourceID) != expectedSourceID {
		return errors.New("source frame source_id mismatch")
	}
	if expectedConnector != "" && strings.TrimSpace(frame.Connector) != "" && strings.TrimSpace(frame.Connector) != expectedConnector {
		return errors.New("source frame connector mismatch")
	}
	return nil
}

func validSourceValidationStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case SourceValidationVerified, SourceValidationUnchecked, SourceValidationRejected:
		return true
	default:
		return false
	}
}

func validateSourceRunFrame(frame SourceCommandFrame) error {
	switch {
	case strings.TrimSpace(frame.SourceID) == "":
		return errors.New("source frame missing source_id")
	case strings.TrimSpace(frame.Connector) == "":
		return errors.New("source frame missing connector")
	case strings.TrimSpace(frame.AdapterRef) == "":
		return errors.New("source frame missing adapter_ref")
	default:
		return nil
	}
}

func sourceCursorFromFrame(sourceID string, frame SourceCommandCursorFrame) (SourceCursor, error) {
	cursor := SourceCursor{
		SourceID:    firstNonEmpty(frame.SourceID, sourceID),
		CursorKind:  strings.TrimSpace(frame.CursorKind),
		CursorValue: strings.TrimSpace(frame.CursorValue),
		WatermarkAt: frame.WatermarkAt,
		UpdatedAt:   frame.UpdatedAt,
	}
	if strings.TrimSpace(cursor.CursorKind) == "" {
		cursor.CursorKind = SourceCursorKindExternalEventID
	}
	if strings.TrimSpace(cursor.SourceID) != strings.TrimSpace(sourceID) {
		return SourceCursor{}, errors.New("source cursor source_id mismatch")
	}
	return normalizeSourceCursor(cursor)
}

func sourceFrameValidationFailure(frame SourceCommandFrame, reason string, cause error) SourceCommandFrame {
	frame.Kind = SourceFrameKindFailed
	frame.Reason = reason
	if cause != nil {
		frame.Detail = cause.Error()
	}
	return frame
}

func recordSourceCommandFrameFailure(ctx context.Context, consumer SourceCommandFrameConsumer, frame SourceCommandFrame, rawLine string) error {
	if consumer.FailureStore == nil {
		return nil
	}
	reason := strings.TrimSpace(frame.Reason)
	if reason == "" {
		reason = "source_runtime_failed"
	}
	connector := firstNonEmpty(frame.Connector, "source_command")
	eventSource := firstNonEmpty(frame.EventSource, "source_command")
	detail := safeSourceFailureDiagnostic(firstNonEmpty(frame.Detail, reason))
	record := SourceFailureRecord{
		Connector:         connector,
		EventSource:       eventSource,
		SourceRunRef:      strings.TrimSpace(frame.SourceID),
		Reason:            reason,
		Detail:            detail,
		DiagnosticExcerpt: detail,
		PayloadHash:       strings.TrimSpace(frame.PayloadHash),
		PayloadSizeBytes:  frame.PayloadSizeBytes,
		SourceValidation:  SourceValidationRejected,
	}
	if rawLine != "" {
		record.PayloadHash = sourcePayloadHash(rawLine)
		record.PayloadSizeBytes = len(rawLine)
		record.DiagnosticExcerpt = safeSourceFailureDiagnostic(fmt.Sprintf("%s; source_bytes=%d", detail, len(rawLine)))
	}
	return consumer.FailureStore.RecordSourceFailure(ctx, record)
}

func sourceCommandNow(consumer SourceCommandFrameConsumer) time.Time {
	if consumer.Now != nil {
		return consumer.Now().UTC()
	}
	return time.Now().UTC()
}
