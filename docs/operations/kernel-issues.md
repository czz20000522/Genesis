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

## Active Issues

### KERNEL-JOB-CONTROL-INTERRUPT-20260623 - P2 - Interrupt and managed executor semantics

- Status: open.
- Area: Tool Runtime / session control.
- Requirement: `docs/requirements/kernel-shell-and-job-control.md`.
- Design: `docs/design/kernel-shell-and-job-control.md`.
- Gap: The kernel now has a minimal managed shell executor behind long-running `shell_exec` receipts and `job_cancel` can reach live executor state, but interrupt semantics for provider streaming and foreground shell execution are still missing. Progress snapshots and idle continuation policy are also still deferred.
- Next slice: Define provider-stream interrupt and foreground shell detach-or-kill behavior. The model-visible `job_cancel` surface must stay semantic; any process ids, signals, `taskkill`, or process group mechanics remain hidden behind the kernel-owned executor.
- Evidence: `ce72dfa44` registers `job_status` and `job_cancel`, replays job status from the ledger, returns `job_not_found` repair feedback for unknown handles, rejects process/control-plane fields, and records idempotent cancel facts for non-terminal jobs. `ea2c6aab8` starts live shell jobs for `timeout_sec > 180`, writes no fake immediate terminal event, records `job.cancel_requested` before executor cancellation, and writes `job.cancelled` only after executor confirmation.
- Verification: Existing verification covers the minimal tool surface and live managed executor cancellation. Remaining verification must prove assistant-output interruption does not kill a background job by default and interrupted foreground shell behavior is deterministic.
- Reference alignment: Aligned with Codex's distinction between session/control events and process lifecycle. Genesis should keep cancellation as a kernel command or model-visible job-control tool, while process mechanics stay behind a kernel-owned managed executor.

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

### KERNEL-DIRECT-SHELL-MANAGED-JOB-PARITY-20260623 - P2 - Direct shell transport does not share managed-job routing

- Status: open.
- Area: Tool Runtime / HTTP transport.
- Requirement: `docs/requirements/kernel-shell-and-job-control.md`.
- Design: `docs/design/kernel-shell-and-job-control.md`.
- Gap: The model-visible `shell_exec` path routes `timeout_sec > 180` to a managed-job receipt, but direct `POST /tools/shell_exec` still calls `ExecShell` and returns an operation-shaped response. That direct transport can therefore run a long timeout synchronously instead of returning a managed-job handle.
- Next slice: Decide the direct tool transport contract: either make `/tools/shell_exec` foreground-only with explicit validation and docs, or introduce a response union/projection that can return managed-job receipts without pretending they are ordinary operations. Do not duplicate a second managed-job owner in HTTP.
- Evidence: `README.md` still documents direct `/tools/shell_exec` as active; `internal/kernel/http.go` routes it to `ExecShell`; `internal/kernel/model_tools.go` routes model `timeout_sec > 180` through `startManagedShellJobReceipt`.
- Verification: Add transport-level tests for `timeout_sec > 180`, invalid direct timeout values, and idempotency behavior before moving this issue to acceptance.
- Reference alignment: Aligned with Codex/Reasonix's principle that shells/transports submit into the same owner path rather than growing parallel lifecycle semantics. The drift risk is letting direct HTTP become a second shell execution contract.

### KERNEL-FOREGROUND-TIMEOUT-OUTCOME-20260623 - P2 - Foreground timeout lacks structured outcome evidence

- Status: open.
- Area: Tool Runtime / process execution.
- Requirement: `docs/requirements/kernel-shell-and-job-control.md`.
- Design: `docs/design/kernel-shell-and-job-control.md`.
- Gap: Foreground runtime timeout uses process context cancellation but the resulting operation does not distinguish a timeout outcome from an ordinary nonzero exit or shell runtime failure, and operation evidence lacks timeout reason or elapsed-time fields.
- Next slice: Define and implement the foreground timeout outcome shape. The command should remain an executed command result with bounded stdout/stderr when available, plus timeout limit and timeout reason metadata. It must not be reported as malformed request feedback.
- Evidence: `internal/kernel/process_runtime.go` uses `context.WithTimeout`; `internal/kernel/types.go` operation evidence has `timeout_sec` but no elapsed/timeout reason fields; current tests cover accepted timeout values on quick commands, not a process that actually exceeds the foreground cap.
- Verification: Add tests for a foreground command that times out, preserves bounded output when available, records timeout metadata, and is projected consistently to model-visible tool results and inspection surfaces.
- Reference alignment: Aligned with Codex-style terminal-equivalent results: kernel reports what the terminal/runtime observed without judging command semantics, while timeout policy remains explicit owner evidence.
