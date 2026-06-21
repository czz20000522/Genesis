# Kernel Retirement Log

This file records Genesis Kernel issues that are ready for acceptance or retired. It is the repo-owned companion to `docs/operations/kernel-issues.md`.

## Retirement Rules

- `ready_for_acceptance` means the code and verification evidence are ready for user or operator acceptance, but the issue is not fully retired yet.
- `retired` means the user or operator accepted the evidence. A retired issue must be absent from `kernel-issues.md`.
- Every entry must include the issue id, title, fixing commits, verification evidence, residual risk, and retirement reason or retirement condition.
- If an entry is reopened, move it back to `kernel-issues.md` and mark this log entry as reopened with the reason.

## Ready For Acceptance

### recvnd2PDI1LuV - P0 - Minimal Go single-binary spike

- Status: ready_for_acceptance.
- Fix commits: `559e1c0c7`, `fd5bf9d8a`, `db9aeca13`, `22d5ca9f4`, `a9b34bda7`, `25e292b81`.
- Evidence: `go test ./...` passed; build passed; `GENESIS_LIVE_PROVIDER=1 go test ./internal/kernel -run TestLiveOpenAICompatibleProviderThroughKernel -count=1 -v` passed using Genesis `~/.genesis/config/models.json` and local `secret://...` credential resolution; binary `/ready` smoke returned `provider=openai-compatible` and `status=ok`; repository version-label scan returned no matches.
- Acceptance condition: reviewer confirms the spike proves a single Go binary with unversioned `/ready`, `/turn`, `/sessions/{id}`, fake provider mode, OpenAI-compatible provider mode, restart-safe ledger replay, and Genesis-owned live provider config.
- Residual risk: this is still a kernel spike, not a full product shell. Streaming, richer tool loop continuation, duplicate idempotency handling, and long-term storage policy remain future kernel work.

### recvnd2PDIz0sA - P0 - Minimal `shell.exec` tool runtime and permission gate

- Status: ready_for_acceptance.
- Fix commits: `924984712`, `6ae64ea5f`, `64aae83cb`, `ab04bf132`.
- Evidence: `go test -count=1 ./...` passed; live smoke covered controlled workspace write/read, alias escape blocked, absolute path escape blocked, environment access blocked, and junction CWD blocked.
- Acceptance condition: operator confirms `default` is a kernel-controlled command set, not an OS-level sandbox, and `yolo` is the only raw OS shell mode.
- Residual risk: the controlled default command set is intentionally narrow and must be extended only with path/effect/redaction tests.

### recvnd2PDIKruI - P0 - Minimal accumulation loop

- Status: ready_for_acceptance.
- Fix commits: `730445409`, `1234f89d4`, `15c320ac0`.
- Evidence: `go test -count=1 ./...` passed; build passed; live smoke covered candidate create/list/read/approve/restart/recall; `TestHTTPMemoryCandidateListAndReadAfterRestart` passed.
- Acceptance condition: user verifies pending candidates are reviewable, approval evidence is recorded, and only approved candidates are recalled.
- Residual risk: recall is intentionally simple text matching; vector search and richer policy are future work, not phase-one retirement blockers.

### recvnd2PDIoXVt - P0 - Unified event stream and restart-safe ledger

- Status: ready_for_acceptance.
- Fix commits: `559e1c0c7`, `924984712`, `730445409`, `6ae64ea5f`, `8534adff8`, `15c320ac0`.
- Evidence: `go test -count=1 ./...` passed; provider failure projection is `failed/provider_unavailable`; memory pending list/read is restart-safe; turn recall source points to the candidate `source_ref`.
- Acceptance condition: restart after turn, tool, memory candidate, and approval events reconstructs session and operation projections.
- Residual risk: ledger is append-only JSONL for the spike; compaction, migration, and long-term storage policy remain future kernel work.

### recvndgCmpUUTp - P0 - Memory pending queue and source evidence

- Status: ready_for_acceptance.
- Fix commits: `1234f89d4`, `15c320ac0`.
- Evidence: missing `source_ref` create returns 400; missing approval evidence returns 400; restart-safe `GET /memory/candidates?status=pending` returns only pending items; `GET /memory/candidates/{id}` exposes approval evidence; unknown status returns 400; missing read returns 404; recall source points to `source_ref`.
- Acceptance condition: reviewer confirms the memory candidate queue is auditable without knowing a source session id.
- Residual risk: no reject/supersede path exists yet; approval-only is the minimal closed loop.

### recvndhZ7RZDvd - P0 - Provider failure must not leave running turns

- Status: ready_for_acceptance.
- Fix commit: `8534adff8`.
- Evidence: `TestHTTPReportsBlockedProvider` passed; live smoke with missing provider base URL returned `/ready=blocked`, `POST /turn=503`, and session projection status `failed` with error `provider_unavailable`.
- Acceptance condition: provider admission or call failure always records a terminal failed state or rejects before admission.
- Residual risk: provider retry/degradation policy is not implemented yet; this retirement only covers terminal ledger correctness.

### recvndhZ7RcTsM - P0 - `shell.exec` default alias workspace escape

- Status: ready_for_acceptance.
- Fix commits: `6ae64ea5f`, `ab04bf132`.
- Evidence: `go test -count=1 ./...` passed; live smoke showed workspace-internal controlled write/read completed, while alias escape, absolute path escape, env access, and junction CWD were blocked.
- Acceptance condition: reviewer confirms default mode is a controlled command set and no request body can self-authorize permission mode or workspace root.
- Residual risk: this is not an OS sandbox. Any future default command must prove real-path containment before execution.

