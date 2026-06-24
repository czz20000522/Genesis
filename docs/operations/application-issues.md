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
- Gap: The smoke path now has deterministic ExternalEvent NDJSON ingestion, a `source_command` typed streaming boundary for inbound source adapters, a Feishu source adapter command that owns `lark-cli event consume` and raw Feishu payload parsing, durable connector-local source failure records with redacted diagnostics, connector-local `SourceRun` / `SourceAttempt` / `SourceCursor` / `SourceVerificationEvidence` state, bounded generic retry/backoff for recoverable `source_command` process failures, connector-local operator lifecycle controls, an operator-run `genesis-ingress feishu-probe` source-command readiness report, and an opt-in `--deliver-final` path that turns kernel final text into one connector-owned send-message outbox item. Real source events still default to `unchecked` because source readiness does not imply event verification. Remaining production gap is full credential/profile probe and refresh integration plus production-grade source recovery beyond bounded retry/backoff.
- Next slice: Add a connector-specific credential/profile readiness probe that can classify `missing_profile`, `profile_expired`, `permission_denied`, and `refresh_required` before source start or final delivery. Product smoke commands must continue to pass the Genesis profile (`--profile genesis`) into the Feishu source adapter; Codex developer-originated test messages may still use the Codex profile as the sender. Inbound source intake must continue through `source_command`, not command-template configuration and not runtime-owned Feishu argv. Connector-specific reconciliation probes should wait until exact outbound action refs, idempotency keys, or external receipt refs exist; reconciliation remains outbound recovery work.
- Evidence: `genesis-ingress feishu-listen --stdin-jsonl ...` covers automated smoke and dedupe without a source process, with synthetic source validation remaining `unchecked` unless evidence exists. Non-stdin source intake now starts a configured `SourceCommandAdapter` process and consumes typed `source.ready`, `source.event`, `source.cursor`, `source.failed`, and `source.stopped` frames through the source command intake loop, which retries only recoverable runtime failures and does not retry blocked readiness/configuration failures. Verified source events require source/connector/adapter/event-bound evidence with an approved evidence kind before they are handled or recorded. Blocked/degraded source runs carry stable readiness reason codes for source command invalid/runtime failure while preserving operator-readable detail. Connector-local `source-clear-blocked`, `source-request-restart`, and `source-reset-cursor --accept-duplicate-risk` record operator source actions without creating kernel facts or fabricating ready state. `cmd/genesis-feishu-source-adapter` owns Feishu event command construction and raw Feishu payload parsing, requires an explicit `--profile`, and emits typed source frames instead of giving runtime raw payloads or Feishu command details. Malformed source frames and malformed Feishu payloads create redacted `SourceFailureRecord` entries before kernel submission. `FileInboundStore`, `FileSourceFailureStore`, `FileSourceLifecycleStore`, and `FileOutboxStore` serialize file-backed load-modify-write operations across process-local instances during smoke use. `genesis-console inspect --source-lifecycle-state ...` projects connector-local source runs, attempts, cursors, verification evidence, and operator actions without kernel mutation. `genesis-ingress feishu-probe --source-command ... --profile genesis --lark-cli ...` validates the source adapter process and final-delivery surfaces without starting the listener, sending a message, or calling kernel. The opt-in `--deliver-final` path uses connector outbox delivery rather than kernel Feishu logic.
- Verification: Remaining verification must prove runtime source code contains no Feishu event consume argv, malformed source frames produce redacted source failures and no `ExternalEvent`, verified source status is only emitted after durable and inspectable source verification exists, verified evidence is itself `verified` and adapter-bound, cursor progress advances only after durable event acceptance, repeated Feishu events dedupe before kernel turn submission and reply delivery, inbound retry exhaustion only affects connector request state, external Feishu identity does not set kernel authority, source failure records use configured source identity instead of untrusted frame metadata, missing profile/credential fails before external source or send actions, recoverable source process failures retry without duplicating kernel facts, blocked source readiness failures do not retry, handler/kernel submission errors do not restart source adapters, and production listener supervision recovers from transient listener failures without creating a kernel Feishu owner.
- Reference alignment: Aligned with Reasonix ACP keeping protocol/session handling outside the controller and Codex app-server keeping client transport ids outside core turn truth.

### APP-CONNECTOR-OPERATOR-CONSOLE-20260623 - P2 - Operator console inspection projection

- Status: open.
- Requirement: `docs/applications/application-connector-runtime-requirement.md`.
- Design: `docs/applications/application-connector-runtime-design.md`.
- Kernel/owner pressure: connector-owned inspection and recovery projection
  against kernel session projection reads without console-owned kernel truth.
