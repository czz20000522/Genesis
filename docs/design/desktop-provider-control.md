# Design: Desktop Provider Control

- **Requirement:** `docs/requirements/desktop-provider-control.md`.
- **Owner split:** desktop configuration service owns user-file mutation and
  secret handling; existing kernel resolution, verification, and session-model
  binding own provider truth.

## Reference Scan

Reasonix's `desktop/settings_app.go` separates provider access and settings
mutation from the runtime controller, and `desktop/tab_profile_test.go` proves
tab-local profile choices do not rewrite provider configuration. Its
`internal/config/edit.go` refuses to remove a referenced provider without a
safe fallback. Genesis aligns with the separation but initially narrows scope
to already configured profiles rather than importing Reasonix's broader
provider editor.

Codex keeps model-provider configuration outside turn execution. Genesis uses
the same boundary: profile selection is configuration state; `genesisd` loads
it through existing resolver code and remains the only provider-context owner.

## Desktop Shape

The existing utility app shell remains unchanged: session rail on the left,
workspace in the center, and a compact Provider panel from the top bar.

```text
Provider panel
  configured profiles
  -> inspect safe metadata / credential-present status
  -> optional one-shot credential write
  -> selected-profile upstream verify
  -> composer binds the selected profile to one idle session
```

The panel uses background steps rather than a new page hierarchy. It is not the
ordinary chat model selector: the composer binds a session's selected model in
kernel session truth.

Blank-Home provider import and ordinary session model choice are superseded by
`desktop-provider-onboarding.md` and `kernel-session-model-binding.md`. This
panel remains the role-runtime control surface; it does not choose models for
ordinary desktop conversations.

## Backend Service

Create a desktop-local provider control service that reuses one shared
configuration reader/writer with `genesisctl`; do not duplicate JSON mutation
rules in Vue or kernel code. The service exposes safe DTOs to Wails:

- profile list;
- credential-present state and one-shot credential rotation;
- selected-profile verification result;

The shared owner validates a profile and binding before writing. It accepts key
bytes only in the desktop backend call, persists them in the existing local
credential store, then discards them. `models.json` remains the migration unit;
the key does not enter it.

Profile verification is a read-only authenticated kernel diagnostic call. The
desktop supplies only role and profile id; `genesisd` resolves that selection
from its configured Home and invokes the existing adapter-specific live verify
path. This is not a configuration-write route and does not make the kernel a
credential owner.

## Runtime And Failure Semantics

Provider configuration and verification do not change a running sidecar. The
composer binds an idle session with the kernel's protected session-model
endpoint; it never changes a global coordinator role. Credential-write,
configuration-validation, and verification failures keep the prior usable
configuration and return a concise retryable diagnostic. No provider-panel
action starts, stops, or restarts either an owned or external kernel.

## Rejected Alternatives

- Frontend-local API keys: rejected because browser storage and console traces
  are not the credential boundary.
- Copying the CLI preset into the desktop: rejected because profile metadata
  would drift across two configuration writers.
- Kernel configuration-write HTTP routes: rejected because daemon authority
  must not become credential/configuration authority.
- Applying a new global binding with a sidecar restart: rejected because an
  ordinary conversation must not change other sessions or runtime lifecycle.
