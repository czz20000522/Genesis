# Requirement: Desktop Provider Onboarding

- **Status:** approved for implementation planning.
- **Owner:** desktop/localconfig owns provider-template configuration and
  protected credentials; kernel owns session binding, provider resolution, and
  execution.
- **Scope:** let a blank Genesis Home import a usable provider without a CLI,
  then expose its discovered profiles to the session model selector.

## Production Target

On a blank Home, desktop opens an import surface modeled on cc-switch's useful
shape: choose a curated provider template, enter the ordinary credential, and
optionally expand advanced configuration. The imported route discovers its
models and creates selectable Genesis profiles. The ordinary chat composer,
not a global settings page, is where each session chooses one of those models.

## Template Scope

The initial curated templates are DeepSeek, OpenAI, OpenCode Go, local
llama.cpp, and an explicitly advanced OpenAI-compatible route. A template
owns protocol defaults, endpoint defaults, credential-reference shape, and
model-discovery behavior. Ordinary fields are limited to a label and the
credential when applicable. Advanced fields remain explicit and optional.

## Semantics

1. Templates live in `localconfig`; Vue does not duplicate endpoint, adapter,
   credential, or discovery metadata.
2. The desktop transfers a one-shot key only to the existing protected
   credential writer. Safe provider/profile projections never contain a key,
   credential path, command environment, or raw local command arguments.
3. Import persists a route and its credential, then runs the existing
   adapter-aware model discovery path. Only discovered or explicitly supplied
   model ids become selectable profiles.
4. A failed import or discovery leaves prior routes and session bindings
   untouched. A successfully stored route whose discovery is temporarily
   unavailable is visible with a stable retryable reason.
5. Local llama.cpp import configures only desktop-owned launch metadata. It
   does not start a model automatically; the session chooser and existing local
   model lifecycle control remain explicit.

## Non-Goals

- No marketplace, arbitrary script execution, provider proxy, credential sync,
  or mass copy of cc-switch's preset catalog.
- No model mapping layer or global active-chat model.
- No automatic `coordinator` mutation or `genesisd` restart after import.

## Acceptance Criteria

1. A blank Home can import each curated cloud template with only the normal
   required field and then select a discovered model in a new session.
2. The advanced OpenAI-compatible template exposes endpoint/model fields only
   after the user deliberately expands it.
3. Importing a provider never changes an existing session's bound model.
4. A restart preserves routes, protected credentials, discovered profiles, and
   each session's selected model.
