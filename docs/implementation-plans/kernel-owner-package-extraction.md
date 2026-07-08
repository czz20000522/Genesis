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
- Reference behavior: provider usage/accounting DTOs belong to the model
  gateway boundary rather than the root facade. Genesis equivalent:
  `TestArchitectureBoundaryModelGatewayOwnerHasSubpackageAccountingSurface`
  fails until token usage and context accounting projection live under
  `internal/kernel/modelgateway`.
- Reference behavior: tool scheduling policy is a runtime/tool owner concern
  independent of concrete shell execution. Genesis equivalent:
  `TestArchitectureBoundaryToolRuntimeOwnerHasSubpackageSchedulingSurface`
  fails until `internal/kernel/toolruntime` owns access-plan, scheduling, and
  execution-batch policy.
- Reference behavior: owner-local scheduling tests protect policy semantics
  after extraction. Genesis equivalent: `internal/kernel/toolruntime` tests
  prove only trusted compatible pure reads become parallel, writes and external
  side effects stay serial, same-handle process I/O is split, and untrusted
  scheduling metadata is rejected.
- Reference behavior: tool output/result payloads are runtime/context DTOs, not
  session facade DTOs. Genesis equivalent:
  `TestArchitectureBoundaryToolRuntimeOwnerHasSubpackageResultSurface` fails
  until model-visible tool result DTOs live under `internal/kernel/toolruntime`.
- Reference behavior: approval and sandbox readiness records are authority-plane
  DTOs, not generic root facade DTOs. Genesis equivalent:
  `TestArchitectureBoundaryAuthorityOwnerHasSubpackageApprovalSurface` fails
  until approval/readiness DTOs live under `internal/kernel/authority`.
- Reference behavior: work request and projection records are Work Registry DTOs,
  not session facade DTOs. Genesis equivalent:
  `TestArchitectureBoundaryWorkRegistryOwnerHasSubpackageTypeSurface` fails
  until work DTOs live under `internal/kernel/workregistry`.
- Reference behavior: managed job and observation delivery records are
  job-runtime DTOs, not tool or session facade DTOs. Genesis equivalent:
  `TestArchitectureBoundaryJobRuntimeOwnerHasSubpackageTypeSurface` fails until
  job and observation delivery DTOs live under `internal/kernel/jobruntime`.

## Phase A: Resource Owner Package

- Status: completed in the current implementation.
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
- Remaining beyond this extraction slice:
  - Generic context hydration remains active under
    `KERNEL-CONTEXT-RESOURCE-HYDRATION-20260625`.
  - Root aliases can be removed only after `Config` and external tests depend
    on the resource owner port directly.

## Phase B: Model Gateway Resilience Package

- Status: completed in the current implementation.
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
- Remaining beyond this extraction slice:
  - `ProviderContextProjection`, provider command transport, built-in OpenAI
    adapter, and provider setup remain root-level Model Gateway files until
    later package slices add their own red tests.

## Phase B2: Model Gateway Accounting DTO Package

- Status: completed in the current implementation.
- Deliverable: move token usage, context accounting projection DTOs, and token
  usage copy helper to `internal/kernel/modelgateway`.
- Red lines:
  - Do not move provider context projection, OpenAI usage wire parsing, context
    compaction selection, or ledger event schema in this slice.
  - Do not change provider-backed token semantics; this is package ownership
    only.
- Tests:
  - Model Gateway accounting structure guard.
  - Model Gateway accounting owner unit tests.
  - Existing context accounting and compaction tests continue through root
    compatibility aliases.
- Evidence:
  - `go test ./internal/kernel/modelgateway -count=1`
  - `go test ./internal/kernel -run "TestArchitectureBoundaryModelGatewayOwnerHasSubpackageAccountingSurface|TestModelGatewayAccounts|TestAutoCompaction|TestCompaction" -count=1`
  - `go test ./internal/kernel -count=1`
  - `go test ./... -count=1`
  - `go build ./...`
  - `git diff --check`
- Remaining beyond this extraction slice:
  - Provider usage wire parsing, provider context projection, and compaction
    policy remain later slices with separate red tests.

## Phase C: Tool Runtime Scheduling Package

- Status: completed in the current implementation.
- Deliverable: move tool effect classes, scheduling specs, access plans,
  execution batch planning, and scheduling DTOs to `internal/kernel/toolruntime`.
- Red lines:
  - Do not move `ToolGateway`, tool registry, shell execution, job execution, or
    tool loop guard in this slice.
  - Do not infer shell read/write purity from command text.
  - Do not expose scheduling metadata to the model-visible tool manifest.
  - Do not make process I/O batches concurrently executable merely because they
    share the scheduling package.
- Tests:
  - Tool Runtime owner structure guard.
  - Tool Runtime scheduling owner unit tests.
  - Existing root scheduling, resource-read scheduling, and tool execution
    tests continue through the root compatibility adapter.
- Evidence:
  - `go test ./internal/kernel/toolruntime -count=1`
  - `go test ./internal/kernel -run "TestArchitectureBoundaryToolRuntimeOwnerHasSubpackageSchedulingSurface|TestPlanToolExecutionBatches|TestToolSchedulingMetadataStaysOutOfModelVisibleManifest|TestNonIdempotentEffectClassesDoNotEnterParallelClass|TestExecuteToolBatches" -count=1`
  - `go test ./internal/kernel -count=1`
  - `go test ./... -count=1`
  - `go build ./...`
  - `git diff --check`
- Remaining beyond this extraction slice:
  - ToolGateway, registry, real executor concurrency, loop guard, and tool DTO
    extraction remain later slices with separate red tests.

