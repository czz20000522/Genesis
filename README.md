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

Provider, role, invocation, and task graph boundaries are defined in `docs/kernel-contract.md`. In short: provider adapters translate model backends; roles are application semantics; only kernel-created invocations with validated capability grants can execute; TaskGraphOwner records project work topology and is not a kernel scheduler.

Kernel HTTP routes stay unversioned. The durable contract is the task/tool schema and ledger evidence, not a numbered path prefix.

## Behavior Specs

BDD acceptance specs live under `features/`. They describe expected kernel
behavior in reviewable Gherkin scenarios before those expectations are wired to
step definitions. Future automation should drive public kernel commands and
projections, not private helpers or UI copy.

Production requirements live under `docs/requirements/`. They describe target
kernel contracts before implementation slices are chosen. Issues and BDD
features should point back to these requirements instead of relying on chat-only
design discussion.

The process contract lives at `docs/process.md`. It keeps requirements, design,
implementation plans, and issues as separate document types.

Design documents live under `docs/design/` and answer owner, boundary, data
flow, protocol, failure, permission, recovery, and observability questions.
Implementation plans live under `docs/implementation-plans/` and state how
phased delivery will land with tests and evidence.

## Initial Kernel Spike

Build the first runnable kernel binary:

```powershell
$root = Join-Path (Get-Location) ".genesis-live\manual"
New-Item -ItemType Directory -Force "$root\bin" | Out-Null
D:\software\Go\bin\go.exe build -o "$root\bin\genesisd.exe" .\cmd\genesisd
```

Build the optional operator setup tool:

```powershell
D:\software\Go\bin\go.exe build -o "$root\bin\genesisctl.exe" .\cmd\genesisctl
```

## Development Verification

Use the local Go toolchain directly when verifying the kernel on Windows:

```powershell
D:\software\Go\bin\go.exe test ./... -count=1
D:\software\Go\bin\go.exe build ./...
```

Race verification requires cgo and a visible MinGW gcc:

```powershell
$env:Path = "D:\software\Go\bin;C:\Users\Tomczz\AppData\Local\Microsoft\WinGet\Packages\BrechtSanders.WinLibs.POSIX.UCRT_Microsoft.Winget.Source_8wekyb3d8bbwe\mingw64\bin;D:\software\msys64\ucrt64\bin;D:\software\msys64\usr\bin;$env:Path"
$env:CGO_ENABLED = "1"
D:\software\Go\bin\go.exe test -race ./internal/kernel -count=1
```

The MSYS2 install root for this workstation is `D:\software\msys64`; it is optional for normal builds but useful for a complete Unix-like local toolchain. Race evidence must say whether it was run locally, run in CI, or not run.

Run it with an explicit ledger path:

```powershell
"$root\bin\genesisd.exe" `
  -addr 127.0.0.1:8765 `
  -ledger "$root\events.jsonl" `
  -runtime-token local-dev-token
