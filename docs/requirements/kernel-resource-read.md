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
redaction policy, truncation metadata, and deterministic tool-result ordering.

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

## Non-Goals

- No arbitrary filesystem reader is introduced by this requirement.
- No `skill.read`, Feishu attachment reader, mail attachment reader, document
  reader, OCR reader, or application-specific resource API enters the kernel.
- No connector or shell may read resource bodies directly and then fabricate
  kernel tool results.
- No production object store, attachment ingestion, binary rendering, vector
  index, or retention policy is required in the first implementation slice.

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
- Tool result output is bounded, redacted, and includes truncation metadata.

## Reference Alignment

- Reasonix marks tools as read-only only by trusted tool metadata; opaque plugin
  tools default to not read-only unless `readOnlyHint` is declared.
- Reasonix resources are pulled into context through explicit `@resource`
  references rather than by making every plugin protocol a kernel feature.
- Codex tool executors default `supports_parallel_tool_calls()` to false; a
  concrete executor must declare parallel support.

## Relationship To Existing Issues

This requirement governs `KERNEL-RESOURCE-PURE-READ-PRIMITIVE-20260624` and the
future implementation slice that can unblock the deferred executor-pool part of
`KERNEL-TOOL-SCHEDULING-CONCURRENCY-20260624`.
