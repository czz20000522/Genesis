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

## Agent Kernel vs Agent Framework

An agent framework is a reasoning organization layer. It usually defines how one agent arranges prompts, tools, memory, planners, retries, and callbacks for a specific product or task family. It can be sophisticated, but its facts are normally local to that agent or application.

An agent kernel is an authority execution layer. It is shared by multiple applications, skills, and agents. It owns the facts that cannot be forged by a caller: tool results, checkpoints, memory truth, credential resolution, sandbox decisions, event log entries, and audit replay. Applications can be clever, but they cannot mint those facts on their own.

Genesis is not trying to be a larger prompt-plus-tools framework. It is the stable execution contract beneath those frameworks. An application can bring its own skill package, interaction model, UI, daemon, or agent loop, but once it wants model context, tool execution, memory, credential access, work state, or audit evidence to count as Genesis truth, it must go through the kernel.

The layer split is:

- Application: intent and experience layer. It decides what user experience or domain workflow it wants.
- LLM: probabilistic planning layer. It proposes actions from kernel-projected context and available tool manifests.
- Tool: reality interface. It touches files, processes, resources, credentials, or external programs through kernel governance.
- Event log: system fact layer. It records what the kernel accepted, executed, blocked, observed, or recovered.
- Kernel: authority layer. It validates, authorizes, executes, records, projects, and replays.

Codex and Reasonix are strong agent products with kernel-like runtimes inside them. They are useful references because their mature parts separate core protocol, tools, sandboxing, events, and shells. Genesis takes that runtime idea and makes it the platform contract for multiple user-space applications instead of one coding-agent product.

## System Boundary / Box Model

The Genesis box is the whole governed LLM runtime. The kernel is the control and fact boundary inside that box, not the whole box.

The LLM is the operator. It reasons over the context the kernel gives it and requests tools when it needs to touch the outside world. The kernel is not the model and does not outsource authority to the model. It decides which context is visible, which tool requests are valid, which effects are authorized, where facts are recorded, and how later projections are rebuilt.

Tools are governed touchpoints to reality. A tool can read a file, run a process, resolve a credential, or operate on a resource only through kernel-owned schema validation, permission policy, sandbox selection, evidence recording, and bounded result projection. A tool is not an application domain API merely because an application can be driven through it.

Skills are user-space instruction packages. They can teach the model how to use installed capabilities, describe workflows, or provide examples, but they are not kernel APIs and do not grant authority. Skill metadata may enter a bounded index. Full skill instructions must enter model context only through a reviewed generic context or resource path, not through a domain-specific kernel shortcut.

Applications are user-space compositions. An application may include a skill, adapter, daemon, UI, CLI, document package, or resource bundle. It submits turns, provides external events, reads kernel projections, and may install tools or CLIs that the model can use through governed generic tools. It does not own provider context assembly, tool permission, sandbox policy, ledger truth, memory truth, credential resolution, audit replay, or compaction.

Shells and adapters are also user-space. CLI, WebUI, desktop UI, Feishu daemon, email listener, and similar entry points translate external interaction into kernel commands and projections. They must not rebuild provider context, execute permission decisions, write ledger facts, or maintain a second memory store. If an entry point needs a shape that the current kernel command does not support, the fix is a kernel-owned command or an anti-corruption adapter, not duplicated lifecycle logic in that shell.

A practical boundary test is the domain-name test. If a capability is named after a concrete domain such as Feishu, email, calendar, calculator, document, OCR, insurance, or medical workflow, it starts outside the kernel. It can enter the kernel only after being reduced to a generic primitive such as `turn.submit`, `tool.invoke`, `resource.read`, `resource.write`, `credential.resolve`, `work.submit`, `work.cancel`, `memory.review`, or `audit.replay`.

Under that rule, a calculator skill is not kernel. It is user-space instruction that can ask the model to use a governed process tool for exact arithmetic. A Feishu daemon is not kernel. It listens to Feishu, submits a turn, and renders or sends results through user-space capabilities. WebUI is not allowed to assemble provider context. Applications are not allowed to write memory truth directly.

