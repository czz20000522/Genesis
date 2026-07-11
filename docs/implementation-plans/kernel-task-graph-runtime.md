# Implementation Plan: TaskGraph Runtime

- Requirement: `docs/requirements/kernel-task-graph-runtime.md`.
- Design: `docs/design/kernel-task-graph-runtime.md`.

## Reference Scan

Codex local spawn control records lifecycle/capacity before completion delivery;
Reasonix `internal/agent/task.go` starts a fresh bounded child with filtered
tools and only returns its final. Neither contains a durable DAG owner. Genesis
will reuse ledger authority and bounded references, not either in-memory task
path.

## Reference Behavior Red Tests

- Explicit lifecycle facts: reject a cycle or illegal transition without an
  appended graph event, then reconstruct the accepted graph after restart.
- Bounded execution binding: prove a ready task can reference an already
  authorized owner without accepting tools, provider/profile data, or a fresh
  authority path.

## Phase A

- Deliverable: ledger-owned graph/node/edge facts, DAG validation, lifecycle
  reducer, ready/blocked projection, and restart reconstruction.
- Red lines: no scheduler, no tool/provider execution, no new permissions.
- Red tests: cycle/duplicate/missing edge refusal, dependency readiness,
  terminal immutability, restart identity, and no append on rejection.
- Completion evidence: ledger-only graph/node/edge/transition facts now reject
  missing invocation references and cycles without an append, derive
  ready/blocked dependency state, retain terminal evidence refs, and rebuild
  after restart.
- Still short: referenced execution and desktop graph interaction.

## Phase B

- Deliverable: optional owner-controlled execution binding, persisted linkage,
  terminal state/evidence reduction, and fail-closed restart reconciliation.
- Red lines: no provider/tool fields in a task proposal, no direct provider
  call, no replay of a started invocation, and no generic scheduler.
- Red tests: binding validation, dependency-gated eligibility, terminal owner
  reduction, and ambiguous restart block.
