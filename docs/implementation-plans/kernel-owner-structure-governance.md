# Implementation Plan: Kernel Owner Structure Governance

## Requirement And Design

- Requirement: `docs/requirements/kernel-owner-structure-governance.md`
- Design: `docs/design/kernel-owner-structure-governance.md`

## Reference Scan

- Codex references inspected:
  - `D:\software\JetBrains\python_workspace\codex-main\codex-rs\core\src\tools\registry.rs`
  - `D:\software\JetBrains\python_workspace\codex-main\codex-rs\core\src\tools\handlers\unified_exec\exec_command.rs`
  - `D:\software\JetBrains\python_workspace\codex-main\codex-rs\core\tests\suite\compact.rs`
  - `D:\software\JetBrains\python_workspace\codex-main\codex-rs\core\tests\suite\approvals.rs`
- Reasonix references inspected:
  - `D:\software\JetBrains\python_workspace\reasonix\docs\SPEC.md`
  - `D:\software\JetBrains\python_workspace\reasonix\internal\tool\tool.go`
  - `D:\software\JetBrains\python_workspace\reasonix\internal\control\controller.go`
- Alignment: Keep tool registry, tool execution, permission, session/control, and projection responsibilities separated by owner-visible structures.
- Intentional differences: Genesis keeps a small single-package Go kernel for the current phase rather than copying Codex's Rust crate layout or Reasonix's app-oriented controller shape.
- Drift risks or follow-up issues: no owner-structure issues are active in
  `docs/operations/kernel-issues.md` at this checkpoint.

## Reference Behavior Red Tests

- Codex typed tool runtime boundaries become Genesis architecture tests proving
  the tool registry cannot bind registered tools to broad `*Kernel` authority.
- Codex and Reasonix topic-oriented behavior suites become Genesis governance
  tests and incremental test moves that keep owner contracts out of one central
  test surface.
- Reasonix dependency-direction and controller/tool separation become Genesis
  tests proving active kernel issues retain reference alignment and central
  kernel files do not absorb new owner replay branches.
- The accepted intentional difference is that Genesis can keep a single Go
  package during this phase; the tests check owner-visible structure and narrow
  authority, not crate/package parity.

## Phase A

- Deliverable: Requirement, design, implementation plan, BDD examples, active issues, and architecture guards.
- Red lines: no runtime behavior change; no application-specific owner; no remote Genesis authority lookup.
- Tests: architecture guard focused on owner structure.
- Evidence: focused architecture test output and `git diff --check`.
- Historical phase note: initial guards covered only obvious central-file drift.
- Closing gate:
  - Requirement/design/issue/BDD items checked: owner replay, DTO placement, transport delegation, tool executor authority, document lifecycle.
  - Drift fixed before commit: none expected.
  - Drift record: no owner-structure gap is active in `docs/operations/kernel-issues.md` at this checkpoint.

## Phase B

- Status: completed in the current implementation.
- Deliverable: Move session replay aggregation behind projection helpers and split global DTO definitions by owner/audience.
- Red lines: no runtime API change; no schema rename; no compatibility alias; no behavior rewrite.
- Tests: existing session, memory, work, tool, and architecture tests.
- Evidence: focused `go test ./internal/kernel`, full `go test ./...`, `go build ./...`, `git diff --check`.
- Historical phase note: tool executor authority and HTTP file split were later
  covered by focused owner guards.

## Phase C

- Status: completed in the current implementation.
- Deliverable: Split HTTP transport handlers by surface while preserving route behavior.
- Red lines: transport may auth/decode/encode/delegate only; owner replay and policy stay outside transport.
- Tests: focused HTTP tests, architecture transport guard, full verification.
- Evidence: same as Phase B.
- Historical phase note: this phase intentionally added no new runtime API.

## Phase D

- Status: completed in the current implementation.
- Deliverable: Replace tool registration `Prepare func(*Kernel, ...)` with a narrow tool invocation context or owner-specific executor interface.
- Red lines: no tool gets broad kernel authority by default; model-visible tool schema remains unchanged.
- Tests: tool registry, tool gateway, model tool loop, shell/job control, architecture tests.
- Evidence: focused and full Go verification.
- Remaining beyond this governance slice: a future external tool plugin
  boundary needs a separate requirement if it is introduced.

## Retirement Criteria

Related issues can leave the active ledger only after the implemented phase passes its architecture guard, focused owner tests, full Go tests, build, diff check, and drift check against this plan. At this checkpoint there are no active owner-structure issue headings in `docs/operations/kernel-issues.md`.
