# Requirement: Shell Timeout And Managed Jobs

- **Status:** approved.
- **Owner:** Tool Runtime, Work Registry, Interface Kernel, and Model Gateway.
- **Scope:** generic shell execution, foreground timeout policy, managed jobs, observation delivery, and job control.

## Background

The kernel already treats `shell_exec` as the generic way for the LLM to touch the local process environment. Short commands can return synchronously, but production use will include long tests, builds, downloads, indexing, and other processes that should not block the provider loop indefinitely.

The requirement is not to add download, build, Feishu, or email logic to the kernel. The requirement is to make generic process execution behave like a governed terminal with permission, audit, checkpoint, managed background work, and recoverable observation delivery.

## Production Target

The LLM can request a shell command with a clear foreground timeout. Short work returns terminal-equivalent command evidence. Long work becomes a managed job with an immediate receipt, append-only job lifecycle events, UI-visible progress, and controlled provider-visible observation delivery.

The kernel must distinguish:

- invalid tool requests that were never executed;
- policy blocks that were never executed;
- commands that were executed and failed or timed out as commands;
- kernel/tool infrastructure failures;
- background job lifecycle facts;
- model-visible observations already delivered to a provider step.

The provider loop must always close a model tool call with a tool result. For a managed job, that tool result is a receipt, not the final command output.

## Users And Roles

Ordinary user:

- can start work that may take longer than one foreground response;
- can see job state and final output through user-facing projections;
- can later ask to continue, inspect, or cancel.

Operator/admin:

- configures permission mode, workspace, runtime token, and future sandbox/approval settings;
- can inspect audit and raw events for job lifecycle and output truncation.

Reviewer:

- verifies timeout validation, managed-job event order, observation delivery, recovery, and cancellation evidence.

LLM:

- supplies semantic shell arguments such as command text and `timeout_sec`;
- receives repair feedback for invalid requests;
- receives terminal-equivalent command evidence for completed foreground commands;
- receives a receipt for accepted managed jobs;
- can query or cancel jobs only through generic job control after permission validation.

Kernel:

- validates timeout and arguments;
- authorizes execution;
- decides foreground versus managed-job path;
- records tool, operation, job, checkpoint, and delivery facts;
- decides when queued observations enter provider context.

Application:

- can submit turns and render job projections;
- cannot own lifecycle policy, job handles, provider observation delivery, or cancellation truth.

## Core Semantics

### Foreground Timeout

1. `shell_exec` exposes `timeout_sec` as the model-visible duration field.
2. Omitted `timeout_sec` defaults to 30 seconds.
3. Foreground synchronous shell accepts integer seconds from 1 through 180.
4. `timeout_sec=180` is the maximum foreground request.
5. `timeout_sec>180` is a valid long-task intent. It routes to managed-job admission and must not continue as ordinary synchronous shell execution.
6. Non-positive, non-integer, missing-type, or malformed timeout values produce repairable `tool_request_invalid` feedback and no effect.
7. Timeout validation happens before command execution and before any workspace or host shell side effect.

Direct HTTP `POST /tools/shell_exec` follows the same kernel owner path. It returns a foreground operation projection for `timeout_sec` values within the foreground cap, and returns a managed job projection for admitted `timeout_sec>180` requests. It does not create a parallel direct-HTTP lifecycle owner. Because direct HTTP can distinguish an omitted JSON field from an explicit value, omitted `timeout_sec` defaults to 30 seconds while explicit non-positive values are invalid. The current local managed executor requires the resolved host sandbox profile; controlled-workspace/default requests above the cap return a blocked operation until a controlled managed executor exists.
8. The model-visible schema uses seconds. Internal runtimes may use other units.

### Shell Environment Policy

