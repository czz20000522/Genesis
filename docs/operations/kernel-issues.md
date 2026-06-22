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

### KERNEL-SKILL-METADATA-SECURITY-20260622 - P1 - Skill catalog metadata must not inject authority-shaped context

- Status: in_progress.
- Problem: the kernel now injects configured skill metadata into model context, but `SKILL.md` front matter is a user-space asset. A malicious or malformed `description` can contain role markers, prompt-injection directives, tool protocol fragments, or hidden-control text that would be presented as kernel-generated context rather than ordinary user data.
- Suggestion: treat skill catalog metadata as untrusted input before injection. Accept concise metadata-only descriptions, but exclude any skill whose `name` or `description` contains hidden control markers, role/protocol authority markers, prompt-injection directives, or secret-shaped content. Missing or rejected skills must not block turn submission.
- Evidence: `loadSkillCatalog` currently checks required fields and invisible control markers, but does not reject prompt-injection or authority-forgery text in skill metadata before `skillCatalogContext` prepends it to model input.
- Verification: a skill with safe metadata is injected; skills with `Ignore previous instructions`, `system:`, `tool_call_id`, invisible control characters, or secret-shaped metadata are excluded; turn submission still succeeds; no full skill body is injected.
- Priority: P1.
