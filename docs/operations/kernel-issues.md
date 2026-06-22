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
