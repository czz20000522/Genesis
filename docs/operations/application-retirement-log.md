# Application Retirement Log

This file records retired user-space application issues. Active application
issues remain in `docs/operations/application-issues.md`.

Retired entries stay compact: one sentence with the retirement conclusion plus
fixing commit evidence. Detailed implementation notes, verification transcripts,
boundary proofs, and remaining production gaps belong in the cited commit,
tests, governing requirement/design, or still-active issues.

## Retired Issues

### APP-FIRST-RUN-PROVIDER-COMMAND-ACCEPTANCE-20260710 - Exercise configured local provider acceptance

- Retired: configured local Qwen now completes the full `-UseConfiguredProfile`
  acceptance flow without generated output or request deadlines, including
  provider verify, final turn, restart replay, and missing-config rejection.
  Evidence: `dbf6454a3` plus the 2026-07-12 live acceptance run.

### APP-CONNECTOR-OPERATOR-CONSOLE-20260623 - Add connector-local reconciliation probe evidence

- Retired: `genesis-console probe-outbox` now records read-only `ReconciliationEvidence` for `recovery_required` outbox items using exact lookup handles only, `inspect` projects the evidence, and terminal recovery remains an explicit connector-owned `resolve-outbox` step without adapter resend or kernel mutation. Evidence: retiring Lore commit.

### APP-CONNECTOR-FEISHU-ADAPTER-DRIVER-BOUNDARY-20260625 - Add Feishu connector adapter manifest and readiness probe

- Retired: `genesis-feishu-connector-adapter` now exposes a stable manifest/readiness probe, requires explicit profile posture before delivery, classifies unsupported actions and profile failures without running `lark-cli`, and keeps Feishu argv inside the adapter process behind typed `ConnectorAction` / `ConnectorActionResult`. Evidence: retiring Lore commit.

### APP-CODE-INTELLIGENCE-RUNTIME-READINESS-20260625 - Add CodeGraph readiness and advisory query projection

- Retired: CodeGraph now sits behind a user-space code intelligence runtime that classifies executable/cache/worktree/staleness/telemetry readiness, blocks unsafe queries by default, projects affected-tests as advisory hints, and keeps CodeGraph out of kernel core. Evidence: retiring Lore commit.

### APP-CODE-INTELLIGENCE-RUNTIME-SCOPE-GATE-20260625 - Validate code query scope before adapter execution

- Retired: code intelligence runtime now validates query kind, required fields, target containment, traversal, filesystem-root, and home-directory targets before adapter execution, while normalizing admitted relative targets for the adapter. Evidence: retiring Lore commit.

### APP-CONNECTOR-INBOUND-CONTEXT-UNIFICATION-20260623 - Unify inbound message slice with connector request context

- Retired: inbound messages now enter the connector runtime through connector-owned `ExternalEvent` and `RequestContext` rather than the removed split `message_ingress` package. Evidence: commit `085315652`.

### APP-CONNECTOR-OUTBOX-RECEIPT-20260623 - Add connector outbox/action/receipt owner

- Retired: external delivery is represented by connector-owned app commands, outbox items, actions, and receipts instead of kernel facts or adapter-local truth. Evidence: commit `08957756c`.

### APP-CONNECTOR-DELIVERY-STATE-MACHINE-20260623 - Add retry scheduling, dead-letter, and partial-success recovery

- Retired: connector delivery now has retry, dead-letter, and recovery-required terminal states owned by the connector runtime. Evidence: commit `e2f983cfe`.

### APP-CONNECTOR-DRIVER-TEMPLATE-20260624 - Do not hardcode external CLI argv in connector runtime

- Retired: external CLI argv moved behind the generic command-template driver while `ConnectorAction` remains the stable semantic contract. Evidence: commit `c4eac775e`.

### APP-CONNECTOR-COMMAND-BOUNDARY-20260624 - Implement connector_command as the long-lived adapter boundary

- Retired: connector delivery can cross to external adapter processes through typed `ConnectorAction` input and `ConnectorActionResult` output without embedding Feishu/mail/WeChat protocol semantics in the runtime. Evidence: commit `3e8d7a124`.

### APP-CONNECTOR-FEISHU-LISTENER-SMOKE-20260624 - Add Feishu inbound stream smoke path

- Retired: the Feishu smoke path can consume normalized NDJSON events, dedupe them, map them to kernel sessions, and submit turns without making Feishu a kernel owner. Evidence: commit `a48f61aa9`.

