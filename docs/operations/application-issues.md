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
  when such a log is needed.
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
- Gap: The smoke path now has deterministic adapter NDJSON ingestion, a real `lark-cli event consume im.message.receive_v1` source driver, and an opt-in `--deliver-final` path that turns kernel final text into one connector-owned send-message outbox item. The real source driver still marks source validation as `unchecked` because webhook/signature/source verification is not yet a durable connector fact. Remaining production gap is long-running listener supervision, connector-local retry/backoff around source failures, durable dead-letter records for malformed source events, token/profile refresh, an operator-run installed-adapter capability probe, and moving the hardcoded `lark-cli event consume` command shape behind connector driver configuration or an external adapter process.
- Next slice: After the bounded live smoke, add connector-local source failure records and retry/backoff if the live source shows recoverable startup or runtime failures. Product smoke must use the Genesis profile (`--profile genesis`); Codex developer-originated test messages may still use the Codex profile as the sender. The probe should prefer a direct official package binary such as `@larksuite/cli/bin/lark-cli.exe`; if local `lark-cli` resolves to an npm `.cmd`/`.ps1` shim or extensionless shell script and no direct binary is configured, production delivery must use a real binary or `connector_command` external adapter process instead of `command_template`. The current in-code Feishu event command is a smoke slice, not the final connector protocol.
- Evidence: `genesis-ingress feishu-listen --stdin-jsonl --profile ...` covers automated smoke and dedupe. `FeishuEventSourceConfig` now constructs an explicit-profile `lark-cli event consume im.message.receive_v1 --as bot` command, rejects non-direct binaries, redacts stderr diagnostics, maps flattened Feishu message events into connector-owned `ExternalEvent` values before `/turn`, and can ignore configured sender ids before kernel submission to avoid bot self-reply loops. The opt-in `--deliver-final` path uses connector outbox delivery rather than kernel Feishu logic.
- Verification: Remaining verification must prove malformed/unauthenticated real Feishu source events fail before kernel submission and become connector-local source records, verified source status is only emitted after durable source verification exists, repeated Feishu events dedupe before kernel turn submission and reply delivery, inbound retry exhaustion only affects connector request state, external Feishu identity does not set kernel authority, missing profile/credential fails before external source or send actions, and source retry/backoff/supervision recovers from transient listener failures.
- Reference alignment: Aligned with Reasonix ACP keeping protocol/session handling outside the controller and Codex app-server keeping client transport ids outside core turn truth.

### APP-CONNECTOR-OPERATOR-CONSOLE-20260623 - P2 - Operator console inspection projection

- Status: open.
- Requirement: `docs/applications/application-connector-runtime-requirement.md`.
- Design: `docs/applications/application-connector-runtime-design.md`.
- Kernel/owner pressure: connector-owned inspection and recovery projection
  against kernel session projection reads without console-owned kernel truth.
- Gap: `genesis-console inspect` now provides the first read-only operator view for connector inbound records, outbox items, delivery receipts, focused connector/status/session filters, and kernel session projections fetched through HTTP. `genesis-console requeue-outbox` can explicitly requeue dead-lettered connector items while preserving receipt history. It intentionally does not requeue `recovery_required` partial/ambiguous outcomes because those require connector-specific reconciliation to avoid duplicate external sends. Remaining production gap is source-failure/dead-letter inspection and recovery-required reconciliation commands.
- Next slice: Expose source dead-letter records once the Feishu listener persists them. Future recovery commands must continue to mutate only connector-owned state, preserve receipt history, and never rewrite kernel facts or fabricate kernel projections. Recovery-required items need explicit reconcile/supersede semantics rather than requeue.
- Evidence: Application Connector Runtime Phase D has a minimal `genesis-console inspect` implementation, focused `--connector`, `--inbound-status`, `--outbox-status`, and `--kernel-session-id` filters, and a connector-local `requeue-outbox` command for dead-lettered items. The requeue path records an operator receipt and clears connector scheduling/lease fields without adapter execution or kernel calls.
- Verification: Remaining verification must prove future source-failure views expose source-failed state without interpreting raw kernel events, and future reconciliation commands preserve kernel fact isolation without resending ambiguous external effects.
- Reference alignment: Aligned with Reasonix `serve/wire.go` projecting internal events to wire shape without becoming event truth owner.
