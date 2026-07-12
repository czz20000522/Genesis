package kernel

import "time"

type WorkerProviderResolver func(profileID string) (Provider, error)
type SessionProviderResolver func(profileID string) (Provider, error)
type ProviderRouteDiscoverer func(routeID string) ProviderRouteModelDiscoveryResult

type Config struct {
	LedgerPath              string
	Provider                Provider
	ProviderVerifier        ProviderVerifier
	JobExecutor             ManagedJobExecutor
	RuntimeToken            string
	ToolPolicy              ToolPolicy
	ContextPolicy           ContextPolicy
	BudgetPolicy            BudgetPolicy
	ShellTimeoutPolicy      ShellTimeoutPolicy
	SourceSnapshotPolicy    SourceSnapshotPolicy
	SkillRoots              []string
	CapabilityDescriptors   []CapabilityDescriptor
	Resources               []ResourceDescriptor
	MaterialStorePath       string
	ParentWorkerConfigRoot  string
	ParentWorkerParentID    string
	WorkerProviderResolver  WorkerProviderResolver
	SessionProviderResolver SessionProviderResolver
	ProviderRouteDiscoverer ProviderRouteDiscoverer
	Clock                   func() time.Time
}

type ProviderVerificationRequest struct {
	ModelRole string `json:"model_role,omitempty"`
	ProfileID string `json:"profile_id,omitempty"`
}

type ProviderVerifier func(ProviderVerificationRequest) ProviderLiveVerifyResult

type ToolPolicy struct {
	PermissionMode string
	WorkspaceRoot  string
	SandboxProfile string
	ApprovalPolicy string
}

type ContextPolicy struct {
	ContextWindowTokens int
	AutoCompactRatio    float64
	RecentTurnLimit     int
	RecentTailTokens    int
	SkillIndexChars     int
	RetryBackoffTurns   int
}

type BudgetPolicy struct {
	ModelToolRoundBudget  int
	ModelToolRoundCeiling int
}

type ShellTimeoutPolicy struct {
	DefaultForegroundTimeoutSec int `json:"default_foreground_timeout_sec"`
	ForegroundTimeoutCapSec     int `json:"foreground_timeout_cap_sec"`
	ManagedJobThresholdSec      int `json:"managed_job_threshold_sec"`
}

type BudgetLeaseProjection struct {
	ModelToolRoundBudget  int `json:"model_tool_round_budget"`
	ModelToolRoundCeiling int `json:"model_tool_round_ceiling"`
}

type RuntimeLimitProjection struct {
	Name           string `json:"name"`
	Class          string `json:"class"`
	Owner          string `json:"owner"`
	DefaultSource  string `json:"default_source"`
	Inspectable    bool   `json:"inspectable"`
	ModelVisible   bool   `json:"model_visible"`
	OverridePolicy string `json:"override_policy"`
	EffectiveValue int    `json:"effective_value,omitempty"`
	Unit           string `json:"unit,omitempty"`
}

const (
	ReadinessReady    = "ready"
	ReadinessNotReady = "not_ready"
)

type ReadyResponse struct {
	Readiness       string         `json:"readiness"`
	ReadinessReason string         `json:"readiness_reason,omitempty"`
	Provider        ProviderStatus `json:"provider"`
	RuntimeAuth     ReadyCheck     `json:"runtime_auth"`
	Ledger          ReadyCheck     `json:"ledger"`
}

type CapabilitiesResponse struct {
	Readiness                 string                     `json:"readiness"`
	ReadinessReason           string                     `json:"readiness_reason,omitempty"`
	Provider                  ProviderStatus             `json:"provider"`
	RuntimeAuth               ReadyCheck                 `json:"runtime_auth"`
	Ledger                    ReadyCheck                 `json:"ledger"`
	BudgetLease               BudgetLeaseProjection      `json:"budget_lease"`
	ShellTimeoutPolicy        ShellTimeoutPolicy         `json:"shell_timeout_policy"`
	SourceSnapshotPersistence ReadyCheck                 `json:"source_snapshot_persistence"`
	Limits                    []RuntimeLimitProjection   `json:"limits"`
	Tools                     []ToolCapabilityProjection `json:"tools"`
	SkillCatalog              SkillCatalogProjection     `json:"skill_catalog"`
}

type ProviderStatus struct {
	Name            string `json:"name"`
	Readiness       string `json:"readiness"`
	ReadinessReason string `json:"readiness_reason,omitempty"`
}

type ReadyCheck struct {
	Readiness       string `json:"readiness"`
	ReadinessReason string `json:"readiness_reason,omitempty"`
}
