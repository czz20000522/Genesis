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

### 2026-07-08 Task 1 Reference Inventory

Inventory rule used for this pass: each surface below has at least one concrete Codex path, one concrete Reasonix path, and one Genesis owner path. Later implementation slices must reopen the exact files relevant to the slice and inspect behavior, not rely only on this table.

| Surface | Codex reference | Reasonix reference | Genesis owner |
| --- | --- | --- | --- |
| Sessions, turns, replay | `codex-rs/core/src/session/session.rs`, `codex-rs/core/src/session/turn.rs`, `codex-rs/core/src/state/session.rs`, `codex-rs/core/src/state/turn.rs`, `codex-rs/core/tests/suite/turn_state.rs` | `internal/agent/session.go`, `internal/agent/agent.go`, `internal/control/controller.go`, `internal/agent/session_test.go`, `internal/control/controller_test.go` | `internal/kernel/kernel.go`, `internal/kernel/http_turn.go`, `internal/kernel/turn_interrupt.go`, `internal/kernel/session_projection.go`, `internal/kernel/http_transport_test.go` |
| Tool registry and dispatch | `codex-rs/core/src/tools/registry.rs`, `codex-rs/core/src/tools/router.rs`, `codex-rs/core/src/tools/orchestrator.rs`, `codex-rs/core/tests/suite/tools.rs`, `codex-rs/core/tests/suite/tool_parallelism.rs` | `internal/tool/tool.go`, `internal/agent/agent.go`, `internal/tool/registry_test.go`, `internal/tool/registry_canon_test.go` | `internal/kernel/tool_registry.go`, `internal/kernel/model_tools.go`, `internal/kernel/tool_execution.go`, `internal/kernel/tool_scheduling.go`, `internal/kernel/tool_loop_integration_test.go`, `internal/kernel/tool_execution_test.go` |
| Permission, approval, sandbox | `codex-rs/core/src/config/permissions.rs`, `codex-rs/core/src/config/resolved_permission_profile.rs`, `codex-rs/core/src/exec_policy.rs`, `codex-rs/core/src/tools/sandboxing.rs`, `codex-rs/core/tests/suite/approvals.rs`, `codex-rs/core/tests/suite/request_permissions_tool.rs` | `internal/permission/permission.go`, `internal/permission/bash_readonly.go`, `internal/sandbox/sandbox.go`, `internal/control/controller.go`, `internal/control/approval_e2e_test.go` | `internal/kernel/authority_gate.go`, `internal/kernel/approval.go`, `internal/kernel/http_approvals.go`, `internal/kernel/controlled_shell.go`, `internal/kernel/approval_owner_test.go`, `internal/kernel/tool_loop_integration_test.go` |
| Shell and process control | `codex-rs/core/src/shell.rs`, `codex-rs/core/src/exec.rs`, `codex-rs/core/src/unified_exec/process_manager.rs`, `codex-rs/core/src/tools/handlers/shell.rs`, `codex-rs/core/tests/suite/unified_exec.rs`, `codex-rs/core/tests/suite/shell_command.rs` | `internal/sandbox/shell.go`, `internal/tool/builtin/bash.go`, `internal/tool/builtin/bgjobs.go`, `internal/control/controller.go`, `internal/control/shell_test.go`, `internal/tool/builtin/bash_powershell_test.go` | `internal/kernel/process_runtime.go`, `internal/kernel/managed_job_executor.go`, `internal/kernel/shell.go`, `internal/kernel/jobs.go`, `internal/kernel/shell_process_tree_test.go`, `internal/kernel/job_control_test.go` |
| Provider boundary and retries | `codex-rs/core/src/client.rs`, `codex-rs/core/src/client_common.rs`, `codex-rs/core/src/responses_retry.rs`, `codex-rs/core/tests/suite/client.rs`, `codex-rs/core/tests/suite/remote_models.rs` | `internal/provider/provider.go`, `internal/provider/retry.go`, `internal/provider/openai/openai.go`, `internal/provider/openai/fetch_models.go`, `internal/config/ccswitch.go`, `internal/provider/provider_test.go` | `internal/kernel/provider.go`, `internal/kernel/openai_compatible.go`, `internal/kernel/provider_command.go`, `internal/kernel/provider_resilience.go`, `internal/kernel/model_config.go`, `internal/kernel/provider_gateway_test.go`, `internal/kernel/provider_command_test.go` |
| Resource and material projection | `codex-rs/core/src/tools/handlers/mcp_resource.rs`, `codex-rs/core/src/tools/handlers/mcp_resource_spec.rs`, `codex-rs/core/src/context/available_skills_instructions.rs`, `codex-rs/core/tests/suite/search_tool.rs`, `codex-rs/core/tests/suite/rmcp_client.rs` | `internal/tool/builtin/readfile.go`, `internal/tool/builtin/workspace.go`, `internal/tool/builtin/confine.go`, `internal/skill/tools.go`, `internal/tool/builtin/readfile_window_test.go` | `internal/kernel/resource/registry.go`, `internal/kernel/resource/source_snapshot.go`, `internal/kernel/material_intake.go`, `internal/kernel/model_tools.go`, `internal/kernel/resource_read_test.go`, `internal/kernel/source_tools_test.go`, `internal/kernel/http_materials_test.go` |
| Timeline, audit, debug, inspection | `codex-rs/core/src/event_mapping.rs`, `codex-rs/core/src/tools/events.rs`, `codex-rs/core/src/prompt_debug.rs`, `codex-rs/core/tests/suite/model_visible_layout.rs`, `codex-rs/core/tests/suite/truncation.rs` | `internal/event/event.go`, `internal/event/sync.go`, `internal/agent/textsink.go`, `internal/agent/evidence_flow_test.go`, `internal/agent/final_readiness_test.go` | `internal/kernel/projections.go`, `internal/kernel/ui_timeline_projection.go`, `internal/kernel/session_debug.go`, `internal/kernel/http_inspection.go`, `internal/kernel/capabilities.go`, `internal/kernel/projection_shape_test.go`, `internal/kernel/timeline_projection_test.go` |
| Config, readiness, models | `codex-rs/core/src/config/mod.rs`, `codex-rs/core/src/config/schema.rs`, `codex-rs/core/src/config/config_loader_tests.rs`, `codex-rs/core/tests/suite/models_cache_ttl.rs`, `codex-rs/core/tests/suite/remote_models.rs` | `internal/config/config.go`, `internal/config/fetch.go`, `internal/config/ccswitch.go`, `internal/provider/openai/fetch_models.go`, `internal/boot/model_error_test.go`, `internal/serve/modelswitch_test.go` | `internal/kernel/model_config.go`, `internal/kernel/provider_verify.go`, `internal/kernel/capabilities.go`, `cmd/genesisctl/main.go`, `cmd/genesisctl/capability.go`, `docs/operations/live-llm-first-run-acceptance.md`, `internal/kernel/model_config_test.go`, `cmd/genesisctl/main_test.go` |

