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

### KERNEL-MEMORY-RECALL-20260622 - P1 - Memory recall needs an explicit kernel observation surface

- Status: in_progress.
- Problem: The contract names `memory.recall`, but the current HTTP surface only exercises recall indirectly through `POST /turn`. Shells, daemons, and future applications cannot inspect which approved memories the kernel would inject for a context without running a model turn.
- Suggestion: Add a protected, unversioned `POST /memory/recall` transport for the conceptual `memory.recall` syscall. The request uses `input_items` with the same input validation and hidden-control rejection as `turn.submit`; the response returns only approved `MemoryRecall` items selected by the current Accumulation policy. The endpoint must be read-only and must not create, approve, reject, supersede, or mutate memory candidates.
- Evidence: `docs/kernel-contract.md` lists `memory.recall` as a kernel syscall, while `README.md` currently documents recall only as a side effect of `POST /turn`. This leaves recall inspection coupled to provider execution.
- Verification: A focused HTTP test first fails with 404 for `POST /memory/recall`, then passes after implementation by proving approved candidates are returned after restart, pending/rejected/superseded candidates are excluded, malformed input is rejected before recall, hidden control text is blocked, missing auth returns 401, and the route does not append ledger events.
- Priority: P1.
