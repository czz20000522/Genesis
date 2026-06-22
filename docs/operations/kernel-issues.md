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

### KERNEL-SKILL-READ-20260622 - P0 - Model loop needs governed skill instruction retrieval

- Status: in_progress.
- Problem: the skill catalog makes installed external skills discoverable, but it only exposes metadata and instruction paths. In default permission mode, `shell.exec` cannot read global skill files outside the workspace root, so the model can know a skill exists without a governed way to read its `SKILL.md` instructions. This breaks the intended "external apps are user-space skills + CLIs" path and pressures the kernel toward app-specific adapters.
- Suggestion: add a generic read-only `skill.read` model tool. The model supplies a catalog skill name, not an arbitrary filesystem path. The kernel reads only the matching catalog entry's `SKILL.md`, returns a bounded redacted envelope that labels the content as user-space instructions, and preserves all existing tool policy and batch preflight semantics. Missing skills or unsafe reads fail closed as tool-call rejection without executing other effects in the batch.
- Evidence: `modelToolDescriptors` currently exposes `shell.exec`; `prepareModelToolCall` rejects every tool except `shell.exec`; `skillCatalogContext` exposes instruction paths but no governed read surface.
- Verification: a provider can request `skill.read` for a configured safe skill, receive the bounded instruction content as tool evidence, then produce a final answer; a provider cannot read arbitrary paths or unknown skills; mixed batches containing invalid `skill.read` plus an allowed shell effect fail before any shell operation.
- Priority: P0.