## Protocol Boundary Owner Pattern

Any surface that crosses out of the Genesis canonical world must pass through a
protocol boundary owner. A boundary owner translates an external protocol into
stable Genesis primitives on the way in, and translates controlled Genesis
actions or projections back into the external protocol on the way out.

The rule is:

```text
External protocol <-> Boundary owner <-> Genesis canonical primitive
```

This pattern applies to model providers, external application connectors,
future WebUI/CLI/desktop shells, resource intake, and credential-backed
integrations. The owner name changes, but the boundary discipline stays the
same.

Common invariants:

- external protocols do not directly enter core owners;
- external identity does not directly equal system identity;
- external errors do not directly equal system errors;
- external paths and ids do not directly become public system ids;
- external credentials are not given to the LLM or prompt context;
- external actions pass through typed request/action/outcome records;
- shells and connectors consume kernel events or projections instead of
  rebuilding kernel truth;
- boundary adapters translate shape and transport, not authority.

Model Gateway is the provider protocol boundary:

```text
Genesis Model Protocol <-> OpenAI / DeepSeek / Claude / local model
```

Application Connector Runtime is the external application protocol boundary:

```text
External Event / Action Protocol <-> Genesis Application Event / Request / Projection / Command
```

Future WebUI, CLI, and desktop shells must follow the same pattern. They may
submit user requests and render kernel events, timelines, audits, context
inspection, connector receipts, or application projections, but they must not
assemble provider context, decide tool authority, mint kernel events, write
memory truth, or create a parallel transcript truth.

The LLM is not a boundary owner. It produces semantic intent. The boundary
owner decides how that intent becomes an admitted request, command, action, or
receipt under the relevant authority and recovery model.

## Provider, Role, Invocation, And TaskGraph Boundaries

This section records the boundary decision for future provider routing, role profiles, multi-agent execution, and project task graphs. The decision comes from the Genesis kernel rewrite discussion and from reviewing the Python TaskGraph and multi-agent design: Genesis should keep the useful control-plane ideas, but it must not copy the old implementation shape or let application roles become kernel authority.

The layer split is:

```text
Application / Agent Framework  owns strategy, role taxonomy, graph templates, and user experience
TaskGraphOwner                 owns project work topology facts
Agent Kernel                   owns controlled execution facts
Model Gateway                  owns Genesis model protocol and model-policy resolution
Provider Adapter               owns vendor payload translation
Real Provider                  owns token generation and native APIs
```

Provider is model backend adaptation, not an agent concept. A provider adapter translates Genesis model protocol into a vendor-native request and translates the vendor response back into Genesis model response shapes. The kernel must not know DeepSeek, OpenAI, Claude, local model, or future vendor message formats outside an adapter boundary. A `provider_command` adapter may behave like a local router or translator, but that router is still provider middleware, not role logic and not application business logic.

Role is application or agent-framework semantics, not provider semantics and not kernel authority. Labels such as `parent`, `child`, `coder`, `reviewer`, `analyst`, `planner`, `executor`, or `verifier` describe how an application wants to organize work. They may appear in an application-defined `AgentProfile` with instruction refs, skill refs, output contract, preferred model capability, requested tool access, context policy, and budget request. The kernel must not treat a role label as a permission grant, provider route, credential identity, sandbox profile, or execution authority.

Names such as `DeepSeek reviewer`, `OpenAI parent`, or `Claude child` are invalid architecture terms. The correct shape is:

```text
AgentProfile(role=reviewer, model_policy_ref=cheap-read-only)
  -> Kernel creates AgentInvocation(capability_grant=read-only, context_scope=diff)
  -> Model Gateway resolves model_policy_ref to a gateway profile
  -> Provider Adapter calls the selected real provider
```

