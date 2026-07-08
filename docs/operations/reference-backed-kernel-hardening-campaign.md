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

### 2026-07-08 Slice 7 Stream Pause Terminal Event

Reference scan:

- Codex owner: `codex-rs/core/src/session/turn.rs` emits structured lifecycle/error events for turn terminal states and keeps realtime deltas separate from terminal protocol facts.
- Reasonix owner: `internal/control/controller.go` treats running, canceled, and recovery-sensitive turn states as controller-owned lifecycle state; `internal/agent/session.go` exposes snapshots for readers rather than asking clients to infer lifecycle from text.
- Genesis owner: `internal/kernel/http_turn.go` reduces kernel `TurnResponse` to NDJSON stream events; desktop Go and Vue clients consume those stream terminal events.

Gap:

- A budget-paused turn returned `TurnResponse.Pause`, but `/turn/stream` emitted `turn_completed` for every nil-error response. Stream clients could not distinguish a waiting paused turn from a completed final answer, and desktop clients only accepted `turn_completed` as the response-bearing terminal event.

Change:

- Added `TestHTTPTurnStreamReportsBudgetPause` in `internal/kernel/http_transport_test.go`.
- Added `turnStreamTerminalEvent` so paused responses emit `turn_paused` while final responses continue emitting `turn_completed`.
- Updated desktop Go and frontend stream clients to accept both `turn_completed` and `turn_paused` as terminal response events, with tests for paused stream responses.

Evidence:

- RED: `go test ./internal/kernel -run TestHTTPTurnStreamReportsBudgetPause -count=1` failed because the terminal event was `turn_completed`.
- GREEN: `go test ./internal/kernel -run TestHTTPTurnStreamReportsBudgetPause -count=1`
- Related: `go test ./internal/kernel -run "TestHTTPTurnStreamReports(BudgetPause|SessionActiveConflict)|TestSubmitTurnStream(EmitsDeltasButPersistsOnlyFinalMessage|DoesNotRetryAfterVisibleDelta)|TestSubmitTurnPausesToolLoopBudgetWithoutExecutingOverBudgetBatch" -count=1`
- Race: `go test -race ./internal/kernel -run "TestHTTPTurnStreamReports(BudgetPause|SessionActiveConflict)|TestSubmitTurnStream(EmitsDeltasButPersistsOnlyFinalMessage|DoesNotRetryAfterVisibleDelta)|TestSubmitTurnPausesToolLoopBudgetWithoutExecutingOverBudgetBatch" -count=1`
- Desktop: `go test ./... -count=1` from `desktop`
- Frontend: `npm.cmd test` from `desktop/frontend`
- Full: `go test ./... -count=1`
- Build: `go build ./...`
- Frontend build: `npm.cmd run build` from `desktop/frontend`

Remaining scope:

- Task 4 session/turn replay now has active conflict, stream conflict, replayed failure class, competing ledger evidence, and stream pause coverage. Next slice should move to Task 5 provider boundary unless a final interrupted-after-restart scan finds a concrete uncovered gap.

### 2026-07-08 Slice 8 Streaming Provider Usage Request

Reference scan:

- Codex owner: `codex-rs/core/src/client.rs` builds provider requests with explicit provider metadata, prompt cache keys, and streaming request options instead of assuming provider defaults will return all accounting fields.
- Reasonix owner: `internal/agent/agent.go` consumes provider `ChunkUsage` as an agent event, and `internal/agent/cachehit_e2e_test.go` treats cache hit/miss tokens as context-engineering evidence rather than UI-only telemetry.
- Genesis owner: `internal/kernel/openai_compatible.go` owns the OpenAI-compatible provider adapter; `internal/kernel/modelgateway/accounting.go` and context compaction already consume normalized usage/cache facts when providers report them.

Gap:

- The OpenAI-compatible stream decoder already preserved `usage` chunks, including cache hit/miss fields, but streaming requests did not ask OpenAI-compatible servers to include usage. Many compatible servers only emit the terminal usage chunk when `stream_options.include_usage` is true, so Genesis could silently lose usage/cache evidence in streamed local or cloud model turns.

