# Design: Shell Timeout And Managed Jobs

- **Status:** approved.
- **Requirement:** `docs/requirements/kernel-shell-and-job-control.md`.
- **Owner:** Tool Runtime, Work Registry, Interface Kernel, and Model Gateway.

## Boundary And Owner

Tool Runtime owns shell argument validation, permission admission, process execution, bounded output, and tool-result projection.

Work Registry owns generic managed-job lifecycle facts when a shell process becomes long-running work.

Interface Kernel owns checkpoints, interruption state, and session control commands.

Model Gateway owns provider-loop closure and provider-visible observation delivery.

User-space shells and applications may render job status and submit user control commands. They do not own job handles, lifecycle facts, provider observation delivery, or cancellation truth.

## Data Flow

Foreground command:

1. Provider requests `shell_exec`.
2. ToolGateway validates arguments and `timeout_sec`.
3. ToolGateway authorizes the effect.
4. Executor runs within the resolved authority profile.
5. Tool Runtime records operation evidence.
6. ToolGateway emits `tool.result` with terminal-equivalent command evidence.
7. Model Gateway continues or completes the provider loop.

Managed job:

1. Provider requests `shell_exec` with a long-task timeout or another long-task signal.
2. ToolGateway validates and authorizes the request.
3. Kernel records `tool.call`.
4. Work/job owner records `job.started`.
5. ToolGateway emits an immediate receipt-style `tool.result` for the original tool call.
6. The provider loop is closed without waiting for final job output.
7. The managed executor may report sparse non-terminal output snapshots; the job owner force-binds kernel-owned job identity and records them as `job.output` only while the job is still non-terminal.
8. Later job terminal facts are recorded as job events.
9. Observation delivery decides when terminal job facts enter provider context.

## Protocol

Model-visible `shell_exec` arguments include semantic command fields and `timeout_sec`.

`timeout_sec` uses seconds:

- missing means 30;
- 1 through 180 is foreground-valid;
- greater than 180 routes to managed-job admission;
- invalid values produce repairable `tool_request_invalid` feedback with no effect.

Direct `POST /tools/shell_exec` uses the same timeout routing. It decodes omitted `timeout_sec` as the 30 second default, rejects explicit non-positive values before effects, returns foreground operation projections for foreground-valid requests, and returns managed job projections for admitted values above 180 seconds. The current local managed executor requires the resolved host sandbox profile; controlled-workspace/default requests above the cap return a blocked operation until a controlled managed executor exists. The HTTP transport does not own managed-job lifecycle; it delegates to the same ToolGateway admission, job ledger, and executor path as model-requested long shell work.

Managed job events:

- `tool.call`
- `job.started`
- receipt `tool.result`
- optional `job.output`
- `job.completed`, `job.failed`, or `job.cancelled`

`job.output` is a durable output snapshot for session/UI/raw-event projections. It is not a stream chunk, not a strong audit event by default, and not a provider-visible observation source unless a future checkpoint policy explicitly promotes it. Executors report output content; the kernel binds session id, job id, turn id, tool id, command/cwd, timeout, receipt, and status from the current job state so executor-supplied identity/control fields cannot redirect or rewrite job facts. Local live output sampling is also lifetime-bounded: a job may record only a small fixed number of durable non-terminal snapshots, and once a snapshot is already truncated, further live output is not recorded as `job.output` again until the terminal job fact. A separate `job.progress` event requires a generic progress schema before it can enter the protocol.

The `tool.call` and receipt `tool.result` events are provider-loop facts. Direct HTTP long shell requests write job lifecycle facts and return a job projection, but they do not forge model tool-call events.

Job-control commands:

- `job_status`
- `job_cancel`

The job handle is a kernel-issued job event id, not a model-created id.

## Failure Semantics

- Invalid timeout or malformed arguments: no effect, repairable `tool_request_invalid`.
- Permission denial: no effect, policy evidence, model-visible denial feedback.
- Nonzero command exit: executed command result with exit code, stdout, stderr, and truncation metadata.
- Foreground timeout: executed command outcome with timeout metadata and available output.
- Tool infrastructure failure: shell runtime, ledger, process-spawn, or projection failure; not command stderr.
- Provider failure: Model Gateway failure; does not mark queued observations delivered.
- Cancellation: explicit job-control fact; not an ordinary command exit.
- Interruption: session/control fact; distinct from cancellation.

Foreground shell interruption is capability-gated. If the active executor cannot attach foreground shell work into a managed job, the kernel cancels the foreground process and returns an interrupted tool result with `interrupt_reason=foreground_attach_unavailable_killed`. That fallback records the absence of attach support without exposing process ids, signals, or host process handles to the model or transport callers.

## Foreground Attach Design

Foreground attach is a future executor seam. The kernel must not infer attach
support from the operating system or from a process id. It asks the active
executor whether foreground attach is supported for the already-admitted
operation.

Current flow:

```text
foreground shell running
  -> user/session interrupt
  -> executor attach capability = false
  -> kill foreground process
  -> write interrupted operation/tool result
  -> no managed job facts are forged
```

Future attach-capable flow:

```text
foreground shell running
  -> user/session interrupt
  -> executor attach capability = true
  -> executor detaches foreground wait path
  -> kernel allocates managed job handle and binds ownership
  -> tool.result returns managed-job receipt
  -> executor reports later job.output and terminal job facts
```

Attach result validation belongs to the kernel. The executor can report that it
has attached a process stream, but it cannot choose job id, session id, turn id,
tool id, checkpoint refs, permission mode, sandbox profile, or audit refs.

Attach failure is neither command stderr nor terminal command failure. It is an
executor/control failure that must produce structured interruption evidence.
If the failure leaves process ownership uncertain, the kernel must prefer a
blocked or interrupted state with operator-visible diagnostics over pretending a
managed job exists.

Reference alignment:

- Codex has background terminal/session controls where long work can continue
  and later be terminated or observed, but process ids stay test/support details
  rather than model authority.
- Reasonix wires a session-scoped job manager so background jobs can outlive a
  turn and be cancelled by controller lifecycle. It treats background work as a
  runtime-managed resource, not as prompt text.
- Genesis keeps the current local executor explicit: it does not advertise
  foreground attach, so the kill fallback remains the only truthful behavior
  until an attach-capable executor is introduced.

## Permission And Authority

Timeout and command arguments do not select permission mode, sandbox profile, workspace root, approval policy, or credential authority.

Job status and cancellation validate kernel-issued handles and apply kernel policy before revealing output or cancelling work.

## Recovery And Observability

Job facts are append-only. The original receipt is not overwritten by later completion.

Projection split:

- provider context sees receipts and delivered observation summaries;
- UI timeline may fold job lifecycle and `job.output` snapshots into one card by tool-call id when present and by kernel job id for direct shell transports;
- raw events preserve full order;
- audit shows lifecycle, timeout, cancellation, truncation, delivery, and control/risk facts without promoting routine progress output into a strong audit summary;
- session projection survives restart.

Observation delivery uses checkpoints:

- idle sessions do not auto-wake;
- running sessions may drain pending observations before a provider step;
- observations are marked delivered only after provider request acceptance;
- provider failure does not lose observations;
- restart replay does not deliver the same terminal observation twice.

## Rejected Alternatives

- Blocking the provider loop for long shell work is rejected because it prevents checkpointing and recovery.
- Returning final job output as the original tool result is rejected because the provider tool-call loop needs an immediate closure receipt.
- Letting UI or daemons push observations directly into model context is rejected because Model Gateway owns provider projection.
- Application-specific job types are rejected because managed jobs are a generic kernel primitive.
