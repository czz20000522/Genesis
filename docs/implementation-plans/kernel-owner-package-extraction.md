# Implementation Plan: Kernel Owner Package Extraction

- **Requirement:** `docs/requirements/kernel-owner-structure-governance.md`
- **Design:** `docs/design/kernel-owner-structure-governance.md`
- **Owner:** Architecture Governance

## Reference Scan

Codex:

- `codex-rs/core/src/tools/registry.rs` and handler modules keep runtime-owned
  tool execution behind typed ports rather than letting every tool reach the
  whole session object.
- Codex core tests exercise behavior through the owning runtime surface after
  files have been split by topic.

Reasonix:

- `internal/tool`, `internal/provider`, and `internal/permission` keep owner
  concepts in focused packages while `internal/agent` composes them.
- Reasonix resource-like and read-only tool behavior stays close to the owner
  that understands the data, with the agent consuming the owner surface.

Genesis translation:

- Move one bounded owner at a time out of the root `internal/kernel` package.
- Keep the public kernel facade stable through aliases or narrow ports while the
  owner package becomes the implementation home.
- Start with `resource` because it has a small DTO/registry surface and already
  has behavior tests around bounded read, redaction, pure-read scheduling, and
  provider-visible projection.

## Reference Behavior Red Tests

- Reference behavior: owner packages make boundaries visible in code structure.
  Genesis equivalent: `TestArchitectureBoundaryResourceOwnerHasSubpackageTypes`
  fails until `Descriptor`, `ModelReadResult`, `Registry`, `ReadRequest`, and
  the resource store type live under `internal/kernel/resource`.
- Reference behavior: owner-local tests protect behavior after a package split.
  Genesis equivalent: `TestRegistryKeepsRawTextWhileReadReturnsRedactedProjection`
  proves raw resource owner text remains stored while the read projection uses
  the injected redaction policy.
- Accepted intentional difference: the root `kernel` package keeps type aliases
  for `ResourceDescriptor` and `ModelResourceReadResult` during this slice so
  `Config` and existing callers do not change while implementation ownership
  moves.

## Phase A: Resource Owner Package

- Deliverable: move resource descriptor/result types and registry/read logic to
  `internal/kernel/resource`.
- Red lines:
  - Do not move `Config`, `SubmitTurn`, or the generic tool loop.
  - Do not introduce skill-specific tools or hydration behavior.
  - Do not duplicate secret detection logic inside the resource package.
- Tests:
  - Resource owner structure guard.
  - Resource owner raw-truth/redacted-read unit test.
  - Existing resource_read, tool scheduling, and timeline resource tests.
- Evidence:
  - `go test ./internal/kernel/resource ./internal/kernel -run "...resource..." -count=1`
  - `go test ./internal/kernel -count=1`
  - `go test ./... -count=1`
  - `go build ./...`
  - `git diff --check`
- Still short of production:
  - Generic context hydration remains active under
    `KERNEL-CONTEXT-RESOURCE-HYDRATION-20260625`.
  - Root aliases can be removed only after `Config` and external tests depend
    on the resource owner port directly.

## Later Phases

- Provider gateway extraction.
- Tool runtime extraction.
- Projection extraction.
- Resource hydration owner extension.

Each later phase needs its own red test and should not reuse this plan as a
blanket approval for moving `Kernel`, `Config`, `SubmitTurn`, or ledger event
schema.
