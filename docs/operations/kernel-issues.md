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

### KERNEL-SKILL-CATALOG-20260622 - P0 - Model context needs generic external skill discovery

- Status: in_progress.
- Problem: the kernel exposes the generic `shell.exec` tool, but the model context has no generic way to know which external skill packages and CLIs are installed. This recreates the Feishu CLI usability failure without requiring a Feishu-specific adapter: installed user-space skills remain invisible unless a shell manually teaches the model every time.
- Suggestion: add a read-only skill catalog primitive owned by the Context/Tool boundary. Kernel configuration may include explicit skill roots, and the kernel scans only `SKILL.md` metadata under those roots. The model context receives a concise catalog of skill names, descriptions, and instruction paths. The kernel must not read full skill bodies into every turn, must not execute external application logic, and must not special-case Feishu, WeChat, email, calendar, or any other app.
- Evidence: current `ModelRequest` only carries user input, approved memory context, tool descriptors, and tool rounds. `modelToolDescriptors` exposes `shell.exec`, but there is no `skill_roots`, skill catalog, or model-visible skill metadata surface.
- Verification: a configured skill root containing `lark-im/SKILL.md` and `mail/SKILL.md` injects both skill summaries before the user turn; a missing root and malformed skill metadata do not block turn submission; provider-visible content contains no full skill body and no application-specific hardcoded code path.
- Priority: P0.
