package kernel

import "time"

type Config struct {
	LedgerPath    string
	Provider      Provider
	JobExecutor   ManagedJobExecutor
	RuntimeToken  string
	ToolPolicy    ToolPolicy
	ContextPolicy ContextPolicy
	BudgetPolicy  BudgetPolicy
	SkillRoots    []string
	Resources     []ResourceDescriptor
	Clock         func() time.Time
}

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

type BudgetLeaseProjection struct {
	ModelToolRoundBudget  int `json:"model_tool_round_budget"`
	ModelToolRoundCeiling int `json:"model_tool_round_ceiling"`
}

type ReadyResponse struct {
	Status      string         `json:"status"`
	Provider    ProviderStatus `json:"provider"`
	RuntimeAuth ReadyCheck     `json:"runtime_auth"`
	Ledger      ReadyCheck     `json:"ledger"`
}

type CapabilitiesResponse struct {
	Status       string                     `json:"status"`
	Provider     ProviderStatus             `json:"provider"`
	RuntimeAuth  ReadyCheck                 `json:"runtime_auth"`
	Ledger       ReadyCheck                 `json:"ledger"`
	BudgetLease  BudgetLeaseProjection      `json:"budget_lease"`
	Tools        []ToolCapabilityProjection `json:"tools"`
	SkillCatalog SkillCatalogProjection     `json:"skill_catalog"`
}

type ProviderStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type ReadyCheck struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}