Change:

- Added `TestOpenAICompatibleProviderStreamRequestsAndPreservesUsage` in `internal/kernel/provider_gateway_test.go`.
- Added `stream_options.include_usage=true` to OpenAI-compatible streaming requests and kept non-streaming requests unchanged.
- Preserved existing streamed usage normalization through `tokenUsageFromChatUsage`, including cache hit/miss fields.

Evidence:

- RED: `go test ./internal/kernel -run TestOpenAICompatibleProviderStreamRequestsAndPreservesUsage -count=1` failed because the request body had `stream:true` but no `stream_options`.
- GREEN: `go test ./internal/kernel -run TestOpenAICompatibleProviderStreamRequestsAndPreservesUsage -count=1`

Remaining scope:

- Continue Task 5 by checking provider-command response classification and local/cloud adapter parity, then decide whether provider model-list refresh belongs in this deterministic campaign or a later production-capability package.

### 2026-07-08 Slice 9 HTTP Provider Failure Classification

Reference scan:

- Codex owner: `codex-rs/core/src/client.rs` and `codex-rs/codex-client/src/retry.rs` keep provider/auth/retry failures as provider transport facts instead of folding them into user request validation.
- Reasonix owner: `internal/provider/retry.go` separates retryable HTTP/provider statuses from caller/config errors, while `internal/provider/openai/openai.go` reports streamed usage and provider failures through the provider abstraction.
- Genesis owner: `internal/kernel/modelgateway/resilience.go` already classifies provider failures, `internal/kernel/kernel.go` writes classified turn failures, and `internal/kernel/http_turn.go` owns the runtime HTTP projection.

Gap:

- `SubmitTurn` appended classified provider failure evidence to the ledger, but `providerCompleteError` returned an unclassified Go error. Synchronous `/turn` callers therefore saw provider outages and provider-command adapter shape failures as `400 invalid_request`, even though the kernel had already classified them as provider failures.

Change:

- Added `TestHTTPProviderFailuresKeepProviderErrorCodes` in `internal/kernel/provider_gateway_test.go`.
- Changed `providerCompleteError` to preserve existing `ProviderClassifiedError` metadata and to classify untyped provider adapter failures as `provider_error`.
- Mapped classified provider errors at HTTP and stream error boundaries so transient upstream failures return `503 provider_transient_failure` and generic adapter failures return `502 provider_error`.

Evidence:

- RED: `go test ./internal/kernel -run TestHTTPProviderFailuresKeepProviderErrorCodes -count=1` failed with `400 invalid_request` for both OpenAI-compatible 500s and provider-command bad JSON.
- GREEN: `go test ./internal/kernel -run TestHTTPProviderFailuresKeepProviderErrorCodes -count=1`
- Related: `go test ./internal/kernel -run "Test(HTTPProviderFailuresKeepProviderErrorCodes|ProviderCommandAdapterShapeFailureDoesNotRetry|ProviderCommandFailureRedactsStderrFromTurnAndHTTP|OpenAICompatibleProvider|HTTPTurnStreamReportsSessionActiveConflict)" -count=1`
- Gateway: `go test ./internal/kernel/modelgateway -count=1`

Remaining scope:

- Continue Task 5 with local/cloud provider adapter parity, then move to provider/model configuration refresh only if the current approved docs already contain that as deterministic scope.

### 2026-07-08 Slice 10 Streaming Tool Call ID Parity

Reference scan:

- Codex owner: `codex-rs/core/src/client.rs` keeps provider tool-call deltas ordered and replayable through the provider client boundary before the tool loop sees them.
- Reasonix owner: `internal/provider/openai/openai.go` synthesizes `call_<index>` when OpenAI-compatible streaming tool-call deltas omit IDs, because empty IDs collapse downstream result pairing.
- Genesis owner: `internal/kernel/openai_compatible.go` merges OpenAI-compatible streaming tool calls by index, then converts them into kernel `ModelToolCall` values for the shared tool loop.

Gap:

- Genesis sorted streamed tool calls by index, but left `tool_call_id` empty when the provider omitted it. Multiple empty provider IDs are legal inside the kernel only because kernel event IDs stay distinct, but replaying those calls to OpenAI-compatible providers can lose provider-visible pairing.

Change:

- Added `TestOpenAICompatibleProviderStreamSynthesizesMissingToolCallIDs` in `internal/kernel/provider_gateway_test.go`.
- Changed `orderedStreamToolCalls` to synthesize `call_<index>` for streamed tool calls whose provider ID is missing, preserving stable provider-visible pairing without changing non-streamed provider calls.

Evidence:

- RED: `go test ./internal/kernel -run TestOpenAICompatibleProviderStreamSynthesizesMissingToolCallIDs -count=1` failed with empty `ToolCallID` values.
- GREEN: `go test ./internal/kernel -run TestOpenAICompatibleProviderStreamSynthesizesMissingToolCallIDs -count=1`
- Related: `go test ./internal/kernel -run "Test(OpenAICompatibleProviderStream|SubmitTurnExecutesOpenAICompatibleToolCallBeforeFinal|CommandProviderToolLoopThroughKernel)" -count=1`

Remaining scope:

- Task 5 provider boundary now has streamed usage, HTTP failure classification, provider-command failure classification, and streamed tool-call ID parity. Next scan should move to provider configuration/verification gaps already present in approved docs; skip model-list refresh until it has a requirement/design package.

### 2026-07-08 Slice 11 Provider Command Live Verify

Reference scan:

- Codex owner: provider configuration and auth checks stay on the provider/client boundary; callers receive provider readiness evidence instead of assuming a configured provider is usable.
- Reasonix owner: `internal/provider/provider.go` exposes one provider abstraction and `internal/provider/openai/openai.go` keeps local/cloud-compatible model calls behind that abstraction; model usage code does not special-case the caller once a provider is resolved.
- Genesis owner: `internal/kernel/provider_verify.go` owns `genesisctl provider verify`, while `internal/kernel/model_config.go` can already resolve both `openai-chat-completions` and `provider_command` routes.

Gap:

- `VerifyProviderLive` rejected resolved `provider_command` configs as `provider_protocol_unsupported`, even though the daemon can already run them. That made local model provider-command routes second-class compared with cloud OpenAI-compatible routes.

Change:

- Replaced the rejection test with `TestVerifyProviderLiveRunsProviderCommandConfig`.
- Changed `VerifyProviderLive` to instantiate either `NewOpenAICompatibleProvider` or `NewCommandProvider` from the resolved config and run the same text probe through the shared `Provider` interface.
- Added stable session/turn IDs to the verify `ModelRequest` so provider-command adapters receive a complete protocol request.

Evidence:

- RED: `go test ./internal/kernel -run TestVerifyProviderLiveRunsProviderCommandConfig -count=1` failed with `provider_protocol_unsupported`.
- GREEN: `go test ./internal/kernel -run TestVerifyProviderLiveRunsProviderCommandConfig -count=1`
- Related: `go test ./internal/kernel -run TestVerifyProviderLive -count=1`

Remaining scope:

- Continue Task 5 by scanning provider setup/config CLI behavior for drift. Keep model-list refresh out of this campaign until a requirement/design explicitly admits it.

### 2026-07-08 Slice 12 Provider Command Verify CLI Coverage

Reference scan:

- Codex owner: provider setup and execution surfaces keep CLI/API callers behind the same provider client boundary.
- Reasonix owner: `internal/config/config.go` resolves provider entries once, while agent execution consumes the resolved provider interface rather than branching by caller.
- Genesis owner: `cmd/genesisctl/main.go` delegates `provider verify` to `kernel.VerifyProviderLive`; kernel verification now supports both OpenAI-compatible and provider-command routes.

Gap:

- The kernel had provider-command live verify coverage, but the user-facing `genesisctl provider verify` command did not have a regression test proving that provider-command configs return the same JSON readiness shape through the CLI.

Change:

- Added `TestProviderVerifyRunsProviderCommandConfig` in `cmd/genesisctl/main_test.go`.
- Added a tiny test-only provider-command helper that emits a valid provider-command final response.

