package codeintelligenceruntime

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestReadinessBlocksWorktreeMismatchAndSkipsQuery(t *testing.T) {
	root := testProjectRoot(t)
	adapter := &fakeAdapter{
		readiness: AdapterReadiness{
			Adapter:             "codegraph",
			ExecutableAvailable: true,
			CachePresent:        true,
			ProjectPath:         filepath.Dir(root),
			IndexPath:           filepath.Join(filepath.Dir(root), ".codegraph"),
			Telemetry:           TelemetryDisabled,
			WorktreeMismatch: &WorktreeMismatch{
				WorktreeRoot:       root,
				IndexedProjectPath: filepath.Dir(root),
			},
		},
		queryResult: AdapterQueryResult{Items: []CodeQueryItem{{Kind: "symbol", Text: "must not run"}}},
	}
	runtime := NewRuntime(adapter)
	project := CodeProjectRef{ProjectRef: "proj_worktree", AdmittedRoot: root}

	readiness, err := runtime.Probe(context.Background(), project)
	if err != nil {
		t.Fatalf("Probe returned error: %v", err)
	}
	if readiness.Status != ReadinessBlocked || readiness.BlockedReason != "worktree_mismatch" || readiness.Freshness != FreshnessWorktreeMismatch {
		t.Fatalf("readiness = %+v, want blocked worktree mismatch", readiness)
	}
	result, err := runtime.Query(context.Background(), project, CodeQuery{
		QueryKind: QueryKindExplore,
		QueryText: "approval crash window",
	})
	if !errors.Is(err, ErrCodeReadinessBlocked) {
		t.Fatalf("Query error = %v, want ErrCodeReadinessBlocked", err)
	}
	if result.Status != QueryStatusBlocked || adapter.queryCalls != 0 {
		t.Fatalf("result = %+v queryCalls=%d, want blocked without adapter query", result, adapter.queryCalls)
	}
}

func TestReadinessClassifiesPendingChangesAsStaleAndRequiresExplicitDegradedAcceptance(t *testing.T) {
	root := testProjectRoot(t)
	adapter := &fakeAdapter{
		readiness: AdapterReadiness{
			Adapter:             "codegraph",
			ExecutableAvailable: true,
			CachePresent:        true,
			ProjectPath:         root,
			IndexPath:           filepath.Join(root, ".codegraph"),
			Telemetry:           TelemetryDisabled,
			PendingChanges:      PendingChanges{Modified: 2},
		},
		queryResult: AdapterQueryResult{Items: []CodeQueryItem{{Kind: "symbol", Text: "EnsureApprovedApprovalEffect"}}},
	}
	runtime := NewRuntime(adapter)
	project := CodeProjectRef{ProjectRef: "proj_stale", AdmittedRoot: root}

	readiness, err := runtime.Probe(context.Background(), project)
	if err != nil {
		t.Fatalf("Probe returned error: %v", err)
	}
	if readiness.Status != ReadinessCacheStale || readiness.BlockedReason != "pending_changes" || readiness.Freshness != FreshnessPendingChanges {
		t.Fatalf("readiness = %+v, want stale pending changes", readiness)
	}
	if _, err := runtime.Query(context.Background(), project, CodeQuery{QueryKind: QueryKindExplore, QueryText: "approval"}); !errors.Is(err, ErrCodeReadinessBlocked) {
		t.Fatalf("Query without degraded acceptance error = %v, want ErrCodeReadinessBlocked", err)
	}
	result, err := runtime.Query(context.Background(), project, CodeQuery{
		QueryKind:               QueryKindExplore,
		QueryText:               "approval",
		AcceptDegradedFreshness: true,
	})
	if err != nil {
		t.Fatalf("Query with degraded acceptance returned error: %v", err)
	}
	if result.Status != QueryStatusCompleted || result.Freshness != FreshnessPendingChanges || len(result.Items) != 1 {
		t.Fatalf("result = %+v, want stale-but-accepted query result", result)
	}
	if result.Items[0].EvidenceRole != EvidenceRoleHint || result.Items[0].Verification != VerificationAdvisory {
		t.Fatalf("item = %+v, want advisory hint", result.Items[0])
	}
}

