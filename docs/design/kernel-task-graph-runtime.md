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
For the current AgentInvocation slice, one ready node may bind one invocation
in that same session. A `running` lifecycle fact makes the node immutable; a
terminal fact adds its event reference and completes or fails the node. The
parent tool gateway rejects any graph id outside its executing session before
it reaches an owner method, and returns no graph projection on that refusal.

Task and edge mutations append new facts rather than rewriting history. The
owner permits them only for nonterminal, unstarted tasks; it rejects a mutation
that would alter terminal evidence, create a cycle, or make a running task's
dependency meaning ambiguous.

## Recovery

Restart rebuilds graph state from the ledger. Phase B reconciles only a
persisted execution binding with its existing owner; ambiguous external work is
blocked with a sanitized recovery reason and never replayed by TaskGraph. The
AgentInvocation owner first converts every persisted nonterminal run to a
durable ambiguous-recovery failure, including a non-delegated invocation; it
never restarts one. Recovery selects the latest durable run for an invocation,
so an older terminal run cannot hide a later ambiguous run. TaskGraph then
re-reduces each binding from that latest owner fact, so a successful worker
terminal fact is repaired after a separate graph-transition append failure
without replaying the worker. If either owner
recovery or graph reconciliation cannot append its required fact, kernel
initialization fails closed instead of presenting an unresolved graph as live.

## Phase C projection

The kernel exposes only authenticated session-scoped graph reads in the first
operator projection slice: `GET /sessions/{session_id}/task-graphs` and
`GET /task-graphs/{graph_id}`. The first returns only graphs belonging to that
session; the second returns the ledger-reduced graph. Neither route creates,
updates, starts, or binds a task. Desktop may render title, status, dependency,
blocked reason, and evidence reference, but it remains a reader in this slice.

`task_graph_edit` is a model-visible kernel-control tool for the ordinary parent
session gateway and is never exposed through an invocation capability grant. Each call carries one named
topology operation, or `bind_invocation` with only opaque graph, node, and
already-admitted invocation ids; it carries no execution, authority, provider,
role, task, or tool field. The tool dispatches to the existing TaskGraph owner
methods under their validation lock, then returns the resulting opaque
identifiers or projection. A rejected proposal writes no graph fact; a leaf
worker is denied by its grant before any owner call.

## Reference alignment

Codex's `codex-rs/core/src/tools/handlers/multi_agents_v2/spawn.rs` accepts an
explicit parent `spawn_agent` call, then delegates capacity reservation and
thread creation to `codex-rs/core/src/agent/control/spawn.rs`; this is a
dispatcher path, not a dependency-graph owner. Reasonix's
`internal/agent/task.go` likewise creates a focused sub-agent only from the
parent's explicit `task` tool call. Neither local reference owns a durable DAG.
Genesis aligns by keeping `delegate_worker` as the explicit dispatcher and
TaskGraph as a separate ledger/project owner that only binds and reduces its
stable references. It rejects in-memory task queues, free model overrides, and
graph-derived authority.
