# Shell Timeout And Managed Jobs Implementation Plan

> **For agentic workers:** Steps use checkbox (`- [ ]`) syntax for tracking. Implement task-by-task with TDD. Do not skip RED verification before production code.

**Goal:** Add the first executable shell timeout and managed-job foundation while preserving the production requirement that long timeout requests are valid managed-job intent, not errors.

**Architecture:** Keep `shell_exec` as the generic tool. Add `timeout_sec` to the model-visible schema and request type. Foreground requests run through the existing shell path with a caller-selected timeout. Requests above the foreground cap create append-only job events and an immediate receipt-style `tool.result` without pretending final command output is available.

**Tech Stack:** Go, file-backed event ledger with SQLite index, `ToolRegistry`, `ToolGateway`, `ExecShell`, provider tool loop tests.

---

## Requirement And Design

- Requirement: `docs/requirements/kernel-shell-and-job-control.md`
- Design: `docs/design/kernel-shell-and-job-control.md`
- Active issues:
  - `KERNEL-JOB-PROGRESS-IDLE-CONTINUATION-20260623`

## Phase A: Foreground Timeout Contract

**Deliverable:** `shell_exec` accepts `timeout_sec`; omitted defaults to 30; 1 through 180 runs foreground; invalid values produce repairable feedback and no effect.

**Files:**

- Modify: `internal/kernel/tool_registry.go`
- Modify: `internal/kernel/model_tools.go`
- Modify: `internal/kernel/shell.go`
- Modify: `internal/kernel/process_runtime.go`
- Modify: `internal/kernel/types.go`
- Test: `internal/kernel/kernel_test.go`
- Test: `internal/kernel/skill_catalog_test.go`

**Red lines:**

- Do not let the model set permission mode, sandbox profile, workspace root authority, approval policy, operation id, event id, or job id.
- Do not treat `timeout_sec > 180` as validation error.
- Do not add application-specific tool names.

- [ ] **Step 1: Write failing tests for model-visible timeout schema**

  Add assertions that `shell_exec` manifest contains `timeout_sec` with numeric/integer semantics and still rejects unknown control-plane fields.

- [ ] **Step 2: Write failing tests for foreground timeout validation**

  Add table tests for:

  - omitted timeout defaults to 30 seconds;
  - `timeout_sec=1` is accepted;
  - `timeout_sec=180` is accepted;
  - `timeout_sec=0`, negative, string, and fractional values produce `tool_request_invalid`;
  - invalid timeout creates no operation/effect.

- [ ] **Step 3: Implement minimal request/schema support**

  Add `TimeoutSec` to shell argument/request structures and pass it through `ToolGateway` to `ExecShell`.

- [ ] **Step 4: Implement foreground timeout execution**

  Replace the hard-coded shell process timeout with the validated request timeout for host shell execution.

- [ ] **Step 5: Run focused tests**

  Run:

  ```powershell
  D:\software\Go\bin\go.exe test ./internal/kernel -run "TestSubmitTurn.*Timeout|TestSubmitTurnProjectsRegisteredToolManifestWithoutSkillCatalogContext" -count=1
  ```

## Phase B-lite: Managed Job Receipt Event Foundation

> Historical phase note: this closed phase established the receipt and append-only event protocol. Phase F-lite supersedes its temporary executor behavior. Do not use this section to reintroduce terminal job facts from the receipt path.

**Deliverable:** `timeout_sec > 180` creates minimal append-only job events and an immediate model-visible receipt. The phase proves event order and provider-loop closure; real executor behavior is owned by Phase F-lite and later slices.

**Files:**

- Modify: `internal/kernel/types.go`
- Modify: `internal/kernel/model_tools.go`
- Modify: `internal/kernel/kernel.go`
- Modify: `internal/kernel/projections.go`
- Create or modify: `internal/kernel/jobs.go`
- Test: `internal/kernel/kernel_test.go`

**Red lines:**

- The job handle is kernel-generated. The model does not supply it.
- The original `tool.result` remains a receipt and is not overwritten by final job output.
- No `job_status`, `job_cancel`, interrupt, or observation-delivery loop in this phase.
- UI/raw/audit projections can expose job facts, but provider context only needs the receipt in this phase.

