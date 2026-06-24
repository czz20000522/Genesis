# Requirement: Connector Source Supervisor And Verification

- **Status:** approved
- **Owner:** user-space application connector runtime
- **Scope:** external source lifecycle, readiness, event authenticity evidence, source run state, attempts, cursors, and source failure diagnostics

## Background

Application connectors need long-running external message sources: Feishu event
streams, WeChat callbacks, email pollers, webhook receivers, console inputs,
and future desktop or local automation sources. These sources are not kernel
owners. They are user-space application infrastructure that bring external
events into Genesis canonical world.

The current Feishu smoke listener proves the application line can receive a
message and submit a kernel turn, but production source handling needs a harder
boundary. A command that starts successfully, a profile that exists, or an
event consume command that is available only proves source adapter readiness.
It does not prove that any specific external event is authentic.

Core decision:

```text
Source Supervisor owns external source lifecycle and event authenticity evidence.
Application Connector Runtime owns request mapping and kernel submission.
Kernel owns authority, execution, facts, recovery, memory, tools, and audit.
Source readiness never implies event verification.
```

## Production Target

The Connector Source Supervisor is the connector-local owner for source
lifecycle and event authenticity evidence. It starts and stops source adapters,
tracks source run state, records source attempts, applies backoff, determines
whether a source is ready, degraded, blocked, or stopped, and emits normalized
`ExternalEvent` values only after source parsing and validation have produced a
clear result.

The supervisor must distinguish:

- source adapter readiness: the configured source surface can start or be
  probed;
- event verification: a specific event or event batch has explainable
  authenticity evidence;
- event rejection: the event is malformed, unauthenticated, outside policy, or
  otherwise unsuitable for connector processing.

`source_validation=verified` is allowed only when verification evidence is
attached to the event or event batch. Valid evidence can include webhook
signature validation, platform token validation, mutually trusted adapter
assertion, or another connector-approved verification result with an evidence
reference. If this evidence does not exist, the event remains
`source_validation=unchecked`, even when the source adapter is ready.

The supervisor does not decide whether Genesis should reply, does not map
external threads to kernel sessions, does not choose skills, does not call
`turn.submit`, and does not build provider context. Those remain the
Application Connector Runtime and kernel responsibilities.

## Users And Roles

Operators inspect source runs, source attempts, readiness, blocked reasons,
source failures, cursor state, and validation status before trusting an inbound
connector flow.

Source adapters translate one external source protocol into connector source
records. They may use a webhook server, polling loop, event stream, external
adapter process, SDK, or command template, but they do not own application
policy or kernel submission.

The Source Supervisor owns source lifecycle, retry, readiness, event validation
classification, source failure records, and source cursor persistence.

Application Connector Runtime owns request context, inbound dedupe, application
session mapping, kernel session mapping, `turn.submit`, and outbox creation.

Genesis Kernel owns authority, provider context, tool execution, ledger facts,
memory, jobs, checkpoints, recovery, and audit. External source identity never
becomes kernel authority by itself.

The LLM sees only the request context and application-approved event content
that the connector runtime chooses to submit. It does not see source cursors,
raw payloads, source credentials, profile tokens, or source supervisor internals.

## Core Semantics

`SourceReadiness` reports whether a configured source adapter can start, be
probed, or keep running. It may depend on executable availability, profile
existence, credential presence, source command availability, network reachability,
or adapter health. Readiness never upgrades an event to verified.

`SourceVerification` reports authenticity evidence for an event or event batch.
It records the validation status, evidence kind, evidence reference, checked
time, and adapter reference. A source event without this evidence is
`unchecked`, not `verified`.

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

`SourceFailureRecord` is the durable source failure fact. It may store reason
code, redacted summary, hash, payload size, source run reference, source attempt
reference, and optional debug or resource reference. It must not store raw
external payload as durable fact.

Raw external payloads are allowed only in restricted debug trace or a future
resource/object owner with TTL, quota, redaction, and access boundary. A
malformed Feishu payload, webhook body, email, or poll response must therefore
produce a bounded failure record, not a raw durable source fact.

Credential and profile handling is readiness only in this requirement:

```text
credential/profile ok -> source may start
credential/profile missing/expired/revoked -> source blocked
refresh required -> source degraded or blocked with operator action
```

This requirement does not define a full credential broker.

## Non-Goals

- No Feishu, WeChat, email, or webhook source owner inside kernel.
- No supervisor-owned session mapping, dedupe, `turn.submit`, provider context,
  skill selection, application reply strategy, or outbox policy.
- No inference that `lark-cli` process readiness means event authenticity.
- No raw source payload in durable source failure facts.
- No full credential store, refresh broker, or credential authority in this
  requirement.
- No production commitment to a hardcoded external CLI command shape.
- No model-visible source cursor, profile token, external credential, or raw
  source transport value.

## Phased Delivery

### Phase A: Requirement And Design Boundary

Document the source supervisor contract, readiness versus verification
distinction, run/attempt/cursor/failure semantics, and the owner split between
Source Supervisor, Application Connector Runtime, and Kernel.

### Phase B: Connector-Local Source State

Implement `SourceRun`, `SourceAttempt`, `SourceCursor`, and source readiness
projection in connector-local storage. A smoke Feishu source can remain
`unchecked` until real verification evidence exists.

### Phase C: Source Adapter Boundary

Move hardcoded source command shape behind connector driver configuration or a
`connector_command` external adapter process. Runtime code owns typed source
records, not Feishu CLI argv or raw SDK payloads.

### Phase D: Event Verification

Add event or event-batch verification evidence for sources that support it:
webhook signatures, platform tokens, trusted adapter assertions, or equivalent
connector-approved evidence. Only then can source validation become `verified`.

### Phase E: Operator Lifecycle Controls

Add operator start, stop, restart, blocked-state inspection, backoff inspection,
and recovery commands for source runs. These commands mutate connector source
state only and cannot fabricate kernel facts.

## Acceptance Criteria

- A ready source adapter does not produce `source_validation=verified` unless
  per-event or per-batch verification evidence exists.
- A verified source event carries validation status, evidence kind, and an
  evidence reference that can be inspected without exposing secrets.
- A malformed or rejected source payload writes a redacted `SourceFailureRecord`
  with reason code, hash, size, source run reference, and optional debug/resource
  reference, but no raw payload in durable facts.
- Source cursor progress can resume intake without duplicating external events,
  while remaining connector-local and model-invisible.
- Missing, expired, or revoked profile/credential state blocks or degrades the
  source before kernel submission.
- The supervisor emits `ExternalEvent` values but does not create request
  context, map sessions, submit turns, write outbox items, or build provider
  context.
- External source identity and source validation status do not grant kernel
  permission, sandbox, credential, memory, or tool authority.
