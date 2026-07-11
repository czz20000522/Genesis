# Implementation Plan: Parent Delegate Worker Tool

## Requirement And Design

- Requirement: `docs/requirements/kernel-parent-worker-runtime.md`.
- Design: `docs/design/kernel-parent-worker-runtime.md`.
- BDD: `features/kernel/parent_worker_runtime.feature`.

## Reference Scan

- Codex inspected `codex-rs/core/src/tools/handlers/multi_agents_v2/spawn.rs`,
  `agent/control/spawn.rs`, `agent/control.rs`, and `wait.rs`. Spawn records a
  parent-child edge, reserves capacity, emits lifecycle activity, and delivers
  completion back to the parent; its history fork and free model overrides are
  intentionally rejected by Genesis.
- Reasonix inspected `internal/agent/task.go` and `coordinator.go`. A child
  starts with a fresh self-contained task, filters meta-tools, and returns only
  its final answer; its ad-hoc profile override and in-memory job path are
  intentionally rejected by Genesis.
- Genesis aligns on bounded final-result return and leaf tool filtering. It
  differs by using persisted role binding as the only provider/profile/tool
  authority and by keeping parent/child facts in the ledger.

## Reference Behavior Red Tests

- Fresh child context and no recursive delegation: add a parent tool-loop test
  whose child request has only the focused task and a manifest without
  `delegate_worker`.
- Bounded result return: add a parent tool-loop test that observes a
  `delegate_worker` tool result with role, terminal status, final text, usage,
  and evidence refs but no raw child prompt/reasoning/tool trace.
- Role-selected provider: add a resolver-backed test proving worker execution
  uses the admitted role profile rather than the daemon's parent provider.

## Phase A: Role-Bound Delegation In A Live Parent Turn

- Deliverable: add the one `delegate_worker(role_id, task)` kernel-control tool,
  persist delegation/invocation identity, resolve the worker profile through a
  daemon-supplied resolver, launch one leaf worker asynchronously, and pause the
  parent turn with a safe queued receipt.
- Red lines: no `review`/`reduce` type, no role/model/tool/fork arguments, no
  worker access to `delegate_worker`, no TaskGraph, and no new provider registry.
- Tests: schema/strict-input, rejected unknown role, leaf manifest filtering,
  worker provider selection, persisted queued/running/terminal state, and parent
  pause without a raw child trace.
- Still short of production: worker execution is in-process and the paused
  parent does not continue automatically until Phase B; restart recovery and
  concurrent dispatch remain Phase B.

## Phase B: Durable Delegation Recovery

- Deliverable: persist focused task and parent tool-call binding before starting
  provider work; reconstruct queued/running delegations after daemon restart,
  resume only from an unambiguous admitted checkpoint, append the bounded worker
  result as the parent tool result, and continue the paused parent turn.
- Red lines: do not replay unknown external tool effects, reconstruct a parent
  tool round from raw provider state, or invent terminal success.
- Tests: restart before worker start, restart after terminal worker result but
  before parent continuation, and fail-closed ambiguous worker-tool recovery.
- Concurrency completion: role `max_parallel` is enforced at admission (default
  6) and parent `max_children` is enforced across roles (default 24); each
  admitted worker snapshots its parent binding id for restart-safe accounting.
- Still short of production: provider-route and model-profile concurrency
  admission remain a separate bounded slice, not TaskGraph scheduling.

## Phase C: Operator And Desktop Projection

- Deliverable: project delegation status, child conversation link, safe failure,
  and parent continuation state for desktop inspection.
- Red lines: no raw child trace in parent projection and no desktop-owned facts.
- Tests: HTTP/session projection and desktop bridge rendering proof.

## Retirement Criteria

Move the issue only after a configured parent completes a role-bound worker and
reviewer delegation, parent reduction returns a unified final, child projections
remain isolated, and restart recovery has reproducible evidence.
