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

### KERNEL-MATERIAL-SOURCE-SNAPSHOT-BUDGET-LEASE-20260626 - P1 - Source snapshot intake budgets must be configurable and pressure-sized

- Status: open.
- Requirement: `docs/requirements/kernel-material-source-snapshot.md`; related
  budget/readiness contract in `docs/requirements/kernel-foundation-capabilities.md`.
- Design: `docs/design/kernel-material-source-snapshot.md` and the runtime
  limit inspection design in `docs/design/kernel-readiness-inspection.md`.
- Title: Source snapshot intake budgets must be configurable and pressure-sized.
- Problem: The first real pressure package,
  `D:\User\Desktop\pleiades-anisop.zip`, is rejected by `/materials/intake`
  with `source_total_size_exceeded`. The current source archive limits are
  hardcoded in `internal/kernel/resource/types.go` as a 512-file limit, 512 KiB
  per file, and 1 MiB total uncompressed budget. That is acceptable for unit
  tests but too small for realistic scientific/operator code packages and is
  not exposed as a runtime BudgetLease/limit policy.
- Suggestion: Move source intake/read budgets into an owner-owned runtime limit
  policy that is configurable and inspectable through capabilities/readiness.
  Keep bounded per-read output for model context, but do not reject an entire
  ordinary code archive simply because its total uncompressed text exceeds a
  tiny test-sized constant. The fix should choose conservative production
  defaults, allow operator override, and keep red tests for oversized/archive
  bomb refusal.
- Evidence: Local smoke on 2026-06-26 started `genesisd` with the real
  `genesis-config` provider and confirmed `/ready` was `ready` with
  `openai-compatible`. Posting the user target package to `/materials/intake`
  returned HTTP 400:
  `refusal_reason_class=source_total_size_exceeded`,
  `message=source archive exceeds total uncompressed size budget`. The zip file
  itself is about 668 KB compressed, so the current uncompressed-total cap blocks
  the intended source snapshot pressure path before any LLM/tool behavior can be
  tested.
- Verification: Add a fixture representing a realistic small code package whose
  compressed size is under 1 MiB but uncompressed text exceeds the old 1 MiB
  constant; prove it can be admitted under the default production source budget
  while individual `source_read` calls remain bounded. Also keep negative tests
  for zip bombs, extreme file counts, oversized single files, and explicit low
  budget overrides. `/capabilities` or an equivalent inspection surface must
  show the effective source intake/read limits.
- Priority: P1.
- Reference alignment: Codex and Reasonix keep file/resource reads bounded at
  the operation/projection boundary, but their production file surfaces are not
  governed by hidden test-sized constants that reject ordinary project-scale
  materials. This should follow the same BudgetLease direction already used for
  model/tool-loop limits.

### KERNEL-SOURCE-SNAPSHOT-PURE-READ-RACE-20260626 - P1 - Source snapshot tools must be truly read-only before parallel execution

- Status: open.
- Requirement: obvious implementation drift against
  `docs/requirements/kernel-material-source-snapshot.md` and the tool
  scheduling contract in `docs/requirements/kernel-tool-scheduling-concurrency.md`.
- Design: `docs/design/kernel-material-source-snapshot.md` and
  `docs/design/kernel-tool-scheduling-concurrency.md`.
- Title: Source snapshot tools must be truly read-only before parallel
  execution.
- Problem: `source_tree` and `source_read` are registered as trusted
  `pure_read` tools using compatible-lock scheduling, so the tool runtime may
  execute multiple calls in the same provider step concurrently. The source
  snapshot registry is backed by plain maps, and `SourceTree` still mutates
  `sourceFiles` while producing a read result. That makes the current
  implementation neither an immutable read model nor a concurrency-safe owner.
- Suggestion: Make source snapshot admission build all source file handles at
  intake time and make `SourceTree` a pure projection that does not mutate
  registry state; or, if the owner still needs lazy mutation, classify
  `source_tree` / `source_read` as state-read/serial until the registry has an
  explicit synchronization and replay contract. Add an anti-regression test that
  concurrent source pure-read batches cannot mutate shared maps or race.
- Evidence: `internal/kernel/tool_registry.go` registers `source_tree` and
  `source_read` with `resourceReadToolSchedulingSpec()`. `toolruntime` batches
  trusted compatible pure reads into a parallel batch, and
  `internal/kernel/tool_execution.go` runs those calls in goroutines.
  `internal/kernel/resource/source_snapshot.go` writes to `r.sourceFiles` inside
  `SourceTree`, while `AdmitSourceRead` and `SourceRead` read the same map.
  Focused tests and `go test ./... -count=1` pass, but they do not run a
  concurrent source snapshot race/semantic guard.
- Verification: Add a red test that prepares a provider batch containing
  multiple source snapshot read calls and proves either (a) the calls execute
  safely with no registry mutation during tool execution, preferably under
  `go test -race` for the focused package, or (b) the planner keeps source
  tools serial until the owner is immutable/concurrency-safe. Also assert that
  `source_file_ref` generation remains scoped to the source snapshot ref in both
  tree and read paths.
