# Application Issue Ledger

This file records active issues for user-space applications that exercise the
Genesis Kernel. Kernel primitive gaps belong in
`docs/operations/kernel-issues.md`.

## Ledger Rules

- Application issues must cite an approved application requirement and design
  unless they are obvious bugs or test gaps.
- Do not record kernel work here. If a gap requires a new kernel primitive,
  create or update the relevant kernel requirement/design/issue.
- Completed issues leave this ledger and move to application retirement evidence
  when such a log is needed. Retirement evidence is compact: one sentence with
  the retirement conclusion plus fixing commit evidence, not a repeated fix
  summary or verification transcript.
- Temporary or narrow implementation slices must name their retirement target.
  Once the approved primitive exists, retire the temporary package, tests, docs,
  and command references instead of preserving a second truth surface.
- Every application issue or next slice must name the kernel primitive or owner
  capability it pressure-tests. If the slice begins faking facts that belong to
  kernel, connector, memory, credential, permission, job, audit, or provider
  owners, stop the app slice and move the gap to the owning issue ledger.
- Issues should stay small: requirement, design, gap, next slice, evidence,
  verification, and reference alignment.

## Active Issues

### APP-WORKFLOW-RUNTIME-PHASE-A-20260711 - P1 - Fixed workflow contract has no runtime

- Status: in_progress.
- Requirement: `docs/applications/workflow-runtime-requirement.md`.
  - Design: `docs/applications/workflow-runtime-design.md`.
  - Closure: the user-space compiler now accepts only declared JSON/YAML graph
    config, rejects executable fields, unknown outcomes, undeclared targets,
    and unbounded cycles, then derives a canonical definition hash and Mermaid
    flowchart. It has no runner, persistence, scheduler, or kernel client.
  - Remaining gap: no real repeated workflow has been selected. Video transcript
    extraction remains a shared Skill or future Workflow; test graphs are not
    product workflow evidence.
  - Next slice: after a real repeated process is selected with steps, terminal
    outcomes, artifact contract, and approval points, add the Phase B runner
    against that contract.
  - Reference alignment: Reasonix confirms transport-neutral durable run and
    approval ownership; Codex has thread operations but no comparable fixed
    workflow owner. Genesis keeps workflow state separate from both.

### APP-USER-CAPABILITY-PACKAGES-20260711 - P1 - Capability mechanism awaits a real user package

- Status: blocked_no_real_package_available.
- Requirement: `docs/requirements/user-capability-package.md`.
  - Design: `docs/applications/user-capability-package-design.md`.
  - Closure: manifest inspection is shared by `genesisctl` and `genesisd`;
    startup maps only safe descriptors into existing kernel discovery. Package
    execution remains user-space behavior.
  - Remaining acceptance: no real user-owned package is installed. Video
    transcript extraction remains a shared Skill or future Workflow, and no
    report-generation capability exists. Do not fabricate a demo package.
  - Next slice: when a real package exists, prove its list/doctor/run and
    discovery path; then repeat with a second distinct real package.
  - Reference alignment: follows Codex safe skill discovery snapshots and
    Reasonix safe capability inspection while rejecting installation and
    kernel-owned package execution.

### APP-DESKTOP-SESSION-RECOVERY-AND-SEARCH-20260711 - P1 - Search and retry await live acceptance

- Status: manual_test_pending.
- Requirement: `docs/requirements/desktop-session-recovery-and-search.md`.
  - Design: `docs/design/desktop-session-recovery-and-search.md`.
  - Closure: the rail now renders existing kernel search results and restores
    normal Project / Task / Chat groups when cleared. A turn whose processing
    projection has terminal outcome `failed` exposes one explicit retry using
    its same-turn user text and a fresh desktop idempotency key.
  - Remaining acceptance: search a persisted session and retry one real
    deterministic provider failure; confirm the original failure remains in
    the timeline and that paused/interrupted/approval paths show no retry.
  - Reference alignment: follows Codex server-owned thread search and rejects
    provider/transport auto-retry as a desktop responsibility.

