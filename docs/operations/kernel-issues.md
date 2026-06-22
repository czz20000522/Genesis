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

### KERNEL-CAPABILITIES-20260622 - P1 - Shells and daemons need a protected kernel capability projection

- Status: in_progress.
- Problem: `/ready` only reports provider, runtime auth, and ledger state. A shell, desktop app, or external daemon cannot ask the kernel which generic tools are model-visible, whether skill catalog loading produced usable skills, or why configured skill roots were excluded, without parsing prompts, local files, or implementation details. This weakens the Readiness/Inspection plane and makes product shells guess.
- Suggestion: Add a protected unversioned `GET /capabilities` inspection route owned by the kernel. It returns provider/runtime/ledger status, canonical tool descriptors summarized as capabilities, and a skill catalog projection with safe skill name/description plus path-free exclusion diagnostics. The route must not expose filesystem paths, secrets, provider credentials, app-specific adapters, or skill bodies.
- Evidence: `ReadyResponse` currently has provider/runtime_auth/ledger only; `skillCatalogContext` is model prompt text, not an operator inspection API; `loadSkillCatalog` silently drops invalid, linked, duplicate, or unsafe skills without a structured inspection surface.
- Verification: Authenticated `GET /capabilities` returns `shell.exec`, `skill.read`, skill count/items without `instruction_path`; unauthenticated request returns 401; duplicate/linked/unsafe skill entries are reported only as path-free exclusions; `/ready` remains unversioned and does not leak skill details.
- Priority: P1.