- Priority: P1.
- Reference alignment: Reasonix resolves file refs through controller/runtime
  ownership before the turn and uses tool reads as bounded observations; Codex
  filesystem/image handlers go through runtime file boundaries. Neither model
  implies a read-only tool should lazily mutate shared owner state while being
  advertised as safe for concurrent execution.

### KERNEL-MATERIAL-SOURCE-SNAPSHOT-DURABLE-OWNER-FACT-20260626 - P2 - Material intake needs a durable owner fact before resume claims

- Status: open.
- Requirement: `docs/requirements/kernel-material-source-snapshot.md`; related
  persistence gate in `docs/design/kernel-persistence-store.md`.
- Design: `docs/design/kernel-material-source-snapshot.md`.
- Title: Material intake needs a durable owner fact before resume claims.
- Problem: `IntakeMaterial` and `IntakeUploadedMaterial` currently register
  source snapshots only in the in-memory resource registry. Uploaded bytes are
  written to `material-store`, but no durable owner fact or index ties the
  generated `source_snapshot_ref` back to the stored upload body, display label,
  purpose, retention, or session scope. After daemon restart, a previously
  visible `source_snapshot_ref` cannot be admitted by `source_tree` /
  `source_read`, and uploaded bodies become unreachable except as raw files.
- Suggestion: Either explicitly mark the current material/source snapshot slice
  as process-lifetime only in readiness/capabilities, or add the minimal durable
  owner fact needed to recover admitted source snapshots. The durable record must
  still obey the persistence promotion gate: store only the owner fact and
  internal storage ref needed for recovery/retention, not full archive contents
  or host-path data in transcript/provider context.
- Evidence: `internal/kernel/material_intake.go` writes upload bodies to a file
  store and calls `RegisterLocalZipSnapshot`, but it does not append a material
  intake event or persist a source owner index. `Kernel.New` initializes
  `resource.NewRegistry(config.Resources)` and does not reload source snapshots
  from ledger/material store. `turn.submitted` may record a model-visible source
  descriptor for that turn, but it does not contain the private resolver state
  needed for future admission after restart.
- Verification: Add a restart/resume red test: upload or intake a zip, obtain a
  `source_snapshot_ref`, construct a fresh kernel with the same ledger and
  material store, then prove the current behavior is either explicitly
  `not_ready/process_lifetime_only` or successfully recovers enough owner state
  for `source_tree` and `source_read` to work without leaking host path or
  storage ref to provider context.
- Priority: P2.
- Reference alignment: Codex separates durable handles from backing storage and
  Reasonix resumes from settled transcript/checkpoint rather than live UI state.
  Genesis can stay minimal, but if uploaded material is advertised as a usable
  session resource, the owner must either recover it or truthfully report that
  the current source snapshot is process-lifetime only.

### KERNEL-PROVIDER-FAKE-PRODUCTION-GUARD-20260626 - P0 - Fake provider must not be a production-ready provider

- Status: open.
- Requirement: obvious production-safety bug; related production target in
  `docs/requirements/kernel-foundation-capabilities.md` and live provider
  acceptance in `docs/operations/live-llm-first-run-acceptance.md`.
- Design: no separate design needed before the first fix; this is a provider
  readiness/admission hardening issue.
- Gap: `cmd/genesisd` currently maps `-provider fake` and an explicitly empty
  provider name to `kernel.FakeProvider{}`. A running `genesisd` then reports the
  fake provider as `ready`. Fake provider is valid only as a deterministic
  lab/test fixture for proving HTTP, ledger, session, projection, and tool-loop
  plumbing. In a production or user-facing daemon, fake provider readiness would
  be a severe misconfiguration because it can make the system look usable while
  no real model is connected.
- Next slice: Make fake provider opt-in as lab/test mode rather than a normal
  production provider. Production-facing startup should prefer `genesis-config`
  and should not silently fall back to fake when provider config is empty,
  missing, invalid, or credential-blocked. A fake provider, if explicitly allowed
  for local smoke tests, must be visibly marked as lab-only in readiness and must
  be rejected by live-provider acceptance gates.
- Evidence: Pressure smoke on 2026-06-26 verified that a fake `genesisd` can
  return `provider.readiness=ready` and complete multi-turn HTTP requests. The
  same smoke verified that `openai-compatible` without `GENESIS_PROVIDER_API_KEY`
  already returns structured `/ready` state:
  `readiness=not_ready`, `readiness_reason=provider_not_ready`,
  `provider.readiness=not_ready`, and
  `provider.readiness_reason=provider_api_key_missing`. The issue is therefore
  not missing structured not-ready behavior for real provider credentials; the
  issue is that fake can still be admitted as a ready provider on the daemon
  production surface.
- Verification: Add red tests proving `genesisd` does not treat fake as a
  production-ready provider by default, an explicitly empty provider name does
  not select fake, missing real-provider credentials remain structured
  `not_ready`, and live-provider acceptance fails if `/ready` or `/turn` uses a
  fake provider. Keep existing deterministic unit tests able to inject
  `FakeProvider` directly as a test fixture without changing kernel semantics.
- Priority: P0.
- Reference alignment: Codex and Reasonix use fake/stub model paths for tests
  and local deterministic harnesses, but production-facing runtime readiness is
  not allowed to present a fake model backend as an ordinary connected provider.
