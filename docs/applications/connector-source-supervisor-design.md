# Design: Connector Source Supervisor And Verification

- **Requirement:** `docs/applications/connector-source-supervisor-requirement.md`
- **Owner:** user-space application connector runtime
- **Status:** approved

## Boundary Model

Connector Source Supervisor is a user-space source lifecycle owner. It sits
before request mapping and kernel submission.

```text
External Source
        |
        v
Source Adapter
        | source_command NDJSON frames
        |
        v
Source Supervisor
        |
        v
SourceRun / SourceAttempt / SourceCursor / SourceFailure
        |
        v
ExternalEvent
        |
        v
Application Connector Runtime
        |
        v
Kernel HTTP primitives
        |
        v
Genesis Kernel
```

The supervisor owns source lifecycle and event authenticity evidence. It does
not own application policy, session mapping, outbox delivery, or kernel facts.

The source adapter owns external protocol translation. For Feishu, that means
the adapter can know how to call `lark-cli`, an SDK, HTTP API, or webhook
server. The connector runtime cannot know those command lines, protocol keys,
payload envelopes, or credential mechanics. Its only inbound source interface is
the typed `source_command` stream.

## Owner Responsibilities

Source Supervisor owns:

- source adapter start, stop, probe, retry, and backoff;
- source run status and blocked/degraded reasons;
- source attempt records and failure references;
- source cursor persistence and resume markers;
- event validation classification: `verified`, `unchecked`, or `rejected`;
- source failure records with bounded, redacted diagnostics.
- validation of `source_command` frame shape before any `ExternalEvent` is
  emitted.

Source Adapter owns:

- external source protocol details such as webhook body, CLI argv, SDK request,
  HTTP response, platform event key, or identity flag;
- protocol-specific parsing into source frames;
- protocol-local readiness assertion frames;
- protocol-local verification evidence assertion frames when it has evidence.

The source adapter does not write connector stores, kernel ledger facts, memory
facts, tool results, checkpoints, or outbox receipts.

Application Connector Runtime owns:

- inbound dedupe after a valid `ExternalEvent` exists;
- external thread/user/message mapping to application request context;
- application session mapping and kernel session mapping;
- kernel primitive calls such as `turn.submit`;
- outbox creation from application policy or kernel projection.

Kernel owns:

- authority, credentials, permission, sandbox, provider context, model execution,
  tools, memory, jobs/work, event ledger, checkpoints, recovery, and audit.

## Data Shapes

`SourceRun`:

```text
source_id
connector
adapter_ref
status: starting / ready / degraded / blocked / stopped
started_at
stopped_at
last_ready_at
blocked_reason
```

`ready` means the source adapter surface is usable. It does not mean inbound
events are verified.

`SourceAttempt`:

```text
attempt_id
source_run_id
started_at
ended_at
outcome
failure_ref
```

The attempt outcome records start, poll, consume, probe, or adapter execution
results. Retry policy reads these records but does not turn external failures
into kernel turn failures.

`SourceCursor`:

```text
source_id
cursor_kind
cursor_value
watermark_at
updated_at
```

Cursor values are connector-owned progress markers. They are not kernel truth,
not public ids, and not model-visible. A cursor may be derived from offset,
watermark, external message id, or adapter-specific checkpoint, but the
connector must not expose the raw external cursor as a Genesis authority value.
Cursor persistence is at-least-once. A cursor is persisted only after the event
or batch it refers to has been durably accepted into connector-owned processing.
If a cursor frame names an event the runtime did not accept, the runtime records
a source failure and leaves cursor state unchanged.

`SourceVerificationEvidence`:

```text
source_event_ref
validation_status: verified / unchecked / rejected
evidence_kind
evidence_ref
checked_at
adapter_ref
```

`verified` requires explainable evidence. `unchecked` is the default when a
source is operational but no authenticity evidence exists. `rejected` means the
event was not accepted into connector request processing.

Verification evidence must be inspectable before a real source can emit
`source_validation=verified`. A write-only evidence path is not sufficient for
production verification because operators and tests must be able to audit why a
source event became trusted.

## Source Command Boundary

`source_command` is a long-running source adapter protocol. It is intentionally
separate from outbound `connector_command`.

```text
Application Connector Runtime
  -> starts SourceAdapter process
  -> reads source_command NDJSON frames
  -> validates frame shape and source identity
  -> writes SourceRun / SourceAttempt / SourceCursor / SourceFailure /
     SourceVerificationEvidence
  -> emits ExternalEvent

Feishu Source Adapter
  -> owns lark-cli / SDK / HTTP / webhook details
  -> emits source.ready / source.event / source.cursor / source.failed /
     source.stopped frames
```

Frame kinds:

```text
source.ready:
  source_id
  connector
  adapter_ref

source.event:
  source_id
  event
  verification_evidence?   # required when event.source_validation=verified
  cursor?                  # candidate cursor, saved only after event accept

source.cursor:
  source_id
  cursor
  after_event_id           # must refer to an accepted source event

source.failed:
  source_id
  connector
  event_source
  reason
  detail
  payload_hash?
  payload_size_bytes?

source.stopped:
  source_id
  reason?
```

Unknown frame kind, malformed JSON, unknown fields, unsafe diagnostic content,
missing required fields, a verified event without evidence, or a cursor that
does not follow an accepted event fails closed. The runtime records a redacted
`SourceFailureRecord` and does not emit an `ExternalEvent`.

`source_id` is stable source identity. Profile, account, credential, token, or
adapter process configuration is binding/readiness state, not part of the
stable source id unless changing it truly means the external message source is a
different historical stream.

`SourceFailureRecord`:

```text
failure_ref
source_run_ref
source_attempt_ref
reason_code
redacted_summary
payload_hash
payload_size_bytes
debug_ref
resource_ref
created_at
```

