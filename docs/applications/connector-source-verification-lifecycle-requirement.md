# Requirement: Connector Source Verification And Lifecycle

- **Status:** approved
- **Owner:** user-space application connector runtime
- **Scope:** external source readiness, event authenticity evidence, source lifecycle state, attempts, cursors, operator controls, and source failure diagnostics

## Background

Application connectors need external message sources: Feishu event streams,
WeChat callbacks, email pollers, webhook receivers, console inputs, and future
desktop or local automation sources. These sources are not kernel owners. They
are user-space application infrastructure that bring external events into
Genesis canonical world.

The current Feishu smoke listener proves the application line can receive a
message and submit a kernel turn, but production source handling needs a harder
boundary. A command that starts successfully, a profile that exists, or an
event consume command that is available only proves source adapter readiness.
It does not prove that any specific external event is authentic.

Core decision:

```text
Source Verification And Lifecycle owns external source readiness, lifecycle,
and event authenticity evidence.
Application Connector Runtime owns request mapping and kernel submission.
Kernel owns authority, execution, facts, recovery, memory, tools, and audit.
Source readiness never implies event verification.
```

## Production Target

Connector Source Verification And Lifecycle is the connector-local owner for
source readiness, source lifecycle, and event authenticity evidence. It tracks
source run state, records source attempts, applies source backoff, records
blocked/degraded/stopped lifecycle facts, records source cursors, and emits
normalized `ExternalEvent` values only after source parsing and validation have
produced a clear result.

This owner must distinguish:

- source adapter readiness: the configured source surface can start or be
  probed;
- event verification: a specific event or event batch has explainable
  authenticity evidence;
- event rejection: the event is malformed, unauthenticated, outside policy, or
  otherwise unsuitable for connector processing.

`source_validation=verified` is allowed only when verification evidence is
attached to the event or event batch and the evidence itself is inspectable,
adapter-bound, source-bound, and event-bound. If this evidence does not exist,
the event remains `source_validation=unchecked`, even when the source adapter
is ready.

Approved evidence kinds are:

- `webhook_signature`: a platform webhook signature was verified against the
  exact event or batch payload;
- `provider_event_signature`: a provider event signature, token, challenge, or
  equivalent provider-issued authenticity proof was verified;
- `trusted_local_adapter_attestation`: a connector-configured local adapter,
  already declared as a trust anchor for this source binding, asserts that it
  performed external verification and provides an evidence reference.

`trusted_local_adapter_attestation` is not a blanket shortcut. It can upgrade
an event only when the adapter binding is configured as trusted for the source,
the evidence status is `verified`, and the evidence binds the source, adapter,
and event or batch. If the local adapter cannot provide that proof, the event
stays `unchecked`.

This owner does not decide whether Genesis should reply, does not map
external threads to kernel sessions, does not choose skills, does not call
`turn.submit`, and does not build provider context. Those remain the
Application Connector Runtime and kernel responsibilities.

This owner also does not own context quality. A Feishu, WeChat, email, webhook,
or console source may deliver a normalized application request, but it must not
summarize, truncate, compact, or rewrite provider context. Long inbound sessions
are handled by kernel/session compaction commands and provider-context
projections after the request reaches the kernel.

## Users And Roles

Operators inspect source runs, source attempts, readiness, blocked reasons,
source failures, cursor state, and validation evidence before trusting an
inbound connector flow.

Source adapters translate one external source protocol into typed source
frames. They may internally use a webhook server, polling loop, event stream,
external SDK, HTTP API, or CLI, but those protocol details stay inside the
adapter implementation. The connector runtime consumes the `source_command`
stream contract; it does not own Feishu, WeChat, email, webhook, or CLI command
syntax.

Connector Source Verification And Lifecycle owns source lifecycle, retry,
readiness, event validation classification, source failure records, verification
evidence, and source cursor persistence.

Application Connector Runtime owns request context, inbound dedupe, application
session mapping, kernel session mapping, `turn.submit`, and outbox creation.

Genesis Kernel owns authority, provider context, tool execution, ledger facts,
memory, jobs, checkpoints, recovery, and audit. External source identity never
becomes kernel authority by itself.

The LLM sees only the request context and application-approved event content
that the connector runtime chooses to submit. It does not see source cursors,
raw payloads, source credentials, profile tokens, or source lifecycle internals.

## Core Semantics

