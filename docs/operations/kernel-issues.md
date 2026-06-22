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

### KERNEL-WORK-REGISTRY-20260622 - P0 - Minimal WorkRegistry needs a durable submit and cancel loop

- Status: in_progress.
- Problem: `docs/kernel-contract.md` names `work.submit` and `work.cancel`, but the Go kernel currently persists turns, tool operations, and memory candidates only. There is no durable work record that a shell, external daemon, or future desktop application can create, inspect after restart, or cancel with audit evidence.
- Recommendation: implement the smallest WorkRegistry primitive as kernel-owned ledger evidence: submit a work record, read it by id, project it through its source session, and cancel it with authority/reason/evidence. Do not implement scheduling, background execution, Feishu tasks, WebUI state, domain workflows, or application-specific task semantics.
- Evidence: `rg` finds WorkRegistry only in `README.md` and `docs/kernel-contract.md`; no `work.*` event types, work projection, or HTTP route exists in `internal/kernel`.
- Verification: regression tests must first fail on missing `POST /work`, `GET /work/{id}`, `POST /work/{id}/cancel`, and session work projection; after implementation they must pass after restart and prove canceled work cannot be canceled into a second competing terminal decision without preserving the first cancel evidence.
- Priority: P0.
