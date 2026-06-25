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

### KERNEL-STATE-SEMANTICS-PROJECTION-DRIFT-20260625 - P1 - State vocabulary migration is incomplete across kernel and application projections

- Status: open.
- Area: Kernel Contract / Projection Governance / Application Runtime Boundaries.
- Requirement: `docs/kernel-contract.md`.
- Design: `docs/kernel-contract.md`.
- Gap: Context hydration already uses `admission_result=admitted|refused`, and kernel/provider/runtime/ledger readiness now use `readiness=ready|not_ready` plus `readiness_reason`. Remaining drift is in runtime/projection/application surfaces that still expose generic `status` words without consistently identifying the axis they belong to. Examples include `TurnProjection.Status`, UI/audit/context inspection read-model status fields, connector source lifecycle using `blocked/degraded/stopped`, and Code Intelligence readiness/query results using `blocked/degraded/cache_stale`. Some of these may remain valid as model-visible tool denial or adapter-local operator language, but each remaining use must be classified before being kept or renamed.
- Next slice: Migrate runtime activity and terminal outcome projections toward `phase + terminal_outcome + terminal_cause`, starting with turn/session projections and then UI/audit/context inspection read models. Preserve model-visible tool denial `blocked` only where it is explicitly a tool-result denial, not readiness or lifecycle. Application connector and Code Intelligence state terms should be split into follow-up application issues once the kernel-facing projection surface is classified. Do not add compatibility aliases during development.
- Evidence: `docs/kernel-contract.md` states that `accepted/rejected/blocked` are not cross-system status words and separates admission, readiness, validation, review, runtime terminal outcome, workflow state, and job state. The readiness slice now has `TestReadinessDTOsDoNotExposeGenericStatusTags`, `TestReadinessSurfacesUseReadinessAxis`, and `TestContextRuntimeReadinessDoesNotUseProviderStatus`, while `TestToolDenialMayStillUseBlockedAsModelVisibleOutcome` proves model-visible tool denial remains a separate axis. Current code still contains runtime/application projection status fields such as `TurnProjection.Status`, `UITimelineResponse.Status`, connector source lifecycle states, and Code Intelligence readiness/query results.
- Verification: Add the next state-contract test around turn/session projection axes before renaming those fields. Each owner migration must prove the same runtime condition is projected as the correct state axis with a reason/cause field, not as a generic status word. Confirm `go test ./internal/kernel -count=1` and affected application tests pass after each owner migration.
- Reference alignment: Reasonix and Codex both use typed event/projection families instead of one global status word; Reasonix can return a blocked tool result in plan mode, but that is a model-visible denial result, not a universal lifecycle/readiness state. Genesis should adopt the same split: tool denial may be observable, owner readiness and lifecycle should use explicit axes.
