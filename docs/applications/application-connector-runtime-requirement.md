# Requirement: Application Connector Runtime

- **Status:** approved
- **Owner:** user-space application connector runtime
- **Scope:** external protocol translation, request context, kernel turn submission, app command/outbox/action/receipt delivery

## Background

Genesis needs external applications such as Feishu, WeChat, email, webhooks,
console, future desktop UI, and local automations to connect to the kernel
without becoming kernel owners. Calling this layer a Feishu Bridge or Channel
Gateway is misleading: it makes the first channel look architectural, or turns
the middle layer into a broad external-channel API.

The correct rule is:

> Connector owns protocol translation and delivery; kernel owns authority,
> execution, facts, and recovery; LLM owns only semantic intent.

This is a specialization of the protocol boundary owner pattern defined in
`docs/kernel-contract.md`.

Model Gateway adapts model protocols:

```text
Genesis Model Protocol <-> OpenAI / DeepSeek / Claude / local model
```

Application Connector Runtime adapts external application protocols:

```text
External Event / Action Protocol <-> Genesis Application Event / Request / Projection / Command
```

Both are boundary adapters, but they do not own the same semantics.

## Production Target

Application Connector Runtime is a user-space boundary owner for external
applications. It receives external events, validates the source, normalizes
external identity/thread/message/resource facts, creates a `RequestContext`,
maps the request to an application session and kernel session, calls kernel HTTP
primitives, reads kernel projections or typed application commands, applies
application policy, writes outbound actions to a `ConnectorOutbox`, executes
`ConnectorAction` through a connector adapter, and records `DeliveryReceipt`.

Kernel remains the owner of authority, execution, facts, and recovery:
permission, sandbox, approval, provider context, tool execution, ledger, memory,
job/work state, checkpoints, audit, and kernel projections.

The LLM owns semantic intent only. It can express an action intent such as
`send_message(channel=feishu, thread_ref=..., body=...)`, but it does not own
the credential, retry policy, rate-limit handling, duplicate-send prevention,
attachment upload, delivery status, half-success recovery, or raw external API
call.

## Users And Roles

Ordinary users interact through an external application, such as a Feishu chat
or console input.

Operators inspect application connector state: inbound events, request context,
session mapping, outbox items, connector actions, delivery receipts, retries,
and dead-lettered failures.

Connectors own external protocol handling: signature/token validation, event
parsing, external identity normalization, resource intake, outbound API/CLI/SDK
execution, rate limits, idempotency, retry, and delivery receipts.

Applications own policy and composition: whether to create or reuse a kernel
session, which kernel primitive to call, whether a kernel result becomes a reply
draft, confirmation request, ignored message, or outbox action.

Genesis Kernel owns the canonical runtime: turn lifecycle, provider context,
model execution, tool runtime, memory, work/jobs, events, audit, recovery, and
kernel projections.

The LLM produces semantic text and typed semantic intent. It cannot receive
external credentials, cannot bypass connector outbox, and cannot turn external
identity into kernel permission authority.

## Core Semantics

`ExternalEvent` is the connector-owned representation of inbound external
activity. It records external protocol, event id, event type, external thread
reference, external sender reference, message/resource references, text/body
content, received time, and validation status.

`SourceFailureRecord` is the connector-owned durable fact for an external source
event that cannot become an `ExternalEvent`, `RequestContext`, or kernel turn.
It records connector, source, reason, validation status, and a safe diagnostic
summary. It must not persist raw external webhook payloads, full CLI output,
headers, credentials, tokens, message bodies, attachment bytes, or vendor debug
payloads. If raw source material is needed for troubleshooting, it belongs in a
separate debug trace owner with explicit enablement, TTL, quota, redaction, and
operator-only access.

`ExternalThreadRef` is an application-owned reference to an external thread,
chat, mailbox, webhook source, or console conversation. It is stable enough for
mapping and outbox delivery, but it is not a kernel authority id.

`RequestContext` is the application-owned context passed into application
policy. It contains source channel, connector, external refs, dedupe key,
resource refs, sender facts, and safety/validation facts. It is not provider
context and is not a kernel ledger fact.