```

Minimal HTTP surface:

- `GET /ready`
- `POST /turn`
- `GET /turns/{id}/events`
- `GET /turns/{id}/audit`
- `GET /sessions/{id}`
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

`genesisd` defaults to Genesis-owned model gateway configuration rather than a fake model. The fake provider is a lab/test fixture and is not production-ready unless an operator explicitly enables lab fake mode.

Protected routes require `Authorization: Bearer <runtime-token>`. `GET /ready` is the only unauthenticated route, but readiness is `blocked` when no runtime token is configured because the kernel cannot accept protected work.

`POST /turn` accepts an optional `idempotency_key` when `session_id` is explicit. This key belongs to the Interface Kernel control plane. Retrying the same logical turn returns the original ledger-backed result and must not call the provider or execute tools a second time.

## Skill Catalog

External skills are user-space assets. Genesis can make installed skill metadata visible to the model without adding application-specific code to the kernel.

`genesisd` scans configured skill roots for `SKILL.md` front matter and injects a concise model-context catalog containing each skill's name and description. Missing roots and malformed skill files are ignored. Skill metadata that looks like authority forgery, prompt injection, hidden control text, or a secret is excluded. Full skill bodies and instruction paths are not loaded into every turn.

By default, `genesisd` looks at the current user's global agent skill root:

```powershell
$HOME\.agents\skills
```

Operators can replace or extend roots with `GENESIS_SKILL_ROOTS` or repeated `-skill-root` flags:

```powershell
$env:GENESIS_SKILL_ROOTS = "$HOME\.agents\skills;$HOME\.genesis\skills"
"$root\bin\genesisd.exe" -skill-root D:\tools\custom-skills
```

This does not make Feishu, email, calendar, or any other application a kernel feature. The initial kernel exposes skill metadata only; full skill-body hydration is deferred until a generic resource/context contract exists. Installed CLIs are still invoked through governed tools such as `shell_exec` under kernel permission policy.

## Provider Configuration

`genesisd` defaults to Genesis-owned model gateway configuration. It reads `models.json` from the Genesis config root, resolves the selected role/profile, selects the gateway route, and starts the configured provider boundary.

Useful operator flags:

- `-config-root`: directory containing `models.json`; defaults to `~/.genesis/config`.
- `-credential-store-root`: directory containing local credential records; defaults to `~/.genesis/credentials`.
- `-model-role`: role binding to resolve; defaults to `foreground.coordinator`.
- `-model-profile-id`: explicit profile override when the operator wants to bypass the role default.

For deterministic lab tests, select the fake provider with explicit lab mode:

```powershell
"$root\bin\genesisd.exe" -provider fake -allow-lab-fake-provider
```

The long-lived provider boundary is `provider_command`: an external executable reads one typed Genesis model request from stdin and writes one typed provider response to stdout. The command owns vendor SDKs, HTTP JSON, account flows, and provider credentials; the kernel owns only the typed request, typed response, readiness, turn loop, and ledger evidence.

```powershell
"$root\bin\genesisd.exe" `
  -provider provider_command `
  -provider-command D:\tools\genesis-provider-openai.exe `
  -provider-command-arg --profile `
  -provider-command-arg primary `
  -provider-command-env GENESIS_PROVIDER_PROFILE=primary `
  -provider-model example-model
```

`-provider-command-env` is limited to non-sensitive adapter configuration such as a profile or route name. Do not pass API keys, bearer tokens, passwords, `secret://...` refs, or other credentials through this flag or through `models.json` provider-command env entries; the kernel rejects credential-shaped entries.

An OpenAI-compatible provider can still be selected directly without using the Genesis config resolver:

```powershell
"$root\bin\genesisd.exe" `
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
"$root\bin\genesisctl.exe" provider-setup `
  -config-root $HOME\.genesis\config `
  -credential-store-root $HOME\.genesis\credentials `
  -profile-id primary `
  -gateway-route primary `
  -base-url https://provider.example.com/api `
  -model provider-model `
  -credential-ref secret://models/provider/primary
```

The command output contains paths, profile ids, route ids, and the `secret://...` ref only. It never writes provider-specific account flows into the turn loop. New provider integrations should prefer `provider_command`; the OpenAI-compatible setup remains an operator convenience for the current built-in adapter.

