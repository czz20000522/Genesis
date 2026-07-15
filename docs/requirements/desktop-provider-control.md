# Requirement: Desktop Provider Control

- **Status:** approved for Phase 2 provider onboarding; non-conversation role
  configuration is deferred to the parent-worker settings surface.
- **Owner:** Genesis desktop application owns local provider configuration
  editing; the kernel owns provider resolution, session-model bindings, and
  turn execution.
- **Scope:** let a desktop user inspect configured provider/model profiles,
  securely update an existing credential, verify a selected profile upstream,
  without using a terminal.
  Desktop conversation onboarding and session-level model choice are governed
  by `desktop-provider-onboarding.md` and `kernel-session-model-binding.md`.

## Production Target

The desktop presents the profiles already configured in Genesis Home, grouped
by provider route. A user can see readiness and non-secret diagnostics, enter
or rotate a profile's API key without the frontend retaining it, verify the
resulting profile, and select it only for the current idle session through the
composer.

## Semantics

1. The desktop reads the user-owned `models.json` and local credential store
   through one desktop backend service. The browser frontend receives only safe
   profile metadata, readiness, and diagnostic reason codes.
2. A profile view exposes profile id, model id, provider route, adapter id,
   protocol, and whether its credential is present. Declared runtime roles may
   be shown as read-only metadata, but are not a desktop mutation target in
   this phase. The view never exposes API keys, raw credential files, command
   environment values, or provider-command arguments.
3. Credential entry is a one-shot desktop backend operation. The key travels
   from the native desktop bridge to the credential writer over stdin or an
   equivalent in-process secret boundary; it is not written to browser storage,
   frontend logs, URLs, model context, kernel events, or desktop projections.
4. Upstream verification uses the selected profile's existing provider adapter
   and returns only readiness plus a redacted reason. Local provider commands
   may use their explicitly configured unbounded verification behavior; cloud
   verification remains bounded.
5. Importing, editing credentials, and verifying a profile do not mutate a
   global model role or restart `genesisd`. The current session binds a profile
   through the kernel's protected session-model endpoint; that binding never
   changes another session.
6. A failed credential write, configuration validation, or verification leaves
   the previous usable configuration intact and projects a stable error reason.
   A failed upstream verification does not erase a newly stored key; the user
   can correct or rotate it.

## Non-Goals

- No provider marketplace or kernel-owned credential/configuration write in
  this role-control slice.
- No model-to-model mapping layer; the user selects a discovered/configured
  profile directly.
- No kernel HTTP route that writes desktop credentials or makes the kernel a
  credential owner.
- No desktop mutation of global role bindings, hot switch of an active turn,
  sidecar restart, or restart of an external kernel.

## Acceptance Criteria

1. A user can see every configured profile in the desktop without revealing a
   secret or local command details.
2. A user can update the key for an existing cloud profile and verify it
   without using a terminal. Ordinary conversation selection is session-bound
   under `kernel-session-model-binding.md`, not a global `coordinator` role.
3. Selecting a model in one idle session neither restarts the owned kernel nor
   changes another session's model binding.
4. Local Qwen, DeepSeek, and OpenCode Go GLM profiles show their distinct
   adapter/protocol/readiness posture through the same safe projection.
5. External-kernel mode never starts, stops, or restarts the kernel.
