# Requirement: Kernel Session Search

- **Status:** approved for phased implementation.
- **Owner:** Genesis Kernel.
- **Scope:** local, ledger-backed discovery of existing sessions through bounded search projections.

## Background

Genesis already exposes exact session reads and a minimal session list. As the
ledger grows, an operator or shell needs a deterministic way to find a prior
session without loading every full session projection or asking a model to infer
history from raw events.

Session search is not memory recall and not provider context injection. It is a
read-only projection over kernel-owned session facts.

## Production Target

Genesis exposes a generic session search surface that lets callers find sessions
by session id, title, and bounded transcript preview fields while preserving
ledger authority and projection safety.

The production target is:

- search is rebuilt from ledger-backed session projections or session read
  models;
- search never rewrites history, titles, turns, or memory;
- results expose bounded snippets and stable metadata, not raw events;
- search does not inspect application-specific domains or external stores;
- future indexes may optimize retrieval, but the event ledger remains truth.

## Users And Roles

Ordinary user:

- searches prior conversations from a shell or desktop sidebar;
- opens the selected session through the existing session projection.

Operator/admin:

- diagnoses whether sessions exist after restart;
- verifies search/read-model health without direct database inspection.

Application:

- calls a kernel search route and renders results;
- does not build its own history index from raw ledger files.

Kernel:

- owns query validation, result limits, snippet construction, and replay or index
  access.

LLM:

- does not receive session search results automatically.
- may see user-selected session context only through later explicit kernel
  admission rules outside this requirement.

## Semantics

1. Session search is a read-only projection.
2. Empty or whitespace-only query is invalid for search; callers that want all
   sessions use the existing list surface.
3. Query matching is deterministic and case-insensitive for Phase A.
4. Phase A matches only safe fields:
   - `session_id`;
   - session list `title`;
   - first user text and final assistant text when those snippets are already
     derivable from the session projection.
5. Results are ordered by most recent `updated_at`, then `session_id`.
6. Result count is bounded by a kernel-owned limit with a small default.
7. Snippets are bounded and must not expose raw event ids, operation ids, job ids,
   approval ids, credential refs, local paths, provider payloads, or storage
   paths.
8. A malformed ledger or unavailable index fails as ledger unavailable; search
   does not silently return partial results.
9. Search result ids are session ids. Opening a result uses the existing
   `/sessions/{id}` projection.

## Non-Goals

- No vector database, embeddings, semantic retrieval, OCR, source-code search, or
  memory recall.
- No automatic provider-context injection from search results.
- No application-specific facets such as Feishu, calendar, email, insurance, or
  medical domains.
- No mutation of session titles or generated summaries.
- No compatibility readers for old development artifacts.

## Acceptance Criteria

1. `GET /sessions/search?q=<query>` returns bounded JSON search results.
2. Missing `q` or an empty query returns a structured validation error.
3. Search matches `session_id` and title without loading unrelated application
   state.
4. Search can find a session by bounded user or final assistant transcript text.
5. Results omit raw event ids and path-shaped or credential-shaped internals.
6. Restarted kernels return the same search results from the same ledger.
7. `/sessions` continues to list sessions without requiring a search query.

