# Requirement: User-space Message Ingress Runtime

- **Status:** approved
- **Owner:** user-space message ingress runtime
- **Scope:** external message receipt, inbound envelope, idempotent kernel turn submission

## Background

Genesis needs external messages from Feishu, WeChat, email, webhooks, console,
and future interfaces to enter the kernel without turning those channels into
kernel capabilities. The ingress side has channel-specific signatures, tokens,
retry behavior, dedupe ids, and thread/user identifiers. Those are application
or adapter concerns.

Outbound channel actions are different. Sending a Feishu message, sending an
email, or calling a WeChat command is a domain action. The LLM should perform
those actions by reading skills and using kernel-governed generic tools such as
`shell_exec` to call external CLIs. The ingress runtime must not decide whether
to reply, what to say, which format to use, or which outbound channel action to
call.

## Production Target

The User-space Message Ingress Runtime is an external-message relay. It accepts
channel events, validates the source at the adapter boundary, normalizes the
message into an inbound `ChannelMessage` envelope, maps the external
conversation to an opaque kernel session, deduplicates inbound messages before
any kernel side effect, and submits a turn to Genesis Kernel.

The runtime injects enough inbound context for the LLM to understand where the
message came from and how a skill/CLI could reply, such as channel, adapter,
chat/thread id, message id, sender display, and message text. That context is
ordinary turn input. It is not kernel authority, provider-context ownership, or
permission policy.

## Users And Roles

Ordinary users send messages through an external channel.

Operators can inspect adapter-local inbox status, duplicate handling, and the
mapped kernel session id.

Adapters own source validation, token/profile configuration, inbound event
parsing, retry, and local inbox state.

The ingress runtime owns normalized inbound envelopes, session mapping, inbound
idempotency, and kernel turn submission.

Genesis Kernel owns turn lifecycle, provider context, model execution, tool
runtime, memory, work/jobs, events, audit, and projections.

The LLM decides whether and how to answer. If it needs to reply through Feishu,
email, WeChat, or another external system, it uses skills plus generic kernel
tools to invoke the relevant external CLI.

## Core Semantics

`ChannelMessage` is the application-owned inbound envelope. It contains at
least `channel`, `adapter`, `message_id`, `thread_id`, `user_id`, `text`,
`received_at`, and optional metadata such as `chat_id` or `sender_display`.
Missing required identity or text fields are rejected before the kernel is
called.

Session mapping is deterministic and opaque. The runtime maps
`channel + adapter + thread_id` to a kernel `session_id` without making the
channel user or thread a kernel authority. The mapping must be stable across
process restarts.

Inbound idempotency is enforced before `turn.submit`. A duplicate
`channel + adapter + message_id` must not submit another kernel turn.

`turn.submit` is the only Phase A kernel write path. The runtime may pass the
kernel `session_id`, a stable idempotency key, and one or more user-visible
input items containing inbound context and message text. It must not write
kernel facts directly.

Console and local diagnostic shells may render the kernel final answer because
they are local user interfaces. Feishu, email, WeChat, and other external
channel sends are not ingress-runtime delivery. They are outbound domain actions
for LLM + skill + CLI + `shell_exec`.

Adapter retry, signature validation, token/profile handling, and rate-limit
backoff are adapter-local. They can block, retry, or fail inbound submission,
but they cannot expand kernel authority.

## Non-Goals

- No Feishu package inside the kernel.
- No bidirectional Channel Gateway.
- No automatic external-channel reply delivery from the ingress runtime.
- No gateway API for Feishu cards, document creation, email attachments, or rich channel surfaces.
- No channel gateway provider-context builder.
- No application writes to kernel ledger, memory, tool result, checkpoint, or audit truth.
- No automatic promotion from channel user/thread identity to kernel permission authority.
- No desktop, WebUI, or mobile UI feature set in this requirement.
- No long-running channel listener production hardening in Phase A.

## Phased Delivery

Phase A proves the local ingress contract with a console inbound adapter, a
Feishu inbound envelope shape, a kernel HTTP client, deterministic session
mapping, durable app-local dedupe, and tests using a fake kernel.

Phase B adds a real Feishu inbound listener or poller and adapter-local
signature/token/retry handling sufficient for mobile smoke testing.

Phase C adds an operator console that reads kernel projections for timeline,
jobs, memory review, capabilities, raw events, audit, and provider context
inspection without interpreting raw events as its own truth.

Phase D uses real skills and external CLIs to identify missing generic kernel
primitives. Application-specific outbound features remain outside the kernel
unless reduced to approved primitives.

## Acceptance Criteria

- A Feishu inbound message can be normalized into `ChannelMessage`, mapped to a
  kernel session, and submitted through `turn.submit`.
- The same inbound message delivered twice does not execute two kernel turns.
- Console and Feishu inbound adapters share the same ingress path.
- The turn input includes enough inbound context for the LLM to know the source
  channel and reply reference, without setting permission, sandbox, credential,
  approval, or provider-context authority.
- The runtime never imports kernel internals or writes kernel ledger, memory,
  tool result, checkpoint, or audit files.
- Channel user and thread identifiers are inspectable as adapter facts but do
  not become kernel permission authority.
- Tests cover valid inbound processing, duplicate handling, invalid envelope
  rejection, inbound context construction, and the absence of external-channel
  auto-reply logic.

## Relationship To Existing Issues

This requirement governs application issues in
`docs/operations/application-issues.md`. It does not create or retire current
kernel issues unless implementation discovers a missing generic kernel primitive.
