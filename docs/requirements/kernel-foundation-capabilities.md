# Requirement: Kernel Foundation Capabilities

- **Status:** approved.
- **Owner:** Genesis Kernel.
- **Scope:** foundation capabilities that make Genesis a shared LLM execution kernel rather than one application.

## Background

Genesis needs a small set of kernel capabilities that hold across every shell, skill, daemon, provider, and future application. Without this baseline, each application would be tempted to assemble prompts, decide permissions, run tools, write memory, and record evidence on its own. That would make Genesis a loose set of agents instead of a shared authority layer.

This requirement defines the baseline capabilities that every implementation slice must preserve: interface admission, model gateway, tool runtime, authority and credentials, work state, accumulation, readiness, inspection, and skill metadata.

## Production Target

Genesis Kernel is the authority execution layer for LLM-driven applications. It accepts intent, projects model context, governs tools, records facts, manages memory and work state, and exposes inspection surfaces.

The production target is:

- every accepted effect has a kernel-owned fact trail;
- every model-visible context fragment comes from an owner projection;
- every tool call goes through registry, validation, permission, execution, and evidence;
- every memory truth flows through candidate, review, and recall policy;
- every shell and application is user-space and cannot bypass kernel truth;
- every inspection surface is bounded, redacted, and purpose-specific.

## Users And Roles

Ordinary user:

- submits intent through a shell or application;
- sees assistant responses, user-facing timelines, memory review surfaces, and understandable blockers.

Operator/admin:

- configures provider, credential, skill roots, runtime token, permission profile, and workspace boundaries;
- inspects readiness, audit, context, capabilities, and ledger-backed diagnostics.

Reviewer:

- checks requirements, design, BDD behavior, issue evidence, retirement evidence, and regression tests;
- verifies that application-specific behavior has not entered the kernel.

LLM:

- sees kernel-projected context, approved memory context, safe skill metadata, and registered model-visible tools;
- proposes semantic tool arguments and memory/work content;
- does not create kernel ids, permission modes, sandbox profiles, credential refs, checkpoints, audit refs, or ledger facts.

Kernel:

- owns admission, authority, provider context, tool execution, event facts, memory truth, work truth, credential resolution, compaction, and projections.

Application:

- submits turns, supplies external context, reads projections, and may install skills or CLIs;
- does not assemble provider context, mint tool results, write memory truth, decide sandbox authority, or rewrite ledger history.

## Core Semantics

### System-Wide Semantics

1. The event ledger is the system fact layer. Session, timeline, audit, context, operation, memory, and readiness views are projections.
2. Control-plane fields are generated, bound, and validated by the kernel. The model can propose semantic content, not event ids, operation ids, session authority, sandbox profiles, approval policies, credential refs, checkpoint refs, or audit refs.
3. Provider context is assembled by the Model Gateway from ledger-backed facts. Same-session conversation history, approved memory, tool call/result rounds, skill index metadata, and compaction summaries are not synthesized by adapters.
4. Tools are registry-owned generic effects. Application-specific verbs do not enter the kernel as tool names.
5. Inspection surfaces expose bounded, redacted, path-safe projections. They are not hidden owner paths for raw secrets, skill bodies, provider-native payloads, or ordinary UI access to kernel internals.
6. Development-stage retired surfaces are removed from active code, tests, and requirements. Historical evidence may remain only in operations records.

### Persistence And Audit Layers

Genesis separates runtime output into five layers:

1. Realtime transport exists for streaming experience only. Token deltas, stdout chunks, progress frames, heartbeat frames, and stream frames live in memory or on the connection by default. They do not become long-term kernel facts until they are reduced to a completed message, tool result, job summary, terminal job fact, or another owner event.
2. Session transcript is the recovery and user-experience spine. It stores user messages, final assistant-visible replies, model-visible tool calls, model-visible final tool results, and product-approved reasoning summaries or notices. It does not store provider raw payloads or hidden reasoning chains.
3. Kernel durable facts store recovery and state truth. Checkpoints, terminal outcomes, permission denials, operation status, job terminal state, compaction outcome, memory review decisions, work decisions, and provider usage accounting belong here even when they are not ordinary UI content.
4. Security and control audit is strong-persistence and low-noise. It records authority changes, permission denials, credential use, dangerous-operation decisions, control-plane writes, governance publication or intake, break-glass actions, boundary-crossing access, and security failures. Ordinary success info and UI actions do not enter this audit layer.
5. Debug trace is opt-in. It may record provider projection summaries, response summaries, internal spans, chunk-level diagnostics, and gateway decisions, but it must have explicit enablement, bounded retention, quota, and redaction. Debug trace does not participate in replay, memory, provider context, or audit decisions.

