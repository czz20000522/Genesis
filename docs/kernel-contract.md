# Genesis Kernel Contract

- **Status:** initial-contract
- **Created:** 2026-06-22
- **Language target:** Go
- **Distribution target:** single local binary, with shells and applications outside the kernel.

## System Model

Genesis is an agent kernel for LLM execution, not a traditional hardware operating system. It provides the runtime environment where probabilistic model reasoning can safely interact with local resources, user memory, tools, and external applications.

The kernel translates between:

- user or application intent;
- model context and planning;
- governed tool execution;
- durable work and memory state;
- feedback, evidence, and recovery.

## Kernel Syscalls

The stable boundary should be task-oriented, not UI-oriented:

- `turn.submit`: submit one user or external-application turn.
- `turn.stream`: observe model/tool/work events for a turn.
- `tool.invoke`: execute an approved kernel tool request.
- `work.submit`: create resumable work from a turn or application.
- `work.cancel`: cancel or interrupt active work.
- `memory.propose`: propose a memory candidate with source refs.
- `memory.review`: approve, reject, or supersede a candidate.
- `memory.recall`: query approved memory under current context policy.
- `credential.resolve`: resolve a credential ref for an authorized tool.

These names are conceptual. The first implementation may expose HTTP endpoints, but HTTP is transport, not the contract.

Kernel-owned HTTP paths are not versioned. Contract evolution belongs in typed request/response schema changes, capability readiness, and explicit acceptance evidence, not in numbered transport prefixes that become stale compatibility surfaces.

The first HTTP transport for `turn.stream` is `GET /turns/{id}/events`. It reads ordered turn events from the kernel ledger. It is intentionally a minimal observation surface, not a streaming protocol commitment and not a shell/UI timeline owner.

The first protected inspection transport is `GET /capabilities`. It is part of Readiness/Inspection, not an application registry. It lets authorized shells, desktop apps, or external daemons inspect provider/runtime/ledger status, canonical kernel tool capability names, and a safe skill catalog projection. It must not expose filesystem paths, provider credentials, raw secret refs, skill bodies, or application-specific outbound APIs.

## Kernel Planes

### Interface Kernel

Owns request normalization, session identity, event emission, idempotency, and turn admission. It does not know which shell submitted the request.

Session events are the primary fact stream for turn-scoped execution. Session, turn, operation, work, and memory views are read models derived from ledger events, not separate sources of truth. `GET /sessions/{id}` may retain object projections for ergonomic inspection, but its ordered `events` list carries typed event payloads so shells can render or replay the canonical sequence without reassembling facts from unrelated projection arrays.

Short synchronous tool calls are represented as `tool.call` followed by `tool.result`. The `tool.call` event owns the model-provided tool slot, and `tool.result.tool_result.for_event_id` points back to that event id. Operation events may appear between them as execution evidence for effectful tools. Long-running kernel-owned jobs are separate future events; short tools do not create jobs merely to report a result.

Turn idempotency is scoped to explicit `session_id + turn.submit + idempotency_key`. The key is a caller-provided control-plane retry boundary and is not model-visible input. Retrying a completed or failed turn with the same key returns the ledger-backed original turn evidence without calling the provider or executing tools again. A key without an explicit session id is invalid because the caller could not reliably address the same logical retry scope.

Turn admission separates untrusted content from control-plane authority. User or external-application text can contain prompt-injection samples, role labels, tool protocol fragments, logs, or quoted hostile instructions; those strings remain user data and do not grant system, developer, tool, credential, or permission authority. The Interface Kernel may record high-confidence text risks as ingress metadata for inspection. It fails closed only for malformed transport schema, hidden control text, unsupported input item types, or real attempts to set kernel-owned control fields.

Idempotency keys are caller-provided control-plane fields, not model-visible task content. For effectful tool calls, the first admitted `session_id + tool + idempotency_key` owns the effect; retries return the existing operation projection from the ledger without executing the effect again.

Narrative fields such as work titles, cancellation reasons, memory review reasons, memory replacement text, and user input text are semantic content. The kernel must not reject them merely because they contain text that resembles a secret, file path, tool name, or hostile example. Control-plane refs, authorities, session ids, idempotency keys, credential refs, and transport schema remain grammar-gated because they bind authority, routing, replay, or storage identity.

### Model Gateway

Owns provider configuration, model calls, streaming, retries, provider error projection, and data-egress policy hooks. It does not own prompts as product copy.

Provider-native usage fields are normalized into kernel-owned final evidence as `input_tokens`, `output_tokens`, and `total_tokens` when the upstream response provides them. Usage is inspection metadata stored with the final model event; shells may display it, but they do not compute or own it.

The local binary resolves provider startup from Genesis-owned model gateway configuration by default. The canonical user config root is `~/.genesis/config`; `models.json` selects a role-bound gateway profile, the gateway route, provider protocol, model id, timeout, and either an external provider command or a built-in adapter endpoint. The kernel may expose operator flags to select a profile or config root, but it must not require Codex environment variables or Codex credentials for Genesis live operation.