AgentInvocation is a kernel runtime fact. An invocation is the admitted execution identity for one bounded model-backed run or agent-framework request. It binds `invocation_id`, optional `parent_invocation_id`, `session_id`, `principal`, `agent_profile_ref`, validated `capability_grant`, `context_scope`, `budget_lease`, optional `source_graph_ref`, optional `source_node_ref`, terminal state, and bounded result delivery. The invocation, not the role label, may consume context, call tools, spend budget, write invocation events, and return results.

Only an invocation with a validated CapabilityGrant may execute. A `CapabilityGrant` is kernel-owned execution authority. It must be checked against the user, application, session, parent invocation, and active permission profile before any model-backed tool call, job, operation, credential resolution, or workspace effect runs. A child invocation can receive only a subset of the authority available to its parent or admitting application. Model output, role labels, graph node kinds, provider routes, and skill text never grant authority by themselves.

TaskGraph is project-level work topology fact, not a kernel scheduler. TaskGraphOwner is a platform owner that may run in the same binary as the kernel, but it is not Agent Kernel core. It owns `graph_id`, `node_id`, `edge_id`, graph edit admission, node status transitions, dependency edges, evidence refs, projection, and replay for project work topology. It does not execute tasks, choose providers, assign role authority, grant tools, manage jobs, or write kernel execution events.

TaskGraph nodes do not grant authority. A node kind such as `review`, `implementation`, `repair`, `verification`, `investigation`, or `decision` may help an application organize work, but it cannot imply read access, write access, shell access, network access, provider choice, role choice, or credential access. Execution requires a separate AgentInvocation whose CapabilityGrant was validated by the kernel.

Applications and parent orchestrators may propose graph edits and delegation. TaskGraphOwner admits or rejects graph changes. The Agent Kernel admits or rejects execution. The normal flow is:

```text
Parent sees bounded TaskGraph projection
Parent proposes a graph edit or delegation for node N
Application resolves the AgentProfile and requested grant
TaskGraphOwner validates graph/node intent
Kernel validates CapabilityGrant <= parent/application authority
Kernel creates AgentInvocation linked to source_graph_ref/source_node_ref
Child invocation runs through Model Gateway and ToolGateway
Kernel writes invocation/tool/job/operation facts
TaskGraphOwner attaches evidence refs and updates node status after admitted graph edits
Parent receives a bounded result projection
```

Model-proposed graph edits are semantic proposals. The model may propose a title, goal, dependency, evidence summary, or intended role. The system generates `graph_id`, `node_id`, `edge_id`, revision, timestamps, status transitions, and accepted evidence refs. If the model supplies hidden control fields or attempts to override graph identity, invocation identity, capability grant, provider route, sandbox profile, or credential refs, the request fails closed before execution or graph mutation.

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

The first HTTP transport for `turn.stream` is `GET /turns/{id}/events`. It reads ordered turn events from the kernel ledger through a raw event inspection projection. It is intentionally a minimal observation surface, not a streaming protocol commitment and not a shell/UI timeline owner. Raw event inspection keeps typed event envelopes and correlation ids for authorized debugging, but projects redacted payload text rather than raw credential-shaped evidence.

The replay/audit read model is `GET /turns/{id}/audit`. It projects event types, operation status, provider-context input kinds, final usage, failure codes, and output truncation metadata. It is not the ordinary UI timeline and not the next provider context. Audit replay may include replay identifiers such as event id and operation id, but it must not expose provider credentials, raw secret-bearing stderr/stdout, or full command output when a bounded preview is enough.

The user-facing timeline read model is `GET /sessions/{id}/timeline`. It merges `tool.call` and `tool.result` causality into stable tool items, projects ordinary user and assistant message items, and omits kernel-owned event, operation, provider-call, checkpoint, and audit identity from ordinary UI items. Raw events remain available through the event inspection surface; WebUI, desktop shells, and external apps must not rebuild chat timelines by rendering raw ledger events directly.