### recvndkw7apwxx - P1 - Shell evidence secret redaction

- Status: ready_for_acceptance.
- Fix commit: `64aae83cb`.
- Evidence: `go test -count=1 ./...` passed; live smoke showed command/stdout entries containing fake API key, bearer token, and JSON `api_key` were replaced with `[REDACTED]` in response and session projection.
- Acceptance condition: reviewer confirms default projections do not expose raw secret-shaped evidence.
- Residual risk: bounded raw evidence access is not designed yet; projections must remain redacted by default.

### recvndkw7abn2e - P1 - `shell.exec` default is not OS-level sandbox

- Status: ready_for_acceptance.
- Fix commit: `ab04bf132`.
- Evidence: README states default does not invoke an OS shell, expand env, or execute arbitrary interpreters; `go test -count=1 ./...` passed; live smoke blocked env access, alias escape, absolute escape, and junction CWD.
- Acceptance condition: documentation and tests agree that `default` is a controlled command set, while `yolo` is the only OS-shell mode.
- Residual risk: stronger sandboxing can be added later, but the current retirement is for not misrepresenting default as sandboxed.

### recvndkw7almZD - P1 - Memory source refs and approval evidence

- Status: ready_for_acceptance.
- Fix commit: `1234f89d4`.
- Evidence: `go test -count=1 ./...` passed; missing `source_ref` returns 400; missing approval reason/evidence returns 400; approved candidate projection includes source and approval evidence; consumer recall source uses `source_ref`.
- Acceptance condition: reviewer confirms memory approval has provenance and recall can point back to that provenance.
- Residual risk: reject/supersede and source deletion policies remain future Accumulation work.

### recvndkw7afapL - P2 - Provider adapter must not assemble memory context

- Status: ready_for_acceptance.
- Fix commits: `a93fc9d6f`, `db9aeca13`.
- Evidence: `go test ./...` passed; build passed; `TestKernelBuildsApprovedMemoryContextBeforeOpenAICompatibleProvider` proves approved memory context is assembled by the kernel/model context path before OpenAI-compatible provider transport.
- Acceptance condition: reviewer confirms provider adapters consume owner-built model input and do not own memory semantics.
- Residual risk: richer context policy may introduce more model-visible parts, but provider adapters must remain transport translators.

### recvndl0tmzxkL - P0 - Runtime token missing should block readiness

- Status: ready_for_acceptance.
- Fix commit: `5948d7ec5`.
- Evidence: `go test -count=1 ./...` passed; live smoke with no runtime token returned `/ready.status=blocked` and `runtime_auth.reason=runtime_token_missing`; configured token returned `/ready.status=ok`.
- Acceptance condition: readiness reflects whether protected routes can actually accept work.
- Residual risk: future readiness checks should remain aggregated and fail closed for required kernel planes.

### recvndyUquaZ5z - P1 - Repo issue and retirement record sync

- Status: ready_for_acceptance.
- Fix commits: `fed9d405a`, `83ff63fbe`.
- Evidence: active issue ledger exists at `docs/operations/kernel-issues.md`; ready/retirement evidence exists at `docs/operations/kernel-retirement-log.md`; README links both records; `rg` can find current active issue ids and all current `ready_for_acceptance` issue ids under repo docs.
- Acceptance condition: reviewer confirms Base `已同步到 repo=true` records have corresponding repo evidence and future retirements leave the active issue ledger.
- Residual risk: this is a manual governance guard. Future agents must update these docs whenever issue state changes.

### recvndAOsH7nn4 - P0 - Ledger unavailable must block readiness

- Status: ready_for_acceptance.
- Fix commit: `35c2111c0`.
- Evidence: `go test ./...` passed; build passed; `TestReadyBlocksWhenLedgerUnwritable` and `TestHTTPLedgerUnavailableBlocksReadyAndTurn` prove an unwritable ledger makes `/ready.status=blocked` with `ledger.reason=ledger_unwritable`, and `POST /turn` returns 503 `ledger_unwritable` rather than 400 `invalid_request`.
- Acceptance condition: reviewer confirms required persistence planes participate in readiness aggregation and persistence failure is not classified as caller input error.
- Residual risk: the current check proves the ledger path can be created/opened for append. It does not implement long-term disk-full prediction, ledger compaction, or malformed-ledger recovery.

### recvndDo1ECC5O - P1 - Corrupt ledger replay must block readiness

- Status: ready_for_acceptance.
- Fix commit: `9ad48a7fd`.
- Evidence: `go test ./...` passed; build passed; `TestHTTPCorruptLedgerBlocksReadyReplayAndAppend` proves a corrupt JSONL ledger makes `/ready.status=blocked` with `ledger.reason=ledger_corrupt`, and `/turn`, `/sessions/{id}`, and `/memory/candidates` return 503 `ledger_corrupt` rather than `ledger_unwritable` or `invalid_request`.
- Acceptance condition: reviewer confirms ledger readiness covers both appendability and replayability, and append paths refuse to write into a corrupt ledger.
- Residual risk: the kernel detects corrupt replay state but does not yet provide a repair, quarantine, or export workflow.

## Retired

No issue has been user-retired in this branch yet. Move accepted entries from `Ready For Acceptance` to this section only after user or operator acceptance, then remove the same issue from `kernel-issues.md`.