First bounded implementation slice selected from this inventory:

- Surface: shell and process control.
- Reference-backed gap: Reasonix forces PowerShell output to UTF-8 with `psUTF8Prologue` in `internal/sandbox/shell.go` so Chinese Windows hosts do not return mojibake under CP936/OEM output pages. Genesis now uses `pwsh.exe` on Windows but does not inject an output-encoding prologue in `internal/kernel/process_runtime.go`.
- Codex alignment: Codex models shell execution behind `codex-rs/core/src/shell.rs` and routes shell execution through owned shell backends, so the shell interpreter boundary is the right owner for command argv shaping.
- Genesis implementation target: add a focused test proving Windows `platformShellCommand` prefixes PowerShell commands with the UTF-8 prologue, then update `process_runtime.go`.
- Non-goal: do not add a general shell resolver, Git Bash preference, WSL fallback, or safe-shell hook in this slice.

### 2026-07-08 Slice 1 Shell UTF-8 Prologue

Reference scan:

- Codex entrypoint and owner: `codex-rs/core/src/shell.rs` owns shell type and argv derivation; shell execution is routed through `codex-rs/core/src/tools/handlers/shell.rs` and `codex-rs/core/src/exec.rs`.
- Reasonix entrypoint and owner: `internal/sandbox/shell.go` owns shell resolution and PowerShell argv; its `psUTF8Prologue` forces UTF-8 output on Chinese Windows/OEM code pages.
- Genesis owner: `internal/kernel/process_runtime.go` owns host shell process selection and argv shaping; `internal/kernel/managed_job_executor.go` reuses that owner for managed shell jobs.

