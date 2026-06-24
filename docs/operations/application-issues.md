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
- Gap: The smoke path now has deterministic ExternalEvent NDJSON ingestion, a `source_command` typed streaming boundary for inbound source adapters, a Feishu source adapter command that owns `lark-cli event consume` and raw Feishu payload parsing, durable connector-local source failure records with redacted diagnostics, connector-local `SourceRun` / `SourceAttempt` / `SourceCursor` / `SourceVerificationEvidence` state, bounded generic retry/backoff for recoverable `source_command` process failures, an operator-run `genesis-ingress feishu-probe` source-command readiness report, and an opt-in `--deliver-final` path that turns kernel final text into one connector-owned send-message outbox item. Real source events still default to `unchecked` because source readiness does not imply event verification. Remaining production gap is credential/profile readiness posture, operator lifecycle controls, and production-grade source recovery beyond bounded retry/backoff.
- Next slice: Product smoke must use the Genesis profile (`--profile genesis`) inside the Feishu source adapter command; Codex developer-originated test messages may still use the Codex profile as the sender. Inbound source intake must continue through `source_command`, not command-template configuration and not runtime-owned Feishu argv. The next implementation cut should add generic credential/profile readiness reason codes and operator lifecycle controls before adding connector-specific reconciliation probes. Reconciliation remains outbound recovery work and requires exact action refs, idempotency keys, or external receipt refs.
- Evidence: `genesis-ingress feishu-listen --stdin-jsonl ...` covers automated smoke and dedupe without a source process. Non-stdin source intake now starts a configured `SourceCommandAdapter` process and consumes typed `source.ready`, `source.event`, `source.cursor`, `source.failed`, and `source.stopped` frames through the source command intake loop, which retries only recoverable runtime failures and does not retry blocked readiness/configuration failures. Verified source events require source/connector/adapter/event-bound evidence with an approved evidence kind before they are handled or recorded. `cmd/genesis-feishu-source-adapter` owns Feishu event command construction and raw Feishu payload parsing, emitting typed source frames instead of giving runtime raw payloads or Feishu command details. Malformed source frames and malformed Feishu payloads create redacted `SourceFailureRecord` entries before kernel submission. `FileInboundStore`, `FileSourceFailureStore`, `FileSourceLifecycleStore`, and `FileOutboxStore` serialize file-backed load-modify-write operations across process-local instances during smoke use. `genesis-console inspect --source-lifecycle-state ...` projects connector-local source runs, attempts, cursors, and verification evidence without kernel mutation. `genesis-ingress feishu-probe --source-command ... --profile ... --lark-cli ...` validates the source adapter process and final-delivery surfaces without starting the listener, sending a message, or calling kernel. The opt-in `--deliver-final` path uses connector outbox delivery rather than kernel Feishu logic.
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
