# Design: Kernel Resource Read

## Boundary

Resource read is a kernel primitive because it governs how admitted content
enters model-visible tool results. It is not a document, mail, Feishu, OCR, or
skill-package feature. Applications and connectors may create or admit resource
refs, but the kernel owns read authorization, bounded projection, and tool
result evidence.

```text
Application / Connector / Future Resource Intake
        |
        v
Resource owner: ref, metadata, grant, body pointer, preview pointer
        |
        v
ToolGateway resource_read
        |
        v
bounded model-visible tool.result
```

## Resource Ref

`resource_ref` is opaque. The model may choose only refs surfaced in context,
timeline/detail projections, connector request context, or future resource
inspection surfaces. The first implementation can use a kernel-issued `res_...`
style ref; later storage can change without changing the tool contract.

The ref is not:

- a path;
- a URL;
- an external message id;
- a credential id;
- a raw payload id;
- a skill package path.

## Reference Model Relationship

Resource read is the first typed tool on top of the Genesis Reference Model. It
does not make all references readable resources.

The resource owner should project public resources through a descriptor:

```text
ReferenceDescriptor
  ref
  ref_kind
  owner
  display_label
  available_operations
  scope
  provenance
  public_metadata
```

For current text resources, `available_operations` can include `read_text` only
when the current actor, session/task scope, grant, resource state, purpose,
budget, and tool surface allow the request. The descriptor is a projection, not
authority truth. `resource_read` must run admission again at call time and may
refuse a previously projected operation when a dynamic reason exists, such as
expired grant, resource unavailable, quarantine, or budget exhaustion.

Do not use `allowed_operations`. It sounds like a final authorization promise.
Use `available_operations` for current projection and reserve
`supported_operations` for owner/debug metadata about what a ref kind can
theoretically support.

The resolver must classify the ref before admitting the operation:

```text
Public Reference      -> may be described and may request typed operations
Runtime Handle        -> only valid for its runtime/control tool
Owner Internal Ref    -> never model-visible and never accepted by resource_read
```

Examples:

- `resource_ref` with `ref_kind=text_resource` can request `read_text`.
- `job_id` can be used by `job_status` or `job_cancel`, but not by
  `resource_read`.
- `event_id`, `tool_call_event_id`, `operation_id`, and `checkpoint_ref` are
  control-plane/runtime handles, not resources.
- `storage_ref`, object keys, database row keys, host paths, raw provider
  payload refs, debug trace paths, connector raw payload ids, and skill package
  paths are owner-internal refs.

Material snapshot tools are deliberately narrow:

```text
source_tree   -> source snapshot/container listing
source_read   -> source file/range text
```

They are not a seed for one kernel tool per content shape. Richer codebase
exploration should go through governed `shell_exec` with `rg`/language tools or
a user-space code-intelligence adapter. Adding another model-visible kernel tool
requires a new owner decision that explains why shell, resource, connector,
skill, or adapter paths are insufficient.

The rejected shape is a universal `ref_read(any_ref)` tool with a large
option-heavy result. That would force callers to infer whether the response was
text, a directory, media, a span, an artifact list, or diagnostic data, and it
would leak owner type boundaries into the model/UI contract.

## Generic Context Hydration

Context hydration is a Model Gateway input-building path backed by resource or
context-owner facts. It is not a tool whose name is tied to a package type.

```text
Skill / Connector / Application source
        |
        v
Resource or context owner admits bounded content handle
        |
        v
Model Gateway selects typed hydrated context fragment
        |
        v
Provider request receives bounded text plus derivation evidence
```

The model-visible skill index stays small and path-free. It can mention that a
skill exists and describe what it helps with. It does not imply that the full
skill body is already present. If later context selection decides that the full
instructions are needed, the selected body must be admitted as a generic
resource/context handle and then rendered as a typed context fragment. The
fragment records source refs or hashes, size/truncation facts, owner, and input
kind evidence for context inspection.

Hydration must not:

- expose `SKILL.md` filesystem paths, package roots, connector payload paths, or
  external message ids as model authority;
- create a model-visible `skill.read` or `read_skill` tool;
- let WebUI, CLI, Feishu listener, or another shell splice raw instruction
  prose into provider context;
- treat hydrated instruction prose as memory truth, credential authority, or a
  tool permission grant;
- store a full rendered prompt as the canonical transcript just because a
  hydrated fragment was used.

The first production implementation should therefore extend the resource/context
owner path before extending skill discovery. A skill loader may produce an
internal body pointer, but the public kernel surface remains a generic context
handle. The model consumes the bounded hydrated text only after the kernel has
selected it for the current provider request or after a generic resource read
has returned terminal-equivalent text.

### Hydration Admission Owner Contract

The stable boundary is a context admission command, not a package-specific
reader. The exact transport can be HTTP, console, connector, or an internal
owner call, but it must normalize into the same owner contract:

```text
context.admit_resource({
  session_id / turn_id / task_ref,
  source_owner,
  resource_ref or context_source_ref,
  intended_input_kind,
  max_visible_bytes,
  freshness,
  derivation_refs,
  sensitivity,
  reason
})
```

