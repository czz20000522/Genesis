# Genesis Kernel Agent Contract

Genesis Kernel is a small authority runtime for LLM execution — the core of a local-first personal AI runtime. The full project vision, phased feature list, and success criteria are in [`docs/project-brief.md`](docs/project-brief.md).

Keep the kernel generic. Do not add application-specific owners such as Feishu, email, calendar, calculator, document, OCR, medical, or insurance logic unless the work is reduced to a generic kernel primitive.

## Development Process Gate

Non-trivial kernel capability work must follow this order:

1. Requirement: define what the production capability must be, why it is needed, production semantics, roles, non-goals, phased delivery, acceptance criteria, and related issues.
2. Design: define owner, boundary, data flow, protocol, failure semantics, permission, recovery, and observability.
3. Reference Scan: before implementation planning or code, inspect Codex and Reasonix for comparable control-plane behavior, record what was learned, and state whether Genesis aligns, intentionally differs, or rejects a drift risk.
4. Implementation Plan: define Phase A/B/C delivery slices, red lines, tests, evidence, what remains short of production, and the reference scan summary.
5. Issue: track only the current gap between approved requirements/designs and implementation.
6. Implementation: change code only after the relevant requirement, design, and reference scan exist, except for obvious bugs or test gaps.
7. Closing Gate: before each commit, compare the implemented slice against the governing requirement, design, plan, issue, and BDD feature. Fix in-scope drift immediately; record out-of-scope drift as an active issue before committing.

Core principle:

> Requirements must be production-grade. Implementation can be experimental or partial inside a phase, but every phase must state what still remains short of the production requirement.

Requirements should be few and stable. Implementation plans are phase-local and may be condensed after the phase closes. Issues record only current gaps and leave the active ledger when ready for acceptance or retired.

Issues must cite an approved requirement and design unless the issue is an obvious bug or test gap. If an issue uses that exception, say so explicitly in the issue.

Implementation plans and non-trivial issue updates must not jump straight from Genesis-local reasoning to code. Look for comparable behavior in `D:\software\JetBrains\python_workspace\codex-main` and `D:\software\JetBrains\python_workspace\reasonix` first. The goal is not to copy them, but to catch missing state, failure, permission, recovery, and projection semantics before coding. Reference scans must inspect concrete implementation paths, not just similarly named concepts: identify the entrypoint, owner state transition, persisted event/record, projection, model-visible fields, error/retry semantics, and tests. If the scan only says "Codex has sessions" or "Reasonix has a controller", it is not sufficient.

Each implementation slice must end with a requirement-by-requirement drift check. Passing tests is not enough if docs, issues, or retirement evidence still describe a temporary shortcut as the current contract.

Genesis kernel authority is local. Do not search GitHub, remotes, pull requests, releases, or online repositories for Genesis project truth unless the user explicitly asks for external publishing history. This project is developed locally; any public or remote Genesis repository is stale or unrelated to the active kernel contract.

Genesis has no production users, deployed uptime obligation, architecture migration debt, or historical data cleanup debt. Development artifacts, local ledgers, generated JSONL, fixtures, and old experiments do not justify compatibility shims, migration readers, fallback loaders, old API aliases, or data cleanup paths. When old development state conflicts with the current contract, delete or regenerate it instead of preserving it.

Architecture, feature, directory, and document reviews are one governance activity. They do not need to run after every commit, but periodic review must check all four together and delete or condense obsolete documents instead of letting requirements, plans, and acceptance records grow without bound.

## Boundary Rules

- The event ledger is kernel truth. Applications, shells, provider commands, and skills do not mint ledger facts.
- Runtime transport chunks are not kernel truth by default. Token deltas, stdout chunks, progress frames, and heartbeats stay in realtime transport or debug trace unless an owner reduces them to transcript, durable fact, audit, or failure evidence.
- Audit is not an info log. Persist only authority changes, risk decisions, credential use, control-plane writes, dangerous-operation decisions, security failures, and recovery-relevant failures.
- Provider context is assembled by the Model Gateway, not by shells or applications.
- Tool execution goes through ToolRegistry and ToolGateway.
- Model-visible schemas expose semantic fields only. Kernel ids, credentials, permission profiles, sandbox profiles, checkpoints, and audit refs are kernel-owned.
- Skill packages are user-space assets. Skill metadata may be indexed; skill bodies are not kernel APIs.
- HTTP route names and runtime policy names must not use numbered version identifiers as active contracts.

