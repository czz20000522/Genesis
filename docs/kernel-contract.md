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

## Kernel Planes

### Interface Kernel

Owns request normalization, session identity, event emission, idempotency, and turn admission. It does not know which shell submitted the request.

Turn admission includes a generic ingress security scan before memory recall, ledger append, provider calls, or tool effects. High-confidence prompt-injection markers, authority-forgery markers, tool-call forgery markers, and invisible-control text fail closed with a structured blocker. The blocker may expose a stable category/reason code, but it must not expose matching thresholds, raw secret material, or shell-specific policy.

### Model Gateway

Owns provider configuration, model calls, streaming, retries, provider error projection, and data-egress policy hooks. It does not own prompts as product copy.

The local binary resolves provider startup from Genesis-owned model gateway configuration by default. The canonical user config root is `~/.genesis/config`; `models.json` selects a role-bound gateway profile, the gateway route, the upstream endpoint, protocol, model id, timeout, and a `secret://...` credential ref. The kernel may expose operator flags to select a profile or config root, but it must not require Codex environment variables or Codex credentials for Genesis live operation.

Provider endpoint paths are upstream configuration, not Genesis route contracts. The kernel's own HTTP transport remains unversioned.

### Tool System

Owns tool descriptors, permission gates, shell/process execution, result envelopes, and tool-loop continuation. Tool descriptors describe generic effects; application-specific instructions live in skills.

### WorkRegistry

Owns durable work state, cancellation, recovery, status projection, and execution evidence. It does not own application business data.

### Accumulation

Owns memory candidates, approval state, safe recall, source refs, and supersession. It does not silently turn model output into truth.

### Auth/Credential Plane

Owns runtime client authentication, credential refs, redaction, and secret resolution for authorized effects. Provider-specific account setup belongs to shells or external applications unless it becomes a generic credential primitive.

The first local credential primitive is the Genesis local secret store. On Windows, `secret://...` refs resolve to same-user DPAPI-protected JSON records under `~/.genesis/credentials`. The kernel can decrypt the selected provider key in memory for the Model Gateway, but it must never expose raw secrets in readiness, events, sessions, logs, docs, tests, or model-visible context. Missing, unreadable, or unsupported credentials fail closed as provider readiness blockers.

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
