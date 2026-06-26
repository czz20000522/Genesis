# Kernel Issue Ledger

This file is the repo-owned ledger for active Genesis Kernel issues. Feishu Base is the collaboration inbox; this file is the durable project source for issues that still need code, verification, or user acceptance.

Retired issues must not remain here. Move accepted retirements to `docs/operations/kernel-retirement-log.md` as compact evidence: one sentence with the retirement conclusion plus the fixing commit evidence.

## Ledger Rules

- Keep only `new`, `open`, `in_progress`, or otherwise active issues in this file.
- Do not record application-specific feature work as kernel work unless it changes a kernel primitive.
- Do not add versioned HTTP route names as current contracts. HTTP is transport; current kernel endpoints are unversioned.
- `ready_for_acceptance` issues move to the retirement log as retirement candidates and leave this active ledger.
- Feishu/Base links may point to collaboration artifacts, but this repo must contain enough evidence to understand the current status without opening Feishu.
- Every active `KERNEL-*` issue must include a `Reference alignment` field that compares the issue to Codex, Reasonix, or an explicitly rejected drift risk.
- Reference alignment uses local reference checkouts only. Do not cite Genesis GitHub repositories, remote issues, releases, or pull requests as authority for this local kernel line unless the user explicitly asks for external publishing context.
- Before a non-trivial implementation slice starts, the related implementation plan or issue must include a Codex/Reasonix reference scan. The scan should identify inspected references, learned control-plane semantics, intentional differences, and remaining drift risks.
- Reference scans must translate relevant Codex/Reasonix behavior tests into Genesis same-semantics red tests or explicitly reject them with a reason. A prose-only reference note is not enough for kernel primitives.
- Issues record the current gap between approved requirements/designs and the implementation. They must not carry raw requirements, design discussion, or the full production acceptance contract.
- Every active issue must cite an approved requirement and design unless it is an obvious bug or test gap. If an issue uses that exception, state the exception explicitly.
- Prefer `Gap`, `Next slice`, `Evidence`, and `Verification` over broad `Problem` or `Suggestion` text when adding new issues.
- Do not use issues as debug logs. Routine info, stream chunks, repeated status polling, and exploratory notes stay out of this ledger unless they identify a current implementation gap.
- When an issue removes a concept from the current kernel contract, long-term tests must assert the positive replacement behavior. Do not keep permanent tests whose only purpose is locking retired names, aliases, routes, or historical helper APIs out of the tree; use temporary scans or retirement-log evidence for cleanup windows, then fold the guard into the current owner contract.
- Development artifacts and historical local data are not compatibility obligations. Do not create or keep issues whose only purpose is migrating, cleaning, importing, or preserving old local generated state unless that state is part of the approved current kernel contract.
- Every implementation slice must finish with a drift check against the governing requirement, design, implementation plan, issue, and BDD feature. In-scope drift is fixed before commit. Out-of-scope drift is recorded here as an active issue with evidence and next slice before commit.
- Periodic governance review checks architecture, feature behavior, directory structure, and document lifetime together. Completed plans and stale documents should be deleted or condensed instead of spawning issues that only preserve old notes.

## Active Issues

### KERNEL-MODEL-PROVIDER-ADAPTER-BOUNDARY-20260626 - P1 - Provider presets must resolve to third-layer adapters, not raw kernel routes

- Status: open.
- Requirement: `docs/requirements/kernel-foundation-capabilities.md` and the
  Model Gateway/provider boundary in `docs/kernel-contract.md`.
- Design: four-layer provider tree:
  kernel/generic model protocol -> Model Gateway primitive -> provider adapter
  -> vendor API.
- Gap: The current `genesisctl provider use` implementation correctly improves
  operator UX, but it models DeepSeek and SCNet primarily as CLI presets that
  write OpenAI-compatible route fields. That is not enough for the approved
  boundary. DeepSeek and SCNet are third-layer provider adapter nodes. The
  kernel and generic Model Gateway request/response must not know vendor private
  fields such as DeepSeek `reasoning_content`, while the provider adapter must
  own vendor-specific profile behavior, capability declarations, translation,
  passback into approved Genesis typed fields, or explicit refusal when a profile
  cannot support the requested feature.
