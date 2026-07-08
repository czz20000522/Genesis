# Implementation Plan: Parent-Led Worker Runtime Phase C

## Requirement And Design

- Requirement: `docs/requirements/kernel-parent-worker-runtime.md`
- Design: `docs/design/kernel-parent-worker-runtime.md`
- Issue: `docs/operations/kernel-issues.md#kernel-parent-worker-child-conversation-20260708`
- BDD: `features/kernel/parent_worker_runtime.feature`

## Reference Scan

- Codex references inspected:
  - `codex-rs/core/src/tools/handlers/multi_agents/wait.rs`
  - `codex-rs/core/src/tools/handlers/multi_agents/spawn.rs`
- Reasonix references inspected:
  - `internal/agent/task.go`
  - `internal/skill/index.go`
  - `internal/agent/hooks_test.go`
- Alignment:
  - Follow Codex by exposing sub-agent status/final result through an explicit collaboration surface instead of merging it into the parent transcript.
  - Follow Reasonix by keeping subagent tool activity and final answer observable while returning only bounded final output to the parent.
- Intentional differences:
  - Genesis Phase C exposes a read-only child conversation projection over existing `AgentInvocation` run events. It does not implement UI rendering or task graph layout.

## Phase C

- Deliverable: expose child conversation projection by invocation id through kernel and HTTP.
- Red lines:
  - Do not store or expose raw focused prompts.
  - Do not expose provider streams or raw tool traces.
  - Do not add TaskGraph nodes, edges, layout, or scheduling.
- Tests:
  - completed run projects role, status, final, usage, model input kinds, context scope, tool set, and evidence refs.
  - projection omits focused prompt and raw tool trace.
  - HTTP GET returns projection and 404 for unknown invocation.
- Evidence:
  - `go test ./internal/kernel -run "Test(AgentInvocationChildConversation|HTTPAgentInvocationChildConversation|ArchitectureBoundary)" -count=1`
  - `go test ./internal/kernel -count=1`
  - `git diff --check`
- Still short of production:
  - desktop/WebUI rendering.
  - TaskGraph requirement, layout, and visualization.

## Retirement Criteria

Delete this plan and move the issue to retirement evidence when the projection API, HTTP route, tests, and closing gate pass.