The timeline projection must distinguish live turn rendering from settled turn rendering. While a turn is running, a shell may show an expanded processing group with compact progress summaries and grouped tool activity. The live elapsed label, such as `正在处理 45s` or `正在处理 1m 5s`, is computed projection state and must not be persisted as per-second facts. When the assistant final message starts streaming, the same processing group should default to a compact summary such as fixed duration and tool counts, while the final assistant message streams as the primary timeline content. This collapse is a projection state, not a rewrite of ledger facts. A user may reopen the processing group, and shells may keep that open/closed preference locally.

Detailed UI surfaces are projections layered under timeline items. A processing group can expand into work-group detail, a tool group can expand into operation detail, and an operation detail can expose bounded command, status, duration, visible output, and truncation metadata. Tool failures, command failures, job failures, stderr, and long stdout/stderr are detail or diagnostics material; they do not become ordinary chat rows and do not force a settled processing group open. The ordinary timeline still stays separate from raw events, audit replay, context inspection, and debug trace. Approval and user-input prompts are control surfaces, not assistant messages. They may appear as standalone user-action nodes because execution is waiting on a user decision, but they must not be rendered as if the model authored them.

The runtime context inspection read model is `GET /turns/{id}/context`. It is a diagnostics surface for the per-turn provider-visible snapshot recorded on `turn.submitted`: user input, model input kinds, model-visible tool manifest, skill summaries, recalled memory refs, provider status, and the resolved permission profile. That profile separates the user-facing `permission_mode` from `authority_policy`, `sandbox_profile`, and `approval_policy`; shells and providers must not infer execution authority from the mode string alone. It does not store or project the fully rendered model-context text in raw events. It is not part of the chat timeline. If an older ledger entry lacks a context snapshot, the projection must report `snapshot_unavailable` rather than pretending the current runtime state was the historical context.

The first protected inspection transport is `GET /capabilities`. It is part of Readiness/Inspection, not an application registry. It lets authorized shells, desktop apps, or external daemons inspect provider/runtime/ledger status, canonical kernel tool capability names, and a safe skill catalog projection. It must not expose filesystem paths, provider credentials, raw secret refs, skill bodies, or application-specific outbound APIs.

## Kernel Planes

### Interface Kernel

Owns request normalization, session identity, event emission, idempotency, and turn admission. It does not know which shell submitted the request.

Session events are the primary fact stream for turn-scoped execution. Session, turn, operation, work, and memory views are read models derived from ledger events, not separate sources of truth. `GET /sessions/{id}` may retain object projections for ergonomic inspection, but those top-level objects and its ordered `events` list are redacted inspection projections, not raw ledger records. Shells can render or replay the canonical sequence without reassembling facts from unrelated projection arrays, but they must not treat session JSON as the append-only evidence store.

Short synchronous tool calls are represented as `tool.call` followed by `tool.result`. The `tool.call` event owns the model-provided tool slot, and `tool.result.tool_result.for_event_id` points back to that event id. Operation events may appear between them as execution evidence for effectful tools. Long-running kernel-owned jobs are separate future events; short tools do not create jobs merely to report a result.

Turn idempotency is scoped to explicit `session_id + turn.submit + idempotency_key`. The key is a caller-provided control-plane retry boundary and is not model-visible input. Retrying a completed or failed turn with the same key returns the ledger-backed original turn evidence without calling the provider or executing tools again. A key without an explicit session id is invalid because the caller could not reliably address the same logical retry scope.

Turn admission separates untrusted content from control-plane authority. User or external-application text can contain prompt-injection samples, role labels, tool protocol fragments, logs, or quoted hostile instructions; those strings remain user data and do not grant system, developer, tool, credential, or permission authority. The Interface Kernel may record high-confidence text risks as ingress metadata for inspection. It fails closed only for malformed transport schema, hidden control text, unsupported input item types, or real attempts to set kernel-owned control fields.