### APP-CONNECTOR-OPERATOR-CONSOLE-SMOKE-20260624 - Add read-only connector inspection console

- Retired: `genesis-console inspect` provides a read-only connector/kernel projection view without importing kernel internals or mutating connector state. Evidence: commit `34b273b86`.

### APP-CONNECTOR-OPERATOR-CONSOLE-FILTERS-20260624 - Add focused connector inspection filters

- Retired: console inspection now supports connector, inbound status, outbox status, and kernel session filters as read-only projections. Evidence: commit `c73b5df99`.

### APP-CONNECTOR-SOURCE-FAILURE-RECORDS-20260624 - Persist malformed source failures at the connector boundary

- Retired: malformed Feishu source events are durably recorded as connector-local source failures before `ExternalEvent`, `/turn`, or inbound submission facts are created. Evidence: commit `6653fe179`.

### APP-CONNECTOR-FINAL-TEXT-DELIVERY-SMOKE-20260624 - Deliver ordinary kernel final text through connector outbox

- Retired: opt-in Feishu smoke delivery sends ordinary kernel final text through connector-owned outbox/action/receipt flow without adding a kernel Feishu package or reply API. Evidence: commit `6d0095b49`.

### APP-CONNECTOR-OUTBOX-STORE-INTEGRITY-20260624 - Preserve connector file outbox writes across processes

- Retired: connector file-backed outbox mutations are serialized across processes so independent enqueue, claim, and receipt writes do not overwrite each other during the smoke phase. Evidence: commit `0bc127a91`.

### APP-CONNECTOR-SOURCE-FAILURE-RAW-PAYLOAD-20260624 - Keep raw source payloads out of durable connector facts

- Retired: source failure state now stores redacted diagnostics instead of raw external payload excerpts. Evidence: commit `e3dc483a4`.

### APP-CONNECTOR-FILE-STORE-SERIALIZATION-20260624 - Serialize inbound and source failure file stores

- Retired: inbound, source failure, and outbox file-backed smoke stores now use locked load-modify-write semantics so independent process-local writers preserve each other's records. Evidence: commit `e3dc483a4`.

### APP-CONNECTOR-IMPLEMENTATION-PLAN-DRIFT-20260624 - Reframe connector plan as implemented slices plus production gaps

- Retired: the connector implementation plan no longer presents retired delivery-state-machine work as an active issue and now points future work at remaining production boundaries. Evidence: commit `7906bd430`.

### APP-CONNECTOR-DRIVER-MIGRATION-20260625 - Move Feishu final delivery to connector_command

- Retired: Feishu final delivery and probe readiness now use `ConnectorCommandAdapter` by default, with `genesis-feishu-connector-adapter` owning the lark-cli send-message protocol and command-template retained only as explicit smoke fallback. Evidence: commit `c69928afe`.

### APP-CONNECTOR-COMMAND-OUTPUT-BOUNDS-20260625 - Bound external connector command output before parsing

- Retired: connector OS command capture, command-template delivery, and the Feishu connector adapter now cap external command output before parsing and return `external_command_output_exceeded` instead of treating oversized raw CLI output as delivery truth. Evidence: commit `ad73f4ee6`.

### APP-CONNECTOR-FILE-STORE-STALE-LOCK-20260625 - Recover stale connector file-store locks

- Retired: connector file-backed stores now inspect crash-left lock records, safely take over stale invalid or dead reservations without losing outbox state, and retain live or unverifiable locks instead of stealing them. Evidence: commit `f02263c47`.

### APP-CONNECTOR-PROFILE-READINESS-PROBE-BUILD-20260625 - Keep profile probe path buildable

- Retired: the profile readiness probe path no longer has duplicate `ok` switch cases and is covered by connector-runtime and ingress profile probe tests. Evidence: fixing Lore commit.

### APP-CONNECTOR-PROFILE-READINESS-PROBE-FAIL-CLOSED-20260625 - Fail closed on false or hanging profile probes

- Retired: `ready=false` profile probe results and timed-out profile probe commands now classify as `operator_action_required` before source or delivery adapters start. Evidence: fixing Lore commit.

### APP-CONNECTOR-EXTERNAL-RESOURCE-REF-NAMING-20260626 - Keep connector resource refs external

- Retired: connector runtime now uses `ExternalResourceRef` for inbound and command-side external resource handles, keeps them connector-local and opaque in kernel turn input, and documents that they are not kernel `resource_ref` authority. Evidence: fixing Lore commit; verified by `go test ./internal/applications/connector_runtime -count=1`.
