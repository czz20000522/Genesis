# Implementation Plan: Reference Behavior Red Test Matrix

- **Issue:** `KERNEL-REFERENCE-BEHAVIOR-RED-TEST-MATRIX-20260625`
- **Requirement:** `docs/requirements/kernel-owner-structure-governance.md`
- **Design:** `docs/design/kernel-owner-structure-governance.md`
- **Owner:** Architecture Governance

## Reference Scan

Codex:

- Core behavior is protected by topic suites such as compaction, approvals,
  unified exec, provider context, and event replay tests.
- Internal tool/provider protocols use typed boundaries and fail before effect
  admission when a control-plane contract is violated.

Reasonix:

- Agent-loop behavior, parallel read-only dispatch, permission gating,
  compaction, and session/control behavior are covered by focused tests close to
  their owner packages.
- The useful reference is not its package layout, but the fact that reference
  behavior is executable as tests rather than left as architectural prose.

Genesis translation:

- Every non-trivial kernel implementation plan that records a reference scan
  must also record a small `Reference Behavior Red Tests` section.
- The section names the Genesis same-semantics red condition, not upstream test
  names or product-specific behavior.

## Reference Behavior Red Tests

- Add a governance test scanning `docs/implementation-plans/kernel-*.md` for a
  `Reference Behavior Red Tests` section whenever the plan contains a
  `Reference Scan`.
- Initial red condition: existing kernel implementation plans had reference
  scans but no explicit red-test translation section.
- Accepted intentional difference: the guard checks section presence only. It is
  not a prose-quality judge and does not assert upstream test names.

## Phase A

- Deliverable: process/template/doc updates plus a structural architecture test.
- Red lines:
  - Do not make the guard judge exact prose quality.
  - Do not require copying upstream test names or suites.
  - Do not block application plans that are not kernel implementation plans.
- Tests:
  - `TestArchitectureBoundaryKernelImplementationPlansNameReferenceBehaviorRedTests`.
- Evidence:
  - Focused architecture test.
  - `go test ./internal/kernel -count=1`.
  - `go test ./... -count=1`.
  - `go build ./...`.
  - `git diff --check`.
- Still short of production:
  - The guard proves the section exists. Human review still judges whether the
    translated test shape is strong enough for a specific issue.

## Retirement Criteria

`KERNEL-REFERENCE-BEHAVIOR-RED-TEST-MATRIX-20260625` can retire when the process,
implementation-plan template, owner-structure requirement/design, and current
kernel implementation plans all require or contain the red-test translation
section, and the structural guard passes.
