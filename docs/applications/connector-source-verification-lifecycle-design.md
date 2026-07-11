# Design: Connector Source Verification And Lifecycle

- **Requirement:** `docs/applications/connector-source-verification-lifecycle-requirement.md`
- **Owner:** user-space application connector runtime
- **Status:** approved

## Boundary Model

Connector Source Verification And Lifecycle is a user-space source readiness,
verification, and lifecycle owner. It sits before request mapping and kernel
submission.

```text
External Source
        |
        v
Source Adapter
        | source_command NDJSON frames
        |
        v
Source Verification / Lifecycle
        |
        v
SourceRun / SourceAttempt / SourceCursor / SourceFailure
        |
        v
SourceVerificationEvidence
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

This owner records source readiness, lifecycle, cursor, failure, and event
authenticity evidence. It does not own application policy, session mapping,
outbox delivery, reconciliation probes, or kernel facts.

It also does not own context quality. Source adapters and supervisors must not
summarize inbound history, truncate provider context, emit
`context.compaction.*` events, or rewrite kernel history. Their output stops at
validated source facts and normalized `ExternalEvent` values; any compaction is
admitted later by the kernel/session owner.

The source adapter owns external protocol translation. For Feishu, that means
the adapter can know how to call `lark-cli`, an SDK, HTTP API, or webhook
server. The connector runtime cannot know those command lines, protocol keys,
payload envelopes, or credential mechanics. Its only inbound source interface is
the typed `source_command` stream.

## Owner Responsibilities

Connector Source Verification And Lifecycle owns:

- source adapter start/probe/retry/backoff posture;
- source run status and blocked/degraded reasons;
- source attempt records and failure references;
- source cursor persistence and resume markers;
- event validation classification: `verified`, `unchecked`, or `rejected`;
- verification evidence shape and source/event/adapter binding;
- operator lifecycle control records for source retry/reset/replay requests;
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
source_batch_ref
source_id
connector
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

Approved `evidence_kind` values:

```text
webhook_signature
provider_event_signature
trusted_local_adapter_attestation
```

`webhook_signature` and `provider_event_signature` are strong evidence kinds
when the connector can verify a provider-issued signature, challenge, token, or
equivalent proof against the exact event or batch.
`trusted_local_adapter_attestation` is accepted only when the configured
adapter binding is explicitly trusted for that source and emits a verified,
inspectable, source-bound, adapter-bound, event-bound evidence reference.

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

For a verified event, the runtime checks that the evidence status is
`verified`, `evidence_kind` is approved, `evidence_ref` is non-empty, and the
source id, connector, adapter ref, and event or batch ref match the configured
source binding. A ready source adapter cannot self-upgrade an event to verified
by omitting evidence or by sending unchecked evidence.

`source_id` is stable source identity. Profile, account, credential, token, or
adapter process configuration is binding/readiness state, not part of the
stable source id unless changing it truly means the external message source is a
different historical stream.

## Feishu Profile Readiness Probe

The Feishu source adapter exposes a separate `--profile-probe` mode for the
generic `profile-probe-command` boundary. It runs only `lark-cli auth status`
with the runtime-supplied explicit profile and emits one
`ProfileReadinessCommandResult`; it never starts event consumption, sends a
message, prints credentials, or creates source frames.

The first mapping is deliberately conservative:

- a structured Feishu CLI `config/not_configured` result becomes
  `missing_profile`;
- a bot identity with `available=true` and `status=ready` becomes ready;
- every other exit, malformed result, unavailable bot identity, or unknown
  upstream status becomes `operator_action_required`.

The adapter must not infer `profile_expired`, `permission_denied`, or
`refresh_required` from prose. Those more specific facts require an upstream
typed status or a separately approved Feishu probe contract. This is readiness
evidence only and cannot mark any source event verified.

## Connector Binding Configuration

The connector runtime is generic, but adapter configuration is typed. The
current user-home runtime settings bind Feishu by explicit `enabled`, bot
profile, identity, and adapter policy. A non-test Feishu listener reads that
binding before it creates source lifecycle state or launches an adapter:

```text
binding missing or enabled=false -> listener refuses before source start
binding enabled=true -> resolve typed Feishu profile/identity -> profile probe
                         -> source_command -> lifecycle owner
