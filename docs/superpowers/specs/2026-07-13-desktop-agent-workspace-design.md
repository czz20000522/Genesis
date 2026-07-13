# Genesis Desktop Agent Workspace Design

## Purpose

Replace the current chat-first desktop surface with a Codex-inspired AI agent
workspace. The supplied Codex screenshot defines principles—calm workspace
hierarchy, compact navigation, a dominant task composer, and contextual
execution detail—not a component-by-component copy target.

## Decisions

1. The desktop has a top bar, 248px navigation rail, central workspace, and a
   demand-opened inspector as its third column.
2. Project is a durable top-level container. Its child sessions appear beneath
   it; Task and Chat remain distinct durable session classes.
3. The central unit is a task workspace. Transcript facts are recast as
   activity and output blocks but retain their kernel-owned event identity.
4. The composer remains visible and is the main action surface. Attachment and
   model controls stay inside it. Model selection stays session-scoped.
5. Right-side details contain approvals, worker/task-graph views, source
   material, diagnostics, and deep timeline detail only when invoked.
6. Header data is fail-closed: show only real session/model/connectivity/root
   data; do not pretend a Git branch or agent phase exists.

## Current-to-target mapping

| Current surface | Target surface |
| --- | --- |
| `ConversationPane` transcript-first view | `AgentWorkspace` composition with timeline + composer |
| Chat bubbles | activity groups, compact task brief, output block |
| global send error banner | failure beside the action and composer retry |
| green solid session buttons | rail rows with only a pale active state |
| raw header status text | compact connection/model disclosure |
| always-visible utility controls | contextual inspector drawer |

## Architecture constraints

- Continue using Vue 3 Composition API, existing Element Plus controls, Wails,
  and `desktop/frontend/src/api/kernelApi.ts` as the sole HTTP choke point.
- No new visual dependency, router, global state library, or kernel endpoint.
- Preserve Project/Task/Chat persistence, session model bindings, attachment
  workflows, approval decisions, search, local-model controls, and timeline
  refresh semantics.
- Use only truthful existing projection fields in primary UI.

## Verification target

Automated tests protect timeline mapping, session-model isolation, no duplicated
live/durable output, startup readiness behavior, and structural source-level
guards for the new workspace component boundaries. A manual visual pass uses
the packaged desktop app at common desktop sizes and exercises Chat, Task, and
Project creation, session selection, cloud model selection, send/retry, and
inspector detail opening.