Idempotency keys are caller-provided control-plane fields, not model-visible task content. For effectful tool calls, the first admitted `session_id + tool + idempotency_key` owns the effect regardless of whether the effect became a foreground operation or a managed job; retries return the existing operation or job projection from the ledger without executing the effect again.

Narrative fields such as work titles, cancellation reasons, memory review reasons, memory replacement text, and user input text are semantic content. The kernel must not reject them merely because they contain text that resembles a secret, file path, tool name, or hostile example. Control-plane refs, authorities, session ids, idempotency keys, credential refs, and transport schema remain grammar-gated because they bind authority, routing, replay, or storage identity.

### Model Gateway

Owns provider configuration, model calls, streaming, retries, provider error projection, and data-egress policy hooks. It does not own prompts as product copy.

Provider-native usage fields are normalized into kernel-owned final evidence as `input_tokens`, `output_tokens`, `total_tokens`, `cache_hit_tokens`, and `cache_miss_tokens` when the upstream response provides them. Usage is inspection metadata stored with the final model event; shells may display it, but they do not compute or own it. After each provider response with usage, the Model Gateway writes `model.context.accounted` evidence for the exact provider context exchange: model input kinds, included history turn ids, compacted-through boundary, provider usage, provider-backed processed input tokens when available, and model-visible tool round/call/result counts. For DeepSeek/OpenAI-compatible responses, processed input tokens come from `prompt_cache_miss_tokens`, not from kernel-local tokenization. Field meanings are tracked in `docs/field-reference.md`. This accounting is input evidence for kernel compaction; it is not the compaction executor.

The local binary resolves provider startup from Genesis-owned model gateway configuration by default. The canonical user config root is `~/.genesis/config`; `models.json` selects a role-bound gateway profile, the gateway route, provider protocol, model id, timeout, and either an external provider command or a built-in adapter endpoint. The kernel may expose operator flags to select a profile or config root, but it must not require Codex environment variables or Codex credentials for Genesis live operation.

`provider_command` is the preferred long-lived boundary for provider integrations. Before every provider call, the Model Gateway rebuilds a provider-context projection from the ledger: same-session conversation history, submitted model input fragments, the model-visible tool manifest, and prior model-visible tool call/result rounds. The kernel writes that projection to the configured command's stdin with `protocol=genesis.provider_command`, `session_id`, `turn_id`, `model`, ordered `input_items`, `tool_manifest`, and `tool_rounds`. The command writes one JSON response to stdout with `kind=final` plus final text and optional usage, or `kind=tool_calls` plus canonical model tool calls. Valid tool-call argument JSON is carried in `arguments`; malformed upstream argument text is carried in `raw_arguments` so ToolGateway can return repair feedback. Provider command requests keep provider-visible `tool_call_id` correlation but omit kernel-owned event, operation, lease, permission, checkpoint, and audit identity. The command owns vendor SDKs, provider-native HTTP JSON, account flows, and provider credentials. The kernel owns typed request/response validation, provider error projection, tool-loop continuation, and ledger evidence. Provider commands run with explicit environment variables only; daemon environment variables are not inherited. Explicit provider-command environment entries are for non-sensitive adapter configuration such as profiles or route names. Provider credentials must stay in the credential plane or in the external command's own identity environment; secret-shaped env names or values fail closed instead of becoming Genesis config. Provider command stderr is redacted before any HTTP, event, session, or ledger projection.

The built-in OpenAI-compatible adapter is retained as a local operator convenience and test fixture. It translates the same kernel-owned model request and tool manifest into upstream chat-completions shape, but its provider-native JSON is not the default kernel contract for new providers.

Provider endpoint paths are upstream configuration, not Genesis route contracts. The kernel's own HTTP transport remains unversioned.

