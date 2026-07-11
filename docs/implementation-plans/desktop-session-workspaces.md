# Persistent Project, Task, And Chat Sessions Implementation Plan

**Goal:** make the desktop create and resume durable Project, Task, and Chat
sessions with the correct local workspace authority.

**Status:** implementation complete; the Phase C local-Qwen desktop acceptance
remains with the user under
`APP-DESKTOP-PERSISTENT-SESSION-WORKSPACES-20260711`.

**Architecture:** the kernel persists immutable primary-workspace bindings and
derives default cwd/default-mode write roots from them. Permission mode decides
global read access and whether an explicit cross-project write is allowed;
desktop persists only its Project catalog and Task root navigation.

**Red lines:** no zip/archive Project import; no desktop-owned cwd override;
no path in model context/search/timeline; no automatic Task deletion; Chat has
local transcript persistence but no local workspace.

## Phase A: Kernel Binding Primitive

1. Write failing kernel and HTTP tests for create/read/restart of `project`,
   `task`, and `none` bindings; include immutable rebinding refusal, no raw
   path in session/search projections, and no implicit cwd for Chat.
2. Add the smallest binding type, ledger event, protected HTTP request, replay
   projection needed internally, and session-scoped tool-policy resolver.
3. Make shell/edit tool preparation resolve an omitted cwd from the binding,
   allow explicit cross-directory read operations in `plan`/`default`, and
   reject default-mode writes outside the primary root.
4. Prove `yolo` can execute an explicit cross-directory write, while the same
   request is blocked in `default` and all writes are blocked in `plan`.

## Phase B: Desktop Session Catalog And Navigation

1. Add desktop tests for a persisted Project catalog and a durable Task-root
   directory creator. The Task test must prove each created session gets a
   different directory under `C:\Users\Tomczz\Documents\Genesis`.
2. Add Project directory selection (not a zip filter), create Project session,
   create Task session, and create Chat session API calls through
   `desktop/frontend/src/api/kernelApi.ts` only.
3. Replace the current zip-only repository intake affordance in `App.vue` with
   Project/Task/Chat entry actions and a session rail grouped by Project and
   standalone Task/Chat items.
4. Preserve existing material attachment as an attachment-only feature; it is
   not a Project bootstrap path.

## Phase C: End-To-End Desktop Acceptance

1. Bind this Genesis repository as a Project and submit “read this repository
   and explain what it does”; prove the model invokes the bound workspace tools
   and its transcript is available after restart.
2. Create two Project sessions and prove their histories differ while a file
   edit in one is visible to the other through the shared project directory.
3. Create two Tasks, restart, and prove their directories and transcripts are
   retained and distinct.
4. Create a Chat, restart, search/read its local transcript, then use an
   explicit local repository path for a read-only exploration turn.
5. Run focused tests, `go test ./... -count=1`, `go build ./...`, desktop Go
   tests, frontend tests/build, and `git diff --check`; finish the governing
   requirement/design/issue/BDD drift check.
