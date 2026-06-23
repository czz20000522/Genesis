# Requirement: Kernel Owner Structure Governance

- **Status:** approved
- **Owner:** Architecture Governance
- **Scope:** Code ownership, projection composition, DTO placement, transport delegation, document lifecycle.

## Background

Genesis Kernel is growing from a spike into a durable control plane. If new capabilities keep adding replay logic, DTOs, transport handlers, and executor bindings to central files, the Go kernel will recreate the same owner drift that the rewrite is meant to avoid.

## Production Target

The kernel keeps authority boundaries visible in code, tests, and documents. Each owner keeps its own request, event, projection, replay, and policy helpers close to the owner. Cross-owner surfaces compose owner results, but do not reimplement owner replay rules. Transport files authenticate, decode, encode, and delegate. Tool registrations bind to the narrow execution authority they need, not to the whole kernel object.

Architecture, feature, directory, and document review are one periodic governance activity. Review must retire or condense obsolete implementation plans, old issue evidence, and stale architecture notes instead of preserving every historical document as an active contract.

## Users And Roles

Developers see owner boundaries in filenames, narrow helpers, and architecture tests. Reviewers can reject a patch when it adds a new owner to a central aggregator without a documented reason. Applications and shells see no new runtime API; this requirement governs internal maintainability and authority shape.

The LLM does not see this governance surface. It affects only kernel implementation, tests, and documentation.

## Core Semantics

- `Kernel` may coordinate owner commands, but it must not become the replay home for every owner fact.
- Session projection composes turn, tool, job, work, memory, and raw event projections from owner-owned helpers.
- Public DTOs are grouped by owner or projection audience instead of living in one global DTO warehouse.
- HTTP transport stays a protocol adapter. It must not decide owner state transitions, replay owner facts, or duplicate owner policy.
- Tool registration must move toward narrow invocation authority. A registered tool should not receive `*Kernel` unless a specific owner decision records a temporary exception.
- Document lifetime is part of architecture governance. Requirements stay few and stable; design docs live with boundaries; implementation plans are phase-local; completed issues leave the active ledger; stale docs are deleted or condensed.

## Non-Goals

- This requirement does not force every owner into a separate Go package during the current phase.
- This requirement does not require tiny files or arbitrary line-count caps.
- This requirement does not add application-specific owners.
- This requirement does not require migrating old local development artifacts.

## Phased Delivery

Phase A records this requirement, the design, implementation plan, active issues, BDD examples, and executable structure guards.

Phase B moves session replay aggregation out of `kernel.go` and splits the global DTO file into owner/audience files without changing runtime behavior.

Phase C splits HTTP transport by surface so route handlers remain thin delegates.

Phase D narrows `ToolRegistry` executor binding so registered tools receive only the invocation authority they need.

## Acceptance Criteria

- Adding a new owner replay branch to `kernel.go` fails an architecture guard unless the code delegates through an owner projection helper.
- Core DTOs for turn, tool/job, work, memory, event, config, and inspection are not defined in one global file.
- HTTP handler files contain transport logic only: auth, content-type checks, route parsing, decode, owner API call, error mapping, and encode.
- A registered tool cannot grow by reaching into unrelated kernel owner fields through `*Kernel` without an active issue or design exception.
- Periodic governance review includes architecture, feature behavior, directory structure, and document lifetime. Obsolete documents are deleted or condensed rather than kept as active contracts.

## Relationship To Existing Issues

This requirement governs `KERNEL-OWNER-SESSION-PROJECTION-20260623`, `KERNEL-OWNER-DTO-FILES-20260623`, `KERNEL-OWNER-HTTP-TRANSPORT-20260623`, and `KERNEL-OWNER-TOOL-CONTEXT-20260623`.
