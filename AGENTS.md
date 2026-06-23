# Genesis Kernel Agent Contract

Genesis Kernel is a small authority runtime for LLM execution. Keep the kernel generic. Do not add application-specific owners such as Feishu, email, calendar, calculator, document, OCR, medical, or insurance logic unless the work is reduced to a generic kernel primitive.

## Development Process Gate

Non-trivial kernel capability work must follow this order:

1. Requirement: define what the production capability must be, why it is needed, production semantics, roles, non-goals, phased delivery, acceptance criteria, and related issues.
2. Design: define owner, boundary, data flow, protocol, failure semantics, permission, recovery, and observability.
3. Reference Scan: before implementation planning or code, inspect Codex and Reasonix for comparable control-plane behavior, record what was learned, and state whether Genesis aligns, intentionally differs, or rejects a drift risk.
4. Implementation Plan: define Phase A/B/C delivery slices, red lines, tests, evidence, what remains short of production, and the reference scan summary.
5. Issue: track only the current gap between approved requirements/designs and implementation.
6. Implementation: change code only after the relevant requirement, design, and reference scan exist, except for obvious bugs or test gaps.
7. Closing Gate: before each commit, compare the implemented slice against the governing requirement, design, plan, issue, and BDD feature. Fix in-scope drift immediately; record out-of-scope drift as an active issue before committing.

Core principle:

> Requirements must be production-grade. Implementation can be experimental or partial inside a phase, but every phase must state what still remains short of the production requirement.

Requirements should be few and stable. Implementation plans are phase-local and may be condensed after the phase closes. Issues record only current gaps and leave the active ledger when ready for acceptance or retired.

Issues must cite an approved requirement and design unless the issue is an obvious bug or test gap. If an issue uses that exception, say so explicitly in the issue.

Implementation plans and non-trivial issue updates must not jump straight from Genesis-local reasoning to code. Look for comparable behavior in `D:\software\JetBrains\python_workspace\codex-main` and `D:\software\JetBrains\python_workspace\reasonix` first. The goal is not to copy them, but to catch missing state, failure, permission, recovery, and projection semantics before coding.

Each implementation slice must end with a requirement-by-requirement drift check. Passing tests is not enough if docs, issues, or retirement evidence still describe a temporary shortcut as the current contract.

Genesis kernel authority is local. Do not search GitHub, remotes, pull requests, releases, or online repositories for Genesis project truth unless the user explicitly asks for external publishing history. This project is developed locally; any public or remote Genesis repository is stale or unrelated to the active kernel contract.

Genesis has no production users, deployed uptime obligation, architecture migration debt, or historical data cleanup debt. Development artifacts, local ledgers, generated JSONL, fixtures, and old experiments do not justify compatibility shims, migration readers, fallback loaders, old API aliases, or data cleanup paths. When old development state conflicts with the current contract, delete or regenerate it instead of preserving it.

Architecture, feature, directory, and document reviews are one governance activity. They do not need to run after every commit, but periodic review must check all four together and delete or condense obsolete documents instead of letting requirements, plans, and acceptance records grow without bound.

## Boundary Rules

- The event ledger is kernel truth. Applications, shells, provider commands, and skills do not mint ledger facts.
- Runtime transport chunks are not kernel truth by default. Token deltas, stdout chunks, progress frames, and heartbeats stay in realtime transport or debug trace unless an owner reduces them to transcript, durable fact, audit, or failure evidence.
- Audit is not an info log. Persist only authority changes, risk decisions, credential use, control-plane writes, dangerous-operation decisions, security failures, and recovery-relevant failures.
- Provider context is assembled by the Model Gateway, not by shells or applications.
- Tool execution goes through ToolRegistry and ToolGateway.
- Model-visible schemas expose semantic fields only. Kernel ids, credentials, permission profiles, sandbox profiles, checkpoints, and audit refs are kernel-owned.
- Skill packages are user-space assets. Skill metadata may be indexed; skill bodies are not kernel APIs.
- HTTP route names and runtime policy names must not use numbered version identifiers as active contracts.

## Verification

Before claiming completion, run the smallest verification that proves the change. For document-only process changes, at least run `git diff --check` and the architecture boundary test when available. For runtime changes, run focused tests, then `go test ./... -count=1` and `go build ./...` unless the change is explicitly outside Go runtime behavior.
