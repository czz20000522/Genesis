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

### KERNEL-IDEMPOTENCY-20260622 - P0 - Duplicate tool idempotency keys must not execute effects twice

- Status: in_progress.
- Type: architecture.
- Problem: `docs/minimal-closed-loop.md` requires duplicate idempotency keys not to execute effects twice, and `docs/kernel-contract.md` assigns idempotency to the Interface Kernel. The current `shell.exec` request has no idempotency key field or ledger lookup, so replaying the same mutating request executes the write again and appends a second operation.
- Expected behavior: `shell.exec` accepts an optional kernel-owned `idempotency_key`. For the same `session_id`, tool name, and key, the first admitted operation owns the effect. Later retries return the existing operation projection without executing the command again or appending new operation events. The key is a control-plane field, not model-visible task text, and survives restart through the event ledger.
- Verification required: a repeated default-mode write with the same idempotency key appends text only once, returns the same `operation_id`, and leaves only one projected operation after restart; blocked operations are also idempotent; unknown/invalid key shapes fail closed; `go test ./...`, build, live fake-provider smoke, and route version scan pass.
- Source: derived from Minimal Closed Loop negative path.