`SourceReadiness` reports whether a configured source adapter can start, be
probed, or keep running. It may depend on executable availability, profile
existence, credential presence, source command availability, network
reachability, or adapter health. Readiness never upgrades an event to verified.

`SourceVerification` reports authenticity evidence for an event or event batch.
It records the validation status, evidence kind, evidence reference, checked
time, and adapter reference. A source event without this evidence is
`unchecked`, not `verified`.

`SourceVerificationEvidence` must bind:

```text
source_event_ref or source_batch_ref
source_id
connector
adapter_ref
validation_status: verified / unchecked / rejected
evidence_kind
evidence_ref
checked_at
```

`source_validation=verified` requires `validation_status=verified`, a non-empty
approved `evidence_kind`, a non-empty `evidence_ref`, and matching source,
connector, adapter, and event or batch references. Weak evidence, mismatched
bindings, write-only evidence, or unverifiable claims must fail closed before
kernel submission.

`SourceRun` is the durable connector-local run record for one configured source:

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

`SourceAttempt` is the connector-local record for one start, poll, consume, or
adapter execution attempt:

```text
attempt_id
source_run_id
started_at
ended_at
outcome
failure_ref
```

`SourceCursor` is a connector-owned offset, cursor, watermark, message id, or
other source progress marker. It is not kernel truth, not model-visible, and not
an application identity. It exists only to resume source intake without
duplicating external events.

Cursor semantics are at-least-once plus connector dedupe. A source adapter may
emit cursor candidates, but the runtime may persist a cursor only after the
corresponding source event has been durably accepted into connector-owned
processing. A cursor frame that refers to an event the runtime did not accept is
a source failure, not progress.

`source_id` identifies the stable external message source. It must not include
short-lived credential or profile material when that material can refresh
without changing the source's historical identity. Profile, account, tenant,
and token posture belong to adapter binding and readiness evidence.

`source_command` is the long-running inbound adapter boundary. It is separate
from outbound `connector_command` because the interaction shape is different:

```text
source_command:     long-running process -> source.ready / source.event /
                    source.cursor / source.failed / source.stopped frames
connector_command:  ConnectorAction request -> ConnectorActionResult
```

The source command process emits newline-delimited JSON frames. The runtime
validates frame shape, records source lifecycle facts, records verification
evidence, records bounded failures, and emits normalized `ExternalEvent` values.
The adapter cannot write connector stores, kernel ledger facts, memory facts,
tool results, checkpoints, or outbox receipts.

`SourceFailureRecord` is the durable source failure fact. It may store reason
code, redacted summary, hash, payload size, source run reference, source attempt
reference, and optional debug or resource reference. It must not store raw
external payload as durable fact.

Raw external payloads are allowed only in restricted debug trace or a future
resource/object owner with TTL, quota, redaction, and access boundary. A
malformed Feishu payload, webhook body, email, or poll response must therefore
produce a bounded failure record, not a raw durable source fact.

Credential and profile handling is readiness only in this requirement. It is
not a kernel credential plane and not a model-visible capability.

```text
credential/profile ok -> source may start
credential/profile missing/expired/revoked -> source blocked
refresh required -> source degraded or blocked with operator action
```

Readiness reason codes:

- `missing_profile`: the configured adapter profile does not exist or cannot be
  resolved;
- `profile_expired`: the profile exists but can no longer authenticate;
- `permission_denied`: the profile exists but lacks source permission;
- `refresh_required`: a credential/profile refresh is needed before source
  intake can continue;
- `operator_action_required`: automatic handling is intentionally stopped until
  an operator fixes or approves the source;
- `source_command_invalid`: the adapter command boundary is invalid before
  execution;
- `source_runtime_failed`: the adapter process, poller, or webhook receiver
  failed after the source started.

This requirement does not define a full credential broker or automatic refresh
flow. It only defines the readiness/blocking facts that a connector adapter or
future credential owner must report.

Operator lifecycle controls are connector-owned. The first production surface
must let an operator inspect source run, attempt, cursor, verification, and
failure state; acknowledge or clear a blocked state after remediation; request
restart/retry for a source owned by an external process supervisor; and
explicitly reset or replay cursor state when the operator accepts the
duplicate-processing risk. These controls cannot write kernel facts, fabricate
inbound events, fabricate verification evidence, or grant kernel authority.

## Non-Goals

