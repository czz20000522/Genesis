# Design: User-space Message Ingress Runtime

- **Requirement:** `docs/applications/user-space-message-ingress-runtime-requirement.md`
- **Owner:** user-space message ingress runtime
- **Status:** approved

## Boundary Model

The runtime is a user-space ingress application owner. It sits between channel
adapters and the kernel HTTP surface.

```text
Feishu / WeChat / Email / Webhook / Console input
        |
        v
User-space Message Ingress Runtime
        |
        v
Kernel turn.submit
        |
        v
Genesis Kernel -> Model Gateway -> LLM

LLM
        |
        v
skill instructions + shell_exec
        |
        v
lark-cli / mail-cli / wechat-cli / other external CLI
        |
        v
External channel action
```

The ingress runtime owns inbound envelopes, session mapping, inbound dedupe, and
kernel HTTP client calls. The kernel owns model context, tool execution, memory
truth, work/job state, event ledger, audit, and projections. Outbound channel
actions are initiated by the LLM through skills and generic kernel tools.

## Components

`ChannelMessage` is the normalized inbound envelope. It is owned by the ingress
runtime and is not a kernel schema.

`SessionMapper` maps `channel + adapter + thread_id` to an opaque kernel
`session_id`. The generated session is stable and does not encode permission
authority.

`InboundStore` records dedupe keys and submission records before calling the
kernel. The store is application-local and restart-safe.

`KernelClient` talks to the kernel over HTTP. It posts `/turn` and may read
session or inspection projections for local diagnostics. It does not import
`internal/kernel`.

`InboundFormatter` converts `ChannelMessage` into ordinary user-visible turn
input that includes source channel, adapter, reply reference, sender facts, and
message text. It does not assemble provider context.

`ConsoleInboundAdapter` reads or accepts a `ChannelMessage` and submits it
through the ingress runtime. A console command may print the kernel final answer
as local UI output.

`FeishuInboundAdapter` maps Feishu inbound event data to `ChannelMessage`.
Feishu outbound send is not part of this runtime.

## Data Flow

1. Adapter receives an external inbound message.
2. Adapter validates source signature/token/profile if applicable.
3. Adapter builds `ChannelMessage`.
4. Runtime validates the envelope.
5. Runtime reserves the dedupe key.
6. Runtime maps the channel thread to a kernel session.
7. Runtime formats inbound context as turn input.
8. Runtime submits `/turn` with the kernel session and stable idempotency key.
9. Runtime records submission status in application-local state.

If step 5 detects a duplicate, steps 6 through 9 are skipped and the previous
submission record is returned.

## Protocol

Phase A turn submission uses the current kernel HTTP transport:

```json
{
  "session_id": "chan_<opaque>",
  "idempotency_key": "<channel>:<adapter>:<message_id>",
  "input_items": [
    {
      "type": "text",
      "text": "Inbound message\nsource_channel: feishu\nadapter: feishu-inbound\nchat_id: oc_xxx\nmessage_id: om_xxx\nsender_id: ou_xxx\n\ntext:\nhello"
    }
  ]
}
```

The inbound context is model-visible user input. It is not control-plane state
and must not include permission mode, sandbox profile, approval policy, provider
configuration, credentials, or kernel-owned ids except the ordinary session id
already bound by the runtime request.

## Failure Semantics

Invalid envelope: no kernel call, application error record.

Duplicate message: no kernel call, return existing application record.

Kernel unavailable or turn rejected: record application submission failure; do
not invent kernel facts.

Feishu, email, or WeChat outbound failure: not handled by this runtime. Such
failures occur later as tool results when the LLM invokes external CLIs through
kernel-governed tools.

Adapter token, profile, signature, or inbound retry failure: adapter-local
failure, no kernel authority expansion.

## Permission And Authority

Channel identities are origin facts only. They can help route inbound messages,
choose deterministic kernel sessions, and provide reply references to the LLM,
but they do not select `permission_mode`, `sandbox_profile`, approval policy,
credentials, or memory authority.

Any future authority mapping must be a kernel-approved command or credential
resolution flow, not a property of Feishu, console, or another adapter.

## Observability

The runtime stores sparse application facts:

- inbound dedupe key;
- mapped kernel session id;
- kernel turn id and status;
- adapter inbound status or error;
- timestamps.

Kernel projections remain the source of truth for turn, tool, job, memory,
audit, and provider-context inspection.

## Reference Scan

Codex app-server exposes typed in-process client requests and a server event
stream. Its callers submit typed requests and consume events; they do not become
the core turn owner. `codex-rs/app-server/src/in_process.rs` also warns callers
to keep request ids unique and to answer only current runtime server requests.

Codex app-server event handling projects core events into app-server
notifications while aborting per-turn pending server requests at turn
boundaries. `codex-rs/app-server/src/bespoke_event_handling.rs` keeps the
projection layer separate from the core conversation.

Reasonix ACP is the closest ingress reference. `internal/acp/service.go` owns
the JSON-RPC session protocol, flattens prompts, calls
`control.Controller.RunTurn`, and persists an ACP transcript after the turn. Its
comments explicitly place controller construction in a factory and route events
through a session sink.

Reasonix `internal/serve/wire.go` converts internal events to browser-friendly
wire events without making the server the source of tool or provider truth.

Genesis aligns with those references by keeping channel adapters outside the
kernel and using typed runtime-owned envelopes plus kernel projections. Genesis
intentionally differs by making external-channel outbound actions a skill/CLI
tool path, not an app-server delivery path.

## Rejected Alternatives

Rejected: put Feishu listener and sender into `internal/kernel`. This would make
one channel a kernel capability and violate the user-space application boundary.

Rejected: create a bidirectional gateway `reply(message, target)` API. It would
become a second owner for outbound communication formats and would grow into a
parallel application framework.

Rejected: let the runtime build provider context with channel metadata. Provider
context is a kernel projection owned by Model Gateway and Interface Kernel.

Rejected: automatically send `final.text` back to Feishu. This would make the
ingress relay decide that a reply is required and would bypass the skill/CLI
outbound path that Genesis uses to let the LLM touch external systems.
