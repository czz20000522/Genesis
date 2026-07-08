# Requirement: Agent Invocation Admission

- **Status:** approved for phased implementation.
- **Owner:** Genesis Kernel invocation authority boundary.
- **Scope:** admit and replay bounded model-backed invocation identities and their capability grants.

## Background

Genesis treats role labels as application semantics, not authority. A future
parent/coordinator can propose a reviewer, coder, planner, or child agent, but
that label must not grant tools, provider routes, credentials, workspace write
access, or budget. The kernel needs an execution identity that records what was
actually admitted before any child or delegated run can consume context, call
tools, or spend budget.

The current kernel can submit turns, run tools, and bind model profiles, but it
does not yet have a runtime fact for a bounded child invocation. Without that
fact, a later multi-agent feature would be tempted to infer authority from
roles, prompts, task graph nodes, or provider names.

## Production Target

Genesis supports admitted `AgentInvocation` records:

- each invocation has a kernel-issued `invocation_id`;
- an invocation belongs to one `session_id`;
- an invocation may reference a parent invocation in the same session;
- the kernel validates a requested `CapabilityGrant` before writing the
  invocation fact;
- a child grant must be a subset of the parent grant;
- a root grant must be allowed by the current kernel `ToolPolicy`;
- role/profile refs are recorded as semantic labels only and do not grant
  authority;
- admitted invocation facts replay deterministically from the event ledger;
- projections expose semantic refs and grant summaries, not permission profiles,
  sandbox profiles, provider credentials, or raw prompts.

## Users And Roles

Application or orchestrator:

- proposes a focused model-backed invocation and requested tool grant;
- may attach role/profile refs and source refs;
- receives an admitted invocation projection or a refusal.

Kernel:

- validates identity, parent relationship, and capability grant;
- writes the admitted invocation event;
- replays invocation projections from the ledger.

LLM:

- may ask an application to delegate, but model text never grants authority.

Provider adapter:

- is not involved in Phase A admission.

## Semantics

Phase A admits invocation facts only:

1. `AdmitAgentInvocation` receives `session_id`, optional
   `parent_invocation_id`, `principal`, optional `agent_profile_ref`, requested
   `capability_grant`, optional `context_scope`, and optional idempotency key.
2. `principal` is a kernel authority string naming the caller, such as
   `operator:test` or `application:desktop`.
3. `capability_grant.tool_names` is a list of registered kernel tool names.
4. Unknown, duplicate, or disallowed tool names are refused.
5. Root invocations can request only tools allowed by the current `ToolPolicy`.
6. Child invocations can request only a subset of their parent invocation's
   tool names.
7. Admission writes `agent_invocation.admitted`.
8. Repeated admission with the same session and idempotency key returns the
   existing invocation.
9. Phase A does not run a model, create a turn, schedule a job, or call a tool.

## Failure Semantics

- Missing session: `session_id is required`.
- Invalid session/idempotency/principal/ref: validation error.
- Unknown parent: `parent_invocation_not_found`.
- Parent in another session: `parent_invocation_session_mismatch`.
- Unknown requested tool: `capability_grant_unknown_tool`.
- Tool blocked by current policy: `capability_grant_tool_not_allowed`.
- Child asks beyond parent grant: `capability_grant_exceeds_parent`.
- Competing facts for the same invocation id: replay error.

## Non-Goals

- No actual sub-agent execution in Phase A.
- No task graph owner or workflow runtime.
- No model selection, model refresh, or provider routing logic.
- No role taxonomy owned by the kernel.
- No permission profile or sandbox profile exposure in model-visible payloads.
- No automatic child invocation from model text.

## Acceptance Criteria

1. The kernel can admit a root invocation with a registered, policy-allowed tool
   grant and replay it from the ledger.
2. Reusing an idempotency key returns the original invocation without writing a
   competing fact.
3. A root invocation cannot request a write tool while `ToolPolicy` is `plan`.
4. A child invocation can request a subset of its parent grant.
5. A child invocation cannot request tools outside its parent grant.
6. Role/profile refs are recorded but do not influence authorization.
7. Projections do not expose workspace roots, sandbox profiles, approval refs,
   credentials, or provider routes.
