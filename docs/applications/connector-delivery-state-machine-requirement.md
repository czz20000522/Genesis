# Requirement: Connector Delivery State Machine

- **Status:** approved
- **Owner:** user-space application connector runtime
- **Scope:** connector outbox delivery state, retry scheduling, delivery leases, dead-letter, partial-success recovery, and delivery receipts

## Background

External application delivery is a real side effect outside Genesis canonical
world. A Feishu message, email, webhook callback, or future desktop
notification can be accepted, rejected, rate-limited, partially completed,
duplicated, retried, or later reported as delivered. The kernel cannot own those
external protocol details, and the LLM cannot safely own them by composing
ad-hoc CLI/API calls.

The connector runtime therefore needs a production-grade delivery state machine
that owns outbound idempotency, attempt claims, retry scheduling, dead-letter
classification, delivery receipts, and partial-success recovery. This state
machine is user-space application state. It must not rewrite kernel turn facts,
provider context, memory truth, tool results, checkpoints, or audit facts.

## Production Target

Connector delivery must behave like a durable, idempotent outbox:

- each semantic `AppCommand` maps to at most one active `ConnectorOutboxItem`
  for a given connector idempotency key;
- each external delivery attempt is claimed through a bounded lease before any
  external side effect is executed;
- adapter outcomes are recorded as `DeliveryReceipt` records before the item is
  moved to the next delivery state;
- retryable failures become scheduled retries with bounded backoff and a next
  attempt time;
- non-retryable failures or exhausted retries become dead-lettered connector
  state;
- partial external success becomes explicit recovery-required state rather than
  silent success or repeated blind retry;
- terminal items are not delivered again unless a future operator-owned recovery
  command explicitly reopens or supersedes them;
- connector delivery failures never become kernel turn failures.

The state machine must remain connector-local. Kernel projections may be read as
input to application policy, but connector delivery state is not kernel event
truth.

## Users And Roles

Ordinary users see whether a reply or external action is pending, sent, failed,
or needs operator recovery. They should not need to understand connector leases,
attempt ids, or raw external API errors.

Operators inspect outbox state, attempt history, retry schedule,
dead-lettered items, partial-success records, and delivery receipts. Operators
may later get recovery commands, but those commands must be explicit
connector-runtime operations rather than hidden automatic mutation.

Connector adapters execute one claimed `ConnectorAction` at a time. They
translate SDK, HTTP, CLI, or IPC outcomes into connector-owned result categories
and bounded diagnostic fields. They must not write kernel facts or expose
credentials to the LLM.

Application policy decides whether a kernel projection or typed application
command should create an outbox item, be ignored, become a draft, or require
confirmation. Application policy may also decide whether a partial success needs
manual recovery.

Genesis Kernel owns turn execution, authority, tools, memory, jobs, checkpoint,
audit, and kernel projections. It does not own connector delivery retry,
receipt, lease, or external protocol state.

The LLM owns semantic intent only. It can request an application action through
approved application schema, but it does not choose retry policy, lease state,
dead-letter state, external credential, or raw delivery transport.

## Core Semantics

`ConnectorOutboxItem` is the durable queue item derived from an `AppCommand`.
It records connector, action kind, target ref, sanitized payload, idempotency
key, current delivery state, attempt count, next eligible attempt time, and
last receipt reference.

`ConnectorAction` is the adapter-executable action materialized from a claimed
outbox item. It is not a kernel tool call and not model-owned shell authority.

`DeliveryAttempt` is a single claimed execution try. A delivery attempt has an
attempt number, started time, lease deadline, owner identity, and terminal
outcome. The owner identity is connector-local worker identity, not kernel
authority.

`DeliveryLease` prevents concurrent workers from executing the same item at the
same time. A lease must expire if the worker crashes. Expired leases make the
item eligible for retry or recovery according to policy. Lease ids and worker
ids are connector control-plane fields and are not model-visible.

`DeliveryReceipt` is the durable record of an attempt outcome. It records the
outbox id, attempt number, connector, status, bounded reason code, optional
external action ref, optional next attempt time, and recorded time. Raw external
responses, credentials, access tokens, and unbounded stdout/stderr do not belong
in receipts.

`RetryPolicy` classifies outcomes:

- retryable transient failures: network drop, timeout, rate limit, temporary
  server failure, connector worker crash before terminal receipt;
- non-retryable failures: invalid target, revoked credential, permission denied,
  malformed action, unsupported action kind, permanent external rejection;
- ambiguous failures: external result unknown after a side effect may have
  happened. These require idempotency lookup or recovery rather than blind
  repeated send;
- partial success: some external side effect happened but follow-up work remains,
  such as uploaded attachment without message send, message sent without local
  receipt persistence, or external API accepted but delivery receipt is delayed.

`Backoff` is bounded and connector-owned. It may use exponential backoff,
external retry hints such as Retry-After, jitter, and maximum attempt limits.
Backoff data determines `next_attempt_at`; it does not wake the LLM or change
kernel state.

`DeadLetter` is terminal connector state for exhausted or non-retryable items.
Dead-lettered items remain inspectable with their receipts and reason. They are
not silently deleted or retried.

`DuplicateSuppressed` is a receipt outcome when a command or terminal item would
otherwise be delivered more than once. Suppression is recorded as connector
state so operators can distinguish "not attempted because duplicate" from
"attempted and sent".

## Failure Semantics

Invalid app command: no external side effect is executed. The connector returns
structured validation failure to application policy.

Adapter unavailable: no external side effect is executed. The item remains
queued or retry scheduled according to policy, and a receipt records the
connector-local reason.

