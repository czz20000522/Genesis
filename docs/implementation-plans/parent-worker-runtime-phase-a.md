# Implementation Plan: Parent-Led Worker Runtime Phase A

## Requirement And Design

- Requirement: `docs/requirements/kernel-parent-worker-runtime.md`
- Design: `docs/design/kernel-parent-worker-runtime.md`
- Issue: `docs/operations/kernel-issues.md#kernel-parent-worker-role-binding-20260708`
- BDD: `features/kernel/parent_worker_runtime.feature`

## Reference Scan

- Codex references inspected:
  - `codex-rs/core/src/tools/handlers/multi_agents.rs`
  - `codex-rs/core/src/tools/handlers/multi_agents/spawn.rs`
  - `codex-rs/core/src/tools/handlers/agent_jobs.rs`
- Reasonix references inspected:
  - `internal/skill/skill.go`
  - `internal/boot/boot.go`
  - `internal/agent/task.go`
- Alignment:
  - Follow Codex by keeping spawned agents as explicit control-plane operations with parent-child metadata and concurrency limits.
  - Follow Reasonix by resolving subagent model/tool constraints from configured skill/profile metadata rather than from the parent prompt.
- Intentional differences:
  - Genesis Phase A only parses and projects role bindings. It does not yet spawn workers, run task graphs, or implement graph visualization.
  - Genesis keeps role bindings in `models.json` beside model profiles, because existing provider refresh and bind operations already use that local config boundary.
- Drift risks or follow-up issues:
  - Task graph node/edge/layout state must be modeled in its own requirement before graph scheduling or visualization is implemented.

## Reference Behavior Red Tests

- Reference behavior: Reasonix scopes subagent tools through configured `allowed-tools`; Codex records spawned agents as distinct identities.
- Genesis equivalent: role binding projection exposes preset tools, and future worker admission must refuse per-call tools outside the role binding.
- Test file or guard: `internal/kernel/model_config_test.go` and `features/kernel/parent_worker_runtime.feature`.
- Initial red condition: Genesis has no parent-worker role binding parser or projection.
- Accepted intentional difference: no worker execution in Phase A.

## Phase A

- Deliverable: read, validate, and project parent/worker role bindings from `models.json`.
- Red lines:
  - Do not add provider SDKs.
  - Do not implement task graph nodes, edges, layout, or scheduling.
  - Do not let role labels grant authority without `AgentInvocation`.
- Tests:
  - focused model config tests for role binding projection and invalid tool/profile references.
  - architecture boundary test.
- Evidence:
  - `go test ./internal/kernel -run "Test(ParentWorker|ResolveProviderConfig)" -count=1`
  - `go test ./internal/kernel -run TestArchitectureBoundary -count=1`
  - `git diff --check`
- Still short of production:
  - worker invocation creation from role binding.
  - child conversation HTTP projection.
  - independent task graph requirement and implementation.
- Closing gate:
  - Requirement/design/issue/BDD items checked before commit.
  - In-scope drift fixed before commit.
  - Out-of-scope drift recorded as active issue.

## Retirement Criteria

The active issue can move to retirement when Phase A role binding config parsing, validation, projection, and focused tests are committed and the remaining worker execution/task graph gaps are either out of scope in this plan or represented by separate active issues.