Evidence:

- GREEN: `go test ./cmd/genesisctl -run TestProviderVerify -count=1`

Remaining scope:

- Task 5 is close to covered. Do a final approved-doc drift scan for provider setup/config; if no concrete gap appears, move to Task 6 resource/material/redaction.

### 2026-07-08 Slice 13 Hydration Continuation Evidence

Reference scan:

- Codex owner: `codex-rs/core/src/tools/handlers/mcp_resource/read_mcp_resource.rs` serializes resource reads through the tool boundary with truncation policy applied to model-visible output.
- Reasonix owner: `internal/tool/builtin/readfile.go` returns bounded file windows and gives the model a concrete continuation hint when more content remains.
- Genesis owner: `internal/kernel/context_hydration.go` admits bounded resource text into the next provider context, while `resource_read` and `source_read` already expose byte continuation offsets.

Gap:

- Truncated context hydration evidence carried `truncated` and `visible_bytes`, but not the next byte offset needed to inspect or rehydrate the remaining authorized resource content from the same source.

Change:

- Added `next_offset_bytes` to `ContextHydrationProjection`.
- Copied the continuation offset from the bounded resource read admission and clone it with the hydration projection.
- Added `TestContextHydrationTruncationCarriesContinuationOffset` to lock the admitted evidence, provider-visible bounded text, and context inspection projection.

Evidence:

- GREEN: `go test ./internal/kernel -run TestContextHydrationTruncationCarriesContinuationOffset -count=1`

Remaining scope:

- Continue Task 6 by scanning material/context projections for any remaining model-visible owner refs or missing bounded-read evidence.

### 2026-07-08 Slice 14 Source Snapshot Context Budget

Reference scan:

- Codex owner: MCP resource reads serialize through a truncation policy before becoming tool output.
- Reasonix owner: `read_file` returns bounded windows instead of unbounded file bodies.
- Genesis owner: `internal/kernel/model_context.go` builds the model-visible source snapshot listing before the provider call.

Gap:

- Individual source reads and trees were bounded, but the model-visible source snapshot listing itself had no byte budget when a session accumulated many admitted snapshots.

Change:

- Capped source snapshot context at 4096 bytes.
- Capped each source snapshot display label at 160 bytes.
- Added a model-visible omission hint when additional snapshots are excluded by the context budget.
- Added `TestSourceSnapshotContextIsBounded`.

Evidence:

- GREEN: `go test ./internal/kernel -run TestSourceSnapshotContextIsBounded -count=1`

Remaining scope:

- Continue Task 6 by checking source snapshot capability/limit projection evidence and any remaining unbounded model-context fragments.

### 2026-07-08 Slice 15 Source Context Limit Projection

Reference scan:

- Codex owner: resource/tool output truncation is tied to model/runtime metadata rather than hidden constants in caller code.
- Reasonix owner: file read windows are visible in the tool contract and continuation output.
- Genesis owner: `internal/kernel/limit_policy.go` projects runtime and source snapshot budgets through `Capabilities().Limits`.

Gap:

- The source snapshot context and display-label budgets were enforced, but not inspectable in the existing limit projection alongside the other source snapshot budgets.

Change:

- Added `source_snapshot.context_bytes` and `source_snapshot.label_bytes` runtime limit projections.
- Extended `TestSourceSnapshotPolicyIsInspectableRuntimeLimit`.

Evidence:

- GREEN: `go test ./internal/kernel -run TestSourceSnapshotPolicyIsInspectableRuntimeLimit -count=1`

Remaining scope:

- Continue Task 6 by scanning model-visible context fragments outside source snapshots for missing projection bounds.

### 2026-07-08 Slice 16 Kernel Observation Context Budget

Reference scan:

- Codex owner: resource/tool outputs are truncated before model-visible replay, and event/control ids stay outside resource content.
- Reasonix owner: file reads provide bounded windows and do not mark unseen content as consumed.
- Genesis owner: `internal/kernel/observations.go` converts pending terminal job facts into provider-context observations and records delivered observation event ids.

Gap:

- Kernel observation context bounded per-output previews, but not the total observation payload. A naive text cap would also be wrong because omitted event ids must not be marked delivered.

Change:

- Capped kernel observation context at 4096 bytes.
- Bounded receipt and failure-reason lines with the existing timeline preview helper.
- Returned only the observation event ids that were actually included in provider context.
- Projected `kernel_observation.context_bytes` through runtime limits.
- Added `TestKernelObservationContextBoundsDeliveredEvents`.

Evidence:

- GREEN: `go test ./internal/kernel -run TestKernelObservationContextBoundsDeliveredEvents -count=1`

Remaining scope:

- Continue Task 6 with a final scan for unbounded model-visible fragments; conversation history remains governed by compaction and should not get an ad hoc text cap without a separate compaction design update.

### 2026-07-08 Slice 17 Inspection Tool Manifest Names

Reference scan:

- Codex owner: `codex-rs/core/src/client.rs` records inference trace attempts separately from normal model requests, while websocket tests assert trace metadata is not sent as a top-level request field.
- Reasonix owner: `internal/acp/dispatch.go` maps typed agent events into transcript-facing updates, while `internal/acp/service.go` persists a separate transcript path for replay and resume.
- Genesis owner: `internal/kernel/projections.go` and `internal/kernel/session_debug.go` expose context inspection and session debug artifacts outside the ordinary chat timeline.

Gap:

- Context inspection and session debug defaulted to returning the full provider tool manifest even though the inspection contract only needs the tool names that were visible for that turn. Full schemas are provider-facing runtime detail and can carry path-shaped or secret-shaped descriptors in custom tool surfaces.

Change:

- Added `ToolManifestInspection` as a name-only inspection projection.
- Changed `ContextInspection` and `SessionDebugExport` to return name-only tool manifest entries while leaving the ledger's `turn.submitted` provider-visible snapshot unchanged.
- Added `TestContextInspectionToolManifestIsNameOnly` and tightened existing context/debug tests to reject full tool schema fields from these inspection surfaces.

Evidence:

- GREEN: `go test ./internal/kernel -run Test(ContextInspectionProjectionPersistsProviderVisibleSnapshot|ContextInspectionToolManifestIsNameOnly|SessionDebugCapturesProviderStepsAndToolLoopWithoutHostPaths|PublicProjectionArraysMarshalAsNonNullArrays|ArchitectureBoundaryKernelTypesStayInExpectedFiles) -count=1`

Remaining scope:

- Continue Task 7 by scanning audit replay error/output previews and debug artifacts for any remaining default leaks of path-shaped runtime internals.

### 2026-07-08 Slice 18 Tool Failure Diagnostic Redaction

Reference scan:

- Codex owner: `codex-rs/core/src/client.rs` extracts response debug context and records inference failures through the trace path, keeping request/debug metadata separate from ordinary user transcript.
- Reasonix owner: `internal/acp/dispatch.go` sends tool results and warnings through typed event updates, while persistent transcript replay is a separate ACP surface.
- Genesis owner: `internal/kernel/tool_execution.go` reduces fatal tool execution failures into `turn.failed`, which is then visible through session, timeline, and audit replay projections.

Gap:

- Provider failure diagnostics were already redacted before persistence, but fatal tool execution errors wrote `err.Error()` directly into `TurnError.Message`. Tool infrastructure errors can include credential-shaped command, runner, or environment diagnostics.

Change:

- Routed tool execution failure messages through `externalBoundaryDiagnosticText` before appending `turn.failed`.
- Added `TestToolExecutionErrorRedactsCredentialShapedDiagnostics` to prove both raw turn events and audit replay carry the redacted diagnostic text.

Evidence:

- GREEN: `go test ./internal/kernel -run Test(ToolExecutionErrorRedactsCredentialShapedDiagnostics|ExecuteToolBatchesRecordsFatalRunnerShapeFailuresWithoutForgingResults) -count=1`

Remaining scope:

- Continue Task 7 by checking whether stream/HTTP error envelopes reuse the persisted redacted turn failure or can still surface raw external diagnostics.

