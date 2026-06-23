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

### APP-CONNECTOR-DELIVERY-STATE-MACHINE-20260623 - P2 - Add retry scheduling, dead-letter, and partial-success recovery

- Status: open.
- Requirement: `docs/applications/application-connector-runtime-requirement.md`.
- Design: `docs/applications/application-connector-runtime-design.md`.
- Gap: The first outbox owner records queued, sent, retrying, failed, and duplicate-suppressed receipts, but it does not yet implement retry scheduling, delivery leases/claims, dead-letter transitions, partial-success recovery, or rate-limit backoff.
- Next slice: Add an explicit delivery state machine with eligible retry selection, bounded attempt policy, dead-letter receipt, and partial-success recovery hooks. Keep these states connector-local and do not rewrite kernel turn facts.
- Evidence: `internal/applications/connector_runtime` currently records the adapter-provided status and suppresses terminal duplicates, but does not schedule retries or transition failed/retrying items to dead-letter.
- Verification: Retrying item becomes eligible only after `next_attempt_at`; exhausted attempts produce one dead-letter receipt; partial success records recoverable receipt state; duplicate execution of terminal sent/dead-letter items does not call the adapter.
- Reference alignment: Codex and Reasonix show protocol boundary discipline, but Genesis needs connector-specific recovery because external delivery has side effects outside kernel truth.

### APP-CONNECTOR-FEISHU-LISTENER-20260623 - P2 - Feishu inbound listener and adapter retry hardening

- Status: open.
- Requirement: `docs/applications/application-connector-runtime-requirement.md`.
- Design: `docs/applications/application-connector-runtime-design.md`.
- Gap: Phase A only proves one-shot Feishu-like inbound envelope submission. It does not run a durable Feishu event listener, verify callback signatures, refresh adapter tokens, or apply inbound retry/backoff policy.
- Next slice: After the outbox/receipt owner exists, add a Feishu listener/poller that emits `ExternalEvent`/`RequestContext` and keeps signature/token/retry state in connector-local storage.
- Evidence: Application Connector Runtime Phase C explicitly covers Feishu inbound listener/poller hardening.
- Verification: A repeated Feishu event must dedupe before kernel turn submission; inbound retry exhaustion must only affect connector request state; external Feishu identity must not set kernel authority.
- Reference alignment: Aligned with Reasonix ACP keeping protocol/session handling outside the controller and Codex app-server keeping client transport ids outside core turn truth.

### APP-CONNECTOR-OPERATOR-CONSOLE-20260623 - P2 - Operator console inspection projection

- Status: open.
- Requirement: `docs/applications/application-connector-runtime-requirement.md`.
- Design: `docs/applications/application-connector-runtime-design.md`.
- Gap: Current code does not provide operator console views for connector state or kernel projections. It only creates the inbound relay needed to submit messages.
- Next slice: Add console inspection commands that read connector state and kernel projections without interpreting raw events as application truth.
- Evidence: Application Connector Runtime Phase D covers operator console inspection.
- Verification: Console inspection must fetch kernel projections through HTTP and connector state through application store APIs; it must not import kernel internals or reconstruct provider context locally.
- Reference alignment: Aligned with Reasonix `serve/wire.go` projecting internal events to wire shape without becoming event truth owner.
