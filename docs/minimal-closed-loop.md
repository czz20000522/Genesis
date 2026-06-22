# Minimal Closed Loop

The first implementation must prove the kernel loop before adding shells or application integrations.

## Initial Acceptance

1. A caller submits a turn through a transport-neutral kernel boundary.
2. The kernel records the turn and emits observable events.
3. The Model Gateway calls one configured OpenAI-compatible provider.
4. The model can either answer directly or request a generic tool.
5. The Tool System enforces permission policy before executing the tool.
6. Tool results return to the model loop as structured evidence.
7. The final answer and provider usage/evidence summary are emitted and replay after restart.
8. WorkRegistry can show the turn/work status after restart.
9. Accumulation can create a memory candidate, approve it, and recall it in a later turn.
10. Configured user-space skill roots can make installed external skill metadata visible to the model, and the model can read a selected skill's bounded instructions through a governed read-only tool without adding application-specific kernel code.
11. Authorized shells and daemons can inspect the current kernel capability surface without parsing prompts, local files, or application-specific code.

## Required Negative Paths

- Unknown transport fields are rejected before model context construction.
- Unauthorized tool effects are blocked before execution.
- Credential refs cannot expose raw secrets to the model or shell output.
- Prompt-injection text is isolated as user data and may be recorded as ingress risk metadata; authority-forgery attempts in transport schema or hidden control text fail closed at turn admission.
- A provider failure returns a structured degraded result, not a panic.
- Duplicate turn idempotency keys do not call the provider or execute model-requested tools twice.
- Duplicate tool idempotency keys do not execute effects twice.
- Unsupported or malformed model-requested tools produce repair feedback before any effect executes when a valid `tool_call_id` is available.
- Missing or malformed external skill metadata does not block turn submission.
- Unknown skill reads and path-shaped skill read arguments produce repair feedback before any effect executes.
- Capability inspection does not expose skill paths, skill bodies, provider credentials, or app-specific outbound APIs.

## Current Tool Loop Proof

The initial tool loop is deliberately narrow. OpenAI-compatible providers receive canonical `shell.exec` and `skill.read` descriptors. If the provider returns a `shell.exec` tool call, Genesis executes it through the existing Tool System, writes turn-scoped operation events to the ledger, and returns the redacted operation projection to the provider as tool evidence. If the provider returns a `skill.read` tool call, Genesis reads only the configured catalog entry by skill name and returns a bounded redacted instruction envelope as tool evidence. The provider then returns the final assistant text.

The kernel distinguishes invalid tool requests, policy blocks, command exits, and infrastructure failures. Invalid model tool arguments are returned as `tool_request_invalid` repair feedback when possible. Permission denials return `operation.blocked` without execution. Nonzero command exits return `operation.failed` with exit code and bounded stdout/stderr. Kernel or tool runtime failures return `tool_infrastructure_failed` and are not disguised as command stderr.

This proves the kernel loop without making any external application part of the kernel. Future Feishu, email, calendar, or document actions remain external skills and CLIs; Genesis only supplies the governed tool execution path.

## Current Skill Catalog Proof

The initial skill catalog is deliberately metadata-first. `genesisd` can scan configured skill roots for `SKILL.md` front matter and inject a concise list of available user-space skills into model context. The injected context names the skill and summarizes what it is for; filesystem paths remain internal. Full skill bodies are not injected into every turn. When the model needs the instructions, it must call `skill.read` with the catalog skill name; the kernel then returns bounded redacted user-space instructions as tool evidence. The kernel does not execute those skills by itself and does not add application-specific tool descriptors.

## Current Inspection Proof

`GET /capabilities` is a protected Readiness/Inspection surface. It returns provider, runtime auth, and ledger status, plus canonical kernel tool capability names and a path-free skill catalog projection. The route is for shells, desktop apps, and external daemons that need to know what the kernel can currently do; it is not a Feishu, email, desktop, or WebUI adapter.

## Not Required Initially

- CLI UX.
- WebUI or desktop UI.
- Feishu, email, calendar, document parsing, OCR, web search, or notifications.
- Multi-agent scheduling.
- Vector search optimization.
- Full migration of Python data.
