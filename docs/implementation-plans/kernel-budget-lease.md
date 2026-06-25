# Implementation Plan: Kernel BudgetLease

## Reference Scan

- Reasonix keeps harness-loop step limits in agent options and configuration (`agent.max_steps`, `agent.planner_max_steps`). Its run loop treats a positive limit as a resumable guard and reports the configured knob when exhausted; `0` is a Reasonix-specific unlimited setting.
- Codex exposes shell output budgets such as `max_output_tokens` in tool schemas because those are output projection limits, not turn authority budgets.
- Genesis keeps a different contract: tool-loop exhaustion is `turn.paused` evidence owned by the kernel, and model-visible schemas must not let the model raise or forge the turn execution budget.

## Scope

Phase A makes `BudgetLease` own the model tool-round budget used by `SubmitTurn`.

- Add kernel config for a default model tool-round budget and an allowed ceiling.
- Normalize unset or non-positive budget values to the documented default; no unlimited behavior is introduced.
- Mint a per-turn `BudgetLease` before the provider/tool loop starts.
- Consume the lease for provider tool-round admission instead of reading package constants.
- Expose the effective lease through capabilities and context inspection.
- Record effective lease values in `turn.paused`.

## Non-Goals

- Do not move shell foreground timeout, output truncation, HTTP request limits, provider response body limits, provider retry attempts, or visible-final repair attempts into this lease in this slice.
- Do not add model-visible tool arguments for budget control.
- Do not add `/v1`, `policy_version`, or other versioned runtime identifiers.

## Verification

- Default/unset config pauses at the documented default and reports that value.
- A higher configured lease permits more committed tool rounds before pause.
- Zero config is not unlimited.
- Model-visible tool manifests do not expose fields that can raise or forge the lease.
- Requirement/design docs distinguish execution budgets from hard safety/projection caps.