`ApplicationSessionMapping` maps request context to a kernel session id. The
kernel session id is opaque and stable. External path/id values are never used
as public system ids directly.

`ApplicationEvent` is the stable internal app event produced from external
protocol input. It can drive `turn.submit`, projection reads, outbox policy, or
operator inspection.

`AppCommand` is a semantic application request, often derived from a kernel
typed result or model semantic intent. Examples: send a message draft, request
confirmation, ignore an event, open a resource, or mark a connector event.

`ConnectorOutbox` is the application-owned durable queue for outbound external
actions. It owns idempotency, retry state, ordering constraints, rate-limit
state, and dead-letter state.

`ConnectorAction` is the adapter-executable external action derived from an
outbox item. It is concrete enough for the connector adapter to send, but it is
not a kernel tool call and not model-owned shell authority.

Connector driver configuration is the adapter-owned translation from
`ConnectorAction` to an external SDK, HTTP request, CLI argv, or local IPC
message. A CLI command shape is not part of the stable application semantic
contract. The stable contract is the connector/action/outbox/receipt state; the
driver may be changed or replaced when the external tool changes.

`connector_command` is the long-lived external adapter process boundary. The
application connector runtime writes one typed `ConnectorAction` request to a
configured adapter process and accepts one typed `ConnectorActionResult`
response. The external adapter owns `lark-cli`, SDK, HTTP, vendor response
parsing, and vendor error normalization. The connector runtime owns adapter
configuration, process timeout, environment allowlist, result validation,
outbox state transitions, and `DeliveryReceipt` persistence.

`command_template` is a transitional CLI-backed driver for early live smoke
tests. It may render configured argv tokens from validated connector action
fields, but it is not a stable Genesis protocol and must not become the only
long-term way to integrate external systems.

`DeliveryReceipt` records the outcome of a connector action: accepted, sent,
failed, retrying, duplicate suppressed, partially completed, or dead-lettered.
External errors are translated into connector receipt reasons instead of being
treated as kernel errors.

`ReconciliationProbe` is a connector-specific, read-only external status query
for an outbox item that is already in `recovery_required`. It exists only to
collect connector-local evidence about an ambiguous or partial outbound action.
It must not resend the action, infer success by fuzzy content search, mutate
kernel facts, or silently mark the outbox item as sent.

A reconciliation probe is allowed only when the connector has an exact lookup
handle for the external action, such as a safe `external_action_ref`, a provider
receipt reference, or a connector idempotency key that the external provider can
query deterministically. If none of those exact handles exist, the probe outcome
is unavailable and the item stays `recovery_required` for operator decision.

`ReconciliationEvidence` is the connector-owned durable fact produced by a
successful probe attempt. It records outbox id, connector, action kind, query
kind, safe query reference or hash, observed external status, checked time,
adapter reference, and a safe diagnostic reason. It stores no raw external API
response, raw CLI output, credential, header, token, attachment content, or
unbounded message body.

Reconciliation evidence may support a later terminal connector decision such as
sent or dead-lettered, but that decision remains connector-owned outbox state.
It never rewrites the original kernel turn, original connector action, or prior
delivery receipts.

Connector-local file stores used for smoke runs are still durable state for the
duration of the run. Any file-backed inbound, source failure, outbox, or receipt
store must serialize load-modify-write cycles across processes and use atomic
replacement. A store that cannot provide cross-process consistency must be
documented as single-process debug-only and must not be used by listener,
console, or worker commands concurrently.

Detailed retry scheduling, lease, dead-letter, and partial-success recovery
semantics are governed by
`docs/applications/connector-delivery-state-machine-requirement.md`.

## Non-Goals

- No Feishu, WeChat, email, calendar, or document owner inside the kernel.
- No direct model access to external API credentials.
- No production path where the LLM freely composes `curl`, SDK, or CLI commands
  to send external messages.
- No connector-built provider context.
- No connector writes to kernel ledger, memory truth, tool result, checkpoint,
  or audit truth.
- No automatic conversion from external identity, role, channel, thread, or
  message ref to kernel permission, sandbox, approval, credential, or memory
  authority.
- No giant connector reply API that tries to model every channel-specific rich
  surface as a kernel API.
