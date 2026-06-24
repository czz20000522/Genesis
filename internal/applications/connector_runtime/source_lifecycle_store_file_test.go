package connectorruntime

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"genesis/internal/testsupport"
)

func TestFileSourceLifecycleStorePersistsRunAttemptAndCursor(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(testsupport.ProjectTempDir(t, "source-lifecycle-store"), "source-lifecycle.json")
	store, err := NewFileSourceLifecycleStore(path)
	if err != nil {
		t.Fatalf("NewFileSourceLifecycleStore returned error: %v", err)
	}
	now := time.Date(2026, 6, 24, 14, 0, 0, 0, time.UTC)
	run := SourceRun{
		SourceID:    "source_feishu_events",
		Connector:   "feishu",
		AdapterRef:  "feishu-source-adapter",
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

	reopened, err := NewFileSourceLifecycleStore(path)
	if err != nil {
		t.Fatalf("reopen NewFileSourceLifecycleStore returned error: %v", err)
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

func TestFileSourceLifecycleStoreRejectsModelVisibleVerifiedWithoutEvidence(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(testsupport.ProjectTempDir(t, "source-lifecycle-validation"), "source-lifecycle.json")
	store, err := NewFileSourceLifecycleStore(path)
	if err != nil {
		t.Fatalf("NewFileSourceLifecycleStore returned error: %v", err)
	}
	err = store.RecordSourceVerification(ctx, SourceVerificationEvidence{
		SourceEventRef:   "evt_without_evidence",
		SourceID:         "source_feishu_events",
		Connector:        "feishu",
		ValidationStatus: SourceValidationVerified,
		CheckedAt:        time.Date(2026, 6, 24, 14, 30, 0, 0, time.UTC),
		AdapterRef:       "feishu-source-adapter",
	})
	if err == nil {
		t.Fatal("RecordSourceVerification should reject verified status without evidence kind and ref")
	}
}

func TestFileSourceLifecycleStoreRejectsUnapprovedVerifiedEvidenceKind(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(testsupport.ProjectTempDir(t, "source-lifecycle-evidence-kind"), "source-lifecycle.json")
	store, err := NewFileSourceLifecycleStore(path)
	if err != nil {
		t.Fatalf("NewFileSourceLifecycleStore returned error: %v", err)
	}
	err = store.RecordSourceVerification(ctx, SourceVerificationEvidence{
		SourceEventRef:   "evt_bad_kind",
		SourceID:         "source_feishu_events",
		Connector:        "feishu",
		ValidationStatus: SourceValidationVerified,
		EvidenceKind:     "unknown_adapter_claim",
		EvidenceRef:      "evidence_1",
		CheckedAt:        time.Date(2026, 6, 24, 14, 35, 0, 0, time.UTC),
		AdapterRef:       "feishu-source-adapter",
	})
	if err == nil {
		t.Fatal("RecordSourceVerification should reject unapproved evidence kind")
	}
}

func TestFileSourceLifecycleStoreKeepsRunStartedAtAcrossStatusUpdates(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(testsupport.ProjectTempDir(t, "source-lifecycle-run-lifecycle"), "source-lifecycle.json")
	store, err := NewFileSourceLifecycleStore(path)
	if err != nil {
		t.Fatalf("NewFileSourceLifecycleStore returned error: %v", err)
	}
	startedAt := time.Date(2026, 6, 24, 15, 0, 0, 0, time.UTC)
	readyAt := startedAt.Add(2 * time.Second)
	run := SourceRun{
		SourceID:   "source_feishu_events",
		Connector:  "feishu",
		AdapterRef: "feishu-source-adapter",
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

func TestFileSourceLifecycleStoreRejectsInvalidReadinessReasonCode(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(testsupport.ProjectTempDir(t, "source-lifecycle-reason-code"), "source-lifecycle.json")
	store, err := NewFileSourceLifecycleStore(path)
	if err != nil {
		t.Fatalf("NewFileSourceLifecycleStore returned error: %v", err)
	}
	err = store.UpsertSourceRun(ctx, SourceRun{
		SourceID:          "source_feishu_events",
		Connector:         "feishu",
		AdapterRef:        "feishu-source-adapter",
		Status:            SourceRunStatusBlocked,
		BlockedReasonCode: "transient_text_only_reason",
		BlockedReason:     "operator detail",
	})
	if err == nil {
		t.Fatal("UpsertSourceRun should reject unknown readiness reason code")
	}
}

func TestFileSourceLifecycleStoreRecordsOperatorControls(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(testsupport.ProjectTempDir(t, "source-lifecycle-operator-controls"), "source-lifecycle.json")
	store, err := NewFileSourceLifecycleStore(path)
	if err != nil {
		t.Fatalf("NewFileSourceLifecycleStore returned error: %v", err)
	}
	startedAt := time.Date(2026, 6, 24, 16, 0, 0, 0, time.UTC)
	if err := store.UpsertSourceRun(ctx, SourceRun{
		SourceID:          "source_feishu_events",
		Connector:         "feishu",
		AdapterRef:        "feishu-source-adapter",
		Status:            SourceRunStatusBlocked,
		StartedAt:         startedAt,
		BlockedReasonCode: SourceReadinessReasonMissingProfile,
		BlockedReason:     "profile missing",
		UpdatedAt:         startedAt,
	}); err != nil {
		t.Fatalf("UpsertSourceRun returned error: %v", err)
	}

	clearedAt := startedAt.Add(time.Minute)
	run, action, err := store.ClearBlockedSourceRun(ctx, "source_feishu_events", "operator_profile_fixed", clearedAt)
	if err != nil {
		t.Fatalf("ClearBlockedSourceRun returned error: %v", err)
	}
	if run.Status != SourceRunStatusStopped || run.BlockedReasonCode != "" || run.BlockedReason != "" {
		t.Fatalf("cleared run = %+v, want stopped with blocked reason cleared", run)
	}
	if action.Action != SourceOperatorActionClearBlocked || action.PreviousStatus != SourceRunStatusBlocked || action.NewStatus != SourceRunStatusStopped {
		t.Fatalf("clear action = %+v", action)
	}

	restartAt := startedAt.Add(2 * time.Minute)
	restartAction, err := store.RequestSourceRestart(ctx, "source_feishu_events", "operator_requested_restart", restartAt)
	if err != nil {
		t.Fatalf("RequestSourceRestart returned error: %v", err)
	}
	if restartAction.Action != SourceOperatorActionRequestRestart || restartAction.PreviousStatus != SourceRunStatusStopped || restartAction.NewStatus != SourceRunStatusStopped {
		t.Fatalf("restart action = %+v, want recorded intent without run status mutation", restartAction)
	}
	runs, err := store.ListSourceRuns(ctx)
	if err != nil {
		t.Fatalf("ListSourceRuns returned error: %v", err)
	}
	if len(runs) != 1 || runs[0].Status != SourceRunStatusStopped {
		t.Fatalf("runs = %+v, want restart request to leave source stopped", runs)
	}

	if _, _, err := store.ResetSourceCursor(ctx, "source_feishu_events", SourceCursorKindExternalEventID, "evt_replay_from", "operator_replay_cursor", false, startedAt.Add(3*time.Minute)); err == nil {
		t.Fatal("ResetSourceCursor should require accepted duplicate-processing risk")
	}
	cursor, cursorAction, err := store.ResetSourceCursor(ctx, "source_feishu_events", SourceCursorKindExternalEventID, "evt_replay_from", "operator_replay_cursor", true, startedAt.Add(4*time.Minute))
	if err != nil {
		t.Fatalf("ResetSourceCursor returned error: %v", err)
	}
	if cursor.CursorValue != "evt_replay_from" {
		t.Fatalf("cursor = %+v, want reset cursor value", cursor)
	}
	if cursorAction.Action != SourceOperatorActionResetCursor || !cursorAction.AcceptedDuplicateRisk {
		t.Fatalf("cursor action = %+v", cursorAction)
	}

	reopened, err := NewFileSourceLifecycleStore(path)
	if err != nil {
		t.Fatalf("reopen NewFileSourceLifecycleStore returned error: %v", err)
	}
	actions, err := reopened.ListSourceOperatorActions(ctx, "source_feishu_events")
	if err != nil {
		t.Fatalf("ListSourceOperatorActions returned error: %v", err)
	}
	if len(actions) != 3 {
		t.Fatalf("operator action count = %d, want 3: %+v", len(actions), actions)
	}
	if actions[0].Action != SourceOperatorActionClearBlocked || actions[1].Action != SourceOperatorActionRequestRestart || actions[2].Action != SourceOperatorActionResetCursor {
		t.Fatalf("actions = %+v, want clear, restart, reset order", actions)
	}
}
