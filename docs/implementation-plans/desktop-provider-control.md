# Desktop Provider Control Implementation Plan

**Goal:** historical implementation record for desktop configuration and
verification of existing model profiles without terminal-only steps.

**Status:** superseded for ordinary conversations by
`docs/requirements/kernel-session-model-binding.md` and
`docs/requirements/desktop-provider-onboarding.md`.

**Architecture:** extract existing `genesisctl` configuration mutation into a
small shared local owner and project safe DTOs through the desktop backend.
Ordinary chat binds a profile to one session through the kernel; it does not
apply a global coordinator binding or restart `genesisd`.

**Red lines:** no frontend secrets, no provider-context assembly in desktop,
no arbitrary endpoint editor, no external-kernel restart, and no active-turn
switch.

## Phase A: Shared Local Configuration Owner

1. Move the existing preset/profile/credential mutation rules used by
   `genesisctl provider use`, `rotate-key`, and `verify` into one internal
   local configuration package; retain CLI behavior through that owner.
2. Add failing tests for safe profile projection, credential redaction,
   validation-before-write, and model-role binding updates.
3. Add a desktop backend facade that lists profiles, accepts a one-shot key,
   calls the kernel's read-only selected-profile verification diagnostic, and
   reports safe reason codes.

## Retired Phase B: Owned Sidecar Activation

1. Add failing desktop tests for active-turn refusal, owned-sidecar restart,
   settled-session preservation, and external restart-required behavior.
2. Implement the smallest apply operation: validate/write binding, then restart
   only the desktop-owned `genesisd` handle and recheck readiness.
3. Keep all non-secret errors visible to the UI and preserve the written
   selection for a manual retry.

This activation path is not an ordinary chat model selector. Any future use
must be restricted to an explicit non-conversation runtime role.

## Phase C: Provider Panel

1. Add frontend tests for safe profile rendering, session/profile selection,
   one-shot key input clearing, and verification status.
2. Add a compact Provider panel from the existing top bar; do not create a
   router, settings subsystem, or model mapping form.
3. The composer, not this panel, renders the session-local profile selector.
   The panel imports, verifies, and rotates configured profiles.

## Phase D: First-Run DeepSeek Flash

1. [x] Add failing shared-owner tests that an empty Home receives only canonical
   DeepSeek Flash route/profile metadata and a protected credential, with no
   secret in models.json.
2. [x] Move the existing CLI preset values behind that `localconfig` owner; keep
   kernel provider verification separate and retain CLI behavior through the
   same owner.
3. [x] Add one desktop backend setup bridge and a Provider-panel empty state. The
   frontend clears the key, reloads projections, verifies the new profile, and
   leaves session binding to the composer.
4. [ ] `manual_test_pending`: prove a fresh configured DeepSeek Flash profile
   through installed session binding, a real turn, and restart replay. Curated
   additional templates are governed by the newer onboarding requirement.

**Reference scan:** Reasonix's `internal/config/edit.go` validates and persists
settings separately from runtime controllers; `desktop/settings_app.go` exposes
curated access setup. Codex merges built-in and user provider configuration in
`codex-rs/core/src/config/mod.rs` before provider resolution. Genesis reuses a
shared local configuration writer but keeps a single approved preset.

## Acceptance

Run focused shared-owner, kernel, desktop backend, and frontend tests; then
`go test ./... -count=1`, `go build ./...`, desktop tests/build, and
`git diff --check`. Finally use the desktop to import or rotate a cloud key,
verify a profile, select it for one session, submit a turn, and confirm an
earlier settled session remains readable without changing another session.