A runtime event can enter long-term facts only when it is user-visible or model-visible, required for replay/recovery/idempotency/checkpointing, changes kernel-owned state, records a permission or risk decision, records failure or abnormal termination, or feeds provider context, compaction, memory recall, or observation delivery. Otherwise it stays in realtime transport, debug trace, or aggregate metrics.

Database storage follows the same owner boundary. A database is not an owner; it
is a persistence backend selected by an owner. Each table must name its owner and
class before it exists: canonical truth, read model/projection, audit, metrics,
debug trace, queue, or index. One PostgreSQL or SQLite instance may hold tables
for several owners, but table semantics and write authority must not be mixed.

Canonical owner tables hold recovery, permission, lifecycle, state transition,
idempotency, and audit truth. Read-model tables exist for UI lists, search,
filtering, and previews; they must remain rebuildable from canonical owner facts
and resource/object refs. A fast UI query does not make the projection table the
truth source.

Database rows store facts about content, not large content bodies. User uploads,
sandbox artifacts, checkpoint snapshots, long transcript segments, export
packages, raw provider payloads, raw external webhook payloads, and debug bundles
belong in a resource/object/file owner. The database stores refs, owner, hash,
size, mime type, sensitivity, lifecycle, storage ref, grants, and timestamps.

Table grain must follow access and lifecycle, not object names. High-frequency
queries, permission checks, lifecycle checks, and idempotency checks use explicit
columns and constraints. Low-frequency, shape-unstable details that are read only
with a parent object prefer JSON payloads or payload refs. A one-to-one child
table is suspect unless it has an independent lifecycle, permission boundary,
query frequency, or hot/cold storage value. State history should not become one
table per state unless the history is canonical state transition evidence rather
than debug or projection detail.

Production tables that cross a user, tenant, workspace, project, or owner-scope
boundary must carry the isolation field in their keys and constraints. Service
code remembering to add `WHERE user_id = ...` is not an isolation design. A
future PostgreSQL RLS policy may strengthen isolation, but schema constraints
must still express the boundary.

JSONL and file stores are lab seams unless a requirement declares them as the
production owner store. When an owner moves from JSONL or a file-backed lab store
to a production database, the old product write path must be retired in the same
slice or in a named cleanup slice. Long-term DB/file dual-write truth is not
allowed.

Every production store or schema proposal must answer:

- owner and owner public API;
- table class;
- reason the data needs a database instead of object/file storage or a rebuilt projection;
- rebuildability and rebuild owner;
- content boundary between explicit columns, JSON/payload refs, and object refs;
- transaction boundary and crash recovery order;
- idempotency keys, unique constraints, and legal state transitions;
- user, tenant, workspace, project, or owner-scope isolation constraints;
- retention, deletion, TTL, archive, and compaction rules;
- required indexes, with speculative indexes rejected until a query exists;
- migration and retirement plan for any JSONL or file lab store being replaced;
- negative tests proving raw provider payloads, stdout chunks, token deltas, raw webhooks, large bodies, credentials, and debug floods do not enter canonical tables.

### Interface Kernel

- `turn.submit` accepts user or application intent through a typed transport schema.
- Unknown transport fields, hidden control text, unsupported input item types, and attempts to set kernel-owned control fields fail closed before provider context construction.
- Prompt-injection-shaped content inside ordinary user text remains untrusted content. It may be recorded as risk metadata, but it does not grant authority.
- Turn idempotency is scoped to explicit `session_id + turn.submit + idempotency_key`. Replays return original ledger-backed evidence without new provider calls or tool effects.
- `turn.stream`, session, timeline, audit, and context inspection read from ledger-backed projections.
- HTTP is a transport for typed kernel commands and projections, not the durable contract.

### Model Gateway