`provider_command` is the preferred long-lived boundary for provider integrations. The kernel writes one JSON request to the configured command's stdin with `protocol=genesis.provider_command`, `session_id`, `turn_id`, `model`, ordered `input_items`, `tool_manifest`, and prior `tool_rounds`. The command writes one JSON response to stdout with `kind=final` plus final text and optional usage, or `kind=tool_calls` plus canonical model tool calls. The command owns vendor SDKs, provider-native HTTP JSON, account flows, and provider credentials. The kernel owns typed request/response validation, provider error projection, tool-loop continuation, and ledger evidence.

The built-in OpenAI-compatible adapter is retained as a local operator convenience and test fixture. It translates the same kernel-owned model request and tool manifest into upstream chat-completions shape, but its provider-native JSON is not the default kernel contract for new providers.

Provider endpoint paths are upstream configuration, not Genesis route contracts. The kernel's own HTTP transport remains unversioned.

The canonical model request carries provenance for each input fragment. Initial kinds are `user_text`, `approved_memory_context`, and `skill_catalog_context`. Public `turn.submit` input remains user or external-application content only; approved memory summaries and skill catalog summaries are kernel-built context fragments. Session and turn-event inspection may expose the ordered `model_input_kinds` list so operators can explain what context categories reached the provider, but it must not expose hidden control fields, skill instruction paths, or full skill bodies.

### Tool System

Owns tool manifests, permission gates, shell/process execution, result envelopes, and tool-loop continuation. Tool specs describe generic effects; application-specific instructions live in skills.

The `ToolRegistry` is the single source for each tool's name, description, input schema, `side_effect_level`, `execution_kind`, and executor binding. `ToolGateway` is the only runtime entry for provider-requested tool calls: it resolves the tool, validates arguments, applies policy, executes the registered executor, and returns model-visible tool results. Capability projection, provider tool manifests, tool preflight, and authority checks must project from that registry rather than duplicating tool-name switches in shells, transports, provider adapters, or the turn loop.

The first model-visible tool spec is canonical `shell_exec` with `side_effect_level=write` and `execution_kind=sandboxed_process`. Provider adapters pass through provider-safe canonical tool ids and do not maintain alternate dotted names. They translate the kernel-generated manifest into provider-native schema shape only; they do not own permission, workspace, execution, idempotency, or evidence semantics. Ledger evidence, capability projection, session projection, provider tool manifests, and registry entries keep the same canonical tool id. A model-requested effectful tool call is admitted only after ToolGateway applies the kernel-owned policy. The model-facing tool result contains terminal-equivalent command evidence or minimal repair feedback only. Full ledger operation projection remains available to authorized inspection surfaces, but model-facing tool results do not duplicate permission mode, policy reasons, operation id, session id, turn id, idempotency key, command/cwd, or event timestamps.

`shell_exec` is a generic terminal/process primitive, not an application API. In `default` mode it uses a deliberately tiny controlled workspace adapter that fail-closes on link aliases that weaken workspace containment; in `yolo` mode it invokes the host shell through the process runtime. Neither path may grow Feishu, email, calendar, document, or channel-specific command aliases inside the kernel.

External skill packages are user-space assets, not kernel applications. The kernel may be configured with explicit skill roots and may scan `SKILL.md` front matter into a read-only skill catalog for model context. This model-visible catalog is metadata only: name and description. The instruction path is an internal read handle and is not exposed to the model. Skill metadata is treated as untrusted before injection; authority-shaped, prompt-injection-shaped, hidden-control, duplicate-name, linked-path, or secret-shaped metadata is excluded rather than repaired into context.

Full skill-body retrieval is not part of the first default model-visible tool surface. If Genesis later needs long skill instructions, they must arrive through a generic resource/context contract or another explicitly reviewed owner path, not through package-specific retrieval tools. Skill prose remains user-space context and never grants kernel authority.

Skill catalog diagnostics are inspection evidence only. The kernel may report path-free exclusion reasons such as missing root, linked path, malformed metadata, unsafe metadata, or duplicate name so an operator can repair the installation, but those diagnostics do not expose the excluded path or body and do not silently repair metadata into model context.

Model-requested tool call batches are preflighted as a unit before any effect executes. If any call in the batch is unsupported or malformed, the entire batch fails closed and no call in that batch may create an operation or external effect. The kernel assigns each admitted tool slot the `tool.call` event id as `tool_call_event_id`, and that event id is the kernel identity for operation idempotency, audit, replay linkage, and `tool.result.for_event_id`. Provider-native `tool_call_id` stays the provider-visible echo id used by provider adapters for assistant tool calls and tool results; duplicate provider ids in one batch fail closed before `tool.call` events or effects. The kernel returns structured `tool_request_invalid` tool results so the model can repair the request, and the repair JSON content does not duplicate provider-native ids. Calls in the same rejected batch that were otherwise valid receive `tool_batch_not_executed` feedback instead of being executed.

Provider tool calls without native ids can still form kernel `tool.call` events and receive tool results carrying `tool_call_event_id`; such calls simply have no provider echo id. Only tool protocol states that cannot form a correlated kernel tool event are fatal provider protocol failures. The kernel does not add application-specific outbound APIs for email, Feishu, calendar, documents, or similar domains; installed skills and external CLIs remain user-space capabilities reachable through generic governed tools.

