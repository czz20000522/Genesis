# Design: Session Model Binding

- **Requirements:** `docs/requirements/kernel-session-model-binding.md` and
  `docs/requirements/desktop-provider-onboarding.md`.
- **Owner split:** kernel owns durable session profile binding and per-turn
  provider choice; localconfig owns profile configuration; desktop renders
  projections and submits explicit requests.

## Reference Scan

Codex records provider/model metadata on its thread state in
`codex-rs/state/src/model/thread_metadata.rs`, while its selection helper
updates model configuration separately in `codex-rs/tui/src/config_update.rs`.
Genesis aligns with the thread-local model truth and intentionally differs by
keeping local provider configuration in Genesis Home rather than Codex config.

Reasonix `desktop/app.go:SetModelForTab` validates an idle tab, carries history
into a new provider controller, and persists its tab model; its
`ModelSwitcher.tsx` renders a compact composer-adjacent chooser. Genesis reuses
the idle-switch guard and compact chooser, but rejects a replacement session:
the Genesis ledger session persists and receives a new model-binding fact.

cc-switch's `ProviderPresetSelector.tsx` and
`UniversalProviderFormModal.tsx` separate a preset choice from ordinary
credential fields and advanced endpoint configuration. Genesis reuses that
shape with five curated templates, not cc-switch's provider catalog or proxy
configuration.

## Kernel Protocol

```text
desktop selects profile for session S
  -> POST /sessions/S/model { profile_id }
  -> kernel rejects active S or an unavailable profile
  -> ledger session.model_bound { profile_id }
  -> SessionProjection safe model binding

desktop submits a turn for S
  -> kernel restores S's profile id
  -> session ProviderResolver(profile id)
  -> one provider fixed for all rounds of this turn
  -> existing adapter-specific context/replay and durable turn facts
```

`Kernel.Config` receives a session-provider resolver alongside the existing
worker resolver. `genesisd` implements it by rebuilding that session's
coordinator provider from the selected configured profile. No global
`coordinator` binding is required. The kernel never receives a credential,
endpoint, or route from desktop. The resolver is selected once before a turn
starts; a model change cannot race a running turn.

`session.model_bound` is an append-only control fact. Projection reduces the
latest binding. Session list and search may expose a safe model label only if
the projection can derive it without reading raw credentials or command data.

## Desktop Shape

The composer owns the session model chooser. It shows the bound model plus a
chevron when selected, or `选择模型` when unbound. Its menu groups safe profile
projections by provider route and displays model and provider labels. Selecting
an item binds only the current session, then updates the composer immediately.
The send action is disabled for an unbound session with one inline instruction.

Provider panel behavior changes from “global default model” to “导入与管理模型”:
blank Home shows template cards; a configured Home lists routes, credentials,
discovery/retry status, and model profiles. Existing parent/worker runtime
controls stay separate and do not appear in the ordinary composer.

## Failure And Recovery

- Unbound session: return `session_model_unselected`; no provider call or
  fallback occurs.
- Active session: return `session_model_change_blocked_active_turn`; no event
  is appended.
- Resolver/discovery failure: preserve the binding/configuration, return a
  stable redacted reason, and offer explicit retry.
- Restart: replay binding facts before a new turn; rebuild the selected
  provider only when that session submits a subsequent turn.

## Rejected Alternatives

- Global `coordinator` switching: one session's choice would silently change
  every other session.
- Desktop catalog selection: it would be lost or contradicted outside desktop
  and cannot govern kernel provider execution.
- Reasonix-style session replacement: it breaks the user's requirement that
  the same Genesis session retain its identity and history.
- Full cc-switch template catalog: it duplicates an unrelated proxy product
  and makes every template a long-term Genesis maintenance commitment.
