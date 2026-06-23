# Design: Kernel Foundation Capabilities

- **Status:** approved.
- **Requirement:** `docs/requirements/kernel-foundation-capabilities.md`.
- **Owner:** Genesis Kernel.

## Boundary And Owner

The kernel owns the control and fact plane for turns, provider context, tool execution, authority, work records, memory truth, credential resolution, compaction, and inspection projections.

User-space owns shells, UI, daemons, provider commands, external applications, domain workflows, and skill packages. User-space can submit typed kernel commands and read projections, but it cannot write ledger facts, assemble provider context, execute governed tools directly, decide permission, or mutate memory truth.

## Data Flow

1. A shell or application submits a typed turn or kernel command.
2. The Interface Kernel validates transport shape, session identity, idempotency, and hidden-control boundaries.
3. The Model Gateway builds provider context from ledger-backed history, approved memory, skill metadata, tool rounds, and compaction state.
4. The provider returns a final answer or canonical tool calls.
5. ToolGateway validates tool calls, resolves authority, executes through registered executors, and records operation/tool events.
6. Accumulation and Work Registry record their own owner facts through kernel commands.
7. Timeline, context, audit, capability, session, work, and memory reads are projections from owner facts.

## Protocol

Kernel transport is typed command plus projection. HTTP is one transport and is not the durable contract.

Core conceptual commands and projections:

- `turn.submit`
- `turn.stream`
- `tool.invoke`
- `work.submit`
- `work.cancel`
- `memory.propose`
- `memory.review`
- `memory.recall`
- `credential.resolve`
- `audit.replay`
- capability, context, timeline, audit, and session projections.

Kernel-owned control fields stay out of model-visible schemas. Provider adapters translate kernel manifests to provider-native shapes but do not own tool permission, idempotency, execution, or ledger evidence.

## Failure Semantics

- Invalid transport or hidden control input fails before provider context construction.
- Provider failure is a Model Gateway failure, not command stderr.
- Invalid tool requests produce repair feedback when protocol state allows it.
- Permission denial blocks before effect and records policy evidence.
- Command failure returns terminal-equivalent output evidence.
- Tool infrastructure failure is separate from command failure.
- Credential failure blocks readiness or authorized effects without exposing raw secrets.

## Permission And Authority

`plan`, `default`, and `yolo` are user-facing modes. The kernel resolves them into authority policy, sandbox profile, and approval policy before any effect.

The model cannot select permission mode, sandbox profile, approval policy, credential authority, workspace root, idempotency identity, checkpoint refs, or audit refs through tool arguments.

Profile resolution is an Authority Plane responsibility. Tool executors receive an already-resolved policy and must not reinterpret user-facing modes locally.

Current profile semantics:

- `plan` resolves to read-only authority and a read-only sandbox profile.
- `default` resolves to workspace-write authority and `controlled_workspace`; this is an adapter-level workspace write gate, not an OS sandbox claim.
- `yolo` resolves to full-access authority and host execution.
- `on_request` approval blocks write-side effects at admission until an approval owner exists; it returns model-repairable `approval_required` feedback and records blocked operation evidence.
- unavailable stronger sandbox profiles fail closed before execution and return model-repairable sandbox feedback. They must not degrade to host execution.

Approval UI, prompts, or shell transports can request or display approval state, but they cannot decide authority, mint tool results, or mark a blocked operation as executed. Future interactive approval must be introduced as typed control-plane state owned by the kernel.

## Recovery And Observability

The ledger is append-only owner truth. Restart replay rebuilds session, operation, work, memory, timeline, context, audit, and readiness projections from recorded facts.

Durable storage is not a copy of every runtime signal. The Interface Kernel and owner subsystems write sparse facts. Streaming tokens, stdout chunks, heartbeats, and progress frames are realtime transport unless an owner reduces them to a transcript item, tool result, terminal job fact, checkpoint, or failure event.

Observability is split by audience:

- timeline for ordinary user-facing events;
- transcript for user and model-visible conversation recovery;
- raw events for ordered owner facts;
- audit for authority, risk, control, credential, and failure evidence;
- context for provider-visible inputs;
- capabilities and readiness for operator status;
- debug trace for opt-in, bounded, redacted diagnostics outside canonical replay.

Provider raw requests are not transcript. Production storage keeps derivation evidence such as included event refs, input kinds, manifest or skill refs, compaction refs, gateway profile id, and normalized usage. Full prompt or provider payload capture belongs only in debug trace, and even then stays bounded and redacted.

## Rejected Alternatives

- Application-owned provider context assembly is rejected because it creates multiple truth owners.
- Domain-specific kernel tools are rejected because they turn the kernel into an application.
- Prompt-only authority controls are rejected because hidden fields and permission decisions must be enforced by validators and owner gates.
- Version-numbered runtime route prefixes are rejected because they become stale compatibility surfaces.
- Treating audit as a general info log is rejected because it makes authority evidence noisy and unbounded.
- Persisting every stream chunk as a canonical ledger event is rejected because transport detail is not system truth.
