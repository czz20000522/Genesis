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
- Periodic governance review checks architecture, feature behavior, directory structure, and document lifetime together. Completed plans and stale documents should be deleted or condensed instead of spawning issues that only preserve old notes.

## Active Issues

### KERNEL-OWNER-HTTP-TRANSPORT-20260623 - P2 - HTTP transport files should stay thin delegates

- Status: open.
- Area: Architecture Governance / transport.
- Requirement: `docs/requirements/kernel-owner-structure-governance.md`.
- Design: `docs/design/kernel-owner-structure-governance.md`.
- Gap: `internal/kernel/http.go` routes turn, shell, work, memory, session, timeline, context, audit, and event surfaces in one file. Current handlers mostly delegate, but the file shape invites transport-local owner logic as routes grow.
- Next slice: Split HTTP transport by surface, keeping route matching, decode, owner API delegation, error mapping, and encode only. Add a guard that blocks ledger replay or owner state transitions inside `http*.go`.
- Evidence: `http.go` contains handlers and path parsers for tool, work, memory, session, timeline, audit, context, and turn events.
- Verification: Existing HTTP tests pass; architecture guard proves transport files do not call owner append/replay helpers directly.
- Reference alignment: Aligned with Reasonix's frontend/controller separation and Codex's protocol/event surfaces. Genesis HTTP remains a shell/adapter, not a second owner.

### KERNEL-OWNER-TOOL-CONTEXT-20260623 - P2 - Tool registrations should not receive the whole Kernel

- Status: open.
- Area: Architecture Governance / Tool Runtime.
- Requirement: `docs/requirements/kernel-owner-structure-governance.md`.
- Design: `docs/design/kernel-owner-structure-governance.md`.
- Gap: `registeredTool.Prepare` currently accepts `*Kernel`, giving every future registered tool broad access to ledger, provider, memory, work, job, and policy fields. That is too wide for a registry that should enforce least authority.
- Next slice: Introduce a narrow tool invocation context or owner-specific executor interface for registered tools. Shell/job tools receive only the authority needed to validate, authorize, execute, append operation/job evidence, and produce model-visible results.
- Evidence: `internal/kernel/tool_registry.go` defines `Prepare func(*Kernel, ...)`, and model tool handling resolves that registration before tool execution.
- Verification: New tools cannot register a `Prepare` function that receives `*Kernel`; tool registry, tool gateway, model loop, shell/job control, and architecture tests pass.
- Reference alignment: Aligned with Codex's `CoreToolRuntime` over typed `ToolInvocation` and Reasonix's `Tool` interface plus per-run `Registry`. Genesis should not pass the whole kernel object as the tool execution capability.

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