The canonical model request carries provenance for each input fragment. Initial kinds are `conversation_history_context`, `skill_index_context`, `approved_memory_context`, `kernel_observation_context`, and `user_text`. Public `turn.submit` input remains user or external-application content only; same-session history, budgeted skill index metadata, approved memory summaries, and kernel observation summaries are kernel-built context fragments. Session history is projected by the Model Gateway from completed ledger turns in the same session. Kernel observations such as terminal managed-job facts are projected by the Model Gateway from undelivered ledger facts, then marked delivered only after provider success. Shells, WebUI, desktop apps, provider commands, and external daemons must not synthesize their own model-visible conversation history or observation delivery. Session and turn-event inspection may expose the ordered `model_input_kinds` list so operators can explain what context categories reached the provider, but it must not expose hidden control fields, skill instruction paths, or full skill bodies.

Context compaction is a kernel compaction-runner responsibility. Triggers may come from the turn loop, a future manual shell command, or another owner, but those triggers only submit a typed kernel compaction command; they must not summarize, truncate, replace history, or write compaction state themselves. When a configured context window and auto-compact threshold are reached by provider-reported input usage, the turn loop submits an auto compaction command to the kernel runner. The runner summarizes the older completed conversation region, writes `context.compaction.started` and `context.compaction.completed` ledger events, and keeps subsequent provider projections to the latest summary plus a recent verbatim tail. The compaction source is made from completed conversation turns only; when a completed turn contains tool work, the source preserves the model-visible `tool.call` plus matching `tool.result` pair before the assistant final answer, without exposing kernel event ids or operation ids as summary content. The tail boundary always preserves a minimum number of complete conversation turns. If `RecentTailTokens` is configured, the selector may keep additional complete recent turns whose provider-backed processed input token accounting fits the budget. If a candidate turn lacks provider-backed accounting, the selector stops expanding the tail instead of estimating from text. A failed compaction writes `context.compaction.failed` with a structured reason and leaves the previous provider projection in place; the completed user response must not be turned into a model failure. After a summarizer failure, the runner may write `context.compaction.deferred` with bounded retry/backoff evidence instead of immediately reattempting on the next eligible turn. Completed compaction evidence records the triggering provider usage and cache-stability metrics derived from provider-backed context accounting for the compacted region. The compaction source and result are auditable in the ledger, but shells and external daemons must not perform their own truncation, summary, or replay rewriting. User-facing timelines may show only progress/completion/failure notices for compaction and must not render the internal summary as a chat message.

### Tool System

Owns tool manifests, permission gates, shell/process execution, result envelopes, and tool-loop continuation. Tool specs describe generic effects; application-specific instructions live in skills.

The `ToolRegistry` is the single source for each tool's name, description, input schema, `side_effect_level`, `execution_kind`, and executor binding. `ToolGateway` is the only runtime entry for provider-requested tool calls: it resolves the tool, validates arguments, applies policy, executes the registered executor, and returns model-visible tool results. Capability projection, provider tool manifests, tool preflight, and authority checks must project from that registry rather than duplicating tool-name switches in shells, transports, provider adapters, or the turn loop.

The current model-visible generic tool surface contains canonical `shell_exec`, `job_status`, and `job_cancel`. `shell_exec` is the primary effectful process primitive with `side_effect_level=write` and `execution_kind=sandboxed_process`. `job_status` and `job_cancel` are generic kernel-control tools for kernel-issued managed-job handles, not application APIs and not process-control surfaces. Provider adapters pass through provider-safe canonical tool ids and do not maintain alternate dotted names. They translate the kernel-generated manifest into provider-native schema shape only; they do not own permission, workspace, execution, idempotency, job lifecycle, cancellation truth, or evidence semantics. Ledger evidence, capability projection, session projection, provider tool manifests, and registry entries keep the same canonical tool ids. A model-requested effectful tool call is admitted only after ToolGateway applies the kernel-owned policy. The model-facing tool result contains terminal-equivalent command evidence, managed-job receipt/status/cancel feedback, or minimal repair feedback only. Full ledger operation and job projection remain available to authorized inspection surfaces, but model-facing tool results do not duplicate permission mode, policy reasons, operation id, session id, turn id, idempotency key, command/cwd, process id, signal, or event timestamps.

