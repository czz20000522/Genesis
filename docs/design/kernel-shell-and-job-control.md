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
7. Later job terminal facts are recorded as job events.
8. Observation delivery decides when terminal job facts enter provider context.

## Protocol

Model-visible `shell_exec` arguments include semantic command fields and `timeout_sec`.

`timeout_sec` uses seconds:

- missing means 30;
- 1 through 180 is foreground-valid;
- greater than 180 routes to managed job once that path exists;
- invalid values produce repairable `tool_request_invalid` feedback with no effect.

Managed job events:

- `tool.call`
- `job.started`
- receipt `tool.result`
- optional `job.progress` or `job.output`
- `job.completed`, `job.failed`, or `job.cancelled`

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

## Permission And Authority

Timeout and command arguments do not select permission mode, sandbox profile, workspace root, approval policy, or credential authority.

Job status and cancellation validate kernel-issued handles and apply kernel policy before revealing output or cancelling work.

## Recovery And Observability

Job facts are append-only. The original receipt is not overwritten by later completion.

Projection split:

- provider context sees receipts and delivered observation summaries;
- UI timeline may fold job lifecycle into one card;
- raw events preserve full order;
- audit shows lifecycle, timeout, cancellation, truncation, and delivery facts;
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