1. Foreground host shell execution and local managed-job execution must receive an explicit kernel-constructed environment. They must not inherit the Genesis daemon process environment by omission.
2. The shell environment policy is a Tool Runtime / Authority Plane decision. It is not a model-visible `shell_exec` argument and cannot be selected by provider output, HTTP request fields, or UI state.
3. The minimal production policy keeps ordinary host shell execution usable by preserving platform execution basics such as path lookup, system root, temporary directory, user home, and locale where needed.
4. Credential-shaped variables are excluded before process spawn. Names or values shaped like provider keys, bearer tokens, credentials, passwords, secrets, API keys, or connector tokens do not enter the child process environment unless a future explicit credential-grant owner exists.
5. Projection redaction is not sufficient. The command process must not receive unintended daemon-local secrets in the first place; UI/session redaction only protects already-recorded evidence.
6. Foreground and managed-job shell paths use the same environment constructor so long-running work cannot bypass the foreground policy.
7. Provider-command environment remains a separate provider boundary. This policy must not weaken existing provider-command env validation.

### Terminal-Equivalent Command Results

1. If a command is accepted and executed, the tool result reports the observed process outcome.
2. A nonzero process exit is `operation.failed`, not kernel failure. The result includes exit code, bounded stdout, bounded stderr, and truncation metadata.
3. A runtime-enforced foreground timeout is a structured execution outcome with available stdout/stderr, elapsed time, timeout limit, and timeout reason. It is not malformed request feedback.
4. Shell runtime, ledger write, process-spawn infrastructure, provider adapter, and projection failures are tool or provider infrastructure failures. They are not command stderr.
5. The kernel does not decide whether the command's domain goal succeeded. It records whether the tool request was valid, authorized, executed, observed, blocked, timed out, or failed as infrastructure.

### Managed Job Events

1. A long shell request writes `tool.call`.
2. The kernel accepts the process as a managed job and writes `job.started`.
3. The `job.started` event id is the job handle. The model does not invent job ids.
4. The kernel immediately writes `tool.result` for the original tool call.
5. The immediate `tool.result` is a receipt, not the final command output.
6. The receipt is model-visible and closes the provider tool-call loop.
7. Terminal job facts are written later as `job.completed`, `job.failed`, or `job.cancelled`.
8. Non-terminal output snapshots may be written as `job.output` when they are useful durable facts. They are bounded session/UI facts, not transport chunks and not provider-visible observations by default. Snapshot persistence is bounded both per event and per job; after a live snapshot is already truncated, further live output is not recorded as `job.output` again until terminal job evidence is recorded.
9. A separate `job.progress` event may be added only when the kernel has a generic progress schema that is not tied to a domain such as download, build, or test execution.
10. Ledger events are append-only. A later output or terminal job event does not overwrite the original receipt.

### Kernel Observation Delivery

1. `job.completed`, `job.failed`, `job.cancelled`, credential blockers, quota blockers, and similar terminal or actionable system facts are Kernel Observation Queue sources.
2. Routine non-terminal `job.output` snapshots are not Kernel Observation Queue sources by default. Promoting a non-terminal snapshot into provider context requires an explicit future policy, because most progress is UI/diagnostic signal rather than model intent.
3. Idle sessions do not auto-wake the model by default.
4. Running sessions may drain pending observations at the next safe checkpoint before a provider step.
5. User input has higher priority than kernel observations when both are pending.
6. Observations are marked delivered only after the provider request that contains them has been accepted by the provider boundary.
7. Provider request failure does not mark observations delivered.
8. Restart replay must not deliver the same completed observation twice.
9. User-facing UI may show job progress immediately. Provider-visible context receives observations only through the kernel delivery rule.

### Checkpoints

A checkpoint is a boundary where the kernel can safely pause, resume, inject observations, or accept user control.

Required initial checkpoint boundaries:

- before provider step;
- after `tool.result`;
- after managed-job receipt;
- after assistant message completion;
- after assistant interruption;
- after compaction completion or failure.

Checkpoints are control-plane state. The model does not fabricate checkpoint refs or event ids.

### Interrupt And Cancellation

