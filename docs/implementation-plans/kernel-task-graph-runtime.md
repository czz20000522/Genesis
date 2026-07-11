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
- Bounded execution reference: prove a ready node only references an admitted
  invocation/workflow and cannot add tools, provider/profile data, or a fresh
  execution path.

## Phase A

- Deliverable: ledger-owned graph/node/edge facts, DAG validation, lifecycle
  reducer, ready/blocked projection, and restart reconstruction.
- Red lines: no scheduler, no tool/provider execution, no new permissions.
- Red tests: cycle/duplicate/missing edge refusal, dependency readiness,
  terminal immutability, restart identity, and no append on rejection.
- Completion evidence: ledger-only graph/node/edge/transition facts now reject
  missing invocation references and cycles without an append, derive
  ready/blocked dependency state, and rebuild after restart.
- Still short: referenced execution and desktop graph interaction.
