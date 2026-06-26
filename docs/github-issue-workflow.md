# GitHub Issue Workflow

GitHub Issues is the active work queue. Repo docs remain the durable architecture and retirement record.

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

## Agent Rules

- One issue is one bounded task.
- One task uses one branch or worktree.
- Implementation agents report results in issue comments.
- Reviewer comments with `BLOCKER:` for required fixes.
- Close only after verification evidence is posted.
