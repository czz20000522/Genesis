# Requirement: Persistent Session Workspace Binding

- **Status:** approved for phased implementation.
- **Owner:** Genesis Kernel session/tool authority and desktop session catalog.
- **Scope:** persist the primary workspace of a session while supporting a
  project-bound session, an independently created local task session, and a
  durable cloud chat without a workspace.

## Production Target

Every desktop conversation has a persistent local transcript. Project sessions
share their user-selected real project directory but have independent session
history. Task sessions receive a new persistent directory under the Genesis
default task root and do not share it with another task. Chat sessions keep
their transcript locally and have no default local workspace. File access is
governed by the selected permission mode.

## Semantics

1. A session has exactly one immutable primary-workspace binding: `project`,
   `task`, or `none`. It determines default cwd and the default-mode write
   root; it is not a universal read boundary.
2. `project` binds an existing absolute local directory. The desktop project
   catalog may associate many independent session ids with the same directory.
3. `task` creates one new absolute directory below the configured default task
   root (`C:\Users\Tomczz\Documents\Genesis` by default). The directory and
   session record persist; neither is automatically deleted.
4. `none` is the durable cloud chat mode. It stores local transcript/ledger
   facts but has no implicit cwd; a filesystem request must name a directory.
5. Permission and workspace are independent: `plan` permits globally readable
   inspection but no writes; `default` permits globally readable inspection
   and writes only below the primary workspace; `yolo` permits host-level
   read/write execution.
6. A user- or model-supplied cwd may name another readable directory. It is
   accepted for read-only operations in `plan` and `default`, but a write
   outside the primary workspace is rejected in `default`.
7. A project directory that becomes unavailable remains bound but only blocks
   operations that need its default cwd or a default-mode write; it never
   redirects an operation to another workspace.
8. Absolute host paths are kernel-owned authority fields: they are not provider
   context, model-visible fields, session search snippets, or ordinary timeline
   facts. The desktop may separately show a selected project's display path.

## Non-Goals

- No archive/zip copy of a Project repository.
- No automatic deletion, task-session expiry, cloud sync, remote filesystem,
  or cross-session workspace isolation for Project members.
- No automatic long-term-memory promotion of files or transcript output.

## Acceptance Criteria

1. Opening a local directory as a Project and creating two sessions gives both
   sessions separate transcript histories and the same real file state.
2. Creating two Tasks creates two persistent distinct directories under the
   default task root; restarting desktop and kernel preserves both.
3. A Chat survives restart and can be searched/read locally; with an explicit
   user path, `plan`/`default` can inspect that path but cannot write it.
4. A default-mode write outside the primary Project/Task root fails closed,
   while `yolo` can execute the same explicit request.
5. The first desktop task, “read this repository and explain what it does,”
   works from a Project binding without first uploading a zip.
