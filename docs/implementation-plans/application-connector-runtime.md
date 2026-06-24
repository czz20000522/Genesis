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

Learned boundary:

- external clients submit typed requests and consume typed events;
- core controller/kernel owns turn state and runtime authority;
- protocol adapters project or translate, but do not become fact owners;
- Genesis needs one additional owner compared with those references:
  connector outbox/receipt for production external delivery.

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
- adds console connector action executor for local diagnostics;
- adds Feishu connector action executor behind a runner interface;
- keeps actual Feishu SDK/CLI production listener hardening as a later issue.

Phase B also updates the generic protocol boundary owner documentation so the
same rule applies to Model Gateway, future WebUI/CLI/desktop shells, resource
intake, and credential-backed integrations.

## Tests

Phase B must add tests for:

- external event normalization does not expose raw external ids as public system ids;
- inbound duplicate does not submit duplicate kernel turn;
- app command creates one outbox item with a connector idempotency key;
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

- Real Feishu listener/poller and signature verification remain Phase C. The
  first Feishu event-source driver now wraps `lark-cli event consume
  im.message.receive_v1` and maps its flattened NDJSON into `ExternalEvent`,
  but it is still a bounded smoke driver: it does not yet make source
  validation verified, and the hardcoded event command shape must move behind
  connector driver configuration or an external adapter process before
  production. Webhook signature verification, durable source dead-letter
  records, backoff supervision, and live mobile smoke are still open.
- Real credential store integration for connector adapters remains future work.
- Delivery retry scheduling, dead-letter, and partial-success recovery remain an
  active issue.
- Rich messages, attachments, and resource intake remain future work.
- Operator console now has read-only inspection and an explicit connector-local
  `requeue-outbox` command for dead-lettered connector items. Rich filtered
  views, connector-specific reconciliation, and safe handling of
  recovery-required partial/ambiguous outcomes remain future work.

## Closing Gate

Before committing Phase B:

1. Re-open requirement, design, this plan, BDD feature, and application issues.
2. Verify implementation against every protocol-boundary rule.
3. Fix in-scope drift before commit.
4. Record out-of-scope drift as active application or kernel issue.
5. Run `go test ./... -count=1`, `go build ./...`, and `git diff --check`.
