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
- Issues should stay small: requirement, design, gap, next slice, evidence,
  verification, and reference alignment.

## Active Issues

### APP-CONNECTOR-OUTBOX-RECEIPT-20260623 - P1 - Add connector outbox/action/receipt owner

- Status: open.
- Requirement: `docs/applications/application-connector-runtime-requirement.md`.
- Design: `docs/applications/application-connector-runtime-design.md`.
- Gap: Current code only implements the Phase A inbound slice in `internal/applications/message_ingress`. It has no `AppCommand`, `ConnectorOutbox`, `ConnectorAction`, or `DeliveryReceipt` owner, so production outbound delivery would still be underspecified if implemented next.
- Next slice: Add the minimal connector runtime package or owner module that defines app commands, outbox items, connector actions, receipts, idempotency, and failed-delivery isolation. Console and Feishu should be adapters of the same primitives.
- Evidence: `cmd/genesis-ingress` and `internal/applications/message_ingress` submit inbound messages to `/turn` and intentionally contain no outbound sender. The approved connector requirement now states outbound production must flow through connector outbox/receipt.
- Verification: App command enqueue produces one outbox item; duplicate app command suppresses duplicate action; connector action failure writes receipt/retry state without changing kernel facts; connector package does not import `internal/kernel` or expose external credentials to model-visible fields.
- Reference alignment: Codex and Reasonix keep protocol adapters outside core controller truth; Genesis extends that boundary with a connector outbox/receipt owner for production external delivery.

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
