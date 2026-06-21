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
$env:TEMP\genesisd.exe `
  -addr 127.0.0.1:8765 `
  -ledger $env:TEMP\genesis-events.jsonl `
  -runtime-token local-dev-token
```

Minimal HTTP surface:

- `GET /ready`
- `POST /turn`
- `GET /sessions/{id}`
- `POST /tools/shell.exec`
- `POST /memory/candidates`
- `POST /memory/candidates/{id}/approve`

The phase 1 provider is intentionally fake. It proves admission, event persistence, session projection, and restart-safe ledger replay before real providers or tools are added.

Protected routes require `Authorization: Bearer <runtime-token>`. `GET /ready` is the only unauthenticated route.

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

- `plan` mode blocks shell execution fail-closed.
- `default` mode requires a kernel-configured workspace root and uses a kernel-controlled command set from inside that workspace. It does not invoke the operating-system shell, expand environment variables, or execute arbitrary interpreters.
- `yolo` mode is explicit high-trust execution and is the only mode that invokes the operating-system shell. It can only be selected by kernel startup configuration.

The controlled default command set is intentionally narrow: text output, simple file reads, and simple file writes whose real path remains inside the configured workspace. Symlink/junction resolution, parent traversal, absolute path escapes, shell metacharacters, and unsupported commands are blocked before any process is spawned.

The HTTP request cannot select `permission_mode` or `workspace_root`; those are kernel-owned authority fields. Every allowed call first records a `running` operation before process execution, then records completion or failure with tool name, permission mode, command, cwd, status, exit code, bounded stdout/stderr, timestamps, and blocker reason when blocked. Operations are persisted in the event ledger and projected through `GET /sessions/{id}` after restart.

## Accumulation

The first memory loop is explicit and governed:

- `POST /memory/candidates` creates a pending candidate from user-visible text and a required `source_ref`.
- `POST /memory/candidates/{id}/approve` approves a candidate with required `approval_authority`, `approval_reason`, and `approval_evidence_ref`.
- `POST /turn` recalls only approved candidates and records recalled memory refs on the turn event.

The first recall strategy is intentionally simple text matching. It proves the governance, provenance, and restart-safe replay loop before adding vector indexes or richer memory policy.