## Verification

Before claiming completion, run the smallest verification that proves the change. For document-only process changes, at least run `git diff --check` and the architecture boundary test when available. For runtime changes, run focused tests, then `go test ./... -count=1` and `go build ./...` unless the change is explicitly outside Go runtime behavior.

---

# Project Conventions

This section documents the actual conventions observed across the Genesis codebase.
Every commit and code review must respect these rules.

## Commit Message Convention

Every non-trivial commit must follow this structured format:

```
<ImperativeVerb> <capitalised noun phrase>

<Body paragraph explaining why and what, one or two sentences.>

Constraint: <architectural invariant being enforced>
Constraint: <second invariant, if applicable>
Rejected: <alternative considered> | <reason it was rejected>
Confidence: high|medium|low
Scope-risk: narrow|moderate|broad
Directive: <actionable rule for future work>
Tested: go test ./internal/kernel -run "<test pattern>" -count=1
Tested: go build ./...
Tested: git diff --check
Not-tested: <what was deliberately skipped and why>
Related: #<issue number> (only if closing a tracked issue)
```

**Verb semantics** — choose the verb that signals the *kind* of change:

| Verb | When to use |
|------|-------------|
| **Keep** | Enforce a boundary, prevent scope creep, preserve ownership. "Keep X inside Y" |
| **Define** | Introduce a new concept, type, or contract |
| **Retire** | Close an issue, remove dead code, mark something superseded |
| **Record** | Evidence-only / documentation-only change (no runtime code) |
| **Constrain** | Narrow an existing surface to prevent misuse |
| **Expose** | Make an existing capability visible through a new surface |
| **Move** | Relocate code preserving semantics (not "move" for refactoring that changes behavior) |
| **Fail** | Add fail-closed behavior — refuse unsafe states |
| **Gate** | Add a conditional guard around an existing path |
| **Close** | Fully resolve a gap with implementation |
| **Probe** | Add inspection / readiness-check capability |
| **Fix** | Bug fix (use sparingly; prefer a more specific verb) |
| **Bind** | Wire a component to its configuration or adapter |

**Rules:**

- Subject line: no trailing period, no issue numbers.
- Body: explain *why*, not just *what*. The code already shows *what*.
- Tags in fixed order as shown above.
- `Tested:` lines are exact shell commands — the first line must always be `Tested: git diff --check`.
- `Confidence:` and `Scope-risk:` are required on every non-trivial commit.
- `Rejected:` is optional but strongly encouraged for architectural decisions.

## Go Code Conventions

### Package and Directory Structure

```
internal/
  kernel/                  # Flat package — all kernel types and logic coexist in package kernel
    kernel.go              # Kernel struct + New()
    config_types.go        # Config + policy structs with json tags
    event_types.go         # Event types, StoredEvent, EventData
    tool_types.go          # ToolSpec, ModelToolCall, type aliases from toolruntime
    turn_types.go          # TurnRequest, TurnResponse, TurnProjection
    work_types.go          # Work types
    memory_types.go        # MemoryCandidate, recall types
    http*.go               # HTTP handler files, split by domain
    doc.go                 # Package-level godoc — required
    <domain>_types.go      # Types sorted by domain
    <domain>.go            # Logic
    <domain>_test.go       # Tests alongside source
  authority/               # package authority — cross-cutting authority domain
  modelgateway/            # package modelgateway — provider resilience, accounting
  toolruntime/             # package toolruntime — scheduling, capabilities
  jobruntime/              # package jobruntime
  workregistry/            # package workregistry
  resource/                # package resource — resource descriptors, source snapshots
  testsupport/             # package testsupport — shared test utilities
applications/              # User-space application runtimes (separate packages)
cmd/                       # Binary entry points, each package main
desktop/                   # Separate go.mod for Wails desktop app
```

**Rules:**

- `internal/kernel/` stays flat. Only cross-cutting domain abstractions (authority, modelgateway, toolruntime, etc.) get their own subpackage.
- Subpackage types are re-exported in the kernel via `type X = subpackage.X` aliases.
- Directory names are singular (`resource/` not `resources/`).
- Each `cmd/` binary has its own `package main` with minimal entry logic.
- `.gitattributes` enforces LF line endings for `.go`, `.mod`, `.md`, `.json`, `.yaml`, `.yml`.

### Imports

Three groups, separated by blank lines, in this order:

