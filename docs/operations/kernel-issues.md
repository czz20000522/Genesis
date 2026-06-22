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

### KERNEL-TOOL-CALL-EVENT-ID-20260622 - P1 - Tool call identity should be kernel event id

- Status: new.
- Type: architecture.
- Problem: `tool.call` / `tool.result` events now exist, but the provider-native `tool_call_id` is still promoted into the kernel slot identity used by tool preparation, operation idempotency, model-visible results, and replay. That makes a provider wire identifier look like a kernel fact identity.
- Suggestion: Generate the tool slot identity from the kernel-owned `tool.call` event id. Store provider-native tool call ids only as adapter correlation data needed to reply to that provider. `tool.result.tool_result.for_event_id` remains the kernel causal link.
- Evidence: Feishu Base record `recvngbsXq5Tti`.
- Verification: provider tool ids can be missing or unstable without becoming kernel identity; every tool result links to `tool.call.event_id`; session replay restores `tool.call -> tool.result`; provider-native ids do not appear as kernel identity fields in model-visible repair payloads; `go test ./... -count=1` passes.
- Reference alignment: Codex distinguishes protocol item ids from internal event/control flow, and tool routing stays typed. Reasonix event-style flows keep local event identity separate from transport correlation. Genesis should keep provider correlation as adapter data and use ledger event ids for kernel facts.

### KERNEL-MALFORMED-TOOL-ARGS-REPAIR-20260622 - P1 - Malformed tool arguments should become model-visible tool result repair feedback

- Status: new.
- Type: architecture.
- Problem: OpenAI-compatible parsing rejects provider tool calls with invalid JSON arguments inside the adapter, turning a model-repairable tool argument error into a provider failure before ToolGateway can produce `tool_request_invalid`.
- Suggestion: Preserve the raw tool call when a correlated tool slot exists, write `tool.call`, and let ToolGateway/validators produce `tool_request_invalid` as `tool.result`. Only provider protocol states that cannot form a correlated tool event should be fatal provider failures.
- Evidence: Feishu Base record `recvngbwKudgIg`.
- Verification: fake or OpenAI-compatible provider malformed tool arguments do not directly fail the turn as provider error; event stream contains `tool.call` then `tool.result` with `tool_request_invalid`; next model round sees repair feedback; no shell operation or external effect occurs; `go test ./... -count=1` passes.
- Reference alignment: Codex returns structured tool-call errors to the model when protocol state can continue, and terminal execution errors are not confused with provider failures. Reasonix typed tool dispatch similarly keeps validation feedback inside the tool path. Genesis should classify malformed tool args as tool request invalid, not provider infrastructure failure.

### KERNEL-MODEL-VISIBLE-TOOL-RESULT-MINIMAL-20260622 - P1 - Model-visible tool results should exclude permission and audit fields

- Status: new.
- Type: architecture.
- Problem: `modelOperationResult` copies permission and audit-adjacent fields such as `permission_mode`, `blocked_reason`, and `infrastructure_reason` from `OperationProjection` into the tool result content returned to the model. The model should see terminal-equivalent command evidence and minimal repair feedback, while authority, audit, trace, checkpoint, and policy details belong to event stream or inspection surfaces.
- Suggestion: Split model-visible tool result content from inspection/audit operation projection. The model-visible shell result keeps terminal-equivalent fields: status, executed/accepted state, exit code, stdout/stderr, truncation metadata, and minimal error codes needed for repair. Permission mode, operation id, audit refs, sandbox lease, event timestamps, and policy internals remain in ledger/session inspection only.
- Evidence: Feishu Base record `recvngbBifsEi0`.
- Verification: successful command results returned to the model exclude permission mode, operation id, audit refs, sandbox lease, and event timestamps; permission denial returns minimal model-visible feedback and full inspection event evidence; session projection still contains full permission/audit evidence; tests cover missing arguments, permission denied, nonzero exit, and infrastructure failure; `go test ./... -count=1` passes.
- Reference alignment: Codex models terminal output and structured tool errors separately from sandbox/approval/audit control state. Reasonix keeps policy/control metadata out of provider-facing tool content. Genesis should keep the LLM in the operator role and the kernel in the terminal, permission, audit, and recovery role.
