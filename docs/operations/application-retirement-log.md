# Application Retirement Log

This file records retired user-space application issues. Active application
issues remain in `docs/operations/application-issues.md`.

## Retired Issues

### APP-CONNECTOR-INBOUND-CONTEXT-UNIFICATION-20260623 - Unify inbound message slice with connector request context

- Retired in: connector inbound unification implementation change set.
- Requirement: `docs/applications/application-connector-runtime-requirement.md`.
- Design: `docs/applications/application-connector-runtime-design.md`.
- Fix summary: Added connector-owned `ExternalEvent`, `RequestContext`, `ApplicationSessionMapping`, inbound submission records, file-backed inbound dedupe store, kernel turn client, and `ProcessExternalEvent` in `internal/applications/connector_runtime`.
- Retirement evidence: `cmd/genesis-ingress` now submits `ExternalEvent` through connector runtime. The narrower `internal/applications/message_ingress` package, its tests, and command references were removed rather than kept as a compatibility layer.
- Boundary evidence: Connector inbound code does not import `genesis/internal/kernel`, does not build provider context, does not expose raw external ids as public system ids, and does not persist kernel final text as connector state.
- Verification: `go test ./internal/applications/connector_runtime -count=1`; `go test ./cmd/genesis-ingress -count=1`; full verification recorded in the fixing commit.
- Residual risk: Real Feishu listener/poller, signature verification, delivery state machine, resource intake, and operator console remain active or future issues.

### APP-CONNECTOR-OUTBOX-RECEIPT-20260623 - Add connector outbox/action/receipt owner

- Retired in: connector outbox/action/receipt implementation change set.
- Requirement: `docs/applications/application-connector-runtime-requirement.md`.
- Design: `docs/applications/application-connector-runtime-design.md`.
- Fix summary: Added the minimal `internal/applications/connector_runtime` outbox/action/receipt owner with `AppCommand`, `ConnectorOutboxItem`, `ConnectorAction`, `DeliveryReceipt`, file-backed outbox/receipt storage, idempotent command enqueue, terminal duplicate suppression, connector action execution, console adapter, and Feishu adapter behind a runner interface.
- Boundary evidence: Connector runtime production code does not import `genesis/internal/kernel`. External credentials are adapter configuration, not action payload fields. App command metadata and external-thread metadata are not copied into connector action payloads. Connector action failure records connector receipt/outbox state and does not write kernel facts.
- Verification: `go test ./internal/applications/connector_runtime -count=1`; full verification recorded in the fixing commit.
- Residual risk: This retires only the minimal owner and first delivery outcome contract. Retry scheduling, dead-letter transitions, and partial-success recovery remain active issues.
