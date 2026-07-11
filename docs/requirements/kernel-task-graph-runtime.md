# Requirement: TaskGraph Runtime

- **Status:** approved for phased implementation.
- **Owner:** Genesis Kernel task-graph authority.

## Production Goal

TaskGraph is the durable owner of a long-running user objective's nodes,
dependencies, state, evidence, and recovery checkpoints. A parent may propose a
graph, but only the owner validates and persists it. A graph node references a
bounded `AgentInvocation`, `Workflow`, job, resource, or approval; it never
creates authority, grants tools, or becomes a second workflow engine.

## Semantics

- Graph and node identity, dependency edges, lifecycle state, evidence refs,
  blocked reason, and recovery checkpoint are ledger facts.
- A node is `proposed`, `ready`, `running`, `waiting`, `blocked`, `completed`,
  `failed`, or `cancelled`; terminal nodes never transition again.
- A node becomes `ready` only when every dependency completed successfully.
- Failure, cancellation, approval wait, or missing referenced execution leaves
  downstream nodes non-ready with an explainable reason; the owner never
  invents completion or replays unknown side effects.
- Parent proposals name only allowed node references and dependency edges. The
  owner rejects missing references, cycles, duplicate edges, and illegal state
  transitions before appending a fact.
- TaskGraph, Workflow, and AgentInvocation cooperate by stable references only.
  Workflow owns fixed procedure; AgentInvocation owns bounded agent work;
  TaskGraph owns dependency topology and project progress.

## Non-goals

- No LLM-generated arbitrary scheduler, hidden automatic retries, worker
  mailbox, provider routing, or new tool grant.
- No graph layout editor, cloud sync, collaborative multi-user ownership, or
  memory recall behavior.
- No direct execution of a node before its referenced owner accepts it.

## Phases

### Phase A: Durable graph facts and validation

Persist graph/node/edge facts, validate DAG and lifecycle transitions, project
ready/blocked explanations, and recover the graph read model after restart.
Do not schedule execution.

### Phase B: Referenced execution and recovery

Let the owner start only an already-authorized referenced invocation/workflow,
persist the linkage before execution, and fail closed when restart cannot prove
whether an external effect started.

### Phase C: Operator and parent projection

Expose progress, blocked reason, evidence, and controlled parent proposal
results. The desktop remains a reader/submission shell.

## Acceptance

1. A valid DAG survives kernel restart with identical node state and evidence.
2. Cycles, unknown references, invalid transitions, and duplicate dependency
   edges append no graph fact.
3. A node is ready only after all dependencies have terminal success.
4. Waiting approval, failure, and cancellation explain why dependents did not
   run.
5. A graph never grants provider, tool, workspace, or permission authority.
6. Restart recovery never replays an ambiguous side effect.
7. User and parent projections show progress and evidence without raw provider
   streams, credentials, or sandbox internals.
