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

`DeliveryReceipt` records the outcome of a connector action: accepted, sent,
failed, retrying, duplicate suppressed, partially completed, or dead-lettered.
External errors are translated into connector receipt reasons instead of being
treated as kernel errors.

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

## Phased Delivery

Phase A keeps the current inbound slice: `ExternalEvent`/`ChannelMessage`
normalization, deterministic session mapping, app-local dedupe, and
`turn.submit` through the kernel HTTP surface.

Phase B adds the minimal connector outbox contract: `AppCommand`,
`ConnectorOutbox`, `ConnectorAction`, and `DeliveryReceipt`, with console and
Feishu adapters using fake or local runners in tests. No rich cards,
attachments, or production listener hardening.

Phase C adds a Feishu inbound connector listener/poller and connector-local
validation/retry/token handling sufficient for mobile smoke testing.

Phase D adds operator console inspection for connector state plus kernel
projections without letting the console reinterpret raw kernel events as its own
truth.

Phase E adds resource intake and richer connector action types only when they
are backed by connector-owned idempotency, receipt, and recovery semantics.

## Acceptance Criteria

- External messages can be validated or source-checked by a connector,
  normalized into an application event/request context, mapped to an opaque
  kernel session, and submitted through `turn.submit`.
- Duplicate inbound external events do not execute duplicate kernel turns.
- Kernel typed result or application command can enqueue a connector outbox
  item without the connector writing kernel facts.
- Connector action execution records delivery receipt, retry state, and failure
  reason in connector state only.
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
message-ingress production framing. The current `message_ingress` package is
only the Phase A inbound slice under this broader requirement.
