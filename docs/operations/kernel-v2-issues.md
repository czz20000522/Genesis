# Kernel Issue Ledger

This file is the repo-owned ledger for active Genesis Kernel issues. Feishu Base is the collaboration inbox; this file is the durable project source for issues that still need code, verification, or user acceptance.

Retired issues must not remain here. Move accepted retirements to `docs/operations/kernel-v2-retirement-log.md` with the fixing commits, verification evidence, residual risks, and retirement reason.

## Ledger Rules

- Keep only `new`, `open`, `in_progress`, or otherwise active issues in this file.
- Do not record application-specific feature work as kernel work unless it changes a kernel primitive.
- Do not add versioned HTTP route names as current contracts. HTTP is transport; current kernel endpoints are unversioned.
- `ready_for_acceptance` issues move to the retirement log as retirement candidates and leave this active ledger after acceptance.
- Feishu/Base links may point to collaboration artifacts, but this repo must contain enough evidence to understand the current status without opening Feishu.

## Active Issues

### recvnd2PDI1LuV - P0 - Minimal Go single-binary spike

- Status: open.
- Type: architecture.
- Problem: the Go kernel spike must prove a runnable single binary with `/ready`, `/turn`, `/sessions/{id}`, fake provider mode, OpenAI-compatible provider mode, restart-safe session replay, and no project-specific or shell-specific assumptions.
- Current evidence: commits `559e1c0c7`, `fd5bf9d8a`, `db9aeca13`, and `22d5ca9f4` prove the fake provider loop, provider configuration, provider context boundary, and an opt-in live provider smoke test.
- Remaining blocker: live external OpenAI-compatible provider verification has not run because no Genesis-owned provider credential/config is present in the current environment. Codex credentials must not be reused as Genesis credentials.
- Acceptance command after credentials are provided:

```powershell
$env:GENESIS_LIVE_PROVIDER = "1"
$env:GENESIS_PROVIDER_BASE_URL = "<provider chat-completions base URL>"
$env:GENESIS_PROVIDER_MODEL = "<model>"
$env:GENESIS_PROVIDER_API_KEY = "<Genesis-owned API key>"
D:\software\Go\bin\go.exe test ./...
```

- Acceptance evidence required: the live smoke `TestLiveOpenAICompatibleProviderThroughKernel` passes, `D:\software\Go\bin\go.exe build -o $env:TEMP\genesisd.exe .\cmd\genesisd` passes, and a route scan shows no versioned kernel route contracts.
- Residual risk: fake and httptest provider coverage proves kernel control flow but not an external provider account, rate limit, or model behavior.

### recvndyUquaZ5z - P1 - Repo issue and retirement record sync

- Status: in_progress.
- Type: architecture.
- Problem: multiple Feishu Base issues are already `ready_for_acceptance`, but the repo did not have a durable active issue ledger or retirement log. Feishu can coordinate review, but repo docs must preserve the authoritative issue status and verification evidence.
- Fix direction: add this active issue ledger and `docs/operations/kernel-v2-retirement-log.md`, then keep Base `已同步到 repo` true only when a corresponding repo record exists.
- Acceptance evidence required: this file lists remaining active issues, the retirement log lists every current `ready_for_acceptance` issue with commits and verification evidence, and `rg` can find the referenced issue ids or commits under `docs/`.
- Residual risk: this document is manual governance. Future issue movement still requires discipline: active issues leave this file only after their retirement record is written.
