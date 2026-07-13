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

- Deliverable: ledger-owned graph/node/edge facts, append-only unstarted task
  metadata and edge mutations, DAG validation, lifecycle reducer,
  ready/blocked projection, and restart reconstruction.
- Red lines: no scheduler, no tool/provider execution, no new permissions.
- Red tests: cycle/duplicate/missing edge refusal, dependency readiness,
  terminal immutability, restart identity, and no append on rejection.
- Completion evidence: ledger-only graph/node/edge/transition facts now reject
  missing invocation references and cycles without an append, derive
  ready/blocked dependency state, retain terminal evidence refs, allow only
  unstarted task metadata and dependency mutations, and rebuild those mutations
  after restart.
- Still short: referenced execution and desktop graph interaction.

## Phase B

- Deliverable: optional owner-controlled execution binding, persisted linkage,
  terminal state/evidence reduction, and fail-closed restart reconciliation.
- Parent command: reuse `task_graph_edit` with `bind_invocation`; it may bind
  only a ready node to one existing same-session invocation and never accepts
  role, task, model, provider, tool, or start data. Its caller session must own
  the graph; a lifecycle `running` fact makes the linked node immutable.
- Red lines: no provider/tool fields in a task proposal, no direct provider
  call, no replay of a started invocation, no call to `delegate_worker`, and
  no generic scheduler.
- Red tests: binding validation, one-node/one-invocation refusal, caller
  session refusal/no leakage, dependency-gated eligibility, running and
  terminal owner reduction, failed-transition restart reconciliation, generic
  and delegated ambiguous-restart failure without replay (including a later
  run after an earlier terminal one), and fail-closed initialization when
  required recovery cannot be written.

## Phase C

- Deliverable: authenticated graph list/read projection scoped to a session,
  then a desktop inspector reader for task titles, status, blockers, dependency
  edges, and evidence references.
- Red lines: no desktop truth, no graph mutation route, no automatic start, and
  no model-visible tool until its separate proposal contract exists.
- Red tests: session scoping, unknown graph refusal, no write caused by graph
  reads, and frontend API choke-point/static guard coverage.
- Reference scan: Codex emits an explicit plan-update event through the tool
  runtime before rendering it; Reasonix keeps planning as a model-session
  handoff. Genesis uses the same command/projection separation but keeps the
  project graph in the ledger.
- Follow-up: parent-only `task_graph_edit` tool, one validated topology command
  per call, child-invocation refusal, and no execution binding or start.
