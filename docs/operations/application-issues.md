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

### APP-CONNECTOR-FEISHU-LISTENER-20260623 - P2 - Feishu inbound listener and adapter retry hardening

- Status: open.
- Requirement: `docs/applications/application-connector-runtime-requirement.md`.
- Design: `docs/applications/application-connector-runtime-design.md`.
- Kernel/owner pressure: connector source verification, inbound dedupe, session
  mapping, turn submission, and connector outbox delivery without kernel Feishu
  ownership.
- Gap: The smoke path now has deterministic adapter NDJSON ingestion, a real `lark-cli event consume im.message.receive_v1` source driver, durable connector-local source failure records for malformed Feishu source events, an operator-run `genesis-ingress feishu-probe` installed-adapter readiness report, and an opt-in `--deliver-final` path that turns kernel final text into one connector-owned send-message outbox item. The real source driver still marks source validation as `unchecked` because webhook/signature/source verification is not yet a durable connector fact. Remaining production gap is long-running listener supervision, connector-local retry/backoff around recoverable source failures, token/profile refresh, and moving the hardcoded `lark-cli event consume` command shape behind connector driver configuration or an external adapter process.
- Next slice: Add connector-local retry/backoff if the live source shows recoverable startup or runtime failures. Product smoke must use the Genesis profile (`--profile genesis`); Codex developer-originated test messages may still use the Codex profile as the sender. If local `lark-cli` resolves to an npm `.cmd`/`.ps1` shim or extensionless shell script and no direct binary is configured, production delivery must use a real binary or `connector_command` external adapter process instead of `command_template`. The current in-code Feishu event command is a smoke slice, not the final connector protocol.
- Evidence: `genesis-ingress feishu-listen --stdin-jsonl --profile ...` covers automated smoke and dedupe. `FeishuEventSourceConfig` now constructs an explicit-profile `lark-cli event consume im.message.receive_v1 --as bot` command, rejects non-direct binaries, redacts stderr diagnostics, maps flattened Feishu message events into connector-owned `ExternalEvent` values before `/turn`, records malformed source events into `FileSourceFailureStore` before kernel submission, and can ignore configured sender ids before kernel submission to avoid bot self-reply loops. `genesis-ingress feishu-probe --profile ... --lark-cli ...` validates the event-source and final-delivery command surfaces without starting the listener, sending a message, or calling kernel. The opt-in `--deliver-final` path uses connector outbox delivery rather than kernel Feishu logic.
- Verification: Remaining verification must prove unauthenticated real Feishu source events fail before kernel submission and become connector-local source records, verified source status is only emitted after durable source verification exists, repeated Feishu events dedupe before kernel turn submission and reply delivery, inbound retry exhaustion only affects connector request state, external Feishu identity does not set kernel authority, missing profile/credential fails before external source or send actions, and source retry/backoff/supervision recovers from transient listener failures.
- Reference alignment: Aligned with Reasonix ACP keeping protocol/session handling outside the controller and Codex app-server keeping client transport ids outside core turn truth.

### APP-CONNECTOR-OPERATOR-CONSOLE-20260623 - P2 - Operator console inspection projection

- Status: open.
- Requirement: `docs/applications/application-connector-runtime-requirement.md`.
- Design: `docs/applications/application-connector-runtime-design.md`.
- Kernel/owner pressure: connector-owned inspection and recovery projection
  against kernel session projection reads without console-owned kernel truth.
- Gap: `genesis-console inspect` now provides the first read-only operator view for connector inbound records, outbox items, source failure records, delivery receipts, focused connector/status/session filters, and kernel session projections fetched through HTTP. `genesis-console requeue-outbox` can explicitly requeue dead-lettered connector items while preserving receipt history. `genesis-console resolve-outbox` can explicitly reconcile `recovery_required` partial/ambiguous outcomes to `sent` or `dead_lettered` with an operator receipt, without adapter execution or kernel mutation. Remaining production gap is richer delivery dead-letter inspection and connector-specific reconciliation probes that can query external systems before choosing the terminal recovery outcome.
- Next slice: Future recovery commands must continue to mutate only connector-owned state, preserve receipt history, and never rewrite kernel facts or fabricate kernel projections. Connector-specific reconciliation probes must produce connector-local evidence before a recovery-required item is resolved.
- Evidence: Application Connector Runtime Phase D has a minimal `genesis-console inspect` implementation, focused `--connector`, `--inbound-status`, `--outbox-status`, and `--kernel-session-id` filters, source failure inspection from `FileSourceFailureStore`, connector-local `requeue-outbox` for dead-lettered items, and connector-local `resolve-outbox` for recovery-required items. The recovery paths record operator receipts and clear connector scheduling/lease fields without adapter execution or kernel calls.
- Verification: Remaining verification must prove future connector-specific reconciliation probes preserve kernel fact isolation, do not resend ambiguous external effects, and only feed explicit operator-owned terminal recovery decisions into connector state.
- Reference alignment: Aligned with Reasonix `serve/wire.go` projecting internal events to wire shape without becoming event truth owner.
