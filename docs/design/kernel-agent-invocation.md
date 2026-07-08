# Design: Agent Invocation Admission

- **Requirement:** `docs/requirements/kernel-agent-invocation.md`
- **Owner:** Genesis Kernel invocation authority boundary.

## Reference Scan

Codex:

- `codex-rs/core/src/tools/handlers/multi_agents.rs` translates model tool
  calls into `AgentControl` operations instead of letting role text create
  agents directly.
- `codex-rs/core/src/agent/control.rs` records parent-child spawn edges,
  watches child status, notifies parents on completion, and preserves spawned
  thread metadata.
- `codex-rs/core/src/tools/handlers/agent_jobs.rs` caps worker concurrency,
  tracks job item state, recovers running items, and creates workers through the
  same agent control surface.
- `codex-rs/core/src/session/multi_agents.rs` separates root-agent and
  subagent usage hints by session source.

Reasonix:

- `internal/agent/task.go` implements a `task` tool that runs a sub-agent in a
  fresh session, filters inherited tools, strips meta-tools to prevent recursive
  delegation, and returns only the final answer to the parent.
- `internal/agent/task_test.go` proves the sub-agent sees a fresh prompt,
  receives only filtered tools, and can use a configured model profile.
- `internal/agent/coordinator.go` keeps planner and executor as separate
  sessions and hands off a plan without mixing role authority into provider
  routing.
- `internal/agent/branch.go` records parent/child conversation metadata beside
  session files rather than inferring topology from text.

Genesis alignment:

- Genesis should follow Codex by making child execution an admitted control
  operation with parent-child metadata.
- Genesis should follow Reasonix by filtering child tools and treating the
  returned result as bounded, not as shared session history.
- Genesis intentionally starts with an admit-only ledger fact because actual
  child execution also needs scheduling, context, model profile, result
  delivery, and cancellation contracts.

## Owner Boundary

Owner: invocation authority boundary in the kernel package.

Related owners:

- ToolGateway owns tool execution admission and results.
- Model Gateway owns provider/profile resolution.
- Job runtime owns background execution state.
- Applications own role taxonomy, task graph meaning, and orchestration
  strategy.

Non-owners:

- Provider adapters do not create invocation facts.
- Role labels do not grant authority.
- Task graph nodes do not grant authority.
- Skills do not grant authority.

## Data Flow

Phase A:

1. Application calls `AdmitAgentInvocation`.
2. Kernel validates session, principal, refs, requested grant, and optional
   parent relationship.
3. Kernel checks requested tools against the default tool registry and current
   `ToolPolicy`.
4. Kernel checks child grants are a subset of the parent grant.
5. Kernel appends `agent_invocation.admitted`.
6. Projection APIs replay invocations from ledger events.

Future execution:

1. Application uses an admitted invocation id to start a child run.
2. Model Gateway resolves model profile from application-owned profile refs.
3. ToolGateway enforces the invocation grant before tool calls.
4. Result delivery attaches a bounded output projection to the invocation.

## Event Shape

Event type: `agent_invocation.admitted`.

Payload:

```json
{
  "invocation_id": "invocation_...",
  "session_id": "session-a",
  "parent_invocation_id": "invocation_parent",
  "principal": "application:desktop",
  "agent_profile_ref": "agent_profile:reviewer",
  "capability_grant": {
    "tool_names": ["resource_read", "source_read"]
  },
  "context_scope": "diff",
  "status": "admitted",
  "idempotency_key": "delegation-1",
  "admitted_at": "2026-07-08T00:00:00Z"
}
```

The payload must not include workspace roots, sandbox profiles, approval refs,
provider routes, credentials, raw prompts, or model outputs.

## Grant Semantics

Phase A grants are tool-name whitelists:

- names are normalized, de-duplicated, sorted, and validated against the kernel
  registry;
- each tool must be allowed by `ToolPolicy`;
- child grants must be subsets of parent grants;
- empty grants are allowed and mean the invocation may run without tools in a
  later phase.

This is intentionally smaller than the production CapabilityGrant target. Later
phases may add budget leases, context sources, resource refs, credential scopes,
and result delivery.

## Replay

Replay loads all `agent_invocation.admitted` events in ledger order and builds a
map by invocation id. A duplicate id with different semantic fields is a replay
error. A duplicate id with identical fields is idempotent.

## Observability

Phase A exposes direct kernel methods:

- `AdmitAgentInvocation`
- `AgentInvocation`
- `AgentInvocations`

The HTTP transport is a thin application-facing surface over those methods:

- `POST /agent-invocations`
- `GET /agent-invocations/{invocation_id}`
- `GET /sessions/{session_id}/agent-invocations`

The transport owns only auth, JSON decode, route parsing, delegation, and JSON
encoding. Capability-grant validation, parent relationship checks, idempotency,
and replay remain in the kernel owner methods.
