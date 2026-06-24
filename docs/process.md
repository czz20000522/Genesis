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

User-space applications may have their own approved requirements and designs when
they are used to pressure-test the kernel or define adapter behavior. Application
requirements live under `docs/applications/` and follow the same flow, but they
must not be recorded as kernel capabilities. Application gaps belong in
`docs/operations/application-issues.md` unless they expose a missing generic
kernel primitive.

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

The scan must read implementation behavior, not just names. A reference is not
"aligned" merely because both projects have a `Session`, `Controller`, `Tool`,
`Job`, `Queue`, or `Provider` symbol. The scan should trace the actual path from
entrypoint to owner state transition and projection: which public request starts
the behavior, which owner creates or mutates state, which events or records are
written, which fields are model-visible, how errors and retries are represented,
and which tests prove the behavior. For example, do not assume that a user
pressing "new conversation" means the core creates a durable session record at
that moment; Codex and Reasonix must be checked for their actual session/thread
creation, lazy initialization, projection, and persistence semantics before
Genesis adopts or rejects a similar behavior.

The implementation plan must record:

- which Codex or Reasonix files, modules, docs, or tests were inspected;
- what behavior or boundary was learned;
- the concrete implementation path observed, including entrypoint, owner,
  state/event writes, projection, and relevant tests;
- whether Genesis aligns, intentionally differs, or rejects a drift risk;
- which unanswered differences remain as active issues or future slices.

Superficial reference scans are implementation drift. If the scan only lists
similarly named modules or broad product ideas without describing the control
flow and state semantics, stop before coding and deepen the reference scan.

If neither project has a comparable implementation, record that explicitly and explain which Genesis requirement/design owns the decision instead. Do not use "no reference found" as permission to skip requirement, design, failure semantics, permissions, recovery, or observability.

Any implementation slice that creates a durable store, table, index, queue, or
database-backed projection must first complete the persistence and store proposal
gate in `docs/requirements/kernel-foundation-capabilities.md`. Do not create a
separate database philosophy document for the same rules; storage admission,
transaction boundaries, rebuildability, retention, and JSONL/file-store
retirement belong with the kernel foundation persistence requirement and its
design.

## Issue Class Convergence Gate

Every non-trivial issue found during review, implementation, live testing, or
user acceptance must be reduced from one defect to one reusable guard. The
reviewer or implementer must record four things before treating the issue as
closed:

1. Name the problem class. Examples: protocol drift, owner confusion, projection
   treated as truth, model-owned control field, database-as-content-bucket,
   transport event persisted as fact, or lab store kept as product truth.
2. List the recurrence surfaces. Identify which shell, adapter, provider
   boundary, tool gateway, store, projection, UI, connector, or operator command
   can repeat the same class of mistake.
3. Convert the issue into enforcement. Choose the smallest durable guard:
   governing requirement/design text, proposal template, active issue shape,
   contract test, architecture test, negative fixture, lint/static scan, or
   acceptance smoke. A chat note alone is not enforcement.
4. Add it to periodic review. The next governance review must be able to ask
   whether the class reappeared, whether the guard still runs, and whether
   duplicated wording can be folded back into the governing requirement/design.

Do not scatter the same rule across several documents. If a new problem class
belongs to an existing requirement, update that requirement or its design and
link issues to it. Add a new document only when no existing owner document can
hold the rule without mixing ownership.

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
