# Task Package Template

Use this template for production-agent work. Keep the package executable, not
aspirational.

## Title

`OWNER-SHORT-GOAL-YYYYMMDD`

## Queue Metadata

- Lane: `backend` or `frontend`
- Priority: `P0`, `P1`, `P2`, or `P3`
- Stage: `skeleton-needed`, `skeleton-ready`, `ready-for-agent`, or
  `blocked-decision`

## Goal

One sentence describing the exact gap to close.

## User Value

Explain which user-visible loop this unblocks. Priority must be based on user
experience, not implementation convenience.

## Reference Files

- Local reference project files to inspect first, such as Codex or Reasonix.
- Genesis requirement/design/issue files that govern this work.
- Current implementation files likely to change.

## In Scope

- Concrete owner path.
- Concrete behavior to implement.
- Required red tests / behavior tests.
- Required docs or retirement-log updates.
- Whether this is skeleton-only or production implementation.

## Out Of Scope

- New owners.
- New provider/application-specific kernel paths.
- Compatibility shims for old development state.
- Anything not already decided in the governing requirement/design.
- Choosing a framework, state model, route shape, persistence policy, or adapter
  pattern not named by the task package.

## Required Checks

```powershell
git diff --check
go test ./... -count=1
go build ./...
```

Replace or narrow the test commands only when the package says why.

## Completion Report

Return:

- commits;
- tests run;
- issue retired or still open;
- known residual risk;
- any drift found outside scope.
