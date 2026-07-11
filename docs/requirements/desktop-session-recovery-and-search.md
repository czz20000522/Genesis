# Requirement: Desktop Session Recovery And Search

- **Status:** approved for Stage 2 implementation.
- **Owner:** kernel owns session search, active-turn interruption, and settled
  turn truth; desktop owns query entry, result rendering, explicit retry, and
  the user-visible stop request.

## Production Target

An operator can search durable sessions from the rail, open a result, retry a
terminally failed user turn without manually copying its text, and explicitly
stop one in-flight turn. A retry is a new turn with a new idempotency key; a
stop is a kernel-owned interruption and never rewrites or hides the turn.

## Semantics

1. Search queries the existing kernel session-search projection; the desktop
   does not build a second browser history index.
2. Results show only returned title/snippet metadata and remain selectable as
   normal sessions.
3. Retry is offered only for a settled terminal error with a recoverable user
   input text. It submits that exact text with a fresh desktop idempotency key.
4. The failed record remains visible as evidence. Approval denial and an
   interrupted/paused turn are not silently retried.
5. No retry runs automatically, and no retry reuses the prior idempotency key.
6. Stop is offered only while the desktop has an in-flight stream for the
   active session. It calls the existing kernel session-interrupt command with
   an operator reason and waits for the normal terminal projection.
7. A no-active-turn refusal is a race-safe refresh condition, not permission
   for the desktop to synthesize an interrupted message or retry the turn.

## Non-Goals

- No full-text index in Vue, bulk retry, retry-policy engine, local stream
  abort masquerading as a settled turn, or hidden provider fallback.
- No mutation of session titles or kernel timeline facts by the desktop.

## Acceptance

- Search finds a persisted session and opens its existing timeline.
- A recoverable failed turn can be explicitly retried once as a distinct turn;
  its original failed evidence stays readable.
- Empty search restores the normal Project / Task / Chat grouping.
- A streaming local-provider turn can be explicitly stopped; its durable
  interrupted projection remains readable after refresh or restart.