```go
import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "path/filepath"
    "sort"
    "strings"
    "sync"
    "time"

    "modernc.org/sqlite" // External dependency (only dependency outside stdlib)

    "genesis/internal/kernel/modelgateway"
    "genesis/internal/kernel/resource"
    "genesis/internal/kernel/toolruntime"
)
```

### Error Handling

**Pattern: Sentinel errors + wrapping**

```go
var ErrLedgerUnavailable = errors.New("ledger unavailable")

func wrapLedgerUnavailable(err error) error {
    if errors.Is(err, ErrLedgerUnavailable) {
        return err
    }
    return fmt.Errorf("%w: %w", ErrLedgerUnavailable, err)
}
```

**Rules:**

- Use `var Err* = errors.New(...)` for expected failure modes — no custom error types.
- Wrap with `fmt.Errorf("%w: ...")`, not `fmt.Errorf("... : %w")`.
- Classify via `errors.Is()` — not type assertions on custom error types.
- Never panic. Always return errors.
- No third-party error libraries.

### Configuration

```go
type Config struct {
    Provider    Provider
    LedgerPath  string
    Clock       func() time.Time
    // ...
}
```

**Rules:**

- Single `Config` struct passed to `New()`. No functional options.
- Defaults applied in the constructor via `normalized*` helper functions (e.g. `normalizedToolPolicy`, `normalizedBudgetPolicy`).
- Policy structs have explicit `json:"..."` tags.
- Config is populated by `cmd/` layer from file, not by the kernel itself.

### Interface Design

- Small interfaces (2-4 methods), stored by the consumer:

```go
type Provider interface {
    Name() string
    Ready() ProviderStatus
    Complete(ctx context.Context, req ModelRequest) (ModelResponse, error)
}

type Ledger interface {
    Append(event StoredEvent) error
    Load() ([]StoredEvent, error)
    Ready() ReadyCheck
    Path() string
}
```

### Concurrency

- Per-domain fine-grained `sync.Mutex` — never a single global lock:

```go
type Kernel struct {
    turnMu         sync.Mutex
    activeTurnMu   sync.Mutex
    operationMu    sync.Mutex
    jobMu          sync.Mutex
    approvalMu     sync.Mutex
    memoryReviewMu sync.Mutex
    workMu         sync.Mutex
}
```

- Lock-then-defer-unlock: `mu.Lock()` / `defer mu.Unlock()`.
- `sync.Mutex` only — no `sync.RWMutex` unless a clear read-hot path exists.
- The kernel is single-threaded per request; long-running work goes to `ManagedJobExecutor`.

### HTTP Handlers

- Single `Handler(k *Kernel) http.Handler` factory with a large `switch` on method + path.
- Authorization via `subtle.ConstantTimeCompare` against `Bearer` token.
- `decodeRequest(w, r, target)` with `DisallowUnknownFields` + `MaxBytesReader`.
- Consistent error envelope:

```go
type errorEnvelope struct {
    Error errorBody `json:"error"`
}
type errorBody struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}
```

- No third-party router.
- Per-domain handler functions live in `http_<domain>.go` files, with signature `func handleXxx(w http.ResponseWriter, r *http.Request, k *Kernel)`.

### Naming

| Category | Convention | Examples |
|----------|-----------|---------|
| Types | PascalCase, descriptive noun | `Kernel`, `Config`, `TurnRequest`, `StoredEvent` |
| Interfaces | Single method → `-er` suffix; multi-method → noun | `Provider`, `Ledger` |
| Exported functions | PascalCase | `New()`, `Ready()`, `SubmitTurn()` |
| Unexported functions | camelCase | `newBudgetLease()`, `wrapLedgerUnavailable()` |
| Private constructors | `new*` prefix | `newBudgetLease()`, `newProviderStatusError()` |
| Exported struct fields | PascalCase | `SessionID`, `ToolCallID` |
| Unexported struct fields | camelCase | `turnMu`, `activeTurns` |
| Abbreviations | All-caps: `ID`, `URL`, `JSON`, `HTTP`, `API` | `SessionID`, `baseURL` |
| Boolean fields | No redundant `Is`/`Has` prefix | `Truncated`, `Executed` (not `IsTruncated`) |
| DTOs | `Request`/`Response`/`Projection` suffix | `TurnRequest`, `ReadyResponse` |
| Sentinel errors | `ErrCamelCase` | `ErrLedgerCorrupt`, `ErrWorkNotFound` |

### Input Sanitization

- `strings.TrimSpace()` on all string input fields at entry points.
- Defensive copies via `clone*` helper functions for shared data.