## Phase C2: Tool Runtime Result DTO Package

- Status: completed in the current implementation.
- Deliverable: move model-visible tool request/error/capability/result DTOs to
  `internal/kernel/toolruntime`.
- Red lines:
  - Do not move `ToolSpec`, provider tool-call correlation DTOs, operation/job
    projection facts, ToolGateway, execution loops, or job owner state.
  - Do not change JSON field names or result taxonomy semantics.
  - Keep root aliases while event schema and HTTP/session projections still
    depend on root package names.
- Tests:
  - Tool Runtime result structure guard.
  - Tool Runtime result JSON-shape unit tests.
  - Existing tool execution and root DTO placement tests stay green through
    aliases.
- Evidence:
  - `go test ./internal/kernel/toolruntime -count=1`
  - `go test ./internal/kernel -run "TestArchitectureBoundaryToolRuntimeOwnerHasSubpackageResultSurface|TestArchitectureBoundaryOwnerDTOsLiveInNamedFiles|TestExecuteToolBatches|TestModelTool|TestTool" -count=1`
  - `go test ./internal/kernel -count=1`
  - `go test ./... -count=1`
  - `go build ./...`
  - `git diff --check`
- Remaining beyond this extraction slice:
  - ToolGateway and execution context still need a narrow owner port before the
    registry can stop receiving `*Kernel`.

## Phase C3: Authority Approval DTO Package

- Status: completed in the current implementation.
- Deliverable: move approval, approval decision, approval policy/effect, and
  sandbox readiness DTOs to `internal/kernel/authority`.
- Red lines:
  - Do not move approval owner behavior, sandbox readiness checks, authority
    admission, HTTP approval routes, or approval execution resume logic.
  - Do not change approval statuses, decision names, readiness statuses, TTL, or
    JSON field names.
  - Keep root aliases while event schema, HTTP, and session projections still
    depend on root package names.
- Tests:
  - Authority owner structure guard.
  - Authority DTO JSON-shape unit tests.
  - Existing approval/sandbox owner tests stay green through aliases.
- Evidence:
  - `go test ./internal/kernel/authority -count=1`
  - `go test ./internal/kernel -run "TestArchitectureBoundaryAuthorityOwnerHasSubpackageApprovalSurface|TestArchitectureBoundaryOwnerDTOsLiveInNamedFiles|TestApproval|TestSandbox" -count=1`
  - `go test ./internal/kernel -count=1`
  - `go test ./... -count=1`
  - `go build ./...`
  - `git diff --check`
- Remaining beyond this extraction slice:
  - Approval owner behavior and sandbox readiness execution still live in root
    until an authority-plane port is designed and covered by red tests.

## Phase C4: Work Registry DTO Package

- Status: completed in the current implementation.
- Deliverable: move work submit/cancel request DTOs, work projection DTO, and
  work status constants to `internal/kernel/workregistry`.
- Red lines:
  - Do not move work submission/cancellation behavior, event append/replay,
    idempotency lookup, validation, or merge conflict detection.
  - Do not change status strings, JSON field names, or HTTP/session projection
    behavior.
  - Keep root aliases while event schema, HTTP, and session projections still
    depend on root package names.
- Tests:
  - Work Registry owner structure guard.
  - Work Registry DTO JSON-shape unit tests.
  - Existing work owner tests stay green through aliases.
- Evidence:
  - `go test ./internal/kernel/workregistry -count=1`
  - `go test ./internal/kernel -run "TestArchitectureBoundaryWorkRegistryOwnerHasSubpackageTypeSurface|TestArchitectureBoundaryOwnerDTOsLiveInNamedFiles|TestWork|TestSubmitWork|TestCancelWork|TestHTTPWork" -count=1`
  - `go test ./internal/kernel -count=1`
  - `go test ./... -count=1`
  - `go build ./...`
  - `git diff --check`
- Remaining beyond this extraction slice:
  - Work behavior still lives in root until event append/replay and validation
    ports are introduced with dedicated red tests.

## Phase C5: Job Runtime DTO Package

- Status: completed in the current implementation.
- Deliverable: move managed job projection DTO and kernel observation delivery
  DTO to `internal/kernel/jobruntime`.
- Red lines:
  - Do not move job lifecycle, managed executor, progress capture, cancellation,
    terminal observation delivery, or projection redaction behavior.
  - Do not change job event names, status strings, JSON field names, or
    provider-visible observation context.
  - Keep root aliases while event schema, tool results, HTTP, and session
    projections still depend on root package names.
- Tests:
  - Job Runtime owner structure guard.
  - Job Runtime DTO JSON-shape unit tests.
  - Existing job/observation/interrupt tests stay green through aliases.
- Evidence:
  - `go test ./internal/kernel/jobruntime -count=1`
  - `go test ./internal/kernel -run "TestArchitectureBoundaryJobRuntimeOwnerHasSubpackageTypeSurface|TestArchitectureBoundaryOwnerDTOsLiveInNamedFiles|TestJob|TestManagedJob|TestKernelObservation|TestInterrupt" -count=1`
  - `go test ./internal/kernel -count=1`
  - `go test ./... -count=1`
  - `go build ./...`
  - `git diff --check`
- Remaining beyond this extraction slice:
  - Job lifecycle and observation delivery behavior still live in root until
    executor and replay ports are introduced with dedicated red tests.

## Later Phases

- Provider gateway extraction beyond resilience classification.
- Tool runtime extraction beyond scheduling policy.
- Projection extraction.
- Resource hydration owner extension.

Each later phase needs its own red test and should not reuse this plan as a
blanket approval for moving `Kernel`, `Config`, `SubmitTurn`, or ledger event
schema.
