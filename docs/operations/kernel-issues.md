# Kernel Issue Ledger

This file is the repo-owned ledger for active Genesis Kernel issues. Feishu Base is the collaboration inbox; this file is the durable project source for issues that still need code, verification, or user acceptance.

Retired issues must not remain here. Move accepted retirements to `docs/operations/kernel-retirement-log.md` as compact evidence: one sentence with the retirement conclusion plus the fixing commit evidence.

## Ledger Rules

- Keep only `new`, `open`, `in_progress`, or otherwise active issues in this file.
- Do not record application-specific feature work as kernel work unless it changes a kernel primitive.
- Do not add versioned HTTP route names as current contracts. HTTP is transport; current kernel endpoints are unversioned.
- `ready_for_acceptance` issues move to the retirement log as retirement candidates and leave this active ledger.
- Feishu/Base links may point to collaboration artifacts, but this repo must contain enough evidence to understand the current status without opening Feishu.
- Every active `KERNEL-*` issue must include a `Reference alignment` field that compares the issue to Codex, Reasonix, or an explicitly rejected drift risk.
- Reference alignment uses local reference checkouts only. Do not cite Genesis GitHub repositories, remote issues, releases, or pull requests as authority for this local kernel line unless the user explicitly asks for external publishing context.
- Before a non-trivial implementation slice starts, the related implementation plan or issue must include a Codex/Reasonix reference scan. The scan should identify inspected references, learned control-plane semantics, intentional differences, and remaining drift risks.
- Reference scans must translate relevant Codex/Reasonix behavior tests into Genesis same-semantics red tests or explicitly reject them with a reason. A prose-only reference note is not enough for kernel primitives.
- Issues record the current gap between approved requirements/designs and the implementation. They must not carry raw requirements, design discussion, or the full production acceptance contract.
- Every active issue must cite an approved requirement and design unless it is an obvious bug or test gap. If an issue uses that exception, state the exception explicitly.
- Prefer `Gap`, `Next slice`, `Evidence`, and `Verification` over broad `Problem` or `Suggestion` text when adding new issues.
- Do not use issues as debug logs. Routine info, stream chunks, repeated status polling, and exploratory notes stay out of this ledger unless they identify a current implementation gap.
- When an issue removes a concept from the current kernel contract, long-term tests must assert the positive replacement behavior. Do not keep permanent tests whose only purpose is locking retired names, aliases, routes, or historical helper APIs out of the tree; use temporary scans or retirement-log evidence for cleanup windows, then fold the guard into the current owner contract.
- Development artifacts and historical local data are not compatibility obligations. Do not create or keep issues whose only purpose is migrating, cleaning, importing, or preserving old local generated state unless that state is part of the approved current kernel contract.
- Every implementation slice must finish with a drift check against the governing requirement, design, implementation plan, issue, and BDD feature. In-scope drift is fixed before commit. Out-of-scope drift is recorded here as an active issue with evidence and next slice before commit.
- Periodic governance review checks architecture, feature behavior, directory structure, and document lifetime together. Completed plans and stale documents should be deleted or condensed instead of spawning issues that only preserve old notes.

## Active Issues

### KERNEL-TASK-GRAPH-RUNTIME-20260712 - P1 - Long-running objectives lack a ledger-owned dependency graph

- Status: in_progress.
- Requirement: `docs/requirements/kernel-task-graph-runtime.md`.
  - Design: `docs/design/kernel-task-graph-runtime.md`.
- Gap: Phase A now persists and reconstructs a DAG with dependency-ready and
  blocked explanations, but it deliberately never starts work. The remaining
  production gap is owner-created invocation linkage from a ready parent
  role-task proposal.
- Next slice: Phase B only from
  `docs/implementation-plans/kernel-task-graph-runtime.md`: persist the
  role/task proposal, create/start linkage, terminal reduction, and fail closed
  on an ambiguous restart; do not introduce a generic scheduler.
- Evidence: role-bound workers now have bounded lifecycle facts, parent result
  delivery, recovery semantics, and route/profile/role/parent admission limits.
- Verification: DAG refusal/no-append, missing-reference refusal, readiness,
  blocked failure reason, terminal immutability, restart projection, then full
  Go tests and build.
- Reference alignment: Codex records explicit spawned-agent lifecycle; Reasonix
  runs bounded isolated tasks. Neither local reference supplies a durable DAG,
  so Genesis rejects an in-memory queue and keeps graph authority in the ledger.

### KERNEL-PARENT-WORKER-DELEGATE-20260712 - P1 - Parent-worker runtime lacks provider/profile admission and desktop acceptance

- Status: in_progress.
- Requirement: `docs/requirements/kernel-parent-worker-runtime.md`.
  - Design: `docs/design/kernel-parent-worker-runtime.md`.
- Gap: role-bound dispatch now resumes the parent after normal completion and
  restart recovery, fails closed for an ambiguous started worker, enforces role
  concurrency (default 6), and enforces parent `max_children` across roles
  (default 24). Desktop now lists the current session's worker projections and
  opens each child conversation only in its inspector; configured positive
  provider-route/model-profile limits now also participate in admission. Manual
  desktop acceptance remains.
