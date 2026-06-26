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

`ExternalResourceRef`:

- `connector`
- `kind`
- `external_id`

These refs name connector-local external resources only. They are not kernel
`resource_ref` values and cannot be passed to kernel resource tools or context
hydration without a separate resource/context intake owner.

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

`SourceFailureRecord`:

- `record_id`
- `connector`
- `event_source`
- `reason`
- `detail`
- `diagnostic_excerpt`
- `source_validation`
- `created_at`

The diagnostic excerpt is a bounded, redacted operator hint. It is not a raw
external payload, not a webhook archive, not a credential trace, and not a
debug bundle.

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
6. Connector updates outbox state: sent, retrying, dead-lettered, or
   recovery-required. Duplicate-suppressed and failed outcomes remain delivery
   receipt classifications, not durable outbox states.

External delivery failure is connector failure, not kernel turn failure. Kernel
facts remain unchanged.

For the first mobile smoke loop, application policy may treat the kernel
`final_text` as an ordinary reply candidate for the same inbound request
context. That policy creates a `send_message` command targeted at the
connector-owned thread ref and deduped by request id plus kernel turn id. This
is intentionally narrower than a rich external action API: it proves the
inbound -> kernel -> outbox -> delivery loop without giving the kernel Feishu
semantics and without letting the LLM or provider context own external delivery.
Duplicate inbound events reuse the existing request record and must not enqueue
or send another reply.

## Credential And Authority

External credentials are connector-owned. They are resolved by connector
configuration, credential references, or adapter-local auth. They are not
projected into prompt context and are not model-visible tool arguments.
CLI-backed adapters must use an explicit connector-configured profile or
credential reference. They must not rely on whatever default identity the host
CLI happens to select.

CLI-backed adapters also own command-shape drift risk. The stable application
semantic layer is `AppCommand` and `ConnectorAction`; external CLI argv belongs
to an adapter driver. The short-term driver is `command_template`: an executable,
explicit profile, action-scoped argv token templates, and external action ref
JSON paths. Templates are argv arrays, not shell strings, and the executable
must be a direct external adapter/CLI executable, not `cmd`, PowerShell, `sh`,
`bash`, npm-generated `.cmd`/`.ps1` shims, extensionless shell scripts, or
another shell wrapper. Until a connector-owned credential-ref binding exists,
every command template action must bind `${profile}` in argv; omitting it is
invalid because it would let host CLI defaults choose the external identity.
The runtime fills only validated connector action fields such as `${profile}`,
`${target.external_id}`, `${payload.body}`, and `${idempotency_key}`. Unknown
variables, credential-shaped variables, missing profiles, templates without
profile binding, unexpected payload keys, shell-string templates, and shell
executables or resolved script wrappers fail closed. Command-template execution
uses a connector command environment allowlist and persists only safe opaque
external action refs, not raw CLI output.

Operator probes may discover a direct binary installed by an external package.
For example, the official `@larksuite/cli` npm package installs a platform
binary under its package `bin` directory and exposes `lark-cli` through npm
shims. `command_template` configuration should point at the direct binary, such
as `.../node_modules/@larksuite/cli/bin/lark-cli.exe` on Windows, not at the
PATH shim. If only a shim is available, the connector must use a
`connector_command` adapter process or another direct binary provider.

The long-lived adapter boundary is `connector_command`. The connector runtime
starts a configured external adapter process, writes typed `ConnectorAction`
JSON to stdin, and reads typed `ConnectorActionResult` JSON from stdout. The
external adapter owns SDK, HTTP, CLI, vendor response parsing, and vendor error
normalization. The connector runtime owns the configured binary, timeout,
environment allowlist, action/result validation, outbox state transitions, and
`DeliveryReceipt` persistence. Stdout is the typed result channel. Stderr,
raw stdout, command lines, SDK payloads, and vendor HTTP responses are
diagnostic material only and must not become receipt truth. Malformed JSON,
unsupported delivery statuses, unsafe external action refs, unsafe reason
strings, missing direct executables, timeouts, and failed adapter processes
fail closed as connector-local delivery failures.

`command_template` is only a transitional driver for early CLI-backed smoke
tests. It may stay as a local operator convenience, but production connectors
should not treat rendered argv as a stable Genesis protocol. If the installed
CLI or external adapter no longer accepts the configured shape, the connector
reports connector-local unavailability or a delivery failure; it must not
silently fall back to another command form or ask the kernel/LLM to guess new
arguments.

Raw command lines, raw stdout, raw stderr, SDK payloads, and vendor HTTP
responses are debug material. They may be retained only through bounded,
redacted, connector-local diagnostic traces. Durable connector truth remains
`ConnectorAction`, `ConnectorActionResult`, and `DeliveryReceipt`. An external
adapter process cannot write kernel events, tool results, checkpoints, jobs,
memory records, or delivery receipts directly; it can only return a result for
the connector runtime to validate and persist.

Raw inbound webhook payloads and malformed source lines follow the same rule.
The durable `SourceFailureRecord` stores a safe diagnostic summary and optional
debug reference only after a separate debug trace owner exists. Until then,
source failures must avoid retaining the raw line, message body, headers,
tokens, credentials, attachment content, or full vendor event object.

