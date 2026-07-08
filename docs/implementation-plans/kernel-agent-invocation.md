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

## Phase C: Child Run Execution

Add a bounded child-run primitive that uses admitted invocation ids, separate
context scope, model gateway profile resolution, cancellation, and result
delivery.