### Subpackage Type Aliases

Types defined in subpackages (authority, toolruntime, jobruntime, resource) are re-exported in `internal/kernel/*_types.go` via type alias:

```go
type ApprovalListResponse = authority.ApprovalListResponse
type ToolCapabilityProjection = toolruntime.CapabilityProjection
type JobProjection = jobruntime.JobProjection
```

## Test Conventions

### File Layout

- Test files live alongside source files as `*_test.go`.
- Test helpers live in a single `*_test_helpers_test.go` file.
- Tests are same-package (white-box, `package kernel` not `package kernel_test`).

### Naming

```
TestFunctionName_ScenarioOrOutcome
```

Examples:

```go
func TestHTTPShellExecAndSessionProjection(t *testing.T) {
func TestKernelLimitClassificationCoversActiveBudgetGuardAndProjectionCaps(t *testing.T) {
func TestHTTPMaterialIntakeLocalPathReturnsSourceSnapshot(t *testing.T) {
```

### Test Helpers

- Every helper must call `t.Helper()` as the first line.
- Common helpers live in `testsupport` package (e.g. `testsupport.ProjectTempDir`).
- Kernel test factories: `newTestKernel(t, ledgerPath)` with `FakeProvider{}` and frozen clock.
- HTTP client helpers: `postJSONWithAuth(url, body)`, `getWithAuth(url)`.
- Assertion helpers: `assertErrorCode`, `assertJSONUsage`, `assertJSONNumber`, `assertLimitClass`.

### Test Doubles

- Fake providers as structs implementing `Provider` (e.g. `FakeProvider`, `singleToolCallProvider`, `toolFeedbackProvider`).
- Fake ledgers (e.g. `staticLedger`, `failOnOperationLedger`, `reviewRaceLedger`).
- Test doubles must embed `sync.Mutex` and lock in every method for thread safety when shared across goroutines.
- Channel-based coordination in race-condition tests.

### Fixtures

- Use `testsupport.ProjectTempDir` for writable test directories.
- All test artifacts go under `.test-tmp/` in the project directory, never outside it.

## BDD Feature Conventions (Gherkin)

Feature files live under `features/kernel/` for kernel behavior and `features/applications/` for application behavior.

**Rules:**

- Describe observable behavior in Genesis domain language — not UI copy or implementation internals.
- Keep scenarios independent and focused on one rule.
- Drive future automation through public kernel commands and projections (`/turn`, `/sessions/{id}`, `/tools/shell_exec`, `/ready`, `/capabilities`, etc.).
- Do not bind scenarios to private helper names, storage file paths, or UI copy.
- Do not encode retired concepts as active expectations.
- Application features go under `features/applications/` with a note naming the kernel primitive being pressure-tested.

## Desktop Frontend Conventions (Vue 3 + TypeScript)

### Tech Stack
- Vue 3 Composition API (`<script setup lang="ts">`) + TypeScript + Vite, plain CSS.
- Mature application-layer libraries are allowed behind thin adapters when they reduce parser, renderer, protocol, packaging, or platform risk.
- No component library or design-system dependency without explicit design approval.
- No router — single-screen workbench.
- No global state library — component-local state only.

### Architecture Constraints
- All kernel API calls go through `desktop/frontend/src/api/kernelApi.ts` (the HTTP choke point).
- Desktop is a user-space shell. It reads kernel projections and submits turns; it does not assemble provider context, write ledger facts, or own memory truth.
- Desktop owns the `genesisd` sidecar lifecycle only when `GENESIS_KERNEL_BASE_URL` is unset. When set, desktop treats the kernel as external and must not start or stop it.

### Component Conventions
- Components: `KernelTopBar`, `SessionRail`, `ConversationPane`, `InspectorDrawer`.
- Props defined with type-only `defineProps<{...}>()`.
- Events via typed `defineEmits<{...}>()`.
- Projection helpers in separate modules (`display.ts`, `timelineView.ts`, `approvalView.ts`).
- Plain CSS only — scoped per component.

## Verification Checklist

Before every implementation commit, run through this checklist:

1. `git diff --check` (whitespace errors)
2. `go test ./... -count=1` (all tests)
3. `go build ./...` (all packages compile)
4. Review diff against code conventions in this document
5. Commit message follows the structured format
6. Closing gate drift check against requirement/design/plan/issue/BDD feature
7. In-scope drift fixed; out-of-scope drift recorded as active issue
