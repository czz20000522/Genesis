# Requirement: Desktop-Owned Local Model Lifecycle

- **Status:** approved for implementation.
- **Owner:** Genesis desktop application.
- **Scope:** start, stop, and project only the WSL llama.cpp process launched by
  the current desktop client.

## Production Target

When its local-model configuration is enabled, Genesis desktop starts the
configured WSL `llama-server` only after the user explicitly chooses Start.
The desktop holds the Windows `wsl.exe --exec` process handle, projects the
local-model state, offers explicit start/stop controls, and stops that same
owned process tree when the client exits. A stopped local model leaves the
desktop and its cloud-provider path usable.

## Boundaries

- This is not a systemd service, a startup task, or a global llama.cpp process
  manager. Before launching, it may make one request to its configured health
  endpoint solely to avoid competing with an already-serving external process;
  it never enumerates or identifies that process.
- Stop acts only on the process tree rooted at the `wsl.exe` process started by
  this desktop instance. It never finds or kills by port, executable name, GPU
  use, model name, or arbitrary WSL PID.
- An externally running llama.cpp server has no owned handle and remains
  untouched.
- The desktop owns local process lifecycle only. The kernel still owns model
  configuration, provider context, turns, and transcript facts.
- The configuration lives in Genesis Home under a desktop-specific file, not in
  a kernel route or a system service unit.

## Semantics

1. The local-model config declares WSL distribution, server path, model path,
   and exact llama.cpp arguments.
2. The launcher uses `wsl.exe -d <distribution> --exec <server> ...`; it never
   invokes `systemctl`, a shell service wrapper, or a detached process.
3. Start is idempotent for a live owned handle. If the configured health
   endpoint is already ready before Start, the desktop reports an unowned
   serving endpoint and does not launch or adopt it. Stop is idempotent and
   clears only an owned handle before termination.
4. Start failure, occupied endpoint, or readiness failure is visible but does not prevent cloud
   usage or stop the kernel sidecar.
5. App shutdown calls the same owned-stop path. The desktop never stops an
   external model merely because the configured port is reachable.

## Acceptance Criteria

1. The desktop starts only the configured WSL command and records its owned PID.
2. Manual stop and app shutdown terminate exactly that owned process tree once.
3. A manually stopped local model can be started again without restarting the
   desktop.
4. Disabled or failed local startup leaves the desktop usable with cloud models.
5. Tests prove no path invokes systemd or stops an unowned process.
6. Tests prove a ready configured endpoint prevents launch and remains
   unowned.
