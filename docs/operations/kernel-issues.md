# Kernel Issue Ledger

This file is the repo-owned ledger for active Genesis Kernel issues. Feishu Base is the collaboration inbox; this file is the durable project source for issues that still need code, verification, or user acceptance.

Retired issues must not remain here. Move accepted retirements to `docs/operations/kernel-retirement-log.md` as compact evidence: one sentence with the retirement conclusion plus the fixing commit evidence.

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

### KERNEL-TOOL-SCHEDULING-CONCURRENCY-20260624 - P2 - Tool scheduling must use effect, footprint, and handle policy

- Status: open.
- Area: Tool Runtime / Model Gateway / Work Registry.
- Requirement: `docs/requirements/kernel-foundation-capabilities.md`.
- Design: `docs/design/kernel-foundation-capabilities.md`.
- Gap: The current tool runtime now has a pure kernel planner that maps trusted access plans into deterministic execution batches and keeps current execution sequential inside each batch. This proves the boundary without adding executor-pool concurrency. Remaining production gap is real concurrent execution for eligible batches, broader registered-tool scheduling metadata, resource lock/idempotency hardening for future tools, and replay guarantees for admitted non-idempotent or external effects.
- Next slice: Keep `shell_exec` serial and only add real executor parallelism after an eligible non-shell read tool or resource primitive exists. The next implementation must preserve provider-order tool result projection and must not let provider flags or model arguments select scheduling metadata.
- Evidence: Genesis records `shell_exec` as an effectful serial fence, classifies `job_status` / `job_cancel` as per-handle `process_io`, and plans synthetic compatible pure reads into one parallel batch while leaving unknown metadata, state reads, writes, same-handle job I/O, and `process_start` admission serial. The turn loop now consumes planner batches but still executes each call in provider order, so runtime behavior remains conservative. Local reference review shows Reasonix parallelizes only contiguous known read-only tools and excludes evidence-ledger readers such as `complete_step` and `todo_write`; Codex relies on handler `supports_parallel_tool_calls()` plus process/session registries, but its shell parallelism is intentionally not adopted for Genesis because arbitrary shell commands cannot be trusted as pure reads.
- Verification: Current verification proves compatible pure reads can be planned into one parallel batch; read/write/read does not cross the write fence; unknown tools and unknown scheduling metadata are serial; state reads wait for prior committed facts; same-handle process I/O is serial while different handles can be planned together; `process_start` admission is serial; `shell_exec` is not pure-read by command text inspection; scheduling metadata stays out of the model-visible tool manifest. Future verification must prove real executor concurrency preserves ledger, transcript, checkpoint, and provider replay order, and crash/replay does not repeat admitted non-idempotent or external effects.
- Reference alignment: Aligned with Reasonix's provider-ordered tool dispatch plus limited read-only parallel batches, and with Codex's handler-declared parallel support plus process/session conflict guards. The intentional divergence is that Genesis does not copy Codex shell parallelism: `shell_exec` remains effectful/serial unless a future hard read-only sandbox or narrower registered read tool provides a trusted access plan.

### KERNEL-TEST-ARTIFACT-LOCALITY-20260624 - P2 - Kernel tests still write fixtures to system temp

- Status: open.
- Area: Test Governance / Developer Operations.
- Requirement: `docs/process.md` Test Artifact Gate.
- Design: Exception: this is a test governance gap against the approved process rule, not a runtime feature design.
- Gap: Application connector tests now use `testsupport.ProjectTempDir` and are guarded against new `t.TempDir()` usage, but kernel and kernel-command tests still write ledgers, workspaces, credential roots, and config roots through Go's system temp directory. On Windows this can land on `C:\` or user temp, violating the project-local artifact rule and leaving cleanup outside the repo-owned scratch area.
- Next slice: Migrate `internal/kernel`, `cmd/genesisd`, and `cmd/genesisctl` tests to project-local test artifact helpers, then add a long-lived guard that blocks new system-temp fixture usage in kernel/application tests. Keep the guard structural; do not assert subjective logs or prose.
- Evidence: `rg -n 't\\.TempDir\\(' -g '*_test.go'` still returns many hits under `internal/kernel`, `cmd/genesisd`, and `cmd/genesisctl` after the application-line migration.
- Verification: Future verification must prove writable test fixtures land under project `.test-tmp/`, are cleaned on test completion or periodic retention cleanup, and no automated test writes ledgers, connector state, credential/config fixtures, or binaries to `C:\`, user temp, or global tool directories.
- Reference alignment: Aligned with Codex-style repo-local test isolation and Reasonix-style harness-local fixture ownership. The active drift risk is treating OS temp as harmless developer scratch while the project requires local-first, repo-owned cleanup and bounded artifact retention.

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
- Gap: The current foundation separates `permission_mode`, `authority_policy`, `sandbox_profile`, and `approval_policy`; it now fails closed when a configured stronger sandbox profile is unavailable and blocks write tools with structured `approval_required` feedback when `approval_policy=on_request`. Remaining gap is real OS-level workspace sandbox enforcement and an interactive approval owner that can approve or deny requests without shell/UI-local authority logic.
- Next slice: Implement an actual sandbox enforcement adapter or an approval owner command path only through kernel-owned profile resolution and typed control-plane events. Do not let tool arguments, shell transports, or UI state select permission mode, sandbox profile, approval policy, or approval outcome.
- Evidence: Current docs and tests state `controlled_workspace` is not an OS sandbox and provider-visible tool results must not include permission/profile control-plane fields. `TestSubmitTurnBlocksUnavailableSandboxProfileBeforeExecution` proves `os_workspace` fails closed without file effects when unavailable. `TestSubmitTurnBlocksReadOnlySandboxOverrideBeforeExecution` proves a configured `read_only` sandbox profile cannot be recorded while default/yolo silently execute through host shell. `TestSubmitTurnBlocksApprovalRequiredBeforeExecution` proves `on_request` write tools return structured approval feedback without execution. `TestSubmitTurnPlanOnRequestKeepsReadOnlyDenialBeforeApproval` proves plan mode remains a hard read-only denial rather than becoming an approval path. `TestArchitectureBoundarySandboxProfileCannotBroadenPermissionMode` proves profile overrides cannot broaden or misrepresent executable profiles. `TestArchitectureBoundaryApprovalOnRequestBlocksWriteToolsAtAdmission` proves approval gating happens at admission while read tools remain allowed.
- Verification: Existing positive contracts remain true; future verification must prove real sandbox denial cannot silently degrade to host execution, approval denial returns structured feedback without execution, approval approval creates typed kernel evidence before execution, and model-supplied control-plane fields remain rejected as repairable invalid requests.
- Reference alignment: Aligned with Codex's sandbox/approval split, where approval policy is turn/control-plane state and sandbox permissions are execution-layer enforcement, and with Reasonix's separation of permission gate, interactive approval, and sandbox wrapper. The active drift risk is over-promising `default` as a real OS sandbox or turning approval into shell/UI-local logic.
