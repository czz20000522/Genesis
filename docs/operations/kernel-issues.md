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

### KERNEL-TURN-EVENTS-20260622 - P1 - Turn events need a direct observation surface

- Status: new.
- Type: architecture.
- Problem: the kernel contract names `turn.stream` as a core syscall, but the current HTTP transport only returns events inline from `POST /turn` and exposes session-wide event summaries through `GET /sessions/{id}`. A shell, desktop app, daemon, or external reviewer that already has a `turn_id` has no direct kernel-owned way to read the ordered events for that turn after restart without fetching and filtering the whole session projection.
- Expected behavior: expose a minimal unversioned turn event observation surface that reads the append-only ledger and returns ordered events for one `turn_id`. It must stay transport-shaped, not UI-shaped, and must not duplicate session lifecycle ownership. Missing turn ids return 404; ledger failures retain existing fail-closed 503 behavior.
- Verification required: submit a turn, restart the kernel with the same ledger, call the turn event endpoint with the returned `turn_id`, and observe `turn.submitted` followed by `model.final`. Unknown turn ids return 404. The endpoint requires runtime authorization and introduces no versioned route names.
- Source: kernel contract `turn.stream`.
