# Requirement: Desktop Agent Workspace

- **Status:** implemented; packaged native visual acceptance remains pending.
- **Owner split:** the kernel owns sessions, turns, timeline truth, approvals,
  task graphs, agent invocations, and session-model bindings. The desktop owns
  navigation, layout, transient draft state, and projection rendering.

## Production target

Genesis Desktop is a calm, high-density agent workspace rather than a chat
clone or a provider-management page. A user can choose a durable Project,
Task, or Chat session, understand the current session and its real execution
state, send a task through a focused composer, and inspect the resulting
activity, output, failures, approvals, and artifacts without leaving the
workspace.

## Required behavior

1. The shell has a top bar, fixed left workspace rail, central workspace, and
   a demand-opened right detail panel. The right panel becomes the third column
   only when the user requests details or when an approval requires attention.
2. The rail presents entry actions, Projects with nested sessions, standalone
   Tasks, durable Chats, history/search, and settings as restrained rows. It
   must not use large colored session cards or a flat list that hides project
   containment.
3. The central surface is a task workspace. It has an identity header, an
   empty-state task invitation when no durable activity exists, an activity
   timeline when activity exists, and one persistent large composer at the
   visual center of the lower workspace.
4. Timeline rows are a projection of existing kernel events. A user prompt,
   visible reasoning, tool/turn progress, approval, failure, and assistant
   output may receive distinct workspace treatments, but no view may invent a
   planning, review, branch, artifact, or completion fact absent from the
   projection.
5. A failed action appears beside the action that failed with a concise cause
   and explicit retry where the existing contract permits it. It must not
   replace the workspace with a global red banner or discard durable output.
6. The composer exposes attachment intake, the active session's model choice,
   stop while streaming, and send. A model choice remains session-scoped; no
   model control implicitly starts or stops a local model.
7. Project, Task, and Chat semantics remain unchanged: Project sessions share
   the selected project's root; Tasks have separate durable roots; Chats have
   durable local transcripts without a project root.
8. The surface uses a white canvas, near-black text, soft gray structure, one
   restrained accent, compact metadata, light borders, restrained depth, and
   keyboard-visible focus. It must not resemble a generic SaaS dashboard.

## Non-goals

- No new kernel facts, timeline event kinds, Git probing, branch display, or
  fabricated agent states.
- No provider configuration, model import, or local llama.cpp lifecycle change
  in this visual refactor.
- No router or global state store. The existing Wails single-screen application
  and kernel API choke point remain intact.
- No permanent right-side monitoring wall. Detail remains contextual so the
  task canvas stays calm.

## Acceptance

- A fresh Chat, Task, and Project each opens a coherent workspace with the
  correct durable identity and composer model binding.
- Existing completed, streaming, reasoning, approval, and failed timeline
  evidence remains visible, legible, and non-duplicated after refresh.
- A user can select a session from each rail section, change only that
  session's model, attach input, send, interrupt, and open details through the
  UI without needing a CLI or port.
- The normal workspace shows no raw kernel/provider error text, fake workflow
  phases, large solid session blocks, or redundant connection/status pills.