- Provider integrations use a typed boundary. External provider commands own vendor SDKs, HTTP payloads, account flows, and provider credentials.
- Built-in provider adapters are local operator conveniences, not the default contract for new providers.
- Provider requests contain ordered input fragments, model-visible tool manifests, and prior model-visible tool rounds.
- Provider requests omit kernel-owned event ids, operation ids, leases, permission profile internals, checkpoints, and audit refs.
- Provider-native usage is normalized into kernel evidence when upstream fields are present: input tokens, output tokens, total tokens, cache hit tokens, cache miss tokens, and provider-backed processed input tokens.
- Token accounting belongs to the Model Gateway. Compaction selectors consume provider-backed accounting and do not fall back to local text token estimates.
- Provider failures become structured model/provider failures. They are not command stderr and are not disguised as tool results.
- Provider retry is a Model Gateway contract. Non-streaming provider calls may retry only pre-output transient failures such as temporary transport errors, HTTP 408, 429, and 5xx. Authentication, authorization, billing/quota, configuration, request-shape, provider-command process, and provider-command response-shape failures fail fast with typed reasons.
- Provider retry evidence is durable and bounded. Each retry, repair, and final failure records attempt status, reason code, retryability, and a redacted message without storing raw provider payloads or credentials.
- A provider response that is syntactically valid but lacks a visible final answer is repairable only through a bounded Model Gateway visible-final repair step. The repair prompt may ask for a visible answer; it must not replay hidden reasoning. Repeated empty visible finals fail with a typed provider-visible-final reason.
- Future streaming provider reconnects must preserve the no-replay-after-visible-output rule: a stream may be retried only before visible assistant output or tool calls have been accepted into the kernel fact trail.
- Context compaction is executed by a kernel compaction runner. Triggers submit typed kernel commands; shells, adapters, provider commands, and daemons do not summarize, truncate, or rewrite history.

### Tool Runtime

- `ToolRegistry` is the single source for tool name, description, schema, side-effect level, execution kind, and executor binding.
- `ToolGateway` is the only runtime entry for model-requested tools.
- The default model-visible tool set starts with generic `shell_exec`; no application-specific outbound tool is introduced by default.
- Model-visible schemas expose only semantic fields the model must choose.
- Model-supplied control-plane fields produce repairable `tool_request_invalid` feedback and no effect.
- Tool call batches are preflighted as a unit before any effect executes.
- Tool results preserve the distinction between invalid request, permission denial, command failure, and tool infrastructure failure.
- Long output is presented with bounded head/tail text, truncation flags, original byte counts, omitted byte counts, and a visible omission marker.
- Redaction is projection policy. It must not mutate append-only operation evidence before it is recorded.

#### Tool Scheduling And Concurrency

Tool concurrency is a kernel scheduling decision, not a provider decision and
not a simple read/write shortcut. A provider may emit several tool calls in one
step, and a provider adapter may expose native `parallel_tool_calls` or similar
vendor flags, but those signals do not grant execution authority. `ToolGateway`
validates schema, permission, resource refs, and tool registration first, then
derives a kernel-owned access plan for each call before any call executes.

`side_effect_level` remains useful for permission admission, but it is not a
complete concurrency contract. Production scheduling must use richer internal
metadata:

- `effect_class`: `pure_read`, `state_read`, `workspace_write`,
  `kernel_state_write`, `process_start`, `process_io`, or
  `external_side_effect`;
- `resource_footprint`: read scopes, write scopes, session/kernel state scopes,
  job or process handles, external targets, and credential/resource grant refs;
- `parallel_policy`: compatible locks, serial fence, per-handle serial, or
  background-after-admission.

These scheduling fields are kernel/control-plane facts. They are not
model-visible tool arguments and the model cannot override them. Tool handlers
may declare capabilities, but the scheduler must fail closed when metadata is
missing, unknown, or incompatible. Unknown tools, unknown effect classes, and
tools without a trusted access plan are serial fences.

The scheduler partitions provider-ordered calls into execution batches:

1. Compatible `pure_read` calls may run concurrently when their footprints do
   not depend on prior uncommitted facts.
2. `state_read` calls, including reads of turn receipts, evidence ledger,
   checkpoint state, job state, or memory review state, wait until all prior
   provider-ordered facts they may observe have been committed.
3. `workspace_write`, `kernel_state_write`, and unknown calls are serial fences
   in the first production-safe implementation. Future lock-set analysis may
   allow independent writes, but only after the resource footprint and
   idempotency contract prove that they do not conflict.
