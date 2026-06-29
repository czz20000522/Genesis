# Kernel Issue Ledger

This file is the repo-owned ledger for active Genesis Kernel issues. Feishu Base is the collaboration inbox; this file is the durable project source for issues that still need code, verification, or user acceptance.

Retired issues must not remain here. Move accepted retirements to `docs/operations/kernel-retirement-log.md` as compact evidence: one sentence with the retirement conclusion plus the fixing commit evidence.

## Ledger Rules

- Keep only `new`, `open`, `in_progress`, or otherwise active issues in this file.
- Do not record application-specific feature work as kernel work unless it changes a kernel primitive.
- Do not add versioned HTTP route names as current contracts. HTTP is transport; current kernel endpoints are unversioned.
- `ready_for_acceptance` issues move to the retirement log as retirement candidates and leave this active ledger.
- Feishu/Base links may point to collaboration artifacts, but this repo must contain enough evidence to understand the current status without opening Feishu.
- Every active `KERNEL-*` issue must include a `Reference alignment` field that compares the issue to Codex, Reasonix, or an explicitly rejected drift risk.
- Reference alignment uses local reference checkouts only. Do not cite Genesis GitHub repositories, remote issues, releases, or pull requests as authority for this local kernel line unless the user explicitly asks for external publishing context.
- Before a non-trivial implementation slice starts, the related implementation plan or issue must include a Codex/Reasonix reference scan. The scan should identify inspected references, learned control-plane semantics, intentional differences, and remaining drift risks.
- Reference scans must translate relevant Codex/Reasonix behavior tests into Genesis same-semantics red tests or explicitly reject them with a reason. A prose-only reference note is not enough for kernel primitives.
- Issues record the current gap between approved requirements/designs and the implementation. They must not carry raw requirements, design discussion, or the full production acceptance contract.
- Every active issue must cite an approved requirement and design unless it is an obvious bug or test gap. If an issue uses that exception, state the exception explicitly.
- Prefer `Gap`, `Next slice`, `Evidence`, and `Verification` over broad `Problem` or `Suggestion` text when adding new issues.
- Do not use issues as debug logs. Routine info, stream chunks, repeated status polling, and exploratory notes stay out of this ledger unless they identify a current implementation gap.
- When an issue removes a concept from the current kernel contract, long-term tests must assert the positive replacement behavior. Do not keep permanent tests whose only purpose is locking retired names, aliases, routes, or historical helper APIs out of the tree; use temporary scans or retirement-log evidence for cleanup windows, then fold the guard into the current owner contract.
- Development artifacts and historical local data are not compatibility obligations. Do not create or keep issues whose only purpose is migrating, cleaning, importing, or preserving old local generated state unless that state is part of the approved current kernel contract.
- Every implementation slice must finish with a drift check against the governing requirement, design, implementation plan, issue, and BDD feature. In-scope drift is fixed before commit. Out-of-scope drift is recorded here as an active issue with evidence and next slice before commit.
- Periodic governance review checks architecture, feature behavior, directory structure, and document lifetime together. Completed plans and stale documents should be deleted or condensed instead of spawning issues that only preserve old notes.

## Active Issues

### KERNEL-SESSION-STORE-PRODUCTION-CLOSURE-20260630 - P2 - File-backed SQLite session store production closure

- Status: open.
- Requirement: `docs/requirements/kernel-foundation-capabilities.md`.
- Design: `docs/design/kernel-foundation-capabilities.md`; current storage contract: `docs/kernel-contract.md`.
- Gap: Commit `ecf386b41` retired the JSONL physical ledger and introduced a file-backed session event store with a SQLite authority index. The current slice is sufficient for local development and restart replay, and the store now fails closed when another local writer lock exists. It is not the full production storage closure: orphan event frames are not garbage-collected, `GET /sessions` derives the list by loading events instead of using a SQLite aggregation path, there is no retention/backup/compaction policy for growing event files, and stale lock operator recovery is intentionally not automated yet.
- Next slice: Add the smallest production closure that has a concrete consumer: orphan-frame repair/GC and SQLite-backed session listing when session volume or desktop history performance requires it; stale lock recovery only after an operator lifecycle surface needs it. Do not add title, preview, archive, provider, model, or token summary columns until a shell or owner explicitly consumes them.
- Evidence: `internal/kernel/sqlite_ledger.go` writes event bodies to `session-events/*.events`, stores query/integrity metadata in SQLite, and acquires a single-writer lock before opening the store. `internal/kernel/sqlite_ledger_test.go` proves restart replay, file-body separation, fail-closed missing event files, and fail-closed external writer locks. `Kernel.ListSessions` currently derives `session_id` and `updated_at` from loaded events.
- Verification: Production closure must continue to prove single-writer admission, no duplicate effects after restart, missing index or missing frame failure modes, orphan cleanup that does not delete indexed truth, and session list behavior without full event-body replay.
- Reference alignment: Aligned with Codex's split between durable thread handles/backing store and projected thread details, and Reasonix's separation of settled transcript/checkpoint from live stream. Genesis intentionally differs by making SQLite an index over file-backed event truth instead of a full transcript table.
