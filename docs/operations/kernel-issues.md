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

### recvndHA93jSZH - P1 - Genesis provider credential needs an executable setup path

- Status: new.
- Type: usability.
- Problem: `genesisd` can read `~/.genesis/config/models.json` and resolve `secret://...` credentials from the Genesis local secret store, but a new user has no executable path to create both files safely. The current implementation has `CryptUnprotectData` read/decrypt support, but no corresponding same-user setup flow that writes DPAPI-protected credential records and model gateway config. Requiring users to hand-write `protected_data_b64` is not acceptable.
- Expected behavior: provide an operator setup surface that creates or verifies a provider profile, gateway route, model id, timeout, and `secret://...` credential record without leaking the API key. This surface belongs to shell/operator setup, not to the kernel runtime or provider account business logic. It must keep the kernel route contract unversioned and avoid Feishu/Codex-specific assumptions.
- Verification required: in a clean config root and credential store root, the setup path writes `models.json` plus a local credential record; command output and repo/test artifacts do not contain the raw API key; `genesisd -config-root <root> -credential-store-root <store> -runtime-token tok` reports `/ready.status=ok`; deleting or corrupting the credential makes `/ready` fail closed with a provider credential reason; fake provider mode remains available for tests.
- Source: Feishu Base record `recvndHA93jSZH`.
