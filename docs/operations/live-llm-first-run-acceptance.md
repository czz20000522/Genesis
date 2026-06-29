# Live LLM First-Run Acceptance

This runbook is the operator-facing acceptance path for a clean Genesis Kernel live provider setup. It is not an application feature and does not add provider-specific behavior to the kernel. It exercises existing kernel surfaces through `genesisctl`, `genesisd`, and unversioned HTTP routes.

## Prerequisites

- Go is installed. On this workstation the expected path is `D:\software\Go\bin\go.exe`.
- An OpenAI-compatible provider base URL is available.
- A model id for that provider is available.
- The provider API key is available through an environment variable. Do not pass the raw key as a command-line argument.

## Scripted Acceptance

Run from the repo root:

```powershell
$env:GENESIS_PROVIDER_API_KEY = "<provider api key>"
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\first_run_live_llm_acceptance.ps1 `
  -BaseUrl https://provider.example.com/api `
  -Model provider-model
```

The script creates an isolated temporary work root, builds `genesisctl.exe` and `genesisd.exe`, writes `models.json`, stores the credential behind a `secret://...` ref, runs `genesisctl provider verify` against the upstream provider, starts `genesisd` through Genesis config, and checks:

- `genesisctl provider verify` reports `readiness=ready` before the daemon starts.
- `GET /ready` reports `readiness=ready`.
- `GET /ready` reports a configured live provider rather than the fake provider.
- `POST /turn` returns a non-empty assistant final from the configured provider, not the fake provider.
- `GET /sessions/{id}/timeline` returns a usable user-facing timeline projection.
- `GET /turns/{id}/events` returns the raw turn event replay.
- `GET /turns/{id}/context` returns the provider-context inspection projection.
- Restarting `genesisd` with the same ledger preserves the same timeline, events, and context projections.
- A missing credential store reports `provider_credential_missing` through readiness and `provider_unavailable` on turn submission instead of panicking or leaking the secret.

The JSON summary printed by the script includes paths, session id, turn id, provider verify status, provider model, projection counts, and failure-probe status. It must not include the raw provider API key.

`GET /ready` is local kernel readiness. It verifies local config resolution, runtime auth, ledger access, and provider adapter readiness. It does not prove upstream credentials are accepted. `genesisctl provider verify` is the explicit upstream-authenticated live readiness probe and must pass before live LLM smoke work.

Useful options:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\first_run_live_llm_acceptance.ps1 -Help

powershell -NoProfile -ExecutionPolicy Bypass -File scripts\first_run_live_llm_acceptance.ps1 `
  -BaseUrl https://provider.example.com/api `
  -Model provider-model `
  -WorkRoot .\.genesis-live\acceptance `
  -Addr 127.0.0.1:8765 `
  -ApiKeyEnv GENESIS_PROVIDER_API_KEY `
  -KeepServer
```

Use `-SkipFailureProbe` only when another process owns the test port and the positive live path is the current focus.

## Manual Acceptance

Build the binaries:

```powershell
$root = Join-Path (Get-Location) ".genesis-live\manual"
New-Item -ItemType Directory -Force "$root\bin" | Out-Null
D:\software\Go\bin\go.exe build -o "$root\bin\genesisctl.exe" .\cmd\genesisctl
D:\software\Go\bin\go.exe build -o "$root\bin\genesisd.exe" .\cmd\genesisd
```

Write provider config and the local credential record:

```powershell
"<provider api key>" | & "$root\bin\genesisctl.exe" provider use deepseek/deepseek-v4-flash `
  -config-root "$root\config" `
  -credential-store-root "$root\credentials" `
  -api-key-stdin
```

The preset command is the preferred operator path for known providers. It
resolves profile id, route, base URL, credential ref, protocol, context-window
metadata, and timeout before calling the same low-level setup owner. To rotate
the key for the active profile without repeating provider metadata:

```powershell
"<new provider api key>" | & "$root\bin\genesisctl.exe" provider rotate-key `
  -config-root "$root\config" `
  -credential-store-root "$root\credentials" `
  -repair-profile-metadata deepseek/deepseek-v4-flash `
  -api-key-stdin
```

`-repair-profile-metadata` is required when rotating an older development
profile that was created before adapter binding metadata existed. It only
repairs the active profile when the known preset still matches the profile,
model, and route; mismatched custom profiles must be repaired through the
normal setup path instead of silently rewriting `models.json`.