External identities are origin facts. They can participate in mapping and
policy, but they do not automatically grant kernel authority.

If an external user or channel needs authority mapping, that must become a
separate kernel/app authority design with explicit credential and permission
semantics.

## Failure Semantics

Invalid external event: connector rejects or dead-letters before application or
kernel side effects.

Malformed source payload: connector records a durable `SourceFailureRecord`
with reason, validation status, and safe diagnostic summary. Raw external
payloads remain outside durable connector truth unless an explicit debug trace
surface with TTL, quota, redaction, and operator-only access is approved.

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

Ambiguous external outcome: connector records recovery-required state. Generic
operator resolution is allowed only as an explicit manual override. Production
automatic resolution must first run a connector-specific reconciliation probe
that queries the external system without resending the action and returns
connector-local evidence for the terminal decision.

Listener runtime failure: connector may retry a bounded smoke source locally,
recording connector-local source runtime failures. Production source lifecycle
controls, credential/profile refresh, source verification, and driver migration
require the connector source verification/lifecycle boundary; they must not
accrete inside the Feishu smoke source function.

## Reconciliation Probe Design

Reconciliation probes are connector-local read-only recovery helpers for
`recovery_required` outbox items. They are not kernel tools, not model-visible
capabilities, and not external action retries.

Input preconditions:

- the outbox item exists and has status `recovery_required`;
- the item has no active delivery lease;
- the connector adapter for this item supports a reconciliation probe;
- the item or its receipts contain an exact external lookup handle:
  `external_action_ref`, provider receipt ref, or connector idempotency key
  that the external provider can query deterministically.

If the item has only target ref and message body, the probe must fail closed as
unavailable. It must not scan chat history by text, compare recent messages
fuzzily, resend the action, or ask the LLM to infer the outcome.

Probe execution:

```text
Operator / scheduled recovery worker
        |
        v
Connector reconciliation probe
        |
        v
External system read-only status query
        |
        v
ReconciliationEvidence
        |
        v
Operator or approved connector policy resolves recovery_required
```

The probe result is evidence, not the terminal outbox transition itself. The
current implementation path remains an explicit operator recovery command; a
future automatic resolver must cite probe evidence and preserve all prior
receipts.

`ReconciliationEvidence`:

```text
probe_id
outbox_id
connector
action_kind
query_kind: external_action_ref / external_receipt_ref / idempotency_key
query_ref_hash or safe query_ref
observed_status: sent / not_found / failed / unknown / unavailable
evidence_ref
reason
checked_at
adapter_ref
```

Allowed terminal support:

- `sent`: the external system confirms the exact action exists or was accepted;
- `dead_lettered`: the external system confirms the exact action was rejected,
  expired, impossible, or definitively absent under a reliable idempotency
  lookup;
- `recovery_required`: the probe was unavailable, inconclusive, timed out, or
  lacked exact lookup handles.

Reconciliation evidence is connector state. It must not mutate kernel events,
memory, jobs, provider context, audit, or tool results. It must not overwrite
the original `DeliveryReceipt` that created `recovery_required`; it can only
append evidence and support a later connector-owned terminal receipt.

Raw external status payloads, HTTP bodies, CLI output, headers, tokens,
attachment content, and message bodies remain debug trace material only. Durable
evidence stores safe refs, hashes, bounded reasons, and observed status.

## Observability

Application connector state is separate from kernel event truth:

- inbound events;
- request contexts;
- session mappings;
- source failure records;
- outbox items;
- connector action attempts;
- delivery receipts;
- dead-letter records;
- adapter health.

Kernel projections remain the source of truth for turn, tool, job, memory,
provider context, audit, and recovery.

Connector file-backed smoke state is durable enough to be shared by listener,
console, and worker commands. Load-modify-write operations must therefore be
serialized across processes with a connector-local file lock and atomic replace.
Future production stores may replace the JSON file implementation, but they
must preserve the same single-owner mutation semantics.

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

For reconciliation specifically, neither Codex nor Reasonix provides a direct
external-channel outbox probe to copy. The useful reference is their evidence
posture: host- or adapter-observed receipts are separate from the model's
claims, and retry/recovery paths do not silently rewrite earlier facts.

## Rejected Alternatives

Rejected: Feishu Bridge as first-class architecture. Feishu is only one adapter.

Rejected: Channel Gateway with broad reply API. It would turn the connector
runtime into a second kernel/application owner.

Rejected: production default where the LLM shells out directly to external
CLIs/APIs for outbound delivery. It cannot reliably own credentials, revoke
auth, rate limits, idempotency, delivery receipts, or half-success recovery.

Rejected: hardcoding Feishu, mail, or WeChat CLI argv in connector runtime code
as the long-term adapter contract. CLI syntax is external tool protocol and must
live in driver configuration or an external adapter process.

Rejected: treating `command_template` as the final production connector
protocol. It reduces the immediate Feishu CLI drift risk, but it still leaves
external command semantics in connector configuration. The long-lived boundary is
`connector_command`, where the external adapter process owns vendor protocol
drift behind typed action/result JSON.

Rejected: connector-owned provider context. Provider context is kernel-owned.

Rejected: connector writes to kernel ledger, memory, tool result, checkpoint, or
audit. Connector state is application-local.
