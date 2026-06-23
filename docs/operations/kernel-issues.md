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
- Issues record the current gap between approved requirements/designs and the implementation. They must not carry raw requirements, design discussion, or the full production acceptance contract.
- Every active issue must cite an approved requirement and design unless it is an obvious bug or test gap. If an issue uses that exception, state the exception explicitly.
- Prefer `Gap`, `Next slice`, `Evidence`, and `Verification` over broad `Problem` or `Suggestion` text when adding new issues.
- When an issue removes a concept from the current kernel contract, long-term tests must assert the positive replacement behavior. Do not keep permanent tests whose only purpose is locking retired names, aliases, routes, or historical helper APIs out of the tree; use temporary scans or retirement-log evidence for cleanup windows, then fold the guard into the current owner contract.

## Active Issues

### KERNEL-OBSERVATION-DELIVERY-20260623 - P1 - Kernel observation queue and delivery checkpoints

- Status: open.
- Area: Interface Kernel / Provider context projection / Ledger.
- Requirement: `docs/requirements/kernel-shell-and-job-control.md`.
- Design: `docs/design/kernel-shell-and-job-control.md`.
- Gap: Job completion and other kernel observations need a delivery model. The kernel currently has provider context projection and checkpoints, but no explicit rule for which background observations have been delivered to the model and which remain pending.
- Next slice: Treat terminal job facts and similar system facts as Kernel Observation Queue sources. Add delivery semantics only through kernel checkpoints, not shell/UI/provider adapter logic.
- Evidence: The current ledger is restart-safe for turns, operations, memory, and compaction evidence, but it does not track observation delivery ids for future job completions.
- Verification: A completed background job is visible in UI/session projection immediately, does not start a provider call while the session is idle, is included in the next provider context when the session resumes or continues, and is not delivered twice after restart.
- Reference alignment: Aligned with Codex's core/session ownership of compaction and tool-loop state: shells submit typed commands and observations; the core decides when provider context incorporates them. This rejects the drift where an external daemon or UI secretly drives model execution.

### KERNEL-JOB-CONTROL-INTERRUPT-20260623 - P2 - Interrupt and job control semantics

- Status: open.
- Area: Tool Runtime / session control.
- Requirement: `docs/requirements/kernel-shell-and-job-control.md`.
- Design: `docs/design/kernel-shell-and-job-control.md`.
- Gap: The current minimal kernel does not define how user interruption interacts with provider streaming, foreground shell execution, or already-managed background jobs. This becomes ambiguous once foreground cap and managed jobs exist.
- Next slice: Define the minimal interrupt and job-control behavior around provider streaming, foreground shell execution, managed jobs, and explicit cancellation.
- Evidence: Current shell and provider paths are short synchronous paths. There is no job handle or cancel owner yet, so cancellation cannot be audited separately from ordinary command failure.
- Verification: Interrupting assistant output does not kill an existing background job; explicit job cancel writes cancel request and terminal cancel evidence; interrupted foreground shell behavior is deterministic and reflected in tool/session projections.
- Reference alignment: Aligned with Codex's distinction between session/control events and process lifecycle. Genesis should keep cancellation as a kernel command or model-visible job-control tool, not as UI-local behavior.

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
