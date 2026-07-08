# Kernel Agent Invocation Implementation Plan

> **For agentic workers:** implement admission truth before execution. Do not
> spawn child models in Phase A.

## Requirement And Design

- Requirement: `docs/requirements/kernel-agent-invocation.md`
- Design: `docs/design/kernel-agent-invocation.md`
- BDD: `features/kernel/agent_invocation.feature`

## Phase A: Invocation Admission Ledger Fact

**Deliverable:** kernel methods can admit and replay agent invocation records
with validated tool-name grants.

**Files:**

- Add: `internal/kernel/agent_invocation_types.go`
- Add: `internal/kernel/agent_invocation.go`
- Test: `internal/kernel/agent_invocation_test.go`
- Modify: `internal/kernel/event_types.go`

**Red lines:**

- Do not run a model or call a provider.
- Do not create jobs, task graphs, or workflow records.
- Do not infer authority from role/profile refs.
- Do not expose sandbox profiles, permission profiles, workspace roots,
  provider routes, credentials, or raw prompts.

- [x] Step 1: Add failing admission and replay tests.

  Cover root admission, replay, idempotency, role/profile no-authority, policy
  denial, child subset, and child exceeding parent.

- [x] Step 2: Add invocation types and event data.

  Define request, grant, projection, validation helpers, and ledger payload.

- [x] Step 3: Implement admission and replay.

  Validate requested grants against tool registry and parent invocation grants;
  append `agent_invocation.admitted`.

- [x] Step 4: Verify.

  Run focused tests, then:

  ```powershell
  git diff --check
  go test ./... -count=1
  go build ./...
  ```

## Phase B: Invocation-Scoped Tool Filtering

Before actual child model execution, make ToolGateway able to intersect a tool
manifest with an invocation's admitted grant.

Delivered:

- [x] `ToolGatewayForInvocation` loads an admitted invocation and returns a
  grant-scoped gateway.
- [x] Invocation-scoped manifests and capability projections expose only
  granted tools.
- [x] Preparing a tool outside the admitted grant returns repairable
  `capability_grant_tool_not_allowed` feedback before execution.

## Phase C: HTTP Transport Exposure

Expose admission and read projections to applications without moving authority
logic into HTTP.

Delivered:

- [x] `POST /agent-invocations` delegates admission to
  `AdmitAgentInvocation`.
- [x] `GET /agent-invocations/{invocation_id}` delegates replay to
  `AgentInvocation` and maps unknown ids to `404`.
- [x] `GET /sessions/{session_id}/agent-invocations` delegates session list
  replay to `AgentInvocations`.
- [x] Transport tests cover admit, read, list, and not-found behavior.

## Phase D: Child Run Execution

Add a bounded synchronous child-run primitive that uses admitted invocation ids,
focused run input, invocation-scoped tool grants, and final-only result
delivery.

**Red lines:**

- Do not add a task graph or workflow runtime.
- Do not add model-profile resolution; Phase D uses the current kernel provider
  and keeps `agent_profile_ref` semantic.
- Do not add background execution, child concurrency pools, recursive
  delegation, or parent notification protocols.
- Do not append child intermediate provider/tool rounds as parent conversation
  history.
- Do not expose provider routes, credentials, sandbox profiles, permission
  profiles, workspace roots, raw prompts, or full child transcripts.

- [ ] Step 1: Add failing direct kernel tests.

  Cover successful run from an admitted invocation, unknown invocation,
  already-running guard, idempotent terminal replay, grant-scoped tool denial,
  provider failure redaction, and final-only parent projection.

- [ ] Step 2: Add run request, result projection, and event payload types.

  Define focused input items, optional idempotency key, run status, sanitized
  failure class, usage accounting, and replay behavior for started and terminal
  events.

- [ ] Step 3: Implement synchronous run owner methods.

  Validate invocation and caller, append `agent_invocation.run_started`, call the
  current provider with fresh child context, route child tool calls through
  `ToolGatewayForInvocation`, and append completed or failed terminal facts.

- [ ] Step 4: Verify.

  Run focused tests, then:

  ```powershell
  git diff --check
  go test ./... -count=1
  go build ./...
  ```
