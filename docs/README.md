# Genesis Documentation Map

This directory separates product definition, kernel contracts, requirements,
designs, implementation plans, and operational records. Do not copy full
concept definitions across documents. Keep one canonical source and link to it.

## Canonical Sources

- `project-brief.md`: product purpose, target user, roadmap, success standards,
  and current technical baseline.
- `kernel-contract.md`: kernel authority boundary, reference model, state
  semantics, provider/tool/session/memory contracts, and non-application rule.
- `minimal-closed-loop.md`: the smallest kernel loop that must work before
  adding more shells or applications.
- `process.md`: how requirements, designs, plans, issues, and retirement
  records should be written and retired.
- `requirements/`: stable production requirements for kernel primitives.
- `design/`: owner, boundary, data flow, protocol, failure, permission,
  recovery, and observability designs for kernel primitives.
- `implementation-plans/`: phase-local delivery plans. These should shrink or
  retire after their phase closes.
- `applications/`: user-space connector, workflow, code intelligence, and
  capability package requirements/designs.
- `operations/`: active issues, compact retirement evidence, runbooks, and
  collaboration/task-package records.

## Current Direction Packs

- `requirements/kernel-parent-worker-runtime.md` and
  `design/kernel-parent-worker-runtime.md`: Chinese parent-led worker runtime
  package for provider/model profile/role binding, leaf workers, task graph
  scheduling, and future memory/context integration.

## Duplication Rule

If a concept changes in more than one file, one of the files is probably
carrying too much authority. Keep detailed definitions in the canonical source;
other documents should summarize and link.

Examples:

- Product goal and roadmap live in `project-brief.md`.
- Kernel scope and authority live in `kernel-contract.md`.
- How to run a development build may be summarized in `README.md`.
- Current issue status lives in `operations/`, not in requirements or designs.
