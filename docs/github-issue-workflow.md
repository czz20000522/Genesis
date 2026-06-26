# GitHub Issue Workflow

GitHub Issues is the active work queue. Repo docs remain the durable architecture and retirement record.

This repo is a coordination queue, not the implementation repository. Agents use issues here to coordinate work, then make code/doc changes in the relevant Genesis worktree.

## Labels

Create these labels first:

- `triage`
- `ready_for_agent`
- `in_progress`
- `ready_for_review`
- `changes_requested`
- `ready_to_merge`
- `blocked`
- `kernel`
- `application`
- `docs`
- `P0`
- `P1`
- `P2`

## Flow

```text
triage -> ready_for_agent -> in_progress -> ready_for_review
  -> ready_to_merge -> closed
  -> changes_requested -> in_progress
  -> blocked
```

## Roles

- User: decides product priority and accepts or rejects behavior.
- Reviewer agent: writes issues, checks diffs/tests/design drift, comments blockers, and closes accepted issues.
- Production agent: implements only assigned `ready_for_agent` issues and reports evidence.

## Production Agent Rules

- One issue is one bounded task.
- One task uses one branch or worktree.
- Read the issue body before editing code.
- Follow `Do`, `Do not`, `References`, `Verification`, and `Acceptance`.
- Move the issue to `in_progress` by changing labels or commenting `STARTED`.
- Do not invent new scope. If scope is insufficient, comment `BLOCKED:` with the missing decision.
- Do not close issues.
- When done, comment with:

```text
RESULT:
- commits:
- changed files:
- verification:
- residual risk:
```

- Then move the issue to `ready_for_review`.

## Reviewer Rules

- Review the issue, implementation diff, relevant docs, and tests.
- Check behavior, architecture boundary, directory/owner structure, and document drift.
- Use attacker/non-happy-path thinking for permissions, replay, concurrency, crash recovery, oversized output, fake readiness, and vendor protocol drift.
- If failed, comment:

```text
BLOCKER:
- problem:
- evidence:
- required fix:
- required verification:
```

- Move the issue to `changes_requested`.
- If accepted, comment `Reviewer acceptance` with verification evidence, move to `ready_to_merge` if merge is still pending, then close after the change is merged or otherwise recorded.

## Source Of Truth

- GitHub Issues: active queue, assignment, comments, and review state.
- Implementation repo docs: architecture contracts, requirement/design docs, active/retired issue ledgers.
- Retirement evidence belongs in the implementation repo, not only in a GitHub comment.
- If GitHub and repo docs disagree, stop and reconcile before assigning more work.
