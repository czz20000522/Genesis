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

Public projection responses keep collection fields stable for shells and
operator clients. Top-level collection fields that are part of a read-model
contract serialize as JSON arrays, including the empty case, rather than
`null` or omitted fields. Projection-tree child collections, capability skill
catalog collections, context input/tool/skill/memory collections, memory list
collections, audit item collections, and turn-event item collections follow the
same rule when clients are expected to iterate them. Optional scalar fields and
diagnostic-only nested collections may still be omitted when absence is the
documented meaning.

## Model Gateway Resilience

Provider reliability belongs to the Model Gateway, not to shells,
applications, provider commands, or connector adapters. The gateway classifies
provider failures before it decides whether a turn may continue.

For the current non-streaming provider surface, the retry planner is:

1. Build the provider request from ledger-backed context.
2. Call the provider once.
3. If the failure is a pre-output transient transport or status failure, such
   as HTTP 408, 429, or 5xx, record `model.provider_attempt` evidence and retry
   within the bounded attempt budget. `Retry-After` may delay the retry within
   the kernel cap.
4. If the failure is authentication, authorization, quota/billing,
   configuration, request-shape, provider-command process failure, or
   provider-command response-shape failure, fail fast with a typed
   `turn.failed` reason and do not retry.
5. If the provider response has tool calls, pass them to ToolGateway through
   the ordinary tool loop.
6. If the provider response has a visible final answer, record the final answer
   and complete the turn.
7. If the provider response has no visible final answer and no tool calls,
   record `model.provider_repair` evidence and issue a bounded repair request
   asking only for a visible final answer. Hidden reasoning is not replayed.
8. If the repair budget is exhausted, fail with a typed visible-final-required
   reason.

Retry and repair evidence is inspection and audit material, not transcript
content. It may include attempt number, max attempts, status, reason code,
retryability, repair kind, and redacted diagnostic text. It must not include
provider raw payloads, credentials, authorization headers, hidden reasoning, or
provider-native request bodies.

Provider retry does not re-execute already admitted tools. Once a provider step
has produced accepted tool calls or visible output, the next failure is handled
at the next provider boundary; it is not a reason to replay previous effects.
When future streaming provider support exists, the stream reconnect rule is
stricter: retry is allowed only before visible assistant output or tool calls
have been accepted into the kernel fact trail.

## Skill Catalog Projection

Skill packages are user-space assets. The kernel scans configured skill roots only to build a safe metadata index for capability and provider-context projection. Discovery is bounded by recursion depth, candidate count per root, and `SKILL.md` metadata file size before parsing. Exclusions use stable path-free reasons so `/capabilities` can explain skipped metadata without exposing package paths, skill bodies, or heavy file contents.

Reference alignment:

- Reasonix bounds skill-root discovery depth and candidate count before registering user-space skill packages.
- Codex keeps skill listing/reading as typed app-server surfaces and separately budgets model-visible skill context.
- Genesis intentionally differs by indexing only safe name/description metadata by default; full skill bodies remain outside the kernel provider context unless a future generic resource/context contract is approved.

## UI Timeline Projection

The user-facing timeline is a projection tree, not the raw event stream and not
the provider replay context. Its job is to keep the conversation readable while
long-running model work remains inspectable.

The projection has two turn views:

- live turn projection: used while a turn is running. It can default the
  processing group to expanded so the user can see compact progress summaries
  and grouped tool activity while work is in flight. Its elapsed label is
  computed from live clock plus recorded start time, for example `正在处理 45s`;
  the changing label is transport/projection state, not a durable event tick.
- settled turn projection: used once the final assistant response starts. It
  defaults the processing group to collapsed and streams or shows the final
  assistant message as the primary content. Its processing label is fixed from
  recorded timing facts, for example `已处理 1m 5s`.

The conceptual shapes are:

```text
LiveTurnProjection
  live_work_items[]
  live_final_message?

SettledTurnProjection
  user_message
  processing_group
  assistant_message

WorkGroupDetailProjection
  progress_notes[]
  tool_groups[]
  compaction_events[]
  checkpoint_events[]
  user_action_requests[]

OperationDetailProjection
  command_preview
  status
  duration
  visible_output
  truncation
  detail_ref?
```

These names describe projection responsibilities, not mandatory endpoint names
or stored table names. A transport can serve them through `timeline` and
protected detail/inspection routes as long as the owner and visibility rules
stay intact.

The transition is triggered by the first assistant final-message delta or final
assistant event for the turn, not by a later cleanup pass. The shell should then
render the processing group as a compact row such as fixed duration, tool count,
job count, and compaction count. The user can reopen the group. That manual open
state is shell-local UI state and must not become kernel truth.

The recommended node hierarchy is:

```text
turn
  user_message
  processing_group
    progress_note
    tool_group
      operation_detail
    compaction_notice
    user_action_request
  assistant_message
```

