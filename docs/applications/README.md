# User-space Applications

This directory records requirements and designs for applications and connectors
that exercise Genesis Kernel without becoming kernel owners.

Application documents follow the same discipline as kernel requirements:
requirements describe production semantics, designs describe ownership and
data flow, implementation plans describe staged delivery, and issues record
current gaps. The boundary is different: application requirements must not
create kernel capabilities unless they reduce the need to an approved generic
kernel primitive.

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
or model-owned external credential.

Application issues belong in `docs/operations/application-issues.md`. Kernel
issues remain in `docs/operations/kernel-issues.md`.