### 2026-07-08 Slice 19 HTTP Tool Diagnostic Redaction

Reference scan:

- Codex owner: websocket request tests keep trace metadata in client metadata and assert it is not sent as an unrelated top-level request field; provider/debug diagnostics are handled as their own transport concern.
- Reasonix owner: ACP dispatch converts typed runtime events into client updates, and warnings/errors are surfaced through typed event fields instead of raw runtime state.
- Genesis owner: `internal/kernel/http_turn.go` and `internal/kernel/http_tools.go` are the HTTP fallback boundary when a turn or direct tool request fails before a full `TurnResponse` can be returned.

Gap:

- Slice 18 redacted persisted `turn.failed` messages, but HTTP and stream fallback paths for `ErrToolInfrastructureFailed` still used raw `err.Error()` when no response error was available. `tool_call_rejected` persisted failures also used raw model/tool-boundary error text.

Change:

- Redacted `ErrToolInfrastructureFailed` messages in `/turn`, `/turn/stream`, and direct HTTP tool fallback envelopes.
- Redacted persisted `tool_call_rejected` turn failure messages.
- Added `TestTurnStreamErrorRedactsToolInfrastructureDiagnostics` and reused the tool execution diagnostic redaction regression.

Evidence:

- GREEN: `go test ./internal/kernel -run Test(TurnStreamErrorRedactsToolInfrastructureDiagnostics|ToolExecutionErrorRedactsCredentialShapedDiagnostics|HTTPTurnStreamReportsSessionActiveConflict|SubmitTurnRejectsDuplicateToolCallIDsBeforeExecution) -count=1`

Remaining scope:

- Continue Task 7 with a final pass over non-turn inspection/readiness/debug error envelopes for raw external-boundary diagnostics.

### 2026-07-08 Slice 20 Ledger Error Envelope Redaction

Reference scan:

- Codex owner: readiness/debug metadata is projected as structured status rather than leaking raw trace or transport internals into ordinary client payloads.
- Reasonix owner: session and transcript service paths return typed RPC errors and keep persisted transcript paths as controlled service state.
- Genesis owner: `internal/kernel/http.go` centralizes `ErrLedgerUnavailable` HTTP envelopes, while `/ready` already projects ledger readiness as reason codes.

Gap:

- `/ready` reported ledger failures with bounded reason codes, but HTTP error envelopes for `ErrLedgerUnavailable` returned the wrapped ledger error text. A lower-level ledger error can include local paths or credential-shaped diagnostics.

Change:

- Changed `writeKernelUnavailable` to use the ledger reason code as both code and message.
- Added `TestHTTPKernelUnavailableDoesNotLeakLedgerDiagnostics` with a ledger load error containing a Windows path and credential-shaped text.

Evidence:

- GREEN: `go test ./internal/kernel -run Test(HTTPKernelUnavailableDoesNotLeakLedgerDiagnostics|HTTPCorruptLedgerBlocksReadyReplayAndAppend|HTTPUnreadableLedgerBlocksReadyAndTurn|HTTPReadyDoesNotExposeInspectionDetails) -count=1`

Remaining scope:

- Task 7 has no remaining obvious default-leak gaps from the current scan. Do a final diff/test pass and then move to Task 8 config, doctor, startup, and readiness.

### 2026-07-08 Slice 21 Invalid Provider Config Readiness

Reference scan:

- Codex owner: `codex-rs/cli/src/doctor.rs` builds redacted diagnostic rows for configuration, authentication, runtime, sandbox, state, and provider reachability without mutating user state.
- Reasonix owner: `internal/doctor/report.go` reports config parse warnings separately from defaulted config state, and provider diagnostics expose key-present status without raw key material.
- Genesis owner: `internal/kernel/model_config.go`, `internal/kernel/provider_verify.go`, `cmd/genesisd`, and `cmd/genesisctl` expose provider readiness through kernel config resolution and CLI/daemon entrypoints.

Gap:

- A missing `models.json` and an unreadable or malformed `models.json` both mapped to `ErrGenesisModelConfigMissing`, which made live provider verify and daemon readiness report `provider_config_missing` for invalid provider configuration. This erased the operator distinction required by Task 8 and made bad config look like first-run setup.

