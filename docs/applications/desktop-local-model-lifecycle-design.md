# Design: Desktop-Owned Local Model Lifecycle

- **Requirement:** `desktop-local-model-lifecycle-requirement.md`
- **Owner:** desktop application.

## Reference Scan

- Codex `codex-rs/rmcp-client/src/stdio_server_launcher.rs` owns a spawned
  process handle, makes termination idempotent, and on Windows uses
  `taskkill /PID <owned pid> /T /F`; its process-group cleanup test proves
  descendants exit.
- Reasonix `internal/lsp/manager.go` keeps session-owned clients behind a
  start gate, removes an owned record before close, and never discovers an
  arbitrary process to shut down.

Genesis follows these ownership rules. It intentionally does not adopt a
machine-wide daemon, automatic restart, or a search-by-port stop path.

## Data Flow

```text
explicit StartLocalModel
  -> read ~/.genesis/config/desktop-local-model.json
  -> probe the configured /health endpoint once
  -> if already ready, project unowned endpoint and do not launch
  -> otherwise wsl.exe -d Ubuntu --exec llama-server <exact configured args>
  -> retain owned Windows process handle and probe /health
  -> project status to desktop

manual stop or desktop shutdown
  -> detach owned handle under supervisor lock
  -> taskkill that wsl.exe PID tree
  -> project stopped state
```

`genesisd` and the local llama.cpp process are separate owned sidecars. Stopping
the model does not stop the kernel; stopping the desktop stops both handles it
created. An externally configured kernel remains external under the existing
desktop sidecar rule, while the local-model supervisor still acts only on its
own handle.

## Failure And Recovery

- Missing/invalid desktop-local-model config projects `local_model_config_invalid`.
- A ready configured endpoint before Start projects
  `local_model_endpoint_already_serving`; it is never adopted or stopped.
- A launcher error projects `local_model_start_failed`.
- A process that starts but does not answer `/health` projects
  `local_model_readiness_probe_failed`; it remains eligible for explicit stop.
- A failed owned-process termination retains that same handle and projects
  `local_model_stop_failed`, so the user can retry rather than seeing a false
  stopped state or losing authority over a still-running GPU process.
- A process exit clears the owned handle and projects `local_model_exited`.
- Stop ignores no-handle/external state and is idempotent.

The desktop has no model-server restart loop. The user chooses Start after a
manual stop or failure.
