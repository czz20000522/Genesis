# Design: Workspace Edit Tool

- **Requirement:** `docs/requirements/kernel-workspace-edit-tool.md`
- **Owner:** Genesis Kernel tool gateway and workspace authority boundary.

## Reference Scan

Codex:

- `codex-rs/core/src/tools/handlers/apply_patch_spec.rs` exposes a dedicated
  model tool for edits instead of asking the model to shell out.
- `codex-rs/core/src/tools/handlers/apply_patch.rs` parses patch deltas,
  converts file changes into semantic progress events, computes affected
  paths, and routes execution through the tool orchestrator.
- `codex-rs/core/src/tools/runtimes/apply_patch.rs` executes an already
  verified patch through the selected environment filesystem, sandbox context,
  and approval path.

Reasonix:

- `internal/tool/builtin/editfile.go` implements exact string replacement and
  requires the old string to be unique.
- `internal/tool/builtin/multiedit.go` applies ordered in-memory edits and
  writes only after every step succeeds.
- `internal/tool/builtin/writefile.go` keeps writes in a typed tool rather than
  shell commands and preserves file encoding when overwriting.
- `internal/tool/builtin/confine.go` binds writer tools to workspace roots,
  resolves symlink-aware real paths, and rejects writes outside the workspace.

Genesis alignment:

- Genesis aligns with Codex by making edits a typed tool gateway operation, not
  an application-side fact.
- Genesis aligns with Reasonix Phase A by using exact replacement semantics and
  workspace confinement before attempting patch grammar support.
- Genesis intentionally differs from Codex in Phase A by not exposing a
  freeform patch grammar. The current Go kernel has no patch parser or remote
  filesystem abstraction, so exact replacement is the safer first production
  slice.

## Owner Boundary

Owner: kernel tool gateway.

Shared existing owners:

- `ToolRegistry` advertises the tool schema.
- `ToolPolicy` decides write authorization.
- `toolruntime` scheduling serializes workspace writes.
- Event ledger stores model tool calls and model tool results as turn truth.

Non-owners:

- Shell runtime does not own model-initiated edits.
- Desktop and console do not apply edits directly for model tool calls.
- Provider adapters do not rewrite edit requests.

## Data Flow

1. Model calls `workspace_edit` with JSON arguments.
2. `ToolGateway.PrepareBatch` strictly decodes the arguments.
3. The kernel resolves the relative path against `ToolPolicy.WorkspaceRoot`.
4. The kernel validates the resolved path is inside the workspace root.
5. The normal tool authorization gate blocks the call when write tools are not
   allowed.
6. Execution reads the file, verifies `old_string` occurs exactly once, writes
   the replacement, and returns a compact result.
7. `commitToolExecutionResult` appends the result event to the ledger.
8. The next provider request receives the semantic tool result through the
   existing provider context projection.

## Model-Visible Schema

Tool input:

```json
{
  "path": "src/example.go",
  "old_string": "old text",
  "new_string": "new text"
}
```

Success result:

```json
{
  "status": "completed",
  "tool": "workspace_edit",
  "executed": true,
  "path": "src/example.go",
  "replacements": 1,
  "bytes_before": 100,
  "bytes_after": 108
}
```

Repair result uses the existing `ToolRequestInvalidProjection` shape.

No result exposes the workspace root, host absolute path, sandbox profile,
approval id, credential, or full file content.

## Path Confinement

The kernel resolves paths with these rules:

- workspace root must be configured and absolute;
- tool paths must be relative;
- cleaned paths must not be empty, `.`, absolute, or traverse above root;
- symlink-aware resolution must prove the final target path is inside the
  workspace root;
- for missing future write phases, the deepest existing ancestor must be
  resolved before appending the missing tail.

Phase A requires an existing regular file, so symlink resolution can evaluate
the target file directly.

## Scheduling

`workspace_edit` uses:

- effect class: workspace write;
- parallel policy: serial fence;
- trusted: false unless admission proves otherwise.

This means independent pure-read tools can still be batched by existing logic,
but edits do not run concurrently with other writes.

## Failure And Recovery

- Invalid arguments are repairable model results.
- Pre-mutation validation failures do not modify the file.
- Read and write failures are sanitized.
- There is no retry loop inside the edit primitive. The model may issue a new
  corrected tool call after receiving repair feedback.
- No audit entry is required in Phase A because the ledgered tool call/result
  already records the turn-level execution fact. Future approval support may
  add audit/approval events.

## Observability

The session, UI timeline, and provider context continue to consume the existing
tool result event. The result is compact by design; callers should inspect the
workspace diff through normal project tools if they need the exact changed
lines.