### APP-DESKTOP-PROVIDER-CONTROL-20260711 - P1 - Desktop provider control awaits live acceptance

- Status: manual_test_pending.
- Requirement: `docs/requirements/desktop-provider-control.md`.
  - Design: `docs/design/desktop-provider-control.md`.
  - Kernel/owner pressure: desktop owns local configuration and credential
    mutation; the kernel owns provider resolution and turn execution. The
    shared local configuration owner must not become a provider-context or
    ledger owner.
- Closure: a shared `localconfig` owner now backs kernel/CLI and desktop
  `models.json` mutation, protected credential writes, safe profile projection,
  and role binding. The desktop provides a compact profile panel, clears the
  one-shot key input after submission, calls a read-only adapter verification
  diagnostic, and restarts only its owned `genesisd` sidecar.
- Remaining acceptance: use real local Qwen, DeepSeek, and OpenCode Go GLM
  profiles from the desktop; verify each profile and switch one cloud profile
  through an owned restart while confirming a prior session remains readable.
- Evidence: the existing desktop boundary test rejects direct kernel imports;
  `genesisctl provider` already proves the intended local config and secret
  store format.
- Verification: focused shared-owner, kernel diagnostic, desktop backend, and
  frontend tests; root and desktop Go builds; frontend type/build checks. The
  user-directed manual live desktop acceptance remains pending.
- Reference alignment: Reasonix separates settings mutation from its runtime
  controller; Codex keeps provider selection outside turns. Genesis aligns
  while intentionally limiting the first UI to preconfigured profiles.

### APP-DESKTOP-LOCAL-MODEL-LIFECYCLE-20260711 - P1 - Local llama.cpp must be client-owned, not a WSL service

- Status: manual_test_pending.
- Requirement: `docs/applications/desktop-local-model-lifecycle-requirement.md`.
  - Design: `docs/applications/desktop-local-model-lifecycle-design.md`.
  - Gap: WSL contained legacy llama.cpp user-systemd units, while desktop only
    owns a genesisd sidecar. The user needs the desktop to own exactly the
    WSL llama.cpp process it starts, allow manual stop for GPU reuse, and stop
    that process on client exit without affecting external GPU work.
  - Closure: desktop now launches the configured WSL command directly, retains
    only that `wsl.exe` handle, provides manual start/stop control, and calls
    the same owned-stop path at client shutdown. No systemd or process
    discovery path was added.
- Evidence: both verified llama user-systemd unit files were removed on
  2026-07-11; direct WSL start reached `/health` and an owned-tree stop left no
  llama process; focused ownership tests and desktop/frontend builds pass.
- Verification: focused desktop ownership tests, desktop build, direct WSL
  start/health/stop proof, and a scan proving no llama systemd units remain.
- Reference alignment: aligns with Codex owned-process-tree termination and
  Reasonix session-owned process handles; intentionally rejects background
  service discovery and automatic restart.

### APP-FIRST-RUN-PROVIDER-COMMAND-ACCEPTANCE-20260710 - P1 - First-run acceptance skips the configured local provider path

- Status: manual_test_pending.
- Exception: obvious acceptance-test and runbook gap. This issue does not add a
  provider capability or change the kernel provider contract.
  - Kernel/owner pressure: the model gateway accepts provider output only through
    the configured provider adapter; provider commands own local wire-format
    translation before a result reaches the kernel.
- Gap: `scripts/first_run_live_llm_acceptance.ps1` constructs a generic
    OpenAI-compatible profile from `-BaseUrl`, `-Model`, and an API key. The
    configured local Qwen route uses `provider_command`, which maps llama.cpp
    `reasoning_content` into a canonical reasoning message. A new
    `-UseConfiguredProfile` path now selects the configured route without
    fabricating an API key; the remaining local-Qwen gap is the explicit
    unbounded-request kernel contract, because no output ceiling or outer
    deadline may truncate a valid local answer.
