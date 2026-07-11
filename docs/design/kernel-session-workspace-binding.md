# Design: Persistent Session Workspace Binding

- **Requirement:** `docs/requirements/kernel-session-workspace-binding.md`.
- **Owner:** Kernel session/tool authority; desktop owns project presentation.

## Reference Scan

Codex app server reads one thread configuration snapshot before startup in
`codex-rs/app-server/src/request_processors/thread_processor.rs`, then uses the
snapshot cwd to create its sandbox and returns that cwd in its desktop protocol
response. Reasonix `internal/cli/acp.go` builds one controller per ACP session
with the supplied absolute cwd; `TestACPFactoryLoadsSessionCwdProjectConfig`
proves that project-local behavior comes from that session root.

Genesis aligns with Codex's separation of cwd and writable roots, and with
Reasonix's explicit policy that reads are unrestricted while writers are
confined to configured roots. The binding is ledger-persistent across restart;
`none` is a durable chat state without an implicit cwd, not a no-filesystem
policy.

## Owners

The kernel owns the durable `session.workspace_bound` fact, validation of its
absolute primary path, and deriving the default cwd/write root for a session.
It never projects the raw path to model context, timeline, or session search.

The desktop owns a small persisted Project catalog: project id, display name,
selected absolute directory, and contained session ids. It creates a Task
directory before requesting a task binding, and it renders Project/Task/Chat
navigation. It does not grant permissions; user text may name an explicit cwd
that the kernel evaluates under the active permission mode.

## Data Flow

```text
desktop Project selected directory / Task auto-created directory / Chat none
  -> protected kernel session workspace bind request
  -> ledger session.workspace_bound fact
  -> session-specific default cwd and write root
  -> shell/edit/source tool authorization and execution
  -> durable transcript and session search (without raw workspace path)
```

The session binding is created before its first turn. Any later request to bind
the same session differently fails. An omitted cwd resolves to the primary
Project/Task root; a Chat has no implicit cwd. An explicit cwd is a requested
filesystem location, not a new session binding.

## Tool Semantics

`plan` grants only read operations, at any explicitly named readable host path.
`default` grants reads at any explicitly named readable host path and grants
writes only below the bound Project/Task root. `yolo` retains the existing
host-level execution policy. The primary root is therefore a write root, not a
read root. A Chat can inspect an explicitly named path in `plan`/`default`, but
cannot issue an implicit file operation and has no default-mode write root.

`workspace_edit` is always a write and follows the primary root in `default`.
`shell_exec` distinguishes controlled read commands from controlled writers
before checking the requested cwd/path. Existing global
`ToolPolicy.WorkspaceRoot` is retired from desktop session selection; it
remains only for non-desktop kernel callers until their own binding is supplied.

## Recovery And Failure

Ledger replay restores the immutable binding before any session tool can run.
If the configured bound directory is missing, unreadable, or no longer an
absolute safe directory, the session remains readable and searchable but loses
its implicit cwd and default-mode write root. Explicit read-only exploration of
another user-named path remains subject to permission and host availability.
No source snapshot is created and no files are copied.

## Rejected Alternatives

- Treating the bound root as a universal read jail is rejected because a user
  must be able to ask a Genesis session to inspect another local repository.
- Desktop-owned permission grants are rejected because restart, HTTP callers,
  and model tool arguments would create competing authority.
- Zip upload/source snapshot is rejected for Project exploration because the
  user explicitly selected a live directory whose current files should be read.