- Next slice: Add a minimal provider adapter contract behind the Model Gateway.
  `provider use deepseek/...` and `provider use scnet/...` should resolve to
  adapter-owned profiles, not only raw `base_url` routes. The DeepSeek adapter
  should own DeepSeek thinking/reasoning behavior such as `reasoning_content`
  passback or explicit unsupported-profile refusal. The SCNet adapter should own
  SCNet model catalog/profile behavior and may delegate to the shared
  OpenAI-compatible chat-completions transport internally. Keep the shared
  transport small; do not add DeepSeek/SCNet branches to kernel sampling logic.
- Evidence: Local code inspection on 2026-06-26 found
  `cmd/genesisctl/provider_presets.go` defines DeepSeek and SCNet presets with
  base URL, model id, credential ref, and timeout. `cmd/genesisctl/main.go`
  passes those fields directly into
  `kernel.SetupOpenAICompatibleProvider`. `internal/kernel/model_config.go`
  resolves protocol `openai-chat-completions` into `OpenAICompatibleConfig` and
  has no explicit adapter identity, adapter capability contract, or
  adapter-owned place for vendor-specific response fields.
- Verification: Add boundary tests proving generic kernel/Model Gateway typed
  request and response structs do not expose DeepSeek `reasoning_content` or
  SCNet vendor wire fields. Add adapter tests proving DeepSeek vendor
  `reasoning_content` is either mapped into an approved Genesis reasoning field
  for supported profiles or rejected/refused structurally for unsupported
  profiles. Add SCNet adapter tests proving SCNet profiles use adapter-owned
  model catalog/base URL/credential refs while reusing shared OpenAI-compatible
  transport internally. Add architecture guard that provider-specific names may
  appear in adapter/preset packages, but not in kernel sampling, provider
  context projection, or generic Model Gateway DTOs.
- Priority: P1.
- Reference alignment: cc-switch is useful for preset-backed provider switching,
  but its app-specific config writer is not the Genesis kernel boundary. Codex
  and Reasonix keep the model loop speaking their own typed protocol and put
  backend-specific wire shape behind provider/client adapters. Genesis should do
  the same: presets select adapter profiles; adapters translate vendor protocol;
  kernel sees only Genesis typed model requests/responses.

### KERNEL-PROVIDER-FAKE-PRODUCTION-GUARD-20260626 - P0 - Fake provider must not be a production-ready provider

- Status: open.
- Requirement: obvious production-safety bug; related production target in
  `docs/requirements/kernel-foundation-capabilities.md` and live provider
  acceptance in `docs/operations/live-llm-first-run-acceptance.md`.
- Design: no separate design needed before the first fix; this is a provider
  readiness/admission hardening issue.
- Gap: `cmd/genesisd` currently maps `-provider fake` and an explicitly empty
  provider name to `kernel.FakeProvider{}`. A running `genesisd` then reports the
  fake provider as `ready`. Fake provider is valid only as a deterministic
  lab/test fixture for proving HTTP, ledger, session, projection, and tool-loop
  plumbing. In a production or user-facing daemon, fake provider readiness would
  be a severe misconfiguration because it can make the system look usable while
  no real model is connected.
- Next slice: Make fake provider opt-in as lab/test mode rather than a normal
  production provider. Production-facing startup should prefer `genesis-config`
  and should not silently fall back to fake when provider config is empty,
  missing, invalid, or credential-blocked. A fake provider, if explicitly allowed
  for local smoke tests, must be visibly marked as lab-only in readiness and must
  be rejected by live-provider acceptance gates.
- Evidence: Pressure smoke on 2026-06-26 verified that a fake `genesisd` can
  return `provider.readiness=ready` and complete multi-turn HTTP requests. The
  same smoke verified that `openai-compatible` without `GENESIS_PROVIDER_API_KEY`
  already returns structured `/ready` state:
  `readiness=not_ready`, `readiness_reason=provider_not_ready`,
  `provider.readiness=not_ready`, and
  `provider.readiness_reason=provider_api_key_missing`. The issue is therefore
  not missing structured not-ready behavior for real provider credentials; the
  issue is that fake can still be admitted as a ready provider on the daemon
  production surface.
- Verification: Add red tests proving `genesisd` does not treat fake as a
  production-ready provider by default, an explicitly empty provider name does
  not select fake, missing real-provider credentials remain structured
  `not_ready`, and live-provider acceptance fails if `/ready` or `/turn` uses a
  fake provider. Keep existing deterministic unit tests able to inject
  `FakeProvider` directly as a test fixture without changing kernel semantics.
- Priority: P0.
- Reference alignment: Codex and Reasonix use fake/stub model paths for tests
  and local deterministic harnesses, but production-facing runtime readiness is
  not allowed to present a fake model backend as an ordinary connected provider.
