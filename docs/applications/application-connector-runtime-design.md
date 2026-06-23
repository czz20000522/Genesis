# Design: Application Connector Runtime

- **Requirement:** `docs/applications/application-connector-runtime-requirement.md`
- **Owner:** user-space application connector runtime
- **Status:** approved

## Boundary Model

Application Connector Runtime is a user-space protocol boundary owner.

```text
External App / Server
        |
        v
Inbound Connector
        |
        v
ExternalEvent / Resource / RequestContext
        |
        v
Application policy
        |
        v
Kernel HTTP primitives
        |
        v
Genesis Kernel
        |
        v
Kernel typed result / app command
        |
        v
Application policy
        |
        v
ConnectorOutbox
        |
        v
ConnectorAction
        |
        v
External App / Server
        |
        v
DeliveryReceipt
```

The connector runtime is not kernel and not Model Gateway. It is similar in
shape to Model Gateway because it adapts an external protocol boundary, but it
adapts application events/actions rather than model requests/responses.

This design uses the protocol boundary owner pattern from
`docs/kernel-contract.md`: external protocol values become connector-owned refs,
requests, actions, and receipts before any Genesis owner consumes them.

## Owner Responsibilities

Connector Runtime owns:

- inbound external event parsing and source validation;
- external identity/thread/message/resource normalization;
- request context creation;
- app-local inbound dedupe and outbox idempotency;
- application session mapping to opaque kernel session ids;
- kernel HTTP client calls through public primitives;
- connector outbox, connector action execution, retry, and delivery receipts;
- adapter-local credential/profile/token handling.

Application policy owns:

- whether an inbound event should create, reuse, or ignore a kernel session;
- which kernel primitive to call;
- whether a kernel typed result becomes an outbox action, draft, confirmation
  request, ignored item, or operator-visible state.

Kernel owns:

- authority, permission, sandbox, approval, credentials, provider context,
  model execution, tool execution, memory, jobs/work, event ledger, audit,
  checkpointing, recovery, and kernel projections.

LLM owns:

- semantic intent and natural language;
- typed semantic requests when the application exposes a schema.

LLM does not own:

- external credentials;
- connector outbox idempotency;
- delivery retry;
- external API transport;
- half-success recovery;
- kernel authority.

## Data Shapes

`ExternalEvent`:

- `connector`
- `external_event_id`
- `event_type`
- `thread_ref`
- `sender_ref`
- `message_ref`
- `body`
- `resource_refs`
- `received_at`
- `validation_status`
- `metadata`

`ExternalThreadRef`:

- `connector`
- `kind`
- `external_id`
- `display`
- `metadata`

`RequestContext`:

- `request_id`
- `dedupe_key`
- `connector`
- `thread_ref`
- `sender_ref`
- `message_ref`
- `resource_refs`
- `source_validation`
- `application_session_id`
- `kernel_session_id`

`AppCommand`:

- `command_id`
- `kind`
- `target_ref`
- `body`
- `resource_refs`
- `requires_confirmation`
- `dedupe_key`

`ConnectorOutboxItem`:

- `outbox_id`
- `command_id`
- `connector`
- `action_kind`
- `target_ref`
- `payload`
- `status`
- `attempt_count`
- `next_attempt_at`
- `idempotency_key`

`DeliveryReceipt`:

- `receipt_id`
- `outbox_id`
- `connector`
- `external_action_ref`
- `status`
- `reason`
- `attempt`
- `recorded_at`

The exact Go structs can evolve, but these concepts are the stable owner
contract.

## Inbound Flow

1. Inbound connector receives an external event.
2. Connector validates signature/token/source according to adapter policy.
3. Connector normalizes external ids into refs and creates `ExternalEvent`.
4. Application creates `RequestContext`.
5. Application checks inbound dedupe before kernel side effects.
6. Application maps the request to an opaque kernel session.
7. Application calls kernel `/turn` or another public primitive.
8. Application records connector-side submission state.

Connector never assembles provider context and never writes kernel facts.

## Outbound Flow

1. Kernel completes a turn or emits a typed projection/application command source.
2. Application policy decides whether an outbound action should exist.
3. Application writes `ConnectorOutboxItem` with a connector idempotency key.
4. Connector adapter claims an outbox item and executes a `ConnectorAction`.
5. Adapter translates external API/CLI/SDK result into `DeliveryReceipt`.
6. Connector updates outbox state: sent, retrying, failed, duplicate
   suppressed, partial, or dead-lettered.

External delivery failure is connector failure, not kernel turn failure. Kernel
facts remain unchanged.

## Credential And Authority

External credentials are connector-owned. They are resolved by connector
configuration, credential references, or adapter-local auth. They are not
projected into prompt context and are not model-visible tool arguments.

External identities are origin facts. They can participate in mapping and
policy, but they do not automatically grant kernel authority.

If an external user or channel needs authority mapping, that must become a
separate kernel/app authority design with explicit credential and permission
semantics.

## Failure Semantics

Invalid external event: connector rejects or dead-letters before application or
kernel side effects.

Duplicate inbound event: connector/application returns the existing request
record and does not resubmit the kernel turn.

Kernel unavailable or turn rejected: application records submission failure;
connector does not forge kernel facts.

Outbox duplicate: connector suppresses duplicate action and records a receipt.

External delivery failure: connector records retry/failure/dead-letter receipt;
kernel turn facts are not rewritten.

Partial external success: connector records partial receipt and a recovery
state. Application policy may decide a follow-up command, but the connector does
not silently pretend success.

## Observability

Application connector state is separate from kernel event truth:

- inbound events;
- request contexts;
- session mappings;
- outbox items;
- connector action attempts;
- delivery receipts;
- dead-letter records;
- adapter health.

Kernel projections remain the source of truth for turn, tool, job, memory,
provider context, audit, and recovery.

## Reference Scan

Codex app-server demonstrates typed request/event boundaries: app-server clients
submit typed requests and consume notifications while core owns turn state.
Useful references:

- `codex-rs/app-server/src/in_process.rs`
- `codex-rs/app-server/src/bespoke_event_handling.rs`

Reasonix ACP and serve layers show protocol adaptation around a controller:
ACP flattens external prompt blocks and calls `control.Controller.RunTurn`,
while wire projection converts internal events for clients without becoming
event truth.

- `reasonix/internal/acp/service.go`
- `reasonix/internal/serve/wire.go`

Genesis should align with the boundary-adapter shape but differ by adding a
connector outbox/receipt owner for production outbound external actions.

## Rejected Alternatives

Rejected: Feishu Bridge as first-class architecture. Feishu is only one adapter.

Rejected: Channel Gateway with broad reply API. It would turn the connector
runtime into a second kernel/application owner.

Rejected: production default where the LLM shells out directly to external
CLIs/APIs for outbound delivery. It cannot reliably own credentials, revoke
auth, rate limits, idempotency, delivery receipts, or half-success recovery.

Rejected: connector-owned provider context. Provider context is kernel-owned.

Rejected: connector writes to kernel ledger, memory, tool result, checkpoint, or
audit. Connector state is application-local.
