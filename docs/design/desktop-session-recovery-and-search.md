# Design: Desktop Session Recovery And Search

- **Requirement:** `docs/requirements/desktop-session-recovery-and-search.md`.
- **Boundary:** the existing kernel `/sessions/search` projection and
  `/sessions/{session_id}/interrupt` command remain the single source of
  search and interruption truth; desktop resubmits or requests an interrupt,
  but does not repair a terminal turn.

## Reference Scan

Codex's app-server protocol defines both `thread/search` and `turn/interrupt`
as server operations (`codex-rs/app-server-protocol/src/protocol/v2/thread.rs`,
`.../v2/turn.rs`); its interrupt suite waits for the terminal interrupted
notification rather than changing local history first. Reasonix's
`desktop/app.go:Cancel` delegates to the controller's in-flight cancel handle,
while `internal/acp/service.go:sessionCancel` owns the active turn and its
eventual terminal result. Genesis aligns: search remains the existing server
projection, and the desktop requests its existing interrupt route then reloads
the resulting timeline. It intentionally differs from neither reference by
inventing a frontend terminal event or using transport cancellation as truth.

## Desktop Shape

Add a compact search input above the session rail. Non-empty search replaces
the grouped rail with kernel results. A failed-turn card exposes one `重试`
action only when the timeline has the settled user text needed to create a new
turn. The action uses the existing stream submit path and refreshes the same
timeline. While that stream is active, the composer replaces send with one
`停止生成` action. It requests the kernel session interrupt with the fixed
operator reason `user requested stop`; the stream and timeline remain the
source of the settled interruption message.

At startup the desktop first reads the kernel session list. It reopens the
latest active entry in its desktop catalogue only when that id is still in the
kernel projection. It creates a first Chat only after that successful list is
empty; list failure leaves the rail intact and asks the operator to retry
rather than minting a replacement session.

## Failure Semantics

Search errors preserve the existing rail and show the normal desktop error
surface. Retry refusal preserves the failed turn and reports the kernel error.
An interrupt conflict reloads the current timeline because the turn may have
settled concurrently. The desktop never guesses whether a provider failure is
retryable or whether a request has become terminal.
