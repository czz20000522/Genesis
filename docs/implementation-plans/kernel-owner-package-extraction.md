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
- Continue with provider resilience before the full provider gateway because
  retry classification, visible-final repair policy, and attempt projection are
  a small Model Gateway slice with direct Codex/Reasonix analogues.

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
- Reference behavior: provider retry/recovery policy lives near provider
  boundary code, not in the root turn facade. Genesis equivalent:
  `TestArchitectureBoundaryModelGatewayOwnerHasSubpackageResilienceSurface`
  fails until `internal/kernel/modelgateway` owns resilience types and functions.
- Reference behavior: retry classification remains behavior-tested at the owner
  package. Genesis equivalent: `internal/kernel/modelgateway` tests cover
  retryable status classification, fail-fast auth, redacted provider attempt
  projection, capped Retry-After handling, and visible-final repair detection.

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

## Phase B: Model Gateway Resilience Package

- Deliverable: move provider retry/repair classification, classified error
  types, retry-delay policy, and provider-attempt projection type to
  `internal/kernel/modelgateway`.
- Red lines:
  - Do not move `SubmitTurn`, `ProviderContextProjection`, `provider_command`,
    OpenAI wire adapters, `Config`, or ledger event schema in this slice.
  - Do not make provider adapters own retry loops; Model Gateway policy remains
    the owner of retry/repair semantics.
  - Do not duplicate redaction logic inside the modelgateway package; inject the
    kernel projection redactor at the root adaptation point.
- Tests:
  - Model Gateway owner structure guard.
  - Model Gateway resilience owner unit tests.
  - Existing provider retry/final repair tests.
- Evidence:
  - `go test ./internal/kernel/modelgateway -count=1`
  - `go test ./internal/kernel -run "TestArchitectureBoundaryModelGatewayOwnerHasSubpackageResilienceSurface|TestOpenAICompatibleProviderRetriesTransientStatusBeforeTurnFailure|TestSubmitTurnRepairsEmptyVisibleFinalBeforeCompleting|TestProviderCommandAdapterShapeFailureDoesNotRetry" -count=1`
  - `go test ./internal/kernel -count=1`
  - `go test ./... -count=1`
  - `go build ./...`
  - `git diff --check`
- Still short of production:
  - `ProviderContextProjection`, provider command transport, built-in OpenAI
    adapter, and provider setup remain root-level Model Gateway files until
    later package slices add their own red tests.

## Later Phases

- Provider gateway extraction beyond resilience classification.
- Tool runtime extraction.
- Projection extraction.
- Resource hydration owner extension.

Each later phase needs its own red test and should not reuse this plan as a
blanket approval for moving `Kernel`, `Config`, `SubmitTurn`, or ledger event
schema.
