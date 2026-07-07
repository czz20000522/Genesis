# KERNEL-REFERENCE-HARDENING-CAMPAIGN-20260708

## Queue Metadata

- Lane: kernel
- Priority: P1
- Stage: ready-for-agent
- Owner: autonomous coding agent
- Branch: master
- Baseline commit: 5d1fa2e1d
- Stop rule: keep taking the next task in this package until the user interrupts, a required external authority is missing, or the remaining work is no longer supported by concrete Codex and Reasonix references.

## Goal

Continuously harden Genesis deterministic kernel infrastructure by comparing local Codex and Reasonix reference behavior, closing bounded production gaps, and only then adding reference-backed production capabilities.

## User Value

The user gets a greener, more reliable local-first agent kernel without having to supervise each low-level infrastructure slice. The campaign focuses on generic agent runtime foundations: sessions, permissions, tools, process control, provider boundaries, projections, recovery, and readiness.

## Reference Files

Genesis governing files:

- `AGENTS.md`
- `docs/process.md`
- `docs/project-brief.md`
- `docs/kernel-contract.md`
- `docs/requirements/kernel-foundation-capabilities.md`
- `docs/design/kernel-foundation-capabilities.md`
- `docs/operations/kernel-issues.md`
- `docs/operations/kernel-retirement-log.md`
- `docs/operations/task-package-template.md`

External local reference roots:

- `D:\software\JetBrains\python_workspace\codex-main`
- `D:\software\JetBrains\python_workspace\reasonix`

Reference scan requirement:

Each implementation slice must inspect concrete files in both external reference projects before changing Genesis code. The scan must identify the entrypoint, owner state transition, persisted record or event, projection, model-visible fields, error or retry semantics, and tests. A scan that only says the reference project has a similar concept is not enough.

## In Scope

- Build and maintain a compact reference inventory for the deterministic agent-kernel surfaces listed in this package.
- Add reference-inspired red tests before implementing non-trivial behavior changes.
- Prefer small, reversible fixes in existing Genesis ownership boundaries.
- Keep the event ledger as kernel truth and keep runtime transport chunks out of durable truth unless reduced to an owner-owned event, fact, audit record, or failure.
- Preserve model-visible schemas as semantic projections; do not expose kernel ids, credentials, permission profiles, sandbox profiles, checkpoints, or audit refs as model contract fields.
- Use Codex and Reasonix as references for control-plane behavior, not as source code to copy.
- Commit each verified slice with a Lore-format commit message.

## Out Of Scope

- Feishu, email, calendar, OCR, medical, insurance, or other application-specific capability logic.
- New memory architecture work, unless a later slice is explicitly opened after deterministic runtime hardening.
- Branch or worktree creation for this campaign; the user requested direct work on `master`.
- Remote, GitHub, release, or pull-request lookup for Genesis project truth.
- Compatibility readers, fallback loaders, migration shims, or cleanup paths for old local development state.
- Broad rewrites, dependency additions, or framework changes without a concrete reference-backed reason and explicit user request.

## Required Checks

Run the smallest focused checks that prove the slice, then run the baseline checks before committing:

```powershell
git diff --check
go test ./... -count=1
go build ./...
```

When a slice touches concurrency, process execution, cancellation, ledger replay, provider retries, or permission races, also run a focused race check such as:

```powershell
go test -race ./internal/kernel -run "<focused test pattern>" -count=1
```

## Execution Loop

For every task below:

1. Reopen the governing Genesis requirement, design, plan, issue, BDD feature, or retirement evidence that owns the surface.
2. Inspect concrete Codex paths and concrete Reasonix paths for comparable behavior.
3. Inspect the current Genesis implementation and tests.
4. Classify the result as `matches`, `gap`, `intentional difference`, or `reference risk rejected`.
5. For each bounded `gap`, write a failing test first.
6. Implement the smallest fix that closes the tested gap.
7. Update issue or retirement evidence only when the implementation state changes.
8. Run focused verification, then `git diff --check`, `go test ./... -count=1`, and `go build ./...`.
9. Commit the slice with Lore trailers and move to the next task.

## Task Queue

### Task 0: Baseline Fence

- Keep commit `5d1fa2e1d` as the campaign baseline.
- Confirm `git status --short --branch` is clean before the first campaign edit.
- Treat later unrelated user edits as outside the campaign unless they touch the active slice.

Acceptance evidence:

- Baseline commit exists.
- `git status --short --branch` reports clean `master`.

### Task 1: Reference Inventory

- Map concrete Codex files for sessions, tools, sandbox or approvals, provider calls, process control, and recovery.
- Map concrete Reasonix files for the same surfaces.
- Map the Genesis owner files and tests for each surface.
- Record the first pass inventory in this document under `Campaign Log`.

Acceptance evidence:

- Each mapped surface has at least one concrete Codex path, one concrete Reasonix path, and one Genesis path.
- The next implementation slice is selected from a documented gap, not from intuition.

### Task 2: Permission, Approval, And Sandbox Fail-Closed Behavior

