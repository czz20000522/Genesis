# Application Retirement Log

This file records retired user-space application issues. Active application
issues remain in `docs/operations/application-issues.md`.

## Retired Issues

### APP-CONNECTOR-OUTBOX-RECEIPT-20260623 - Add connector outbox/action/receipt owner

- Retired in: connector outbox/action/receipt implementation change set.
- Requirement: `docs/applications/application-connector-runtime-requirement.md`.
- Design: `docs/applications/application-connector-runtime-design.md`.
- Fix summary: Added the minimal `internal/applications/connector_runtime` outbox/action/receipt owner with `AppCommand`, `ConnectorOutboxItem`, `ConnectorAction`, `DeliveryReceipt`, file-backed outbox/receipt storage, idempotent command enqueue, terminal duplicate suppression, connector action execution, console adapter, and Feishu adapter behind a runner interface.
- Boundary evidence: Connector runtime production code does not import `genesis/internal/kernel`. External credentials are adapter configuration, not action payload fields. App command metadata and external-thread metadata are not copied into connector action payloads. Connector action failure records connector receipt/outbox state and does not write kernel facts.
- Verification: `go test ./internal/applications/connector_runtime -count=1`; full verification recorded in the fixing commit.
- Residual risk: This retires only the minimal owner and first delivery outcome contract. Retry scheduling, dead-letter transitions, partial-success recovery, and inbound `ExternalEvent`/`RequestContext` unification remain active issues.
