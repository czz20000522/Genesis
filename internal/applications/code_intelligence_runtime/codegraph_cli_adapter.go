package codeintelligenceruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const defaultCodeGraphTimeout = 30 * time.Second

type CommandRunner func(context.Context, string, []string, string) ([]byte, error)

type CodeGraphCLIAdapter struct {
	Executable string
	Runner     CommandRunner
	Timeout    time.Duration
}

func (a CodeGraphCLIAdapter) Readiness(ctx context.Context, project CodeProjectRef) (AdapterReadiness, error) {
	executable, ok := a.resolveExecutable()
	if !ok {
		return AdapterReadiness{Adapter: "codegraph", ExecutableAvailable: false, Telemetry: TelemetryUnknown}, nil
	}
	telemetry := a.telemetry(ctx, executable, project.AdmittedRoot)
	output, err := a.run(ctx, executable, []string{"status", project.AdmittedRoot, "--json"}, project.AdmittedRoot)
	if err != nil {
		return AdapterReadiness{
			Adapter:             "codegraph",
			ExecutableAvailable: true,
			CachePresent:        false,
			Telemetry:           telemetry,
			DiagnosticReason:    "status_failed",
		}, nil
	}
	var status codeGraphStatus
	if err := json.Unmarshal(output, &status); err != nil {
		return AdapterReadiness{}, err
	}
	return AdapterReadiness{
		Adapter:             "codegraph",
		ExecutableAvailable: true,
		CachePresent:        status.Initialized,
		ProjectPath:         status.ProjectPath,
		IndexPath:           status.IndexPath,
		Telemetry:           telemetry,
		PendingChanges: PendingChanges{
			Added:    status.PendingChanges.Added,
			Modified: status.PendingChanges.Modified,
			Removed:  status.PendingChanges.Removed,
		},
		WorktreeMismatch:   status.worktreeMismatch(),
		ReindexRecommended: status.Index.ReindexRecommended,
	}, nil
}

func (a CodeGraphCLIAdapter) Query(ctx context.Context, project CodeProjectRef, query CodeQuery) (AdapterQueryResult, error) {
	executable, ok := a.resolveExecutable()
	if !ok {
		return AdapterQueryResult{}, errors.New("codegraph executable missing")
	}
	switch strings.TrimSpace(query.QueryKind) {
	case QueryKindAffectedTests:
		target := strings.TrimSpace(query.TargetPath)
		if target == "" {
			return AdapterQueryResult{}, errors.New("target_path is required")
		}
		output, err := a.run(ctx, executable, []string{"affected", "--path", project.AdmittedRoot, target, "--json"}, project.AdmittedRoot)
		if err != nil {
			return AdapterQueryResult{}, err
		}
		items := parseAffectedTests(output)
		return AdapterQueryResult{Items: items, DiagnosticReason: "adapter_reported_affected_tests"}, nil
	case QueryKindExplore:
		text := strings.TrimSpace(query.QueryText)
		if text == "" {
			return AdapterQueryResult{}, errors.New("query_text is required")
		}
		output, err := a.run(ctx, executable, []string{"explore", text, "--path", project.AdmittedRoot}, project.AdmittedRoot)
		if err != nil {
			return AdapterQueryResult{}, err
		}
		return AdapterQueryResult{
			Items: []CodeQueryItem{{Kind: "explore_result", Text: boundedText(string(output), 32*1024)}},
		}, nil
	default:
		return AdapterQueryResult{}, errors.New("unsupported query_kind")
	}
}

func (a CodeGraphCLIAdapter) resolveExecutable() (string, bool) {
	executable := strings.TrimSpace(a.Executable)
	if executable != "" {
		if a.Runner != nil {
			return executable, true
		}
		resolved, err := exec.LookPath(executable)
		if err != nil {
			return "", false
		}
		return resolved, true
	}
	found, err := exec.LookPath("codegraph")
	if err != nil {
		return "", false
	}
	return found, true
}

func (a CodeGraphCLIAdapter) telemetry(ctx context.Context, executable string, dir string) string {
	output, err := a.run(ctx, executable, []string{"telemetry", "status"}, dir)
	if err != nil {
		return TelemetryUnknown
	}
	text := strings.ToLower(string(output))
	switch {
	case strings.Contains(text, "disabled"):
		return TelemetryDisabled
	case strings.Contains(text, "enabled"):
		return TelemetryEnabled
	default:
		return TelemetryUnknown
	}
}

func (a CodeGraphCLIAdapter) run(ctx context.Context, executable string, args []string, dir string) ([]byte, error) {
	runner := a.Runner
	if runner == nil {
		runner = runCommand
	}
	timeout := a.Timeout
	if timeout <= 0 {
		timeout = defaultCodeGraphTimeout
	}
	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return runner(commandCtx, executable, args, dir)
}

func runCommand(ctx context.Context, executable string, args []string, dir string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		if stderr.Len() > 0 {
			return output, errors.New(strings.TrimSpace(stderr.String()))
		}
		return output, err
	}
	return output, nil
}

type codeGraphStatus struct {
	Initialized    bool   `json:"initialized"`
	ProjectPath    string `json:"projectPath"`
	IndexPath      string `json:"indexPath"`
	PendingChanges struct {
		Added    int `json:"added"`
		Modified int `json:"modified"`
		Removed  int `json:"removed"`
	} `json:"pendingChanges"`
	WorktreeMismatch *struct {
		WorktreeRoot       string `json:"worktreeRoot"`
		IndexedProjectPath string `json:"indexedProjectPath"`
	} `json:"worktreeMismatch"`
	Index struct {
		ReindexRecommended bool `json:"reindexRecommended"`
	} `json:"index"`
}

func (s codeGraphStatus) worktreeMismatch() *WorktreeMismatch {
	if s.WorktreeMismatch == nil {
		return nil
	}
	return &WorktreeMismatch{
		WorktreeRoot:       s.WorktreeMismatch.WorktreeRoot,
		IndexedProjectPath: s.WorktreeMismatch.IndexedProjectPath,
	}
}

func parseAffectedTests(output []byte) []CodeQueryItem {
	var payload struct {
		AffectedTests []string `json:"affectedTests"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return []CodeQueryItem{{Kind: QueryItemAffectedTest, Text: boundedText(string(output), 16*1024)}}
	}
	items := make([]CodeQueryItem, 0, len(payload.AffectedTests))
	for _, test := range payload.AffectedTests {
		test = strings.TrimSpace(test)
		if test == "" {
			continue
		}
		items = append(items, CodeQueryItem{Kind: QueryItemAffectedTest, Ref: filepath.ToSlash(test)})
	}
	return items
}

func boundedText(text string, limit int) string {
	if limit <= 0 || len(text) <= limit {
		return text
	}
	return text[:limit]
}
