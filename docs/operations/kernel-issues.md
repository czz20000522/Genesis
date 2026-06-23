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

### KERNEL-JOB-PROGRESS-IDLE-CONTINUATION-20260623 - P2 - Local managed job streaming and attach capability

- Status: open.
- Area: Tool Runtime / Interface Kernel / Model Gateway.
- Requirement: `docs/requirements/kernel-shell-and-job-control.md`.
- Design: `docs/design/kernel-shell-and-job-control.md`.
- Gap: Kernel now has the first `job.output` snapshot contract, the local managed shell executor emits bounded sparse live output snapshots, and foreground shell interruption is capability-gated. The local executor explicitly does not advertise foreground attach support, so interrupted foreground shell work is killed with `interrupt_reason=foreground_attach_unavailable_killed` and no managed job is forged. Remaining gap is true foreground shell attach/detach for a future attach-capable executor.
- Next slice: Implement or integrate an attach-capable executor without exposing process ids, host signals, or process handles to model-visible tools or transport callers.
- Evidence: `6e3287525` adds `InterruptSession`, `POST /sessions/{id}/interrupt`, `assistant.interrupted`, `operation.interrupted`, and tests proving provider-step interruption does not cancel an existing background job. `TestSubmitTurnDeliversAllTerminalJobObservationKinds` proves user-triggered continuation drains queued terminal observations without autonomous wakeup. `TestJobOutputSnapshotIsDurableButNotProviderObservation`, `TestManagedJobExecutorCanReportOutputSnapshot`, `TestManagedJobExecutorOutputSnapshotIsBounded`, `TestManagedJobExecutorCannotRedirectOutputSnapshotIdentity`, and `TestUITimelineFoldsDirectManagedJobEventsByJobID` prove `job.output` snapshots are bounded durable session/UI facts while remaining out of default provider observation delivery, kernel-bound to the originating job, and folded for direct shell transports. `TestLocalManagedJobExecutorEmitsSparseOutputSnapshot`, `TestManagedJobOutputCaptureDoesNotEmitEveryChunk`, `TestManagedJobOutputCaptureCapsDurableSnapshotsPerJob`, and `TestManagedJobOutputCaptureStopsAfterTruncatedSnapshot` prove the local executor reduces live stdout/stderr to sparse durable snapshots instead of persisting every transport chunk or allowing unbounded per-job progress persistence. `TestInterruptSessionDuringForegroundShellWritesInterruptedToolResult`, `TestLocalManagedJobExecutorDoesNotAdvertiseForegroundAttach`, and `TestForegroundInterruptReasonStaysKillFallbackUntilAttachIsImplemented` prove the current foreground interrupt path records the truthful kill fallback and does not forge managed-job attach facts.
- Verification: Remaining verification must prove any future attach-capable executor can convert interrupted foreground shell work into kernel-owned managed-job facts while keeping host process handles hidden behind executor semantics.
- Reference alignment: Aligned with Codex background terminal list/terminate control surfaces and Reasonix's `ProgressFunc` plus session-scoped job manager. The active drift risk is turning live progress into provider-owned context, UI-owned truth, or a strong audit log instead of a kernel-owned durable fact with separate projections.

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
