# Production Gap Total Goal

- Status: active temporary implementation plan
- Owner: implementation coordination only
- Retirement target: delete this document after the goal is complete and the durable requirements, designs, issues, and retirement evidence have absorbed the final state.

## Operating Rule

This goal reduces production gaps without inventing unapproved architecture. Implement only slices that are already bounded by approved requirements and designs. When a gap lacks an approved boundary, add or update requirement/design/contract tests and stop before production implementation.

User issue pressure tests can continue while this goal is active. New issue-ledger changes are checked at phase boundaries. If a new issue is unrelated to the current phase, it stays queued until the goal completes. After this goal completes, create an hourly issue-repair automation.

## Global Constraints

- Do not add DB schema.
- Do not add application-domain owners to kernel.
- Do not add versioned runtime identifiers.
- Do not preserve new and old paths as long-term compatibility.
- Do not put Feishu, mail, WeChat, calendar, WebUI, or desktop UI into kernel.
- Do not turn unsettled design questions into temporary production behavior.
- Do not expose credentials, profiles, process ids, signals, host handles, permission modes, sandbox profiles, approval policies, or connector internals to the LLM as authority.

## Phase 0: Baseline And Reference Scan

Inputs:

- `docs/operations/kernel-issues.md`
- `docs/operations/application-issues.md`
- local Codex checkout: `D:\software\JetBrains\python_workspace\codex-main`
- local Reasonix checkout: `D:\software\JetBrains\python_workspace\reasonix`

Tasks:

- Confirm the current branch contains the latest issue-ledger commit.
- Run baseline verification: `go test ./... -count=1`, `go build ./...`, and `git diff --check`.
- For each following phase, inspect Codex and Reasonix for the relevant implementation pattern before editing.
- Record the reference scan in this document until a phase creates a durable requirement/design update.

Done when:

- Baseline commands pass or failures are classified as pre-existing.
- Reference scan entries exist for the next implementable phase.

Evidence:

- Latest issue-ledger commit is present: `054688f63 Record connector driver migration gap for production handoff`.
- Baseline passed before Phase 1 edits: `go test ./... -count=1`, `go build ./...`, and `git diff --check`.
- Phase 1 reference scan:
  - Codex uses `tempfile::TempDir` / `tempfile::tempdir()` broadly in tests. That matches Rust test isolation, but it does not satisfy Genesis's stricter repo-local test artifact rule.
  - Reasonix uses `t.TempDir()` / `os.MkdirTemp()` broadly, including a Windows cleanup helper in `internal/boot/temphelper_test.go`. That is useful as a cleanup idea, not as Genesis's policy.
  - Genesis already has `testsupport.ProjectTempDir` and `docs/process.md` defines the Test Artifact Gate. Phase 1 therefore centralizes kernel/command fixtures on repo-local `.test-tmp/go` and adds a guard against regressing to system temp.

## Phase 1: Test Artifact Locality

Issue:

- `KERNEL-TEST-ARTIFACT-LOCALITY-20260624`

Target:

- Migrate `internal/kernel`, `cmd/genesisd`, and `cmd/genesisctl` test artifacts away from system temp and into repo-local `.test-tmp/` through `testsupport.ProjectTempDir` or an equivalent helper.
- Add a structural guard blocking new system-temp fixture usage in kernel/application tests.

Non-goals:

- No subjective log/prose tests.
- No migration of historical local artifacts.
- No C-drive scratch directories.

Done when:

- Tests prove writable fixtures land under project-local test scratch.
- `go test ./... -count=1`, `go build ./...`, and `git diff --check` pass.

Evidence:

- Completed by `85cbb091d`.
- `internal/kernel`, `cmd/genesisd`, and `cmd/genesisctl` tests now use repo-local project temp helpers.
- `internal/testsupport` guards those test roots against new `t.TempDir()` or `os.MkdirTemp()` fixture usage.

## Phase 2: Connector Outbound Driver Migration

Issue:

- `APP-CONNECTOR-DRIVER-MIGRATION-20260625`

Target:

- Move Feishu final delivery and probe default behavior to `ConnectorCommandAdapter`.
- Keep `command_template` only as explicit smoke fallback, or retire it if no active approved consumer remains.
- Runtime must not treat `lark-cli im +messages-send` as a production default protocol.

