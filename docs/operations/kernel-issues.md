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
- When an issue removes a concept from the current kernel contract, long-term tests must assert the positive replacement behavior. Do not keep permanent tests whose only purpose is locking retired names, aliases, routes, or historical helper APIs out of the tree; use temporary scans or retirement-log evidence for cleanup windows, then fold the guard into the current owner contract.

## Active Issues

### KERNEL-SHELL-TIMEOUT-CAP-20260623 - P1 - Foreground shell timeout policy and cap

- Status: open.
- Area: Tool Runtime / shell execution policy.
- Problem: The current `shell_exec` path has a fixed short execution shape. It does not yet expose a model-visible `timeout_sec`, enforce the foreground default of 30 seconds, cap ordinary foreground execution at 180 seconds, or convert requests above the cap into managed work.
- Suggestion: Add `timeout_sec` as the model-visible duration field for `shell_exec`. Missing value defaults to 30 seconds. Values from 1 through 180 run as foreground shell attempts. Values above 180 are a valid long-task intent and must route into the managed-job path rather than continuing as a synchronous shell. Non-positive or malformed values return repairable `tool_request_invalid` feedback and do not execute.
- Evidence: Current permission work resolved `permission_mode` into `authority_policy`, `sandbox_profile`, and `approval_policy`, but timeout ownership has not been split from the shell execution path.
- Verification: A model tool call without `timeout_sec` runs with the 30 second default; `timeout_sec=180` is accepted as foreground; `timeout_sec=181` returns an immediate managed-job receipt; invalid timeout values return repair feedback and produce no effect.
- Reference alignment: Aligned with Codex-style tool-loop boundaries where short tools close with a tool result, while long-running work must move behind a managed process/job abstraction. Reasonix also keeps frontend shells behind a controller rather than letting an adapter own lifecycle policy.

### KERNEL-MANAGED-JOB-FOUNDATION-20260623 - P1 - Minimal managed job event model

- Status: open.
- Area: Work Registry / Tool Runtime.
- Problem: The kernel contract mentions future long-running jobs, but the current implementation has no `job.started`, `job.completed`, job handle, receipt-style `tool.result`, or job inspection surface. Without this, downloads, long tests, builds, and other long commands either block the provider loop or must be killed instead of checkpointed.
- Suggestion: Introduce the minimal job event sequence for long shell work: `tool.call`, `job.started`, immediate `tool.result` receipt, and terminal `job.completed` or `job.failed`. The `tool.result` closes the provider tool-call loop with a receipt such as "accepted as managed job" rather than pretending the final command result is available. The job event id is the kernel-generated handle; the model must not invent one.
- Evidence: `docs/kernel-contract.md` currently says long-running kernel-owned jobs are future events, and current shell operation projection is only synchronous operation evidence.
- Verification: A shell request above the foreground cap does not block the provider step, writes a `job.started` event, returns a model-visible receipt as `tool.result`, writes a terminal job event through a fake or minimal executor, and survives ledger replay.
- Reference alignment: Aligned with Codex's separation between tool-call closure and managed process lifecycle. The intentional difference is scope: Genesis should start with a generic job primitive, not a coding-agent-specific task runner.

### KERNEL-OBSERVATION-DELIVERY-20260623 - P1 - Kernel observation queue and delivery checkpoints

- Status: open.
- Area: Interface Kernel / Provider context projection / Ledger.
- Problem: Job completion and other kernel observations need a delivery model. The kernel currently has provider context projection and checkpoints, but no explicit rule for which background observations have been delivered to the model and which remain pending.
- Suggestion: Treat terminal job facts and similar system facts as Kernel Observation Queue sources. Idle sessions must not auto-wake the model by default. Running sessions may drain new observations at the next safe checkpoint before a provider step. Mark observations as delivered only after the provider request is accepted, so provider failure does not silently lose facts.
- Evidence: The current ledger is restart-safe for turns, operations, memory, and compaction evidence, but it does not track observation delivery ids for future job completions.
- Verification: A completed background job is visible in UI/session projection immediately, does not start a provider call while the session is idle, is included in the next provider context when the session resumes or continues, and is not delivered twice after restart.
- Reference alignment: Aligned with Codex's core/session ownership of compaction and tool-loop state: shells submit typed commands and observations; the core decides when provider context incorporates them. This rejects the drift where an external daemon or UI secretly drives model execution.

### KERNEL-JOB-CONTROL-INTERRUPT-20260623 - P2 - Interrupt and job control semantics

- Status: open.
- Area: Tool Runtime / session control.
- Problem: The current minimal kernel does not define how user interruption interacts with provider streaming, foreground shell execution, or already-managed background jobs. This will become ambiguous as soon as foreground cap and managed jobs exist.
- Suggestion: Specify and implement the minimal behavior: interrupting provider streaming cancels the provider step and records `assistant.interrupted`; interrupting a foreground shell attempts to detach into a managed job when the executor supports it, otherwise kills and records an interrupted tool result; interrupting an already-managed job does not cancel it. Add a separate `job_cancel` control path for explicit cancellation.
- Evidence: Current shell and provider paths are short synchronous paths. There is no job handle or cancel owner yet, so cancellation cannot be audited separately from ordinary command failure.
- Verification: Interrupting assistant output does not kill an existing background job; explicit job cancel writes cancel request and terminal cancel evidence; interrupted foreground shell behavior is deterministic and reflected in tool/session projections.
- Reference alignment: Aligned with Codex's distinction between session/control events and process lifecycle. Genesis should keep cancellation as a kernel command or model-visible job-control tool, not as UI-local behavior.

### KERNEL-SANDBOX-APPROVAL-NEXT-20260623 - P2 - Stronger sandbox and approval policy beyond the minimal profile split

- Status: open.
- Area: Authority Plane / Tool Runtime.
- Problem: The current foundation correctly separates `permission_mode`, `authority_policy`, `sandbox_profile`, and `approval_policy`, but `approval_policy` is always `never`, and `default` uses a controlled workspace adapter rather than an OS-level sandbox. That is acceptable for the first ground layer, but not enough for broader arbitrary command execution.
- Suggestion: Keep the current split as the owner path. Future stronger sandboxing must be selected by kernel-owned profile resolution and reported through readiness; future approval must be a typed control-plane flow and never a model-supplied escalation field. Do not let `shell_exec` arguments select permission, sandbox, approval, workspace root, or authority policy.
- Evidence: Current docs and tests now state `controlled_workspace` is not an OS sandbox and provider-visible tool results must not include permission/profile control-plane fields.
- Verification: The existing positive contract remains true; when a stronger sandbox or approval flow is added, unknown or unavailable sandbox profiles fail closed, approval denial returns structured feedback without execution, and model-supplied control-plane fields are rejected as repairable invalid requests.
- Reference alignment: Aligned with Codex's sandbox/approval split and Reasonix's central controller model. The active drift risk is over-promising `default` as a real OS sandbox or turning approval into shell/UI-local logic.
