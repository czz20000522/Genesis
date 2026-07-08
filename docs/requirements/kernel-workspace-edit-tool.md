# Requirement: Workspace Edit Tool

- **Status:** approved for phased implementation.
- **Owner:** Genesis Kernel tool gateway and workspace authority boundary.
- **Scope:** expose a model-callable, workspace-confined file edit primitive as a generic kernel tool.

## Background

Genesis can read admitted resources and source snapshots through typed,
bounded tool calls. It can also execute governed shell commands, but shell
commands are a broad process surface. A local-first agent kernel needs a more
deterministic way for a model to request targeted file changes without turning
every edit into an opaque shell command.

This is a generic tool primitive, not an application feature. The kernel owns
admission, workspace confinement, execution, result projection, and ledgered
tool result facts. Applications may render edit results, but they do not mint
edit truth.

## Production Target

Genesis supports workspace-scoped file edits through model tool calls:

- edit tools are advertised through the normal `ToolRegistry`;
- write authority is governed by the existing `ToolPolicy`;
- `plan` mode blocks edit tools;
- `default` and `yolo` modes may execute according to the resolved policy;
- `approval_required` remains a semantic refusal until an approval flow is
  explicitly implemented for the edit tool;
- every edit is confined to the configured workspace root;
- paths are normalized and must not escape through absolute paths, `..`, or
  symlinked ancestors;
- invalid requests return repairable tool results instead of panics or
  infrastructure errors;
- successful results expose bounded semantic summaries, not full file content;
- tool scheduling treats edits as workspace writes and serializes them against
  other effectful tools.

## Users And Roles

LLM:

- proposes a targeted edit using a stable tool schema;
- receives a compact success or repair result.

Operator:

- chooses the permission mode and workspace root;
- remains responsible for reviewing generated edits through normal project
  workflow.

Kernel:

- validates arguments;
- enforces workspace confinement;
- applies the edit;
- records the tool call and result in the event ledger;
- exposes only semantic model-visible fields.

Application shell:

- may render edit results or approvals later;
- does not bypass the kernel tool gateway for model-initiated edits.

## Semantics

Phase A introduces a single exact-replace tool, `workspace_edit`:

1. The request contains `path`, `old_string`, and `new_string`.
2. `path` is resolved relative to `ToolPolicy.WorkspaceRoot`.
3. The resolved path must be inside the workspace root after symlink-aware
   normalization.
4. The target must be an existing regular file.
5. `old_string` must be non-empty and must occur exactly once in the file.
6. The replacement is applied once.
7. If the file already has the requested post-edit content, the result may be
   reported as a no-op only when the exact replacement semantics prove it.
8. Result payload includes status, executed flag, tool name, relative path,
   bytes before and after, and replacements applied.
9. Result payload does not include full file content, host absolute paths,
   workspace root, approval ids, sandbox profiles, or credentials.

Later phases may add:

- atomic multi-edit against one file;
- file creation or full-file write;
- patch grammar support;
- first-class approval flow for edit tools.

## Failure Semantics

- Missing workspace root: `workspace_root_required`.
- Invalid path: `invalid_workspace_edit_path`.
- Path outside workspace: `path_outside_workspace`.
- Missing file: `workspace_edit_target_missing`.
- Directory target: `workspace_edit_target_not_file`.
- Missing `old_string`: `workspace_edit_old_string_required`.
- Old string not found: `workspace_edit_old_string_not_found`.
- Old string not unique: `workspace_edit_old_string_not_unique`.
- Read failure: `workspace_edit_read_failed`.
- Write failure: `workspace_edit_write_failed`.

Failures before mutation return `tool_request_invalid` with `executed=false`.
Infrastructure failures after admission return sanitized failures and must not
expose host paths.

## Non-Goals

- No application-specific file formats or code intelligence semantics.
- No shell command rewriting or PowerShell wrapper behavior.
- No broad filesystem manager.
- No automatic formatting or linting.
- No compatibility layer for old edit artifacts.
- No migration reader for previous experiments.

## Acceptance Criteria

1. The `workspace_edit` tool appears in the manifest as a write tool with
   workspace-write scheduling.
2. In `plan` mode, `workspace_edit` returns `permission_denied` without
   mutating the file.
3. In write-enabled mode, a valid exact replacement mutates one workspace file
   and returns a bounded success payload.
4. The tool rejects paths outside the workspace, including traversal and
   symlink escape attempts.
5. The tool rejects missing, empty, absent, or non-unique `old_string` values
   without mutating the file.
6. Tool results and model-visible projections do not leak absolute host paths
   or workspace roots.
7. The implementation is covered by focused kernel tests and the normal repo
   verification gate.
