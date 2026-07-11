# Connector Binding Configuration Plan

- **Requirement:** `docs/applications/application-connector-runtime-requirement.md`.
- **Design:** `docs/applications/connector-source-verification-lifecycle-design.md`.
- **Issue:** `APP-CONNECTOR-FEISHU-LISTENER-20260623`.

## Reference Scan

Reasonix's `internal/boot/boot.go:Build` resolves `config.LoadForRoot(root)`
before choosing a model, creating the provider, composing the stable prompt
prefix, or building the tool registry. `internal/config/config.go:LoadForRoot`
merges user and project config in a documented order and returns an error on a
bad source instead of silently falling back. Genesis aligns on the essential
control-plane point: a real listener reads one explicit user-home binding before
it constructs source lifecycle state or starts an external process. Genesis
intentionally differs by not giving connector settings a project override: the
external bot identity is a Genesis Home/user authority binding, not repository
configuration.

Codex app-server's `config_manager.rs:ConfigManager::load_for_cwd` is called by
`request_processors/turn_processor.rs` before thread settings overrides are
accepted; invalid permission-profile configuration rejects the request rather
than being downgraded. Genesis follows the fail-closed part of that posture for
listener enablement and typed Feishu identity, but does not import Codex's
generic configuration stack into a connector adapter.

Neither reference owns a mobile-channel source/outbox lifecycle. Genesis keeps
that truth in `connector_runtime`; the binding only supplies whether a specific
adapter is allowed to activate and the typed values needed to invoke it.

## Slice

Phase A reads the existing user-home Feishu runtime binding for non-test
listeners. It requires `listener.enabled=true`; the configured profile,
identity, and optional command are propagated into the Feishu source adapter.
Missing, invalid, empty-profile, or disabled binding refuses before kernel
runtime construction, source lifecycle persistence, or source process start.
The literal `lark-cli` user setting is resolved to its installed direct binary
when present, so a Windows npm PowerShell shim is never executed as the adapter
command. Keep `stdin-jsonl` as the deterministic test surface. Do not invent a
universal vendor-settings map before a second adapter needs a shared field.

Phase B, only after an actual second connector, may extract the common
enablement/adapter-ref portion into a small shared descriptor. It must preserve
typed adapter settings for mail subscription renewal, WeChat gateway choice, or
QQ event intents. A marketplace, generic vendor credential schema, or protocol
SDK facade is outside this plan.

## Evidence

Automated proof:

- missing and invalid runtime-settings files classify as local configuration
  errors;
- a disabled binding prevents the source helper side effect;
- an enabled binding supplies its profile, command, and identity to source
  argument construction;
- configured `lark-cli` resolves to the direct installed binary;
- profile readiness continues to gate source start after binding resolution.

Operator evidence:

- with the supplied user binding set to `enabled=false`, `genesis-ingress
  feishu-listen` refuses before it can start a listener;
- real inbound/outbound proof remains operator-driven and requires an explicit
  enablement change plus a user-originated Feishu message.
