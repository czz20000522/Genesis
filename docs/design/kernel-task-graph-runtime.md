# Design: TaskGraph Runtime

- **Requirement:** `docs/requirements/kernel-task-graph-runtime.md`.
- **Owner:** a flat Kernel task-graph owner backed by the event ledger.

## Boundary

The owner persists project task topology and reduces optional execution-owner
projections to task state. It does not execute tools or call providers directly;
AgentInvocation, Workflow, job, approval, and external wait owners retain that
work. Parent and user can revise the graph in either planning direction.

## Facts

`task_graph.created`, `task_graph.node_added`, `task_graph.node_updated`,
`task_graph.edge_added`, `task_graph.edge_removed`, and
`task_graph.node_transitioned` carry opaque ids, task metadata, optional
execution reference, state, reason class, and evidence refs.
The read projection is reconstructed only from these facts. A transition
validates the current reduced graph while the graph mutex is held.

## Validation

The owner rejects self edges, duplicate edges, missing endpoints, cycles, and
any transition outside the explicit state table. Graph topology does not imply
execution. A later owner-controlled binding may reference an admitted
invocation, workflow, job, approval, or external wait; it cannot name a
provider or grant a tool. A node is ready when all predecessors completed.
The parent independently invokes `delegate_worker` when it elects to run an
agent task. TaskGraph neither admits nor starts that invocation; it only
validates the later stable-reference binding and reduces owner evidence.

Task and edge mutations append new facts rather than rewriting history. The
owner permits them only for nonterminal, unstarted tasks; it rejects a mutation
that would alter terminal evidence, create a cycle, or make a running task's
dependency meaning ambiguous.

## Recovery

Restart rebuilds graph state from the ledger. Phase B reconciles only a
persisted execution binding with its existing owner; ambiguous external work is
blocked with a sanitized recovery reason and never replayed by TaskGraph.

## Phase C projection

The kernel exposes only authenticated session-scoped graph reads in the first
operator projection slice: `GET /sessions/{session_id}/task-graphs` and
`GET /task-graphs/{graph_id}`. The first returns only graphs belonging to that
session; the second returns the ledger-reduced graph. Neither route creates,
updates, starts, or binds a task. Desktop may render title, status, dependency,
blocked reason, and evidence reference, but it remains a reader in this slice.

`task_graph_edit` is a model-visible kernel-control tool for the ordinary parent
session gateway and is never exposed through an invocation capability grant. Each call carries one named
topology operation and no execution, authority, or provider field. The tool
dispatches to the existing TaskGraph owner methods under their validation lock,
then returns the resulting opaque identifiers or projection. A rejected
proposal writes no graph fact; a leaf worker is denied by its grant before any
owner call.

## Reference alignment

Codex's `core/src/tools/handlers/plan.rs` accepts a model plan update only
through its tool runtime and emits an explicit event; its
`tui/src/history_cell/plans.rs` renders the resulting state as a projection.
Reasonix's `internal/agent/coordinator.go` keeps a planner's text as a separate
conversation handoff rather than a project owner. Neither local reference owns
a durable dependency graph. Genesis aligns on explicit proposal-to-fact and
reader projection separation, and rejects in-memory task queues, free model
overrides, and graph-derived authority.
