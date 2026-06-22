# Minimal Closed Loop

The first implementation must prove the kernel loop before adding shells or application integrations.

## Initial Acceptance

1. A caller submits a turn through a transport-neutral kernel boundary.
2. The kernel records the turn and emits observable events.
3. The Model Gateway calls one configured provider through the typed provider boundary.
4. The model can either answer directly or request a generic tool.
5. The Tool System enforces permission policy before executing the tool.
6. Tool results return to the model loop as structured evidence.
7. The final answer and provider usage/evidence summary are emitted and replay after restart.
8. WorkRegistry can show the turn/work status after restart.
9. Accumulation can create a memory candidate, approve it, and recall it in a later turn.
10. Configured user-space skill roots can make installed external skill metadata visible as a bounded model index and protected inspection projection without adding application-specific kernel code.
11. Authorized shells and daemons can inspect the current kernel capability surface without parsing prompts, local files, or application-specific code.

## Required Negative Paths

- Unknown transport fields are rejected before model context construction.
- Unauthorized tool effects are blocked before execution.
- Credential refs cannot expose raw secrets to the model or shell output.
- Prompt-injection text is isolated as user data and may be recorded as ingress risk metadata; authority-forgery attempts in transport schema or hidden control text fail closed at turn admission.
- A provider failure returns a structured degraded result, not a panic.
- Duplicate turn idempotency keys do not call the provider or execute model-requested tools twice.
- Duplicate tool idempotency keys do not execute effects twice.
- Missing or malformed external skill metadata does not block turn submission.
- Unsupported or malformed model-requested tool calls produce repair feedback before any effect executes when protocol state allows it.
- Capability inspection does not expose skill paths, skill bodies, provider credentials, or app-specific outbound APIs.

## Current Provider Boundary Proof

The preferred provider integration is `provider_command`. A command adapter receives a typed Genesis model request on stdin and returns either `kind=final` or `kind=tool_calls` on stdout. This keeps provider SDKs, vendor HTTP payloads, account flows, and provider credentials outside the kernel while preserving a typed turn loop for final answers, usage, and tool calls. The built-in OpenAI-compatible adapter remains available for local operation, but new provider work should target the command boundary.

## Current Tool Loop Proof

The initial tool loop is deliberately narrow. Providers receive a kernel-generated tool manifest from `ToolRegistry`; today that manifest contains canonical `shell_exec` with `side_effect_level=write` and `execution_kind=sandboxed_process`. If the provider returns a `shell_exec` tool call, Genesis routes it through `ToolGateway`, writes `tool.call`, any turn-scoped operation events, and `tool.result` to the ledger, then returns terminal-equivalent command evidence or minimal repair feedback to the provider. Unsupported unregistered tool requests return repair feedback without executing effects and still produce a `tool.result` linked to the original `tool.call`. The provider then returns the final assistant text.

The kernel distinguishes invalid tool requests, policy blocks, command exits, and infrastructure failures. Invalid model tool arguments are returned as `tool_request_invalid` repair feedback when possible. Permission denials return model-visible `permission_denied` feedback while session inspection keeps full `operation.blocked` evidence. Nonzero command exits return `operation.failed` with exit code and bounded stdout/stderr. Kernel or tool runtime failures return `tool_infrastructure_failed` and are not disguised as command stderr.

This proves the kernel loop without making any external application part of the kernel. Future Feishu, email, calendar, or document actions remain external skills and CLIs; Genesis only supplies the governed tool execution path.

## Current Skill Catalog Proof

The initial skill catalog is deliberately metadata-first. `genesisd` can scan configured skill roots for `SKILL.md` front matter and expose a concise path-free index through provider context and protected inspection surfaces. That index lets the model and shells discover installed user-space skills without making full skill bodies or unbounded descriptions part of every turn. Full skill bodies are not injected into every turn and are not available through a default model-visible skill-specific tool. A future full-instruction path must be a generic use-time resource/context contract rather than a Feishu, email, calendar, document, or skill-package adapter inside the kernel.

## Current Multi-Turn Proof

Multi-turn history is kernel-owned. The Model Gateway rebuilds same-session provider context from the ledger, not from shell-supplied history. When a context window is configured, provider-reported usage can trigger automatic compaction: the turn loop submits a typed kernel compaction command, and the kernel compaction runner summarizes older completed turns through `context.compaction.started` and `context.compaction.completed` checkpoints. Future turns receive that summary plus a recent verbatim tail made of complete conversation turns. Each provider response with usage writes `model.context.accounted` evidence, including exact provider usage/cache fields and provider-backed processed input tokens when the provider reports them. Token-budgeted tail selection consumes that accounting and never falls back to a kernel-local text estimate. A compaction failure records `context.compaction.failed`, preserves the completed user turn, and can be retried by a later eligible turn. The first implementation keeps the mechanism small and auditable; richer context selection or skill hydration must extend this owner path rather than moving summary logic into a WebUI, CLI, Feishu daemon, or provider adapter. User-facing shells should render only progress/completion/failure notices for compaction, never the internal summary itself.

## Current Inspection Proof

`GET /capabilities` is a protected Readiness/Inspection surface. It returns provider, runtime auth, and ledger status, plus canonical kernel tool capability names and a path-free skill catalog projection. The route is for shells, desktop apps, and external daemons that need to know what the kernel can currently do; it is not a Feishu, email, desktop, or WebUI adapter.

## Not Required Initially

- CLI UX.
- WebUI or desktop UI.
- Feishu, email, calendar, document parsing, OCR, web search, or notifications.
- Multi-agent scheduling.
- Vector search optimization.
- Full migration of Python data.