4. `process_start` is serially admitted so the kernel can allocate job/process
   handles, leases, idempotency, and audit evidence deterministically. After
   admission, the managed job may run in the background under Work/Job control.
5. `process_io` such as status, stdin, signal, wait, or cancel operations is
   serialized per job/process handle. Two operations on the same handle must not
   race, even if their tool schemas look read-only.
6. `external_side_effect` calls are not ordinary parallel tool work. They must
   route through the appropriate connector/outbox owner with idempotency,
   delivery receipt, and reconciliation evidence.

Execution may be concurrent, but durable facts and model-visible tool results
remain deterministic. `tool.call` events are written in provider call order.
`tool.result` events and provider-visible result rounds are projected in the
same provider call order, even when the underlying executions finish in a
different order. UI and diagnostics may show live concurrent progress, but they
must not turn completion order into ledger order, transcript truth, checkpoint
truth, or provider replay order.

The current default shell tool stays conservative. `shell_exec` is an arbitrary
process primitive and is treated as effectful/serial unless it is routed through
a future hard read-only sandbox or replaced by a narrower registered read tool
with a trusted access plan. A shell command that looks like `rg` or `cat` is not
automatically a `pure_read` kernel effect, because shell syntax, scripts,
environment variables, network access, and invoked programs can create hidden
side effects.

Crash and resume semantics are part of scheduling. Once a non-idempotent or
external effect has been admitted, replay must return the recorded operation,
job, outbox item, or repair evidence instead of executing it again. Rejected
batches must still fail closed before any effect, and admitted batches must
record enough ordering evidence to resume without duplicating completed or
in-flight effects.

### Authority And Credential Plane

- Runtime-protected routes require a configured runtime token. Readiness is blocked when protected work cannot be accepted.
- Credentials are referenced through kernel-owned refs, not raw secrets in config, prompts, events, logs, readiness, provider context, or model-visible tool results.
- `plan`, `default`, and `yolo` are user-facing permission modes. The kernel resolves them into `authority_policy`, `sandbox_profile`, and `approval_policy` before admission.
- Current `default` is a controlled-workspace adapter, not an OS-level sandbox claim.
- Default `approval_policy` is `never`. `on_request` is a kernel-owned admission state: until an approval owner exists, write tools are blocked with structured `approval_required` feedback and no side effect.
- A stronger workspace OS sandbox profile is a future enforcement target. If configured before an executor can enforce it, admission fails closed with structured sandbox feedback rather than silently running unconfined.
- Tool arguments cannot select permission mode, sandbox profile, approval policy, workspace root, credential authority, or runtime client authority.

#### Future Sandbox And Approval Production Semantics

An OS-level sandbox is an execution enforcement adapter, not a permission mode
label. A future `os_workspace` profile must create enforceable execution
evidence before a tool effect runs and must report terminal sandbox outcome
after execution. If the adapter cannot enforce the requested profile on the
current platform, the request remains blocked; the kernel must not silently
fall back to host execution.

Interactive approval is a kernel-owned control path. A write-side effect that
requires approval must first create a pending approval request bound to the
tool call, session, resolved policy, requested effect, and requester-visible
summary. A UI, CLI, desktop shell, or external application may display that
request and submit a decision, but it cannot decide authority locally, mint a
tool result, or rewrite operation state.

An approval decision is valid only when submitted through a kernel-owned command
that binds:

- kernel-generated approval id;
- original tool call or operation ref;
- approving or denying authority;
- decision: approved or denied;
- reason and evidence ref;
- policy snapshot being approved;
- timestamp generated by the kernel.

Approval denial records a terminal blocked decision and no external effect.
Approval approval records decision evidence before execution, then the tool
runtime executes under the already-resolved policy and sandbox profile. Approval
does not let a caller broaden permission mode, change workspace root, change
sandbox profile, select credentials, or inject model-visible control fields.

### Work Registry

- `work.submit` records a kernel-owned work item with session linkage, title, and source ref.
- `work.cancel` records terminal cancellation evidence with authority, reason, and evidence ref.
- Work records survive restart and project through sessions.
- Work Registry is not an application task system, notification system, queue worker, retry engine, lease system, or scheduler unless those needs are reduced to generic kernel primitives.

### Accumulation

