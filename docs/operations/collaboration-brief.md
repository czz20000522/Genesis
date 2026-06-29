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

## Priority Policy

Priority is judged from user experience and the minimum production loop, not
from implementation convenience.

- P0: without it, a user cannot complete the basic Genesis loop. Examples:
  chat/turn submission, live provider readiness, provider context projection,
  ToolGateway execution, session/timeline reads, material intake/upload,
  context compaction safety, and a usable chat-first frontend shell.
- P1: without it, the system works but daily use is fragile or hard to inspect.
  Examples: session debug export, Feishu inbound stability, local skill store,
  detail projections, provider setup ergonomics, and first-run runbooks.
- P2: domain or productivity expansion after the core loop is usable. Examples:
  CodeGraph adapter, specialized scientific-operator flows, non-core
  connectors, and richer resource previews.
- P3: ecosystem or polish that should not block production use. Examples:
  skill marketplace, visual refinements that do not block interaction, and
  optional workflow authoring UI.

If two tasks have the same engineering cost, choose the one that removes a
user-visible blocker first. UI aesthetics are low priority unless they block
reading, input, trust, or recovery.

## Skeleton Rule

Use a walking skeleton only when it prevents later boundary drift. A skeleton is
allowed to have no production implementation, but it must be truthful:

- stable names first, provider/application-specific names only at adapter edges;
- `not_ready`, `not_implemented`, or fail-closed behavior instead of fake
  success;
- contract tests for ownership, visibility, and forbidden drift;
- no duplicate temporary owner that will need a migration later.

The skeleton owner is the day-work reviewer, not the production agent. A
production agent can fill an already-decided skeleton, but it must not invent a
new owner, route, adapter, tool, schema, or frontend state model to make one
task pass.

Skeleton granularity should be the smallest shape that proves flow and naming:

- stable package, route, type, function, or component names;
- control flow that returns `not_ready` or an explicit empty projection when
  production capability is absent;
- tests that prove the placeholder cannot be mistaken for production success;
- no fake provider, fake upload, fake delivery, or fake UI success in production
  mode.

When a needed edge has no prior decision or external reference, stop and ask for
direction. Do not choose a framework, state model, route shape, adapter pattern,
or persistence policy just to keep an agent moving.

Example:

```text
Kernel -> Model Gateway protocol -> provider adapter runtime -> DeepSeek/SCNet adapter -> vendor API
```

Do not start with `DeepSeekGateway` in kernel and plan to rename later.

## Task Queue Policy

Local docs are the rulebook and decision record. They are not the day-to-day
task queue.

Use GitHub Issues plus GitHub Projects as the cloud queue when network access is
available:

- one Issue per executable task package or review finding;
- one Project board/table for ordering, lane split, and night-agent intake;
- labels for stable filters:
  - `lane:backend`
  - `lane:frontend`
  - `priority:P0`, `priority:P1`, `priority:P2`, `priority:P3`
  - `stage:skeleton-needed`
  - `stage:skeleton-ready`
  - `stage:ready-for-agent`
  - `stage:blocked-decision`
  - `type:design`, `type:implementation`, `type:review`
- optional milestones for phase boundaries, not for low-level owner status.

Night agents only pick tasks marked `stage:ready-for-agent` with governing docs
or references linked in the issue. If GitHub is unavailable, keep local commits
as authority and sync the queue later; do not let cloud downtime block design
or implementation.

## Lane Split

Backend lane owns the runtime skeleton and production gaps behind stable
kernel/application boundaries:

- `turn.submit`, session, timeline, context inspection, and debug export;
- Model Gateway, provider adapter binding, readiness, and credential setup;
- ToolGateway, ToolRegistry, authority, sandbox, approval, jobs, and budgets;
- material intake, source snapshots, source tools, references, and resources;
- connector runtime, inbound source supervision, outbox, and delivery receipts.

Frontend lane owns user experience over backend projections, not kernel truth:

- the kernel reconstruction frontend uses the Python line's Vue/Vite direction
  until an explicit replacement decision is recorded;
- chat-first shell and input composer;
- upload/material attachment entry;
- live turn progress and settled processing group rendering;
- timeline detail drawer, debug export access, and readiness/error surfaces;
- provider setup affordance that calls backend control APIs instead of reading
  credential files directly.

Frontend must not assemble provider context, decide tool authority, write memory
truth, mint tool results, or reinterpret raw ledger events as chat state.

## Branch Policy

Target steady state:

- `master`: Genesis Kernel reconstruction line.
- `python-latest`: latest Python productization line kept for reference.

Prefer local commits as authority. Use cloud issues for coordination, not as the
only record, because network access is unreliable.
