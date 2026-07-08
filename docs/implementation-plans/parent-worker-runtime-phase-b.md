# Implementation Plan: Parent-Led Worker Runtime Phase B

## Requirement And Design

- Requirement: `docs/requirements/kernel-parent-worker-runtime.md`
- Design: `docs/design/kernel-parent-worker-runtime.md`
- Issue: `docs/operations/kernel-issues.md#kernel-parent-worker-invocation-20260708`
- BDD: `features/kernel/parent_worker_runtime.feature`

## Reference Scan

- Codex references inspected:
  - `codex-rs/core/src/tools/handlers/multi_agents/spawn.rs`
  - `codex-rs/core/src/tools/handlers/agent_jobs.rs`
- Reasonix references inspected:
  - `internal/agent/task.go`
  - `internal/skill/tools.go`
- Alignment:
  - Follow Codex by creating distinct child execution identities.
  - Follow Reasonix by deriving worker tools from configured metadata and returning bounded child results through the existing invocation owner.
- Intentional differences:
  - Genesis Phase B adds a kernel helper over existing `AgentInvocation`; it does not add HTTP, CLI, task graph, or visual layout surfaces.

## Phase B

- Deliverable: admit a worker invocation from a configured role binding, using the role preset tool set and refusing extra parent-provided tools.
- Red lines:
  - No recursive worker creation.
  - No task graph implementation.
  - No per-call tool grant widening.
- Tests:
  - role-bound worker admission uses preset tools.
  - same role admits multiple invocation identities.
  - extra requested tools are refused before ledger append.
- Evidence:
  - `go test ./internal/kernel -run "Test(AdmitWorkerInvocationFromRole|AgentInvocation)" -count=1`
  - `go test ./internal/kernel -run TestArchitectureBoundary -count=1`
  - `git diff --check`
- Still short of production:
  - child conversation HTTP projection.
  - task graph requirement and visualization.

## Retirement Criteria

Delete this plan and move the issue to retirement evidence when the kernel helper, tests, and closing gate pass.
