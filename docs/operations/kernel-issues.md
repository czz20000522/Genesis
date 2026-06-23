# Kernel Issue Ledger

This file is the repo-owned ledger for active Genesis Kernel issues. Feishu Base is the collaboration inbox; this file is the durable project source for issues that still need code, verification, or user acceptance.

Retired issues must not remain here. Move accepted retirements to `docs/operations/kernel-retirement-log.md` with the fixing commits, verification evidence, residual risks, and retirement reason.

## Ledger Rules

- Keep only `new`, `open`, `in_progress`, or otherwise active issues in this file.
- Do not record application-specific feature work as kernel work unless it changes a kernel primitive.
- Do not add versioned HTTP route names as current contracts. HTTP is transport; current kernel endpoints are unversioned.
- `ready_for_acceptance` issues move to the retirement log as retirement candidates and leave this active ledger.
- Feishu/Base links may point to collaboration artifacts, but this repo must contain enough evidence to understand the current status without opening Feishu.
- Every active `KERNEL-*` issue must include a `Reference alignment` field that compares the issue to Codex, Reasonix, or an explicitly rejected drift risk.
- Before a non-trivial implementation slice starts, the related implementation plan or issue must include a Codex/Reasonix reference scan. The scan should identify inspected references, learned control-plane semantics, intentional differences, and remaining drift risks.
- Issues record the current gap between approved requirements/designs and the implementation. They must not carry raw requirements, design discussion, or the full production acceptance contract.
- Every active issue must cite an approved requirement and design unless it is an obvious bug or test gap. If an issue uses that exception, state the exception explicitly.
- Prefer `Gap`, `Next slice`, `Evidence`, and `Verification` over broad `Problem` or `Suggestion` text when adding new issues.
- Do not use issues as debug logs. Routine info, stream chunks, repeated status polling, and exploratory notes stay out of this ledger unless they identify a current implementation gap.
- When an issue removes a concept from the current kernel contract, long-term tests must assert the positive replacement behavior. Do not keep permanent tests whose only purpose is locking retired names, aliases, routes, or historical helper APIs out of the tree; use temporary scans or retirement-log evidence for cleanup windows, then fold the guard into the current owner contract.

## Active Issues

### KERNEL-JOB-CONTROL-INTERRUPT-20260623 - P2 - Interrupt and managed executor semantics

- Status: open.
- Area: Tool Runtime / session control.
- Requirement: `docs/requirements/kernel-shell-and-job-control.md`.
- Design: `docs/design/kernel-shell-and-job-control.md`.
- Gap: The kernel now has a minimal managed shell executor behind long-running `shell_exec` receipts and `job_cancel` can reach live executor state, but interrupt semantics for provider streaming and foreground shell execution are still missing. Progress snapshots and idle continuation policy are also still deferred.
- Next slice: Define provider-stream interrupt and foreground shell detach-or-kill behavior. The model-visible `job_cancel` surface must stay semantic; any process ids, signals, `taskkill`, or process group mechanics remain hidden behind the kernel-owned executor.
- Evidence: `ce72dfa44` registers `job_status` and `job_cancel`, replays job status from the ledger, returns `job_not_found` repair feedback for unknown handles, rejects process/control-plane fields, and records idempotent cancel facts for non-terminal jobs. The current managed-executor slice starts live shell jobs for `timeout_sec > 180`, writes no fake immediate terminal event, records `job.cancel_requested` before executor cancellation, and writes `job.cancelled` only after executor confirmation.
- Verification: Existing verification covers the minimal tool surface and live managed executor cancellation. Remaining verification must prove assistant-output interruption does not kill a background job by default and interrupted foreground shell behavior is deterministic.
- Reference alignment: Aligned with Codex's distinction between session/control events and process lifecycle. Genesis should keep cancellation as a kernel command or model-visible job-control tool, while process mechanics stay behind a kernel-owned managed executor.

### KERNEL-SANDBOX-APPROVAL-NEXT-20260623 - P2 - Stronger sandbox and approval policy beyond the minimal profile split

- Status: open.
- Area: Authority Plane / Tool Runtime.
- Requirement: `docs/requirements/kernel-foundation-capabilities.md`.
- Design: `docs/design/kernel-foundation-capabilities.md`.
- Gap: The current foundation correctly separates `permission_mode`, `authority_policy`, `sandbox_profile`, and `approval_policy`, but `approval_policy` is always `never`, and `default` uses a controlled workspace adapter rather than an OS-level sandbox. That is acceptable for the first ground layer, but not enough for broader arbitrary command execution.
- Next slice: Keep the current split as the owner path while adding future stronger sandbox and approval behavior only through kernel-owned profile resolution and typed control-plane flow.
- Evidence: Current docs and tests now state `controlled_workspace` is not an OS sandbox and provider-visible tool results must not include permission/profile control-plane fields.
- Verification: The existing positive contract remains true; when a stronger sandbox or approval flow is added, unknown or unavailable sandbox profiles fail closed, approval denial returns structured feedback without execution, and model-supplied control-plane fields are rejected as repairable invalid requests.
- Reference alignment: Aligned with Codex's sandbox/approval split and Reasonix's central controller model. The active drift risk is over-promising `default` as a real OS sandbox or turning approval into shell/UI-local logic.
