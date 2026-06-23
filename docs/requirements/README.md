# Kernel Requirements

This directory is the production-requirements layer for Genesis Kernel.

Requirements answer only what the kernel needs, why it is needed, the production semantics, and what is explicitly not a goal. They do not carry implementation details, design debate, issue triage, or acceptance evidence for a specific patch.

The full development process is defined in `docs/process.md`.

Core principle:

> Requirements must be production-grade. Implementation can be experimental or partial inside a phase, but every phase must state what still remains short of the production requirement.

## Documentation Layers

Requirement:

- answers what we want, why we want it, production semantics, users and roles, core concepts, non-goals, staged delivery intent, acceptance criteria, and the relationship to current issues;
- is written before non-trivial implementation work;
- may be stricter than the current code;
- must stay inside the kernel boundary;
- should be few and stable. Extend an existing requirement when the need belongs to an existing kernel primitive.

Design:

- answers boundary, owner, data flow, protocol, failure semantics, permission model, recovery model, and observability;
- belongs under `docs/design/`;
- explains how a requirement should be implemented without turning into an issue log.

Implementation Plan:

- answers how Phase A/B/C will land;
- states each phase's deliverable, red lines, tests, evidence, and known distance from the production requirement;
- belongs under `docs/implementation-plans/`.

Issue:

- records the current gap between approved requirements/designs and the implementation;
- belongs in `docs/operations/kernel-issues.md`;
- must not carry raw requirements, design discussion, or the full acceptance contract.

BDD Feature:

- records reviewable behavior examples that can become executable tests;
- belongs under `features/`;
- should drive public kernel commands and projections, not private helpers or UI copy.

Retirement Record:

- stores accepted or superseded issue evidence;
- belongs in `docs/operations/kernel-retirement-log.md`;
- must not keep retired concepts alive as active requirements;
- cites the governing requirement and design instead of copying their full production semantics.

## Size Control Rules

Requirements do not record patch evidence, active checklists, live issue triage, or debug findings. Those belong in implementation plans, issue records, retirement evidence, or short-lived debug traces.

Implementation plans can be detailed while work is active. After a phase closes, reduce the plan to the delivered slice, the remaining gap, and the commands needed to reproduce the evidence.

Issues record only the current gap. If an issue starts carrying background, design alternatives, or a complete acceptance contract, move that content back to the requirement or design and shrink the issue.

Periodic governance review must include document lifetime. Architecture, feature, directory, and document review happen together: stale plans are deleted or condensed, old issue narratives leave the active ledger, and requirements remain few and stable.

## Requirement Template

Use this template for every non-trivial capability:

```markdown
# Requirement: <Capability Name>

- **Status:** draft | approved | superseded
- **Owner:** <kernel owner>
- **Scope:** <short scope>

## Background

Why this capability is needed and what production problem it solves.

## Production Target

The final state the kernel must satisfy. This section does not shrink to match
the current implementation phase.

## Users And Roles

What ordinary users, operators/admins, reviewers, the LLM, the kernel, and
user-space applications can see or do.

## Core Semantics

Stable concepts, states, events, permissions, failure behavior, recovery
behavior, and visibility rules.

## Non-Goals

What is explicitly out of scope, especially application-specific behavior.

## Phased Delivery

Phase A/B/C delivery slices. Each phase must say what it proves and what still
falls short of the production target.

## Acceptance Criteria

Positive cases, negative cases, fail-closed behavior, recovery, audit,
UI/log/projection visibility, and test evidence.

## Relationship To Existing Issues

Which active issues are governed by this requirement, and which issues are only
implementation gaps.
```

## Boundary Rules

Requirements must not create application-specific kernel ownership. Feishu, email, calendar, calculator, document, OCR, medical, insurance, and similar domains remain user-space unless a requirement reduces the need to a generic primitive such as `turn.submit`, `tool.invoke`, `resource.read`, `resource.write`, `credential.resolve`, `work.submit`, `work.cancel`, `memory.review`, or `audit.replay`.

Requirements must not introduce numbered kernel route prefixes, policy labels, or runtime identifiers. Contract evolution belongs in owner-owned schema changes, readiness evidence, and acceptance records.