- [ ] **Step 1: Write failing provider-loop test for managed-job receipt**

  A provider requests `shell_exec` with `timeout_sec=181`. The test expects:

  - no foreground operation is created;
  - events include `tool.call`, `job.started`, receipt `tool.result`, and `model.final`;
  - terminal job events are produced later by the managed executor rather than forged inside the receipt path;
  - provider receives a tool result with status `managed_job_started`;
  - final response is returned.

- [ ] **Step 2: Write failing restart projection test**

  Reload the kernel from the same ledger and verify the session projection still includes the job lifecycle events.

- [ ] **Step 3: Add job projection types**

  Add a small `JobProjection` with job id, session id, turn id, tool, status, command, cwd, timeout, receipt text, started/completed timestamps, and optional reason.

- [ ] **Step 4: Add job event helpers**

  Add helpers that append `job.started` events and terminal job facts from executor completion. Use the `job.started` event id as the handle.

- [ ] **Step 5: Route long timeout shell calls to managed jobs**

  In the prepared shell execution path, when `timeout_sec > 180`, append `job.started`, return a receipt-style `ModelToolResult`, and hand the command to the managed executor instead of calling foreground `ExecShell`.

- [ ] **Step 6: Run focused tests**

  Run:

  ```powershell
  D:\software\Go\bin\go.exe test ./internal/kernel -run "TestSubmitTurn.*ManagedJob|TestSubmitTurn.*Timeout" -count=1
  ```

## Phase C: Verification And Issue State

**Deliverable:** focused and full verification evidence for the first two issues. Do not retire job control, cancellation, or interrupt issues.

- [ ] **Step 1: Run contract scans**

  Run:

  ```powershell
  $pattern = '/v' + '[0-9]|policy_' + 'version|compaction_' + 'v1|' + 'skill\\.' + 'read|read_' + 'skill|skill_' + 'read'
  rg -n $pattern AGENTS.md README.md docs internal features
  ```

- [ ] **Step 2: Run focused architecture test**

  Run:

  ```powershell
  D:\software\Go\bin\go.exe test ./internal/kernel -run TestArchitectureBoundary -count=1
  ```

- [ ] **Step 3: Run full Go verification**

  Run:

  ```powershell
  D:\software\Go\bin\go.exe test ./... -count=1
  D:\software\Go\bin\go.exe build ./...
  ```

- [ ] **Step 4: Update issue ledger or retirement evidence**

  If Phase A and B-lite pass, move only the fully satisfied slices to acceptance evidence. At Phase C time, job status/cancel and interrupt were still active gaps; current active gap state must be read from `docs/operations/kernel-issues.md`.

## Phase D-lite: Terminal Job Observation Delivery

**Deliverable:** terminal managed-job facts enter provider context through kernel-owned observation delivery and are marked delivered only after provider success.

**Fix commit:** `531f8d008`.

**Completed slice:**

- `job.completed`, `job.failed`, and `job.cancelled` are terminal observation sources.
- `ProviderContextProjection` injects undelivered terminal job facts as `kernel_observation_context`.
- `SubmitTurn` writes `kernel.observation.delivered` only after provider completion succeeds.
- Provider failure leaves observations pending.
- Restart replay suppresses already delivered observation ids.

**Still deferred from full production delivery at Phase D time:**

- executor-reported `job.output` snapshots, later completed by Phase H-lite;
- local live-output sampling and foreground attach behavior;
- explicit auto-resume policy, if that is ever approved.

## Phase E-lite: Minimal Job Control Tools

**Deliverable:** model-visible generic job control with `job_status` and `job_cancel`. This phase completes the first job-control surface. At Phase E-lite time it did not implement provider-stream interruption, foreground attach-or-kill, or real background process management; Phase F-lite below adds the first real managed executor boundary.

**Fix commit:** `ce72dfa44`.

**Completed slice:**

- The model-visible manifest includes `shell_exec`, `job_status`, and `job_cancel`, with no process-level `job_terminate`.
- `job_status` replays current job state from the session ledger and creates no operations.
- `job_cancel` records a semantic `job.cancel_requested` fact for a non-terminal job. Terminal cancellation is executor-owned in Phase F-lite.
- Cancelling completed, failed, or already cancelled jobs returns the current terminal state without writing a competing terminal fact.
- Unknown job ids and model-supplied process/control-plane fields return structured repair feedback.

**Files:**

