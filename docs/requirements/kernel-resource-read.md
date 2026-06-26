# Requirement: Kernel Resource Read

## Background

Genesis needs a generic way to let the model read bounded, already-admitted
resource content without turning every domain into a kernel feature. Skills,
connector attachments, application payloads, generated artifacts, and future
object-store records all need a common read path. The same primitive also gives
the Tool Runtime a legitimate non-shell `pure_read` candidate for future safe
parallel execution.

## Production Goal

The kernel owns stable resource references, metadata, lifecycle, grants, body
storage references, preview references, and bounded read projections. A model can
read only a resource ref that the kernel or an application has already admitted
into the current task context. Reading a resource is a governed kernel tool
operation with schema validation, permission/grant checks, bounded output,
content-fidelity policy, projection-budget metadata, and deterministic
tool-result ordering.

## Roles

- User/application: creates, admits, or references resources through an
  application or future resource intake path.
- Kernel: owns resource refs, read authorization, bounded projection, tool
  result shape, and audit/replay facts.
- LLM: chooses a visible resource ref and optional range/page parameters; it
  cannot invent authority, filesystem paths, credentials, storage paths, or raw
  object ids.
- Shell/UI/connector: can display resource refs and read projections, but cannot
  bypass the resource owner, write resource truth, or mark a resource read as a
  tool result.

## Core Semantics

- `resource_ref` is an opaque model-visible handle, not a filesystem path and
  not an external protocol id.
- `resource_ref` is one kind of Public Reference under the kernel Reference
  Model. Runtime handles such as `event_id`, `tool_call_event_id`, `job_id`,
  `operation_id`, `work_ref`, `request_ref`, and `checkpoint_ref` are not
  resources and must not be accepted by `resource_read`.
- Owner-internal refs such as `storage_ref`, object keys, database row keys,
  host paths, raw provider payload refs, debug trace paths, connector raw
  payload ids, and skill package paths must never become model-visible
  `resource_ref` values.
- A resource read is `pure_read` only when the resource owner can prove the ref
  points to immutable or snapshot-stable content for the duration of the turn.
- Unknown, expired, unauthorized, binary-only, or malformed refs fail before any
  body read and return repairable `tool_request_invalid` feedback when protocol
  state allows it.
- Output is bounded by bytes and/or line/page parameters. Results include
  `truncated`, original size when known, next offset/page when available, and
  content type.
- Resource reads must never expose storage paths, connector credentials,
  external raw payloads, skill package paths, raw debug payloads, or hidden
  control-plane ids.
- Resource refs can participate in scheduling footprints. Compatible immutable
  reads can be planned into the same parallel batch; reads of mutable owner
  state remain `state_read`.

## Reference Descriptor And Operation Grants

Genesis should unify reference identity and admission without merging every
reference into a universal read tool. The resource owner should be able to
project a descriptor for each model-visible resource:

```text
ReferenceDescriptor:
  ref
  ref_kind
  owner
  display_label
  available_operations
  scope
  provenance
  public_metadata
```

`available_operations` is a current projection computed from actor, scope,
grant, purpose, resource state, and active tool surface. It is not the authority
truth and it is not the same as static `supported_operations`. Tool execution
must re-run admission, because grants can expire, resources can become
unavailable, snapshots can be quarantined, and budgets can be exhausted after a
descriptor was projected.

Resource read uses an operation-level grant:

```text
operation = read_text
```

Future source and artifact tools may share the same reference descriptor and
resolver foundation, but they must keep typed tools and typed result schemas:

```text
source_tree   ~= list_children on a source snapshot/container
source_read   ~= read_text on a source file
source_search ~= search on a source snapshot
source_span   ~= read_span on a source span/citation ref
artifact_list / artifact_preview remain artifact-owner tools
```

Genesis must not introduce a universal `ref_read(any_ref)` result full of
optional fields for text, directories, media, artifacts, spans, and debug data.
The common part is descriptor, resolver, grant, and admission; the operation
tools stay typed by owner capability.

Skill loading, workflow node selection, project binding, and workspace context
do not grant resource access. They may surface a descriptor or request
admission, but the resolver still decides from actor, scope, grant, purpose,
snapshot, freshness, and budget. Resolver failure fails closed with structured
repair feedback and must not fall back to host paths or owner-internal storage
refs.

## Context Hydration Semantics

Context hydration is the act of admitting a bounded resource projection into a
future provider request. It is not the same thing as ordinary inspection or UI
preview. The kernel owns the hydration decision because hydrated text changes
what the model can reason over and may affect token pressure, compaction,
replay, and audit evidence.

Production context hydration must satisfy:

- the hydrated content is addressed by a kernel resource ref or another
  reviewed generic context handle, never by package path, connector id, external
  file path, URL, or raw payload id;
- the source owner records why the content is eligible for this session, turn,
  or task, including grant, scope, freshness, size, and content type evidence;
- the Model Gateway receives a typed context fragment with a hard byte/token
  cap and derivation refs, not a caller-built raw prompt string;
- hydrated content records provider-context derivation evidence without making
  the full rendered prompt the canonical transcript;
- repeated turns must be able to rebuild, compact, or omit hydrated content
  deterministically from owner facts rather than shell-local memory.

The production admission contract is:

- an application, connector, skill loader, or future resource intake path may
  request hydration only by submitting a generic resource/context handle plus
  source evidence; it cannot submit a caller-built prompt fragment;
- the resource/context owner validates scope, grant, content type, size,
  freshness, sensitivity, and derivation evidence before any provider context
  is assembled;
- admitted hydration becomes a kernel-owned fact bound to a session, turn, task,
  or checkpoint scope, with a stable context handle, bounded projection limits,
  input kind, and derivation refs;
- refused hydration is a structured owner refusal and must not silently fall
  back to raw prompt splicing, filesystem reads, connector payload paths, or
  package-specific tools;
- the Model Gateway renders only admitted hydrated fragments and records enough
  context-inspection evidence to explain which handles were included without
  making the full provider prompt canonical truth.

The current implementation supports session-scoped pending hydration only.
`context.admit_resource` records `context.hydration.admitted` for an admitted
bounded text resource with no `turn_id`; the next submitted turn consumes that
fact exactly once as a `hydrated_context` model input. The admitted fact stores
resource identity, hash, byte cap, visible byte count, truncation, input kind,
scope, and derivation evidence; it does not persist the provider-visible body.
The Model Gateway retrieves the bounded body through the resource owner while
assembling the next provider request. A non-empty `turn_id` is currently refused
with `admission_result=refused` and
`refusal_reason_class=scope_violation` because post-submit turn-scoped
hydration is not yet consumable by provider projection.

Skill packages are one possible source of hydrated context, not a special
kernel feature. The skill catalog remains metadata-only by default. If a future
turn needs a full `SKILL.md` body, a skill or application owner must first admit
that body as a bounded generic resource/context handle. The model may then
consume the handle through the generic context/resource path. It must not call a
skill-specific kernel tool such as `read_skill` or `skill.read`.

## Non-Goals

- No arbitrary filesystem reader is introduced by this requirement.
- No `skill.read`, Feishu attachment reader, mail attachment reader, document
  reader, OCR reader, or application-specific resource API enters the kernel.
- No connector or shell may read resource bodies directly and then fabricate
  kernel tool results.
- No production object store, attachment ingestion, OCR/binary rendering,
  vector index, freshness policy, richer selection policy, or retention policy
  is required in the first implementation slice.

## Phased Delivery

- Phase A: document the resource boundary and add an active implementation gap.
- Phase B: implement a minimal immutable text resource registry and
  `resource_read` tool. This is a controlled kernel primitive, not a production
  object store.
- Phase C: replace the in-memory/configured registry with a resource owner store
  proposal when applications or connectors need durable resource intake.
- Phase D: allow real executor-pool parallelism only for registered
  `pure_read` resource reads whose footprints are compatible and whose replay
  semantics are proven.
- Phase E: implement the generic context hydration owner contract on top of
  admitted resource or context handles. The current Phase E slice supports
  session-scoped pending text-resource hydration for the next turn only.
  Skill-body hydration, connector attachment hydration, and
  application-provided long instructions must all use this generic path rather
  than package-specific tools. Later slices may add object storage, attachment
  ingest, OCR/binary rendering, vector indexing, freshness policy, and richer
  selection policy.

## Acceptance Criteria

- The default shell tool is still not inferred as `pure_read` by command text.
- `resource_read` is the first eligible non-shell `pure_read` tool only after
  this requirement and design are in place.
- Unknown resource refs are rejected without reading filesystem or external
  state.
- Two compatible immutable resource reads can be planned in one parallel batch.
- A resource read after a write fence cannot cross that fence.
- Model-visible tool manifest does not expose scheduling metadata, storage
  paths, hidden body refs, or credential refs.
- Tool result output is bounded by projection budget and includes truncation
  metadata. It must not irreversibly replace user-owned resource text merely
  because the text resembles a credential.
- Full skill bodies are absent from default provider context, capabilities,
  timeline, and context inspection unless they have been explicitly admitted as
  bounded generic hydrated context.
- Generic hydration admission records typed model input kinds and derivation
  refs so compaction and replay can distinguish default skill index metadata
  from hydrated resource content.
- Session-scoped admitted hydration enters the next new turn exactly once.
- Turn-scoped post-submit hydration is refused until provider projection can
  consume it without rewriting transcript, duplicating context, or exposing
  caller-owned handles.

## Reference Alignment

- Reasonix marks tools as read-only only by trusted tool metadata; opaque plugin
  tools default to not read-only unless `readOnlyHint` is declared.
- Reasonix resources are pulled into context through explicit `@resource`
  references rather than by making every plugin protocol a kernel feature.
- Codex tool executors default `supports_parallel_tool_calls()` to false; a
  concrete executor must declare parallel support.
- Codex separates bounded model-visible context fragments from app-server
  inspection APIs; skill instructions are injected through explicit turn
  fragments when selected, not by treating skill bodies as always-on transcript.
- Reasonix MCP resources are listed and read through resource protocol calls,
  then wrapped as explicit context references. Genesis keeps that on-demand
  pattern but requires kernel-owned resource/context refs instead of exposing
  plugin or filesystem paths as authority.

## Relationship To Existing Issues

This requirement governed the retired
`KERNEL-RESOURCE-PURE-READ-PRIMITIVE-20260624` implementation slice and
continues to govern current generic hydration admission, future resource owner
store, and executor-pool slices.