`processing_group` is a UI projection of work already recorded in owner facts.
It does not create a new kernel work owner and it does not replace operation,
job, compaction, or tool facts. `tool_group` groups adjacent or causally related
tool activity for readability. Tool failures, command syntax errors, job
failures, stderr, and long stdout/stderr do not become ordinary chat messages
and do not force the settled processing group open. `operation_detail` exposes
command preview, status, duration, bounded visible output, truncation metadata,
and a detail ref when a protected inspection surface can provide more
information.

Realtime deltas are applied by upsert, not by appending a new visible row for
every chunk. A shell should update the node identified by the stable item or
operation ref: reasoning/progress deltas update the progress node, command
output deltas update the command node, and tool progress updates the tool node.
Chunk transport remains non-canonical until an owner reduces it to a transcript
item, tool result, job fact, checkpoint, or failure event.

The main timeline, detail pane, and diagnostics panel have different jobs:

- main timeline: user message, collapsed or expanded processing group, final
  assistant response, and explicit user-action requests;
- detail pane or drawer: selected node details, including command output,
  diff previews, tool arguments/results, truncation metadata, and references to
  protected inspection surfaces;
- diagnostics: raw kernel event JSON, audit replay, context inspection,
  provider input kinds, sandbox/authority evidence, and debug traces when
  enabled.

Approval and user-input requests are not chat messages. They are control
surfaces tied to authority and request lifecycle. A shell may display them near
the conversation as standalone action nodes because the run needs user input,
but the timeline must not present them as assistant-authored content and must
not let them mint tool results outside the owning kernel path. Approval action
nodes should behave like Reasonix-style approval prompts: the pending action is
visible and answerable, while the underlying command or tool failure remains a
detail/diagnostics concern until the kernel owner records the approved or
denied outcome.

Reference alignment:

- Codex-style desktop rendering shows the desired interaction: live activity is
  visible while work is running, then the processing work collapses when the
  final answer starts streaming.
- Reasonix treats approval as a distinct request/control surface rather than a
  tool result row or assistant message, which matches Genesis' authority-plane
  boundary.
- CodexGui is useful as a shell reference for typed thread/turn/item cards,
  live delta aggregation through upsert, card previews plus detail panes, and
  approval surfaces outside the chat stream.
- CodexGui is not the Genesis projection contract. Its current flat
  conversation item model and concentrated view-model logic are application
  implementation choices, not kernel design. Genesis keeps projection grouping
  in backend/kernel-owned read models so WebUI, desktop UI, CLI, and future
  external shells do not each reinterpret raw events.

## Tool Scheduling

Tool scheduling belongs to ToolGateway and the kernel runtime. Provider adapters
may translate native tool-call batches into Genesis tool calls, but they do not
decide which calls can run in parallel. Tool handlers may declare trusted
scheduling metadata, but the scheduler owns the final access plan and fails
closed when metadata is absent or incompatible.

The scheduling flow is:

1. normalize provider tool calls into provider order;
2. resolve each tool through `ToolRegistry`;
3. validate model-visible arguments and reject hidden control-plane fields;
4. authorize each call through the Authority Plane;
5. derive a `ToolAccessPlan` from effect class, resource footprint, state
   dependency, handle/lease scope, idempotency, and external target;
6. partition calls into deterministic execution batches;
7. execute compatible calls concurrently only when the plan allows it;
8. append durable tool facts and project model-visible tool results in provider
   call order.

The first implementation should expose the planner as a pure function before it
adds real executor parallelism. The planner can be tested with synthetic tool
specs and footprints while current `shell_exec` behavior remains serial.
`shell_exec` is not classified as a pure read by command text inspection; only a
future hard read-only sandbox or a narrower registered read tool can provide a
trusted pure-read access plan.

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

### Future Sandbox / Approval Flow

The current kernel blocks `approval_policy=on_request` write effects because no
interactive approval owner exists yet. A production approval path must extend
the same Authority Plane instead of adding a shell-local prompt or transport
shortcut.

Future flow:

```text
model tool call
  -> ToolGateway validates schema and rejects hidden control fields
  -> Authority Plane resolves permission_mode / authority_policy /
     sandbox_profile / approval_policy
  -> approval required?
       yes: write approval.requested and stop before effect
       no: continue
  -> sandbox profile enforceable?
       no: write sandbox block and stop before effect
       yes: acquire sandbox execution boundary
  -> execute tool
  -> write operation/tool result evidence
```

Approval decision flow:

```text
shell/UI/application displays approval.requested
  -> caller submits approval decision command
  -> kernel validates approval id, authority, policy snapshot, and evidence
  -> denied: write approval.denied and terminal operation block
  -> approved: write approval.approved before effect admission resumes
```

The approval owner must keep decision fields out of model-visible tool schemas.
The model can request an effect; it cannot invent approval ids, permission
modes, sandbox profiles, workspace roots, credential refs, or decision evidence.

Sandbox enforcement flow:

