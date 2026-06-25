package codeintelligenceruntime

const (
	ReadinessReady        = "ready"
	ReadinessNotInstalled = "not_installed"
	ReadinessCacheMissing = "cache_missing"
	ReadinessCacheStale   = "cache_stale"
	ReadinessDegraded     = "degraded"
	ReadinessBlocked      = "blocked"

	FreshnessFresh              = "fresh"
	FreshnessUnknown            = "unknown"
	FreshnessWorktreeMismatch   = "worktree_mismatch"
	FreshnessPendingChanges     = "pending_changes"
	FreshnessReindexRecommended = "reindex_recommended"

	TelemetryDisabled = "disabled"
	TelemetryEnabled  = "enabled"
	TelemetryUnknown  = "unknown"

	QueryKindExplore       = "explore"
	QueryKindAffectedTests = "affected_tests"

	QueryStatusCompleted = "completed"
	QueryStatusBlocked   = "blocked"
	QueryStatusFailed    = "failed"

	QueryItemAffectedTest = "affected_test"

	EvidenceRoleHint = "hint"

	VerificationAdvisory = "advisory"
)

type CodeProjectRef struct {
	ProjectRef        string `json:"project_ref"`
	DisplayName       string `json:"display_name,omitempty"`
	RootDigest        string `json:"root_digest,omitempty"`
	AdmittedRoot      string `json:"admitted_root"`
	AdapterBindingRef string `json:"adapter_binding_ref,omitempty"`
}

type PendingChanges struct {
	Added    int `json:"added"`
	Modified int `json:"modified"`
	Removed  int `json:"removed"`
}

func (p PendingChanges) Total() int {
	return p.Added + p.Modified + p.Removed
}

type WorktreeMismatch struct {
	WorktreeRoot       string `json:"worktree_root,omitempty"`
	IndexedProjectPath string `json:"indexed_project_path,omitempty"`
}

type AdapterReadiness struct {
	Adapter             string            `json:"adapter"`
	ExecutableAvailable bool              `json:"executable_available"`
	CachePresent        bool              `json:"cache_present"`
	ProjectPath         string            `json:"project_path,omitempty"`
	IndexPath           string            `json:"index_path,omitempty"`
	Telemetry           string            `json:"telemetry"`
	PendingChanges      PendingChanges    `json:"pending_changes,omitempty"`
	WorktreeMismatch    *WorktreeMismatch `json:"worktree_mismatch,omitempty"`
	ReindexRecommended  bool              `json:"reindex_recommended,omitempty"`
	Degraded            bool              `json:"degraded,omitempty"`
	DiagnosticReason    string            `json:"diagnostic_reason,omitempty"`
}

type CodeIndexReadiness struct {
	ProjectRef         string            `json:"project_ref"`
	Adapter            string            `json:"adapter"`
	Status             string            `json:"status"`
	Freshness          string            `json:"freshness"`
	Telemetry          string            `json:"telemetry"`
	BlockedReason      string            `json:"blocked_reason,omitempty"`
	ProjectPath        string            `json:"project_path,omitempty"`
	IndexPath          string            `json:"index_path,omitempty"`
	PendingChanges     PendingChanges    `json:"pending_changes,omitempty"`
	WorktreeMismatch   *WorktreeMismatch `json:"worktree_mismatch,omitempty"`
	DiagnosticReason   string            `json:"diagnostic_reason,omitempty"`
	ReindexRecommended bool              `json:"reindex_recommended,omitempty"`
}

type CodeQuery struct {
	QueryKind               string `json:"query_kind"`
	QueryText               string `json:"query_text,omitempty"`
	SymbolRef               string `json:"symbol_ref,omitempty"`
	TargetPath              string `json:"target_path,omitempty"`
	ResultLimit             int    `json:"result_limit,omitempty"`
	AcceptDegradedFreshness bool   `json:"accept_degraded_freshness,omitempty"`
}

type CodeQueryItem struct {
	Kind         string `json:"kind"`
	Ref          string `json:"ref,omitempty"`
	Text         string `json:"text,omitempty"`
	EvidenceRole string `json:"evidence_role,omitempty"`
	Verification string `json:"verification,omitempty"`
	Proof        bool   `json:"proof,omitempty"`
}

type AdapterQueryResult struct {
	Items            []CodeQueryItem `json:"items,omitempty"`
	DiagnosticReason string          `json:"diagnostic_reason,omitempty"`
}

type CodeQueryResult struct {
	ProjectRef       string          `json:"project_ref"`
	QueryKind        string          `json:"query_kind"`
	Status           string          `json:"status"`
	ReadinessStatus  string          `json:"readiness_status"`
	Freshness        string          `json:"freshness"`
	Items            []CodeQueryItem `json:"items,omitempty"`
	Truncated        bool            `json:"truncated,omitempty"`
	DiagnosticReason string          `json:"diagnostic_reason,omitempty"`
}
