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
- Residual risk: Runtime loading of connector driver configuration, packaged external adapter processes, Feishu listener/poller hardening, and operator capability probes remain active under `APP-CONNECTOR-FEISHU-LISTENER-20260623`.

### APP-CONNECTOR-COMMAND-BOUNDARY-20260624 - Implement connector_command as the long-lived adapter boundary

- Retired in: connector command adapter process implementation change set.
- Requirement: `docs/applications/application-connector-runtime-requirement.md`.
- Design: `docs/applications/application-connector-runtime-design.md`.
- Fix summary: Added `ConnectorCommandAdapter`, which starts a configured direct external adapter executable, writes typed `ConnectorAction` JSON to stdin, reads one typed `ConnectorActionResult` JSON object from stdout, validates status/reason/external action refs, applies a default timeout, uses an explicit or allowlisted environment, and returns normalized connector-local results to the outbox runtime.
- Boundary evidence: The adapter contains no Feishu, mail, WeChat, SDK, HTTP, or CLI command semantics. External adapter stdout is the typed result channel; malformed JSON, unsupported statuses, unsafe external refs, unsafe reasons, missing direct executables, timeouts, and failed adapter processes fail closed as connector-local delivery failures. Stderr and raw stdout are not persisted as `DeliveryReceipt` truth.
- Verification: `TestConnectorCommandAdapterSendsTypedActionAndReadsTypedResult`; `TestConnectorCommandAdapterRejectsMalformedJSON`; `TestConnectorCommandAdapterRejectsUnsupportedStatus`; `TestConnectorCommandAdapterRedactsStderrAndDoesNotPersistRawOutput`; `TestConnectorCommandAdapterRejectsSecretShapedExternalActionRef`; `TestConnectorCommandAdapterRejectsMismatchedActionConnector`; `TestConnectorCommandAdapterRejectsCredentialShapedEnv`; `TestRuntimeExecuteOutboxItemWithConnectorCommandRecordsReceipt`; `TestRuntimeExecuteOutboxItemWithConnectorCommandFailureRecordsRedactedReceipt`; `go test ./internal/applications/connector_runtime -count=1`.
- Residual risk: This retires only the generic process boundary. Packaged Feishu external adapter processes, installed-adapter capability probes, listener/poller hardening, runtime driver config loading, and operator console inspection remain active under the remaining application issues.

### APP-CONNECTOR-FEISHU-LISTENER-SMOKE-20260624 - Add Feishu inbound stream smoke path

- Retired in: Feishu listener smoke implementation change set.
- Requirement: `docs/applications/application-connector-runtime-requirement.md`.
- Design: `docs/applications/application-connector-runtime-design.md`.
- Fix summary: Added `genesis-ingress feishu-listen --stdin-jsonl --profile ...`, which consumes Feishu adapter NDJSON as connector-owned `ExternalEvent` records, submits them through the existing Application Connector Runtime, emits one `ProcessResult` JSON record per input event, and requires an explicit Feishu profile before kernel submission.
- Boundary evidence: The command does not implement Feishu protocol logic inside the kernel. It accepts already-normalized connector events from a user-space event source, reuses connector-local dedupe and session mapping, and does not expose raw external ids as kernel authority.
- Verification: `TestFeishuListenConsumesNDJSONEventsAndDedupes`; `TestFeishuListenRequiresExplicitProfileBeforeKernelCall`; `go test ./cmd/genesis-ingress -count=1`.
- Residual risk: This retires only the automated smoke stream. Real Feishu webhook or `lark-cli event` process supervision, signature/source validation, source retry/backoff, token refresh, and installed-adapter probes remain active under `APP-CONNECTOR-FEISHU-LISTENER-20260623`.

### APP-CONNECTOR-OPERATOR-CONSOLE-SMOKE-20260624 - Add read-only connector inspection console

- Retired in: operator console smoke implementation change set.
- Requirement: `docs/applications/application-connector-runtime-requirement.md`.
- Design: `docs/applications/application-connector-runtime-design.md`.
- Fix summary: Added `genesis-console inspect`, which reads connector inbound state through `FileInboundStore`, outbox and receipt state through `FileOutboxStore`, and optional kernel session projections through the kernel HTTP surface.
- Boundary evidence: The console does not import kernel internals, reconstruct provider context, write kernel facts, or mutate connector state. Kernel projections are fetched through HTTP and emitted as inspection material beside connector-owned state.
- Verification: `TestConsoleInspectReadsConnectorStateAndKernelProjection`; `go test ./cmd/genesis-console -count=1`.
- Residual risk: This retires only the read-only inspection view. Recovery commands for recovery-required and dead-lettered connector state remain active under `APP-CONNECTOR-OPERATOR-CONSOLE-20260623`.

### APP-CONNECTOR-FINAL-TEXT-DELIVERY-SMOKE-20260624 - Deliver ordinary kernel final text through connector outbox

- Retired in: Feishu final-text delivery smoke implementation change set.
- Requirement: `docs/applications/application-connector-runtime-requirement.md`.
- Design: `docs/applications/application-connector-runtime-design.md`.
- Fix summary: Added opt-in final-text delivery to `genesis-ingress feishu-listen --deliver-final`. When a connector inbound event completes a kernel turn with non-empty final text, application policy creates one `send_message` `AppCommand`, enqueues it through the connector outbox, executes it through the configured connector adapter, and records `DeliveryReceipt`. Duplicate inbound events reuse the existing request record and do not resend the reply. `--ignore-sender-id` lets live Feishu smoke ignore Genesis bot-originated events before kernel submission.
- Boundary evidence: The kernel still has no Feishu package or reply API. Final-text delivery is connector application policy, uses `AppCommand` and `ConnectorAction`, and delivery errors stay in connector state as `delivery_error`/receipt evidence without rewriting kernel turn facts.
- Verification: `TestProcessExternalEventDeliversFinalTextThroughConnectorOutbox`; `TestProcessExternalEventDuplicateDoesNotDeliverFinalAgain`; `TestProcessExternalEventDeliveryFailureDoesNotFailKernelSubmission`; `TestFeishuEventSourceIgnoresConfiguredSenderIDs`; `go test ./internal/applications/connector_runtime -run "Test(ProcessExternalEvent|FeishuEventSource|CommandTemplateDriver)" -count=1`; `go test ./cmd/genesis-ingress -count=1`. Live smoke on 2026-06-24 with `--profile genesis`, kernel `http://127.0.0.1:8876`, and collaboration chat `oc_42fd594ba10832c8feb86f8aaa5918a6` observed a user Feishu message, kernel final text `GENESIS_FINAL_TEXT_DELIVERY_SMOKE_20260624_115624`, outbox status `sent`, receipt status `sent`, and Feishu message id `om_x100b6c81107facacb28ab7dd200afba`.
- Residual risk: This retires only ordinary final-text smoke delivery. Source verification, listener supervision, source retry/backoff, durable source dead-letter records, profile/token refresh, installed-adapter capability probes, connector store cross-process integrity, rich messages, attachments, and production adapter packaging remain active or future issues.