- No Feishu, WeChat, email, or webhook source owner inside kernel.
- No source-lifecycle-owned session mapping, dedupe, `turn.submit`, provider context,
  skill selection, application reply strategy, or outbox policy.
- No inference that `lark-cli` process readiness means event authenticity.
- No raw source payload in durable source failure facts.
- No full credential store, automatic refresh broker, or credential authority in
  this requirement.
- No production commitment to a hardcoded external CLI command shape.
- No model-visible source cursor, profile token, external credential, or raw
  source transport value.
- No connector-specific reconciliation probe in this requirement. Delivery
  reconciliation remains outbound recovery work and depends on connector outbox
  receipt fields such as action ref, idempotency key, or external receipt ref.

## Phased Delivery

### Phase A: Requirement And Design Boundary

Document the source verification and lifecycle contract, readiness versus
verification distinction, run/attempt/cursor/failure semantics, readiness reason
codes, operator lifecycle controls, and the owner split between connector
source lifecycle, Application Connector Runtime, and Kernel.

### Phase B: Connector-Local Source State

Implement `SourceRun`, `SourceAttempt`, `SourceCursor`, and source readiness
projection in connector-local storage. A smoke Feishu source can remain
`unchecked` until real verification evidence exists.

### Phase C: Source Adapter Boundary

Replace hardcoded source command shape with the `source_command` typed streaming
boundary. Runtime code starts a source adapter process, validates typed frames,
records `SourceRun`, `SourceAttempt`, `SourceCursor`, `SourceFailureRecord`, and
`SourceVerificationEvidence`, and emits normalized `ExternalEvent` values. It
does not know `lark-cli event consume`, Feishu event keys, identity flags, SDK
payloads, webhook body shapes, or other external source protocol details.

The first Feishu source adapter may internally use `lark-cli`, SDK, HTTP, or
webhook details, but those details are adapter-owned and observable to the
runtime only through source frames.

This phase also includes bounded generic retry/backoff for recoverable
`source_command` process failures. Retry applies to the source adapter process
boundary, not to malformed events, kernel submission policy, session mapping,
or external protocol translation. Blocked source readiness failures must not be
retried as if they were transient runtime failures.

### Phase D: Event Verification

Add event or event-batch verification evidence for sources that support it:
webhook signatures, provider event signatures, trusted local adapter
attestations, or equivalent connector-approved evidence. Only then can source
validation become `verified`.

### Phase E: Operator Lifecycle Controls

Add operator inspection, blocked-state acknowledgement/reset, source retry or
restart request, cursor reset/replay request, and recovery notes for source
runs. These commands mutate connector source state only and cannot fabricate
kernel facts.

## Acceptance Criteria

- A ready source adapter does not produce `source_validation=verified` unless
  per-event or per-batch verification evidence exists.
- A verified source event carries validation status, evidence kind, and an
  evidence reference that can be inspected without exposing secrets.
- Verification evidence binds source id, connector, adapter ref, and event or
  batch reference; a ready adapter or existing profile alone cannot produce
  `source_validation=verified`.
- Missing profile, expired profile, permission denial, refresh requirement, and
  operator-action-required states block source intake before kernel submission.
- A malformed or rejected source payload writes a redacted `SourceFailureRecord`
  with reason code, hash, size, source run reference, and optional debug/resource
  reference, but no raw payload in durable facts.
- A malformed source frame writes a redacted `SourceFailureRecord` and does not
  emit an `ExternalEvent`.
- Source cursor progress can resume intake without duplicating external events,
  while remaining connector-local and model-invisible.
- Source cursor progress advances only after the referenced event has been
  durably accepted by connector-owned processing.
- Missing, expired, or revoked profile/credential state blocks or degrades the
  source before kernel submission.
- The source lifecycle owner emits `ExternalEvent` values but does not create request
  context, map sessions, submit turns, write outbox items, or build provider
  context.
- Operator lifecycle commands mutate only connector-owned source state. They
  cannot write kernel facts, fabricate `ExternalEvent`, fabricate verification
  evidence, or infer outbound delivery success.
- External source identity and source validation status do not grant kernel
  permission, sandbox, credential, memory, or tool authority.
- Runtime source code contains no Feishu-specific event consume argv, identity
  flag, event key, or source protocol parser; those live only in Feishu source
  adapter code.
- `SourceVerificationEvidence` is inspectable before any source is allowed to
  produce `source_validation=verified` events.
