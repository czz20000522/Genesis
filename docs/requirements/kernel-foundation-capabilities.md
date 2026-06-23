# Requirement: Kernel Foundation Capabilities

- **Status:** approved.
- **Owner:** Genesis Kernel.
- **Scope:** foundation capabilities that make Genesis a shared LLM execution kernel rather than one application.

## Background

Genesis needs a small set of kernel capabilities that hold across every shell, skill, daemon, provider, and future application. Without this baseline, each application would be tempted to assemble prompts, decide permissions, run tools, write memory, and record evidence on its own. That would make Genesis a loose set of agents instead of a shared authority layer.

This requirement defines the baseline capabilities that every implementation slice must preserve: interface admission, model gateway, tool runtime, authority and credentials, work state, accumulation, readiness, inspection, and skill metadata.

## Production Target

Genesis Kernel is the authority execution layer for LLM-driven applications. It accepts intent, projects model context, governs tools, records facts, manages memory and work state, and exposes inspection surfaces.

The production target is:

- every accepted effect has a kernel-owned fact trail;
- every model-visible context fragment comes from an owner projection;
- every tool call goes through registry, validation, permission, execution, and evidence;
- every memory truth flows through candidate, review, and recall policy;
- every shell and application is user-space and cannot bypass kernel truth;
- every inspection surface is bounded, redacted, and purpose-specific.

## Users And Roles

Ordinary user:

- submits intent through a shell or application;
- sees assistant responses, user-facing timelines, memory review surfaces, and understandable blockers.

Operator/admin:

- configures provider, credential, skill roots, runtime token, permission profile, and workspace boundaries;
- inspects readiness, audit, context, capabilities, and ledger-backed diagnostics.

Reviewer:

- checks requirements, design, BDD behavior, issue evidence, retirement evidence, and regression tests;
- verifies that application-specific behavior has not entered the kernel.

LLM:

- sees kernel-projected context, approved memory context, safe skill metadata, and registered model-visible tools;
- proposes semantic tool arguments and memory/work content;
- does not create kernel ids, permission modes, sandbox profiles, credential refs, checkpoints, audit refs, or ledger facts.

Kernel:

- owns admission, authority, provider context, tool execution, event facts, memory truth, work truth, credential resolution, compaction, and projections.

Application:

- submits turns, supplies external context, reads projections, and may install skills or CLIs;
- does not assemble provider context, mint tool results, write memory truth, decide sandbox authority, or rewrite ledger history.

## Core Semantics

### System-Wide Semantics

1. The event ledger is the system fact layer. Session, timeline, audit, context, operation, memory, and readiness views are projections.
2. Control-plane fields are generated, bound, and validated by the kernel. The model can propose semantic content, not event ids, operation ids, session authority, sandbox profiles, approval policies, credential refs, checkpoint refs, or audit refs.
3. Provider context is assembled by the Model Gateway from ledger-backed facts. Same-session conversation history, approved memory, tool call/result rounds, skill index metadata, and compaction summaries are not synthesized by adapters.
4. Tools are registry-owned generic effects. Application-specific verbs do not enter the kernel as tool names.
5. Inspection surfaces expose bounded, redacted, path-safe projections. They are not hidden owner paths for raw secrets, skill bodies, provider-native payloads, or ordinary UI access to kernel internals.
6. Development-stage retired surfaces are removed from active code, tests, and requirements. Historical evidence may remain only in operations records.

### Persistence And Audit Layers

Genesis separates runtime output into five layers:

1. Realtime transport exists for streaming experience only. Token deltas, stdout chunks, progress frames, heartbeat frames, and stream frames live in memory or on the connection by default. They do not become long-term kernel facts until they are reduced to a completed message, tool result, job summary, terminal job fact, or another owner event.
2. Session transcript is the recovery and user-experience spine. It stores user messages, final assistant-visible replies, model-visible tool calls, model-visible final tool results, and product-approved reasoning summaries or notices. It does not store provider raw payloads or hidden reasoning chains.
3. Kernel durable facts store recovery and state truth. Checkpoints, terminal outcomes, permission denials, operation status, job terminal state, compaction outcome, memory review decisions, work decisions, and provider usage accounting belong here even when they are not ordinary UI content.
4. Security and control audit is strong-persistence and low-noise. It records authority changes, permission denials, credential use, dangerous-operation decisions, control-plane writes, governance publication or intake, break-glass actions, boundary-crossing access, and security failures. Ordinary success info and UI actions do not enter this audit layer.
5. Debug trace is opt-in. It may record provider projection summaries, response summaries, internal spans, chunk-level diagnostics, and gateway decisions, but it must have explicit enablement, bounded retention, quota, and redaction. Debug trace does not participate in replay, memory, provider context, or audit decisions.

A runtime event can enter long-term facts only when it is user-visible or model-visible, required for replay/recovery/idempotency/checkpointing, changes kernel-owned state, records a permission or risk decision, records failure or abnormal termination, or feeds provider context, compaction, memory recall, or observation delivery. Otherwise it stays in realtime transport, debug trace, or aggregate metrics.