`debug_ref` and `resource_ref` are optional. If raw payload retention is
required, it must live behind a restricted debug trace or resource/object owner
with TTL, quota, redaction, and access controls.

## Readiness Versus Verification

Readiness answers whether a source adapter can operate. Examples:

- executable or external adapter process is available;
- profile exists and can be used;
- event consume or poll command exists;
- webhook endpoint is bound;
- credential check does not report missing, expired, or revoked state.

Verification answers whether a specific event or event batch is authentic.
Examples:

- webhook signature validation passed;
- platform token or challenge validation passed;
- a trusted external adapter returned a verified assertion with evidence;
- an operator-approved local source provided a signed or otherwise trusted
  batch assertion.

Readiness never upgrades an event to verified. A Feishu `lark-cli event consume`
source that starts successfully emits unchecked events until the connector has
durable event authenticity evidence.

## Source Lifecycle

Source lifecycle is connector-local:

```text
configured -> starting -> ready
configured -> starting -> blocked
ready -> degraded -> ready
ready -> degraded -> blocked
ready -> stopped
degraded -> stopped
blocked -> starting
```

`starting` records that an attempt is underway. `ready` records last successful
readiness evidence. `degraded` records a recoverable failure with backoff or
operator warning. `blocked` records a required operator action, such as missing
profile, expired credential, revoked credential, unsupported source command, or
policy rejection. `stopped` records an operator or supervisor stop.

Each start, probe, consume, or poll attempt creates a `SourceAttempt`. Attempts
must be bounded and observable. A repeated source failure updates connector
source state and failure records, not kernel facts.

## Cursor And Dedupe Interaction

The source cursor prevents source reprocessing after restart. Inbound dedupe
prevents repeated `ExternalEvent` processing after a valid event is emitted.
They are related but not the same owner responsibility.

- Source Supervisor owns cursor persistence before or during source intake.
- Application Connector Runtime owns inbound event idempotency and request
  dedupe before `turn.submit`.
- Kernel receives only the final application request and does not know source
  cursor details.

Cursor advancement must be conservative. If an event cannot be parsed,
validated, or durably accepted, the supervisor records a source failure and does
not advance the cursor. The exact cursor strategy is adapter-specific, but the
durable fact must stay connector-local.

## Credential And Profile Readiness

This design does not introduce a credential store. It only defines readiness
outcomes:

- profile or credential ok: source may start;
- profile or credential missing, expired, or revoked: source is blocked;
- refresh required: source is degraded or blocked with an operator action.

Credentials, tokens, and profiles are not model-visible, not copied into
`ExternalEvent`, and not used to grant kernel authority. A future credential
broker can replace the readiness probe implementation without changing source
supervisor ownership.

## Failure Semantics

Malformed, unauthenticated, policy-rejected, unsupported, or adapter-failed
source data must be classified before kernel submission.

The supervisor writes a bounded `SourceFailureRecord` for source failures. The
record can include reason code, redacted summary, payload hash, payload size,
source run reference, source attempt reference, and optional debug/resource ref.
It must not make raw payload a durable source fact.

Failure categories:

- `source_not_ready`: adapter or credential readiness failed before intake;
- `source_runtime_failed`: adapter process, poller, or webhook source failed;
- `source_payload_malformed`: payload could not be parsed into source shape;
- `source_verification_failed`: authenticity verification failed;
- `source_policy_rejected`: connector source policy rejected the event;
- `source_cursor_failed`: cursor read or write failed.

Only accepted source events become `ExternalEvent`. Rejected source inputs do
not become kernel turns.

## Observability

Operator surfaces should show:

- current source runs by connector and source id;
- source status, last ready time, blocked reason, and last failure;
- recent source attempts and outcomes;
- source cursor summary without raw external cursor disclosure when sensitive;
- validation status counts: verified, unchecked, rejected;
- source failures with redacted diagnostics and debug/resource references.

Debug trace may show richer adapter details only when explicitly enabled and
subject to TTL, quota, redaction, and access boundary.

## Reference Alignment

Codex app-server daemon patterns are relevant because daemon lifecycle,
readiness, lock, and process state stay behind a boundary instead of becoming
core turn truth.

Codex transport authentication and signature checks are relevant because
external authenticity is established at the boundary before core execution, not
in the model prompt.

Reasonix readiness, retry, and event projection patterns are relevant because
adapters can report operational state while the controller remains the owner of
core execution semantics.

These references align on the principle that external protocol health and
authenticity evidence must be normalized before entering the core runtime.

## Rejected Alternatives

- Treating `lark-cli` readiness as event verification. Rejected because adapter
  availability does not authenticate individual events.
- Putting Feishu listener supervision inside kernel. Rejected because Feishu is
  a user-space connector, not a kernel owner.
- Letting the supervisor call `turn.submit` or map sessions. Rejected because
  those are Application Connector Runtime responsibilities.
- Keeping a `while true lark-cli` loop as the production supervisor. Rejected
  because it lacks source run, attempt, cursor, readiness, and failure
  semantics.
- Moving `lark-cli event consume ...` into an argv or string-template
  configuration. Rejected because it preserves protocol drift, argument
  escaping, output-shape, and credential handling problems inside Genesis
  runtime configuration. Feishu command syntax belongs to a Feishu source
  adapter implementation behind typed frames.
- Reusing outbound `connector_command` for inbound source streams. Rejected
  because outbound actions are bounded request/result calls, while inbound
  sources are long-running lifecycle, event, cursor, verification, and failure
  streams.
- Persisting raw malformed payloads as source facts. Rejected because durable
  facts must be sparse, redacted, and bounded; raw payload belongs only in
  restricted debug/resource storage when explicitly enabled.
