package codeintelligenceruntime

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"genesis/internal/testsupport"
)

func TestCodeGraphCLIAdapterClassifiesMissingExecutable(t *testing.T) {
	adapter := CodeGraphCLIAdapter{Executable: filepath.Join(testsupport.ProjectTempDir(t, "missing-codegraph"), "missing-codegraph")}

	readiness, err := adapter.Readiness(context.Background(), CodeProjectRef{AdmittedRoot: testsupport.ProjectTempDir(t, "missing-codegraph-root")})
	if err != nil {
		t.Fatalf("Readiness returned error: %v", err)
	}
	if readiness.ExecutableAvailable || readiness.CachePresent {
		t.Fatalf("readiness = %+v, want missing executable without cache claim", readiness)
	}
}

func TestCodeGraphCLIAdapterProjectsStatusJSON(t *testing.T) {
	runner := func(_ context.Context, _ string, args []string, _ string) ([]byte, error) {
		switch strings.Join(args, " ") {
		case "telemetry status":
			return []byte("Telemetry: disabled"), nil
		default:
			if len(args) == 3 && args[0] == "status" && args[2] == "--json" {
				return []byte(`{
					"initialized": true,
					"projectPath": "D:/repo",
					"indexPath": "D:/repo/.codegraph",
					"pendingChanges": {"added": 1, "modified": 2, "removed": 3},
					"worktreeMismatch": {
						"worktreeRoot": "D:/repo/.worktrees/kernel",
						"indexedProjectPath": "D:/repo"
					},
					"index": {"reindexRecommended": true}
				}`), nil
			}
			t.Fatalf("unexpected args: %v", args)
			return nil, nil
		}
	}
	adapter := CodeGraphCLIAdapter{Executable: "codegraph", Runner: runner}

	readiness, err := adapter.Readiness(context.Background(), CodeProjectRef{AdmittedRoot: "D:/repo/.worktrees/kernel"})
	if err != nil {
		t.Fatalf("Readiness returned error: %v", err)
	}
	if !readiness.ExecutableAvailable || !readiness.CachePresent || readiness.Telemetry != TelemetryDisabled {
		t.Fatalf("readiness = %+v, want executable cache and disabled telemetry", readiness)
	}
	if readiness.PendingChanges.Total() != 6 || !readiness.ReindexRecommended {
		t.Fatalf("readiness = %+v, want pending changes and reindex recommendation", readiness)
	}
	if readiness.WorktreeMismatch == nil || readiness.WorktreeMismatch.IndexedProjectPath != "D:/repo" {
		t.Fatalf("worktree mismatch = %+v, want parsed mismatch", readiness.WorktreeMismatch)
	}
}

func TestCodeGraphCLIAdapterAffectedTestsRemainAdapterItems(t *testing.T) {
	var seen [][]string
	runner := func(_ context.Context, _ string, args []string, _ string) ([]byte, error) {
		seen = append(seen, append([]string(nil), args...))
		if args[0] != "affected" {
			t.Fatalf("unexpected args: %v", args)
		}
		payload, err := json.Marshal(map[string][]string{
			"affectedTests": {"internal/kernel/approval_owner_test.go", ""},
		})
		if err != nil {
			t.Fatalf("marshal affected payload: %v", err)
		}
		return payload, nil
	}
	adapter := CodeGraphCLIAdapter{Executable: "codegraph", Runner: runner}

	result, err := adapter.Query(context.Background(), CodeProjectRef{AdmittedRoot: "D:/repo"}, CodeQuery{
		QueryKind:  QueryKindAffectedTests,
		TargetPath: "internal/kernel/approval.go",
	})
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].Kind != QueryItemAffectedTest || result.Items[0].Ref != "internal/kernel/approval_owner_test.go" {
		t.Fatalf("items = %+v, want one affected-test adapter item", result.Items)
	}
	wantArgs := []string{"affected", "--path", "D:/repo", "internal/kernel/approval.go", "--json"}
	if !reflect.DeepEqual(seen[0], wantArgs) {
		t.Fatalf("args = %+v, want %+v", seen[0], wantArgs)
	}
}