Lease conflict: no external side effect is executed by the losing worker. The
losing worker receives a connector-local claim failure.

Retryable adapter failure: a receipt records retry scheduling, attempt count,
reason, and next attempt time. The item becomes eligible only after that time.

Non-retryable adapter failure: a receipt records failure and the item becomes
dead-lettered unless application policy explicitly defines a recovery path.

Partial external success: a receipt records partial success and the item enters
recovery-required state. The connector must not blindly repeat the same action
unless idempotency guarantees prove it is safe.

Unknown external outcome: a receipt records ambiguous outcome. The connector
must prefer idempotency lookup, status query, or operator recovery over blind
duplicate delivery.

Receipt write failure after external success: the connector must not pretend the
send failed and retry blindly. It must preserve enough connector-local evidence
to recover or reconcile before another external send is attempted.

## Visibility And Storage

Long-lived connector storage contains outbox items, attempt records, delivery
receipts, retry schedule, dead-letter records, and bounded diagnostics.

Routine stream chunks, raw CLI stdout/stderr, SDK debug payloads, tokens,
headers, credentials, and full external API bodies are debug trace data at most.
They are not durable connector facts by default.

Operator projections can show summarized delivery state, attempt count, next
retry time, latest receipt, and bounded reason. User-facing projections should
collapse low-level attempts into simple states such as pending, sent, retrying,
failed, or needs recovery.

Kernel projections remain separate. Connector delivery state may reference
kernel sessions or command sources by opaque refs, but it does not mutate kernel
truth.

## Non-Goals

- No Feishu, WeChat, email, calendar, or document owner inside the kernel.
- No connector retry policy in provider context or LLM prompt instructions.
- No model-visible access to external credentials, lease ids, worker ids, raw
  external API payloads, or connector control-plane fields.
- No automatic LLM wakeup when a connector retry succeeds or fails.
- No guarantee that every external protocol supports perfect delivery status;
  ambiguous outcomes must be modeled explicitly.
- No rich message/card/attachment production semantics in this requirement
  except where they affect partial-success recovery.
- No cross-connector global queue scheduler. Each connector runtime owns its
  own delivery policy unless a later application requirement introduces a shared
  user-space scheduler.

## Phased Delivery

Phase A: retry eligibility and terminal duplicate suppression.

- Proves: retrying items are selected only after `next_attempt_at`; terminal
  sent/dead-lettered items do not call adapters; duplicate suppression is
  recorded as a receipt.
- Still short: no real leases, partial-success recovery, or operator recovery.

Phase B: delivery lease and claim semantics.

- Proves: only one worker can claim an eligible item; expired leases become
  recoverable; lease ids remain connector-local and model-invisible.
- Still short: no rich external reconciliation for ambiguous outcomes.

Phase C: retry policy and dead-letter.

- Proves: retryable outcomes schedule bounded backoff; exhausted or
  non-retryable outcomes become dead-lettered; receipts record reason and next
  attempt.
- Still short: no connector-specific status lookup for ambiguous external
  outcomes.

Phase D: partial-success and ambiguous-outcome recovery.

- Proves: partial success is not treated as sent or failed; repeated delivery is
  blocked until idempotency lookup, connector reconciliation, or explicit
  operator recovery decides the next action.
- Still short: channel-specific rich action recovery remains connector-adapter
  work.

Phase E: operator projection and recovery commands.

- Proves: operators can inspect delivery state and issue explicit connector
  recovery commands without kernel ledger mutation or model-owned authority.

## Acceptance Criteria

- A duplicate `AppCommand` with the same connector idempotency key creates or
  returns one outbox item and does not enqueue another action.
- A retrying item is not eligible before `next_attempt_at` and is eligible after
  `next_attempt_at`.
- A terminal sent or dead-lettered item does not call the connector adapter when
  execution is attempted again; a duplicate-suppressed receipt is recorded.
- A retryable failure records a receipt, increments attempt count, schedules the
  next attempt, and preserves kernel turn facts unchanged.
- Exhausted retries or non-retryable failures create one dead-letter transition
  and remain inspectable.
- A lease prevents concurrent delivery workers from executing the same item.
- An expired lease becomes recoverable without exposing process handles,
  worker ids, or lease ids to the LLM.
- A partial success records recovery-required state and does not trigger blind
  duplicate delivery.
- Connector receipts and durable state do not store raw credentials, tokens,
  unbounded external payloads, or raw CLI stdout/stderr.
- Tests cover positive retry flow, premature retry rejection, terminal
  duplicate suppression, exhausted retry dead-letter, lease conflict,
  partial-success recovery-required state, and kernel fact isolation.

## Reference Alignment

Codex app-server keeps transport backpressure and typed request handling at the
boundary: requests use bounded submission, duplicates are rejected, and server
requests are not silently abandoned when queues are full. Genesis should apply
the same idea to connector delivery: failed or saturated delivery workers must
produce connector-local state rather than hanging or rewriting core facts.

Reasonix provider retry separates retryability, capped backoff, Retry-After,
and retry notification from the core agent semantics. Genesis should apply the
same separation to connector delivery: retry classification and backoff are
connector policy, while kernel/provider state remains unchanged.

Genesis intentionally differs from both references by owning an application
outbox/receipt state machine, because external application delivery has side
effects outside the model/provider loop.

## Relationship To Existing Issues

This requirement governs
`APP-CONNECTOR-DELIVERY-STATE-MACHINE-20260623` in
`docs/operations/application-issues.md`.

It extends `docs/applications/application-connector-runtime-requirement.md`
without replacing it. The broader connector runtime requirement owns the full
application boundary. This document owns the delivery state machine production
semantics.
