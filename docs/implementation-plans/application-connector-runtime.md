# Implementation Plan: Application Connector Runtime

- **Requirement:** `docs/applications/application-connector-runtime-requirement.md`
- **Design:** `docs/applications/application-connector-runtime-design.md`
- **Status:** active

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

## Tests

Phase B must add tests for:

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

## Red Lines

- No Feishu package under `internal/kernel`.
- No connector import of `genesis/internal/kernel`.
- No connector-owned provider context.
- No connector writes to kernel ledger, memory, tool result, checkpoint, or audit.
- No model-visible external credentials.
- No direct production model shell/CLI outbound path.

## Still Short Of Production After Phase B

- Real Feishu listener/poller hardening and signature verification remain
  Phase C. The first Feishu event-source driver now wraps `lark-cli event
  consume im.message.receive_v1` and maps its flattened NDJSON into
  `ExternalEvent`, but it is still a bounded smoke driver: it does not yet make
  source validation verified, and the hardcoded event command shape must move
  behind connector driver configuration or an external adapter process before
  production. Webhook signature verification, durable source dead-letter
  records, backoff supervision, installed-adapter health probes, and production
  source retry remain open.
- Real credential store integration for connector adapters remains future work.
- The file-backed outbox store is still local connector infrastructure. It now
  protects bounded cross-process smoke writes, but a future production store,
  database, append-only journal, or single owner process may replace it instead
  of preserving the current JSON file format.
- Delivery retry scheduling, dead-letter, and partial-success recovery remain an
  active issue.
- Rich messages, attachments, and resource intake remain future work.
- Operator console now has read-only inspection and an explicit connector-local
  `requeue-outbox` command for dead-lettered connector items. Rich filtered
  views and connector-specific reconciliation remain future work.
- Operator console now has an explicit connector-local `resolve-outbox` command
  for recovery-required partial/ambiguous outcomes. It records a terminal
  operator receipt as `sent` or `dead_lettered`, preserves receipt history,
  clears connector-local lease/schedule fields, and does not execute adapters
  or call kernel.

## Closing Gate

Before committing Phase B:

1. Re-open requirement, design, this plan, BDD feature, and application issues.
2. Verify implementation against every protocol-boundary rule.
3. Fix in-scope drift before commit.
4. Record out-of-scope drift as active application or kernel issue.
5. Run `go test ./... -count=1`, `go build ./...`, and `git diff --check`.
