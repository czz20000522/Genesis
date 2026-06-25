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
- Reference scans must translate relevant Codex/Reasonix behavior tests into Genesis same-semantics red tests or explicitly reject them with a reason. A prose-only reference note is not enough for kernel primitives.
- Issues record the current gap between approved requirements/designs and the implementation. They must not carry raw requirements, design discussion, or the full production acceptance contract.
- Every active issue must cite an approved requirement and design unless it is an obvious bug or test gap. If an issue uses that exception, state the exception explicitly.
- Prefer `Gap`, `Next slice`, `Evidence`, and `Verification` over broad `Problem` or `Suggestion` text when adding new issues.
- Do not use issues as debug logs. Routine info, stream chunks, repeated status polling, and exploratory notes stay out of this ledger unless they identify a current implementation gap.
- When an issue removes a concept from the current kernel contract, long-term tests must assert the positive replacement behavior. Do not keep permanent tests whose only purpose is locking retired names, aliases, routes, or historical helper APIs out of the tree; use temporary scans or retirement-log evidence for cleanup windows, then fold the guard into the current owner contract.
- Development artifacts and historical local data are not compatibility obligations. Do not create or keep issues whose only purpose is migrating, cleaning, importing, or preserving old local generated state unless that state is part of the approved current kernel contract.
- Every implementation slice must finish with a drift check against the governing requirement, design, implementation plan, issue, and BDD feature. In-scope drift is fixed before commit. Out-of-scope drift is recorded here as an active issue with evidence and next slice before commit.
- Periodic governance review checks architecture, feature behavior, directory structure, and document lifetime together. Completed plans and stale documents should be deleted or condensed instead of spawning issues that only preserve old notes.

## Active Issues

### KERNEL-REFERENCE-BEHAVIOR-RED-TEST-MATRIX-20260625 - P1 - Codex/Reasonix kernel behavior tests must be translated into Genesis red-test gates

- Status: open.
- Area: Architecture Governance / Test Governance / Kernel Foundation.
- Requirement: `docs/requirements/kernel-owner-structure-governance.md`.
- Design: `docs/design/kernel-owner-structure-governance.md`.
- Gap: Active issues require a Codex/Reasonix reference alignment field, but the
  implementation process does not yet require those reference findings to become
  Genesis same-semantics red tests. This allowed review to miss production-class
  gaps that the reference projects already encode as behavior: runtime budgets
  as configured harness limits rather than hidden constants, parallel execution
  signals matching actual executor behavior, typed internal protocols rejecting
  drift, approval/replay crash windows, provider context projection boundaries,
  and tool/result taxonomy. A prose reference scan can say "aligned" while no
  failing Genesis test proves the equivalent behavior.
- Next slice: Add a lightweight reference behavior test matrix requirement to
  the kernel implementation workflow. For each non-trivial kernel issue or
  production slice, the implementation plan must list: reference project/file or
  test behavior inspected, Genesis semantic equivalent, intended Genesis test
  file, initial red condition, accepted intentional differences, and remaining
  drift risk. The matrix should be small and local to the slice; do not copy
  upstream test suites wholesale and do not create permanent tests that only
  assert upstream names. The first pass should backfill current active kernel
  areas: BudgetLease/tool-loop control, provider_command strict protocol shape,
  tool scheduling/parallel execution, approval replay, provider context
  projection, resource_read/context hydration, managed job observation, and UI
  timeline/detail projection.
- Evidence: `KERNEL-BUDGET-LEASE-20260625` was found only after user challenge
  despite Reasonix exposing `agent.max_steps` / `planner_max_steps` as
  configured harness-loop limits and Codex separating output caps from runtime
  execution authority. `KERNEL-TOOL-BATCH-PARALLEL-SIGNAL-20260625` was found
  only after comparing the implemented planner/executor split with Reasonix's
  read-only parallel-dispatch semantics and Codex handler runtime support. The
  existing ledger rule requires reference alignment prose, but not a red-test
  translation table or proof that the reference behavior has a Genesis
  equivalent.
- Verification: Add or update the owner-structure/test-governance requirement
  and design so every non-trivial kernel implementation plan must include a
  `Reference behavior red tests` section. Add a small contract/governance test or
  review script that checks active implementation plans for this section when
  they claim Codex/Reasonix reference alignment. Backfill the section for current
  active implementation plans without blocking builds on subjective prose. For at
  least BudgetLease, provider_command strict response, and tool batch parallel
  signal, ensure the production fix lands with failing-first behavior tests that
  directly encode the translated reference semantics.