Change:

- Added `TestPlatformShellCommandOnWindowsForcesUTF8OutputEncoding` in `internal/kernel/process_runtime_test.go`.
- Prefixed Windows `pwsh.exe -Command` payloads with the UTF-8 output prologue in `internal/kernel/process_runtime.go`.

Evidence:

- RED: `go test ./internal/kernel -run TestPlatformShellCommandOnWindowsForcesUTF8OutputEncoding -count=1` failed before implementation because the command payload was `Write-Output '你好'` without the UTF-8 prologue.
- GREEN: `go test ./internal/kernel -run TestPlatformShellCommandOnWindowsForcesUTF8OutputEncoding -count=1`
- Related: `go test ./internal/kernel -run "TestPlatformShellCommandOnWindowsForcesUTF8OutputEncoding|TestForegroundShellTimeoutTerminatesDescendantProcessTree|TestForegroundShellInterruptHandsOffDescendantProcessTree|TestSubmitTurnLiveManagedExecutorRecordsCompletedOutput|TestSubmitTurnJobCancelReachesLiveManagedExecutor|TestSubmitTurnDeliversCompletedJobObservationToNextProviderStep" -count=1`

Remaining scope:

- This slice intentionally did not add shell auto-detection, Git Bash preference, WSL fallback, or safe-shell hook support.
- Next slice should return to Task 2 and inspect permission/approval/sandbox fail-closed behavior against the mapped Codex and Reasonix files.

### 2026-07-08 Slice 2 Unknown Policy Fail-Closed Coverage

Reference scan:

- Codex owner: `codex-rs/core/src/tools/sandboxing.rs` has explicit `ExecApprovalRequirement::Forbidden` and approval requirement states; `codex-rs/core/src/config/resolved_permission_profile.rs` carries trusted resolved profile snapshots instead of using raw runtime strings as authority.
- Reasonix owner: `internal/permission/permission.go` keeps policy decision pure and testable through `Policy.Decide` and `Gate.Check`; `internal/permission/bash_readonly.go` separately classifies read-only shell subjects.
- Genesis owner: `internal/kernel/authority_gate.go` already maps unknown sandbox and approval policies to blocked reasons, and `internal/kernel/model_tools.go` already reduces those reasons to model-visible `sandbox_profile_unavailable` and `approval_policy_invalid` payloads.

Change:

- Added integration coverage in `internal/kernel/tool_loop_integration_test.go` for unknown sandbox and approval policies.
- Verified both cases block before execution, leave the target file absent, preserve full blocked operation evidence in session projection, and keep model-visible tool results free of control-plane fields.

Evidence:

- Test-gap check: `go test ./internal/kernel -run "TestSubmitTurnBlocksUnknown(SandboxProfile|ApprovalPolicy)BeforeExecution" -count=1`

Remaining scope:

- No production code changed because the fail-closed behavior was already implemented.
- Continue Task 2 with approval replay, stale approval, and sandbox readiness references before moving to process/job control again.

### 2026-07-08 Task 3 Process And Job Control Scan

Reference scan:

- Codex owner: `codex-rs/core/src/unified_exec/process_manager.rs` owns process ids, output collection, cancellation, and terminal process state; `codex-rs/core/src/unified_exec/head_tail_buffer.rs` preserves bounded head/tail output.
- Reasonix owner: `internal/jobs/jobs.go` owns session-scoped background job status and cancellation; `internal/tool/builtin/bgjobs.go` exposes status, wait, and kill controls; `internal/tool/builtin/bash.go` separates foreground timeout from background jobs.
- Genesis owner: `internal/kernel/shell.go`, `internal/kernel/managed_job_executor.go`, `internal/kernel/jobs.go`, `internal/kernel/job_control_test.go`, and `internal/kernel/job_progress_test.go` already cover managed job receipt, process tree cleanup, output snapshots, terminal observation delivery, cancellation requests, lost local ownership recovery, and bounded head/tail shell output.

Classification:

- `matches` for the reference-backed semantics checked in this pass: running/completed/failed/cancelled projections, bounded job output snapshots, cancellation without forged terminal facts, and process tree cleanup are already covered by focused tests.
- `intentional difference`: Reasonix `bash_output` is incremental since last read; Genesis keeps append-only job facts and returns the latest job projection through `job_status`/`job_wait` instead of mutating read offsets.

Evidence:

- Existing coverage inspected: `TestSubmitTurnJobCancelReachesLiveManagedExecutor`, `TestSubmitTurnJobCancelLedgerOnlyRunningJobRecordsRequestWithoutForgingTerminalFact`, `TestLocalManagedJobExecutorEmitsSparseOutputSnapshot`, `TestManagedJobOutputCaptureStopsAfterTruncatedSnapshot`, `TestExecShellReportsHeadTailTruncationMetadata`, and process-tree interruption/timeout tests.

Remaining scope:

- No code change in this scan. The next bounded gap came from Task 4 session/turn idempotency handling.

### 2026-07-08 Slice 3 Running Turn Idempotency Conflict

Reference scan:

- Codex owner: `codex-rs/core/src/state/turn.rs` has explicit `ActiveTurn` state, a `RunningTask`, and cancellation waiters for in-flight turns.
- Reasonix owner: `internal/control/controller.go` exposes `ErrTurnRunning` for a second foreground turn while one is active in the same controller; `internal/agent/session.go` serializes session history mutation behind session ownership.
- Genesis owner: `internal/kernel/kernel.go` owns `activeTurns`, `tryBeginActiveTurn`, and `turnByIdempotencyKey`; `internal/kernel/http_turn.go` maps `ErrSessionActive` to HTTP `409 session_active`.

Gap:

- Retrying the same `session_id + idempotency_key` while the original turn was still running returned a plain error from `turnByIdempotencyKey`. HTTP reduced that to `400 invalid_request`, even though the semantic state is an active-session conflict and must not be reported as malformed input.

Change:

- Added `TestHTTPTurnSubmitIdempotencyKeyReturnsConflictWhileOriginalTurnRuns` in `internal/kernel/http_transport_test.go`.
- Changed `turnByIdempotencyKey` to return `ErrSessionActive` for an existing, non-terminal idempotent turn so the HTTP layer uses the existing conflict mapping.

Evidence:

- RED: `go test ./internal/kernel -run TestHTTPTurnSubmitIdempotencyKeyReturnsConflictWhileOriginalTurnRuns -count=1` failed with `status = 400, want 409`.
- GREEN: `go test ./internal/kernel -run TestHTTPTurnSubmitIdempotencyKeyReturnsConflictWhileOriginalTurnRuns -count=1`
- Related: `go test ./internal/kernel -run "TestHTTPTurnSubmitIdempotencyKey(ReturnsExistingTurnAfterRestart|ReturnsConflictWhileOriginalTurnRuns|ReturnsExistingFailureAfterRestart|RequiresValidExplicitSession)|TestForegroundAttachReplayDoesNotDuplicateManagedJob|TestSubmitTurnRefusesWhileManualCompactionOwnsSession|TestManualCompactionControlSurfaceRefusesRunningSession" -count=1`
- Race: `go test -race ./internal/kernel -run "TestHTTPTurnSubmitIdempotencyKeyReturnsConflictWhileOriginalTurnRuns" -count=1`