### Interface Kernel

- `turn.submit` accepts user or application intent through a typed transport schema.
- Unknown transport fields, hidden control text, unsupported input item types, and attempts to set kernel-owned control fields fail closed before provider context construction.
- Prompt-injection-shaped content inside ordinary user text remains untrusted content. It may be recorded as risk metadata, but it does not grant authority.
- Turn idempotency is scoped to explicit `session_id + turn.submit + idempotency_key`. Replays return original ledger-backed evidence without new provider calls or tool effects.
- `turn.stream`, session, timeline, audit, and context inspection read from ledger-backed projections.
- HTTP is a transport for typed kernel commands and projections, not the durable contract.

### Model Gateway

- Provider integrations use a typed boundary. External provider commands own vendor SDKs, HTTP payloads, account flows, and provider credentials.
- Built-in provider adapters are local operator conveniences, not the default contract for new providers.
- Provider requests contain ordered input fragments, model-visible tool manifests, and prior model-visible tool rounds.
- Provider requests omit kernel-owned event ids, operation ids, leases, permission profile internals, checkpoints, and audit refs.
- Provider-native usage is normalized into kernel evidence when upstream fields are present: input tokens, output tokens, total tokens, cache hit tokens, cache miss tokens, and provider-backed processed input tokens.
- Token accounting belongs to the Model Gateway. Compaction selectors consume provider-backed accounting and do not fall back to local text token estimates.
- Provider failures become structured model/provider failures. They are not command stderr and are not disguised as tool results.
- Context compaction is executed by a kernel compaction runner. Triggers submit typed kernel commands; shells, adapters, provider commands, and daemons do not summarize, truncate, or rewrite history.

### Tool Runtime

- `ToolRegistry` is the single source for tool name, description, schema, side-effect level, execution kind, and executor binding.
- `ToolGateway` is the only runtime entry for model-requested tools.
- The default model-visible tool set starts with generic `shell_exec`; no application-specific outbound tool is introduced by default.
- Model-visible schemas expose only semantic fields the model must choose.
- Model-supplied control-plane fields produce repairable `tool_request_invalid` feedback and no effect.
- Tool call batches are preflighted as a unit before any effect executes.
- Tool results preserve the distinction between invalid request, permission denial, command failure, and tool infrastructure failure.
- Long output is presented with bounded head/tail text, truncation flags, original byte counts, omitted byte counts, and a visible omission marker.
- Redaction is projection policy. It must not mutate append-only operation evidence before it is recorded.

### Authority And Credential Plane

- Runtime-protected routes require a configured runtime token. Readiness is blocked when protected work cannot be accepted.
- Credentials are referenced through kernel-owned refs, not raw secrets in config, prompts, events, logs, readiness, provider context, or model-visible tool results.
- `plan`, `default`, and `yolo` are user-facing permission modes. The kernel resolves them into `authority_policy`, `sandbox_profile`, and `approval_policy` before admission.
- Current `default` is a controlled-workspace adapter, not an OS-level sandbox claim.
- Current `approval_policy` is `never`. Future approval must be a typed control-plane flow, not a model-supplied escalation field.
- Tool arguments cannot select permission mode, sandbox profile, approval policy, workspace root, credential authority, or runtime client authority.

### Work Registry

- `work.submit` records a kernel-owned work item with session linkage, title, and source ref.
- `work.cancel` records terminal cancellation evidence with authority, reason, and evidence ref.
- Work records survive restart and project through sessions.
- Work Registry is not an application task system, notification system, queue worker, retry engine, lease system, or scheduler unless those needs are reduced to generic kernel primitives.

### Accumulation

- Memory enters the kernel as a candidate, not as a silent model promise.
- Pending candidates require source refs and remain out of recall until approved.
- Approval, rejection, and supersession are durable owner decisions with authority, reason, and evidence.
- Rejected and superseded candidates are excluded from recall.
- Supersession creates a replacement pending candidate; it is not hidden approval or direct text mutation.
- `memory.recall` is a read-only observation surface. It does not run a model, append review evidence, or mutate candidates.
- Turn submission may record recalled approved memory refs on the admitted turn event.

### Readiness And Inspection

- `/ready` reports whether the kernel can accept protected work and names structured blockers.
- Capability inspection reports provider/runtime/ledger status, canonical kernel tool names, and safe skill metadata.
- Timeline, raw events, audit, and context inspection are separate projections for different audiences.
- Context inspection reports provider-visible input kinds, tool manifest names, skill metadata summaries, approved memory refs, provider status, and resolved permission profile without exposing full rendered prompts or raw secrets.
- Audit inspection reports event types, operation status, provider context input kinds, usage, failure codes, and truncation metadata.
- Ordinary UI timeline omits kernel-owned ids and control-plane internals unless the user opens a diagnostics surface.

