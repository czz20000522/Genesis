# Requirement: User-space Code Intelligence Runtime

- **Status:** approved
- **Owner:** user-space code intelligence runtime
- **Scope:** codebase indexing readiness, semantic code query requests, adapter result shaping, local cache policy, and kernel boundary protection

## Background

Genesis needs code intelligence for repository navigation, symbol lookup, call
graph inspection, impact analysis, and future developer-facing workflows. That
capability is valuable, but it is not a kernel primitive. CodeGraph, language
servers, local AST indexes, MCP servers, and CLI search tools are external code
intelligence providers.

The production boundary is:

```text
User-space Code Intelligence Runtime -> CodeGraph Adapter -> codegraph CLI/MCP
```

The kernel remains responsible for authority, tool execution, facts, recovery,
memory, jobs, audit, and provider context. The code intelligence runtime may
feed user-space applications, operator tools, or future generic resource/context
admission, but it must not enter kernel core or assemble provider context
directly.

## Production Target

The user-space code intelligence runtime adapts repository code intelligence
requests into stable Genesis application-level requests and results. It owns
project registration, cache readiness, index freshness posture, query shaping,
result bounding, result evidence, and operator diagnostics.

The CodeGraph adapter owns CodeGraph-specific details: CLI/MCP invocation,
index initialization, sync, command flags, result parsing, and `.codegraph/`
cache mechanics. The runtime treats CodeGraph as an external adapter behind a
stable request/result boundary, not as a kernel dependency or a public Genesis
data contract.

## Users And Roles

Operators use code intelligence to inspect large repositories without broad
text scans and to decide which files, symbols, tests, or owners need direct
source inspection.

User-space applications can request code intelligence as part of a workflow,
for example impact review, architecture review, or implementation planning.

Code intelligence adapters translate stable runtime requests into a concrete
provider such as CodeGraph CLI, CodeGraph MCP, language server, or another local
indexer.

Genesis Kernel does not know CodeGraph. It may receive generic application
events, resource handles, or turn requests produced by a user-space application,
but it cannot import CodeGraph packages, read `.codegraph/`, expose CodeGraph
tools as kernel tools, or treat CodeGraph output as ledger truth.

The LLM may consume bounded, admitted context produced through approved
application or resource/context paths. It does not own the CodeGraph credential,
index cache, project path, telemetry policy, or raw adapter protocol.

## Core Semantics

`CodeProjectRef` is an opaque application-owned reference to a repository root
that has been admitted for code intelligence. It is not a raw public filesystem
path, kernel authority id, or provider context fragment.

`CodeIndexReadiness` records whether a code intelligence adapter is usable for a
project. Status values are `ready`, `not_installed`, `cache_missing`,
`cache_stale`, `degraded`, or `blocked`. Readiness does not prove query
correctness; source files and tests remain the final truth.

`CodeIndexCache` is rebuildable local adapter state. For CodeGraph, `.codegraph/`
is a local cache directory, must be ignored by git, must not be committed, and
does not create data migration or compatibility obligations.

`CodeQuery` is a stable user-space request such as semantic explore, symbol
lookup, callers, callees, impact, or affected-tests hint. It names a
`CodeProjectRef`, query kind, bounded query text or symbol ref, and result
limits. It must not include credentials, broad home-directory paths, filesystem
roots, or host handles.

`CodeQueryResult` is bounded adapter output with result kind, snippets or refs,
confidence, adapter diagnostics, and freshness evidence. It is guidance for
navigation, not proof of behavior. Direct source inspection, compiler output,
tests, and linters remain authoritative.

`CodeIntelligenceEvidence` records how a result was produced: adapter kind,
project ref, readiness state, freshness posture, query kind, result count,
truncation, and safe diagnostic reason codes. It must not persist raw MCP
frames, full CLI stdout/stderr, credentials, telemetry payloads, or index
database contents.

## Security And Privacy

The runtime must scope queries to an admitted project root. It must not index or
query home directories, filesystem roots, secret stores, browser profiles, or
unrelated parent directories.

For private-code work, telemetry must be disabled or explicitly approved by an
operator before adapter use. If telemetry state cannot be determined, the
adapter is `blocked` for private project queries.

Adapter results are bounded and sanitized before they become application
outputs. Long raw output belongs in an explicit debug trace with TTL, quota,
redaction, and operator-only access, not in durable application facts.

## Non-goals

- No CodeGraph package, MCP shape, CLI argv, cache database, or index path enters
  kernel core.
- No `codegraph` model-visible kernel tool is added.
- No provider context is assembled by the code intelligence runtime.
- No `.codegraph/` contents are committed, migrated, or treated as current
  Genesis state.
- No external index result is treated as proof that source behavior is correct.

## Phased Delivery

Phase A defines this requirement, the design, and the repository ignore policy
for `.codegraph/`.

Phase B adds a user-space readiness probe with a fake adapter and optional live
CodeGraph smoke. It reports installed/cache/freshness/telemetry posture without
querying private code when blocked.

Phase C adds stable `CodeQuery` to `CodeQueryResult` execution through a
CodeGraph adapter. Tests use fake adapter outputs; live CodeGraph commands are
manual smoke only.

Phase D integrates selected results with generic resource/context admission
when that owner is ready. The code intelligence runtime submits bounded
resource/context candidates; it still does not assemble provider context.

## Acceptance Criteria

- `.codegraph/` is documented and ignored as local rebuildable cache before any
  real repository index is initialized.
- CodeGraph can be replaced by another adapter without changing kernel core.
- A blocked or stale index prevents claims of current code intelligence rather
  than silently producing authoritative-looking results.
- Application outputs distinguish CodeGraph guidance from source/test truth.
- Kernel tests and package boundaries prove no CodeGraph dependency or tool
  surface enters kernel core.
