# Implementation Plan: Application Connector Runtime

- **Requirement:** `docs/applications/application-connector-runtime-requirement.md`
- **Design:** `docs/applications/application-connector-runtime-design.md`
- **Status:** implemented smoke slices with active production gaps

## Reference Scan Summary

Inspected local references:

- `D:\software\JetBrains\python_workspace\codex-main\codex-rs\app-server\src\in_process.rs`
- `D:\software\JetBrains\python_workspace\codex-main\codex-rs\app-server\src\bespoke_event_handling.rs`
- `D:\software\JetBrains\python_workspace\reasonix\internal\acp\service.go`
- `D:\software\JetBrains\python_workspace\reasonix\internal\serve\wire.go`
- `D:\software\JetBrains\python_workspace\codex-main\codex-rs\message-history\src\lib.rs`
- `D:\software\JetBrains\python_workspace\codex-main\codex-rs\app-server-daemon\README.md`
- `D:\software\JetBrains\python_workspace\reasonix\internal\serve\serve.go`
- `D:\software\JetBrains\python_workspace\codex-main\codex-rs\app-server\src\request_processors\thread_lifecycle.rs`
- `D:\software\JetBrains\python_workspace\codex-main\codex-rs\app-server\src\request_processors\thread_processor.rs`
- `D:\software\JetBrains\python_workspace\reasonix\internal\serve\wire.go`
- `D:\software\JetBrains\python_workspace\reasonix\internal\acp\service.go`

Learned boundary:

- external clients submit typed requests and consume typed events;
- core controller/kernel owns turn state and runtime authority;
- protocol adapters project or translate, but do not become fact owners;
- Genesis needs one additional owner compared with those references:
  connector outbox/receipt for production external delivery.
- cross-process mutation needs a serialization boundary around the full state
  change, not only an in-process mutex: Codex history uses advisory locking for
  concurrent writers, Codex daemon lifecycle serializes mutating commands per
  home directory, and Reasonix holds a write lock across controller rebuild so
  concurrent readers never observe a half-swapped state.
- operator/control commands should delegate to the owner that owns the state
  transition. Codex composes resume responses from thread/core state and sends
  requests through listener commands instead of fabricating thread truth in the
  request shell. Reasonix projects controller events through `serve/wire.go` and
  routes `session/cancel` through session/controller state instead of editing
  transcript facts in the ACP transport.

## Phase A Current Slice

Implemented in `internal/applications/connector_runtime` and consumed by
`cmd/genesis-ingress`:

- validate inbound `ExternalEvent`;
- create `RequestContext`;
- derive opaque kernel session id;
- derive grammar-safe kernel idempotency key;
- persist app-local inbound dedupe state;
- format request context without raw external ids or authority fields;
- call kernel `/turn`;
- keep no external outbound sender in the ingress path.

Earlier narrow message-ingress code has been removed instead of preserved as a
parallel truth surface.

## Phase B Current Slice

The inbound unification slice:

- defines `ExternalEvent`, `RequestContext`, and `ApplicationSessionMapping`;
- moves inbound submission into `internal/applications/connector_runtime`;
- deletes the narrower message-ingress package instead of wrapping it forever;
- keeps current opaque session mapping and inbound dedupe semantics;
- keeps connector inbound code outside kernel and outside provider context.

The outbound slice:

- introduces `internal/applications/connector_runtime`;
- defines `ExternalThreadRef`, `AppCommand`, `ConnectorOutboxItem`,
  `ConnectorAction`, and `DeliveryReceipt`;
- adds a file-backed outbox store with idempotency and receipt records;
- serializes file-backed outbox load-modify-write operations with a
  connector-local lock file so separate console/worker processes do not lose
  independent items, receipts, or delivery claims during smoke use;
- adds console connector action executor for local diagnostics;
- adds Feishu connector action executor behind a runner interface;
- keeps actual Feishu SDK/CLI production listener hardening as a later issue.

The Phase C smoke loop:

- adds an opt-in `genesis-ingress feishu-listen --deliver-final` path;
- treats kernel `final_text` as one ordinary `send_message` app command for
  the same connector request context;
- dedupes replies by request id plus kernel turn id;
- executes delivery through the connector outbox/adapter path and records a
  `DeliveryReceipt`;
