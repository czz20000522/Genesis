# Application Issue Ledger

This file records active issues for user-space applications that exercise the
Genesis Kernel. Kernel primitive gaps belong in
`docs/operations/kernel-issues.md`.

## Ledger Rules

- Application issues must cite an approved application requirement and design
  unless they are obvious bugs or test gaps.
- Do not record kernel work here. If a gap requires a new kernel primitive,
  create or update the relevant kernel requirement/design/issue.
- Completed issues leave this ledger and move to application retirement evidence
  when such a log is needed. Retirement evidence is compact: one sentence with
  the retirement conclusion plus fixing commit evidence, not a repeated fix
  summary or verification transcript.
- Temporary or narrow implementation slices must name their retirement target.
  Once the approved primitive exists, retire the temporary package, tests, docs,
  and command references instead of preserving a second truth surface.
- Every application issue or next slice must name the kernel primitive or owner
  capability it pressure-tests. If the slice begins faking facts that belong to
  kernel, connector, memory, credential, permission, job, audit, or provider
  owners, stop the app slice and move the gap to the owning issue ledger.
- Issues should stay small: requirement, design, gap, next slice, evidence,
  verification, and reference alignment.

## Active Issues

### APP-CONNECTOR-COMMAND-OUTPUT-BOUNDS-20260625 - P2 - Connector external command output must be bounded before parsing

- Status: open.
- 标题: Connector external command output must be bounded before parsing.
- 问题: Application Connector Runtime already records normalized connector actions, results, and delivery receipts instead of persisting raw external commands, but the in-process external CLI runner still captures command output with `CombinedOutput()` before the adapter parses or truncates it. That leaves Feishu/lark-cli and future connector drivers able to return arbitrarily large stdout/stderr into process memory before `firstStringAtJSONPath`, `firstSafeExternalActionRef`, or `SafeCLIProbeExcerpt` can trim the visible projection.
- 建议: Replace the shared connector `OSCommandRunner` capture path with a bounded runner that mirrors the existing `ConnectorCommandAdapter` capped stdout/stderr behavior. The runner should cap captured output before returning bytes, expose a truncation flag or stable failure reason, redact credential-shaped diagnostics for operator-visible failure summaries, and preserve direct-argv execution without shell strings. Add tests for oversized successful stdout, oversized stderr on command failure, and oversized output returned through both `CommandTemplateDriver` and `cmd/genesis-feishu-connector-adapter`.
- 证据: `internal/applications/connector_runtime/adapters.go::OSCommandRunner.Run` calls `cmd.CombinedOutput()` directly. `internal/applications/connector_runtime/command_template_driver.go` and `cmd/genesis-feishu-connector-adapter/main.go` parse only the first 4096 bytes for external action refs after the runner has already returned the full output. `internal/applications/connector_runtime/connector_command_adapter.go` already uses `connectorCommandCappedBuffer` with `maxConnectorCommandOutputBytes`, so the lower-level connector-command process boundary is bounded while the concrete CLI driver boundary is not. Existing connector tests cover redaction and malformed refs, but no test covers oversized external CLI output before parsing.
- 验证: Add a fake `CommandRunner` or local helper process that emits output larger than the configured connector command cap. Verify `CommandTemplateDriver.Execute` and `genesis-feishu-connector-adapter` return a bounded result without storing or exposing the full output, and that failure receipts/reasons do not contain raw oversized or credential-shaped output. Run `go test ./internal/applications/connector_runtime ./cmd/genesis-feishu-connector-adapter -count=1`.
- 优先级: P2.
- Reference alignment: Aligned with Codex's bounded/truncated tool output and context projection tests, including `codex-rs/core/src/exec.rs` capped process output and `codex-rs/core/src/tools/mod.rs` truncation formatting. Aligned with Reasonix's `internal/hook` capped output buffers and `internal/agent/guards_test.go` head/tail tool-output truncation. Genesis should intentionally keep connector raw CLI output as debug-only material, but the external process capture itself must still be bounded before parsing.

### APP-CONNECTOR-FEISHU-LISTENER-20260623 - P2 - Connector source verification and lifecycle gate

- Status: open.
- Requirement:
  `docs/applications/connector-source-verification-lifecycle-requirement.md`;
  broader connector contract:
  `docs/applications/application-connector-runtime-requirement.md`.
- Design: `docs/applications/connector-source-verification-lifecycle-design.md`;
  broader connector design:
  `docs/applications/application-connector-runtime-design.md`.
- Kernel/owner pressure: connector source verification, inbound dedupe, session
  mapping, turn submission, and connector outbox delivery without kernel Feishu
  ownership.
