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

### KERNEL-MEMORY-SUPERSEDE-20260622 - P1 - Memory review needs explicit supersession

- Status: in_progress.
- Problem: `docs/kernel-contract.md` defines `memory.review` as approve, reject, or supersede, and Accumulation owns supersession. The current Go kernel can approve and reject memory candidates only. Correcting an approved, rejected, or pending candidate would require either approving a new candidate while the old approved memory still recalls, or mutating/reapproving rejected truth, both of which violate the review model.
- Recommendation: implement the smallest supersession primitive as one kernel-owned ledger decision: `POST /memory/candidates/{id}/supersede` records the original candidate as `superseded` with authority/reason/evidence and atomically creates one replacement candidate in `pending` state with replacement text and source ref. Superseded candidates must be excluded from recall; replacement candidates must not recall until approved. Do not implement vector policy, memory editing UI, migration shims, or domain-specific memory workflows.
- Evidence: `rg supersede internal/kernel` finds no implementation; README lists approve/reject only before this issue; retirement evidence for memory reject records supersession as future work.
- Verification: regression tests must first fail on missing `/memory/candidates/{id}/supersede`; after implementation they must prove restart-safe superseded and replacement projections, exclusion from recall before replacement approval, recall after replacement approval, idempotent duplicate supersede preserving the first replacement, and rejection of missing evidence/source fields.
- Priority: P1.