- Modify: `internal/kernel/tool_registry.go`
- Modify: `internal/kernel/model_tools.go`
- Modify: `internal/kernel/jobs.go`
- Modify: `internal/kernel/types.go`
- Test: `internal/kernel/kernel_test.go`

**Red lines:**

- The model supplies a kernel-issued `job_id` and optional semantic cancel reason only.
- The model never supplies process id, signal, force flag, permission mode, sandbox profile, workspace root, event id, or audit refs.
- `job_status` is read-only and must not create operations or strong audit facts.
- `job_cancel` is a semantic request. The kernel decides executor behavior; the model does not choose terminate mechanics.
- Terminal jobs are idempotent under cancellation. A cancel request against a completed, failed, or already cancelled job returns the current terminal state instead of creating a competing terminal fact.

- [x] **Step 1: Write failing model-tool manifest tests**

  Assert the registered model-visible tools include `shell_exec`, `job_status`, and `job_cancel`, with no application-specific job tools or process-level terminate tool.

- [x] **Step 2: Write failing job status tests**

  Cover:

  - completed managed job returns `status=completed`;
  - unknown job id returns structured `job_not_found` feedback;
  - model-supplied control-plane fields are rejected as `tool_request_invalid`;
  - restart replay can still answer status from the ledger;
  - status query creates no operation.

- [x] **Step 3: Write failing job cancel tests**

  Cover:

  - cancelling a running job records `job.cancel_requested` and does not forge `job.cancelled` before executor confirmation;
  - cancelling a terminal job returns the terminal state without writing a competing terminal event;
  - duplicate cancel request is idempotent;
  - unknown job id returns structured `job_not_found`;
  - model-supplied terminate mechanics are rejected.

- [x] **Step 4: Implement job lookup projection**

  Add a small job lookup helper that replays current job state from ledger events. It must remain generic and independent of application domains.

- [x] **Step 5: Register and execute job control tools**

  Register `job_status` as read-side-effect and `job_cancel` as write-side-effect. Both route through `ToolGateway`, return model-visible JSON, and preserve tool-call closure.

- [x] **Step 6: Run focused tests**

  Run:

  ```powershell
  D:\software\Go\bin\go.exe test ./internal/kernel -run "TestSubmitTurn.*JobStatus|TestSubmitTurn.*JobCancel|TestSubmitTurnRoutesLongShellTimeoutToManagedJobReceipt" -count=1
  ```

## Still Short After Phase E-lite

- Progress snapshot delivery and idle continuation controls are not implemented.
- Provider-stream interruption and foreground attach-or-kill behavior are not implemented.
- Stronger sandbox and approval policy remain separate future work.

## Phase F-lite: Minimal Managed Executor Boundary

**Deliverable:** long-running `shell_exec` requests start a real kernel-owned managed shell job after the receipt `tool.result` is recorded. `job_cancel` records a semantic cancellation request and asks the executor to cancel the live job; terminal `job.completed`, `job.failed`, or `job.cancelled` facts are written only by executor completion.

**Reference scan:**

- Codex app-server separates `turn/interrupt` from process control. `turn/interrupt` completes a turn as interrupted, while `process/spawn`, `process/kill`, and `process/exited` are separate process APIs. Genesis follows the same boundary by keeping model-visible `job_cancel` semantic and hiding process ids/signals behind the kernel executor.
- Reasonix `internal/jobs` uses a session-scoped background job manager whose jobs survive turns and are cancelled explicitly or on controller close. Genesis follows that lifetime model for local managed shell jobs, while retaining Genesis's append-only ledger as the durable fact source.
- Reasonix shell cancellation kills the process tree (`taskkill /T` on Windows, process group kill on Unix). Genesis uses the same executor-level responsibility and does not expose those mechanics to the model.

**Completed slice:**

- `timeout_sec > 180` records `job.started`, appends the receipt `tool.result`, then starts a local managed shell executor.
- The receipt is no longer overwritten or followed by a synthetic terminal job fact.
- The default executor runs shell commands under a cancellable session-scoped manager and records terminal job facts with exit code and bounded stdout/stderr.
- `job_cancel` records `job.cancel_requested` but does not forge `job.cancelled`; live executor completion writes the terminal cancellation fact.
- Ledger-only running jobs can receive a cancellation request without pretending a host process was terminated.

**Still deferred from full production delivery at Phase F time:**