The low-level command remains available for custom OpenAI-compatible providers:

```powershell
"$root\bin\genesisctl.exe" provider-setup `
  -config-root "$root\config" `
  -credential-store-root "$root\credentials" `
  -profile-id live-acceptance `
  -gateway-route live-acceptance `
  -base-url https://provider.example.com/api `
  -model provider-model `
  -credential-ref secret://models/provider/live-acceptance
```

Verify the configured provider against upstream auth before starting the daemon:

```powershell
& "$root\bin\genesisctl.exe" provider verify `
  -config-root "$root\config" `
  -credential-store-root "$root\credentials" `
  -model-role foreground.coordinator `
  -timeout-sec 10
```

The command must return JSON with `readiness = "ready"`. Missing credentials should return `readiness = "not_ready"` with a credential readiness reason, and invalid or expired upstream keys should return `provider_auth_failed`. The output must not contain the raw key, Authorization header, or provider response body.

Start the kernel through Genesis-owned config:

```powershell
$token = "local-live-acceptance-token"
"$root\bin\genesisd.exe" `
  -addr 127.0.0.1:8765 `
  -ledger "$root\events.sqlite" `
  -runtime-token $token `
  -provider genesis-config `
  -config-root "$root\config" `
  -credential-store-root "$root\credentials" `
  -skill-root "$root\skills\scientific-operator" `
  -disable-default-skill-roots
```

Focused live sessions should pass the intended skill roots first. Explicit
roots are scanned before defaults, and `/capabilities` plus session debug expose
path-free root status and `skill_index_budget_excluded` warnings when the
bounded index drops configured skills. `-disable-default-skill-roots` /
`GENESIS_DISABLE_DEFAULT_SKILL_ROOTS=true` is a smoke/dev escape hatch, not the
normal skill-store model.

In another PowerShell session, verify readiness and one real turn:

```powershell
Invoke-RestMethod http://127.0.0.1:8765/ready

$headers = @{ Authorization = "Bearer $token" }
$turn = Invoke-RestMethod `
  -Method Post `
  -Uri http://127.0.0.1:8765/turn `
  -Headers $headers `
  -ContentType "application/json" `
  -Body (@{
    session_id = "manual-live-first-run"
    idempotency_key = "manual-live-first-run-1"
    input_items = @(@{ type = "text"; text = "Reply with exactly: GENESIS_LIVE_LLM_ACCEPTANCE_OK" })
  } | ConvertTo-Json -Depth 16 -Compress)

Invoke-RestMethod -Headers $headers "http://127.0.0.1:8765/sessions/manual-live-first-run/timeline"
Invoke-RestMethod -Headers $headers "http://127.0.0.1:8765/turns/$($turn.turn_id)/events"
Invoke-RestMethod -Headers $headers "http://127.0.0.1:8765/turns/$($turn.turn_id)/context"
```

Optional debug capture for a difficult session:

```powershell
Invoke-RestMethod `
  -Method Post `
  -Uri http://127.0.0.1:8765/sessions/manual-live-first-run/debug/enable `
  -Headers $headers `
  -ContentType "application/json" `
  -Body "{}"

# submit the turn after enabling debug, then export the bounded debug artifact
Invoke-RestMethod -Headers $headers "http://127.0.0.1:8765/sessions/manual-live-first-run/debug"
```

Session debug is opt-in. Its artifact is a debug trace stored outside transcript/audit truth. Deleting it must not affect session resume, timeline, context inspection, or audit replay. It should be used to inspect provider-step input kinds, model-visible tool manifest, bounded input/tool-result previews, provider final/error, and usage when live model behavior is poor.

Stop and restart `genesisd` with the same `-ledger`, `-config-root`, and `-credential-store-root`, then re-run the three inspection requests. The same turn must remain readable after restart.

## Failure Diagnostic Check

Start `genesisd` with the same `-config-root` and an empty `-credential-store-root`. `GET /ready` should return a blocked provider with reason `provider_credential_missing`, and `POST /turn` should return a structured `provider_unavailable` error. This proves provider configuration failure is projected as structured kernel state and not as a panic, fake-provider fallback, or secret-bearing log message.
