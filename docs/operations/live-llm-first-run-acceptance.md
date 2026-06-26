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

The script creates an isolated temporary work root, builds `genesisctl.exe` and `genesisd.exe`, writes `models.json`, stores the credential behind a `secret://...` ref, starts `genesisd` through Genesis config, and checks:

- `GET /ready` reports `readiness=ready`.
- `GET /ready` reports a configured live provider rather than the fake provider.
- `POST /turn` returns a non-empty assistant final from the configured provider, not the fake provider.
- `GET /sessions/{id}/timeline` returns a usable user-facing timeline projection.
- `GET /turns/{id}/events` returns the raw turn event replay.
- `GET /turns/{id}/context` returns the provider-context inspection projection.
- Restarting `genesisd` with the same ledger preserves the same timeline, events, and context projections.
- A missing credential store reports `provider_credential_missing` through readiness and `provider_unavailable` on turn submission instead of panicking or leaking the secret.

The JSON summary printed by the script includes paths, session id, turn id, provider model, projection counts, and failure-probe status. It must not include the raw provider API key.

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
  -api-key-stdin
```

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

Start the kernel through Genesis-owned config:

```powershell
$token = "local-live-acceptance-token"
"$root\bin\genesisd.exe" `
  -addr 127.0.0.1:8765 `
  -ledger "$root\events.jsonl" `
  -runtime-token $token `
  -provider genesis-config `
  -config-root "$root\config" `
  -credential-store-root "$root\credentials"
```

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

Stop and restart `genesisd` with the same `-ledger`, `-config-root`, and `-credential-store-root`, then re-run the three inspection requests. The same turn must remain readable after restart.

## Failure Diagnostic Check

Start `genesisd` with the same `-config-root` and an empty `-credential-store-root`. `GET /ready` should return a blocked provider with reason `provider_credential_missing`, and `POST /turn` should return a structured `provider_unavailable` error. This proves provider configuration failure is projected as structured kernel state and not as a panic, fake-provider fallback, or secret-bearing log message.
