# Desktop First-Run DeepSeek Design

## Decision

When Genesis Home has no model profile, the desktop exposes one curated setup:
DeepSeek Flash. The user enters an API key, stores it in the existing protected
credential store, verifies the new profile through the kernel adapter, then
explicitly applies it to `coordinator`. No arbitrary endpoint form, provider
marketplace, or automatic selection is introduced.

## Flow

1. The existing Provider panel finds no configured profiles and renders a
   compact DeepSeek Flash setup form instead of an empty dead end.
2. The Wails backend passes the one-shot key to a desktop-local setup method.
   The frontend clears the input regardless of outcome.
3. A shared `localconfig` owner creates or replaces only the fixed
   `deepseek-flash` profile, its `deepseek` route, the canonical adapter
   metadata, and `secret://models/deepseek/local` credential record.
4. The desktop reloads safe profile projections and verifies the selected
   profile through the existing read-only kernel diagnostic.
5. The user presses the existing apply action. It writes the `coordinator`
   binding and restarts only a desktop-owned kernel; external-kernel mode keeps
   the saved binding and reports that its owner must restart it.

## Boundaries

- `localconfig` owns preset metadata, models.json mutation, and protected
  credential persistence. It does not execute providers or write kernel facts.
- `genesisd` continues to own adapter resolution and upstream verification.
- Desktop owns only the native secret handoff, safe result projection, and its
  owned sidecar restart.
- The browser never receives the API key after submission, any credential
  reference, raw endpoint configuration, or provider request detail.

## Failure Rules

- Invalid or empty key: do not create a profile or binding.
- Setup write failure: preserve existing configuration and present a concise
  setup error.
- Verification failure: retain the newly stored profile and key for an
  explicit retry; do not bind or restart automatically.
- Apply failure: preserve the selected profile and written binding; do not
  restart an external kernel or silently substitute another provider.

## Reference Scan

Reasonix's `internal/config/edit.go` is a validated, persistence-focused
configuration mutation layer, while `desktop/settings_app.go` exposes a
curated provider-access action rather than mixing setup into turn execution.
Codex's `codex-rs/core/src/config/mod.rs` merges built-in and configured model
providers before resolving the active provider. Genesis follows the first
boundary but intentionally stays narrower: one explicit DeepSeek Flash preset
rather than a general provider editor or built-in catalog.

## Acceptance

With an empty Genesis Home, a desktop user can create a DeepSeek Flash profile,
verify it, apply it to `coordinator`, submit a real turn, restart Genesis, and
read that settled session. The API key must not appear in models.json, desktop
projections, browser storage, logs, or the event ledger.
