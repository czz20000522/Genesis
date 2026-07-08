# Design: Provider Model Refresh

- **Requirement:** `docs/requirements/kernel-provider-model-refresh.md`
- **Owner:** Genesis Kernel model configuration boundary.

## Reference Scan

Codex:

- `codex-rs/core/tests/suite/models_cache_ttl.rs` exercises a models manager
  that can use cached model metadata offline, refresh when cache version is
  missing or stale, and avoid network calls when a cache is valid.
- `codex-rs/core/tests/suite/models_etag_responses.rs` refreshes `/models` when
  response headers indicate a catalog change, but also proves the refresh is
  deduplicated.

Reasonix:

- `internal/provider/openai/fetch_models.go` fetches OpenAI-compatible
  `GET /models`, limits response size, sorts model ids, and classifies status
  errors.
- `internal/config/fetch.go` derives likely model-list URLs from a provider base
  URL and only falls back to `/v1/models` when the first endpoint is missing.
- `internal/config/config.go` treats a provider as one base URL and key with
  many models through `Models`, while `ModelList`, `DefaultModel`, and
  `HasModel` are local config operations after fetch.

Genesis alignment:

- Genesis should follow the Reasonix operator flow of fetching once and
  persisting selected provider models, while adopting Codex's principle that
  cached local model metadata remains usable without network access.
- Genesis intentionally differs from Codex by making refresh manual in Phase A.
  The local catalog changes only when an operator invokes refresh.

## Owner Boundary

Owner: kernel model configuration boundary and `genesisctl` operator commands.

Non-owners:

- Model Gateway uses the active profile selected by config; it does not refresh
  provider catalogs during turns.
- Desktop and console may call typed commands later, but do not own catalog
  persistence.
- Provider commands do not invent catalog facts unless a future contract defines
  model-list responses.

## Data Flow

Phase A refresh:

1. Operator runs `genesisctl provider models refresh` with optional role/profile
   and config-root overrides.
2. Command resolves the selected gateway profile and route from `models.json`.
3. Kernel confirms the route protocol is `openai-chat-completions`.
4. Kernel resolves the local credential ref into an API key for the request only.
5. Kernel derives model-list endpoint candidates or uses an explicit override.
6. Kernel fetches a bounded `/models` response, extracts model ids, sorts and
   de-duplicates them, and rejects an empty list.
7. Kernel writes the catalog snapshot into `models.json` without changing active
   role bindings.
8. Command prints a sanitized summary.

Phase B binding:

1. Operator chooses a model id from the local catalog.
2. A bind command updates a profile's `model_id` or creates a new profile under
   the same provider route.
3. The selected role binding changes only when explicitly requested.

## Config Shape

The persisted catalog is local configuration, not ledger truth. A future
implementation may choose the exact JSON shape, but it must preserve these
semantic fields:

```json
{
  "provider_model_catalogs": {
    "route-or-provider-id": {
      "route": "openai",
      "protocol": "openai-chat-completions",
      "models": ["model-a", "model-b"],
      "refreshed_at": "2026-07-08T00:00:00Z",
      "source": "models_endpoint"
    }
  }
}
```

The catalog must not contain API keys, credential refs, request headers, local
secret paths, raw response bodies, or provider error bodies.

## Failure Semantics

- Missing config: `provider_config_missing`.
- Invalid config: `provider_config_invalid`.
- Missing/unsupported credential: existing provider credential reason.
- Unsupported protocol: `provider_model_refresh_unsupported`.
- Endpoint not implemented: `provider_models_endpoint_missing`.
- Auth failure: `provider_models_auth_failed`.
- Empty list: `provider_models_empty`.
- Decode failure: `provider_models_decode_failed`.
- Network failure: `provider_models_request_failed`.

Failures do not change active bindings or existing catalogs.

## Permission And Safety

Refresh is an operator command with network side effects. It must be explicit.
Diagnostics are sanitized. The raw provider response body can be logged only in
debug files designed for operator inspection, not in default JSON output.

## Observability

`genesisctl` output should include:

- readiness/status;
- selected profile or route;
- count of models refreshed;
- first few model ids only when useful for humans;
- sanitized reason on failure.

No event-ledger audit entry is required for local config refresh until the
kernel exposes this as a runtime control-plane route.
