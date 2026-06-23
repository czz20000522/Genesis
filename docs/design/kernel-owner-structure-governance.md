# Design: Kernel Owner Structure Governance

## Requirement

Governs `docs/requirements/kernel-owner-structure-governance.md`.

## Boundary And Owner

Architecture Governance owns structure rules and guards. Runtime owners still own their domain semantics:

- Interface Kernel owns turns and session entry.
- Tool Runtime owns tool calls, operations, jobs, and tool results.
- Work Registry owns work records.
- Accumulation owns memory candidates, review decisions, and recall.
- Readiness and Inspection own safe projections for humans and operators.

Governance does not own those facts. It only prevents central files and adapters from becoming hidden owners.

## Data Flow

```text
ledger events
  -> owner replay helpers
  -> session projection composer
  -> HTTP/CLI/UI/application transport projection
```

Transport flow stays:

```text
request -> auth/content-type -> decode/parse route -> owner API -> error map -> JSON response
```

Transport cannot replay ledger facts, merge owner state, or decide owner policy.

## Protocol

No runtime protocol is added. The protocol is an executable governance contract:

- architecture tests scan central files for owner replay drift;
- architecture tests keep DTO ownership visible through file placement;
- active issues must cite this requirement and design;
- implementation plans record the local Codex/Reasonix reference scan;
- closing gates compare code, docs, issues, and BDD examples before commit.

## Failure Semantics

A governance failure is a test failure, not a runtime failure. The fix is to move logic to the owner helper, split the DTO into the owner file, shrink the transport handler, update the governing design, or record a temporary exception with an active issue.

## Permission And Authority

This design does not grant runtime authority. It constrains code authority: central coordinators may call owner APIs, but they do not mint owner facts or bypass owner validation.

Tool executor authority should be narrow. The long-term shape is a tool invocation context exposing only the owner capabilities needed to validate, authorize, execute, append evidence, and project the result for that tool.

## Recovery And Observability

Recovery still comes from the ledger and owner replay. Structure guards ensure the replay code remains close to the owner that understands the event. Periodic governance review checks architecture, feature behavior, directory structure, and document lifetime together. Documents that no longer represent active requirements, designs, or current issue gaps are deleted, condensed, or moved to retirement evidence.

## Reference Alignment

Codex keeps typed runtime contracts around tool execution: `codex-rs/core/src/tools/registry.rs` defines `CoreToolRuntime` over `ToolInvocation`, and `codex-rs/core/src/tools/handlers/unified_exec/exec_command.rs` implements an exec handler through that tool runtime instead of giving every tool arbitrary session authority. Codex tests also exercise compaction, approval, exec, and event behavior through core events rather than UI-only state.

Reasonix records an acyclic dependency direction in `docs/SPEC.md`: `cli -> {agent, plugin, config} -> {tool, provider}`. Its `internal/tool/tool.go` keeps a per-run registry and its `internal/permission` design gates each tool call independently of CLI. Its `control.Controller` is a frontend-agnostic control layer, which is useful as a reference but also shows the risk Genesis should avoid as the kernel grows.

Genesis aligns with the registry, owner, and projection ideas but intentionally does not copy either project's application-specific package layout.

## Rejected Alternatives

- Keep `kernel.go` as the single replay switch for every owner. Rejected because it makes new owner growth too easy and hides authority boundaries.
- Keep all DTOs in `types.go`. Rejected because file-level structure is part of the review surface in a fast AI-assisted codebase.
- Add line-count caps. Rejected because readability and owner placement matter more than arbitrary size.
- Preserve obsolete process documents indefinitely. Rejected because stale documents become false architecture.
