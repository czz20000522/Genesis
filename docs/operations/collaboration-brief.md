# Genesis Collaboration Brief

This file is the repo-local briefing for Codex, scheduled reviewers, and
production agents. Read it before using chat history or GitHub issue context.
Git commits and repo docs are the source of truth; GitHub/Feishu are
coordination surfaces only.

## Current Workflow

Day work is for direction:

- decide owner boundaries, names, and rejected paths;
- write the smallest durable requirement/design update;
- build a correct skeleton when the long-term shape matters;
- prepare bounded task packages with reference files and red tests.

Night work is for production agents:

- fill already-decided implementation gaps;
- stay inside the named owner and files;
- add behavior tests and drift checks;
- stop rather than inventing a new owner, route, adapter, or tool.

Scheduled review is for pressure:

- read this file, active issue ledgers, latest commits, and relevant designs;
- review feature behavior, architecture, directory/owner shape, and docs drift;
- add issues for real gaps instead of editing production code.

## Skeleton Rule

Use a walking skeleton only when it prevents later boundary drift. A skeleton is
allowed to have no production implementation, but it must be truthful:

- stable names first, provider/application-specific names only at adapter edges;
- `not_ready`, `not_implemented`, or fail-closed behavior instead of fake
  success;
- contract tests for ownership, visibility, and forbidden drift;
- no duplicate temporary owner that will need a migration later.

Example:

```text
Kernel -> Model Gateway protocol -> provider adapter runtime -> DeepSeek/SCNet adapter -> vendor API
```

Do not start with `DeepSeekGateway` in kernel and plan to rename later.

## Branch Policy

Target steady state:

- `master`: Genesis Kernel reconstruction line.
- `python-latest`: latest Python productization line kept for reference.

Prefer local commits as authority. Use cloud issues for coordination, not as the
only record, because network access is unreliable.

