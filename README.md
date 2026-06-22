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

Kernel HTTP routes stay unversioned. The durable contract is the task/tool schema and ledger evidence, not a numbered path prefix.

## Initial Kernel Spike

Build the first runnable kernel binary:

```powershell
D:\software\Go\bin\go.exe build -o $env:TEMP\genesisd.exe .\cmd\genesisd
```

Build the optional operator setup tool:

```powershell
D:\software\Go\bin\go.exe build -o $env:TEMP\genesisctl.exe .\cmd\genesisctl
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
- `GET /turns/{id}/events`
- `GET /sessions/{id}`
- `POST /tools/shell.exec`
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

The initial provider is intentionally fake. It proves admission, event persistence, session projection, and restart-safe ledger replay before real providers or tools are added.

Protected routes require `Authorization: Bearer <runtime-token>`. `GET /ready` is the only unauthenticated route, but readiness is `blocked` when no runtime token is configured because the kernel cannot accept protected work.

`POST /turn` accepts an optional `idempotency_key` when `session_id` is explicit. This key belongs to the Interface Kernel control plane. Retrying the same logical turn returns the original ledger-backed result and must not call the provider or execute tools a second time.

## Skill Catalog

External skills are user-space assets. Genesis can make installed skill metadata visible to the model without adding application-specific code to the kernel.

`genesisd` scans configured skill roots for `SKILL.md` front matter and injects a concise model-context catalog containing each skill's name, description, and instruction path. Missing roots and malformed skill files are ignored. Full skill bodies are not loaded into every turn.

By default, `genesisd` looks at the current user's global agent skill root:

```powershell
$HOME\.agents\skills
```

Operators can replace or extend roots with `GENESIS_SKILL_ROOTS` or repeated `-skill-root` flags:

```powershell
$env:GENESIS_SKILL_ROOTS = "$HOME\.agents\skills;$HOME\.genesis\skills"
$env:TEMP\genesisd.exe -skill-root D:\tools\custom-skills
```

This does not make Feishu, email, calendar, or any other application a kernel feature. The active model still uses governed tools such as `shell.exec` to read skill instructions and invoke installed CLIs under kernel permission policy.

## Provider Configuration

`genesisd` defaults to Genesis-owned model gateway configuration. It reads `models.json` from the Genesis config root, resolves the selected role/profile, selects the gateway route, and resolves the route credential from the Genesis local secret store.

Useful operator flags:

- `-config-root`: directory containing `models.json`; defaults to `~/.genesis/config`.
- `-credential-store-root`: directory containing local credential records; defaults to `~/.genesis/credentials`.
- `-model-role`: role binding to resolve; defaults to `foreground.coordinator`.
- `-model-profile-id`: explicit profile override when the operator wants to bypass the role default.

For deterministic tests, select the fake provider explicitly:

```powershell
$env:TEMP\genesisd.exe -provider fake
```

An OpenAI-compatible provider can still be selected directly without using the Genesis config resolver:

```powershell
$env:TEMP\genesisd.exe `
  -provider openai-compatible `
  -provider-base-url https://provider.example.com/api `
  -provider-model example-model `
  -provider-api-key-env GENESIS_PROVIDER_API_KEY
```

The provider base URL is configuration, not a kernel route. Include whatever provider-specific prefix the upstream service requires in `-provider-base-url`.

### Provider Setup

`genesisctl provider-setup` is an operator setup surface, not a product shell and not part of turn execution. It writes the Genesis-owned `models.json` and a `secret://...` local credential record so a new machine does not require hand-written protected credential data.

It accepts the API key only from an environment variable or stdin. It does not provide an API-key flag and does not print the secret:

```powershell
$env:GENESIS_PROVIDER_API_KEY = "<provider api key>"
$env:TEMP\genesisctl.exe provider-setup `
  -config-root $HOME\.genesis\config `
  -credential-store-root $HOME\.genesis\credentials `
  -profile-id primary `
  -gateway-route primary `
  -base-url https://provider.example.com/api `
  -model provider-model `
  -credential-ref secret://models/provider/primary
```

The command output contains paths, profile ids, route ids, and the `secret://...` ref only. It never writes provider-specific account flows into the kernel runtime.

## Tool Runtime

The first kernel tool is `shell.exec`. It is deliberately small:

- `plan` mode blocks shell execution fail-closed.
- `default` mode requires a kernel-configured workspace root and uses a kernel-controlled command set from inside that workspace. It does not invoke the operating-system shell, expand environment variables, or execute arbitrary interpreters.
- `yolo` mode is explicit high-trust execution and is the only mode that invokes the operating-system shell. It can only be selected by kernel startup configuration.