Non-goals:

- No Feishu owner in kernel.
- No model-visible credential or profile.
- No long-term dual-path compatibility.

Done when:

- Default production path is connector-command based.
- Any remaining command-template path has an explicit retirement target and opt-in smoke scope.

Reference scan:

- Codex `shell-escalation` keeps Unix escalation protocol ownership inside the protocol layer and gives callers a narrow `ShellCommandExecutor`; callers keep process/sandbox capture without rewriting the escalation protocol.
- Reasonix `internal/plugin` keeps external tool servers behind configured stdio/HTTP transports and JSON-RPC, with the harness seeing registered tools rather than hardcoded vendor commands.
- Genesis alignment: `ConnectorAction` stays the stable semantic contract; the Feishu adapter process owns `lark-cli im +messages-send` and vendor output parsing. `genesis-ingress` may configure a connector adapter executable, but it must not render Feishu message-send argv itself.

Evidence:

- Completed by `c69928afe`.
- Feishu final delivery now configures `ConnectorCommandAdapter`, and the user-space `genesis-feishu-connector-adapter` owns the lark-cli send-message syntax.
- The transitional command-template driver is marked as an explicit local smoke fallback and guarded out of `genesis-ingress`.

## Phase 3: Connector Credential/Profile Readiness

Issue:

- `APP-CONNECTOR-FEISHU-LISTENER-20260623`

Target:

- Add source/final-delivery readiness classification for `missing_profile`, `profile_expired`, `permission_denied`, and `refresh_required`.
- Fail closed before source start or final delivery when readiness blocks.

Non-goals:

- No credential broker.
- No automatic credential refresh.
- No credential/profile exposure to LLM.

Done when:

- Missing/expired/denied/refresh-required cases are classified before external source or delivery effects.
- Connector state records readiness without writing kernel facts.

## Phase 4: Reconciliation Probe Requirement And Design

Issue:

- `APP-CONNECTOR-OPERATOR-CONSOLE-20260623`

Target:

- Define connector-specific reconciliation probes only after stable `external_action_ref`, delivery receipt, idempotency key, or exact external receipt refs exist.

Non-goals:

- No blind chat-history search.
- No resend as reconciliation.
- No kernel fact mutation.

Done when:

- Requirement/design describe probe input, evidence, terminal outcomes, and fail-closed behavior.

## Phase 5: Sandbox / Approval Requirement And Contract Tests

Issue:

- `KERNEL-SANDBOX-APPROVAL-NEXT-20260623`

Target:

- Add or refine requirement/design/contract tests for real OS sandbox and interactive approval owner boundaries.

Non-goals:

- No real OS sandbox implementation until a concrete enforcement adapter is selected.
- No interactive approval implementation until owner commands, typed events, approval result binding, and model-invisible control fields are defined.

Done when:

- Contract tests prove current fail-closed semantics and future implementation boundaries without pretending sandbox/approval is production-complete.

## Phase 6: Foreground Attach Executor Requirement And Contract Tests

Issue:

- `KERNEL-JOB-PROGRESS-IDLE-CONTINUATION-20260623`

Target:

- Define attach-capable executor adapter contract and negative tests.

Non-goals:

- No fake attach facts.
- No exposure of process id, host signal, or process handle to model or HTTP caller.
- No replacement of the current truthful kill fallback until an executor can attach.

Done when:

- The current kill fallback remains truthful.
- Future attach-capable executor expectations are explicit and testable.

## Phase 7: Generic Skill Hydration / Context Resource Requirement

Target:

- Write requirement/design for on-demand skill/context hydration through generic resource/context primitives.

Non-goals:

- No full skill body in every prompt.
- No `read_skill`, `skill.read`, or other skill-specific kernel tool.

Done when:

- The approved path uses generic resource/context contracts and keeps skill packages as user-space assets.

## Phase Closure

Before closing the goal:

- Re-read kernel and application issue ledgers.
- Remove completed issues from active ledgers and add compact retirement evidence.
- Delete or retire this temporary plan.
- Run `go test ./... -count=1`, `go build ./...`, and `git diff --check`.
- If issue ledgers changed while this goal was active, create an hourly automation to continue issue repair after goal completion.