func TestReadinessBlocksTelemetryEnabledUnlessExplicitlyAccepted(t *testing.T) {
	root := testProjectRoot(t)
	adapter := &fakeAdapter{
		readiness: AdapterReadiness{
			Adapter:             "codegraph",
			ExecutableAvailable: true,
			CachePresent:        true,
			ProjectPath:         root,
			IndexPath:           filepath.Join(root, ".codegraph"),
			Telemetry:           TelemetryEnabled,
		},
	}
	runtime := NewRuntime(adapter)
	project := CodeProjectRef{ProjectRef: "proj_telemetry", AdmittedRoot: root}

	readiness, err := runtime.Probe(context.Background(), project)
	if err != nil {
		t.Fatalf("Probe returned error: %v", err)
	}
	if readiness.Status != ReadinessBlocked || readiness.BlockedReason != "telemetry_enabled" {
		t.Fatalf("readiness = %+v, want telemetry block", readiness)
	}

	runtime = NewRuntime(adapter, WithTelemetryAccepted(true))
	readiness, err = runtime.Probe(context.Background(), project)
	if err != nil {
		t.Fatalf("Probe with accepted telemetry returned error: %v", err)
	}
	if readiness.Status != ReadinessReady {
		t.Fatalf("readiness with accepted telemetry = %+v, want ready", readiness)
	}
}

func TestAffectedTestsAreProjectedAsHintsNotVerificationProof(t *testing.T) {
	root := testProjectRoot(t)
	adapter := &fakeAdapter{
		readiness: AdapterReadiness{
			Adapter:             "codegraph",
			ExecutableAvailable: true,
			CachePresent:        true,
			ProjectPath:         root,
			IndexPath:           filepath.Join(root, ".codegraph"),
			Telemetry:           TelemetryDisabled,
		},
		queryResult: AdapterQueryResult{
			Items: []CodeQueryItem{
				{Kind: QueryItemAffectedTest, Ref: "internal/kernel/approval_owner_test.go", Text: "TestApprovalApprovedCrashWindowReplayExecutesFrozenEffectOnce"},
			},
			DiagnosticReason: "adapter_reported_affected_tests",
		},
	}
	runtime := NewRuntime(adapter)
	project := CodeProjectRef{ProjectRef: "proj_affected", AdmittedRoot: root}

	result, err := runtime.Query(context.Background(), project, CodeQuery{
		QueryKind:  QueryKindAffectedTests,
		TargetPath: "internal/kernel/approval.go",
	})
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items = %+v, want one affected-test hint", result.Items)
	}
	item := result.Items[0]
	if item.EvidenceRole != EvidenceRoleHint || item.Verification != VerificationAdvisory || item.Proof {
		t.Fatalf("affected-test item = %+v, want advisory hint without proof", item)
	}
	if result.DiagnosticReason != "adapter_reported_affected_tests" {
		t.Fatalf("diagnostic reason = %q, want adapter_reported_affected_tests", result.DiagnosticReason)
	}
}

func TestQueryResultsAreBoundedByRequestedLimit(t *testing.T) {
	root := testProjectRoot(t)
	adapter := &fakeAdapter{
		readiness: AdapterReadiness{
			Adapter:             "codegraph",
			ExecutableAvailable: true,
			CachePresent:        true,
			ProjectPath:         root,
			IndexPath:           filepath.Join(root, ".codegraph"),
			Telemetry:           TelemetryDisabled,
		},
		queryResult: AdapterQueryResult{
			Items: []CodeQueryItem{
				{Kind: "symbol", Text: "one"},
				{Kind: "symbol", Text: "two"},
				{Kind: "symbol", Text: "three"},
			},
		},
	}
	runtime := NewRuntime(adapter)
	project := CodeProjectRef{ProjectRef: "proj_bounded", AdmittedRoot: root}

	result, err := runtime.Query(context.Background(), project, CodeQuery{
		QueryKind:   QueryKindExplore,
		QueryText:   "approval",
		ResultLimit: 2,
	})
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if len(result.Items) != 2 || !result.Truncated {
		t.Fatalf("result = %+v, want two bounded items and truncation marker", result)
	}
}

func testProjectRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(t.TempDir())
	if err != nil {
		t.Fatalf("abs temp dir: %v", err)
	}
	return root
}

type fakeAdapter struct {
	readiness   AdapterReadiness
	queryResult AdapterQueryResult
	queryCalls  int
}

func (f *fakeAdapter) Readiness(_ context.Context, _ CodeProjectRef) (AdapterReadiness, error) {
	return f.readiness, nil
}

func (f *fakeAdapter) Query(_ context.Context, _ CodeProjectRef, _ CodeQuery) (AdapterQueryResult, error) {
	f.queryCalls++
	return f.queryResult, nil
}
