# Desktop Session Recovery And Search Implementation Plan

**Status:** implementation complete; real search and failed-turn retry
acceptance remains with the user under
`APP-DESKTOP-SESSION-RECOVERY-AND-SEARCH-20260711`.

## Phase A: Search Projection

Wire the existing `SearchSessions` desktop bridge into `SessionRail`; add
frontend tests proving the query uses the kernel projection and clearing it
restores normal groups.

## Phase B: Explicit Retry

Add a recoverable failed-turn projection carrying only the original user text
already present in the local timeline. Submit through the existing stream path
with a fresh idempotency key; test that original error evidence is retained and
approval/paused outcomes offer no retry.

## Phase C: Explicit Interrupt

Expose the existing kernel session-interrupt command through the shared desktop
API client and composer only while its current stream is active. The action sends
one fixed operator reason, does not cancel the transport locally, and reloads
the timeline after a successful request or a `no_active_turn` race. Add a
frontend API regression test; reuse the kernel interrupt suite for durable
event semantics.

## Verification

Run frontend tests/build, desktop tests/build, focused kernel search tests, and
the root Go suite. Manual acceptance searches a prior Project/Task/Chat session
and retries one deterministic provider error.