Remaining scope:

- Continue Task 4 by checking replay and duplicate submission surfaces that are not already covered by HTTP idempotency and foreground-attach replay tests.

### 2026-07-08 Slice 4 Stream Turn Conflict Error Shape

Reference scan:

- Codex owner: `codex-rs/core/src/session/session.rs` documents one running task per session and projects stream errors as typed protocol events rather than unstructured text.
- Reasonix owner: `internal/control/controller.go` uses `ErrTurnRunning` for running-turn conflicts and applies the same running guard to send, rewind, fork, branch, and summarize control surfaces.
- Genesis owner: `internal/kernel/http_turn.go` owns both `/turn` and `/turn/stream` transport reduction; `internal/kernel/kernel.go` owns the `ErrSessionActive` sentinel.

Gap:

- `/turn` mapped active-session conflicts to HTTP `409 session_active`, but `/turn/stream` reduced the same `ErrSessionActive` to a generic NDJSON `turn_failed` error code. Streaming responses cannot change status after headers, but the stream event still needs the same semantic error code.

Change:

- Added `TestHTTPTurnStreamReportsSessionActiveConflict` in `internal/kernel/http_transport_test.go`.
- Added `turnStreamError` in `internal/kernel/http_turn.go` so stream errors preserve provider unavailable, ingress block, tool infrastructure, and active-session semantic codes when no replayed turn error is already present.

Evidence:

- RED: `go test ./internal/kernel -run TestHTTPTurnStreamReportsSessionActiveConflict -count=1` failed because the stream event carried a generic error code.
- GREEN: `go test ./internal/kernel -run "TestHTTPTurn(StreamReportsSessionActiveConflict|SubmitIdempotencyKeyReturnsConflictWhileOriginalTurnRuns)" -count=1`
- Related: `go test ./internal/kernel -run "TestHTTPTurn(StreamReportsSessionActiveConflict|SubmitIdempotencyKeyReturnsConflictWhileOriginalTurnRuns|SubmitIdempotencyKeyReturnsExistingTurnAfterRestart|SubmitIdempotencyKeyReturnsExistingFailureAfterRestart)|TestSubmitTurnStream(EmitsDeltasButPersistsOnlyFinalMessage|DoesNotRetryAfterVisibleDelta)" -count=1`
- Race: `go test -race ./internal/kernel -run "TestHTTPTurn(StreamReportsSessionActiveConflict|SubmitIdempotencyKeyReturnsConflictWhileOriginalTurnRuns)" -count=1`

Remaining scope:

- Continue Task 4 by checking replay read models and idempotency failure cases not covered by the HTTP and stream transport slices.

### 2026-07-08 Slice 5 Replayed Turn Failure Status

Reference scan:

- Codex owner: `codex-rs/core/src/session/turn.rs` converts turn execution errors into structured protocol lifecycle/error events, so callers keep semantic failure information instead of only receiving free text.
- Reasonix owner: `internal/control/controller.go` keeps running, failure, and snapshot state under the controller and refuses incompatible control actions while a turn is active; `internal/agent/session.go` exposes copied snapshots for concurrent readers.
- Genesis owner: `internal/kernel/kernel.go` owns idempotency replay through `turnByIdempotencyKey` and `replayedTurnFailure`; `internal/kernel/http_turn.go` owns HTTP status reduction for replayed turn errors.

Gap:

- A live `tool_infrastructure_failed` turn maps to HTTP 503, but replaying an already persisted failed turn with the same idempotency key reduced the same semantic failure to HTTP 400 because replayed failures did not unwrap to `ErrToolInfrastructureFailed` and `turnErrorHTTPStatus` lacked the status mapping.

Change:

- Added `TestHTTPTurnSubmitIdempotencyKeyPreservesReplayedToolInfrastructureStatus` in `internal/kernel/http_transport_test.go`.
- Extended `replayedTurnFailure.Unwrap` and `turnErrorHTTPStatus` so replayed turn failures preserve provider unavailable, ingress block, tool infrastructure, session active, tool-call rejection, and interruption semantics consistently with live submission.