Tool result taxonomy follows a terminal-equivalent boundary:

- `tool_request_invalid`: the model's tool request failed kernel schema or admission checks and was not executed. The result is repair feedback for the model when protocol state allows it.
- `permission_denied`: the request was structurally valid but permission or policy denied execution. No command effect occurred. The model-visible result contains minimal repair feedback; the full `operation.blocked` evidence with policy reason remains in session/operation inspection.
- `operation.failed`: the command was accepted and executed, but the command process exited nonzero. The kernel returns `exit_code`, `stdout`, and `stderr` as observed command evidence and does not judge command semantics.
- `tool_infrastructure_failed`: the shell runtime, ledger, or tool runtime infrastructure failed. This is not represented as command stderr or a normal command exit. Model Gateway provider errors remain provider failures, not tool results.

Long stdout and stderr are bounded with a head/tail policy. Operation evidence reports `stdout_truncated` or `stderr_truncated`, original byte counts, omitted byte counts, and `output_truncation=head_tail` when truncation occurs. The model-visible stdout or stderr text also includes a visible omission marker such as `[... N bytes omitted ...]` between the preserved head and tail content.

### WorkRegistry

Owns durable work state, cancellation, recovery, status projection, and execution evidence. It does not own application business data.

The first WorkRegistry transport is intentionally a record ledger, not a scheduler. `work.submit` creates a kernel-owned work record with `session_id`, user-visible `title`, and required `source_ref`. `work.cancel` records an explicit terminal cancellation decision with `cancel_authority`, `cancel_reason`, and `cancel_evidence_ref`. Work records are projected through their source session and can be read after restart.

WorkRegistry does not execute background jobs in the first kernel spike. Shells, external daemons, and future applications may submit work records as resumable coordination evidence, but application task semantics, Feishu task objects, calendar events, desktop notifications, queue workers, retries, leases, and scheduler policy remain outside the kernel until they prove generic kernel ownership.

### Accumulation

Owns memory candidates, approval state, safe recall, source refs, and supersession. It does not silently turn model output into truth.

Candidate review decisions are durable owner evidence. Approved candidates may enter recall under context policy; rejected candidates are explicit review outcomes and must remain excluded from recall. A rejected candidate cannot later be approved through the minimal review surface; a future supersession flow must create an explicit replacement decision instead of mutating rejected truth into approved truth.

The first supersession flow is a single kernel ledger decision. Superseding a candidate marks the original candidate as `superseded` with authority, reason, evidence, and replacement candidate id, while creating one replacement candidate in `pending` state from the supplied replacement text and source ref. The replacement does not enter recall until it is independently approved. Supersession is not a text edit, hidden approval, or migration shim for rejected truth.

The first explicit recall transport is a read-only observation surface. `memory.recall` accepts user-context `input_items`, applies the same input validation and hidden-control rejection as `turn.submit`, and returns the approved memory refs selected by the current Accumulation policy. It does not run a model turn, does not append review evidence, and does not mutate candidate state. Turn submission remains responsible for recording recalled memories on the admitted turn event.

### Auth/Credential Plane

Owns runtime client authentication, credential refs, redaction, and secret resolution for authorized effects. Provider-specific account setup belongs to shells or external applications unless it becomes a generic credential primitive.

The first local credential primitive is the Genesis local secret store. On Windows, `secret://...` refs resolve to same-user DPAPI-protected JSON records under `~/.genesis/credentials`. The kernel can decrypt the selected provider key in memory for the Model Gateway, but it must never expose raw secrets in readiness, events, sessions, logs, docs, tests, or model-visible context. Missing, unreadable, or unsupported credentials fail closed as provider readiness blockers.

The operator setup surface may write Genesis-owned model gateway config and `secret://...` local credential records. It is not a shell for turn execution and must not embed provider account workflows, application-specific logic, or Codex credentials into the kernel runtime.

## Explicit Non-Kernel Surfaces

- CLI, WebUI, desktop UI, and future mobile shells.
- External event daemons.
- Feishu, WeChat, email, calendar, document, OCR, and similar app integrations.
- Skill packages and prompt packages.
- Product-specific workflows and domain owners until they prove they are kernel primitives.

## Reference Projects

Reasonix is a reference for Go single-binary distribution, config-driven tool/plugin loading, and one transport-agnostic controller behind multiple frontends.

Codex is a reference for tool approval, sandboxing, session/turn/event rigor, and separation between core protocol and shells.

Neither project is a blueprint to copy wholesale. Genesis should stay smaller and more generic than a coding agent.

For every non-trivial kernel boundary change, the issue or retirement evidence must state whether the change is aligned with Codex, aligned with Reasonix, intentionally different, or a known drift risk. The comparison is about control-plane ideas: model-visible surface, tool result taxonomy, permission/sandbox ownership, registry boundaries, event/ledger recovery, and shell/application separation. It is not a maturity checklist and must not justify copying application-specific behavior into the kernel.
