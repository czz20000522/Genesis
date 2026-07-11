# Workflow Runtime Phase A Implementation Plan

- **Requirement:** `docs/applications/workflow-runtime-requirement.md`.
- **Design:** `docs/applications/workflow-runtime-design.md`.
- **Issue:** `APP-WORKFLOW-RUNTIME-PHASE-A-20260711`.

## Reference Scan

Reasonix `internal/control/controller.go` owns one transport-agnostic run state,
serializes approval waits, and resumes from persisted session state; its
controller tests exercise cancellation and approval unblocking. Genesis aligns
with explicit run state and durable recovery, but does not reuse a chat
controller as a fixed-process owner.

Codex `codex-rs/app-server/src/request_processors/thread_processor.rs` exposes
thread-scoped shell work rather than a developer-authored workflow definition.
Genesis intentionally differs: a Workflow must compile a fixed declarative graph
before admission, and its nodes cannot acquire authority from the definition.

## Phase A: Compile And Inspect

Add a small `internal/applications/workflowruntime` package. It accepts JSON or
YAML workflow config, validates only the fixed-graph subset, produces a
canonical definition hash, and generates a Mermaid flowchart projection.

Validation rejects duplicate or unknown node ids, undeclared outcomes, missing
transitions, unbounded cycles, free-form executables, and an absent or mismatched
flowchart. It has no runner, kernel client, scheduler, persistence, or UI.

Tests prove stable hashes across source key ordering, unknown outcomes fail
closed, cycles require an explicit bound, and generated flowcharts contain each
declared edge and terminal state.

## Phase B: Deterministic Local Runner

Add file-backed runs, serial mock executors, declared retries, approval waits,
pause/resume/cancel, and run logs. A run binds to the compiled definition
snapshot; restart resumes only that snapshot.

## Phase C: Kernel Primitive Calls

Introduce explicitly registered node executors that call public kernel APIs and
retain only returned refs or projections. Do not import kernel internals or
allow a workflow definition to grant tools, credentials, provider context, or
approval decisions.

## Deferred Real-Workflow Evidence

No actual repeated workflow has been selected. Video transcript extraction
remains a general Skill or future Workflow, not a Phase A fixture. Do not claim
Stage 6 complete until an operator supplies a real repeated process with its
steps, terminal outcomes, artifact contract, and approval points.
