# Task Package Template

Use this template for production-agent work. Keep the package executable, not
aspirational.

## Title

`OWNER-SHORT-GOAL-YYYYMMDD`

## Goal

One sentence describing the exact gap to close.

## Reference Files

- Local reference project files to inspect first, such as Codex or Reasonix.
- Genesis requirement/design/issue files that govern this work.
- Current implementation files likely to change.

## In Scope

- Concrete owner path.
- Concrete behavior to implement.
- Required red tests / behavior tests.
- Required docs or retirement-log updates.

## Out Of Scope

- New owners.
- New provider/application-specific kernel paths.
- Compatibility shims for old development state.
- Anything not already decided in the governing requirement/design.

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