- executor-reported `job.output` snapshots, later completed by Phase H-lite;
- local live-output sampling;
- explicit auto-resume policy, if approved later;
- provider-stream interruption;
- foreground shell attach-or-kill behavior on user interrupt;
- stronger sandbox/approval integration for arbitrary host shell execution.

## Phase G-lite: Turn Interrupt Control

**Deliverable:** user-space shells can submit a typed interrupt command for the active session turn. Provider-step interruption records a durable `assistant.interrupted` fact and does not cancel existing managed jobs. Foreground shell interruption records an `operation.interrupted` outcome and closes the original model tool call with an interrupted tool result.

**Reference scan:**

- Codex app-server marks stale in-progress turns as interrupted when thread status resolves away from active, and keeps background-terminal list/terminate APIs separate from turn interruption. Genesis follows that split by keeping `assistant.interrupted` as a turn/control fact and `job_cancel` as the explicit managed-job cancellation path.
- Codex app-server exposes background terminal termination through a process-specific app-server surface, not as the model's generic tool result. Genesis keeps process ids, signals, and `taskkill` mechanics behind the managed executor and does not expose them through `job_cancel` or interruption payloads.
- Reasonix ACP sessions hold a per-turn `context.CancelFunc`; `session/cancel` aborts the active turn and reports `stopReason=cancelled`. Genesis follows the same per-turn cancellation model through `InterruptSession`, while recording the durable `assistant.interrupted` event in the kernel ledger.
- Reasonix provider history repair fills interrupted tool-call holes with an explicit interrupted placeholder when resuming. Genesis's foreground shell interrupt path instead records an actual `tool.result(status=interrupted)` before closing the current turn, so provider history remains paired without synthetic provider-adapter repair.

**Completed slice:**

- `InterruptSession` is the kernel-owned typed control command for active turn cancellation.
- `POST /sessions/{id}/interrupt` is a thin HTTP delegate that authenticates, decodes, calls `InterruptSession`, and returns `202 Accepted` for an active turn.
- Active provider-step cancellation writes `assistant.interrupted`, returns `ErrTurnInterrupted`, projects the turn as `interrupted`, and does not write `turn.failed`.
- Interrupting a later provider turn does not call the managed job executor's `Cancel` path for an existing background job.
- Interrupting foreground `shell_exec` through the active turn context writes `operation.interrupted`, writes a paired `tool.result(status=interrupted)`, and then records `assistant.interrupted` without calling the provider for a final answer.
- UI timeline, raw event, audit replay, session projection, and idempotency replay recognize interrupted turns and interrupted operations.

**Still deferred from full production delivery:**

- local live-output sampling;
- explicit auto-resume policy, if approved later;
- attach/detach of an already-running foreground shell into a managed job when an executor supports that capability;
- stronger sandbox/approval integration for arbitrary host shell execution.

## Phase H-lite: Managed Job Output Snapshot Projection

**Deliverable:** managed executors can report sparse non-terminal output snapshots through the kernel-owned job boundary. The kernel records those snapshots as `job.output`, projects them to session/UI/raw-event surfaces, and keeps them out of default provider observation delivery.

**Reference scan:**

- Codex app-server exposes background terminal list/terminate as control-plane surfaces, keeping process lifecycle separate from turn interruption. Genesis keeps `job.output` as a job fact and does not turn it into a provider-owned stream.
- Reasonix `internal/tool/progress.go` uses a `ProgressFunc` context sink for live frontend progress, and `internal/jobs` drains only completion summaries into the next turn. Genesis follows the split by allowing executor progress reports while only terminal job facts enter `kernel_observation_context` by default.

**Completed slice:**

- `ManagedJobStartRequest` exposes an `Observe(JobProjection)` callback for executor-originated non-terminal snapshots.
- The job owner records snapshots as `job.output` only if the latest job state is still non-terminal.
- `job.output` replays into session job projection and UI timeline output preview.
- `job.output` is not a pending kernel observation source and does not create `kernel_observation_context` on the next provider step.
- Routine `job.output` is not promoted into audit replay summaries; raw event inspection still preserves the append-only fact.

**Later completed in current implementation:**

- The local managed shell executor reports sparse live stdout/stderr snapshots through bounded `job.output` facts.
- Foreground shell interruption uses executor capability detection, attaches when supported, and falls back to a truthful killed-interrupt result when attach is unavailable.