The resource/context owner validates that the source is admitted, text-like,
bounded, scoped to the target session or task, and safe for model-visible
projection. It then writes one of two facts:

```text
context.hydration.admitted
context.hydration.refused
```

Admitted facts include the generated context handle, source owner, derivation
refs, visible byte cap, content type, truncation policy, model input kind, and
scope. They do not include filesystem paths, connector credentials, package
roots, raw payloads, full provider prompt text, or provider-visible resource
bodies. Refused facts include `admission_result=refused`, a
`refusal_reason_class`, and safe diagnostic summary; they do not cause fallback
prompt splicing.

The Model Gateway consumes admitted hydration facts while building provider
context. It retrieves bounded text through the resource/context owner, constructs
a provider-only hydrated fragment for that request, and records
context-inspection evidence such as included context handles, input kinds,
source owner, derivation refs, byte counts, and truncation status. It does not
persist the provider-only fragment as transcript, memory truth, tool permission,
connector delivery state, or raw ledger data.

The current implementation intentionally supports only session-scoped pending
hydration: `context.admit_resource` may admit a bounded text resource without a
`turn_id`, and the next submitted turn consumes that fact once as a typed
`hydrated_context` input. Post-submit turn-scoped hydration is not implemented.
Any non-empty `turn_id` is refused with
`refusal_reason_class=scope_violation` until a paused/running provider
projection can consume the fact without rewriting transcript or duplicating
context.

## Tool Contract

Model-visible tool:

```text
resource_read({
  resource_ref: string,
  offset_bytes?: integer,
  limit_bytes?: integer
})
```

`offset_bytes` defaults to zero. `limit_bytes` defaults to the kernel bounded
read limit and cannot exceed the kernel maximum. Negative offsets, zero/negative
limits, unknown refs, and non-text resources are invalid requests and do not
read a body.

Model-visible result:

```text
{
  "status": "completed",
  "executed": true,
  "resource_ref": "...",
  "mime_type": "text/plain",
  "text": "...",
  "offset_bytes": 0,
  "returned_bytes": 123,
  "original_bytes": 456,
  "truncated": true,
  "next_offset_bytes": 123
}
```

The result is a terminal-equivalent read result. Infrastructure failures remain
tool infrastructure failures and must not be disguised as resource content.

## Scheduling

`resource_read` can be trusted as:

```text
effect_class: pure_read
parallel_policy: compatible_locks
resource_footprint.read_scopes: ["resource:<resource_ref>"]
```

Only immutable or snapshot-stable refs qualify. Future mutable refs must use
`state_read` or a stronger owner-specific consistency contract.

`shell_exec` remains serial and effectful. The scheduler must not infer
`pure_read` from shell command text, file path extensions, or model claims.

## First Implementation Slice

The first implementation may use an in-memory/configured immutable text
resource registry on `Kernel.Config`. This registry is intentionally a stepping
stone:

- it proves the model/tool/schema/scheduling contract;
- it does not provide production ingestion, retention, grants, object storage,
  or cross-process durability;
- it must be replaced or backed by a resource owner store before connectors or
  applications depend on durable resources.

The first slice should still be strict:

- refs are unique and non-empty;
- only text resources are accepted;
- content is bounded in tool results;
- scheduling metadata is internal and absent from the model manifest;
- unknown refs fail closed before any body read.

## Current Hydration Boundary

Current Genesis implements the metadata-only skill index, the first
`resource_read` primitive, and generic session-scoped resource hydration
admission for bounded text resources. Full skill bodies, connector attachment
text, and long application instructions stay outside default provider context
unless an owner admits them through the generic resource/context path.

Current limits remain explicit: there is no production object store, attachment
ingest, OCR/binary rendering, vector index, freshness policy, or richer
selection policy in this slice. Turn-scoped post-submit hydration is also
unsupported and must be refused rather than recorded as an optimistic fact.

## Reference Alignment

Reasonix exposes read-only status through trusted tool metadata and keeps MCP
resources as explicit context references. Genesis keeps the same separation but
does not copy Reasonix's arbitrary local file reader into the kernel. Codex
requires concrete executors to opt into parallel tool calls; Genesis similarly
requires `resource_read` to carry a trusted access plan before any executor pool
uses it as a parallel candidate.

## Rejected Directions

- Treating `shell_exec("cat file")` as a pure read is rejected because shell
  commands can hide side effects.
- Reintroducing `skill.read` is rejected because skills are user-space context
  packages; full hydration must use the generic resource path.
- Treating full `SKILL.md` bodies as always-on provider context is rejected
  because it creates unbounded prompt growth, weakens cache stability, and
  bypasses context selection.
- Letting applications pass caller-built prompt strings for long instructions is
  rejected because provider context assembly is a Model Gateway owner path.
- Letting connectors hand raw attachment paths to the model is rejected because
  external refs and local storage paths are not kernel resource refs.
- Building a production object store in the first slice is rejected because the
  current pressure is to prove the primitive and concurrency contract first.