- Dependency: `KERNEL-PROVIDER-REASONING-MESSAGES-20260710` must first provide
  the canonical reasoning path. The acceptance runbook must prove that path
  rather than forcing the local adapter through a generic OpenAI-compatible
  profile.
- Next slice: after
  `KERNEL-LOCAL-PROVIDER-UNBOUNDED-20260711` provides the explicit local
  no-deadline contract, run the configured Qwen acceptance path without a
  generated output cap. Do not make the generic OpenAI-compatible provider
  discard unknown vendor fields or invent an API key for a provider command.
- Evidence: `-UseConfiguredProfile` passed end-to-end against an isolated
  deterministic provider command: provider verify, final turn, restart replay,
  and a missing-config `provider_unavailable` probe all passed without an API
  key. On 2026-07-11, real Qwen reached the configured command but generated
  unbounded reasoning until the verify deadline, producing structured
  `provider_error`; its WSL launcher was then stopped. On 2026-07-12, the
  same isolated acceptance surface passed against configured OpenCode Go GLM
  5.2 (`coder` / `opencode-go-glm-5-2`): provider verify was ready, the final
  was `GENESIS_LIVE_LLM_ACCEPTANCE_OK`, restart replay retained one timeline
  item, three events, and ready context, and the missing-config probe returned
  `provider_config_missing` / `provider_unavailable`. This closes the Stage 1
  real-provider proof, not the pending local-Qwen acceptance.
- Verification: The selected acceptance surface must execute a real turn,
  restart against the same ledger, replay the settled projections, and retain a
  negative readiness or credential-path proof appropriate to its provider kind.
- Reference alignment: This preserves the existing provider boundary: the
  OpenAI-compatible adapter rejects unrecognized vendor response fields, while
  `provider_command` owns adapter-specific translation. No connector, desktop,
  or kernel truth owner should translate or suppress that field.

### APP-DESKTOP-PERSISTENT-SESSION-WORKSPACES-20260711 - P1 - Local-model acceptance is incomplete after Project/Task/Chat entry implementation

- Status: manual_test_pending.
- Requirement: `docs/requirements/kernel-session-workspace-binding.md`.
  - Design: `docs/design/kernel-session-workspace-binding.md`.
  - Kernel/owner pressure: `KERNEL-SESSION-WORKSPACE-BINDING-20260711` must
    first own the session-to-workspace authority. Desktop may catalogue and
    render projects but may not supply arbitrary cwd values to tools.
- Gap: the desktop now offers Project directory selection, per-session Task
  directories, persistent Chat entries, and a locally persisted session
  catalog. The unclosed evidence gap is three settled local-Qwen turns: the
  Project path produced reasoning plus a final, while capped Task validation
  correctly failed because llama.cpp returned only reasoning and no final.
- Next slice: add the normal-turn cancellation/settlement evidence needed for
  an intentionally unbounded local model, then repeat all three desktop entry
  turns without introducing a default output cap. Material upload remains an
  attachment feature, not a project model.
- Evidence: The approved initial desktop task is “read this repository and say
  what it does”; it operates on a Project binding and current local files. The
  UI build, typed bridge, session catalog, and kernel binding tests pass.
- Verification: Project shared-directory/independent-history, Task
  independent-persistent-directory, Chat local-transcript/no-workspace, and
  restart acceptance tests in `docs/implementation-plans/desktop-session-workspaces.md`.
- Reference alignment: Codex presents project and standalone conversation
  entry points while retaining thread history; Reasonix makes session cwd an
  explicit controller input. Genesis intentionally retains local chat records
  even when the model call is cloud-backed.

### APP-DESKTOP-TURN-INTERRUPT-20260712 - P1 - Desktop cannot stop an unbounded local turn

