# Kernel Issue Ledger

This file is the repo-owned ledger for active Genesis Kernel issues. Feishu Base is the collaboration inbox; this file is the durable project source for issues that still need code, verification, or user acceptance.

Retired issues must not remain here. Move accepted retirements to `docs/operations/kernel-retirement-log.md` with the fixing commits, verification evidence, residual risks, and retirement reason.

## Ledger Rules

- Keep only `new`, `open`, `in_progress`, or otherwise active issues in this file.
- Do not record application-specific feature work as kernel work unless it changes a kernel primitive.
- Do not add versioned HTTP route names as current contracts. HTTP is transport; current kernel endpoints are unversioned.
- `ready_for_acceptance` issues move to the retirement log as retirement candidates and leave this active ledger.
- Feishu/Base links may point to collaboration artifacts, but this repo must contain enough evidence to understand the current status without opening Feishu.
- Every active `KERNEL-*` issue must include a `Reference alignment` field that compares the issue to Codex, Reasonix, or an explicitly rejected drift risk.
- When an issue removes a concept from the current kernel contract, long-term tests must assert the positive replacement behavior. Do not keep permanent tests whose only purpose is locking retired names, aliases, routes, or historical helper APIs out of the tree; use temporary scans or retirement-log evidence for cleanup windows, then fold the guard into the current owner contract.

## Active Issues

### KERNEL-LIVE-LLM-FIRST-RUN-ACCEPTANCE-20260622 - P0 - Real LLM must have a user-executable first-run acceptance path

- Status: new.
- Type: user feedback.
- Problem: Genesis has provider setup, `models.json`, `secret://` credential refs, OpenAI-compatible/provider-command paths, and gated live smoke tests, but a user still has to piece together first-run live LLM validation from README fragments and test names.
- Suggestion: Add a user-executable first-run acceptance path, preferably a runbook or script. It must cover provider setup, `genesisd` startup through Genesis config, `/ready`, a real `/turn`, timeline/events/context inspection, restart replay, and structured provider failure diagnostics. It must prove this with a real provider credential path, not only fake provider or `GENESIS_LIVE_PROVIDER=1` developer tests.
- Evidence: README describes provider setup and live provider fragments; live smoke tests are gated by `GENESIS_LIVE_PROVIDER=1`; there is no single acceptance artifact that walks a user through setup, run, inspect, restart, and failure validation.
- Validation: In a clean temporary config/credential/ledger setup, follow the runbook or script to write a provider credential without printing the raw key, start `genesisd` with Genesis config, get `/ready=ok`, submit a real turn with non-empty assistant final, inspect `/sessions/{id}/timeline`, `/turns/{id}/events`, and `/turns/{id}/context`, restart and re-read the same projections, then intentionally break credential or endpoint and observe structured readiness/turn errors without panic or secret leakage.
- Reference alignment: Codex and Reasonix both keep provider configuration and live smoke paths executable by operators rather than hidden in tests. Genesis needs the same operator-facing acceptance surface while keeping credentials and provider-specific details outside kernel logic.

### KERNEL-PROVIDER-GATEWAY-EVENT-PROJECTION-20260622 - P1 - Provider gateway should be driven by provider-visible event projection

- Status: new.
- Type: architecture issue.
- Problem: `provider_command` now avoids leaking kernel-owned event identity, but the long-lived provider boundary is still shaped around internal `ModelRequest` fields (`input_items`, `tool_manifest`, `tool_rounds`) rather than a provider-visible event projection from the ledger. This risks creating parallel context semantics as reasoning deltas, message deltas, compression, memory, skills, UI timeline, and audit replay expand.
- Suggestion: Define and converge toward `ProviderContextProjection` / `ModelGatewayStepRequest`: input is kernel event log, output is only provider-visible event stream plus tool manifest. Provider adapters consume that projection, not raw ledger and not internal owner structs. Provider responses should be typed and whitelisted, for example assistant reasoning delta, assistant message delta/completed, and tool call; provider must not return kernel-owned events such as tool result, checkpoint, session completed, sandbox, authority, or audit events.
- Evidence: `docs/kernel-contract.md` still describes `provider_command` as receiving ordered input items, tool manifest, and prior model-visible tool rounds. `internal/kernel/provider.go` defines `ModelRequest` with input/tool round fields, and `internal/kernel/provider_command.go` projects from that request. No `ProviderContextProjection` or assistant reasoning event projection exists yet.
- Validation: Add fixtures where kernel event log includes user messages, assistant deltas, tool calls/results, checkpoint/session/audit/sandbox facts, and credentials. The provider projection must include only model-visible content and tool manifest, filter or summarize kernel-owned events, redact/truncate visible output, and reject provider responses that try to emit kernel-owned event types. A provider-command two-step tool loop must prove the second step sees model-visible tool result content, not raw operation/audit events.
- Reference alignment: Codex separates provider/model-visible call identity from canonical host identity. Reasonix separates event stream facts from frontend/provider projections. Genesis should make provider-visible event projection the gateway contract instead of letting provider adapters consume owner-internal structs.

### KERNEL-EVENT-OBSERVABILITY-POLICY-20260622 - P1 - Separate UI timeline, raw event inspection, audit log, and provider context projections

- Status: new.
- Type: architecture issue.
- Problem: Genesis now has initial timeline, context inspection, and raw event routes, but does not yet fully define the policy boundary among user timeline, raw event inspection, audit/replay log, and provider context replay. Without this, future WebUI/logging/provider work may reuse one over-rich object for all audiences.
- Suggestion: Define and implement four projection/retention policies: `UiTimelineProjection` for ordinary user chat cards; `RawEventInspectionProjection` for authorized debug events with redaction/truncation; `AuditReplayLog` for replay/export facts, provider-visible snapshot refs/hashes, visible output, and truncation metadata; `ProviderContextProjection` for next-step model-visible content only. Reasoning content may be recorded/displayed but must not be blindly replayed to providers.
- Evidence: `docs/kernel-contract.md` distinguishes `/sessions/{id}/timeline`, `/turns/{id}/context`, and `/turns/{id}/events`; `internal/kernel/projections.go` implements initial timeline/context projections. There is no complete raw-event inspection/audit replay/provider replay policy yet.
- Validation: A fixture with user message, assistant final, tool call, operation running/completed, tool result, failure, and memory recall must prove: timeline omits ids/raw event types; raw event inspection includes typed envelope and redacted payload; audit/replay output has replay facts and truncation metadata without credentials; provider context omits audit/permission/raw operation details. Docs must state when WebUI should use timeline, context drawer, raw event inspector, and audit log.
- Reference alignment: Reasonix uses separate transcript/tool card/context concepts, and Codex keeps protocol/tool/audit boundaries distinct. Genesis should follow that separation so “show complete information” does not collapse UI, audit, and model context into one object.
