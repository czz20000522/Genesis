# Design: Kernel Session Search

- **Requirement:** `docs/requirements/kernel-session-search.md`
- **Owner:** Genesis Kernel projection layer.

## Reference Scan

Codex:

- `codex-rs/tui/src/bottom_pane/chat_composer/history_search.rs` keeps the
  active search interaction in the composer while leaving persistent history
  storage and traversal rules outside the UI state.
- The important boundary is that search previews are transient UI state; they do
  not mutate the draft until the user accepts a result.

Reasonix:

- `internal/serve/serve.go` exposes `/sessions` and `/history` as read-only
  session projections. The `/sessions` handler scans persisted session files,
  derives title/turn preview metadata, orders newest first, and leaves resume or
  delete as separate explicit operations.
- `internal/acp/e2e_test.go` covers `session/list`, `session/resume`, and
  deletion as separate ACP operations over persisted transcripts.

Genesis alignment:

- Genesis should keep search as a projection beside `ListSessions`, `Session`,
  and timeline projections.
- Search should return enough metadata for a shell to select a session, but the
  full conversation remains behind `/sessions/{id}`.

## Owner Boundary

Owner: kernel projection/read-model layer.

Non-owners:

- Desktop, console, provider commands, and application connectors may render or
  call search, but they do not build the search index.
- Model Gateway does not inject search results into provider context.
- Memory and accumulation subsystems do not own session search truth.

## Data Flow

Phase A:

1. HTTP validates `q` and optional `limit`.
2. Kernel loads session list from the ledger read model when available.
3. Kernel loads ledger events and builds bounded per-session searchable previews
   only for candidate sessions needed by the query.
4. Kernel matches query case-insensitively against session id, title, first user
   text, and final assistant text.
5. Kernel returns ordered `SessionSearchResult` projections.

Future phases may add a SQLite search read model, but it remains rebuildable from
ledger events.

## Projection Shape

```json
{
  "query": "string",
  "items": [
    {
      "session_id": "string",
      "title": "string",
      "updated_at": "time",
      "match_fields": ["session_id", "title", "user_text", "assistant_text"],
      "snippet": "bounded text"
    }
  ]
}
```

Rules:

- `query` is normalized trimmed input.
- `match_fields` contains semantic field names only.
- `snippet` is bounded plain text. It is not an event payload and not a path to
  underlying storage.
- No raw ids except `session_id`.

## Failure Semantics

- Empty query: HTTP 400 with `invalid_request`.
- Invalid limit: HTTP 400 with `invalid_request`.
- Ledger unavailable or corrupt: existing ledger unavailable envelope.
- No matches: HTTP 200 with an empty `items` array.

## Permission And Safety

Session search is a protected inspection route. It requires the same runtime
token as `/sessions`.

Search must not expose:

- event ids;
- operation ids;
- job ids;
- approval ids;
- credential refs;
- raw provider payloads;
- local storage paths.

## Observability

No audit event is required for ordinary search reads. Audit remains reserved for
authority, risk, credentials, control-plane writes, security, and recovery
events.

