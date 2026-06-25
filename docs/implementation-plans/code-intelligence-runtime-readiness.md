# Implementation Plan: Code Intelligence Runtime Readiness

- **Requirement:** `docs/applications/code-intelligence-runtime-requirement.md`
- **Design:** `docs/applications/code-intelligence-runtime-design.md`
- **Issue:** `APP-CODE-INTELLIGENCE-RUNTIME-READINESS-20260625`
- **Status:** implemented first readiness/query slice

## Reference Scan Summary

Inspected local references:

- `D:\software\JetBrains\python_workspace\reasonix\internal\codegraph\codegraph.go`
- `D:\software\JetBrains\python_workspace\reasonix\internal\codegraph\read_only.go`
- `D:\software\JetBrains\python_workspace\reasonix\internal\codegraph\e2e_test.go`
- `D:\software\JetBrains\python_workspace\codex-main\codex-rs\app-server\src\request_processors\thread_processor.rs`
- `D:\software\JetBrains\python_workspace\codex-main\codex-rs\core\src\tools\handlers\tool_search_spec.rs`
- `D:\software\JetBrains\python_workspace\codex-main\codex-rs\core\src\tools\handlers\mcp_resource\read_mcp_resource.rs`

Learned control-plane semantics:

- Reasonix keeps CodeGraph behind a plugin/MCP adapter, initializes `.codegraph/`
  as local project state, marks CodeGraph tools read-only, and gates real e2e
  use behind explicit environment opt-in.
- Codex validates externally supplied dynamic tool namespaces and schemas before
  model exposure, keeps MCP resource reads behind typed handlers, and uses
  deferred discovery instead of dumping all external capabilities into context.
- Both references treat external tool/resource output as bounded evidence or
  context, not as source-of-truth proof.

Intentional Genesis differences:

- Genesis does not expose CodeGraph as a model-visible kernel tool in this
  slice.
- Genesis first creates a user-space runtime readiness/query projection with a
  fake adapter for deterministic tests and a CLI adapter for optional live
  smoke.
- `affectedTests` and covering-test claims are advisory hints. Go tests and
  source inspection remain the verification authority.

Remaining drift risks:

- accidentally turning CodeGraph query output into provider context without the
  resource/context owner;
- using a parent repository `.codegraph/` cache for a worktree;
- treating CodeGraph affected-tests output as a complete test selection oracle.
