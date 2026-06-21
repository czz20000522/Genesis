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

### recvndQ9cGNIqE - P1 - Stale running shell operations must not trap idempotent retries

- Status: new.
- Type: architecture.
- Problem: `shell.exec` idempotency currently returns the latest operation for a matching `session_id + tool + idempotency_key` even when that operation is still `running`. If `genesisd` exits after writing `operation.running` and before writing a terminal event, restart plus retry with the same idempotency key returns the stale running projection, does not re-execute, and does not write a terminal event. The caller cannot tell whether the effect happened, cannot recover the operation, and cannot progress the retry.
- Expected behavior: stale `running` operations loaded from the append-only ledger must enter an auditable recovery state instead of being treated as completed idempotent results. The minimal acceptable behavior is fail-closed: return a structured stale-operation blocker and record a terminal failure/recovery event without executing the effect again.
- Verification required: create a ledger containing only an `operation.running` event for `shell.exec`, restart the kernel, and retry `/tools/shell.exec` with the same `session_id + idempotency_key`. The response must not be `200 running`; the file effect must not execute; session projection must show a terminal or recovery-needed operation state; focused tests must cover restart replay and HTTP behavior.
- Source: Feishu Base record `recvndQ9cGNIqE`.
