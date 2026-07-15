# Desktop-Owned Local Model Lifecycle Implementation Plan

**Goal:** replace the retired WSL systemd llama.cpp services with a desktop-owned
WSL child process that the user can stop without affecting other GPU work.

**Status:** implementation complete; real desktop start/stop acceptance remains
with the user under `APP-DESKTOP-LOCAL-MODEL-LIFECYCLE-20260711`.

**Architecture:** add one desktop-local model supervisor alongside the existing
kernel sidecar supervisor. It reads a desktop-only Genesis Home config, launches
one `wsl.exe --exec` child, retains its handle, probes health, and uses the same
owned handle for manual stop and app shutdown.

## Task 1: Lock ownership with red tests

- Add desktop tests for startup using a configured launcher, a ready external
  endpoint that prevents launch, manual stop, shutdown stop, disabled config,
  and external/unowned no-op behavior.
- Run the focused test and observe the missing local-model supervisor failure.

## Task 2: Implement the narrow supervisor

- Add `desktop/local_model_supervisor.go` with an owned handle, mutex-protected
  start gate, idempotent stop, and direct `wsl.exe -d <distribution> --exec`
  launch.
- Do not call `systemctl`, use a detached shell, or enumerate existing llama
  processes.
- Re-run the focused tests.

## Task 3: Bind it to the desktop shell

- Extend `desktop/app.go` startup/shutdown and Wails bridge methods with local
  model status plus manual Start/Stop actions.
- Add compact top-bar controls using the existing plain-CSS desktop shell.
- Verify the external kernel rule remains unchanged.

## Task 4: Verify real ownership

- Write `~/.genesis/config/desktop-local-model.json` with the accepted Qwen
  baseline command.
- Start the desktop-owned model, verify its health, stop it from the desktop
  bridge, and prove no llama systemd units remain.
- Run desktop tests/build, `go test ./... -count=1`, `go build ./...`, and
  `git diff --check`.