The controlled default command set is intentionally narrow: text output, simple file reads, and simple file writes whose real path remains inside the configured workspace. Symlink/junction resolution, parent traversal, absolute path escapes, shell metacharacters, and unsupported commands are blocked before any process is spawned.

The HTTP request cannot select `permission_mode` or `workspace_root`; those are kernel-owned authority fields. Every allowed call first records a `running` operation before process execution, then records completion or failure with tool name, permission mode, command, cwd, status, exit code, bounded stdout/stderr, timestamps, and blocker reason when blocked. Operations are persisted in the event ledger and projected through `GET /sessions/{id}` after restart.

`shell.exec` accepts an optional `idempotency_key` control-plane field. Within the same `session_id` and tool, the first operation for a key owns the effect. Later retries with the same key return the persisted operation projection and do not execute the command again or append new operation events.

### Model Tool Loop

`POST /turn` now supports the same governed `shell.exec` tool through the model loop. The Model Gateway exposes a canonical `shell.exec` descriptor to OpenAI-compatible providers, normalizes provider tool calls, and hands them to the Tool System. The Tool System applies the same `ToolPolicy` used by direct `POST /tools/shell.exec` calls.

When the model requests `shell.exec`, the kernel writes a `model.tool_call` event, executes or blocks the operation, records turn-scoped `operation.*` events, and sends the redacted operation projection back to the provider as structured tool evidence. The provider must then return the final assistant text. `GET /turns/{id}/events` replays the full sequence after restart.

Unsupported model-requested tools fail closed as `tool_call_rejected`; no effect is executed. This does not make email, Feishu, calendar, or other applications kernel features. Those remain external skills, CLIs, and daemons that can be reached through generic governed tools when installed and authorized.

When a provider returns multiple tool calls in one batch, Genesis validates the whole batch before executing any effect. A mixed batch that includes an unsupported or malformed tool fails closed without creating a partial shell effect.

## Turn Events

`GET /turns/{id}/events` is the first HTTP transport for the conceptual `turn.stream` syscall. It reads the append-only ledger and returns the ordered events for one turn id after restart. It is a kernel observation surface for shells and external applications; it is not a UI timeline owner and does not duplicate session lifecycle logic.

When an OpenAI-compatible provider returns token usage, Genesis normalizes it onto the final message as `usage.input_tokens`, `usage.output_tokens`, and `usage.total_tokens`. The same final usage summary is stored in the ledger and appears in session projection after restart.

## Work Registry

The first WorkRegistry surface is a durable record loop:

- `POST /work` creates a work record with `session_id`, required `title`, and required `source_ref`.
- `GET /work/{id}` reads one work record after restart.
- `POST /work/{id}/cancel` records a terminal cancellation decision with required `cancel_authority`, `cancel_reason`, and `cancel_evidence_ref`.

Work records project through `GET /sessions/{id}`. This is not a background scheduler, queue, Feishu task integration, or product workflow owner. It is the kernel ledger primitive that future shells and external applications can use as resumable coordination evidence.

## Accumulation

The first memory loop is explicit and governed:

- `POST /memory/candidates` creates a pending candidate from user-visible text and a required `source_ref`.
- `GET /memory/candidates?status=pending` lists restart-safe candidates for review; omit `status` to list all candidates.
- `GET /memory/candidates/{id}` reads one candidate with source and approval evidence.
- `POST /memory/candidates/{id}/approve` approves a candidate with required `approval_authority`, `approval_reason`, and `approval_evidence_ref`.
- `POST /memory/candidates/{id}/reject` rejects a candidate with required `rejection_authority`, `rejection_reason`, and `rejection_evidence_ref`.
- `POST /memory/candidates/{id}/supersede` marks one candidate as superseded with required authority/reason/evidence and atomically creates a replacement pending candidate.
- `POST /memory/recall` previews which approved memory refs the current Accumulation policy would recall for the supplied `input_items`.
- `POST /turn` recalls only approved candidates and records recalled memory refs on the turn event.

Rejected and superseded candidates are restart-safe review decisions and are not recalled. A replacement candidate created by supersession remains pending until separately approved. The explicit recall preview is read-only; it does not run the model or append ledger events. The first recall strategy is intentionally simple text matching. It proves the governance, provenance, and restart-safe replay loop before adding vector indexes or richer memory policy.

## Operations Records

- Active kernel issues: `docs/operations/kernel-issues.md`
- Ready or retired issue evidence: `docs/operations/kernel-retirement-log.md`

Feishu Base remains a collaboration inbox. The repo records above are the durable project source for active issues, acceptance evidence, and retirement decisions.
