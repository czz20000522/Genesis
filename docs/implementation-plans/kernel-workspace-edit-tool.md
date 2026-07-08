# Kernel Workspace Edit Tool Implementation Plan

> **For agentic workers:** keep Phase A exact, workspace-confined, and routed
> through the existing tool gateway.

## Requirement And Design

- Requirement: `docs/requirements/kernel-workspace-edit-tool.md`
- Design: `docs/design/kernel-workspace-edit-tool.md`
- BDD: `features/kernel/workspace_edit_tool.feature`

## Phase A: Exact Replace Tool

**Deliverable:** model-callable `workspace_edit` replaces one unique string in
one existing workspace file and returns a compact semantic result.

**Files:**

- Modify: `internal/kernel/tool_registry.go`
- Modify: `internal/kernel/model_tools.go`
- Modify: `internal/kernel/tool_scheduling.go`
- Modify or add: `internal/kernel/workspace_edit.go`
- Test: `internal/kernel/workspace_edit_test.go`

**Red lines:**

- Do not expose host absolute paths or workspace roots in model-visible output.
- Do not implement file creation, full-file write, or patch grammar in Phase A.
- Do not bypass `ToolRegistry`, `ToolGateway`, `ToolPolicy`, or existing tool
  scheduling.
- Do not add dependencies.

- [x] Step 1: Add failing manifest and scheduling tests.

  Prove `workspace_edit` is registered as a write tool and prepares a
  workspace-write serial access plan.

- [x] Step 2: Add failing execution and refusal tests.

  Cover valid replacement, plan-mode denial without mutation, outside-workspace
  rejection, symlink escape rejection, missing file, and non-unique old string.

- [x] Step 3: Implement path admission and exact replacement.

  Add path normalization, workspace confinement, regular-file check, exact
  single replacement, and compact JSON result.

- [x] Step 4: Verify.

  Run focused tests, then:

  ```powershell
  git diff --check
  go test ./... -count=1
  go build ./...
  ```

## Phase B: Atomic Multi-Edit

Add ordered in-memory edits against one file. Write only after every edit
succeeds. Preserve the same result and confinement rules.

## Phase C: Patch Grammar

Consider a patch grammar only after Phase A/B prove the tool gateway, path
confinement, scheduling, and projection semantics. This phase should reopen the
Codex `apply_patch` references before design.