`plan`, `default`, and `yolo` are user-facing permission modes, not executor implementations. The kernel resolves them before admission: `plan` resolves to read-only authority and a read-only sandbox profile; `default` resolves to workspace-write authority and the controlled-workspace sandbox profile; `yolo` resolves to full-access authority and the host sandbox profile. The default approval policy is `never`. When kernel configuration selects `on_request`, write tools are blocked at admission with structured `approval_required` feedback until an approval owner exists. The controlled-workspace profile is deliberately not an OS-level sandbox claim: it is the current bounded command adapter. If a future runtime adds approval prompts or a stronger OS sandbox, it must extend this owner path rather than letting shell requests select those control-plane fields. A configured stronger sandbox profile that is unavailable to the current executor must fail closed rather than silently run through host execution.

Model-visible tool schemas expose only semantic or user-supplied fields that the model must choose to perform the action. System-bound and audit-only fields such as event ids, operation ids, lease ids, task ids, `tool_call_event_id`, `provider_tool_call_id`, permission mode, authority policy, sandbox profile, approval policy, idempotency keys, timestamps, hashes, lineage, checkpoint refs, and audit refs are generated, bound, and validated by the kernel. If a model supplies those fields inside tool arguments, the tool request is rejected as repairable `tool_request_invalid` feedback and no effect executes; the kernel does not silently canonicalize model-supplied control-plane values into owner truth.

`shell_exec` is a generic terminal/process primitive, not an application API. In `default` mode it uses a deliberately tiny controlled workspace adapter that fail-closes on link aliases that weaken workspace containment; in `yolo` mode it invokes the host shell through the process runtime. Neither path may grow Feishu, email, calendar, document, or channel-specific command aliases inside the kernel.

The current local managed shell executor runs host shell processes and therefore requires the resolved host sandbox profile. Long `shell_exec` requests in `default` mode are blocked as controlled-workspace policy until a managed executor exists for the controlled adapter. This is a ToolGateway admission decision, not an HTTP or provider-adapter branch.

External skill packages are user-space assets, not kernel applications. The kernel may be configured with explicit skill roots and may scan `SKILL.md` front matter into a read-only skill index. Provider context receives only a bounded metadata index so the model can discover installed user-space capabilities without paying for full instructions every turn. Protected inspection surfaces such as `GET /capabilities` and `GET /turns/{id}/context` can show the same path-free index. Full skill bodies are not model-visible kernel state. A future use-time skill hydration path must be a generic resource/context contract with bounded handles, not a Feishu, email, calendar, document, or skill-package adapter inside the kernel. The instruction path is an internal read handle and is not exposed to the model. Skill metadata is treated as untrusted before indexing; authority-shaped, prompt-injection-shaped, hidden-control, duplicate-name, linked-path, or secret-shaped metadata is excluded rather than repaired into context.

Full skill-body retrieval is not part of the first default model-visible tool surface. If Genesis later needs long skill instructions, they must arrive through a generic resource/context contract or another explicitly reviewed owner path, not through package-specific retrieval tools. Skill prose remains user-space context and never grants kernel authority.

Skill catalog diagnostics are inspection evidence only. The kernel may report path-free exclusion reasons such as missing root, linked path, malformed metadata, unsafe metadata, or duplicate name so an operator can repair the installation, but those diagnostics do not expose the excluded path or body and do not silently repair metadata into model context.

Model-requested tool call batches are preflighted as a unit before any effect executes. If any call in the batch is unsupported or malformed, the entire batch fails closed and no call in that batch may create an operation or external effect. The kernel assigns each admitted tool slot the `tool.call` event id as `tool_call_event_id`, and that event id is the kernel identity for operation idempotency, audit, replay linkage, and `tool.result.for_event_id`. Provider-native `tool_call_id` stays the provider-visible echo id used by provider adapters for assistant tool calls and tool results; duplicate provider ids in one batch fail closed before `tool.call` events or effects. The kernel returns structured `tool_request_invalid` tool results so the model can repair the request, and the repair JSON content does not duplicate provider-native ids. Calls in the same rejected batch that were otherwise valid receive `tool_batch_not_executed` feedback instead of being executed.