- Reference alignment: Reasonix and Codex do not rely on architectural prose
  alone for core agent/kernel behavior; their confidence comes from behavior
  suites around max steps, read-only parallel dispatch, approval/sandbox
  boundaries, provider/tool protocol translation, event projection, and replay.
  Genesis should translate those semantics into its own owner vocabulary instead
  of treating local review memory as the gate.

### KERNEL-JOB-PROGRESS-IDLE-CONTINUATION-20260623 - P2 - Local managed job streaming and attach capability

- Status: open.
- Area: Tool Runtime / Interface Kernel / Model Gateway.
- Requirement: `docs/requirements/kernel-shell-and-job-control.md`.
- Design: `docs/design/kernel-shell-and-job-control.md`.
- Gap: Kernel now has the first `job.output` snapshot contract, the local managed shell executor emits bounded sparse live output snapshots, and foreground shell interruption is capability-gated. The local executor explicitly does not advertise foreground attach support, and `7c41de5e8` now requires any future foreground-attach capability to be backed by a typed attach method rather than metadata alone. Interrupted foreground shell work is still killed with `interrupt_reason=foreground_attach_unavailable_killed` and no managed job is forged. Remaining gap is true foreground shell attach/detach implementation through a future attach-capable executor.
- Next slice: Implement or integrate an attach-capable executor without exposing process ids, host signals, or process handles to model-visible tools or transport callers.
- Evidence: `6e3287525` adds `InterruptSession`, `POST /sessions/{id}/interrupt`, `assistant.interrupted`, `operation.interrupted`, and tests proving provider-step interruption does not cancel an existing background job. `docs/requirements/kernel-shell-and-job-control.md` and `docs/design/kernel-shell-and-job-control.md` now define foreground attach as an executor capability, require kernel-bound managed-job identity, and forbid exposing host process ids, signals, or handles to model/HTTP callers. `7c41de5e8` adds `ManagedJobForegroundAttachRequest` / `ManagedJobForegroundAttachResult` and `TestForegroundAttachCapabilityRequiresAttachMethod`, proving an executor cannot become attach-capable by advertising a capability bit without an attach method. `TestSubmitTurnDeliversAllTerminalJobObservationKinds` proves user-triggered continuation drains queued terminal observations without autonomous wakeup. `TestJobOutputSnapshotIsDurableButNotProviderObservation`, `TestManagedJobExecutorCanReportOutputSnapshot`, `TestManagedJobExecutorOutputSnapshotIsBounded`, `TestManagedJobExecutorCannotRedirectOutputSnapshotIdentity`, and `TestUITimelineFoldsDirectManagedJobEventsByJobID` prove `job.output` snapshots are bounded durable session/UI facts while remaining out of default provider observation delivery, kernel-bound to the originating job, and folded for direct shell transports. `TestLocalManagedJobExecutorEmitsSparseOutputSnapshot`, `TestManagedJobOutputCaptureDoesNotEmitEveryChunk`, `TestManagedJobOutputCaptureCapsDurableSnapshotsPerJob`, and `TestManagedJobOutputCaptureStopsAfterTruncatedSnapshot` prove the local executor reduces live stdout/stderr to sparse durable snapshots instead of persisting every transport chunk or allowing unbounded per-job progress persistence. `TestInterruptSessionDuringForegroundShellWritesInterruptedToolResult`, `TestLocalManagedJobExecutorDoesNotAdvertiseForegroundAttach`, and `TestForegroundInterruptReasonStaysKillFallbackUntilAttachIsImplemented` prove the current foreground interrupt path records the truthful kill fallback and does not forge managed-job attach facts.
- Verification: Remaining verification must prove any future attach-capable executor can convert interrupted foreground shell work into kernel-owned managed-job facts while keeping host process handles hidden behind executor semantics.
- Reference alignment: Aligned with Codex background terminal list/terminate control surfaces and Reasonix's `ProgressFunc` plus session-scoped job manager. The active drift risk is turning live progress into provider-owned context, UI-owned truth, or a strong audit log instead of a kernel-owned durable fact with separate projections.

### KERNEL-CONTEXT-RESOURCE-HYDRATION-20260625 - P2 - Full skill and long-context hydration must use generic resource/context handles

