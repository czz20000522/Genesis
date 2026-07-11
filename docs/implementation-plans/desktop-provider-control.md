# Desktop Provider Control Implementation Plan

**Goal:** allow the desktop to configure, verify, and activate existing model
profiles without terminal-only steps.

**Status:** implementation complete; real configured-profile acceptance remains
with the user under `APP-DESKTOP-PROVIDER-CONTROL-20260711`.

**Architecture:** extract existing `genesisctl` configuration mutation into a
small shared local owner, project safe DTOs through the desktop backend, and
restart only a desktop-owned kernel after a validated binding change.

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

## Phase B: Owned Sidecar Activation

1. Add failing desktop tests for active-turn refusal, owned-sidecar restart,
   settled-session preservation, and external restart-required behavior.
2. Implement the smallest apply operation: validate/write binding, then restart
   only the desktop-owned `genesisd` handle and recheck readiness.
3. Keep all non-secret errors visible to the UI and preserve the written
   selection for a manual retry.

## Phase C: Provider Panel

1. Add frontend tests for safe profile rendering, role/profile selection,
   one-shot key input clearing, verification status, and apply-result states.
2. Add a compact Provider panel from the existing top bar; do not create a
   router, settings subsystem, or model mapping form.
3. Show only configured profiles initially: local Qwen, DeepSeek, and OpenCode
   Go models validate the generic projection path.

## Acceptance

Run focused shared-owner, kernel, desktop backend, and frontend tests; then
`go test ./... -count=1`, `go build ./...`, desktop tests/build, and
`git diff --check`. Finally use the desktop to rotate or enter a cloud key,
verify a profile, switch `coordinator`, restart the owned sidecar, submit a
turn, and confirm an earlier settled session remains readable.