Provider tool calls without native ids can still form kernel `tool.call` events and ledger `tool.result` evidence. In provider context replay, those calls simply have no provider echo id and no kernel event id; the kernel-owned `tool_call_event_id` stays in ledger/session/audit inspection. Only tool protocol states that cannot form a correlated kernel tool event are fatal provider protocol failures. The kernel does not add application-specific outbound APIs for email, Feishu, calendar, documents, or similar domains; installed skills and external CLIs remain user-space capabilities reachable through generic governed tools.

Tool result taxonomy follows a terminal-equivalent boundary:

- `tool_request_invalid`: the model's tool request failed kernel schema or admission checks and was not executed. The result is repair feedback for the model when protocol state allows it.
- `permission_denied`: the request was structurally valid but permission or policy denied execution. No command effect occurred. The model-visible result contains minimal repair feedback; the full `operation.blocked` evidence with policy reason remains in session/operation inspection.
- `operation.failed`: the command was accepted and executed, but the command process exited nonzero. The kernel returns `exit_code`, `stdout`, and `stderr` as observed command evidence and does not judge command semantics.
- `tool_infrastructure_failed`: the shell runtime, ledger, or tool runtime infrastructure failed. This is not represented as command stderr or a normal command exit. Model Gateway provider errors remain provider failures, not tool results.

The local append-only ledger preserves shell operation command/stdout/stderr as observed so restart replay and later audit can distinguish kernel truth from display policy. HTTP responses, session projections, raw event inspection, audit replay, and model-visible tool results consume redacted and bounded projections of that ledger evidence. Redaction is a projection policy; it must not mutate shell operation events before they are appended.

Long stdout and stderr are bounded with a head/tail policy. Operation evidence reports `stdout_truncated` or `stderr_truncated`, original byte counts, omitted byte counts, and `output_truncation=head_tail` when truncation occurs. The model-visible stdout or stderr text also includes a visible omission marker such as `[... N bytes omitted ...]` between the preserved head and tail content.

### WorkRegistry

Owns durable work state, cancellation, recovery, status projection, and execution evidence. It does not own application business data.

The first WorkRegistry transport is intentionally a record ledger, not a scheduler. `work.submit` creates a kernel-owned work record with `session_id`, user-visible `title`, and required `source_ref`. `work.cancel` records an explicit terminal cancellation decision with `cancel_authority`, `cancel_reason`, and `cancel_evidence_ref`. Work records are projected through their source session and can be read after restart.

WorkRegistry remains a durable record ledger, not the managed shell executor or a scheduler. Shells, external daemons, and future applications may submit work records as resumable coordination evidence, but application task semantics, Feishu task objects, calendar events, desktop notifications, queue workers, retries, leases, and scheduler policy remain outside the kernel until they prove generic kernel ownership.

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
- Role taxonomies, graph templates, planner strategies, and application orchestration policies.
- Product-specific workflows and domain owners until they prove they are kernel primitives.

## Reference Projects

Reasonix is a reference for Go single-binary distribution, config-driven tool/plugin loading, and one transport-agnostic controller behind multiple frontends.

Codex is a reference for tool approval, sandboxing, session/turn/event rigor, and separation between core protocol and shells.

Neither project is a blueprint to copy wholesale. Genesis should stay smaller and more generic than a coding agent.

For every non-trivial kernel boundary change, the issue or retirement evidence must state whether the change is aligned with Codex, aligned with Reasonix, intentionally different, or a known drift risk. The comparison is about control-plane ideas: model-visible surface, tool result taxonomy, permission/sandbox ownership, registry boundaries, event/ledger recovery, and shell/application separation. It is not a maturity checklist and must not justify copying application-specific behavior into the kernel.