- Memory enters the kernel as a candidate, not as a silent model promise.
- Pending candidates require source refs and remain out of recall until approved.
- Approval, rejection, and supersession are durable owner decisions with authority, reason, and evidence.
- Rejected and superseded candidates are excluded from recall.
- Supersession creates a replacement pending candidate; it is not hidden approval or direct text mutation.
- `memory.recall` is a read-only observation surface. It does not run a model, append review evidence, or mutate candidates.
- Turn submission may record recalled approved memory refs on the admitted turn event.

### Readiness And Inspection

- `/ready` reports whether the kernel can accept protected work and names structured blockers.
- Capability inspection reports provider/runtime/ledger status, canonical kernel tool names, and safe skill metadata.
- Timeline, raw events, audit, and context inspection are separate projections for different audiences.
- Context inspection reports provider-visible input kinds, tool manifest names, skill metadata summaries, approved memory refs, provider status, and resolved permission profile without exposing full rendered prompts or raw secrets.
- Audit inspection reports event types, operation status, provider context input kinds, usage, failure codes, and truncation metadata.
- Ordinary UI timeline omits kernel-owned ids and control-plane internals unless the user opens a diagnostics surface.
- Ordinary UI timeline is the chat-readable projection. It shows user messages, final assistant messages, and compact processing summaries; it does not render raw kernel events, raw tool results, raw job lifecycle events, audit facts, or context inspection facts as chat rows.
- A turn processing group has live and terminal projection states. While the turn is running, the shell may display a changing elapsed label such as `正在处理 45s` or `正在处理 1m 5s`; this changing label is realtime/projection state and is not appended as a durable tick. After the turn settles, the projection fixes the duration label, such as `已处理 1m 5s`, from recorded start/end facts.
- Tool and job activity is summarized under a processing group. Normal command failures, malformed command results, job failures, stderr, and long stdout/stderr do not create ordinary chat messages and do not default-expand the timeline. They remain available through detail or diagnostics projections with bounded previews and truncation metadata.
- Approval and user-input requests are user-action projection nodes, not assistant messages and not tool failure rows. They may appear in the ordinary timeline because the run needs a user decision, but they must keep authority actions separate from transcript content.
- Detail projections are selected-node read models. They may show tool group details, operation status, command preview, duration, bounded output, truncation, and detail refs. They remain separate from raw event JSON, audit replay, context inspection, sandbox evidence, and debug trace.

### Skill Catalog

- Skill packages are user-space assets. The kernel may index safe metadata, but skills do not become kernel APIs.
- Configured skill roots can be scanned for `SKILL.md` metadata.
- Skill root scanning is bounded by recursion depth, candidate count per root, and metadata file size before any provider-context projection is built.
- Provider context receives only a bounded path-free metadata index by default.
- Skill bodies, instruction paths, package paths, and full examples are not injected into every turn.
- Unsafe, malformed, duplicate, linked-path, authority-shaped, hidden-control, prompt-injection-shaped, or secret-shaped metadata is excluded rather than repaired into model context.
- Full skill hydration, if added later, must use a generic resource/context contract and must not introduce a package-specific skill-body retrieval tool.
- Skill metadata can help the model discover user-space capabilities, but it grants no authority.

## Non-Goals

The foundation kernel does not include:

- CLI, WebUI, desktop UI, or mobile UI product behavior;
- Feishu, WeChat, email, calendar, document, OCR, web search, medical, insurance, or other domain logic;
- full skill-body injection by default;
- application-specific outbound APIs;
- multi-agent scheduling as a kernel primitive;
- vector database optimization as a first requirement;
- migration compatibility for retired Python data surfaces.

## Phased Delivery

Phase A: turn, ledger, fake provider, readiness, and restart-safe session replay.

- Proves: admission, event facts, provider loop shape, readiness blockers, and restart replay.
- Still short of production: no real provider, no governed tool loop, no accumulation, no work evidence.

Phase B: tool runtime, permission profile, shell execution, and terminal-equivalent tool results.

- Proves: registry ownership, model-visible tool manifest, permission denial, command output evidence, and repair feedback.
- Proves now also: configured unavailable sandbox profiles and approval-required write effects fail closed before execution with model-repairable feedback.
- Still short of production: shell sandbox is controlled workspace rather than OS sandbox; interactive approval is not implemented; richer job progress, interrupt behavior, and tool scheduling/concurrency remain governed by this requirement and the shell/job requirement.

