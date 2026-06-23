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

## Document Lifetime

Requirements are few and stable. Prefer updating an existing requirement when a new need fits an existing kernel primitive. Create a new requirement only when the production semantics introduce a new long-lived kernel capability or owner boundary.

Design documents live as long as the owner boundary lives. They may record rejected alternatives, but they should not carry phase checklists, issue status, or patch evidence.

Implementation plans are short-lived. They can contain phase checklists and temporary red lines while work is active. When a phase closes, keep only the summary needed to explain the delivered slice and the remaining production gap; move verification evidence to the retirement log.

Issues are gap records. Completed issues leave the active issue ledger. Retirement entries keep fixing commits, verification, residual risk, and acceptance condition; they should cite the requirement and design instead of restating them.

## Required Flow

For every non-trivial capability:

1. Write or update the requirement first.
2. Write or update the design once the requirement is accepted.
3. Run a reference scan against Codex and Reasonix before implementation planning or code.
4. Write or update the implementation plan before code changes.
5. Create or update issues only as gaps against the approved requirement or design.
6. Implement the smallest phase that can produce evidence.
7. Verify the phase and record the evidence.
8. Move accepted issues out of the active issue ledger and into retirement evidence.

Obvious bugs and test gaps may skip a new requirement or design document when the current approved requirement/design already covers the expected behavior. The issue must state that it is using the bug/test-gap exception.

## Reference Scan Gate

Every non-trivial implementation must first look for comparable behavior in:

- `D:\software\JetBrains\python_workspace\codex-main`
- `D:\software\JetBrains\python_workspace\reasonix`

The scan is about control-plane semantics, not feature parity. Check for comparable treatment of model-visible surface, tool result taxonomy, permission and sandbox ownership, registry boundaries, event or ledger recovery, provider context projection, session control, and shell or application separation.

The implementation plan must record:

- which Codex or Reasonix files, modules, docs, or tests were inspected;
- what behavior or boundary was learned;
- whether Genesis aligns, intentionally differs, or rejects a drift risk;
- which unanswered differences remain as active issues or future slices.

If neither project has a comparable implementation, record that explicitly and explain which Genesis requirement/design owns the decision instead. Do not use "no reference found" as permission to skip requirement, design, failure semantics, permissions, recovery, or observability.

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

## Persistence And Audit Gate

Runtime can produce many events, but long-term storage stays sparse. A runtime event may enter the durable fact layer only when it satisfies at least one condition:

- it was visible to the user or model;
- it is required for replay, recovery, idempotency, or checkpointing;
- it changes kernel-owned state;
- it records a permission, credential, security, risk, or control-plane decision;
- it records failure, blocking, degradation, or abnormal termination;
- it is an input to provider context, compaction, memory recall, or observation delivery.

Everything else stays in realtime transport, debug trace, or aggregate metrics. Audit is not an info log. It records authority changes, risk decisions, control-plane writes, credential use, dangerous-operation decisions, and security failures.
