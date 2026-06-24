package connectorruntime

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"genesis/internal/testsupport"
)

func TestFileSourceSupervisorStorePersistsRunAttemptAndCursor(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(testsupport.ProjectTempDir(t, "source-supervisor-store"), "source-supervisor.json")
	store, err := NewFileSourceSupervisorStore(path)
	if err != nil {
		t.Fatalf("NewFileSourceSupervisorStore returned error: %v", err)
	}
	now := time.Date(2026, 6, 24, 14, 0, 0, 0, time.UTC)
	run := SourceRun{
		SourceID:    "source_feishu_events",
		Connector:   "feishu",
		AdapterRef:  "lark-cli:event.consume",
		Status:      SourceRunStatusStarting,
		StartedAt:   now,
		LastReadyAt: now,
	}
	if err := store.UpsertSourceRun(ctx, run); err != nil {
		t.Fatalf("UpsertSourceRun returned error: %v", err)
	}
	attempt := SourceAttempt{
		AttemptID:   "attempt_1",
		SourceRunID: run.SourceID,
		StartedAt:   now,
		EndedAt:     now.Add(time.Second),
		Outcome:     SourceAttemptOutcomeFailed,
		FailureRef:  "source_failure_1",
	}
	if err := store.RecordSourceAttempt(ctx, attempt); err != nil {
		t.Fatalf("RecordSourceAttempt returned error: %v", err)
	}
	cursor := SourceCursor{
		SourceID:    run.SourceID,
		CursorKind:  SourceCursorKindExternalEventID,
		CursorValue: "evt_123",
		WatermarkAt: now.Add(2 * time.Second),
		UpdatedAt:   now.Add(3 * time.Second),
	}
	if err := store.SaveSourceCursor(ctx, cursor); err != nil {
		t.Fatalf("SaveSourceCursor returned error: %v", err)
	}

	reopened, err := NewFileSourceSupervisorStore(path)
	if err != nil {
		t.Fatalf("reopen NewFileSourceSupervisorStore returned error: %v", err)
	}
	runs, err := reopened.ListSourceRuns(ctx)
	if err != nil {
		t.Fatalf("ListSourceRuns returned error: %v", err)
	}
	if len(runs) != 1 || runs[0].SourceID != run.SourceID || runs[0].Status != SourceRunStatusStarting {
		t.Fatalf("runs = %+v, want persisted source run", runs)
	}
	attempts, err := reopened.ListSourceAttempts(ctx, run.SourceID)
	if err != nil {
		t.Fatalf("ListSourceAttempts returned error: %v", err)
	}
	if len(attempts) != 1 || attempts[0].Outcome != SourceAttemptOutcomeFailed || attempts[0].FailureRef != "source_failure_1" {
		t.Fatalf("attempts = %+v, want persisted failed attempt", attempts)
	}
	gotCursor, ok, err := reopened.GetSourceCursor(ctx, run.SourceID, SourceCursorKindExternalEventID)
	if err != nil {
		t.Fatalf("GetSourceCursor returned error: %v", err)
	}
	if !ok || gotCursor.CursorValue != "evt_123" || !gotCursor.WatermarkAt.Equal(cursor.WatermarkAt) {
		t.Fatalf("cursor = %+v ok=%v, want persisted connector-local cursor", gotCursor, ok)
	}
}

func TestFileSourceSupervisorStoreRejectsModelVisibleVerifiedWithoutEvidence(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(testsupport.ProjectTempDir(t, "source-supervisor-validation"), "source-supervisor.json")
	store, err := NewFileSourceSupervisorStore(path)
	if err != nil {
		t.Fatalf("NewFileSourceSupervisorStore returned error: %v", err)
	}
	err = store.RecordSourceVerification(ctx, SourceVerificationEvidence{
		SourceEventRef:   "evt_without_evidence",
		ValidationStatus: SourceValidationVerified,
		CheckedAt:        time.Date(2026, 6, 24, 14, 30, 0, 0, time.UTC),
		AdapterRef:       "lark-cli:event.consume",
	})
	if err == nil {
		t.Fatal("RecordSourceVerification should reject verified status without evidence kind and ref")
	}
}

func TestFileSourceSupervisorStoreKeepsRunStartedAtAcrossStatusUpdates(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(testsupport.ProjectTempDir(t, "source-supervisor-run-lifecycle"), "source-supervisor.json")
	store, err := NewFileSourceSupervisorStore(path)
	if err != nil {
		t.Fatalf("NewFileSourceSupervisorStore returned error: %v", err)
	}
	startedAt := time.Date(2026, 6, 24, 15, 0, 0, 0, time.UTC)
	readyAt := startedAt.Add(2 * time.Second)
	run := SourceRun{
		SourceID:   "source_feishu_events",
		Connector:  "feishu",
		AdapterRef: "lark-cli:event.consume",
		Status:     SourceRunStatusStarting,
		StartedAt:  startedAt,
		UpdatedAt:  startedAt,
	}
	if err := store.UpsertSourceRun(ctx, run); err != nil {
		t.Fatalf("UpsertSourceRun starting returned error: %v", err)
	}
	run.Status = SourceRunStatusReady
	run.StartedAt = readyAt
	run.LastReadyAt = readyAt
	run.UpdatedAt = readyAt
	if err := store.UpsertSourceRun(ctx, run); err != nil {
		t.Fatalf("UpsertSourceRun ready returned error: %v", err)
	}

	runs, err := store.ListSourceRuns(ctx)
	if err != nil {
		t.Fatalf("ListSourceRuns returned error: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("runs = %+v, want one run", runs)
	}
	if !runs[0].StartedAt.Equal(startedAt) {
		t.Fatalf("started_at = %s, want original source run start %s", runs[0].StartedAt, startedAt)
	}
	if !runs[0].LastReadyAt.Equal(readyAt) {
		t.Fatalf("last_ready_at = %s, want %s", runs[0].LastReadyAt, readyAt)
	}
}
