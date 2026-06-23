# Genesis Kernel Development Process

This process keeps requirements, design, implementation plans, and issues from collapsing into one document. Genesis is moving quickly, but temporary implementation must not become accidental architecture.

Core principle:

> Requirements must be production-grade. Implementation can be experimental or partial inside a phase, but every phase must state what still remains short of the production requirement.

## Document Roles

Requirement documents answer:

- what the kernel capability must be;
- why it is needed;
- production semantics;
- users and roles;
- core concepts, states, events, permissions, and failure behavior;
- non-goals;
- phased delivery intent;
- acceptance criteria;
- relationship to existing issues.

Design documents answer:

- boundary and owner;
- data flow;
- protocol and schema;
- failure semantics;
- permission and authority;
- recovery;
- observability and projections;
- rejected alternatives.

Implementation plans answer:

- how Phase A/B/C will land;
- each phase's deliverable;
- red lines;
- tests;
- evidence;
- what remains short of the production requirement.

Issues answer only:

- which current implementation behavior differs from an approved requirement or design;
- the next implementation slice;
- current evidence;
- focused verification needed for retirement.

## Required Flow

For every non-trivial capability:

1. Write or update the requirement first.
2. Write or update the design once the requirement is accepted.
3. Write or update the implementation plan before code changes.
4. Create or update issues only as gaps against the approved requirement or design.
5. Implement the smallest phase that can produce evidence.
6. Verify the phase and record the evidence.
7. Move accepted issues out of the active issue ledger and into retirement evidence.

Obvious bugs and test gaps may skip a new requirement or design document when the current approved requirement/design already covers the expected behavior. The issue must state that it is using the bug/test-gap exception.

## Issue Maintenance Rule

Every active issue must cite an approved requirement and design unless it is an obvious bug or test gap. Issues must not carry raw requirements, design discussion, or the full acceptance contract. They should use the smallest useful shape:

- Requirement:
- Design:
- Gap:
- Next slice:
- Evidence:
- Verification:
- Reference alignment:

If no approved requirement or design exists, the next step is to write that document instead of implementing code.

## Boundary Guard

Requirements and designs must stay inside the kernel boundary. Feishu, email, calendar, calculator, document, OCR, medical, insurance, and similar domains remain user-space unless the need is reduced to a generic primitive such as `turn.submit`, `tool.invoke`, `resource.read`, `resource.write`, `credential.resolve`, `work.submit`, `work.cancel`, `memory.review`, or `audit.replay`.

Temporary implementation can be limited, fake, or local to a phase. The requirement cannot be weakened to match that temporary shape. Each phase records the delta between current proof and production target.
