# Requirement: Desktop Provider Control

- **Status:** approved for Phase 2 implementation.
- **Owner:** Genesis desktop application owns local configuration editing and
  sidecar restart orchestration; the kernel owns provider resolution and turn
  execution.
- **Scope:** let a desktop user inspect configured provider/model profiles,
  securely update an existing credential, verify a selected profile upstream,
  and activate a profile for a meaningful role without using a terminal. When
  Genesis Home has no profile, it also offers the one approved DeepSeek Flash
  setup path.

## Production Target

The desktop presents the profiles already configured in Genesis Home, grouped
by provider route. A user can select a role and profile, see readiness and
non-secret diagnostics, enter or rotate that profile's API key without the
frontend retaining it, verify the resulting profile, and apply the selected
binding to an owned `genesisd` without losing settled sessions.

## Semantics

1. The desktop reads the user-owned `models.json` and local credential store
   through one desktop backend service. The browser frontend receives only safe
   profile metadata, readiness, and diagnostic reason codes.
2. A profile view exposes profile id, model id, provider route, adapter id,
   protocol, configured role bindings, and whether its credential is present.
   It never exposes API keys, raw credential files, command environment values,
   or provider-command arguments.
3. Credential entry is a one-shot desktop backend operation. The key travels
   from the native desktop bridge to the credential writer over stdin or an
   equivalent in-process secret boundary; it is not written to browser storage,
   frontend logs, URLs, model context, kernel events, or desktop projections.
4. Upstream verification uses the selected profile's existing provider adapter
   and returns only readiness plus a redacted reason. Local provider commands
   may use their explicitly configured unbounded verification behavior; cloud
   verification remains bounded.
5. Applying a role/profile binding writes the canonical local model
   configuration. If the desktop owns `genesisd`, it restarts only that owned
   sidecar after no active turn is present. If the kernel is external, desktop
   saves the binding and reports `external_kernel_restart_required`; it never
   restarts an external process.
6. A failed credential write, configuration validation, verification, or owned
   restart leaves the previous selected binding usable and projects a stable
   error reason. A failed upstream verification does not erase a newly stored
   key; the user can correct or rotate it.
7. With no configured profile, the desktop may create only the fixed DeepSeek
   Flash profile (`deepseek-flash`) from its known adapter metadata and a
   one-shot API key. It then follows the same verify and explicit apply path as
   any existing profile.

## Non-Goals

- No provider marketplace, remote model discovery, arbitrary endpoint editor,
  automatic provider configuration import, or second preset in this slice.
- No model-to-model mapping layer; the user selects a discovered/configured
  profile directly.
- No kernel HTTP route that writes desktop credentials or makes the kernel a
  credential owner.
- No hot switch of an active turn and no restart of an external kernel.

## Acceptance Criteria

1. A user can see every configured profile and role binding in the desktop
   without revealing a secret or local command details.
2. A user can update the key for an existing cloud profile, verify it, select
   it for `coordinator`, and complete a subsequent turn through the selected
   model without using a terminal.
3. The desktop restarts only its owned kernel after applying a binding and
   settled sessions remain searchable and readable afterward.
4. Local Qwen, DeepSeek, and OpenCode Go GLM profiles show their distinct
   adapter/protocol/readiness posture through the same safe projection.
5. External-kernel mode never starts, stops, or restarts the kernel while still
   explaining that a restart is required to activate a saved binding.
6. With an empty Genesis Home, a user can set up DeepSeek Flash, verify it,
   explicitly bind `coordinator`, and complete a turn without using a terminal.
