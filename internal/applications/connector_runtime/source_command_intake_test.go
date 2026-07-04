package connectorruntime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"genesis/internal/testsupport"
)

func TestSourceCommandIntakeRetriesRuntimeFailureAndConsumesEvent(t *testing.T) {
	ctx := context.Background()
	sourceStore, failureStore := newSourceCommandTestStores(t, "source-command-intake-retry")
	dir := testsupport.ProjectTempDir(t, "source-command-intake-retry-attempts")
	attemptFile := filepath.Join(dir, "attempts.txt")
	env := append(connectorCommandEnvironment(os.Environ()),
		"GENESIS_SOURCE_COMMAND_HELPER=fail-once-then-ready-event-stopped",
		"GENESIS_SOURCE_COMMAND_ATTEMPT_FILE="+attemptFile,
		"GENESIS_SOURCE_COMMAND_SOURCE_ID=source_test_chat",
		"GENESIS_SOURCE_COMMAND_CONNECTOR=test_connector",
		"GENESIS_SOURCE_COMMAND_ADAPTER_REF=test-source-adapter",
	)
	intake := SourceCommandIntake{
		Adapter: SourceCommandAdapter{
			Executable:   os.Args[0],
			Args:         []string{"-test.run=TestSourceCommandAdapterHelper"},
			Env:          env,
			SourceID:     "source_test_chat",
			Connector:    "test_connector",
			AdapterRef:   "test-source-adapter",
			SourceStore:  sourceStore,
			FailureStore: failureStore,
			IgnoreSenderIDs: []string{
				"bot_self",
			},
		},
		Retry: SourceCommandRetryPolicy{
			MaxAttempts: 2,
			Backoff:     25 * time.Millisecond,
		},
		Sleep: func(ctx context.Context, d time.Duration) error {
			if d != 25*time.Millisecond {
				t.Fatalf("backoff = %s, want 25ms", d)
			}
			return ctx.Err()
		},
	}
	var handled []ExternalEvent
	if err := intake.Run(ctx, func(event ExternalEvent) error {
		handled = append(handled, event)
		return nil
	}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(handled) != 1 || handled[0].ExternalEventID != "evt_1" {
		t.Fatalf("handled = %+v, want event from successful retry", handled)
	}
	attempts, err := sourceStore.ListSourceAttempts(ctx, "source_test_chat")
	if err != nil {
		t.Fatalf("ListSourceAttempts returned error: %v", err)
	}
	if len(attempts) != 2 || attempts[0].Outcome != SourceAttemptOutcomeFailed || attempts[1].Outcome != SourceAttemptOutcomeStopped {
		t.Fatalf("attempts = %+v, want failed then stopped attempts", attempts)
	}
	failures, err := failureStore.ListSourceFailures(ctx)
	if err != nil {
		t.Fatalf("ListSourceFailures returned error: %v", err)
	}
	if len(failures) != 1 || failures[0].Reason != "source_runtime_failed" {
		t.Fatalf("failures = %+v, want one runtime failure record", failures)
	}
}

func TestSourceCommandBackoffDelayUsesBoundedExponentialSchedule(t *testing.T) {
	policy := SourceCommandRetryPolicy{Backoff: 100 * time.Millisecond}
	if got := sourceCommandBackoffDelay(policy, 1); got != 100*time.Millisecond {
		t.Fatalf("attempt 1 delay = %s, want 100ms", got)
	}
	if got := sourceCommandBackoffDelay(policy, 2); got != 200*time.Millisecond {
		t.Fatalf("attempt 2 delay = %s, want 200ms", got)
	}
	if got := sourceCommandBackoffDelay(SourceCommandRetryPolicy{Backoff: 20 * time.Second}, 2); got != maxSourceCommandBackoff {
		t.Fatalf("capped delay = %s, want %s", got, maxSourceCommandBackoff)
	}
}

func TestSourceCommandIntakeDoesNotRetryBlockedSource(t *testing.T) {
	ctx := context.Background()
	sourceStore, failureStore := newSourceCommandTestStores(t, "source-command-intake-blocked")
	intake := SourceCommandIntake{
		Adapter: SourceCommandAdapter{
			Executable:   "",
			SourceID:     "source_test_chat",
			Connector:    "test_connector",
			AdapterRef:   "test-source-adapter",
			SourceStore:  sourceStore,
			FailureStore: failureStore,
		},
		Retry: SourceCommandRetryPolicy{
			MaxAttempts: 3,
			Backoff:     25 * time.Millisecond,
		},
		Sleep: func(context.Context, time.Duration) error {
			t.Fatal("blocked source must not enter retry backoff")
			return nil
		},
	}
	err := intake.Run(ctx, func(event ExternalEvent) error {
		t.Fatalf("blocked source must not emit event: %+v", event)
		return nil
	})
	if !errors.Is(err, ErrSourceCommandBlocked) {
		t.Fatalf("Run error = %v, want ErrSourceCommandBlocked", err)
	}
	attempts, err := sourceStore.ListSourceAttempts(ctx, "source_test_chat")
	if err != nil {
		t.Fatalf("ListSourceAttempts returned error: %v", err)
	}
	if len(attempts) != 1 || attempts[0].Outcome != SourceAttemptOutcomeBlocked {
		t.Fatalf("attempts = %+v, want one blocked attempt without retry", attempts)
	}
}

func TestSourceCommandIntakeDoesNotRetryStartFailure(t *testing.T) {
	ctx := context.Background()
	sourceStore, failureStore := newSourceCommandTestStores(t, "source-command-intake-start-failure")
	intake := SourceCommandIntake{
		Adapter: SourceCommandAdapter{
			Executable:   os.Args[0],
			WorkingDir:   filepath.Join(testsupport.ProjectTempDir(t, "source-command-start-failure"), "missing-dir"),
			SourceID:     "source_test_chat",
			Connector:    "test_connector",
			AdapterRef:   "test-source-adapter",
			SourceStore:  sourceStore,
			FailureStore: failureStore,
		},
		Retry: SourceCommandRetryPolicy{
			MaxAttempts: 3,
			Backoff:     25 * time.Millisecond,
		},
		Sleep: func(context.Context, time.Duration) error {
			t.Fatal("start failure must not enter retry backoff")
			return nil
		},
	}
	err := intake.Run(ctx, func(event ExternalEvent) error {
		t.Fatalf("start failure must not emit event: %+v", event)
		return nil
	})
	if !errors.Is(err, ErrSourceCommandBlocked) {
		t.Fatalf("Run error = %v, want ErrSourceCommandBlocked", err)
	}
	attempts, err := sourceStore.ListSourceAttempts(ctx, "source_test_chat")
	if err != nil {
		t.Fatalf("ListSourceAttempts returned error: %v", err)
	}
	if len(attempts) != 1 || attempts[0].Outcome != SourceAttemptOutcomeBlocked {
		t.Fatalf("attempts = %+v, want one blocked start attempt without retry", attempts)
	}
}

func TestSourceCommandIntakeDoesNotRetryHandlerFailure(t *testing.T) {
	ctx := context.Background()
	sourceStore, failureStore := newSourceCommandTestStores(t, "source-command-intake-handler-failure")
	env := append(connectorCommandEnvironment(os.Environ()),
		"GENESIS_SOURCE_COMMAND_HELPER=ready-event-stopped",
		"GENESIS_SOURCE_COMMAND_SOURCE_ID=source_test_chat",
		"GENESIS_SOURCE_COMMAND_CONNECTOR=test_connector",
		"GENESIS_SOURCE_COMMAND_ADAPTER_REF=test-source-adapter",
	)
	intake := SourceCommandIntake{
		Adapter: SourceCommandAdapter{
			Executable:      os.Args[0],
			Args:            []string{"-test.run=TestSourceCommandAdapterHelper"},
			Env:             env,
			SourceID:        "source_test_chat",
			Connector:       "test_connector",
			AdapterRef:      "test-source-adapter",
			SourceStore:     sourceStore,
			FailureStore:    failureStore,
			IgnoreSenderIDs: []string{"bot_self"},
		},
		Retry: SourceCommandRetryPolicy{
			MaxAttempts: 2,
			Backoff:     25 * time.Millisecond,
		},
		Sleep: func(context.Context, time.Duration) error {
			t.Fatal("handler failure must not restart the source process")
			return nil
		},
	}
	err := intake.Run(ctx, func(ExternalEvent) error {
		return errors.New("kernel submission unavailable")
	})
	if err == nil || !strings.Contains(err.Error(), "kernel submission unavailable") {
		t.Fatalf("Run error = %v, want original handler failure", err)
	}
	if errors.Is(err, ErrSourceCommandRuntimeFailed) {
		t.Fatalf("handler failure must not be classified as source runtime failure: %v", err)
	}
	attempts, err := sourceStore.ListSourceAttempts(ctx, "source_test_chat")
	if err != nil {
		t.Fatalf("ListSourceAttempts returned error: %v", err)
	}
	if len(attempts) != 1 || attempts[0].Outcome != SourceAttemptOutcomeFailed {
		t.Fatalf("attempts = %+v, want one failed attempt without retry", attempts)
	}
	runs, err := sourceStore.ListSourceRuns(ctx)
	if err != nil {
		t.Fatalf("ListSourceRuns returned error: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("runs = %+v, want one source run", runs)
	}
	if runs[0].Status == SourceRunStatusDegraded || strings.Contains(runs[0].BlockedReason, "kernel submission unavailable") {
		t.Fatalf("handler failure leaked into source run state: %+v", runs[0])
	}
}
