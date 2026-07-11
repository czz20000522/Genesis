# Design: TaskGraph Runtime

- **Requirement:** `docs/requirements/kernel-task-graph-runtime.md`.
- **Owner:** a flat Kernel task-graph owner backed by the event ledger.

## Boundary

The owner persists graph topology and role-task node proposals. It resolves the
configured parent binding and calls the existing `AgentInvocation` admission
and run primitives only when a node is ready. It does not execute tools or call
providers directly; `AgentInvocation` owns that work. Workflow, job, resource,
and approval remain their existing owners.

## Facts

`task_graph.created`, `task_graph.node_added`, `task_graph.edge_added`,
`task_graph.node_started`, and `task_graph.node_transitioned` carry opaque ids,
role/task proposal, invocation linkage, state, reason class, and evidence refs.
The read projection is reconstructed only from these facts. A transition
validates the current reduced graph while the graph mutex is held.

## Validation

The owner rejects self edges, duplicate edges, missing endpoints, cycles, and
any transition outside the explicit state table. A proposal contains role/task
only; it cannot name a provider or tool. At readiness, the owner resolves the
configured parent binding and calls `AdmitWorkerInvocationFromRole`, then
persists invocation linkage before starting it. A node is ready when all
predecessors completed.

## Recovery

Restart rebuilds graph state from the ledger. Phase B resumes only a persisted
invocation linkage with an unambiguous nonterminal checkpoint; otherwise it
marks the node blocked with a sanitized recovery reason. A proposal with no
created invocation may be admitted once when it is ready; started provider work
is never replayed by TaskGraph.

## Reference alignment

Codex's local spawned-agent control plane persists parent-child lifecycle and
capacity activity; Reasonix creates isolated bounded tasks and returns final
text. Neither local reference owns a durable dependency graph. Genesis aligns
on explicit lifecycle facts and bounded references, and rejects in-memory task
queues, free model overrides, and graph-derived authority.