For a clean live-provider first run, use the operator runbook and script:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\first_run_live_llm_acceptance.ps1 -Help
```

The runbook lives at `docs/operations/live-llm-first-run-acceptance.md`. It covers provider setup, `genesisd` startup through Genesis config, `/ready`, one real `/turn`, timeline/events/context inspection, restart replay, and a missing-credential failure probe.

## Tool Runtime

The primary effectful process tool is `shell_exec`. It is deliberately small:

- `plan` mode blocks shell execution fail-closed.
- `default` mode requires a kernel-configured workspace root and uses a kernel-controlled command set from inside that workspace. It does not invoke the operating-system shell, expand environment variables, or execute arbitrary interpreters.
- `yolo` mode is explicit high-trust execution and is the only mode that invokes the operating-system shell. It can only be selected by kernel startup configuration.

Genesis resolves these user-facing modes into separate kernel policy facts before execution: `authority_policy` decides whether the requested effect class is admissible, `sandbox_profile` names the executor isolation actually used, and `approval_policy` decides whether escalation can be requested. The default approval policy is `never`; when configured as `on_request`, write tools are blocked at admission with structured `approval_required` feedback until an approval owner exists. Stronger sandbox profiles that are configured before the executor can enforce them fail closed instead of degrading to host execution.

The tool surface is generated from `ToolRegistry` and executed through `ToolGateway`. The current model-visible tools are `shell_exec`, `resource_read`, `source_tree`, `source_read`, `job_status`, and `job_cancel`. `resource_read` plus `source_tree`/`source_read` are narrow read bridges for admitted resources and material snapshots; large project exploration should use governed shell/rg or a user-space code-intelligence adapter before adding more kernel tools. Capability projection, provider tool manifests, model tool preflight, and direct `shell_exec` HTTP execution all project from the registry. The turn loop and provider adapters do not special-case shell execution or job control.

The controlled default command set is intentionally narrow: text output, simple file reads, and simple file writes whose real path remains inside the configured workspace. Symlink/junction resolution, parent traversal, absolute path escapes, shell metacharacters, and unsupported commands are blocked before any process is spawned.

The HTTP request cannot select `permission_mode`, `workspace_root`, `authority_policy`, `sandbox_profile`, or `approval_policy`; those are kernel-owned control-plane fields. Foreground HTTP calls first record a `running` operation before process execution, then record completion or failure with tool name, resolved permission profile, command, cwd, status, exit code, bounded stdout/stderr, timestamps, and blocker reason when blocked. Operations are persisted in the event ledger and projected through `GET /sessions/{id}` after restart.

Direct `POST /tools/shell_exec` is a transport projection over the same kernel owner path. Foreground requests return an `OperationProjection`. Requests with `timeout_sec > 180` enter the managed-job admission path; the current local managed executor requires the host sandbox profile, so `default`/controlled-workspace requests return a blocked operation until a controlled managed executor exists. Accepted managed jobs return a managed `JobProjection` receipt with HTTP 202 when a new job is accepted, and the latest job projection with HTTP 200 for an idempotent retry. Omitted `timeout_sec` defaults to 30 seconds; explicit non-positive values are invalid.

`shell_exec` accepts an optional `idempotency_key` control-plane field. Within the same `session_id` and tool, the first foreground operation or managed job for a key owns the effect. Later retries with the same key return the persisted operation or job projection and do not execute the command again or append duplicate lifecycle start events.

### Model Tool Loop

`POST /turn` supports the registry-generated model tool surface through the model loop. The Model Gateway exposes kernel-generated `shell_exec`, `job_status`, and `job_cancel` manifests to providers, normalizes provider tool calls, and hands them to ToolGateway. ToolGateway applies the same kernel policy path used by direct `POST /tools/shell_exec` calls.

When the model requests `shell_exec`, the kernel writes a `tool.call` event, executes or blocks the operation, records turn-scoped `operation.*` events for foreground execution, writes a `tool.result` event whose `for_event_id` points back to the `tool.call`, and sends terminal-equivalent command evidence, managed-job receipt, or minimal repair feedback back to the provider. `job_status` and `job_cancel` inspect or request cancellation for kernel-issued managed-job handles without exposing process ids, signals, or host termination mechanics to the model. Full permission and audit evidence stays in session/operation/job inspection. The provider must then return the final assistant text. `GET /turns/{id}/events` replays the full sequence after restart.

Unsupported model-requested tools fail closed as `tool_call_rejected`; no effect is executed. This does not make email, Feishu, calendar, or other applications kernel features. Those remain external skills, CLIs, and daemons that can be reached through generic governed tools when installed and authorized.

When a provider returns multiple tool calls in one batch, Genesis validates the whole batch before executing any effect. A mixed batch that includes an unsupported or malformed tool fails closed without creating a partial shell effect.

## Turn Events

`GET /turns/{id}/events` is the first HTTP transport for the conceptual `turn.stream` syscall. It reads the append-only ledger and returns the ordered events for one turn id after restart. It is a kernel observation surface for shells and external applications; it is not a UI timeline owner and does not duplicate session lifecycle logic.

`GET /turns/{id}/audit` projects replay-oriented turn facts with event types, operation status, usage, failure codes, and output truncation metadata. It is separate from the user timeline, raw event inspection, and provider context. It does not expose provider credentials or raw command output beyond bounded redacted previews.

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

- Development process: `docs/process.md`
- Kernel requirements: `docs/requirements/`
- Kernel designs: `docs/design/`
- Kernel implementation plans: `docs/implementation-plans/`
- Active kernel issues: `docs/operations/kernel-issues.md`
- Closed issue evidence: `docs/operations/kernel-retirement-log.md`

Feishu Base remains a collaboration inbox. The repo records above are the durable project source for active issues, acceptance evidence, and retirement decisions.
