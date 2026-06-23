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

Already implemented as `internal/applications/message_ingress` and
`cmd/genesis-ingress`:

- validate inbound channel message;
- derive opaque kernel session id;
- derive grammar-safe kernel idempotency key;
- persist app-local inbound dedupe state;
- format inbound context;
- call kernel `/turn`;
- keep no external outbound sender in the ingress package.

This slice is useful but narrower than the production connector requirement.

## Phase B Current Slice

The first Phase B slice implemented the outbound owner:

- introduced `internal/applications/connector_runtime`;
- defined `ExternalThreadRef`, `AppCommand`, `ConnectorOutboxItem`,
  `ConnectorAction`, and `DeliveryReceipt`;
- added a file-backed outbox store with idempotency and receipt records;
- added console connector action executor for local diagnostics;
- added Feishu connector action executor behind a runner interface;
- kept actual Feishu SDK/CLI production listener hardening as a later issue.

Phase B also updates the generic protocol boundary owner documentation so the
same rule applies to Model Gateway, future WebUI/CLI/desktop shells, resource
intake, and credential-backed integrations.

## Phase B Remaining Slice

Unify inbound connector context:

- define `ExternalEvent`, `RequestContext`, and `ApplicationSessionMapping`;
- move or wrap current `message_ingress` behavior as the inbound connector path;
- keep current opaque session mapping and inbound dedupe semantics;
- keep connector inbound code outside kernel and outside provider context.

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

- Real Feishu listener/poller and signature verification remain Phase C.
- Real credential store integration for connector adapters remains future work.
- Rich messages, attachments, and resource intake remain future work.
- Operator console remains future work.

## Closing Gate

Before committing Phase B:

1. Re-open requirement, design, this plan, BDD feature, and application issues.
2. Verify implementation against every protocol-boundary rule.
3. Fix in-scope drift before commit.
4. Record out-of-scope drift as active application or kernel issue.
5. Run `go test ./... -count=1`, `go build ./...`, and `git diff --check`.