```text
resolved sandbox_profile
  -> executor capability check
  -> profile unavailable: fail closed before effect
  -> profile available: run inside adapter boundary
  -> executor reports sandbox terminal outcome
```

The sandbox adapter may be OS-specific. Its unavailability is an explicit
kernel blocker, not a reason to fall back to host shell. The adapter reports
enforcement evidence to the kernel; provider-visible results remain minimal
repair or command evidence and do not expose process ids, host handles, policy
snapshots, or sandbox internals.

Reference alignment:

- Codex keeps approval policy, sandbox policy, and requested sandbox overrides
  in control-plane state; approval requests are events that precede execution,
  and approval responses are not model-authored tool results.
- Reasonix separates permission policy, interactive approval, and sandbox
  wrapper. A tool can be permission-gated before execution, and the UI renders
  approval as a standalone control surface rather than chat text.
- Genesis intentionally differs by failing closed for `os_workspace` until a
  concrete executor can enforce it; no silent host fallback is allowed for a
  configured stronger profile.

## Memory Context Sensitivity

Accumulation owns raw memory candidate truth, review decisions, and recall
eligibility. The Model Gateway owns the provider-visible memory fragment derived
from those facts. These are not the same surface.

The provider-context flow is:

1. Accumulation replays approved candidates and returns recall records with raw
   candidate text and source refs as owner truth.
2. Turn admission records the recalled refs on the turn event.
3. Before building `approved_memory_context`, the Model Gateway applies the
   model-visible context projection policy.
4. The projection keeps useful semantic text, but redacts credential-shaped
   substrings such as provider keys, bearer tokens, authorization headers,
   passwords, API keys, and connector tokens.
5. Session, context inspection, timeline, and provider-command projections use
   the same model-visible redaction boundary or a stricter inspection boundary.
6. The raw memory candidate remains unchanged inside the Accumulation owner
   store and review surfaces that are explicitly owner-authorized.

Approval therefore means "eligible for recall," not "safe to replay raw to the
provider forever." A future sensitivity owner may add explicit grants, scopes,
or credential handles, but the default projection remains conservative and does
not rely on prompt instructions telling the model to ignore secrets.

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

Store design starts with owner stores, not a global ERD. Session, resource,
kernel/runtime, job, audit, and projection owners each write the smallest
persistence proposal that satisfies the requirement's store gate. A proposal may
share one database instance with other owners, but it must still name the owning
API and table class for each table.

Initial owner-store proposals should use these entry points:

- session owner: session identity, transcript envelopes, request receipts, and timeline indexes;
- resource owner: resource refs, metadata, lifecycle, grants, body refs, storage refs, and derived preview refs;
- kernel/runtime owner: durable fact indexes, checkpoint refs, terminal outcomes, and context summary refs;
- job owner: job records, status transitions, observation queue, output summary refs, and cancel/wait facts;
- audit owner: risk, control, credential, break-glass, and governance records;
- projection owner: rebuildable UI, read, search, and preview tables.

Store proposals must write transaction boundaries before they name tables. The
first accepted user message and session creation must share a transaction.
Request receipt and idempotency key binding must share a transaction. Resource
metadata and body storage need an explicit consistency strategy, including how
the owner repairs an object written without committed metadata or metadata that
points to a missing object. Kernel durable fact append and checkpoint pointer
updates must define ordering and restart replay. Outbox writes that follow owner
state changes must share the owner transaction; external side-effect workers run
after commit and cannot roll back owner truth.

## Rejected Alternatives

- Application-owned provider context assembly is rejected because it creates multiple truth owners.
- Domain-specific kernel tools are rejected because they turn the kernel into an application.
- Prompt-only authority controls are rejected because hidden fields and permission decisions must be enforced by validators and owner gates.
- Version-numbered runtime route prefixes are rejected because they become stale compatibility surfaces.
- Treating audit as a general info log is rejected because it makes authority evidence noisy and unbounded.
- Persisting every stream chunk as a canonical ledger event is rejected because transport detail is not system truth.
- Starting database design from a global ERD is rejected because it encourages noun-based tables instead of owner-owned truth, projections, queues, and indexes.
- Keeping JSONL and database stores as permanent dual product truth is rejected because it splits replay, migration, and corruption semantics across two owners.
- Treating `read` versus `write` as the complete concurrency contract is rejected because state reads, process handles, external side effects, and arbitrary shell commands need kernel-owned access plans.
- Rendering raw kernel events directly as the chat UI is rejected because long turns become unreadable and shells would duplicate projection logic.
- Persisting UI open/closed state as kernel truth is rejected because collapse is a shell projection preference, not a system fact.
- Treating approval prompts as assistant messages is rejected because approvals are authority-control interactions, not model-authored transcript content.
- Rendering tool or job failures as ordinary chat rows is rejected because command-level failure is a detail/diagnostics fact unless the kernel needs an explicit user decision.
