# Implementation Plan: Kernel BudgetLease

## Status

Closed for the current BudgetLease slice. Current implementation evidence lives in
`internal/kernel/budget_lease.go`, `internal/kernel/tool_loop_control_test.go`,
`internal/kernel/limit_policy_test.go`, and the foundation requirement/design.

## Reference Scan

- Reasonix keeps harness-loop step limits in agent options and configuration
  (`agent.max_steps`, `agent.planner_max_steps`). Its run loop treats a positive
  limit as a resumable guard and reports the configured knob when exhausted; `0`
  is a Reasonix-specific unlimited setting.
- Codex exposes shell output budgets such as `max_output_tokens` in tool schemas
  because those are output projection limits, not turn authority budgets.
- Genesis keeps a different contract: tool-loop exhaustion is `turn.paused`
  evidence owned by the kernel, and model-visible schemas must not let the model
  raise or forge the turn execution budget.

## Reference Behavior Red Tests

- Reasonix configured harness-loop limit became Genesis tests proving the
  tool-round budget is configurable, normalized, capped, and reported in
  `turn.paused` evidence.
- Codex model-visible output-budget schema became Genesis tests proving
  execution budget fields are absent from model-visible tool schemas.
- The initial red condition was the hidden package constant controlling
  tool-loop admission without an inspectable `BudgetLease` projection.

## Delivered Scope

- Kernel config owns a default model tool-round budget and an allowed ceiling.
- Unset, non-positive, and over-ceiling budget values normalize to bounded
  kernel-owned values; no unlimited behavior is introduced.
- Each turn mints a `BudgetLease` before provider/tool-loop execution.
- The provider loop consumes the lease for tool-round admission instead of
  reading package constants.
- Capabilities and context inspection expose the effective lease.
- `turn.paused` records the effective lease values.

## Non-Goals

- Shell foreground timeout, output truncation, HTTP request limits, provider
  response body limits, provider retry attempts, and visible-final repair
  attempts stay with their existing owner-specific policies.
- Models do not receive tool arguments that control execution budgets.
- No numbered runtime identifiers, policy versions, or versioned route prefixes
  were added.

## Verification

- `TestSubmitTurnUsesConfiguredBudgetLeaseForToolRounds`
- `TestSubmitTurnNormalizesZeroBudgetLeaseToDefault`
- `TestBudgetLeaseIsInspectableButNotModelVisible`
- `TestBudgetLeaseClampsConfiguredBudgetToCeiling`
- `TestKernelLimitClassificationCoversActiveBudgetGuardAndProjectionCaps`
