# Genesis Kernel

Genesis Kernel is a small local-first runtime for an LLM-driven agent environment.

It is not a web app, CLI app, Feishu adapter, coding agent, or desktop product. Those are shells or external applications. The kernel exposes stable task and tool contracts that any shell can call.

## Kernel Scope

The kernel owns only these planes:

- Interface Kernel: accept turns, normalize input, route sessions, emit events.
- Model Gateway: call configured model providers and project provider failures.
- Tool System: expose tools, enforce permission policy, execute effects, return evidence.
- WorkRegistry: persist work state, cancellation, recovery, and resumable execution records.
- Accumulation: persist memory candidates, approval state, recall records, and source refs.
- Auth/Credential Plane: protect runtime clients and resolve credential refs without leaking secrets.

## Out Of Scope

- CLI, WebUI, desktop UI, browser UI.
- Feishu, WeChat, email, calendar, document, OCR, or other application-specific logic.
- Skill bodies, product prompts, user workflows, and channel daemons.
- Project-specific assumptions from the previous Python implementation.

## Design Rule

External applications are user-space programs. The kernel may receive events from them and may let the active model call their CLIs through governed tools, but it must not become those applications.

## Phase 1 Spike

Build the first runnable kernel binary:

```powershell
D:\software\Go\bin\go.exe build -o $env:TEMP\genesisd.exe .\cmd\genesisd
```

Run it with an explicit ledger path:

```powershell
$env:TEMP\genesisd.exe -addr 127.0.0.1:8765 -ledger $env:TEMP\genesis-events.jsonl
```

Minimal HTTP surface:

- `GET /ready`
- `POST /turn`
- `GET /sessions/{id}`
- `POST /tools/shell.exec`

The phase 1 provider is intentionally fake. It proves admission, event persistence, session projection, and restart-safe ledger replay before real providers or tools are added.

## Provider Configuration

`genesisd` defaults to the fake provider:

```powershell
$env:TEMP\genesisd.exe -provider fake
```

An OpenAI-compatible provider can be selected without changing kernel code:

```powershell
$env:TEMP\genesisd.exe `
  -provider openai-compatible `
  -provider-base-url https://provider.example.com/api `
  -provider-model example-model `
  -provider-api-key-env GENESIS_PROVIDER_API_KEY
```

The provider base URL is configuration, not a kernel route. Include whatever provider-specific prefix the upstream service requires in `-provider-base-url`.

## Tool Runtime

The first kernel tool is `shell.exec`. It is deliberately small:

- `plan` mode blocks commands that look mutating.
- `default` mode requires a workspace root and runs only from inside that workspace.
- `yolo` mode is explicit high-trust execution.

Every call records an operation with tool name, permission mode, command, cwd, status, exit code, bounded stdout/stderr, timestamps, and blocker reason when blocked. Operations are persisted in the event ledger and projected through `GET /sessions/{id}` after restart.
