# Requirement: User-space Workflow Runtime

- **Status:** approved
- **Owner:** user-space workflow runtime
- **Scope:** developer-authored workflow definitions, fixed workflow runs, node outcomes, deterministic edge transitions, run logs, and kernel boundary protection

## Background

Genesis needs production workflows that are stricter than an LLM plan and more
structured than a skill. A skill can tell the model how to do a task. A tool can
perform one governed effect. A TaskGraph can represent dynamic work topology.
None of those should own a fixed production process such as:

```text
draft file
  -> machine check
      fail -> revise file
      pass -> LLM review
          fail -> revise file
          pass -> finish
```

The workflow runtime fills that gap. It treats a workflow as a developer-authored
execution contract. The runtime executes the graph that developers shipped. A
node may contain an LLM invocation, a machine check, a human decision, a tool
operation, a resource transform, or a subworkflow, but the node cannot modify the
workflow definition while it runs.

## Production Target

The user-space workflow runtime is a first-class Genesis platform owner for
fixed production processes. It owns workflow definition loading, definition
identity, run admission, node state, edge transition, retry policy, run logs,
artifact refs, evidence refs, and operator inspection.

The workflow runtime is user-space. It is not Agent Kernel core. When a node
needs model execution, tool execution, resources, jobs, budgets, memory, or audit
facts, it calls kernel public primitives and receives kernel-owned refs or
projections. It cannot write kernel ledger facts, tool results, memory truth,
provider context, approval decisions, or checkpoint facts directly.

## Users And Roles

Developers define workflow contracts: nodes, allowed outcomes, edge rules,
node-level capability requests, retry policy, budget policy refs, and artifact
requirements.

Operators start, inspect, pause, resume, cancel, and compare workflow runs. They
use run logs to decide how developers should revise the next workflow definition.

Applications can submit workflow run requests and consume workflow projections.
They do not rewrite the workflow graph during a run.

LLM-backed nodes perform semantic work inside the node contract. They can produce
content, reasons, proposed fixes, review comments, and outcome proposals allowed
by the node schema. They cannot add nodes, change edges, replace checkers,
select tools outside the node grant, or broaden authority.

Genesis Kernel owns execution authority, model invocations, tool effects,
resource admission, job state, BudgetLease, memory truth, audit, checkpoint, and
typed kernel facts.

## Core Semantics

Skill, tool, TaskGraph, and workflow are orthogonal:

- Skill is instruction and knowledge. Loading a skill does not bind or restrict
  the tools available to the model, and tool availability does not imply a skill.
- Tool is one kernel-governed effect or read. It does not own a process graph.
- TaskGraph is dynamic work topology. An LLM, application, or orchestrator may
  propose nodes, dependencies, edits, and status changes subject to owner
  validation.
- Workflow is a developer-authored execution automaton. A run follows the
  definition snapshot it was admitted with.

`WorkflowConfig` is the developer-authored source file for a workflow. It may be
YAML, JSON, or another reviewed declarative format, but it is not a script
language and it cannot execute code. It declares nodes, edges, allowed outcomes,
terminal outcomes, loop limits, budget policy refs, capability requests, and
executor refs. Changing config changes the workflow for future runs only.

`WorkflowDefinition` is the compiled and normalized execution contract produced
from `WorkflowConfig` before execution. It contains node specs, edge rules,
allowed outcomes, node input/output schemas, node capability requests, budget
policy refs, retry policy, artifact requirements, terminal states, and a
flowchart projection. Only developers or operator-approved deployment tooling
may create or replace a config and compile it into a definition.

Every workflow must have a corresponding flowchart. The flowchart is the human
review projection of the executable definition: nodes, declared outcomes, edges,
terminal states, loops, approval waits, and subworkflow calls must be visible.
The config is the executable graph source. The compiled node and edge spec is
the runtime source of truth. The flowchart is generated from that source; if a
developer writes the diagram by hand, deployment tooling must validate it
against the definition hash before the workflow can run. A stale or mismatched
flowchart blocks admission because it would make operators review a different
process from the one the runtime will execute.

`WorkflowRun` binds to one definition identity and definition hash at admission.
That binding does not change. Fixing or optimizing the process creates a new
workflow definition for future runs. Existing runs resume or replay against the
definition snapshot they started with.

`WorkflowNode` executes as a black-box step under its node contract. Initial node
kinds are:

- `llm_invocation`
- `machine_check`
- `human_approval`
- `tool_operation`
- `resource_transform`
- `connector_action`
- `subworkflow`

