# User-space Applications

This directory records requirements and designs for applications and connectors
that exercise Genesis Kernel without becoming kernel owners.

Application documents follow the same discipline as kernel requirements:
requirements describe production semantics, designs describe ownership and
data flow, implementation plans describe staged delivery, and issues record
current gaps. The boundary is different: application requirements must not
create kernel capabilities unless they reduce the need to an approved generic
kernel primitive.

Application work deliberately pressures the kernel. A connector or application
may keep a temporary smoke path only while it is proving a boundary. Once that
path needs permission authority, long-running task ownership, production-grade
storage, resource lifecycle, credential resolution, or streaming semantics, the
application issue must stop and point back to the kernel or owning foundation
requirement. The application layer then resumes only after the missing primitive
is strengthened. Temporary app-side scaffolding should be retired as those
primitives mature; do not preserve early smoke infrastructure as permanent
architecture.

Every application phase must say which kernel primitive or owner capability it
is pressure-testing. Examples include turn submission, session mapping,
projection reads, connector outbox delivery, resource intake, credential
authority, job control, sandbox policy, memory review, or audit replay. If the
phase cannot name the owner under pressure, it is probably just adding an app
feature and should not proceed until the boundary is clarified.

Application code must not fake missing kernel or owner facts to keep a demo
moving. If the next step requires forged provider context, fabricated kernel
events, invented tool results, app-owned memory truth, connector-owned
permission decisions, or ad hoc credential authority, stop the application slice
and open or update the owning kernel/foundation issue instead.

## Protocol Boundary Rule

Any surface that crosses out of the Genesis canonical world must pass through a
protocol boundary owner. The owner translates external protocols into stable
Genesis application events, resources, request contexts, projections, commands,
actions, and receipts, then translates controlled application actions back into
the external protocol.

The rule:

- external protocols do not directly enter core owners;
- external identity does not directly equal system identity;
- external errors do not directly equal system errors;
- external paths and ids do not directly become public system ids;
- external credentials are not given to the LLM or prompt context;
- external actions pass through request/action/outbox/receipt.

## Connector Runtime Boundary

User-space connector runtimes may:

- receive external events;
- normalize those events into application-owned envelopes;
- map external threads or users to opaque kernel sessions;
- submit turns through the kernel HTTP surface;
- read kernel projections and typed application commands;
- decide application policy outside the kernel;
- enqueue connector actions into an application-owned outbox;
- execute connector actions through channel adapters;
- keep adapter-local retry, token, signature, delivery, and receipt state.

User-space connector runtimes must not:

- assemble provider context;
- write kernel ledgers, memory truth, tool results, checkpoints, or audit facts;
- make external channel identity a kernel permission authority;
- expose application-specific rich actions as kernel APIs;
- let the LLM directly own external credentials or raw API credentials;
- let the LLM freely compose external API or CLI calls as the production path;
- import kernel internals instead of using public kernel transport or syscalls.

External channel actions belong to connector/action/outbox owners. A connector
adapter may internally use an SDK, HTTP API, `lark-cli`, mail CLI, or another
tool, but that is connector implementation detail rather than a kernel ability
or model-owned external credential. CLI-backed adapters should translate
`ConnectorAction` through connector driver configuration or an external adapter
process; connector runtime code should not become a permanent collection of
hardcoded external CLI commands.

The long-lived outbound adapter process boundary is `connector_command`. The
connector runtime sends typed `ConnectorAction` JSON to a configured external
adapter process and accepts typed `ConnectorActionResult` JSON back. The runtime
then validates the result and writes `DeliveryReceipt`. `command_template` is
only a transitional CLI driver for early connector smoke tests; it is not the
stable Genesis protocol.

The long-running inbound source adapter process boundary is `source_command`.
It is not the same protocol as `connector_command`: source adapters emit typed
`source.ready`, `source.event`, `source.cursor`, `source.failed`, and
`source.stopped` NDJSON frames. The connector runtime validates frames and
writes source lifecycle, failure, cursor, and verification facts. Feishu,
WeChat, email, webhook, SDK, HTTP, or CLI details stay in source adapter code,
not in connector runtime. Genesis stores normalized source, action, result, and
receipt facts, not external command lines, raw stdout, raw stderr, SDK payloads,
or vendor HTTP responses.

Application issues belong in `docs/operations/application-issues.md`. Kernel
issues remain in `docs/operations/kernel-issues.md`.

## Application Requirements

- `application-connector-runtime-requirement.md`: the broad connector boundary
  contract for external events, request context, application commands, outbox,
  actions, and receipts.
- `connector-source-verification-lifecycle-requirement.md`: the production
  source readiness, lifecycle, event authenticity, cursor, operator-control,
  and failure-diagnostic contract for external listener, poller, webhook, and
  console sources.
- `connector-source-verification-lifecycle-design.md`: the owner split, data
  shapes, lifecycle, cursor, verification evidence, profile readiness,
  operator controls, and failure semantics for connector source intake.
- `connector-delivery-state-machine-requirement.md`: the production delivery
  state machine for connector outbox retry, leases, dead-letter,
  partial-success recovery, and receipts.
- `code-intelligence-runtime-requirement.md`: the production boundary for
  user-space code intelligence adapters, including CodeGraph cache readiness,
  query shaping, result evidence, and kernel non-ownership.
- `code-intelligence-runtime-design.md`: the owner split and data flow for
  `User-space Code Intelligence Runtime -> CodeGraph Adapter -> codegraph
  CLI/MCP`.
- `workflow-runtime-requirement.md`: the production boundary for
  developer-authored workflow config, compiled definitions, fixed workflow
  runs, deterministic node transitions, run logs, and the TaskGraph/Workflow
  split.
- `workflow-runtime-design.md`: the owner split, data shapes, execution model,
  config compilation, generated flowchart projection, failure behavior, and
  log-driven optimization model for user-space Workflow Runtime.
- `user-capability-package-design.md`: the user-home and package layout for
  organizing small user-defined external capabilities under `~/.genesis`
  without turning them into kernel features.
