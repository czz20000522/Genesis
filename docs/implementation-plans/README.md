# Implementation Plans

Implementation plans answer how approved requirements and designs will land in bounded phases.

A plan is not a requirement and not an issue ledger. It describes execution order, phase boundaries, red lines, tests, evidence, and known distance from the production requirement.

Kernel plans and user-space application plans can both live here. Application
plans must state how they avoid becoming kernel owners and must link application
issues rather than kernel issues unless a missing generic kernel primitive is
discovered.

Use this template:

```markdown
# Implementation Plan: <Capability Name>

## Requirement And Design

Link the approved requirement and design documents.

## Reference Scan

- Codex references inspected:
- Reasonix references inspected:
- Alignment:
- Intentional differences:
- Drift risks or follow-up issues:

## Reference Behavior Red Tests

- Reference behavior:
- Genesis equivalent:
- Test file or guard:
- Initial red condition:
- Accepted intentional difference:

## Phase A

- Deliverable:
- Red lines:
- Tests:
- Evidence:
- Still short of production:
- Closing gate:
  - Requirement/design/issue/BDD items checked:
  - Drift fixed before commit:
  - Drift recorded as active issue:

## Phase B

- Deliverable:
- Red lines:
- Tests:
- Evidence:
- Still short of production:

## Phase C

- Deliverable:
- Red lines:
- Tests:
- Evidence:
- Still short of production:

## Retirement Criteria

State what must be true before related issues can move to retirement evidence.
```

Every phase must run the closing gate before commit. If the implementation is intentionally narrower than the production requirement, the plan must name that remaining gap and either link an active issue or state why the requirement already allows the staged limitation.

## Closed Plan Handling

After a phase closes, delete the plan by default. Keep long-lived truth in the
requirement/design, and keep execution evidence in the issue retirement log or
campaign log.

Do not leave old checklists, active-issue wording, or phase-local shortfalls
that can be mistaken for current backlog. Current gaps belong in
`docs/operations/kernel-issues.md`; accepted or ready evidence belongs in
`docs/operations/kernel-retirement-log.md` or a campaign log.

Keep a closed plan only when it is still actively guiding an immediately
following phase. If it is kept, it should be short and must clearly say which
future work is outside the closed slice.

Future work mentioned in deleted plans is not active implementation scope unless
a new active issue or requirement/design package reopens it.
