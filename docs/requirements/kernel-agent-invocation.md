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
- an admitted invocation can be run as a bounded child model execution only by
  an explicit application call;
- child execution starts from a fresh, scoped model context rather than the
  parent's full conversation history;
- child tool access is limited to the invocation's admitted grant;
- parent-visible output is the child's bounded final result, not its full
  intermediate transcript or tool trace;
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

Phase D runs admitted invocations synchronously:

1. `RunAgentInvocation` receives an admitted `invocation_id`, caller principal,
   focused input items, and an optional idempotency key.
2. The invocation must already exist and be `admitted`.
3. The run principal is validated independently from the admission principal.
4. The child model request is built from the focused input items plus bounded
   context implied by `context_scope`; it does not inherit the parent's full
   same-session conversation history by default.
5. The run uses the current kernel provider in Phase D. `agent_profile_ref`
   remains a recorded semantic ref until a separate model-profile resolution
   slice is approved.
6. Tool calls made during the child run go through
   `ToolGatewayForInvocation(invocation_id)`.
7. The run writes started and terminal invocation facts to the ledger.
8. The terminal projection returns only final text, model, usage, status, and
   sanitized failure class; intermediate provider/tool rounds remain child-run
   evidence and are not appended to the parent chat transcript.
9. Repeated run with the same invocation and idempotency key returns the
   existing terminal projection.

## Failure Semantics

- Missing session: `session_id is required`.
- Invalid session/idempotency/principal/ref: validation error.
- Unknown parent: `parent_invocation_not_found`.
- Parent in another session: `parent_invocation_session_mismatch`.
- Unknown requested tool: `capability_grant_unknown_tool`.
- Tool blocked by current policy: `capability_grant_tool_not_allowed`.
- Child asks beyond parent grant: `capability_grant_exceeds_parent`.
- Competing facts for the same invocation id: replay error.
- Run unknown invocation: `agent_invocation_not_found`.
- Run already running invocation: `agent_invocation_already_running`.
- Run completed invocation with a different idempotency key:
  `agent_invocation_already_terminal`.
- Child provider failure: sanitized `provider_unavailable` or provider failure
  class, with no provider credentials or raw transport diagnostics.

## Non-Goals

- No actual sub-agent execution in Phase A-C.
- No task graph owner or workflow runtime.
- No model selection, model refresh, or provider routing logic in Phase D.
- No role taxonomy owned by the kernel.
- No permission profile or sandbox profile exposure in model-visible payloads.
- No automatic child invocation from model text.
- No background child execution, child concurrency pool, recursive delegation
  cap, or parent notification protocol in Phase D.

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
8. Authorized HTTP routes can admit, read, and list invocations by delegating to
   kernel owner methods without duplicating authorization semantics.
9. The kernel can run an admitted invocation from a focused prompt and record a
   terminal invocation result.
10. A child invocation cannot call tools outside its admitted grant during run.
11. Child final output is returned as bounded invocation result and is not
    projected as parent conversation history.
12. Child run idempotency returns the original terminal result without writing a
    competing fact.
