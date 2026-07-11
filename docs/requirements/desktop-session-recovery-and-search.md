# Requirement: Desktop Session Recovery And Search

- **Status:** approved for Stage 2 implementation.
- **Owner:** kernel owns session search and settled turn truth; desktop owns
  query entry, result rendering, and explicit retry submission.

## Production Target

An operator can search durable sessions from the rail, open a result, and retry
a terminally failed user turn without manually copying its text. A retry is a
new turn with a new idempotency key; it never rewrites, hides, or reopens the
failed turn.

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

## Non-Goals

- No full-text index in Vue, bulk retry, retry-policy engine, or hidden
  provider fallback.
- No mutation of session titles or kernel timeline facts by the desktop.

## Acceptance

- Search finds a persisted session and opens its existing timeline.
- A recoverable failed turn can be explicitly retried once as a distinct turn;
  its original failed evidence stays readable.
- Empty search restores the normal Project / Task / Chat grouping.