Phase C: work registry, accumulation, credential plane, and protected inspection.

- Proves: memory candidate/review/recall, work submit/cancel, runtime token, credential blockers, capabilities, timeline, audit, and context projections.
- Still short of production: richer memory selection, approval, stronger sandbox, and broader recovery policy remain future work.

Phase D: real provider boundary, provider-backed usage accounting, multi-turn projection, skill metadata, and compaction.

- Proves: provider command, built-in provider convenience, model usage normalization, provider-backed token accounting, metadata-only skills, and kernel-owned compaction.
- Still short of production: full use-time skill hydration, richer context policy, progress snapshots, and idle continuation policy remain future work.

Phase E: hardening and production readiness.

- Proves: stronger sandbox/approval where available, managed-job hardening, interrupt semantics, deterministic tool scheduling, and broader recovery evidence.
- Still short of production until complete: stronger authority flows, foreground attach-or-kill, arbitrary long-running effect recovery, and richer resource-footprint based parallelism remain constrained.

## Acceptance Criteria

Positive cases:

- valid turn submission produces ledger events, provider result, session projection, and restart replay;
- fake and real provider paths return structured final responses;
- valid governed shell execution returns terminal-equivalent result evidence;
- compatible pure-read tool calls can be planned into one parallel batch without changing provider-visible result order;
- approved memory can be recalled in a later turn;
- protected inspection surfaces show readiness, capability, timeline, audit, and context projections;
- ordinary timeline can present a turn as user message, processing group, final assistant message, and optional user-action request without exposing raw events or control-plane ids.

Negative cases:

- malformed transport fields fail before provider context construction;
- unauthorized tool effects are blocked before execution;
- model-supplied control-plane fields are rejected;
- unknown tools, unknown scheduling metadata, state reads after uncommitted prior facts, conflicting writes, and same-handle process I/O do not run in parallel;
- `shell_exec` is not classified as pure read by command text inspection alone;
- tool/job failures do not become ordinary chat messages and do not force the processing group open after the final assistant message starts;
- approval-required state is projected as a user action rather than as an assistant-authored message or a generic failed tool row;
- raw secrets do not appear in context, logs, readiness, events, or model-visible results;
- unsafe skill metadata is excluded;
- rejected and superseded memories do not enter recall.

Fail-closed and recovery:

- provider failures are structured and do not panic the kernel;
- idempotent turn retries do not repeat provider calls or tool effects;
- idempotent tool retries do not repeat effects;
- replay of an admitted non-idempotent or external effect returns recorded evidence instead of executing it again;
- restart replay reconstructs session, work, operation, and memory projections from ledger facts.

Audit and visibility:

- ordinary timeline, raw events, audit, and context projections remain separate;
- bounded output includes truncation metadata;
- live elapsed labels update without appending durable per-second facts, and terminal duration labels are fixed from recorded timing facts;
- concurrent execution progress may be visible to UI/diagnostics, but ledger, transcript, checkpoint, and provider replay order stay deterministic;
- readiness explains blockers without exposing secrets.

Test evidence:

- focused owner tests for each positive and negative path;
- architecture boundary tests for user-space separation;
- build and full test evidence before issue retirement.

## Relationship To Existing Issues

This requirement governs the foundation baseline and is the source for future foundation gaps.

Current related active issues:

- `KERNEL-SANDBOX-APPROVAL-NEXT-20260623`: implementation gap for stronger sandbox and approval beyond the current authority-profile split.
- `KERNEL-JOB-CONTROL-INTERRUPT-20260623`: remaining interrupt, progress snapshot, idle continuation, and foreground attach-or-kill semantics. It is governed by the shell/job requirement because it extends the generic Tool Runtime and managed-job path rather than the foundation baseline itself.

Related ready-for-acceptance shell/job evidence:

- `KERNEL-SHELL-TIMEOUT-CAP-20260623`, `KERNEL-MANAGED-JOB-FOUNDATION-20260623`, `KERNEL-OBSERVATION-DELIVERY-20260623`, `KERNEL-RESOURCE-PURE-READ-PRIMITIVE-20260624`, and `KERNEL-TOOL-SCHEDULING-CONCURRENCY-20260624` are recorded in `docs/operations/kernel-retirement-log.md`.

Issues should cite this requirement only for gaps against these production semantics. They should not restate the full requirement or reopen application-specific kernel ownership.
