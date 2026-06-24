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
- Letting connectors hand raw attachment paths to the model is rejected because
  external refs and local storage paths are not kernel resource refs.
- Building a production object store in the first slice is rejected because the
  current pressure is to prove the primitive and concurrency contract first.
