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

### KERNEL-TURN-IDEMPOTENCY-20260622 - P0 - Turn submit retries must not create duplicate model/tool effects

- **Status:** in_progress
- **Owner:** Interface Kernel
- **Problem:** `turn.submit` is an effectful kernel entry. A shell, external daemon, or future desktop app can time out after submitting a turn and retry the same request. Without a turn-level idempotency key, the retry creates a second `turn.submitted`, calls the provider again, and can repeat model-requested tool effects even though the caller intended one logical turn.
- **Recommendation:** Add caller-provided `idempotency_key` to `turn.submit`, scoped to `session_id + turn.submit + idempotency_key`. The key is a control-plane field, not model-visible content. A completed or failed prior turn with the same key must return the existing turn response from the ledger without calling the provider or executing tools again. Invalid keys fail before ledger append. Supplying an idempotency key without an explicit `session_id` must fail because the retry scope would otherwise be unobservable to the caller.
- **Evidence:** Tool and WorkRegistry idempotency already exist, but Interface Kernel turn admission still lacks the same retry boundary. This is now the broadest remaining duplicate-effect risk because a turn can include both provider calls and governed tool execution.
- **Verification:** A duplicate `POST /turn` with the same `session_id + idempotency_key` after restart returns the original `turn_id` and final answer, the provider is not called again, and the session projection contains one turn. A duplicate turn that originally failed returns the same failed turn evidence without a second provider call. Invalid `idempotency_key` and missing `session_id` with an idempotency key return 400 and do not create a session. Focused tests, `go test ./...`, race test, builds, `git diff --check`, and no-version route scan must pass.
- **Priority:** P0