1. Interrupting provider streaming cancels the provider step, writes `assistant.interrupted`, and returns the session to a resumable checkpoint.
2. Interrupting an already-managed background job does not cancel it.
3. Interrupting foreground shell execution attempts to detach or attach it as a managed job when the executor supports that capability.
4. If the executor cannot attach foreground shell work as a managed job, the kernel kills the process and writes an interrupted tool result with structured evidence such as `interrupt_reason=foreground_attach_unavailable_killed`.
5. Explicit job cancellation is a separate control path. It may be invoked by a user control command or a model-visible job-control tool after permission validation.
6. Job cancellation writes cancellation request and terminal cancellation evidence. It is not represented as an ordinary nonzero command exit.

### Attach-Capable Executor Contract

Foreground attach is an executor capability, not a kernel assumption. The
kernel may convert interrupted foreground shell work into a managed job only
when the active executor explicitly advertises attach support and returns a
kernel-validated attachment result for the running process.

An attach-capable executor must satisfy:

- it can detach the foreground wait path without killing the process;
- it can transfer lifecycle observation to the managed job owner;
- it reports bounded output state without replaying unbounded live chunks;
- it lets the kernel allocate and bind the managed job handle;
- it does not expose host process id, signal, process handle, terminal handle,
  or platform-specific control token to model-visible tools or HTTP callers;
- it can report attach failure distinctly from command failure.

If attach succeeds, the kernel writes managed-job facts and returns a receipt
tool result for the interrupted tool call. If attach fails or is unsupported,
the current truthful fallback remains: kill the foreground process and write an
interrupted tool result with `foreground_attach_unavailable_killed` or another
executor-reported attach failure reason. The kernel must not forge
`job.started`, `job.attached`, or running-job projections for a process it did
not successfully take ownership of.

Replay must not duplicate the process effect. Once a foreground command has
started, restart or retry can only return recorded operation/job/interruption
facts; it must not re-run the command to recreate a missing attach fact.

### Job Control

Required eventual generic controls:

- `job_status`: inspect a managed job's current state and relevant bounded output.
- `job_cancel`: request cancellation of a managed job.

Job-control semantics:

- job-control tools validate that the referenced handle is a kernel-owned job handle;
- job-control tools do not let the model select permission mode, sandbox, owner, workspace root, or ledger ids;
- `job_status` can return running, cancel-requested, completed, failed, or cancelled states as bounded observation; an unknown handle returns repair feedback instead of a synthetic job state;
- `job_cancel` requires kernel authority through ToolGateway policy, records an explicit cancellation request when admitted, and records terminal cancellation evidence only when the executor confirms cancellation;
- model-visible job control arguments contain semantic fields only, currently a kernel-issued `job_id` and optional cancellation reason; authority, event ids, process ids, signals, and audit evidence are kernel-owned facts;
- application-specific retries remain outside the kernel unless reduced to generic job or resource primitives.

### Projection

1. Provider-visible context gets the immediate job receipt and later delivered observation summaries, not every progress tick.
2. UI timeline can show folded job cards, live progress, output preview, and terminal status.
3. Raw event inspection shows append-only event order.
4. Audit projection shows lifecycle, status, timeout, cancellation, delivery, and risk/control facts. Routine progress output remains a session/UI/raw-event fact unless it records failure, terminal outcome, or another audit-worthy transition.
5. Session projection survives restart and can show currently running or terminal jobs without re-running commands.

## Non-Goals

This requirement does not add:

- downloader-specific tools;
- Feishu, email, calendar, document, OCR, browser, build-system, or test-runner job types;
- autonomous model wakeups while a session is idle;
- OS-level sandbox promises beyond the current authority profile;
- UI ownership of job lifecycle;
- provider adapter ownership of observation delivery;
- application-specific retry, progress, or formatting logic.

## Phased Delivery

Phase A: timeout contract and validation.

- Proves: model-visible `timeout_sec`, default 30 seconds, foreground range 1 through 180, invalid value repair feedback, and no side effects before validation.
- Still short of production: managed-job lifecycle is outside this validation-only slice and must be proved by later phases.

Phase B: managed-job ledger foundation.

- Proves: `tool.call`, `job.started`, receipt `tool.result`, terminal job event, append-only event order, and provider-loop closure.
- Still short of production: this phase proves the ledger and provider-loop contract; real executor lifecycle, status, cancellation, and progress are later responsibilities.

