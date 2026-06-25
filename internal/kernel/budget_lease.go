package kernel

const (
	defaultModelToolRoundBudget  = 4
	defaultModelToolRoundCeiling = 32
)

type budgetLease struct {
	modelToolRoundBudget  int
	modelToolRoundCeiling int
}

func normalizedBudgetPolicy(policy BudgetPolicy) BudgetPolicy {
	ceiling := policy.ModelToolRoundCeiling
	if ceiling <= 0 {
		ceiling = defaultModelToolRoundCeiling
	}
	budget := policy.ModelToolRoundBudget
	if budget <= 0 {
		budget = defaultModelToolRoundBudget
	}
	if budget > ceiling {
		budget = ceiling
	}
	return BudgetPolicy{
		ModelToolRoundBudget:  budget,
		ModelToolRoundCeiling: ceiling,
	}
}

func newBudgetLease(policy BudgetPolicy) budgetLease {
	normalized := normalizedBudgetPolicy(policy)
	return budgetLease{
		modelToolRoundBudget:  normalized.ModelToolRoundBudget,
		modelToolRoundCeiling: normalized.ModelToolRoundCeiling,
	}
}

func (k *Kernel) newTurnBudgetLease() budgetLease {
	return newBudgetLease(k.budgetPolicy)
}

func (k *Kernel) budgetLeaseProjection() BudgetLeaseProjection {
	return k.newTurnBudgetLease().projection()
}

func (l budgetLease) allowModelToolRound(roundIndex int) bool {
	return roundIndex < l.modelToolRoundBudget
}

func (l budgetLease) projection() BudgetLeaseProjection {
	return BudgetLeaseProjection{
		ModelToolRoundBudget:  l.modelToolRoundBudget,
		ModelToolRoundCeiling: l.modelToolRoundCeiling,
	}
}