### Skill Catalog

- Skill packages are user-space assets. The kernel may index safe metadata, but skills do not become kernel APIs.
- Configured skill roots can be scanned for `SKILL.md` metadata.
- Provider context receives only a bounded path-free metadata index by default.
- Skill bodies, instruction paths, package paths, and full examples are not injected into every turn.
- Unsafe, malformed, duplicate, linked-path, authority-shaped, hidden-control, prompt-injection-shaped, or secret-shaped metadata is excluded rather than repaired into model context.
- Full skill hydration, if added later, must use a generic resource/context contract and must not introduce a package-specific skill-body retrieval tool.
- Skill metadata can help the model discover user-space capabilities, but it grants no authority.

## Non-Goals

The foundation kernel does not include:

- CLI, WebUI, desktop UI, or mobile UI product behavior;
- Feishu, WeChat, email, calendar, document, OCR, web search, medical, insurance, or other domain logic;
- full skill-body injection by default;
- application-specific outbound APIs;
- multi-agent scheduling as a kernel primitive;
- vector database optimization as a first requirement;
- migration compatibility for retired Python data surfaces.

## Phased Delivery

Phase A: turn, ledger, fake provider, readiness, and restart-safe session replay.

- Proves: admission, event facts, provider loop shape, readiness blockers, and restart replay.
- Still short of production: no real provider, no governed tool loop, no accumulation, no work evidence.

Phase B: tool runtime, permission profile, shell execution, and terminal-equivalent tool results.

- Proves: registry ownership, model-visible tool manifest, permission denial, command output evidence, and repair feedback.
- Still short of production: shell sandbox is controlled workspace rather than OS sandbox; approval is not implemented; richer job progress and interrupt behavior remain governed by the shell/job requirement.

Phase C: work registry, accumulation, credential plane, and protected inspection.

- Proves: memory candidate/review/recall, work submit/cancel, runtime token, credential blockers, capabilities, timeline, audit, and context projections.
- Still short of production: richer memory selection, approval, stronger sandbox, and broader recovery policy remain future work.

Phase D: real provider boundary, provider-backed usage accounting, multi-turn projection, skill metadata, and compaction.

- Proves: provider command, built-in provider convenience, model usage normalization, provider-backed token accounting, metadata-only skills, and kernel-owned compaction.
- Still short of production: full use-time skill hydration, richer context policy, progress snapshots, and idle continuation policy remain future work.

Phase E: hardening and production readiness.

- Proves: stronger sandbox/approval where available, managed-job hardening, interrupt semantics, and broader recovery evidence.
- Still short of production until complete: stronger authority flows, foreground attach-or-kill, and arbitrary long-running effect recovery remain constrained.

## Acceptance Criteria

Positive cases:

- valid turn submission produces ledger events, provider result, session projection, and restart replay;
- fake and real provider paths return structured final responses;
- valid governed shell execution returns terminal-equivalent result evidence;
- approved memory can be recalled in a later turn;
- protected inspection surfaces show readiness, capability, timeline, audit, and context projections.

Negative cases:

- malformed transport fields fail before provider context construction;
- unauthorized tool effects are blocked before execution;
- model-supplied control-plane fields are rejected;
- raw secrets do not appear in context, logs, readiness, events, or model-visible results;
- unsafe skill metadata is excluded;
- rejected and superseded memories do not enter recall.

Fail-closed and recovery:

- provider failures are structured and do not panic the kernel;
- idempotent turn retries do not repeat provider calls or tool effects;
- idempotent tool retries do not repeat effects;
- restart replay reconstructs session, work, operation, and memory projections from ledger facts.

Audit and visibility:

- ordinary timeline, raw events, audit, and context projections remain separate;
- bounded output includes truncation metadata;
- readiness explains blockers without exposing secrets.

Test evidence:

- focused owner tests for each positive and negative path;
- architecture boundary tests for user-space separation;
- build and full test evidence before issue retirement.

## Relationship To Existing Issues

This requirement governs the foundation baseline and is the source for future foundation gaps.

Current related active issues:

- `KERNEL-SANDBOX-APPROVAL-NEXT-20260623`: implementation gap for stronger sandbox and approval beyond the current authority-profile split.
- `KERNEL-JOB-CONTROL-INTERRUPT-20260623`: remaining interrupt, progress snapshot, idle continuation, and foreground attach-or-kill semantics. It is governed by the shell/job requirement because it extends the generic Tool Runtime and managed-job path rather than the foundation baseline itself.

Related ready-for-acceptance shell/job evidence:

- `KERNEL-SHELL-TIMEOUT-CAP-20260623`, `KERNEL-MANAGED-JOB-FOUNDATION-20260623`, and `KERNEL-OBSERVATION-DELIVERY-20260623` are recorded in `docs/operations/kernel-retirement-log.md`.

Issues should cite this requirement only for gaps against these production semantics. They should not restate the full requirement or reopen application-specific kernel ownership.