- Status: manual_test_pending.
- Requirement: `docs/requirements/desktop-session-recovery-and-search.md`.
  - Design: `docs/design/desktop-session-recovery-and-search.md`.
  - Kernel/owner pressure: the kernel already owns
    `POST /sessions/{session_id}/interrupt`, active-turn cancellation, and
    durable `assistant.interrupted` evidence. Desktop must request that
    command, not treat a cancelled frontend stream as a settled turn.
- Gap: an explicitly unbounded llama.cpp turn can run legitimately for a long
  time, but the desktop exposes no stop action even though the kernel supports
  caller-driven interruption.
- Closure: the composer exposes `停止生成` only while its current stream is
  active. It calls the existing authenticated kernel route through the shared
  desktop API client; an interrupted stream reloads the kernel timeline, while
  a `no_active_turn` race does not forge a local terminal message.
- Remaining acceptance: start the desktop-owned local Qwen process, submit a
  deliberately long Project, Task, and Chat turn, stop each from the composer,
  and confirm their durable interrupted projections remain readable after
  desktop restart.
- Reference alignment: Codex's app-server owns `turn/interrupt` and terminal
  interruption notifications; Reasonix's desktop delegates cancellation to its
  controller. Genesis follows both by retaining interruption truth in kernel
  events and projections.

### APP-CONNECTOR-FEISHU-LISTENER-20260623 - P2 - Connector source verification and lifecycle gate

- Status: open.
- GitHub mirror: #37 tracks this remaining source lifecycle production gap in
  the external issue queue; keep the detailed contract here until the lifecycle
  requirement/design changes.
- Requirement:
  `docs/applications/connector-source-verification-lifecycle-requirement.md`;
  broader connector contract:
  `docs/applications/application-connector-runtime-requirement.md`.
- Design: `docs/applications/connector-source-verification-lifecycle-design.md`;
  broader connector design:
  `docs/applications/application-connector-runtime-design.md`.
- Kernel/owner pressure: connector source verification, inbound dedupe, session
  mapping, turn submission, and connector outbox delivery without kernel Feishu
  ownership.
- Gap: The smoke path now has deterministic ExternalEvent NDJSON ingestion, a `source_command` typed streaming boundary for inbound source adapters, a Feishu source adapter command that owns `lark-cli event consume` and raw Feishu payload parsing, durable connector-local source failure records with redacted diagnostics, connector-local `SourceRun` / `SourceAttempt` / `SourceCursor` / `SourceVerificationEvidence` state, bounded generic retry/backoff for recoverable `source_command` process failures, connector-local operator lifecycle controls, an operator-run `genesis-ingress feishu-probe` source-command readiness report, and an opt-in `--deliver-final` path that turns kernel final text into one connector-owned send-message outbox item. Feishu CLI executable selection and Feishu probe composition now live in the Feishu application package/cmd layer rather than the generic connector runtime. Real source events still default to `unchecked` because source readiness does not imply event verification. Profile readiness blocks source start and final delivery with `missing_profile`, `profile_expired`, `permission_denied`, or `refresh_required`; probe mode reports readiness without starting source adapters, sending messages, or calling kernel; and `--profile-probe-command` can now classify profile posture through a typed connector-local command before source/delivery adapters start. Local verification of `lark-cli --profile codex event consume --help` shows no `--after-event-id` resume flag, so cursor-aware restart cannot be implemented by hardcoding a nonexistent Feishu CLI parameter. Remaining production gap is automatic refresh posture integration and production-grade source recovery beyond bounded retry/backoff.
- Next slice: Keep inbound source intake through `source_command` and keep Feishu event argv in the Feishu source adapter. Product smoke commands must pass the Genesis profile (`--profile genesis`) for the Genesis bot identity; Codex developer-originated test messages may still use the Codex profile as the sender. Future source verification work must add real event authenticity evidence rather than upgrading readiness to verification. Cursor resume must be adapter/driver capability-gated through an actual lark-cli parameter, adapter manifest, or upstream source protocol; until then the cursor is reliable dedupe/recovery evidence, not a promised replay control. Connector-specific reconciliation probes should wait until exact outbound action refs, idempotency keys, or external receipt refs exist; reconciliation remains outbound recovery work.
- Current implementation slice: add a Feishu adapter `profile-probe` mode that
  translates only structured `lark-cli auth status` evidence into the generic
  profile-readiness command result. It may prove `missing_profile` and a ready
  bot identity; unknown upstream states fail closed as
  `operator_action_required` until an upstream typed refresh or permission
  contract is available.
