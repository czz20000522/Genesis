# Genesis Kernel Development Process

This process keeps requirements, design, implementation plans, and issues from collapsing into one document. Genesis is moving quickly, but temporary implementation must not become accidental architecture.

Genesis kernel truth is local. The active project contract lives in this checkout, its worktrees, and the approved local reference projects used for comparison. Do not search GitHub, remote repositories, pull requests, releases, or online package history for Genesis authority unless the user explicitly asks for external publishing context. Any public or remote Genesis repository should be treated as stale or unrelated to this local kernel line.

Genesis has no production users, deployed data contract, uptime obligation, architecture migration debt, or historical data cleanup debt. Development artifacts, generated ledgers, local JSONL files, fixtures, old experiments, and stale task records are not compatibility obligations. If they conflict with the approved current contract, delete or regenerate them rather than preserving shims, fallback readers, migration paths, old aliases, or cleanup flows.

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

Architecture, feature, directory, and document reviews are one periodic governance activity. They do not need to run for every implementation slice, but a scheduled or requested governance pass must check all four together:

- architecture still matches the approved requirement and design;
- feature behavior still matches the BDD and acceptance examples;
- directory and file structure still make owners visible;
- document sets still contain only active requirements, living designs, useful phase summaries, active issues, and retirement evidence.

When a document no longer serves one of those roles, delete it or condense it into the current positive contract. Do not keep stale implementation notes, temporary checklists, or obsolete architecture narratives as active documents.

## Required Flow

For every non-trivial capability:

1. Write or update the requirement first.
2. Write or update the design once the requirement is accepted.
3. Run a reference scan against Codex and Reasonix before implementation planning or code.
4. Write or update the implementation plan before code changes.
5. Create or update issues only as gaps against the approved requirement or design.
6. Implement the smallest phase that can produce evidence.
7. Run the implementation closing gate.
8. Verify the phase and record the evidence.
9. Move accepted issues out of the active issue ledger and into retirement evidence.

Obvious bugs and test gaps may skip a new requirement or design document when the current approved requirement/design already covers the expected behavior. The issue must state that it is using the bug/test-gap exception.

## Reference Scan Gate

Every non-trivial implementation must first look for comparable behavior in:

- `D:\software\JetBrains\python_workspace\codex-main`
- `D:\software\JetBrains\python_workspace\reasonix`

The scan is about control-plane semantics, not feature parity. Check for comparable treatment of model-visible surface, tool result taxonomy, permission and sandbox ownership, registry boundaries, event or ledger recovery, provider context projection, session control, and shell or application separation.

The reference scan is local. It compares Genesis against the local `codex-main` and `reasonix` checkouts. It must not look up a Genesis remote, GitHub repository, issue tracker, or release history as project authority.

The implementation plan must record:

- which Codex or Reasonix files, modules, docs, or tests were inspected;
- what behavior or boundary was learned;
- whether Genesis aligns, intentionally differs, or rejects a drift risk;
- which unanswered differences remain as active issues or future slices.

If neither project has a comparable implementation, record that explicitly and explain which Genesis requirement/design owns the decision instead. Do not use "no reference found" as permission to skip requirement, design, failure semantics, permissions, recovery, or observability.

## Implementation Closing Gate

Every implementation slice must close with a requirement-by-requirement drift check before commit. The fixed order is:

1. Re-open the governing requirement, design, implementation plan, active issue, and relevant BDD feature.
2. Check each production semantic, non-goal, red line, failure behavior, permission rule, recovery rule, projection rule, and phased "still short of production" claim against the actual code, tests, and docs touched by the slice.
3. If a drift is small and inside the slice scope, fix it before committing.
4. If a drift is real but outside the slice scope, add or update an active issue that cites the approved requirement/design, records the evidence, and states the next slice.
5. If an old issue, implementation-plan note, or retirement entry now describes an obsolete temporary behavior as active, rewrite it to the current positive contract or mark it superseded in the retirement log.
6. Only then run verification and commit.

This gate is mandatory even when the code compiles and tests pass. Tests prove selected behavior; the closing gate checks whether the implementation, documentation, and issue ledger still describe the same architecture.

The commit message should mention the drift check in `Tested:` or `Not-tested:` when it materially shaped the slice.

Periodic governance review uses the same drift principle at a wider scope. It is the right time to remove completed implementation plans, stale requirement fragments, obsolete directory narratives, or old feature examples that no longer express the current kernel contract.

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

Implementation drift found during the closing gate must either be fixed before commit or recorded here as an active issue. Do not leave known drift only in chat, implementation-plan notes, or a local grep result.

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
