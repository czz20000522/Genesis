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

### KERNEL-USAGE-SUMMARY-20260622 - P1 - Final answer must project provider usage summary

- Status: in_progress.
- Type: kernel inspection / Model Gateway projection.
- Problem: `docs/minimal-closed-loop.md` requires the final answer and usage/evidence summary to be emitted, but current `FinalMessage` only contains text and model. OpenAI-compatible provider usage data is ignored, so shells and reviewers cannot inspect token usage through `/turn`, turn events, or session projection after restart.
- Suggestion: Normalize provider usage into a small kernel-owned usage summary and persist it with the final model event. The first contract should use generic `input_tokens`, `output_tokens`, and `total_tokens` names rather than provider-native field names.
- Evidence: `internal/kernel/openai_compatible.go` decodes model and choices but not `usage`; `internal/kernel/types.go` has no usage field on `ModelResponse` or `FinalMessage`.
- Verification: A fake OpenAI-compatible server returns usage; `POST /turn` final response includes normalized usage; `GET /sessions/{id}` after restart projects the same usage; existing fake provider behavior remains unchanged.
