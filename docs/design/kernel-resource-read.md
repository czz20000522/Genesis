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
context.hydration.accepted
context.hydration.rejected
```

Accepted facts include the generated context handle, source owner, derivation
refs, visible byte cap, content type, truncation policy, model input kind, and
scope. They do not include filesystem paths, connector credentials, package
roots, raw payloads, or full provider prompt text. Rejected facts include a
reason code and safe diagnostic summary; they do not cause fallback prompt
splicing.

The Model Gateway consumes accepted hydration facts while building provider
context. It records context-inspection evidence such as included context handles,
input kinds, source owner, derivation refs, byte counts, and truncation status.
It does not treat hydration as transcript, memory truth, tool permission, or
connector delivery state.

Until `context.hydration.accepted` exists, provider context may include only the
metadata-only skill index, approved memory projection, conversation history,
kernel observations, repair context, and ordinary user text. Full skill bodies,
connector attachment text, and long application instructions remain absent by
default.

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

Current Genesis implements the metadata-only skill index and the first
`resource_read` primitive. It does not yet implement full skill-body hydration.
That absence is intentional. The approved next implementation must first decide
how a user-space skill or application admits a body into a generic resource or
context owner and how the Model Gateway records the resulting hydrated fragment.
Until that exists, full skill bodies remain outside default provider context and
outside model-visible tools.

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
