# Genesis

Genesis is a local-first personal AI runtime for a governed LLM-powered digital
self.

The short version: Genesis gives an LLM a controlled home where sessions,
tools, memory, resources, credentials, external capabilities, and evidence are
owned by explicit runtime boundaries instead of scattered prompts and scripts.

Start here:

- Product brief and roadmap: `docs/project-brief.md`
- Kernel authority contract: `docs/kernel-contract.md`
- Minimal closed loop: `docs/minimal-closed-loop.md`
- Development process: `docs/process.md`
- Documentation map: `docs/README.md`

## Shape

```text
desktop / connectors / capability packages
        |
        v
Genesis Kernel HTTP primitives
        |
        v
model gateway / tool runtime / session ledger / memory / credentials
```

The kernel is not the desktop app, Feishu adapter, WebUI, CLI, OCR parser,
email client, or capability package. Those are user-space shells or
applications that use kernel primitives.

## Quick Build

```powershell
D:\software\Go\bin\go.exe test ./... -count=1
D:\software\Go\bin\go.exe build ./...
```

Build the local daemon:

```powershell
$root = Join-Path (Get-Location) ".genesis-live\manual"
New-Item -ItemType Directory -Force "$root\bin" | Out-Null
D:\software\Go\bin\go.exe build -o "$root\bin\genesisd.exe" .\cmd\genesisd
```

Run with an explicit ledger and runtime token:

```powershell
"$root\bin\genesisd.exe" `
  -addr 127.0.0.1:8765 `
  -ledger "$root\events.sqlite" `
  -runtime-token local-dev-token
```

`GET /ready` is the only unauthenticated readiness route. Protected routes
require `Authorization: Bearer <runtime-token>`.

## User Home

Installed user state lives under `~/.genesis`, not in the source checkout:

```text
~/.genesis/
  config/
  credentials/
  models/
  accumulation/
  runtime/
  logs/
  skills/
  capabilities/
```

Small personal tools belong under `~/.genesis/capabilities/<id>` and become
discoverable through skills. They do not become kernel features.

## Current Kernel Surfaces

- `GET /ready`
- `GET /capabilities`
- `POST /turn`
- `GET /turns/{id}/events`
- `GET /turns/{id}/audit`
- `GET /sessions`
- `GET /sessions/{id}`
- `GET /sessions/{id}/timeline`
- `POST /tools/shell_exec`
- `POST /work`
- `GET /work/{id}`
- `POST /work/{id}/cancel`
- `POST /memory/candidates`
- `GET /memory/candidates`
- `GET /memory/candidates/{id}`
- `POST /memory/candidates/{id}/approve`
- `POST /memory/candidates/{id}/reject`
- `POST /memory/candidates/{id}/supersede`
- `POST /memory/recall`

Detailed behavior belongs in the contract, requirements, and designs linked
above, not in this README.
