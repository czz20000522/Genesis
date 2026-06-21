# Kernel Issue Ledger

This file is the repo-owned ledger for active Genesis Kernel issues. Feishu Base is the collaboration inbox; this file is the durable project source for issues that still need code, verification, or user acceptance.

Retired issues must not remain here. Move accepted retirements to `docs/operations/kernel-retirement-log.md` with the fixing commits, verification evidence, residual risks, and retirement reason.

## Ledger Rules

- Keep only `new`, `open`, `in_progress`, or otherwise active issues in this file.
- Do not record application-specific feature work as kernel work unless it changes a kernel primitive.
- Do not add versioned HTTP route names as current contracts. HTTP is transport; current kernel endpoints are unversioned.
- `ready_for_acceptance` issues move to the retirement log as retirement candidates and leave this active ledger.
- Feishu/Base links may point to collaboration artifacts, but this repo must contain enough evidence to understand the current status without opening Feishu.

## Active Issues

### KERNEL-TOOL-LOOP-20260622 - Turn loop cannot execute model-requested tools

- **Priority:** P0
- **Status:** open
- **Area:** Tool System / Model Gateway / Interface Kernel
- **Problem:** `docs/minimal-closed-loop.md` requires the model to either answer directly or request a generic tool, then receive the tool result as structured evidence before producing the final answer. The current kernel only exposes `shell.exec` as a direct HTTP route for callers; `Provider.Complete` can only return final text, so a turn cannot run a model-requested tool loop.
- **Suggestion:** Add a minimal kernel-owned tool loop for `shell.exec`: provider adapters normalize tool calls into canonical tool requests, the Tool System enforces the existing `ToolPolicy`, operation evidence is written to the ledger, and the result is sent back through the Model Gateway for the final answer. The kernel must keep permission mode and workspace root as startup-owned authority fields, not model-visible arguments.
- **Evidence:** Current `Provider` returns only `ModelResponse.Text`; `OpenAICompatibleProvider` ignores provider `tool_calls`; `Kernel.SubmitTurn` appends `turn.submitted`, calls the provider once, and appends `model.final`.
- **Verification:** An OpenAI-compatible test server returns a `shell.exec` tool call; `SubmitTurn` must execute the governed command, append turn-scoped operation events, call the provider again with structured tool evidence, and return the final assistant text. The same flow must replay through `GET /turns/{id}/events` after restart. Unsupported tool requests fail closed without executing effects.