Evidence:

- RED: `go test ./internal/kernel -run TestHTTPTurnSubmitIdempotencyKeyPreservesReplayedToolInfrastructureStatus -count=1` failed with `retry status = 400, want 503`.
- GREEN: `go test ./internal/kernel -run TestHTTPTurnSubmitIdempotencyKeyPreservesReplayedToolInfrastructureStatus -count=1`
- Related: `go test ./internal/kernel -run "TestHTTPTurn(StreamReportsSessionActiveConflict|SubmitIdempotencyKey(ReturnsExistingFailureAfterRestart|PreservesReplayedToolInfrastructureStatus|ReturnsConflictWhileOriginalTurnRuns))" -count=1`
- Race: `go test -race ./internal/kernel -run "TestHTTPTurn(StreamReportsSessionActiveConflict|SubmitIdempotencyKey(ReturnsExistingFailureAfterRestart|PreservesReplayedToolInfrastructureStatus|ReturnsConflictWhileOriginalTurnRuns))" -count=1`
- Full: `go test ./... -count=1`
- Build: `go build ./...`

Remaining scope:

- Continue Task 4 by checking whether replayed paused/interrupted turns and session projection snapshots preserve the same semantic status across restart.

### 2026-07-08 Slice 6 Competing Idempotency Evidence

Reference scan:

- Codex owner: `codex-rs/core/src/state/turn.rs` and `codex-rs/core/src/session/session.rs` keep one active turn owner and lifecycle state for a session, so duplicate foreground state is a control-plane invariant rather than user request text.
- Reasonix owner: `internal/control/controller.go` rejects a second running turn with `ErrTurnRunning`, while checkpoint/session persistence records turn boundaries for later recovery instead of letting conflicting turn ownership become normal input.
- Genesis owner: `internal/kernel/kernel.go` owns ledger replay through `turnByIdempotencyKey`; HTTP admission in `internal/kernel/http_turn.go` already treats ledger corruption as service-unavailable authority failure.

Gap:

- If persisted ledger evidence showed the same `session_id + idempotency_key` on two different turn IDs, Genesis returned a generic invalid request. That misclassified contradictory authority facts as a caller formatting problem instead of failing closed as ledger corruption.

Change:

- Added `TestHTTPTurnSubmitIdempotencyKeyRejectsCompetingLedgerEvidence` in `internal/kernel/http_transport_test.go`.
- Changed `turnByIdempotencyKey` to wrap competing idempotency evidence as `ErrLedgerCorrupt`, preserving the existing HTTP `503 ledger_corrupt` path and preventing a retry from minting another contradictory turn.

Evidence:

- RED: `go test ./internal/kernel -run TestHTTPTurnSubmitIdempotencyKeyRejectsCompetingLedgerEvidence -count=1` failed with `status = 400, want 503`.
- GREEN: `go test ./internal/kernel -run TestHTTPTurnSubmitIdempotencyKeyRejectsCompetingLedgerEvidence -count=1`
- Related: `go test ./internal/kernel -run "TestHTTPTurnSubmitIdempotencyKey(PreservesReplayedToolInfrastructureStatus|RejectsCompetingLedgerEvidence|ReturnsExistingFailureAfterRestart|ReturnsConflictWhileOriginalTurnRuns|ReturnsExistingTurnAfterRestart)" -count=1`
- Race: `go test -race ./internal/kernel -run "TestHTTPTurnSubmitIdempotencyKey(PreservesReplayedToolInfrastructureStatus|RejectsCompetingLedgerEvidence|ReturnsExistingFailureAfterRestart|ReturnsConflictWhileOriginalTurnRuns|ReturnsExistingTurnAfterRestart)" -count=1`
- Full: `go test ./... -count=1`
- Build: `go build ./...`

Remaining scope:

- Continue Task 4 by checking paused/interrupted HTTP replay after restart and then decide whether Task 4 can close in favor of Task 5 provider boundary work.