- No requirement that every connector use the same transport implementation.
  A connector adapter may use SDK, HTTP, CLI, or local IPC internally.
- No hardcoded long-term coupling between connector runtime code and a specific
  external CLI command line. Short-term CLI-backed adapters use argv-template
  driver configuration; longer-lived connectors may use an external adapter
  process with a typed action/result protocol.
- No CLI-backed adapter may rely on host default identity, inherit arbitrary
  process environment, or persist raw command output as a receipt field.
  Connector drivers must use explicit identity binding, environment allowlists,
  and safe opaque external action refs.
- No connector durable state may store raw external webhook payloads, raw
  external API bodies, raw CLI/stdout/stderr, credentials, headers, tokens,
  attachment bytes, or unbounded message bodies as ordinary facts. These are
  debug trace material only under explicit debug retention policy.
- No `command_template` as the final adapter contract for production connectors.
  Production connectors should move to `connector_command` or another
  typed-process boundary with the same ownership rules.

## Phased Delivery

Phase A provides the connector-owned inbound slice: `ExternalEvent`,
`RequestContext`, deterministic application/kernel session mapping,
app-local dedupe, and `turn.submit` through the kernel HTTP surface.

Phase B adds the minimal connector outbox contract: `AppCommand`,
`ConnectorOutbox`, `ConnectorAction`, and `DeliveryReceipt`, with console and
Feishu delivery exercised through connector driver configuration and fake or
local runners in tests. It also provides the minimal `connector_command`
process runner so long-lived adapters can sit behind typed action/result JSON
instead of hardcoded connector runtime argv. No rich cards, attachments, or
production listener hardening.

Phase C adds a Feishu inbound connector listener/poller, connector-local
validation/retry/token handling, an ordinary final-text reply policy for mobile
smoke testing, and an explicit installed-adapter capability probe. The ordinary
reply policy may turn a kernel final text into one connector-owned
`send_message` outbox item; it must remain application policy, not a kernel
reply API. If Feishu delivery still uses `command_template`, the phase must keep
it documented as a transitional driver and must not treat the rendered CLI argv
as a Genesis contract.

Phase D adds operator console inspection for connector state plus kernel
projections without letting the console reinterpret raw kernel events as its own
truth.

Phase D also defines connector-specific reconciliation probe semantics for
`recovery_required` outbox items. It may add requirement/design and contract
tests before any live external probe exists.

Phase E adds resource intake and richer connector action types only when they
are backed by connector-owned idempotency, receipt, and recovery semantics.

## Acceptance Criteria

- External messages can be validated or source-checked by a connector,
  normalized into an application event/request context, mapped to an opaque
  kernel session, and submitted through `turn.submit`.
- Duplicate inbound external events do not execute duplicate kernel turns.
- Kernel typed result or application command can enqueue a connector outbox
  item without the connector writing kernel facts.
- When ordinary final-text delivery is enabled, one completed kernel turn can
  enqueue and execute at most one connector reply action for the corresponding
  request context.
- Connector action execution records delivery receipt, retry state, and failure
  reason in connector state only.
- Recovery-required outbox items can be inspected without resending the action;
  connector-specific reconciliation probes, when available, require exact
  external lookup handles and produce connector-local evidence before any
  terminal recovery decision.
- Malformed or rejected external source events create connector-local source
  failure diagnostics without persisting raw external payloads in canonical
  connector state.
- File-backed connector smoke stores preserve independent writes across
  concurrently running listener, console, and worker processes.
- Feishu is the first adapter, not a top-level architecture.
- Console and Feishu use the same connector runtime primitives.
- Connector code does not import kernel internals, build provider context, write
  kernel ledgers, or expose external credentials to the LLM.
- Tests cover inbound dedupe, opaque id mapping, outbox idempotency, delivery
  receipt, failed delivery isolation, and the absence of kernel or model-owned
  external credential paths.

## Relationship To Existing Issues

This requirement governs application issues in
`docs/operations/application-issues.md`. It supersedes the narrower
message-ingress production framing. Temporary or narrower slices may only exist
with an explicit retirement target; once the connector-owned primitive exists,
the temporary package, tests, docs, and command references must be removed
rather than treated as an additional truth surface.
