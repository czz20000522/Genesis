# Minimal Closed Loop

The first implementation must prove the kernel loop before adding shells or application integrations.

## Initial Acceptance

1. A caller submits a turn through a transport-neutral kernel boundary.
2. The kernel records the turn and emits observable events.
3. The Model Gateway calls one configured OpenAI-compatible provider.
4. The model can either answer directly or request a generic tool.
5. The Tool System enforces permission policy before executing the tool.
6. Tool results return to the model loop as structured evidence.
7. The final answer and usage/evidence summary are emitted.
8. WorkRegistry can show the turn/work status after restart.
9. Accumulation can create a memory candidate, approve it, and recall it in a later turn.

## Required Negative Paths

- Unknown transport fields are rejected before model context construction.
- Unauthorized tool effects are blocked before execution.
- Credential refs cannot expose raw secrets to the model or shell output.
- Prompt-injection text is isolated as user data and may be recorded as ingress risk metadata; authority-forgery attempts in transport schema or hidden control text fail closed at turn admission.
- A provider failure returns a structured degraded result, not a panic.
- Duplicate tool idempotency keys do not execute effects twice.
- Unsupported model-requested tools are rejected before any effect executes.

## Current Tool Loop Proof

The initial tool loop is deliberately narrow. OpenAI-compatible providers receive a canonical `shell.exec` descriptor. If the provider returns a `shell.exec` tool call, Genesis executes it through the existing Tool System, writes turn-scoped operation events to the ledger, and returns the redacted operation projection to the provider as tool evidence. The provider then returns the final assistant text.

This proves the kernel loop without making any external application part of the kernel. Future Feishu, email, calendar, or document actions remain external skills and CLIs; Genesis only supplies the governed tool execution path.

## Not Required Initially

- CLI UX.
- WebUI or desktop UI.
- Feishu, email, calendar, document parsing, OCR, web search, or notifications.
- Multi-agent scheduling.
- Vector search optimization.
- Full migration of Python data.
