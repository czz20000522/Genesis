# Genesis Kernel Agent Contract

Genesis Kernel is a small authority runtime for LLM execution. Keep the kernel generic. Do not add application-specific owners such as Feishu, email, calendar, calculator, document, OCR, medical, or insurance logic unless the work is reduced to a generic kernel primitive.

## Development Process Gate

Non-trivial kernel capability work must follow this order:

1. Requirement: define what the production capability must be, why it is needed, production semantics, roles, non-goals, phased delivery, acceptance criteria, and related issues.
2. Design: define owner, boundary, data flow, protocol, failure semantics, permission, recovery, and observability.
3. Implementation Plan: define Phase A/B/C delivery slices, red lines, tests, evidence, and what remains short of production.
4. Issue: track only the current gap between approved requirements/designs and implementation.
5. Implementation: change code only after the relevant requirement and design exist, except for obvious bugs or test gaps.

Core principle:

> Requirements must be production-grade. Implementation can be experimental or partial inside a phase, but every phase must state what still remains short of the production requirement.

Requirements should be few and stable. Implementation plans are phase-local and may be condensed after the phase closes. Issues record only current gaps and leave the active ledger when ready for acceptance or retired.

Issues must cite an approved requirement and design unless the issue is an obvious bug or test gap. If an issue uses that exception, say so explicitly in the issue.

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
