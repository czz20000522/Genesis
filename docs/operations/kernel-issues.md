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

### KERNEL-PRESSURE-ACCEPTANCE-20260623 - P1 - Minimal kernel loop needs deterministic pressure verification

- Status: open.
- Type: architecture issue.
- Problem: the current kernel has a credible minimal closed loop, but acceptance evidence is still dominated by focused unit tests, targeted smokes, and manual live-provider runbooks. There is no deterministic pressure gate that repeatedly exercises multi-turn sessions, model tool loops, context compaction, ledger replay after restart, idempotent retries, permission blocks, provider failures, and bounded projection behavior under sustained use.
- Recommendation: add a kernel-generic pressure suite or runbook using fake providers and provider-command stubs. It should run many turns in one session, interleave successful tool calls, invalid repairable tool requests, blocked tools, nonzero command exits, compaction triggers, restart/replay checks, and failure injection. The suite must remain inside kernel primitives and must not introduce Feishu, email, calendar, WebUI, or other application owners.
- Reference alignment: Codex relies on core/session/tool-loop tests and recovery checks rather than treating shell or app surfaces as the source of truth. Reasonix keeps frontend/controller flows behind reproducible runtime checks. Genesis should add the same kind of deterministic core pressure gate without widening the kernel into product-specific adapters.
- Evidence: the current issue ledger has no active pressure issue, and existing acceptance evidence proves many individual boundaries but not sustained closed-loop behavior across repeated turns and restarts.
- Verification: a reproducible command, preferably `go test ./internal/kernel -run TestKernelPressure -count=1`, proves a long-running kernel session can complete repeated turns with tool loops, compaction, replay, retries, and injected failures. Broader verification must include `go test ./... -count=1`, `go build ./...`, `go test -race ./internal/kernel -count=1`, and `git diff --check`.
- Priority: P1.
