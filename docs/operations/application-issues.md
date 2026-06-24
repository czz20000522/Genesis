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
- Gap: Phase A only proves one-shot Feishu-like inbound envelope submission. It does not run a durable Feishu event listener, verify callback signatures, require explicit `lark-cli --profile ...` configuration, refresh adapter tokens, apply inbound retry/backoff policy, or expose an operator-run lark-cli capability probe.
- Next slice: After the outbox/receipt owner exists, add a Feishu listener/poller that emits `ExternalEvent`/`RequestContext` and keeps signature/token/retry state in connector-local storage. The Feishu connector must load an explicit profile and action driver configuration, then run an operator dry-run/probe contract for the installed `lark-cli` before live sends. The probe should prefer a direct official package binary such as `@larksuite/cli/bin/lark-cli.exe`; if local `lark-cli` resolves to an npm `.cmd`/`.ps1` shim or extensionless shell script and no direct binary is configured, production delivery must use a real binary or `connector_command` external adapter process instead of `command_template`. A command shape change must be handled by connector driver configuration or an external adapter process, not connector runtime code.
- Evidence: Application Connector Runtime Phase C explicitly covers Feishu inbound listener/poller hardening.
- Verification: A repeated Feishu event must dedupe before kernel turn submission; inbound retry exhaustion must only affect connector request state; external Feishu identity must not set kernel authority; missing profile must fail before any external send.
- Reference alignment: Aligned with Reasonix ACP keeping protocol/session handling outside the controller and Codex app-server keeping client transport ids outside core turn truth.

### APP-CONNECTOR-OPERATOR-CONSOLE-20260623 - P2 - Operator console inspection projection

- Status: open.
- Requirement: `docs/applications/application-connector-runtime-requirement.md`.
- Design: `docs/applications/application-connector-runtime-design.md`.
- Gap: Current code does not provide operator console views for connector state, delivery attempt history, recovery-required items, dead-lettered items, or kernel projections. It also does not yet expose explicit connector recovery commands for delivery state machine Phase E.
- Next slice: Add console inspection commands that read connector state and kernel projections without interpreting raw events as application truth. Then add explicit connector recovery commands for recovery-required or dead-lettered delivery items.
- Evidence: Application Connector Runtime Phase D covers operator console inspection; Connector Delivery State Machine Phase E covers operator projection and recovery commands.
- Verification: Console inspection must fetch kernel projections through HTTP and connector state through application store APIs; it must not import kernel internals or reconstruct provider context locally. Recovery commands must mutate only connector-owned state and must not rewrite kernel facts.
- Reference alignment: Aligned with Reasonix `serve/wire.go` projecting internal events to wire shape without becoming event truth owner.
