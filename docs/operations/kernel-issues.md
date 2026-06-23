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
- Reference alignment uses local reference checkouts only. Do not cite Genesis GitHub repositories, remote issues, releases, or pull requests as authority for this local kernel line unless the user explicitly asks for external publishing context.
- Before a non-trivial implementation slice starts, the related implementation plan or issue must include a Codex/Reasonix reference scan. The scan should identify inspected references, learned control-plane semantics, intentional differences, and remaining drift risks.
- Issues record the current gap between approved requirements/designs and the implementation. They must not carry raw requirements, design discussion, or the full production acceptance contract.
- Every active issue must cite an approved requirement and design unless it is an obvious bug or test gap. If an issue uses that exception, state the exception explicitly.
- Prefer `Gap`, `Next slice`, `Evidence`, and `Verification` over broad `Problem` or `Suggestion` text when adding new issues.
- Do not use issues as debug logs. Routine info, stream chunks, repeated status polling, and exploratory notes stay out of this ledger unless they identify a current implementation gap.
- When an issue removes a concept from the current kernel contract, long-term tests must assert the positive replacement behavior. Do not keep permanent tests whose only purpose is locking retired names, aliases, routes, or historical helper APIs out of the tree; use temporary scans or retirement-log evidence for cleanup windows, then fold the guard into the current owner contract.
- Development artifacts and historical local data are not compatibility obligations. Do not create or keep issues whose only purpose is migrating, cleaning, importing, or preserving old local generated state unless that state is part of the approved current kernel contract.
- Every implementation slice must finish with a drift check against the governing requirement, design, implementation plan, issue, and BDD feature. In-scope drift is fixed before commit. Out-of-scope drift is recorded here as an active issue with evidence and next slice before commit.
- Periodic governance review checks architecture, feature behavior, directory structure, and document lifetime together. Completed plans and stale documents should be deleted or condensed instead of spawning issues that only preserve old notes.

## Active Issues

### KERNEL-JOB-PROGRESS-IDLE-CONTINUATION-20260623 - P2 - Managed job progress snapshots and idle continuation policy

- Status: open.
- Area: Tool Runtime / Interface Kernel / Model Gateway.
- Requirement: `docs/requirements/kernel-shell-and-job-control.md`.
- Design: `docs/design/kernel-shell-and-job-control.md`.
- Gap: Turn interruption and live job cancellation now have a minimal kernel path, but managed jobs still do not emit progress snapshots, idle sessions do not have a user-triggered continuation policy, and the local executor cannot attach/detach an already-running foreground shell into a managed job.
- Next slice: Add sparse `job.progress` or `job.output` snapshots only when they are useful durable facts, define how a user "continue" command drains pending job observations for an idle session, and keep attach-capable foreground shell behavior behind executor capability detection instead of exposing process ids or signals.
- Evidence: `6e3287525` adds `InterruptSession`, `POST /sessions/{id}/interrupt`, `assistant.interrupted`, `operation.interrupted`, and tests proving provider-step interruption does not cancel an existing background job. Existing managed-job tests cover terminal completion and explicit `job_cancel`.
- Verification: Remaining verification must prove progress snapshots are bounded and not injected into every provider step, idle continuation drains queued observations without autonomous wakeup, and any future attach-capable executor behavior remains hidden behind kernel-owned executor semantics.
- Reference alignment: Aligned with Codex and Reasonix's split between turn/session cancellation and background process lifecycle. The remaining drift risk is turning progress or continuation into UI/provider-owned behavior instead of kernel-owned observation delivery and checkpoint policy.

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
