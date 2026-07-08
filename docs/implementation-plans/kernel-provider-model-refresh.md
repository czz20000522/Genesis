# Kernel Provider Model Refresh Implementation Plan

> **For agentic workers:** keep refresh manual, config-owned, and redacted.

## Requirement And Design

- Requirement: `docs/requirements/kernel-provider-model-refresh.md`
- Design: `docs/design/kernel-provider-model-refresh.md`
- BDD: `features/kernel/provider_model_refresh.feature`

## Phase A: Manual Refresh And Local Catalog

**Deliverable:** `genesisctl provider models refresh --json` fetches the selected
OpenAI-compatible provider's model ids and persists a local catalog snapshot
without changing active profile bindings.

**Files:**

- Modify: `internal/kernel/model_config.go`
- Modify: `internal/kernel/provider_setup.go` or new model-refresh owner file
- Modify: `cmd/genesisctl/main.go`
- Test: `internal/kernel/provider_model_refresh_test.go`
- Test: `cmd/genesisctl/main_test.go`

**Red lines:**

- Do not refresh automatically from `/ready`, daemon startup, or turn execution.
- Do not write API keys, credential refs, headers, provider response bodies, or
  local secret paths into `models.json` catalog entries.
- Do not mutate active model profile bindings during refresh.
- Do not probe `provider_command` routes for `/models` in Phase A.

- [x] Step 1: Add failing kernel tests for successful refresh.

  Cover authenticated `/models`, sorted/de-duplicated ids, persisted catalog,
  and unchanged active profile binding.

- [x] Step 2: Add failing kernel tests for failure safety.

  Cover missing credential, unsupported protocol, endpoint miss, auth failure,
  empty list, decode failure, and no partial catalog update.

- [x] Step 3: Implement refresh fetch and catalog persistence.

  Add an internal refresh request/result API, bounded response parsing, endpoint
  candidate derivation, and sanitized reason classification.

- [x] Step 4: Add `genesisctl provider models refresh`.

  Support `--json`, config/credential roots, role/profile override, timeout, and
  dry-run if useful. Output only sanitized fields.

- [x] Step 5: Verify.

  Run focused kernel and CLI tests, then:

  ```powershell
  git diff --check
  go test ./... -count=1
  go build ./...
  ```

## Phase B: Explicit Catalog Binding

Only after Phase A is stable:

- [x] add a bind command that selects a model id from the persisted catalog;
- [x] update the selected profile `model_id` explicitly;
- [x] keep role binding changes separate and visible.

Delivered in Phase B:

- `BindProviderModelFromCatalog` updates the selected profile's `model_id` only
  when the requested model exists in the persisted catalog.
- `genesisctl provider models bind <model-id> [--json]` exposes the operation.
- The role binding remains unchanged; creating additional profiles can be a
  later explicit extension if needed.
