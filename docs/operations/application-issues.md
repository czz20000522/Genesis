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

- Status: blocked_no_real_workflow_available.
- Requirement: `docs/applications/workflow-runtime-requirement.md`.
  - Design: `docs/applications/workflow-runtime-design.md`.
  - Closure: the user-space compiler now accepts only declared JSON/YAML graph
    config, rejects executable fields, unknown outcomes, undeclared targets,
    and unbounded cycles, then derives a canonical definition hash and Mermaid
    flowchart. It has no runner, persistence, scheduler, or kernel client.
  - Remaining gap: no real repeated workflow has been selected. Video transcript
    extraction remains a shared Skill or future Workflow; test graphs are not
    product workflow evidence.
  - Product decision: defer Workflow. Do not add a runner, scheduler, or
    synthetic example workflow. Resume only after a real repeated process has
    named steps, terminal outcomes, artifact contract, and approval points.
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
  `models.json` mutation, protected credential writes, and safe profile
  projection. Desktop role binding and sidecar restart are retired from this
  provider-control surface; ordinary selection remains session-scoped. An
  empty Home is a first-run state rather than an error:
  the desktop offers the curated DeepSeek, OpenAI, OpenCode Go, local
  llama.cpp, and explicit OpenAI-compatible templates, clears one-shot keys
  after submission, reloads safe profiles, and calls the existing read-only
  adapter verification diagnostic. After a successful first import it creates
  one empty durable Chat when no session exists; the imported model remains
  unbound until the user selects it for that session. It does not bind or
  restart automatically.
- Remaining acceptance: use real local Qwen, DeepSeek, and OpenCode Go GLM
  profiles from the desktop; verify each profile and select one cloud profile
  in a session while confirming a prior session remains readable and unchanged.
- Evidence: the existing desktop boundary test rejects direct kernel imports;
  `genesisctl provider` already proves the intended local config and secret
  store format.
- Verification: focused shared-owner, kernel diagnostic, desktop backend, and
  frontend tests; root and desktop Go builds; frontend type/build checks. The
  user-directed manual live desktop acceptance remains pending.
- Reference alignment: Reasonix separates settings mutation from its runtime
  controller; Codex keeps provider selection outside turns. Genesis aligns
  while intentionally limiting the first UI to preconfigured profiles.
- First-run evidence: `localconfig` and desktop bridge tests prove safe profile
  creation without a secret projection; the configured DeepSeek Flash live
  acceptance proves `deepseek-v4-flash`, settled turn completion, and restart
  replay. The installed empty-Home click path remains manual_test_pending.
  Arbitrary endpoint behavior is limited to the explicit advanced template;
  marketplace behavior stays out of scope.

### APP-DESKTOP-SESSION-MODEL-AND-IMPORT-20260713 - P1 - Session selection and curated cloud import need installed-desktop acceptance

- Status: manual_test_pending.
- Requirement: `docs/requirements/kernel-session-model-binding.md` and
  `docs/requirements/desktop-provider-onboarding.md`.
  - Design: `docs/design/kernel-session-model-binding.md`.
  - Closure: the kernel persists an append-only session profile binding and
    resolves one provider per turn; the desktop selector changes only the
    current session. Curated cloud routes are saved before route-level model
    discovery and profiles are materialized only after discovery succeeds.
    No ordinary chat writes a global coordinator binding or restarts the
    kernel. A sidecar with a session resolver is runtime-ready even while no
    session has selected a profile.
  - Evidence: a live DeepSeek Flash turn returned
    `GENESIS_SESSION_MODEL_PROOF_OK`; after a daemon restart, sessions bound
    to `deepseek-flash` and `opencode-go-deepseek-v4-flash` retained their
    distinct profile ids and the settled DeepSeek timeline remained readable.
  - Remaining acceptance: use the installed desktop UI to import a new cloud
    route, select it independently in two Project/Task/Chat sessions, and
    prove the visible selector and failure recovery. Local llama.cpp remains
    separate: the installer packages its provider-command adapter under the
    private kernel runtime, while its desktop-owned lifecycle is accepted by
    the dedicated local-model issue below.

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
    the same owned-stop path at client shutdown. Before launch, one configured
    health request prevents competing with an already-serving endpoint; that
    endpoint remains unowned and cannot be stopped by Genesis. No systemd or
    process-discovery path was added.
- Evidence: both verified llama user-systemd unit files were removed on
  2026-07-11; direct WSL start reached `/health` and an owned-tree stop left no
  llama process; focused ownership tests and desktop/frontend builds pass.
- Verification: focused desktop ownership tests, desktop build, direct WSL
  start/health/stop proof, and a scan proving no llama systemd units remain.
- Reference alignment: aligns with Codex owned-process-tree termination and
  Reasonix session-owned process handles; intentionally rejects background
  service discovery and automatic restart.

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
  An isolated live DeepSeek Flash check additionally bound one Project, one
  Task, and one Chat, produced reasoning and final output for each, then
  restarted the same daemon and reread all three bindings and timelines. This
  does not replace installed-desktop or local-Qwen acceptance.
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