Phase C: real job manager.

- Proves: session-scoped process registry, real process lifecycle, bounded output, terminal status, status query, cancellation, executor-reported output snapshots, and restart-safe projection.
- Still short of production: foreground attach behavior and stronger sandbox/approval integration can remain limited.

Phase D: observation delivery.

- Proves: pending/delivered observation tracking, checkpoint injection, provider-accepted delivery marking, no duplicate delivery after restart, and no idle auto-wake.
- Still short of production: advanced auto-resume policy, if ever needed, remains out of scope.

Phase E: interrupt behavior.

- Proves: assistant interruption, foreground shell kill fallback when attach is unavailable, background job survival, explicit cancellation, and separate audit facts for interruption versus cancellation.
- Still short of production until complete: true foreground attach remains constrained by executor capability.

## Acceptance Criteria

Positive cases:

- omitted timeout uses 30 seconds;
- 1 through 180 seconds are foreground-valid;
- valid foreground command returns terminal-equivalent evidence;
- values above 180 seconds enter managed-job admission rather than synchronous shell execution;
- direct HTTP shell transport returns a job receipt/projection for admitted values above 180 seconds, or a blocked operation when policy/profile admission rejects the managed executor;
- model-requested managed job writes `tool.call`, `job.started`, receipt `tool.result`, and terminal job event;
- job status and cancellation use generic job controls;
- completed job observations can enter the next provider context through kernel delivery.
- executor-reported `job.output` snapshots are durable session/UI facts and are not injected as provider observations by default.

Negative cases:

- invalid timeout values produce repair feedback and no effect;
- permission denial produces no command effect;
- model-supplied job handles or control-plane authority fields are rejected unless they reference kernel-issued handles through a valid job-control path;
- infrastructure failures are not represented as command stderr;
- application-specific job types do not enter the kernel.

Fail-closed and recovery:

- foreground timeout preserves available output and timeout metadata;
- provider request failure does not mark observations delivered;
- restart replay does not duplicate delivered observations;
- cancellation and interruption write separate auditable facts.
- routine progress snapshots do not enter strong audit or provider context unless a future policy explicitly promotes them.
- routine progress snapshots cannot grow without a per-job bound; terminal job output remains the durable bounded command result.

Audit and visibility:

- UI/session projection can fold job events into one user-facing item;
- raw event and audit projections preserve event order;
- provider-visible context receives receipts and delivered summaries, not every progress tick.

Test evidence:

- focused Tool Runtime tests for timeout validation and command taxonomy;
- Work Registry or job owner tests for job lifecycle;
- Interface/Model Gateway tests for observation delivery;
- architecture tests proving no application-specific job owner enters the kernel;
- full build and test evidence before issue retirement.

## Relationship To Existing Issues

This requirement governs these implementation slices:

- `KERNEL-SHELL-TIMEOUT-CAP-20260623`: `ready_for_acceptance` for `timeout_sec`, default/cap behavior, invalid value repair feedback, and routing above the cap.
- `KERNEL-MANAGED-JOB-FOUNDATION-20260623`: `ready_for_acceptance` for managed-job event model and receipt-style tool result.
- `KERNEL-FOREGROUND-TIMEOUT-OUTCOME-20260623`: `ready_for_acceptance` for foreground runtime timeout as terminal-equivalent command evidence with timeout metadata and available output.
- `KERNEL-OBSERVATION-DELIVERY-20260623`: `ready_for_acceptance` for pending/delivered observation tracking and checkpoint delivery semantics.
- `KERNEL-JOB-PROGRESS-IDLE-CONTINUATION-20260623`: remaining true foreground attach semantics after the local managed executor sparse-output and kill-fallback slices.

`KERNEL-SANDBOX-APPROVAL-NEXT-20260623` is adjacent authority-plane work governed by `docs/requirements/kernel-foundation-capabilities.md`.

Issues under this requirement should record only the current implementation gap, evidence, next slice, and verification needed for that slice. They should not restate this full production contract.
