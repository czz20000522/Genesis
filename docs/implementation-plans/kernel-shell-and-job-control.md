# Shell Timeout And Managed Jobs Implementation Plan

> **For agentic workers:** Steps use checkbox (`- [ ]`) syntax for tracking. Implement task-by-task with TDD. Do not skip RED verification before production code.

**Goal:** Add the first executable shell timeout and managed-job foundation while preserving the production requirement that long timeout requests are valid managed-job intent, not errors.

**Architecture:** Keep `shell_exec` as the generic tool. Add `timeout_sec` to the model-visible schema and request type. Foreground requests run through the existing shell path with a caller-selected timeout. Requests above the foreground cap create append-only job events and an immediate receipt-style `tool.result` without pretending final command output is available.

**Tech Stack:** Go, existing append-only JSONL ledger, `ToolRegistry`, `ToolGateway`, `ExecShell`, provider tool loop tests.

---

## Requirement And Design

- Requirement: `docs/requirements/kernel-shell-and-job-control.md`
- Design: `docs/design/kernel-shell-and-job-control.md`
- Active issues:
  - `KERNEL-SHELL-TIMEOUT-CAP-20260623`
  - `KERNEL-MANAGED-JOB-FOUNDATION-20260623`

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

**Deliverable:** `timeout_sec > 180` creates minimal append-only job events and an immediate model-visible receipt. The first implementation may use a fake/minimal executor that completes immediately after the receipt; it must prove event order and provider-loop closure.

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
  - events include `tool.call`, `job.started`, receipt `tool.result`, `job.completed`, and `model.final`;
  - provider receives a tool result with status `managed_job_started`;
  - final response is returned.

- [ ] **Step 2: Write failing restart projection test**

  Reload the kernel from the same ledger and verify the session projection still includes the job lifecycle events.

- [ ] **Step 3: Add job projection types**

  Add a small `JobProjection` with job id, session id, turn id, tool, status, command, cwd, timeout, receipt text, started/completed timestamps, and optional reason.

- [ ] **Step 4: Add job event helpers**

  Add helpers that append `job.started` and terminal `job.completed` events. Use the `job.started` event id as the handle.

- [ ] **Step 5: Route long timeout shell calls to managed jobs**

  In the prepared shell execution path, when `timeout_sec > 180`, append job events and return a receipt-style `ModelToolResult` instead of calling `ExecShell`.

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

  If Phase A and B-lite pass, move only the fully satisfied slices to acceptance evidence. Leave job status/cancel and interrupt as active gaps.

## Phase D-lite: Terminal Job Observation Delivery

**Deliverable:** terminal managed-job facts enter provider context through kernel-owned observation delivery and are marked delivered only after provider success.

**Fix commit:** `531f8d008`.

**Completed slice:**

- `job.completed`, `job.failed`, and `job.cancelled` are terminal observation sources.
- `ProviderContextProjection` injects undelivered terminal job facts as `kernel_observation_context`.
- `SubmitTurn` writes `kernel.observation.delivered` only after provider completion succeeds.
- Provider failure leaves observations pending.
- Restart replay suppresses already delivered observation ids.

**Still deferred from full production delivery:**

- real background process execution;
- progress snapshots such as `job.progress` or `job.output`;
- user-triggered continuation after idle job completion;
- explicit auto-resume policy, if that is ever approved.

## Still Short Of Production After This Plan

- Real background process management is not complete.
- `job_status` is not implemented.
- `job_cancel` is not implemented.
- Progress snapshot delivery and idle continuation controls are not implemented.
- Provider-stream interruption and foreground attach-or-kill behavior are not implemented.
- Stronger sandbox and approval policy remain separate future work.
