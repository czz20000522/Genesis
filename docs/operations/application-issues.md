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
- Issues should stay small: requirement, design, gap, next slice, evidence,
  verification, and reference alignment.

## Active Issues

### APP-CONNECTOR-FEISHU-LISTENER-20260623 - P2 - Feishu inbound listener and adapter retry hardening

- Status: open.
- Requirement: `docs/applications/application-connector-runtime-requirement.md`.
- Design: `docs/applications/application-connector-runtime-design.md`.
- Gap: The smoke path now has `genesis-ingress feishu-listen --stdin-jsonl --profile ...`, which consumes Feishu adapter NDJSON as connector-owned `ExternalEvent` records and dedupes before kernel turn submission. Remaining production gap is the real Feishu event source wrapper: webhook or `lark-cli event` process supervision, callback signature or source validation, token/profile refresh, connector-local retry/backoff, dead-lettering for malformed source events, and an operator-run installed-adapter capability probe.
- Next slice: Add the real Feishu event source driver around the existing `feishu-listen` stream contract. The driver must keep Feishu protocol details outside kernel and connector runtime truth, require an explicit profile or credential ref, and report source/probe failures as connector-local state. The probe should prefer a direct official package binary such as `@larksuite/cli/bin/lark-cli.exe`; if local `lark-cli` resolves to an npm `.cmd`/`.ps1` shim or extensionless shell script and no direct binary is configured, production delivery must use a real binary or `connector_command` external adapter process instead of `command_template`. A command shape change must be handled by connector driver configuration or an external adapter process, not connector runtime code.
- Evidence: `genesis-ingress feishu-listen --stdin-jsonl --profile ...` covers the first automated smoke route and proves repeated Feishu events dedupe before `/turn`. Application Connector Runtime Phase C still covers the production listener/poller hardening.
- Verification: Remaining verification must prove malformed/unauthenticated real Feishu source events fail before kernel submission, repeated Feishu events dedupe before kernel turn submission, inbound retry exhaustion only affects connector request state, external Feishu identity does not set kernel authority, and missing profile/credential fails before external source or send actions.
- Reference alignment: Aligned with Reasonix ACP keeping protocol/session handling outside the controller and Codex app-server keeping client transport ids outside core turn truth.

### APP-CONNECTOR-OPERATOR-CONSOLE-20260623 - P2 - Operator console inspection projection

- Status: open.
- Requirement: `docs/applications/application-connector-runtime-requirement.md`.
- Design: `docs/applications/application-connector-runtime-design.md`.
- Gap: `genesis-console inspect` now provides the first read-only operator view for connector inbound records, outbox items, delivery receipts, and kernel session projections fetched through HTTP. Remaining production gap is recovery tooling for recovery-required and dead-lettered connector state, plus richer filtered views for live operation.
- Next slice: Add explicit connector recovery commands for recovery-required or dead-lettered delivery items. Recovery commands must mutate only connector-owned state, preserve receipt history, and never rewrite kernel facts or fabricate kernel projections.
- Evidence: Application Connector Runtime Phase D has a minimal `genesis-console inspect` implementation; Connector Delivery State Machine Phase E still covers operator recovery commands.
- Verification: Remaining verification must prove recovery commands mutate only connector-owned state, cannot rewrite kernel facts, preserve delivery receipt history, and expose recovery-required/dead-lettered state without interpreting raw kernel events.
- Reference alignment: Aligned with Reasonix `serve/wire.go` projecting internal events to wire shape without becoming event truth owner.