Change:

- Added `ErrGenesisModelConfigInvalid` and mapped unreadable or unparsable `models.json` to `provider_config_invalid`.
- Preserved missing file behavior as `provider_config_missing`.
- Added regression coverage for the kernel resolver, live provider verify, `genesisctl provider verify`, and `genesisd` provider construction, including secret-shaped invalid JSON content that must not appear in readiness output.

Evidence:

- GREEN: `go test ./internal/kernel -run Test(ResolveProviderConfigFromGenesisRejectsInvalidModelsJSON|VerifyProviderLiveReportsInvalidConfigDistinctFromMissing|VerifyProviderLiveReportsMissingCredentialWithoutNetworkProbe) -count=1`
- GREEN: `go test ./cmd/genesisd ./cmd/genesisctl -run Test(BuildProviderFromGenesisConfigReportsInvalidConfig|BuildProviderEmptyNameDoesNotSelectFake|ProviderVerifyReportsInvalidConfigAsJSON|ProviderVerifyReportsMissingCredentialAsJSON) -count=1`
- GREEN: `git diff --check`
- GREEN: `go test ./... -count=1`
- GREEN: `go build ./...`

Remaining scope:

- Continue Task 8 by checking whether startup and doctor-like diagnostics cover unknown provider names, missing provider credentials, live auth failures, and desktop sidecar ownership without conflating them with kernel process failure.

### 2026-07-08 Slice 22 Provider Readiness Reason Propagation

Reference scan:

- Codex owner: doctor output carries each check's category, status, detail, and issue metadata so callers do not have to infer the failing subsystem from a generic top-level failure.
- Reasonix owner: doctor and inspect reports keep provider key readiness as explicit provider fields while the CLI keeps configuration/load warnings visible near the top of the report.
- Genesis owner: `Kernel.Ready()` aggregates provider, runtime auth, and ledger readiness for `/ready` and feeds the same projection into `/capabilities`.

Gap:

- `/ready.provider.readiness_reason` carried specific provider blockers, but top-level `/ready.readiness_reason` collapsed all provider failures to `provider_not_ready`. A caller watching only the top-level readiness could not distinguish missing credentials, invalid provider config, provider command setup failures, or other sanitized provider blockers.

Change:

- Propagated the sanitized provider readiness reason to top-level readiness when the provider is the blocking subsystem.
- Preserved the existing `safeInspectionReadinessReason` guard so unsafe provider-supplied strings become `provider_status_unavailable`.
- Tightened readiness state tests to require the specific top-level provider reason and to prove unsafe provider reasons are redacted at both top-level and provider substatus.

Evidence:

- GREEN: `go test ./internal/kernel -run Test(ReadinessSurfacesUseReadinessAxis|TopLevelReadinessRedactsUnsafeProviderReason|ContextRuntimeReadinessDoesNotUseProviderStatus) -count=1`

Remaining scope:

- Run full verification for Slice 22, then continue Task 8 with startup/doctor-like diagnostics for unknown provider names and local dependency readiness.

### 2026-07-08 Slice 23 Unknown Provider Startup Readiness

Reference scan:

- Codex owner: doctor checks classify provider/config reachability without starting long-lived services or echoing raw configuration values.
- Reasonix owner: boot distinguishes unknown model/provider configuration from key readiness, while doctor/inspect reports provider state through bounded fields.
- Genesis owner: `cmd/genesisd.buildProvider` is the daemon startup boundary that turns `--provider` / `GENESIS_PROVIDER` into a kernel provider.

Gap:

- An unknown daemon provider name returned a raw error from `buildProvider`, causing daemon startup to fail and exposing the untrusted provider string in the startup error. That made a provider selection typo look like a process failure rather than a structured not-ready provider state.

Change:

- Changed unknown daemon provider selection to return `BlockedProvider("provider", "provider_unknown")`.
- Added a daemon test with a secret-shaped unknown provider value to prove the raw value is not propagated through provider readiness.

