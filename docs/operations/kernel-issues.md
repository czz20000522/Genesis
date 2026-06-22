# Kernel Issue Ledger

This file is the repo-owned ledger for active Genesis Kernel issues. Feishu Base is the collaboration inbox; this file is the durable project source for issues that still need code, verification, or user acceptance.

Retired issues must not remain here. Move accepted retirements to `docs/operations/kernel-retirement-log.md` with the fixing commits, verification evidence, residual risks, and retirement reason.

## Ledger Rules

- Keep only `new`, `open`, `in_progress`, or otherwise active issues in this file.
- Do not record application-specific feature work as kernel work unless it changes a kernel primitive.
- Do not add versioned HTTP route names as current contracts. HTTP is transport; current kernel endpoints are unversioned.
- `ready_for_acceptance` issues move to the retirement log as retirement candidates and leave this active ledger.
- Feishu/Base links may point to collaboration artifacts, but this repo must contain enough evidence to understand the current status without opening Feishu.
- Every active `KERNEL-*` issue must include a `Reference alignment` field that compares the issue to Codex, Reasonix, or an explicitly rejected drift risk.
- When an issue removes a concept from the current kernel contract, long-term tests must assert the positive replacement behavior. Do not keep permanent tests whose only purpose is locking retired names, aliases, routes, or historical helper APIs out of the tree; use temporary scans or retirement-log evidence for cleanup windows, then fold the guard into the current owner contract.

## Active Issues

### KERNEL-CONTEXT-COMPACTION-REFINE-20260622

- Status: open

- Priority: P1

- Title: Context compaction spike is correct enough for the closed loop but needs production-grade selection and retry policy

- Problem: The current context compaction path proves the owner boundary and runtime loop: provider usage can trigger compaction, the ledger records started/completed/failed events, future provider context receives a summary plus a recent tail, user timelines hide the internal summary, and failed compaction can retry on a later eligible turn. The implementation is still a deliberately small spike. It uses a simple completed-turn split, fixed recent turn count, provider-reported input usage from the final response, and no dedicated compaction backoff, tail token budget, compaction economics, or cache-stability analysis.

- Suggestion: Refine the owner implementation after the kernel closed loop stabilizes. Use provider-normalized usage plus model context metadata for threshold decisions, keep a token-budgeted recent tail without splitting tool call/result pairs, add bounded retry/backoff evidence for summarizer failures, and record enough internal metrics to explain cache hit/miss behavior without exposing summaries to user timelines.

- Evidence: `internal/kernel/context_compaction.go` now writes `context.compaction.started`, `context.compaction.completed`, and `context.compaction.failed` events and projects future context from the latest completed summary. `internal/kernel/projections.go` renders only fixed compaction notices in `UITimeline`. `internal/kernel/openai_compatible.go` normalizes provider cache hit/miss usage into kernel-owned usage fields.

- Verification: Current guard tests cover summary-plus-tail model context, hidden user timeline summary, failed compaction evidence, retry on a later eligible turn, and OpenAI-compatible cache usage normalization. Future retirement requires tests for token-budgeted tail selection, tool pair boundary preservation, compaction failure backoff, and cache-stability metrics.

- Reference alignment: Codex uses model/context-window driven compaction thresholds, durable compacted history replacement, and retry around the compaction model call. Reasonix keeps a cache-first growing prefix, soft/trigger/force ratios, a bounded recent tail, stuck-compaction guard, and compaction started/done UI events. Genesis intentionally keeps only the owner boundary and closed-loop proof for now, while recording this issue so the spike does not become the final policy.