- Compare how Codex and Reasonix decide whether a command, tool, patch, or privileged action can execute.
- Check Genesis permission modes, approval events, policy projections, and refusal behavior.
- Close bounded gaps where Genesis accepts ambiguous, unknown, or partially configured authority state.

Acceptance evidence:

- Unknown permission or sandbox states fail closed.
- Model-visible refusal fields stay semantic and path-free.
- Approval or denial truth is ledger-owned when durable.

### Task 3: Tool, Shell, Process, Job, Cancel, And Interrupt Control

- Compare reference behavior for tool admission, running process ownership, cancellation, descendant process handling, timeouts, and result projection.
- Check Genesis `shell_exec`, managed jobs, process tree cleanup, and event reduction.
- Close bounded gaps that affect deterministic execution or cleanup.

Acceptance evidence:

- Completed, failed, canceled, and timed-out executions have stable projections.
- Process cleanup does not depend on UI or application ownership.
- Output redaction and truncation semantics remain covered by tests.

### Task 4: Sessions, Turns, Replay, Idempotency, And Recovery

- Compare reference behavior for session creation, turn admission, replay, duplicate submissions, active-turn state, and recovery after partial failure.
- Check Genesis ledger replay and projection rebuild paths.
- Close bounded gaps where a crash, duplicate request, or replay changes semantic state incorrectly.

Acceptance evidence:

- Replayed state matches live state for the covered behavior.
- Duplicate or invalid turn admission cannot mint contradictory ledger facts.

### Task 5: Provider Boundary, Provider Command, Strict Responses, And Accounting

- Compare Codex and Reasonix provider abstraction, adapter command behavior, retry handling, model-visible tool calls, final response parsing, and usage accounting.
- Check Genesis provider profiles, gateway routes, provider_command protocol, local llama.cpp adapter, and OpenAI-compatible path.
- Close bounded gaps without adding provider-specific policy into the kernel.

Acceptance evidence:

- Provider failures are classified and sanitized.
- Usage accounting and cache fields are accepted when present and absent when unknown.
- Local and cloud-like providers share the same kernel-facing adapter contract.

### Task 6: Resource, Material, Redaction, And Model-Visible Payloads

- Compare reference behavior for reading workspace resources, exposing file or material content to the model, and preventing unsafe path or credential leaks.
- Check Genesis resource registry, material intake, context hydration, and capability projections.
- Close bounded gaps in safe projection and source-ref boundaries.

Acceptance evidence:

- Model-visible resource payloads are semantic and bounded.
- Kernel-owned refs, paths, credentials, and storage details remain hidden unless explicitly designed as user-visible facts.

### Task 7: Timeline, Audit, Debug, And Inspection Surfaces

- Compare reference behavior for user-facing transcript, debug trace, audit records, and internal state inspection.
- Check Genesis timeline, audit, session debug, capabilities, and readiness projections.
- Close bounded gaps where debug data is confused with durable truth or user-facing transcript.

Acceptance evidence:

- Audit remains reserved for authority, risk, credential, control-plane, security, and recovery-relevant records.
- Debug and inspection routes do not leak secret-shaped or path-shaped internals by default.

### Task 8: Config, Doctor, Startup, And Readiness

- Compare reference behavior for config validation, startup errors, doctor checks, missing dependency reports, and provider readiness.
- Check Genesis CLI setup, live acceptance scripts, daemon readiness, and desktop sidecar boundary.
- Close bounded gaps that make operator failure states ambiguous.

Acceptance evidence:

- Missing dependencies, invalid config, missing credentials, and provider auth failures have distinct, sanitized readiness outcomes.
- Desktop or application shells do not own kernel process semantics unless explicitly in their layer.

### Task 9: Reference-Backed Production Capability Queue

Begin only after Tasks 1-8 no longer expose obvious deterministic hardening gaps.

Candidate capabilities must have concrete Codex and Reasonix references before implementation:

- child-agent invocation surfaces and bounded capability grants;
- patch or file-edit tooling as a generic tool primitive;
- history or session search as a generic projection;
- manual provider model refresh and model-profile binding improvements;
- local runtime doctor checks for provider_command adapters.

Acceptance evidence:

- The capability has a production-grade requirement and design.
- Reference-inspired behavior tests exist before implementation.
- The implementation stays generic and kernel-owned.

## Escalation Criteria

Ask the user only when:

- a change would delete or rewrite user-authored work outside the active slice;
- a production capability requires a product-semantic decision not present in approved docs;
- a reference conflict cannot be resolved by existing Genesis owner boundaries;
- credentials, external logins, or network account actions are required;
- a dependency addition or broad rewrite becomes the only credible path.

## Campaign Log

### 2026-07-08 Baseline

- Baseline commit: `5d1fa2e1d`
- Verification before baseline commit:
  - `git diff --check`
  - `python scripts/providers/llama_cpp_provider_command.py --self-test`
  - `go test ./... -count=1`
  - `go build ./...`
- Clean baseline confirmed with `git status --short --branch`.

