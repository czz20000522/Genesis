# Design: User-space Code Intelligence Runtime

- **Requirement:** `docs/applications/code-intelligence-runtime-requirement.md`
- **Owner:** user-space code intelligence runtime
- **Status:** approved

## Boundary Model

```text
Operator / User-space Application
        |
        v
User-space Code Intelligence Runtime
        |
        v
CodeProjectRef / CodeQuery / CodeQueryResult
        |
        v
CodeGraph Adapter
        |
        v
codegraph CLI or CodeGraph MCP
        |
        v
.codegraph/ local cache
```

The runtime is an application-layer protocol boundary. It is similar in shape
to Model Gateway and Application Connector Runtime because it adapts an
external protocol, but it does not own model requests or external channel
delivery. It owns code-intelligence application facts only.

Kernel integration is indirect:

```text
CodeQueryResult
        |
        v
Application policy / operator decision
        |
        v
optional generic resource or context admission
        |
        v
Kernel public primitive
```

The runtime never writes kernel ledger facts, memory truth, tool results,
provider context, checkpoints, approval facts, or audit facts.

## Owner Responsibilities

Code Intelligence Runtime owns:

- admitted project references and path scoping;
- adapter readiness and freshness projection;
- query kind validation and result limits;
- safe result projection and truncation;
- code-intelligence evidence records;
- operator diagnostics for blocked, stale, or degraded indexes.

CodeGraph Adapter owns:

- locating the `codegraph` executable or MCP endpoint;
- telemetry posture checks before private-code use;
- index init/sync/status command details;
- CLI/MCP request and response translation;
- CodeGraph-specific parsing and failure classification;
- `.codegraph/` cache lifecycle as rebuildable adapter state.

Kernel owns:

- turn lifecycle, provider context, authority, tool execution, resource
  admission, memory, jobs, audit, checkpoint, and durable kernel facts.

LLM owns:

- semantic intent over bounded context that an application or kernel owner has
  explicitly admitted. It does not own CodeGraph commands, project paths,
  telemetry state, cache files, or MCP frames.

## Data Shapes

`CodeProjectRef`:

- `project_ref`
- `display_name`
- `root_digest`
- `admitted_root`
- `adapter_binding_ref`

`CodeIndexReadiness`:

- `project_ref`
- `adapter`
- `status`
- `freshness`
- `telemetry_posture`
- `blocked_reason`
- `checked_at`

`CodeQuery`:

- `project_ref`
- `query_kind`
- `query_text`
- `symbol_ref`
- `result_limit`
- `freshness_required`

`CodeQueryResult`:

- `query_ref`
- `status`
- `items`
- `result_count`
- `truncated`
- `freshness`
- `diagnostic_reason`

`CodeIntelligenceEvidence`:

- `query_ref`
- `project_ref`
- `adapter`
- `readiness_ref`
- `result_ref`
- `diagnostic_summary`

These shapes are user-space application contracts. They are not kernel event
types and not model-visible tool schemas by default.

## Readiness Flow

1. Runtime receives an operator or application request for a project.
2. Runtime resolves the request to an admitted `CodeProjectRef`.
3. Runtime asks the adapter for readiness.
4. CodeGraph adapter checks executable or MCP availability, telemetry posture,
   cache presence, and freshness.
5. Runtime records `CodeIndexReadiness`.
6. Queries run only when readiness is `ready` or when the caller explicitly
   accepts a degraded freshness posture.

`cache_missing` can produce an operator action recommendation to initialize the
index. Initialization is an adapter operation that creates `.codegraph/` under
the admitted project root only after the ignore policy exists.

## Query Flow

1. Application submits `CodeQuery`.
2. Runtime validates query kind, project scope, and result limit.
3. Runtime checks latest readiness.
4. Adapter translates the stable query into CodeGraph CLI/MCP.
5. Adapter returns bounded parsed results and diagnostics.
6. Runtime writes `CodeQueryResult` and `CodeIntelligenceEvidence`.
7. Application may use result refs for direct source inspection, operator
   display, or future generic resource/context admission.

The runtime does not treat CodeGraph output as final truth. Any implementation
or review conclusion must still inspect source and run tests or build checks.

## Cache And Ignore Policy

`.codegraph/` is rebuildable local cache. It must be listed in `.gitignore`
before real indexing is initialized in this repository. The runtime must not
read the cache database directly; only the adapter may operate it through
CodeGraph-supported surfaces.

Removing `.codegraph/` must not require migrations, compatibility readers, or
kernel repair flows. A missing cache is a readiness state, not data loss.

## Failure Semantics

`not_installed`: adapter executable or MCP endpoint is unavailable.

`cache_missing`: project has no local cache yet.

`cache_stale`: adapter reports pending changes or mismatch that may make query
results stale.

`degraded`: adapter can answer but with incomplete freshness or bounded
diagnostics.

`blocked`: telemetry, path scope, credential, or policy posture prevents safe
query execution.

Malformed adapter output is a runtime failure record and does not become a
`CodeQueryResult`. Raw output is bounded and kept only in debug diagnostics when
explicitly enabled.

## Architecture Guards

- Kernel packages must not import a code intelligence runtime package.
- Kernel tool registry must not expose `codegraph`, `codegraph_query`, or
  CodeGraph-specific tools.
- CodeGraph adapter tests use fake CLI/MCP outputs for repeatable behavior.
- Live CodeGraph smoke is manual or explicitly marked external; it must not be
  required for normal `go test ./...`.

## Rejected Paths

Rejected: putting CodeGraph in kernel core. The kernel would become a
code-domain owner and would inherit adapter cache, telemetry, and query
freshness policy.

Rejected: letting the LLM call `codegraph` directly through shell as the
production path. That bypasses readiness, telemetry, scope, result bounding,
and evidence.

Rejected: committing `.codegraph/` or using its database as a Genesis contract.
It is adapter-local cache and can be rebuilt.