Evidence:

- GREEN: `go test ./cmd/genesisd -run Test(BuildProviderUnknownProviderStaysStructuredNotReady|BuildProviderOpenAICompatibleMissingKeyStaysStructuredNotReady|BuildProviderCanSelectCommandProviderDirectly|BuildProviderBlocksSecretShapedCommandEnvironment) -count=1`
- GREEN: `go test ./internal/kernel -run Test(ReadinessSurfacesUseReadinessAxis|TopLevelReadinessRedactsUnsafeProviderReason) -count=1`

Remaining scope:

- Run full verification for Slice 23, then continue Task 8 with live auth failure, command-provider dependency blockers, and desktop sidecar ownership checks.

### 2026-07-08 Slice 24 Local Provider Doctor

Reference scan:

- Codex owner: `codex doctor` is read-mostly and reports local configuration, authentication, runtime, sandbox, state, and provider reachability through redacted structured rows.
- Reasonix owner: `reasonix doctor --json` emits redacted local diagnostics, while inspect/provider readiness reports whether provider keys are configured without exposing their values.
- Genesis owner: `genesisctl provider verify` already performs a live provider probe, but there was no non-network local diagnostic that classified model config, local credentials, and command-provider dependencies.

Gap:

- Operators had to use live `provider verify` even for local setup problems such as missing credentials or a missing provider-command executable. That conflated local doctor checks with upstream auth/reachability checks and made first-pass startup diagnosis less deterministic.

Change:

- Added `genesisctl doctor [--json]` as a local, non-network provider readiness diagnostic.
- The command resolves Genesis model config and local credentials, constructs the configured provider, and calls `Ready()` only. It does not submit a model request.
- Added JSON coverage for missing local credential, missing provider-command dependency, and ready provider-command configuration, including checks that credential refs and command paths are not emitted.

Evidence:

- GREEN: `go test ./cmd/genesisctl -run Test(DoctorReportsMissingCredentialAsJSON|DoctorReportsProviderCommandDependencyAsJSON|DoctorReportsReadyProviderCommandAsJSON|ProviderVerifyReportsInvalidConfigAsJSON|ProviderVerifyReportsMissingCredentialAsJSON) -count=1`

Remaining scope:

- Run full verification for Slice 24, then continue Task 8 with desktop sidecar ownership checks and any remaining readiness/documentation drift.

### 2026-07-08 Slice 25 Desktop Sidecar Boundary Check

Reference scan:

- Codex owner: local runtime and background-service diagnostics are separated from normal provider calls; doctor checks can report background server state without taking ownership of unrelated external services.
- Reasonix owner: CLI boot and doctor surfaces distinguish local setup/readiness from the interactive agent loop, and inspect reports provider/tool state without forcing startup ownership.
- Genesis owner: `desktop/app.go`, `desktop/local_service_supervisor.go`, and `desktop/app_test.go` own desktop sidecar behavior outside the kernel package.

Result:

- `loadDesktopConfig` marks sidecar ownership as `external` when `GENESIS_KERNEL_BASE_URL` is set.
- `LocalServiceSupervisor.StartKernel` returns the external projection without invoking the launcher when ownership is external.
- `LocalServiceSupervisor.StopOwned` returns the external projection without stopping any process when ownership is external.
- Owned sidecars still launch, probe readiness, expose pid/log metadata, and stop idempotently through the desktop-owned supervisor.

Classification:

- Matches Task 8 acceptance. No code change needed in this slice.

Evidence:

- GREEN: `go test ./... -run Test(LocalServiceSupervisorProjectsExternalKernelWithoutOwnership|LocalServiceSupervisorDoesNotOwnExternalKernel|DesktopStartupAndShutdownRouteThroughLocalServiceSupervisor|LocalServiceSupervisorShutdownOnlyStopsOwnedProcessOnce) -count=1` from `desktop/`
- GREEN: `go test ./... -count=1` from `desktop/`

Remaining scope:

- Task 8 has no remaining obvious deterministic gap from the current scan. Run a final root and desktop hygiene pass, then move to Task 9 candidate selection.