A node returns only declared outputs: outcome, evidence refs, artifact refs,
diagnostic summary, resource refs, and bounded structured data. It cannot mutate
the workflow definition, replace itself, create undeclared edges, or mint kernel
facts.

`WorkflowEdge` is a deterministic transition rule over node outcome and run
state. Unknown outcomes fail closed. A node may explain why it returned `fail`,
`pass`, `needs_revision`, `blocked`, or another declared outcome, but it cannot
choose an undeclared transition.

`WorkflowRunLog` is optimization evidence, not a self-modification channel. It
records node duration, failed checks, loop counts, retries, BudgetLease usage,
tool calls, artifact revisions, human approval latency, checker disagreement,
and terminal cause. Developers use logs to ship a new definition version; the
running workflow does not edit itself.

`WorkflowFlowchart` is a read model for humans and review tools. It does not
grant authority, does not decide transitions, and does not replace the
definition spec. It exists so reviewers can inspect the process before a run and
operators can understand where a run currently sits.

## TaskGraph Relationship

TaskGraph and Workflow solve different problems.

TaskGraph records what work exists and how it relates. It is dynamic, revisable,
and may be shaped by LLM or application proposals after owner validation.

Workflow defines how a class of work is allowed to execute. It is
developer-authored, fixed during a run, and contract-bound.

A workflow run may create or update TaskGraph nodes as evidence or progress, and
a TaskGraph node may point to a workflow run that is handling the work. That link
does not merge the owners. TaskGraph remains work topology; Workflow remains
execution contract.

## Non-Goals

- Do not put Workflow Runtime inside kernel core.
- Do not let LLM nodes create or replace workflow definitions during a run.
- Do not make workflow config a general script or expression language.
- Do not use workflow definitions to grant kernel authority. Kernel
  CapabilityGrant and BudgetLease still control execution.
- Do not bind skills to tools or tools to skills through workflow configuration.
- Do not let applications or workflow nodes assemble provider context directly.
- Do not turn TaskGraph into a fixed workflow engine.
- Do not turn Workflow into a dynamic LLM planner.

## Phased Delivery

Phase A: Contract and dry-run projection.

- Define workflow config, compiled workflow definitions, run identity, node
  outcomes, edge transitions, flowchart projection, and run logs as user-space
  contracts.
- Provide a validator that rejects graph mutation rights, unknown outcomes,
  undeclared node ids, edge cycles without an explicit loop limit, and
  node-requested authority outside the definition.
- Reject workflow definitions whose flowchart is missing, stale, or inconsistent
  with the executable node/edge spec.
- Generate `definition_hash` from the canonical compiled definition, not from
  comments, formatting, or diagram layout hints.
- Prove no kernel imports workflow runtime internals.

Phase B: Local deterministic runner.

- Execute in-memory or file-backed workflow runs with deterministic node
  transition.
- Support mock node executors, machine check nodes, human approval placeholders,
  bounded loop counts, cancellation, and run inspection.
- Persist run logs as workflow owner facts, not kernel ledger facts.

Phase C: Kernel primitive integration.

- Let node executors call kernel public primitives for LLM invocations, tool
  operations, resources, jobs, BudgetLease, and audit refs.
- Store only kernel refs/projections in workflow evidence.
- Prove nodes cannot mint tool results, memory truth, provider context, or
  approval facts.

Phase D: Optimization and versioning.

- Add run comparison, failure-rate summaries, budget summaries, node latency,
  artifact churn, and reviewer disagreement reports.
- Support developer-authored definition replacement for future runs while old
  runs keep their original definition hash.

## Acceptance Criteria

- A workflow run binds to one definition hash and cannot switch definition mid-run.
- Workflow config compiles into a canonical definition before any run starts.
- Each workflow definition has a flowchart projection that matches the
  executable node/edge spec.
- A missing, stale, or mismatched flowchart blocks workflow admission.
- LLM-backed nodes cannot add nodes, change edges, change allowed outcomes, or
  broaden their tool/capability set.
- Unknown outcomes fail closed and do not pick a best-effort edge.
- A loop runs only through declared edges and within declared retry or loop
  limits.
- Run logs record enough evidence for a developer to revise the next workflow
  definition without letting the running workflow self-modify.
- Workflow docs and code keep TaskGraph dynamic and Workflow fixed during a run.
- Kernel boundary tests prove `internal/kernel` does not import workflow runtime
  packages and workflow runtime does not write kernel facts directly.