- Current configuration slice: make real Feishu listener startup require the
  user-home connector binding's explicit enabled switch and consume its typed
  profile/identity values before source start. The generic runtime remains
  adapter-neutral; a later mail, WeChat, or QQ adapter may reuse the
  enablement/lifecycle boundary without forcing identical vendor settings.
- Evidence: `genesis-ingress feishu-listen --stdin-jsonl ...` covers automated smoke and dedupe without a source process, with synthetic source validation remaining `unchecked` unless evidence exists. Non-stdin source intake now starts a configured `SourceCommandAdapter` process and consumes typed `source.ready`, `source.event`, `source.cursor`, `source.failed`, and `source.stopped` frames through the source command intake loop, which retries only recoverable runtime failures and does not retry blocked readiness/configuration failures. Verified source events require source/connector/adapter/event-bound evidence with an approved evidence kind before they are handled or recorded. Blocked/degraded source runs carry stable readiness reason codes for source command invalid/runtime failure while preserving operator-readable detail; profile readiness now also blocks source start or final delivery with `missing_profile`, `profile_expired`, `permission_denied`, or `refresh_required` before any external effect. Connector-local `source-clear-blocked`, `source-request-restart`, and `source-reset-cursor --accept-duplicate-risk` record operator source actions without creating kernel facts or fabricating ready state. `cmd/genesis-feishu-source-adapter` owns Feishu event command construction and raw Feishu payload parsing, requires an explicit `--profile`, and emits typed source frames instead of giving runtime raw payloads or Feishu command details. On 2026-07-11, the installed CLI's `event consume --help` confirmed that it lacks `--after-event-id`; source and ingress command assembly now keep persisted cursors out of CLI argv while retaining them as connector-local dedupe/recovery evidence. Malformed source frames and malformed Feishu payloads create redacted `SourceFailureRecord` entries before kernel submission. `FileInboundStore`, `FileSourceFailureStore`, `FileSourceLifecycleStore`, and `FileOutboxStore` serialize file-backed load-modify-write operations across process-local instances during smoke use. `genesis-console inspect --source-lifecycle-state ...` projects connector-local source runs, attempts, cursors, verification evidence, and operator actions without kernel mutation. `genesis-ingress feishu-probe --source-command ... --delivery-command ... --profile genesis` validates the source adapter process, profile readiness posture, and connector-command final-delivery surface without starting the listener, sending a message, or calling kernel; `--profile-probe-command` now allows typed connector-local profile posture classification, and regression tests prove probe-reported `profile_expired` / `refresh_required`, explicit `ready=false`, and timed-out probes block before source or delivery adapter effects. The opt-in `--deliver-final` path uses connector outbox delivery rather than kernel Feishu logic.
- Current local evidence (2026-07-12): `lark-cli auth status --profile genesis`
  reports a ready bot identity and no user identity. An explicit
  `genesis-ingress feishu-probe --profile genesis` with the built Genesis
  source and delivery adapters returned ready without starting a listener or
  sending a message. This proves only local profile/adapter readiness; a real
  event remains `unchecked` until event-bound authenticity evidence exists.
- Verification: Remaining verification must prove automatic refresh posture integration and production listener supervision beyond bounded retry/backoff without creating a kernel Feishu owner.
- Reference alignment: Aligned with Reasonix ACP keeping protocol/session handling outside the controller and Codex app-server keeping client transport ids outside core turn truth.