- Gap: The smoke path now has deterministic ExternalEvent NDJSON ingestion, a `source_command` typed streaming boundary for inbound source adapters, a Feishu source adapter command that owns `lark-cli event consume` and raw Feishu payload parsing, durable connector-local source failure records with redacted diagnostics, connector-local `SourceRun` / `SourceAttempt` / `SourceCursor` / `SourceVerificationEvidence` state, bounded generic retry/backoff for recoverable `source_command` process failures, connector-local operator lifecycle controls, an operator-run `genesis-ingress feishu-probe` source-command readiness report, and an opt-in `--deliver-final` path that turns kernel final text into one connector-owned send-message outbox item. Real source events still default to `unchecked` because source readiness does not imply event verification. Profile readiness can now block source start and final delivery with `missing_profile`, `profile_expired`, `permission_denied`, or `refresh_required`. Remaining production gap is real adapter/credential-provider profile probing, automatic refresh posture integration, and production-grade source recovery beyond bounded retry/backoff.
- Next slice: Keep inbound source intake through `source_command` and keep Feishu event argv in the Feishu source adapter. Product smoke commands must pass the Genesis profile (`--profile genesis`) for the Genesis bot identity; Codex developer-originated test messages may still use the Codex profile as the sender. Future source verification work must add real event authenticity evidence rather than upgrading readiness to verification. Connector-specific reconciliation probes should wait until exact outbound action refs, idempotency keys, or external receipt refs exist; reconciliation remains outbound recovery work.
- Evidence: `genesis-ingress feishu-listen --stdin-jsonl ...` covers automated smoke and dedupe without a source process, with synthetic source validation remaining `unchecked` unless evidence exists. Non-stdin source intake now starts a configured `SourceCommandAdapter` process and consumes typed `source.ready`, `source.event`, `source.cursor`, `source.failed`, and `source.stopped` frames through the source command intake loop, which retries only recoverable runtime failures and does not retry blocked readiness/configuration failures. Verified source events require source/connector/adapter/event-bound evidence with an approved evidence kind before they are handled or recorded. Blocked/degraded source runs carry stable readiness reason codes for source command invalid/runtime failure while preserving operator-readable detail; profile readiness now also blocks source start or final delivery with `missing_profile`, `profile_expired`, `permission_denied`, or `refresh_required` before any external effect. Connector-local `source-clear-blocked`, `source-request-restart`, and `source-reset-cursor --accept-duplicate-risk` record operator source actions without creating kernel facts or fabricating ready state. `cmd/genesis-feishu-source-adapter` owns Feishu event command construction and raw Feishu payload parsing, requires an explicit `--profile`, and emits typed source frames instead of giving runtime raw payloads or Feishu command details. Malformed source frames and malformed Feishu payloads create redacted `SourceFailureRecord` entries before kernel submission. `FileInboundStore`, `FileSourceFailureStore`, `FileSourceLifecycleStore`, and `FileOutboxStore` serialize file-backed load-modify-write operations across process-local instances during smoke use. `genesis-console inspect --source-lifecycle-state ...` projects connector-local source runs, attempts, cursors, verification evidence, and operator actions without kernel mutation. `genesis-ingress feishu-probe --source-command ... --delivery-command ... --profile genesis` validates the source adapter process, profile readiness posture, and connector-command final-delivery surface without starting the listener, sending a message, or calling kernel. The opt-in `--deliver-final` path uses connector outbox delivery rather than kernel Feishu logic.
- Verification: Remaining verification must prove runtime source code contains no Feishu event consume argv, malformed source frames produce redacted source failures and no `ExternalEvent`, verified source status is only emitted after durable and inspectable source verification exists, verified evidence is itself `verified` and adapter-bound, cursor progress advances only after durable event acceptance, repeated Feishu events dedupe before kernel turn submission and reply delivery, inbound retry exhaustion only affects connector request state, external Feishu identity does not set kernel authority, source failure records use configured source identity instead of untrusted frame metadata, real profile probes classify missing/expired/denied/refresh-needed profile posture before external source or send actions, recoverable source process failures retry without duplicating kernel facts, blocked source readiness failures do not retry, handler/kernel submission errors do not restart source adapters, and production listener supervision recovers from transient listener failures without creating a kernel Feishu owner.
- Reference alignment: Aligned with Reasonix ACP keeping protocol/session handling outside the controller and Codex app-server keeping client transport ids outside core turn truth.

### APP-CONNECTOR-OPERATOR-CONSOLE-20260623 - P2 - Operator console inspection projection

- Status: open.
- Requirement: `docs/applications/application-connector-runtime-requirement.md`.
- Design: `docs/applications/application-connector-runtime-design.md`.
- Kernel/owner pressure: connector-owned inspection and recovery projection
  against kernel session projection reads without console-owned kernel truth.
- Gap: `genesis-console inspect` now provides the first read-only operator view for connector inbound records, outbox items, outbox delivery summaries, source failure records, delivery receipts, focused connector/status/session filters, and kernel session projections fetched through HTTP. `genesis-console requeue-outbox` can explicitly requeue dead-lettered connector items while preserving receipt history. `genesis-console resolve-outbox` can explicitly reconcile `recovery_required` partial/ambiguous outcomes to `sent` or `dead_lettered` with an operator receipt, without adapter execution or kernel mutation. Reconciliation probe production semantics are now documented as connector-local read-only evidence before terminal recovery. Remaining implementation gap is a concrete connector-specific probe surface and evidence store once exact external lookup handles exist.
- Next slice: Future recovery commands must continue to mutate only connector-owned state, preserve receipt history, and never rewrite kernel facts or fabricate kernel projections. A connector-specific reconciliation probe must require exact lookup handles, produce `ReconciliationEvidence`, and avoid resend/fuzzy search before a recovery-required item is resolved.
- Evidence: Application Connector Runtime Phase D has a minimal `genesis-console inspect` implementation, focused `--connector`, `--inbound-status`, `--outbox-status`, and `--kernel-session-id` filters, outbox delivery summary projection with last receipt and recommended operator action, source failure inspection from `FileSourceFailureStore`, connector-local `requeue-outbox` for dead-lettered items, and connector-local `resolve-outbox` for recovery-required items. The recovery paths record operator receipts and clear connector scheduling/lease fields without adapter execution or kernel calls.
- Verification: Remaining verification must prove future connector-specific reconciliation probes preserve kernel fact isolation, require exact external lookup handles, do not resend ambiguous external effects, do not use fuzzy content search, and only feed evidence-backed connector-owned terminal recovery decisions into connector state.
- Reference alignment: Aligned with Reasonix `serve/wire.go` projecting internal events to wire shape without becoming event truth owner.
