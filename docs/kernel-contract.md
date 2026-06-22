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

## Kernel Planes

### Interface Kernel

Owns request normalization, session identity, event emission, idempotency, and turn admission. It does not know which shell submitted the request.

Turn admission separates untrusted content from control-plane authority. User or external-application text can contain prompt-injection samples, role labels, tool protocol fragments, logs, or quoted hostile instructions; those strings remain user data and do not grant system, developer, tool, credential, or permission authority. The Interface Kernel may record high-confidence text risks as ingress metadata for inspection. It fails closed only for malformed transport schema, hidden control text, unsupported input item types, or real attempts to set kernel-owned control fields.

Idempotency keys are caller-provided control-plane fields, not model-visible task content. For effectful tool calls, the first admitted `session_id + tool + idempotency_key` owns the effect; retries return the existing operation projection from the ledger without executing the effect again.

### Model Gateway

Owns provider configuration, model calls, streaming, retries, provider error projection, and data-egress policy hooks. It does not own prompts as product copy.

Provider-native usage fields are normalized into kernel-owned final evidence as `input_tokens`, `output_tokens`, and `total_tokens` when the upstream response provides them. Usage is inspection metadata stored with the final model event; shells may display it, but they do not compute or own it.

The local binary resolves provider startup from Genesis-owned model gateway configuration by default. The canonical user config root is `~/.genesis/config`; `models.json` selects a role-bound gateway profile, the gateway route, the upstream endpoint, protocol, model id, timeout, and a `secret://...` credential ref. The kernel may expose operator flags to select a profile or config root, but it must not require Codex environment variables or Codex credentials for Genesis live operation.

Provider endpoint paths are upstream configuration, not Genesis route contracts. The kernel's own HTTP transport remains unversioned.

### Tool System

Owns tool descriptors, permission gates, shell/process execution, result envelopes, and tool-loop continuation. Tool descriptors describe generic effects; application-specific instructions live in skills.

The first model-visible tool descriptor is canonical `shell.exec`. Provider adapters may translate it to provider-native function/tool schemas, but they do not own permission, workspace, execution, idempotency, or evidence semantics. A model-requested tool call is admitted only after the Tool System applies the kernel-owned policy; the resulting operation projection is the structured evidence returned to the model loop.

Model-requested tool call batches are preflighted as a unit before any effect executes. If any call in the batch is unsupported or malformed, the entire batch fails closed and no earlier call in that batch may create an operation or external effect.

Unsupported provider tool calls are rejected as turn failures before any effect runs. The kernel does not add application-specific outbound APIs for email, Feishu, calendar, documents, or similar domains; installed skills and external CLIs remain user-space capabilities reachable through generic governed tools.

### WorkRegistry

Owns durable work state, cancellation, recovery, status projection, and execution evidence. It does not own application business data.

The first WorkRegistry transport is intentionally a record ledger, not a scheduler. `work.submit` creates a kernel-owned work record with `session_id`, user-visible `title`, and required `source_ref`. `work.cancel` records an explicit terminal cancellation decision with `cancel_authority`, `cancel_reason`, and `cancel_evidence_ref`. Work records are projected through their source session and can be read after restart.

WorkRegistry does not execute background jobs in the first kernel spike. Shells, external daemons, and future applications may submit work records as resumable coordination evidence, but application task semantics, Feishu task objects, calendar events, desktop notifications, queue workers, retries, leases, and scheduler policy remain outside the kernel until they prove generic kernel ownership.

### Accumulation

Owns memory candidates, approval state, safe recall, source refs, and supersession. It does not silently turn model output into truth.

Candidate review decisions are durable owner evidence. Approved candidates may enter recall under context policy; rejected candidates are explicit review outcomes and must remain excluded from recall. A rejected candidate cannot later be approved through the minimal review surface; a future supersession flow must create an explicit replacement decision instead of mutating rejected truth into approved truth.

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