- supports `--ignore-sender-id` so bot-originated Feishu events do not trigger
  a reply loop during live smoke;
- keeps final-text delivery as application policy, not kernel reply semantics.

Phase B also updates the generic protocol boundary owner documentation so the
same rule applies to Model Gateway, future WebUI/CLI/desktop shells, resource
intake, and credential-backed integrations.

## Implemented Test Coverage

Current automated coverage includes:

- external event normalization does not expose raw external ids as public system ids;
- inbound duplicate does not submit duplicate kernel turn;
- app command creates one outbox item with a connector idempotency key;
- final text delivery enqueues and executes one connector reply action when
  enabled;
- duplicate inbound events do not deliver the same final text twice;
- duplicate app command does not enqueue duplicate connector action;
- failed connector action records `DeliveryReceipt` without changing kernel facts;
- connector package does not import `internal/kernel`;
- connector package does not expose external credentials to model-visible fields.
- malformed source events create connector-local source failure diagnostics
  without persisting raw external payloads;
- file-backed connector stores preserve independent writes across process-local
  store instances.

## Red Lines

- No Feishu package under `internal/kernel`.
- No connector import of `genesis/internal/kernel`.
- No connector-owned provider context.
- No connector writes to kernel ledger, memory, tool result, checkpoint, or audit.
- No model-visible external credentials.
- No direct production model shell/CLI outbound path.

## Still Short Of Production After Phase B

- Real Feishu listener/poller hardening and signature verification remain
  Phase C and are governed by
  `docs/applications/connector-source-supervisor-requirement.md` and
  `docs/applications/connector-source-supervisor-design.md`. Source intake now
  uses the `source_command` typed streaming boundary: connector runtime starts a
  source adapter process, consumes source frames, and records `SourceRun`,
  `SourceAttempt`, `SourceCursor`, `SourceFailureRecord`, and
  `SourceVerificationEvidence` state. The Feishu source adapter command owns
  `lark-cli event consume` and raw Feishu payload parsing, then emits typed
  source frames. This is still a bounded smoke-grade source path: it does not
  yet make source validation verified and does not provide a production process
  supervisor, webhook signature verification, or credential/profile refresh.
  Runtime source code must not know Feishu event consume argv, identity flags,
  event keys, or raw source payload envelopes.
  `genesis-ingress feishu-probe` now gives operators a no-side-effect
  installed-adapter readiness report for the event source and ordinary final
  delivery surfaces. Webhook signature verification, durable source dead-letter
  records, credential/profile refresh integration, and production source
  supervision remain open.
- `source_command` is not `connector_command` and is not an argv template. It
  is a long-running source process stream with `source.ready`, `source.event`,
  `source.cursor`, `source.failed`, and `source.stopped` frames. Cursors persist
  only after durable event acceptance, malformed frames become redacted source
  failures, and `source_validation=verified` requires inspectable verification
  evidence.
- Real credential store integration for connector adapters remains future work.
- The file-backed outbox store is still local connector infrastructure. It now
  protects bounded cross-process smoke writes, but a future production store,
  database, append-only journal, or single owner process may replace it instead
  of preserving the current JSON file format.
- Delivery retry scheduling, dead-letter, partial-success recovery, leases, and
  operator recovery commands are implemented as connector-local state machine
  slices. Remaining recovery work is connector-specific reconciliation evidence
  before production automatic terminal resolution.
- Rich messages, attachments, and resource intake remain future work.
- Operator console now has read-only inspection, outbox delivery summaries with
  last-receipt diagnostics, and an explicit connector-local `requeue-outbox`
  command for dead-lettered connector items. Connector-specific reconciliation
  probes remain future work.
- Operator console now has an explicit connector-local `resolve-outbox` command
  for recovery-required partial/ambiguous outcomes. It records a terminal
  operator receipt as `sent` or `dead_lettered`, preserves receipt history,
  clears connector-local lease/schedule fields, and does not execute adapters
  or call kernel.

## Continuing Gate

Before committing another connector slice:

1. Re-open requirement, design, this plan, BDD feature, and application issues.
2. Verify implementation against every protocol-boundary rule.
3. Fix in-scope drift before commit.
4. Record out-of-scope drift as active application or kernel issue.
5. Run `go test ./... -count=1`, `go build ./...`, and `git diff --check`.
