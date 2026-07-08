# Requirement: Provider Model Refresh

- **Status:** approved for phased implementation.
- **Owner:** Genesis Kernel model configuration boundary.
- **Scope:** manually refresh provider model lists and persist bounded local model catalogs for later profile binding.

## Background

Genesis can already configure a single active model profile through
`models.json`, resolve credentials through local secret refs, and verify a live
provider. Some providers expose many usable models behind one base URL and one
credential. The operator needs a deterministic way to fetch that list once,
persist it locally, and then bind roles to selected models without refreshing
automatically on every startup.

This is configuration management, not provider context assembly. A refreshed
model list is an operator-owned local catalog. It does not by itself change the
active model used by a running turn.

## Production Target

Genesis supports manual model refresh for configured OpenAI-compatible provider
routes:

- refresh is explicitly invoked by an operator command or future kernel admin
  route;
- the provider model list is fetched from an authenticated `/models` endpoint;
- refreshed model ids are normalized, de-duplicated, sorted, and persisted under
  the provider or route catalog in `models.json`;
- existing active model profile bindings stay unchanged unless an explicit bind
  command changes them;
- refresh results expose model ids and sanitized diagnostics only, never API
  keys, credential refs, request headers, local secret paths, or raw response
  bodies;
- provider-command adapters are not probed for `/models` in Phase A unless they
  later declare an explicit model-list command contract.

## Users And Roles

Operator:

- refreshes a provider's model catalog after first configuration or after the
  vendor changes available models;
- reviews the persisted catalog before binding a role to one model.

Shell or desktop application:

- may call a typed command/API to refresh and render the returned catalog;
- does not scrape provider endpoints independently.

Kernel:

- owns config loading, credential resolution, fetch admission, response
  normalization, persistence, and redaction.

Provider:

- exposes a model list through OpenAI-compatible `GET /models`.

LLM:

- does not receive the provider catalog automatically.

## Semantics

1. Refresh is manual. Genesis never refreshes provider model catalogs during
   normal startup, readiness checks, turn submission, or provider context
   assembly.
2. Refresh uses the selected provider route from `models.json` and local
   credential refs. The raw key is never stored in `models.json` or output.
3. Phase A supports `openai-chat-completions` routes only.
4. A route may specify an override model-list URL in the config. Otherwise
   Genesis derives `{base}/models` and `{base}/v1/models` candidates and tries
   the fallback only when the first endpoint is plausibly missing.
5. Refresh writes a local catalog snapshot with:
   - provider or route id;
   - normalized model ids;
   - `refreshed_at`;
   - source endpoint class, not the full credential-bearing request;
   - sanitized readiness status.
6. Empty model lists are a structured failure by default; the previous catalog
   remains unchanged.
7. Network/auth/decode failures are reported as sanitized refresh failures and
   must not partially update the catalog.
8. Binding a role to a refreshed model is a separate explicit operation.
9. Existing single-profile resolution remains valid when no catalog exists.

## Non-Goals

- No automatic refresh on startup or when `/ready` is called.
- No model benchmarking, scoring, pricing fetch, balance fetch, or thinking
  strength selection in Phase A.
- No provider-specific endpoints beyond OpenAI-compatible `/models` candidates.
- No mutation of active role bindings during refresh.
- No compatibility reader for old experimental catalog files.
- No exposure of raw provider response bodies in user-facing diagnostics.

## Acceptance Criteria

1. A manual refresh command can fetch model ids from a configured
   OpenAI-compatible provider route.
2. Successful refresh persists a deterministic local catalog in `models.json`.
3. Refresh sorts and de-duplicates model ids.
4. Missing credential, auth failure, endpoint miss, empty response, and decode
   failure return distinct sanitized outcomes.
5. Failed refresh leaves the previous catalog and active bindings unchanged.
6. Provider-command routes are refused with a structured unsupported result in
   Phase A.
7. A later explicit bind operation can select a model from the catalog without
   fetching the provider again.
