# Kernel Issue Ledger

This file is the repo-owned ledger for active Genesis Kernel issues. Feishu Base is the collaboration inbox; this file is the durable project source for issues that still need code, verification, or user acceptance.

Retired issues must not remain here. Move accepted retirements to `docs/operations/kernel-retirement-log.md` with the fixing commits, verification evidence, residual risks, and retirement reason.

## Ledger Rules

- Keep only `new`, `open`, `in_progress`, or otherwise active issues in this file.
- Do not record application-specific feature work as kernel work unless it changes a kernel primitive.
- Do not add versioned HTTP route names as current contracts. HTTP is transport; current kernel endpoints are unversioned.
- `ready_for_acceptance` issues move to the retirement log as retirement candidates and leave this active ledger.
- Feishu/Base links may point to collaboration artifacts, but this repo must contain enough evidence to understand the current status without opening Feishu.
- Every active `KERNEL-*` issue must include a `Reference alignment` field that compares the issue to Codex, Reasonix, or an explicitly rejected drift risk.

## Active Issues

### KERNEL-MALFORMED-TOOL-ARGS-REPAIR-20260622 - P1 - Malformed provider-command arguments should become model repair feedback

- Status: reopened.
- Type: architecture.
- Problem: OpenAI-compatible tool calls now preserve malformed argument text until ToolGateway can return `tool_request_invalid`, but `provider_command` still rejects invalid `json.RawMessage` arguments in `provider_command.toModelResponse`. That upgrades a model-repairable tool argument error into a provider command failure and bypasses the normal `tool.call -> tool.result` evidence path.
- Suggestion: Let provider command responses carry malformed tool arguments as raw text and pass them through to ToolGateway. If a tool slot can be correlated by provider `tool_call_id` or kernel `tool_call_event_id`, invalid argument syntax should become a model-visible `tool_request_invalid` result without executing effects. The provider command contract should distinguish raw argument text from validated JSON instead of requiring valid JSON before the kernel sees the tool request.
- Evidence: Feishu Base record `recvngbwKudgIg`.
- Verification: A provider command fake adapter returns a tool call with malformed raw arguments; the turn does not fail as provider error; event stream writes `tool.call` and `tool.result`; `tool.result.status=tool_request_invalid` with `error.code=invalid_tool_arguments`; the next model/provider round receives repair feedback; no shell operation or external effect occurs; `go test ./... -count=1` and race tests pass.
- Reference alignment: Codex keeps provider protocol pairing separate from tool validation and returns recoverable function-call argument errors through tool output when the loop can continue. Reasonix keeps provider `tool_call_id` pairing while sanitizing tool pairing at the provider boundary, not by turning malformed tool arguments into provider infrastructure errors.

### KERNEL-MODEL-SYSTEM-FIELD-BOUNDARY-20260622 - P1 - Model schemas must expose semantic fields only

- Status: new.
- Type: architecture.
- Problem: Genesis needs one cross-cutting rule that separates model-supplied semantic fields from system-bound control fields. Current tests cover some unknown tool arguments, but do not uniformly guard `event_id`, `operation_id`, `lease_id`, `task_id`, `tool_call_event_id`, `provider_tool_call_id`, and similar kernel-owned identifiers from entering model-visible tool schemas or being accepted from model arguments.
- Suggestion: Document model-visible schema field categories: semantic/model-supplied, user-supplied, system-bound, and audit-only. Tool input schemas may expose only the semantic/user fields required by the model. If model arguments include unknown or forbidden system fields, ToolGateway returns repair feedback without execution, never silently canonicalizes or lets model-supplied values override system-bound identities.
- Evidence: Feishu Base record `recvnghlSA6O8O`.
- Verification: `shell_exec` arguments containing `permission_mode`, `event_id`, `operation_id`, `lease_id`, `task_id`, `tool_call_event_id`, `provider_tool_call_id`, or comparable system fields return `tool_request_invalid`/unknown or forbidden feedback and produce no external effect; event/session/operation projections still show system-generated fields; provider adapter ids do not pollute kernel event identity; architecture tests pass.
- Reference alignment: Codex keeps tool input schemas focused on model-action payloads while host identifiers, approvals, sandbox state, and event ids stay host-owned. Reasonix provider/tool abstractions keep provider call ids for pairing but do not ask models to generate host lifecycle ids.

### KERNEL-PROVIDER-GATEWAY-TRANSLATOR-20260622 - P1 - Provider wire compatibility belongs behind gateway translators

- Status: new.
- Type: architecture.
- Problem: The kernel has a preferred `provider_command` boundary, but the built-in OpenAI-compatible adapter still lives in the kernel package and directly constructs `/chat/completions` payloads. If DeepSeek/OpenAI Chat/OpenAI Responses/OpenRouter differences are added there, provider compatibility logic will invade the kernel instead of staying behind a provider gateway/translator boundary.
- Suggestion: Keep ModelGateway/Kernel dependent only on canonical `ModelRequest`, `ModelResponse`, and typed provider events. Put DeepSeek, OpenAI Chat Completions, OpenAI Responses, OpenRouter, and similar wire-format differences behind provider command processes or explicit adapter/translator boundaries. Adapters own canonical request to upstream request translation and upstream final/tool/error conversion back to canonical events.
- Evidence: Feishu Base record `recvnghm0cSTqL`.
- Verification: Adding a DeepSeek/OpenAI-compatible route does not add vendor branches to the kernel turn loop; provider gateway adapter tests prove canonical request to chat upstream to canonical final/tool-call/tool-result replay; architecture tests restrict `/chat/completions`, DeepSeek, Responses, and comparable wire terms to adapter/translator boundaries; a provider command or adapter smoke covers final text and one `shell_exec` tool loop; `go test ./... -count=1` passes.
- Reference alignment: Codex keeps provider wire protocol handling behind API/client/protocol modules while the core tool loop consumes typed items. Reasonix registers provider implementations behind a provider abstraction and keeps OpenAI/Anthropic wire terms inside provider packages. Genesis should follow that separation rather than adding provider compatibility branches to the kernel loop.