- Status: open.
- Area: Model Gateway / Resource Owner / Skill Catalog.
- Requirement: `docs/requirements/kernel-resource-read.md`.
- Design: `docs/design/kernel-resource-read.md`.
- Gap: Genesis currently has a path-free metadata-only skill index and a bounded `resource_read` primitive, but it does not yet have a production path for admitting full skill bodies, connector attachment text, or long application instructions into provider context as generic hydrated context. The current absence is intentional; the remaining gap is a generic resource/context owner flow that can admit bounded content, record derivation evidence, and let the Model Gateway render typed context fragments without reintroducing `skill.read` or caller-built prompt splicing.
- Next slice: Implement the generic context-hydration admission fact/store and provider-context projection only after the resource/context owner storage shape is selected. The implementation must keep full skill bodies out of default context, capabilities, timeline, and context inspection unless a bounded generic hydrated context handle was admitted for the turn/task.
- Evidence: `419559d9d` adds the Phase E admission owner contract to `docs/requirements/kernel-resource-read.md` and `docs/design/kernel-resource-read.md`, defining `context.admit_resource`, `context.hydration.accepted`, and `context.hydration.rejected` as generic resource/context owner facts rather than skill-specific tools or caller-built prompt fragments. The same commit adds `TestArchitectureBoundaryNoSkillSpecificHydrationTools`, proving the model-visible tool manifest cannot grow `skill.read`, `read_skill`, or skill-body surfaces. Current tests such as `TestKernelInjectsBudgetedSkillIndexWithoutSkillBodies`, `TestSubmitTurnProjectsRegisteredToolManifestWithoutSkillCatalogContext`, and `TestTurnEvidenceRecordsModelInputKindsWithoutSkillPaths` prove the default provider context excludes skill bodies and paths.
- Verification: Future verification must prove full skill bodies are absent by default; a hydrated body appears only through a generic resource/context handle with typed model input evidence, bounded output, derivation refs, and no filesystem path or package root leakage; `skill.read`, `read_skill`, and skill-specific kernel tools remain absent from the model-visible tool manifest.
- Reference alignment: Aligned with Codex's split between bounded model-visible context fragments and skill instruction injection when selected, while intentionally avoiding a Genesis package-specific skill-read tool. Aligned with Reasonix's explicit `@resource`/MCP resource reads as on-demand context rather than always-on prompt content. The active drift risk is treating skill packages as kernel APIs or letting shells/connectors assemble provider context directly.

### KERNEL-TEST-SURFACE-OWNER-SPLIT-20260625 - P3 - Kernel behavior tests still centralize multiple owner surfaces in one file

- Status: open.
- Area: Architecture Governance / Test Governance.
- Requirement: `docs/requirements/kernel-owner-structure-governance.md`.
- Design: `docs/design/kernel-owner-structure-governance.md`.
- Gap: Production code has started moving owner replay, DTOs, HTTP transport, timeline, resource, job, and tool scheduling behavior into named files, but the main behavior test surface still keeps many unrelated owner contracts in `internal/kernel/kernel_test.go`. This is not a line-count issue; the risk is that future changes to memory, work, provider, shell, job, tool-loop, HTTP, and projection contracts keep landing in one central test file, making owner drift harder to review and making it easier to treat `Kernel` as the only testable authority surface.
- Next slice: When an owner/topic is next touched, move the affected tests and local helpers into owner/topic files such as memory, work, shell/job, provider gateway, tool loop, HTTP transport, projection, or readiness tests. Do this incrementally with no runtime behavior change, no compatibility helpers, and no arbitrary file-size threshold. Keep cross-owner pressure scenarios in dedicated pressure tests instead of the central kernel behavior file.
- Evidence: `internal/kernel/kernel_test.go` currently contains 153 tests and about 347 KB of mixed contracts, including submit-turn, HTTP, shell execution, WorkRegistry, memory review/recall/supersede, provider command, OpenAI-compatible tool loop, permission/sandbox/approval, managed jobs, job control, truncation, and repair feedback. Local Codex reference tests are large but organized as topic suites such as `compact.rs`, `approvals.rs`, `unified_exec.rs`, and `realtime_conversation.rs`; local Reasonix tests are similarly owner/topic oriented under `internal/agent`, `internal/tool`, `internal/provider`, `internal/acp`, and UI packages.
- Verification: Future verification should prove moved tests still pass with `go test ./internal/kernel -count=1` and `go test ./... -count=1`, and a governance review should confirm no new owner/topic cluster was added to `kernel_test.go` when a more specific test file exists. This issue must not introduce permanent tests that only assert line counts or subjective organization.
- Reference alignment: Aligned with Codex and Reasonix using owner/topic-oriented test suites while rejecting arbitrary size caps. The active drift risk is letting the test suite preserve the same central-owner shape that production code is trying to retire.
