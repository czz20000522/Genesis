package codeintelligenceruntime

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
)

const defaultResultLimit = 20

var ErrCodeReadinessBlocked = errors.New("code intelligence readiness blocked")

type Adapter interface {
	Readiness(context.Context, CodeProjectRef) (AdapterReadiness, error)
	Query(context.Context, CodeProjectRef, CodeQuery) (AdapterQueryResult, error)
}

type Runtime struct {
	adapter           Adapter
	telemetryAccepted bool
}

type Option func(*Runtime)

func WithTelemetryAccepted(accepted bool) Option {
	return func(r *Runtime) {
		r.telemetryAccepted = accepted
	}
}

func NewRuntime(adapter Adapter, opts ...Option) Runtime {
	r := Runtime{adapter: adapter}
	for _, opt := range opts {
		if opt != nil {
			opt(&r)
		}
	}
	return r
}

func (r Runtime) Probe(ctx context.Context, project CodeProjectRef) (CodeIndexReadiness, error) {
	if strings.TrimSpace(project.ProjectRef) == "" {
		project.ProjectRef = "code_project"
	}
	root, reason := normalizeAdmittedRoot(project.AdmittedRoot)
	if reason != "" {
		return CodeIndexReadiness{
			ProjectRef:    project.ProjectRef,
			Status:        ReadinessBlocked,
			Freshness:     FreshnessUnknown,
			Telemetry:     TelemetryUnknown,
			BlockedReason: reason,
		}, nil
	}
	project.AdmittedRoot = root
	if r.adapter == nil {
		return CodeIndexReadiness{
			ProjectRef:    project.ProjectRef,
			Status:        ReadinessNotInstalled,
			Freshness:     FreshnessUnknown,
			Telemetry:     TelemetryUnknown,
			BlockedReason: "adapter_missing",
		}, nil
	}
	adapterReadiness, err := r.adapter.Readiness(ctx, project)
	if err != nil {
		return CodeIndexReadiness{}, err
	}
	return r.projectReadiness(project, adapterReadiness), nil
}

func (r Runtime) Query(ctx context.Context, project CodeProjectRef, query CodeQuery) (CodeQueryResult, error) {
	if root, reason := normalizeAdmittedRoot(project.AdmittedRoot); reason == "" {
		project.AdmittedRoot = root
	}
	readiness, err := r.Probe(ctx, project)
	if err != nil {
		return CodeQueryResult{}, err
	}
	result := CodeQueryResult{
		ProjectRef:       readiness.ProjectRef,
		QueryKind:        strings.TrimSpace(query.QueryKind),
		ReadinessStatus:  readiness.Status,
		Freshness:        readiness.Freshness,
		DiagnosticReason: readiness.BlockedReason,
	}
	if !readinessAllowsQuery(readiness.Status, query.AcceptDegradedFreshness) {
		result.Status = QueryStatusBlocked
		if result.DiagnosticReason == "" {
			result.DiagnosticReason = readiness.Status
		}
		return result, ErrCodeReadinessBlocked
	}
	if r.adapter == nil {
		result.Status = QueryStatusBlocked
		result.DiagnosticReason = "adapter_missing"
		return result, ErrCodeReadinessBlocked
	}
	adapterResult, err := r.adapter.Query(ctx, project, query)
	if err != nil {
		result.Status = QueryStatusFailed
		result.DiagnosticReason = "adapter_query_failed"
		return result, err
	}
	result.Status = QueryStatusCompleted
	result.DiagnosticReason = adapterResult.DiagnosticReason
	limit := normalizeResultLimit(query.ResultLimit)
	items := normalizeQueryItems(adapterResult.Items)
	if len(items) > limit {
		items = items[:limit]
		result.Truncated = true
	}
	result.Items = items
	return result, nil
}

func (r Runtime) projectReadiness(project CodeProjectRef, adapter AdapterReadiness) CodeIndexReadiness {
	telemetry := strings.TrimSpace(adapter.Telemetry)
	if telemetry == "" {
		telemetry = TelemetryUnknown
	}
	readiness := CodeIndexReadiness{
		ProjectRef:         project.ProjectRef,
		Adapter:            strings.TrimSpace(adapter.Adapter),
		ProjectPath:        strings.TrimSpace(adapter.ProjectPath),
		IndexPath:          strings.TrimSpace(adapter.IndexPath),
		Telemetry:          telemetry,
		PendingChanges:     adapter.PendingChanges,
		WorktreeMismatch:   adapter.WorktreeMismatch,
		DiagnosticReason:   strings.TrimSpace(adapter.DiagnosticReason),
		ReindexRecommended: adapter.ReindexRecommended,
		Freshness:          FreshnessFresh,
	}
	switch {
	case !adapter.ExecutableAvailable:
		readiness.Status = ReadinessNotInstalled
		readiness.Freshness = FreshnessUnknown
		readiness.BlockedReason = "executable_missing"
	case telemetry == TelemetryEnabled && !r.telemetryAccepted:
		readiness.Status = ReadinessBlocked
		readiness.BlockedReason = "telemetry_enabled"
	case telemetry == TelemetryUnknown && !r.telemetryAccepted:
		readiness.Status = ReadinessBlocked
		readiness.BlockedReason = "telemetry_unknown"
	case adapter.WorktreeMismatch != nil:
		readiness.Status = ReadinessBlocked
		readiness.Freshness = FreshnessWorktreeMismatch
		readiness.BlockedReason = "worktree_mismatch"
	case !adapter.CachePresent:
		readiness.Status = ReadinessCacheMissing
		readiness.Freshness = FreshnessUnknown
		readiness.BlockedReason = "cache_missing"
	case adapter.PendingChanges.Total() > 0:
		readiness.Status = ReadinessCacheStale
		readiness.Freshness = FreshnessPendingChanges
		readiness.BlockedReason = "pending_changes"
	case adapter.ReindexRecommended:
		readiness.Status = ReadinessCacheStale
		readiness.Freshness = FreshnessReindexRecommended
		readiness.BlockedReason = "reindex_recommended"
	case adapter.Degraded:
		readiness.Status = ReadinessDegraded
		readiness.Freshness = FreshnessUnknown
		if readiness.BlockedReason == "" {
			readiness.BlockedReason = "adapter_degraded"
		}
	default:
		readiness.Status = ReadinessReady
	}
	return readiness
}

func normalizeAdmittedRoot(root string) (string, string) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", "admitted_root_required"
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", "admitted_root_invalid"
	}
	return filepath.Clean(abs), ""
}

func readinessAllowsQuery(status string, acceptDegraded bool) bool {
	status = strings.TrimSpace(status)
	if status == ReadinessReady {
		return true
	}
	if acceptDegraded && (status == ReadinessCacheStale || status == ReadinessDegraded) {
		return true
	}
	return false
}

func normalizeResultLimit(limit int) int {
	if limit <= 0 {
		return defaultResultLimit
	}
	if limit > defaultResultLimit {
		return defaultResultLimit
	}
	return limit
}

func normalizeQueryItems(items []CodeQueryItem) []CodeQueryItem {
	out := make([]CodeQueryItem, 0, len(items))
	for _, item := range items {
		item.EvidenceRole = EvidenceRoleHint
		item.Verification = VerificationAdvisory
		item.Proof = false
		out = append(out, item)
	}
	return out
}
