# Design: TaskGraph Runtime

- **Requirement:** `docs/requirements/kernel-task-graph-runtime.md`.
- **Owner:** a flat Kernel task-graph owner backed by the event ledger.

## Boundary

The owner persists project task topology and reduces optional execution-owner
projections to task state. It does not execute tools or call providers directly;
AgentInvocation, Workflow, job, approval, and external wait owners retain that
work. Parent and user can revise the graph in either planning direction.

## Facts

`task_graph.created`, `task_graph.node_added`, `task_graph.edge_added`, and
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

## Recovery

Restart rebuilds graph state from the ledger. Phase B reconciles only a
persisted execution binding with its existing owner; ambiguous external work is
blocked with a sanitized recovery reason and never replayed by TaskGraph.

## Reference alignment

Codex's local spawned-agent control plane persists parent-child lifecycle and
capacity activity; Reasonix creates isolated bounded tasks and returns final
text. Neither local reference owns a durable dependency graph. Genesis aligns
on explicit lifecycle facts and bounded references, and rejects in-memory task
queues, free model overrides, and graph-derived authority.