- Next slice: complete manual desktop acceptance for the child-conversation
  inspector, then use real multi-role work to validate the bounded runtime; do
  not introduce a TaskGraph scheduler.
- Evidence: `delegate_worker` creates one role-bound `AgentInvocation`,
  snapshots its parent binding id and worker profile, resolves the configured
  provider through the daemon resolver, excludes recursive delegation from the
  worker manifest, and returns a bounded terminal result into the paused parent
  turn. Queued work recovers after restart and ambiguous started work records a
  sanitized failure instead of replaying it.
- Desktop evidence: the existing HTTP choke point reads only the session worker
  list and individual bounded child projection; the main transcript remains
  parent-only and the desktop does not write delegation facts.
- Verification: parent tool-loop pause and continuation, queued/terminal/ambiguous
  restart recovery, role/profile resolver, leaf grant filtering, role and
  parent concurrency limits, then full Go tests and build.
- Reference alignment: Codex persists parent-child spawn/lifecycle and parent
  completion delivery; Reasonix starts fresh filtered subagents and returns only
  final text. Genesis rejects their free profile/fork controls in favor of role
  binding and ledger-owned invocation facts.

### KERNEL-SESSION-WORKSPACE-BINDING-20260711 - P1 - Desktop end-to-end acceptance remains after session-scoped binding implementation

- Status: open.
- Requirement: `docs/requirements/kernel-session-workspace-binding.md`.
  - Design: `docs/design/kernel-session-workspace-binding.md`.
- Gap: immutable `session.workspace_bound` events now select a per-session
    tool policy and hide roots from ordinary projections. The remaining gap is
    end-to-end desktop acceptance against a local model that reaches a visible
    final for all three entry modes.
  - Next slice: exercise Project shared-directory/independent-history, Task
    independent durable roots, and Chat no-default-cwd through the desktop
    application. Do not put the root into a desktop cwd override or copied
    archive.
  - Evidence: focused binding, restart, no-path-projection, cross-directory
    read, default-write refusal, and yolo-write tests pass. A real local-Qwen
    Project turn settled with durable reasoning and final output; temporary
    capped Task validation ended with reasoning only and the strict adapter
    correctly refused to invent a visible final.
- Verification: binding/restart/fail-closed/no-path-projection kernel tests,
  desktop bridge/frontend tests, then three settled local-model desktop turns.
- Reference alignment: Codex obtains cwd from its thread configuration
  snapshot; Reasonix ACP constructs one controller with its session cwd and
  tests project-local config loading. Genesis must additionally ledger-persist
  the binding and make `none` a durable chat state.

### KERNEL-PROVIDER-REASONING-MESSAGES-20260710 - P1 - Provider reasoning lacks additional provider replay contracts

- Status: open.
- Requirement: `docs/requirements/kernel-provider-reasoning-messages.md`.
  - Design: `docs/design/kernel-provider-reasoning-messages.md`.
  - Gap: Ordered canonical conversation now supports the response-only DeepSeek
    V4 rule and the replaying `zai-glm` / `glm-5.2` contract. The remaining production gap is opaque
    signed-state, provider-switch suppression explanations, compaction
    recovery, and additional provider contracts.
  - Next slice: only begin another provider contract after its official
    continuation rule, ownership boundary, and red tests are approved. Do not
    generalize OpenAI-compatible vendor fields or treat the OpenCode Go route
    as an adapter identity.
  - Evidence: Focused command, kernel restart/timeline, and desktop projection
    tests pass; the llama.cpp adapter self-test preserves `reasoning_content`.
    On 2026-07-10, `genesisctl provider verify` succeeded against the configured
    local Qwen provider. A real turn wrote `model.reasoning` before `model.final`;
    after daemon restart, session and timeline returned the same reasoning id and
    text length. Phase B focused DeepSeek follow-up, tool-loop, mismatch,
    provider-command, configuration, and full Go tests pass. The Phase C GLM
    focused tests prove both clear and preserved-thinking paths plus zero-egress
    refusal. On 2026-07-11, the configured `coder` profile verified ready as
    `frank/GLM-5.2`; an isolated daemon completed one bounded `shell_exec(pwd)`
    loop, persisted `model.reasoning` before `model.final`, and replayed both
    facts after restart. On 2026-07-11, the DeepSeek adapter was aligned with
    Reasonix's concrete OpenAI-client behavior: retained reasoning is omitted
    from ordinary and tool-continuation egress, while assistant tool calls
    serialize `content:""` for DeepSeek's strict request shape.
- Verification: `features/kernel/provider_reasoning_messages.feature`; focused
  DeepSeek provider-egress, tool-loop, provider-command, configuration, and
  context tests; then full Go and desktop suites.
- Reference alignment: Codex projects reasoning as a typed turn item; Reasonix
  persists reasoning and opaque signatures with an assistant message. Genesis
  keeps replay decisions in the selected adapter and kernel-owned transcript
  facts rather than a shell-local session.
