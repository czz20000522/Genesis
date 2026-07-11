# Design: Desktop Session Recovery And Search

- **Requirement:** `docs/requirements/desktop-session-recovery-and-search.md`.
- **Boundary:** the existing kernel `/sessions/search` projection remains the
  single search truth; desktop resubmits, but does not repair, a failed turn.

## Reference Scan

Codex's app-server protocol defines `thread/search` as a server operation
(`codex-rs/app-server-protocol/src/protocol/v2/thread.rs`) and its local store
returns search-only result metadata (`codex-rs/thread-store/src/local/search_threads.rs`).
Genesis aligns by using its existing server search endpoint rather than a Vue
index. Reasonix's desktop locale and controller expose session-search UI while
provider retry logic remains in its provider owner; Genesis intentionally keeps
operator retry explicit and makes it a fresh turn rather than replaying a
transport request.

## Desktop Shape

Add a compact search input above the session rail. Non-empty search replaces
the grouped rail with kernel results. A failed-turn card exposes one `重试`
action only when the timeline has the settled user text needed to create a new
turn. The action uses the existing stream submit path and refreshes the same
timeline.

## Failure Semantics

Search errors preserve the existing rail and show the normal desktop error
surface. Retry refusal preserves the failed turn and reports the kernel error.
The desktop never guesses whether a provider failure is retryable.