```

`stdin-jsonl` remains an isolated deterministic test surface and does not
enable a real listener. Future mail, WeChat, or QQ adapters use the same
enablement gate and generic lifecycle/outbox owners, while their protocol
configuration remains in their adapter binding. This avoids both accidental
background listeners and a fake universal channel schema.

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

`source_validation=verified` requires evidence whose own status is verified,
whose kind and reference are non-empty, and whose adapter binding matches the
configured source adapter. Weak assertions such as `validation_status=unchecked`
or adapter-mismatched evidence must be rejected before `ExternalEvent` emission.

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

Readiness reason codes are explicit and stable:

```text
missing_profile
profile_expired
permission_denied
refresh_required
operator_action_required
source_command_invalid
source_runtime_failed
```

These reason codes describe connector readiness only. They do not become
kernel authority, model context, or event verification.

Each start, probe, consume, or poll attempt creates a `SourceAttempt`. Attempts
must be bounded and observable. A repeated source failure updates connector
source state and failure records, not kernel facts.

Minimum source command supervision is generic. The runtime may retry a
`source_command` process after recoverable runtime failures with bounded
attempts and backoff, but blocked readiness failures such as missing command,
invalid executable, invalid environment, missing profile, or credential posture
must fail closed without retry churn. The retry loop owns timing; process
attempts and run status are recorded at the source command intake boundary. The
source adapter still owns external protocol details and only emits typed
frames. Handler errors after a valid `ExternalEvent` is emitted belong to
Application Connector Runtime or kernel submission; they must not be
reclassified as source runtime failures, written into source run diagnostics, or
repaired by restarting the source adapter.

## Cursor And Dedupe Interaction

The source cursor prevents source reprocessing after restart. Inbound dedupe
prevents repeated `ExternalEvent` processing after a valid event is emitted.
They are related but not the same owner responsibility.

- Connector Source Verification And Lifecycle owns cursor persistence before or
  during source intake.
- Application Connector Runtime owns inbound event idempotency and request
  dedupe before `turn.submit`.
- Kernel receives only the final application request and does not know source
  cursor details.

Cursor advancement must be conservative. If an event cannot be parsed,
validated, or durably accepted, the connector source lifecycle owner records a
source failure and does not advance the cursor. The exact cursor strategy is
adapter-specific, but the durable fact must stay connector-local.

## Credential And Profile Readiness

This design does not introduce a credential store. It only defines readiness
outcomes:

- profile or credential ok: source may start;
- profile or credential missing, expired, or revoked: source is blocked;
- refresh required: source is degraded or blocked with an operator action.

Credentials, tokens, and profiles are not model-visible, not copied into
`ExternalEvent`, and not used to grant kernel authority. A future credential
broker can replace the readiness probe implementation without changing source
lifecycle ownership.

If a profile is missing, expired, denied, or requires refresh, the source enters
`blocked` or `degraded` before kernel submission. The adapter may provide a
safe readiness reason and operator action hint, but it must not persist raw
tokens, authorization headers, profile secrets, or platform credential payloads.

The current Feishu ingress surface supports a connector-local profile readiness
command probe. The probe is a direct executable boundary that receives the
configured profile as a generated argument and returns typed readiness JSON. It
does not start source adapters, send messages, call the kernel, or expose
credentials to the model. Unsupported readiness values, command failure, or
malformed output fail closed as connector-local operator action requirements.
The probe has its own bounded timeout; explicit `ready=false` without a
supported reason and timed-out probes both fail closed before source or delivery
adapters start. This is an application connector boundary, not a kernel
credential store.

## Operator Lifecycle Controls

Operator controls mutate connector-owned source state only. They are not a
kernel control plane and cannot manufacture inbound events or verification
evidence.

Minimum controls:

- inspect source runs, attempts, cursors, verification evidence, and source
  failures;
- acknowledge or clear a blocked state after an operator fixes the cause;
- request retry or restart for a source owned by an external process
  supervisor;
- reset or replay a cursor only after the operator accepts duplicate-processing
  risk;
- record a connector-local operator note or recovery record.

Stop and restart are requests unless the connector runtime owns a concrete
process handle. When an external daemon owns the listener, the connector
records desired state and recovery facts, not a fake stopped/restarted fact.

Operator controls cannot write kernel ledger events, alter kernel sessions,
write memory truth, fabricate `ExternalEvent`, fabricate
`SourceVerificationEvidence`, or mark outbound delivery as reconciled.

## Failure Semantics

`SourceFailureRecord` is a connector fact, not a raw adapter payload mirror.
Failure records use configured source identity where available; untrusted frame
fields cannot override connector, source run, or adapter identity. Adapter
provided payload hashes are only accepted when they match the canonical
`sha256:<hex>` shape. Raw payloads stay out of durable failure facts unless a
separate resource/debug owner with TTL, quota, and access boundaries stores
them.

Malformed, unauthenticated, policy-rejected, unsupported, or adapter-failed
source data must be classified before kernel submission.

Connector Source Verification And Lifecycle writes a bounded
`SourceFailureRecord` for source failures. The record can include reason code,
redacted summary, payload hash, payload size, source run reference, source
attempt reference, and optional debug/resource ref. It must not make raw payload
a durable source fact.

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

For profile probing specifically, Codex's account processor reads auth state
behind an account boundary and does not expose token material merely to report
status (`codex-rs/app-server/src/request_processors/account_processor.rs`).
Genesis follows that posture: the adapter emits only the bounded readiness
classification, while the CLI output and any credential material remain outside
connector state and model context.

These references align on the principle that external protocol health and
authenticity evidence must be normalized before entering the core runtime.

## Rejected Alternatives

- Treating `lark-cli` readiness as event verification. Rejected because adapter
  availability does not authenticate individual events.
- Putting Feishu listener supervision inside kernel. Rejected because Feishu is
  a user-space connector, not a kernel owner.
- Letting the source lifecycle owner call `turn.submit` or map sessions. Rejected because
  those are Application Connector Runtime responsibilities.
- Keeping a `while true lark-cli` loop as the production source lifecycle
  implementation. Rejected because it lacks source run, attempt, cursor,
  readiness, and failure
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
- Treating outbound reconciliation probes as part of source lifecycle. Rejected
  because reconciliation operates on connector outbox delivery receipts and
  requires exact action refs, idempotency keys, or external receipt refs before
  it can produce connector-local evidence.