- Gap: `genesis-console inspect` now provides the first read-only operator view for connector inbound records, outbox items, outbox delivery summaries, source failure records, delivery receipts, focused connector/status/session filters, and kernel session projections fetched through HTTP. `genesis-console requeue-outbox` can explicitly requeue dead-lettered connector items while preserving receipt history. `genesis-console resolve-outbox` can explicitly reconcile `recovery_required` partial/ambiguous outcomes to `sent` or `dead_lettered` with an operator receipt, without adapter execution or kernel mutation. Remaining production gap is connector-specific reconciliation probes that can query external systems before choosing the terminal recovery outcome.
- Next slice: Future recovery commands must continue to mutate only connector-owned state, preserve receipt history, and never rewrite kernel facts or fabricate kernel projections. Connector-specific reconciliation probes must produce connector-local evidence before a recovery-required item is resolved.
- Evidence: Application Connector Runtime Phase D has a minimal `genesis-console inspect` implementation, focused `--connector`, `--inbound-status`, `--outbox-status`, and `--kernel-session-id` filters, outbox delivery summary projection with last receipt and recommended operator action, source failure inspection from `FileSourceFailureStore`, connector-local `requeue-outbox` for dead-lettered items, and connector-local `resolve-outbox` for recovery-required items. The recovery paths record operator receipts and clear connector scheduling/lease fields without adapter execution or kernel calls.
- Verification: Remaining verification must prove future connector-specific reconciliation probes preserve kernel fact isolation, do not resend ambiguous external effects, and only feed explicit operator-owned terminal recovery decisions into connector state.
- Reference alignment: Aligned with Reasonix `serve/wire.go` projecting internal events to wire shape without becoming event truth owner.

### APP-CONNECTOR-DRIVER-MIGRATION-20260625 - P2 - Feishu final delivery still defaults to the transitional command template driver

- Status: open.
- Requirement: `docs/applications/application-connector-runtime-requirement.md`.
- Design: `docs/applications/application-connector-runtime-design.md`.
- Kernel/owner pressure: connector-owned outbound delivery, connector action/result validation, and delivery receipts without kernel Feishu ownership or runtime-owned Feishu CLI protocol drift.
- Gap: Inbound source intake now uses the typed `source_command` adapter boundary, and `ConnectorCommandAdapter` already provides the long-lived typed outbound action/result boundary. However the Feishu final delivery smoke path and readiness probe still construct `NewFeishuSendMessageCommandTemplateDriver`, and that driver hardcodes `lark-cli im +messages-send` argv in connector runtime code. This is acceptable only as a transitional smoke convenience; it should not remain the default production delivery path because Feishu CLI syntax, profile mechanics, vendor response parsing, and command drift belong behind an external connector adapter process.
- Next slice: Add a production-oriented Feishu final delivery path that configures `ConnectorCommandAdapter` for `send_message` delivery and probe readiness. The runtime should send typed `ConnectorAction` JSON and validate typed `ConnectorActionResult` JSON, while the external Feishu adapter process owns `lark-cli`, SDK, HTTP, profile, vendor response parsing, and vendor error normalization. Keep `command_template` only as an explicitly named local smoke fallback with a retirement target, or remove it if no active smoke consumer remains.
- Evidence: `cmd/genesis-ingress/main.go` still calls `configureFeishuFinalDelivery`, which installs `connectorruntime.NewFeishuSendMessageCommandTemplateDriver`. `internal/applications/connector_runtime/feishu_delivery_driver.go` hardcodes the Feishu `send_message` argv. `internal/applications/connector_runtime/feishu_probe.go` probes final delivery by rendering that same command template. The approved design says `connector_command` is the long-lived adapter boundary and `command_template` is only a transitional driver for early CLI-backed smoke tests. `internal/applications/connector_runtime/connector_command_adapter.go` and its tests already prove the typed action/result path exists and can record delivery receipts.
- Verification: Add tests proving Feishu final delivery can be configured through a `connector_command` adapter, the adapter receives typed `ConnectorAction` without Feishu argv leaking through runtime state, unsafe/malformed adapter results fail closed as connector-local delivery failures, readiness probe reports connector-command status without rendering `lark-cli im +messages-send`, and any remaining `command_template` path is explicitly marked as smoke/transitional rather than production default. Re-run `go test ./internal/applications/connector_runtime ./cmd/genesis-ingress ./cmd/genesis-console ./cmd/genesis-feishu-source-adapter -count=1`, `go test ./... -count=1`, `go build ./...`, and `git diff --check`.
- Reference alignment: Aligned with Reasonix ACP/serve adapter boundaries, where protocol projection maps typed internal events to wire shape without owning core execution semantics, and with Codex app-server connector/app surfaces, where external connector availability and typed notifications are separated from core turn truth. The active drift risk is leaving vendor command syntax inside the connector runtime after the stable `ConnectorAction` / `ConnectorActionResult` boundary already exists.
