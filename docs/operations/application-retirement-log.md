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

### APP-CONNECTOR-DELIVERY-STATE-MACHINE-20260623 - Add retry scheduling, dead-letter, and partial-success recovery

- Retired in: connector delivery state machine implementation change set.
- Requirement: `docs/applications/connector-delivery-state-machine-requirement.md`.
- Design: `docs/applications/application-connector-runtime-design.md`.
- Fix summary: Added connector-local retry eligibility, `next_attempt_at`, delivery leases, `ClaimNextOutboxItem`, auto-claiming `ExecuteOutboxItem`, `ExecuteClaimedOutboxItem`, bounded retry backoff, retry exhaustion to dead-letter, non-retryable failure dead-lettering, partial-success and ambiguous-outcome recovery-required state, and duplicate suppression for terminal sent/dead-letter/recovery-required items.
- Boundary evidence: Delivery state remains under `internal/applications/connector_runtime`. Connector actions do not contain lease ids, worker ids, credentials, or raw external payloads. Delivery receipts record bounded status, reason, attempt, external action ref, and next retry time; connector delivery failure does not mutate kernel facts.
- Verification: `go test ./internal/applications/connector_runtime -count=1`; full verification recorded in the fixing commit.
- Residual risk: Operator console inspection and explicit connector recovery commands for recovery-required/dead-lettered items remain active under `APP-CONNECTOR-OPERATOR-CONSOLE-20260623`. Real Feishu listener/poller hardening remains active under `APP-CONNECTOR-FEISHU-LISTENER-20260623`.

### APP-CONNECTOR-DRIVER-TEMPLATE-20260624 - Do not hardcode external CLI argv in connector runtime

- Retired in: connector command-template driver implementation change set.
- Requirement: `docs/applications/application-connector-runtime-requirement.md`.
- Design: `docs/applications/application-connector-runtime-design.md`.
- Fix summary: Replaced the Feishu-specific Go argv builder with a generic `CommandTemplateDriver`. `ConnectorAction` remains the stable semantic contract; CLI argv, explicit profile, action template, and external action ref JSON paths now belong to driver configuration. The optional Feishu dry-run contract renders through the same template path instead of calling a Feishu-specific command helper.
- Boundary evidence: Production connector runtime code no longer defines `FeishuAdapter`, `feishuSendMessageArgs`, or Feishu-specific response parsing. The driver accepts argv token templates only, rejects shell-string templates, shell executables, and resolved script wrappers such as `.cmd`, `.bat`, `.ps1`, and extensionless Windows shims, rejects templates that do not bind `${profile}`, rejects unknown or credential-shaped variables, requires explicit profile values, rejects unexpected action payload/metadata, executes OS commands with an allowlisted environment, redacts optional CLI probe diagnostics, and records only safe opaque external action refs in bounded delivery result fields.
- Verification: `go test ./internal/applications/connector_runtime -run CommandTemplateDriver -count=1`; `go test ./internal/applications/connector_runtime -count=1`; full verification recorded in the fixing commit.
- Residual risk: Runtime loading of connector driver configuration and the longer-term `connector_command` external adapter process are not implemented yet. Feishu listener/poller hardening and operator capability probes remain active under `APP-CONNECTOR-FEISHU-LISTENER-20260623`.
