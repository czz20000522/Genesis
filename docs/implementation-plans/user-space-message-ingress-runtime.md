# Implementation Plan: User-space Message Ingress Runtime

- **Requirement:** `docs/applications/user-space-message-ingress-runtime-requirement.md`
- **Design:** `docs/applications/user-space-message-ingress-runtime-design.md`
- **Status:** active

## Reference Scan Summary

Inspected local references only:

- `D:\software\JetBrains\python_workspace\codex-main\codex-rs\app-server\src\in_process.rs`
- `D:\software\JetBrains\python_workspace\codex-main\codex-rs\app-server\src\bespoke_event_handling.rs`
- `D:\software\JetBrains\python_workspace\reasonix\internal\acp\service.go`
- `D:\software\JetBrains\python_workspace\reasonix\internal\serve\wire.go`

Learned boundary:

- app-facing transports submit typed requests and consume typed events;
- core turn lifecycle remains in the controller/kernel;
- projection layers convert events for clients but do not become provider
  context, tool, memory, or ledger owners;
- per-session/request ids must be stable and unique at the caller boundary;
- application persistence can store transcript or inbox sidecars without
  becoming the core fact ledger.

Genesis alignment:

- use a user-space package instead of `internal/kernel`;
- talk to kernel over HTTP instead of importing kernel internals;
- keep channel dedupe and inbound submission state application-local;
- expose Feishu only as an inbound adapter/envelope source;
- leave Feishu outbound send to LLM + skill + `shell_exec` + `lark-cli`.

## Phase A Deliverable

Create the minimal local ingress runtime:

- `ChannelMessage` envelope and validation;
- deterministic opaque session mapping;
- restart-safe application dedupe store;
- inbound context formatter;
- kernel HTTP `/turn` client;
- runtime processor that reserves dedupe before kernel call;
- console one-shot inbound command with optional local final-answer display;
- Feishu one-shot inbound command that submits messages and does not auto-send
  external replies;
- BDD feature documenting the behavior.

Phase A does not implement a long-running Feishu event listener, outbound Feishu
delivery, rich Feishu cards, documents, attachments, operator console inspection
UI, or real inbound retry backoff.

## Tests

Write tests before implementation for:

- invalid `ChannelMessage` fails before kernel call;
- first inbound message submits exactly one kernel turn;
- duplicate inbound message returns the previous record and does not call kernel;
- inbound context includes channel reply references but no permission/sandbox or
  provider-context authority fields;
- Feishu one-shot processing does not invoke `lark-cli` or any outbound sender;
- the user-space package does not import `internal/kernel`.

## Red Lines

- No Feishu package under `internal/kernel`.
- No import of `genesis/internal/kernel` from the application runtime.
- No app-owned provider context assembly.
- No app writes to kernel ledger, memory, tool result, checkpoint, or audit
  storage.
- No channel identity mapped to kernel permission mode or sandbox profile.
- No automatic external-channel reply delivery from ingress runtime.

## Phase A Still Short Of Production

- Feishu inbound listener/poller remains future work.
- Adapter retry/signature/token handling is only represented by explicit
  envelope validation and future issues.
- Operator console projection browsing remains future work.
- Multi-channel concurrent processing and store locking are minimal.

## Closing Gate

Before commit:

1. Re-read requirement, design, this plan, BDD feature, and application issues.
2. Check every non-goal and red line against the diff.
3. Add active application issues for remaining Phase B/C gaps.
4. Run `go test ./... -count=1`, `go build ./...`, and `git diff --check`.
