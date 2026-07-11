# Design: TaskGraph Runtime

- **Requirement:** `docs/requirements/kernel-task-graph-runtime.md`.
- **Owner:** a flat Kernel task-graph owner backed by the event ledger.

## Boundary

The owner persists graph topology and reduces referenced execution projections
to node state. It does not execute tools, call providers, or define workflow
steps. `AgentInvocation`, `Workflow`, job, resource, and approval remain their
existing owners.

## Facts

`task_graph.created`, `task_graph.node_added`, `task_graph.edge_added`, and
`task_graph.node_transitioned` carry opaque ids, a sanitized target reference,
state, reason class, and evidence refs. The read projection is reconstructed
only from these facts. A transition validates the current reduced graph while
the graph mutex is held.

## Validation

The owner rejects self edges, duplicate edges, missing endpoints, cycles, and
any transition outside the explicit state table. It accepts a node reference
only after the referenced owner has admitted it; graph proposal text has no
authority effect. A node is ready when all predecessor nodes are completed.

## Recovery

Restart rebuilds graph state from the ledger. Phase A makes no execution call.
Phase B may resume only a persisted linkage whose referenced owner has an
unambiguous nonterminal checkpoint; otherwise it marks the node blocked with a
sanitized recovery reason.

## Reference alignment

Codex's local spawned-agent control plane persists parent-child lifecycle and
capacity activity; Reasonix creates isolated bounded tasks and returns final
text. Neither local reference owns a durable dependency graph. Genesis aligns
on explicit lifecycle facts and bounded references, and rejects in-memory task
queues, free model overrides, and graph-derived authority.
