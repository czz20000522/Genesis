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

### KERNEL-MEMORY-REJECT-20260622 - P1 - Memory review needs a reject path

- Status: in_progress.
- Type: Accumulation / memory review governance.
- Problem: The kernel contract defines `memory.review` as approve, reject, or supersede, but the current HTTP and owner API only expose candidate approval. A user can leave a bad candidate pending forever, but cannot explicitly reject it with review evidence. A rejected candidate also needs to stay out of recall after restart.
- Suggestion: Add a kernel-owned reject operation for memory candidates with required rejection authority, reason, and evidence ref. Rejection should be restart-safe, visible in candidate list/read/session projection, filtered by `status=rejected`, and must prevent later approval of the same candidate unless a future supersession flow is explicitly implemented.
- Evidence: `internal/kernel/memory.go` has only `MemoryCandidatePending` and `MemoryCandidateApproved`; `internal/kernel/http.go` only routes `/memory/candidates/{id}/approve`; retirement log residual risks still call out missing reject/supersede.
- Verification: A rejected candidate appears as `status=rejected` after restart, is excluded from pending and approved recall, rejects missing rejection evidence, and cannot later be approved into active memory.
