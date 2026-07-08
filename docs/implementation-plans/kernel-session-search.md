# Kernel Session Search Implementation Plan

> **For agentic workers:** implement task-by-task with tests first. Keep search
> read-only and projection-owned.

## Requirement And Design

- Requirement: `docs/requirements/kernel-session-search.md`
- Design: `docs/design/kernel-session-search.md`
- BDD: `features/kernel/session_search.feature`

## Phase A: Bounded Session Search Projection

**Deliverable:** `GET /sessions/search?q=<query>` returns bounded session search
results over session id, title, first user text, and final assistant text.

**Files:**

- Modify: `internal/kernel/inspection_types.go`
- Modify: `internal/kernel/http.go`
- Modify: `internal/kernel/http_inspection.go`
- Add: `internal/kernel/session_search.go`
- Test: `internal/kernel/session_search_test.go`
- Optional desktop bridge after kernel route is stable.

**Red lines:**

- Do not create a semantic/vector search dependency.
- Do not expose raw event ids, operation ids, job ids, approval ids, credential
  refs, storage paths, or provider payloads in results.
- Do not mutate session titles or write ledger events.
- Do not make `/sessions` require a query.

- [x] Step 1: Add failing tests for HTTP validation.

  Cover missing `q`, empty `q`, invalid `limit`, and valid no-match query.

- [x] Step 2: Add failing tests for result matching.

  Cover matches by session id, title, first user text, and final assistant text.

- [x] Step 3: Add failing tests for restart stability and projection safety.

  Confirm results after reopening the same ledger and assert raw ids/path-shaped
  internals are absent from search JSON.

- [x] Step 4: Implement DTOs and kernel search helper.

  Add `SessionSearchResponse` and `SessionSearchResult`, then implement a
  projection helper using existing ledger/session projection code.

- [x] Step 5: Implement HTTP route.

  Add `GET /sessions/search` before `/sessions/{id}` routing and map validation
  errors to `invalid_request`.

- [x] Step 6: Verify.

  Run focused tests, then:

  ```powershell
  git diff --check
  go test ./... -count=1
  go build ./...
  ```

## Phase B: Shell/Desktop Integration

Only after Phase A is stable:

- [x] expose a desktop bridge method or console command that calls the kernel route;
- [x] keep rendering and selection in the shell;
- [x] opening a result must still use `/sessions/{id}`.

Delivered in Phase B:

- `desktop.App.SearchSessions(query, limit)` calls `GET /sessions/search` through
  the typed desktop HTTP bridge.
- `desktop/frontend/src/api/kernelApi.ts` exposes `searchSessions` for both Wails
  bridge and direct HTTP mode.
- No session rail or conversation UI behavior was added in this phase.
