# Design: Kernel Owner Structure Governance

## Requirement

Governs `docs/requirements/kernel-owner-structure-governance.md`.

## Boundary And Owner

Architecture Governance owns structure rules and guards. Runtime owners still own their domain semantics:

- Interface Kernel owns turns and session entry.
- Tool Runtime owns tool calls, operations, jobs, and tool results.
- Work Registry owns work records.
- Accumulation owns memory candidates, review decisions, and recall.
- Readiness and Inspection own safe projections for humans and operators.

Governance does not own those facts. It only prevents central files and adapters from becoming hidden owners.

## Current `internal/kernel` Owner Map

This map describes the current file placement. It is not a target package
layout. The purpose is to make owner boundaries visible while the kernel is
still allowed to extract gradually.

| Current owner | Current files | Notes |
| --- | --- | --- |
| Kernel facade and core loop | `kernel.go`, `kernel_refs.go`, `doc.go`, `id.go` | `Kernel` coordinates owners and exposes the facade. It should keep shrinking, but it remains the current composition point. |
| Config and readiness state | `config_types.go`, `capabilities.go`, `budget_lease.go` | Config is still cross-cutting kernel setup. Budget leases are kernel control policy, not model-visible tool input. |
| Ledger and event schema | `ledger.go`, `event_types.go` | Event envelope and durable schema stay central until owner event contracts have stable exported ports. |
| Interface and turn admission | `turn_types.go`, `turn_interrupt.go`, `ingress_security.go` | Turn request/response, interruption, and ingress security are kernel entry semantics. |
| Model Gateway and provider boundary | `provider.go`, `provider_command.go`, `openai_compatible.go`, `provider_resilience.go`, `provider_resilience_types.go`, `provider_setup.go`, `model_config.go`, `model_context.go`, `provider_accounting_types.go`, `modelgateway/resilience.go`, related provider tests | This owner translates provider/runtime behavior into Genesis model responses, usage evidence, retry/repair evidence, and provider-visible context. The first extracted slice is provider retry/repair classification under `internal/kernel/modelgateway`. |
| Tool Runtime | `tool_types.go`, `tool_registry.go`, `model_tools.go`, `tool_scheduling.go`, `tool_execution.go`, `tool_loop_guard.go`, `toolruntime/scheduling.go`, related tool tests | This owner validates, schedules, authorizes, executes, and projects generic kernel tools. It must not become an application tool catalog. The first extracted slice is scheduling policy under `internal/kernel/toolruntime`. |
| Authority, sandbox, and approval | `authority_gate.go`, `sandbox_readiness.go`, `approval.go`, `approval_types.go`, `approval_owner_test.go` | This owner resolves permission, sandbox readiness, approval requests, decisions, and frozen-effect admission. |
| Shell and process execution | `shell.go`, `controlled_shell.go`, `controlled_shell_links_*.go`, `shell_environment.go`, `process_runtime.go`, `process_termination_*.go`, `managed_job_executor.go`, related shell/process tests | Shell is a generic execution primitive. It is not a place for app-specific CLI protocols. |
| Job and observation owner | `jobs.go`, `observations.go`, `job_progress_test.go`, `interrupt_test.go` | Jobs and observations own managed job status, sparse output facts, terminal observations, and delivery to later turns. |
| Resource and skill metadata | `resource/registry.go`, `resource/types.go`, `resource/registry_test.go`, root aliases in `resource_types.go`, `resource_read_test.go`, `skill_catalog.go`, `skill_catalog_types.go`, `skill_catalog_test.go` | Resource read is a generic primitive. Skill catalog exposes bounded metadata only; skill bodies remain user-space assets unless admitted through a generic resource/context contract. |
| Accumulation and memory | `memory.go`, `memory_types.go`, `memory_context_test.go` | Memory owner owns candidates, review decisions, recall eligibility, and model-visible safe projection. |
| Work Registry | `work.go`, `work_types.go` | Work owner records generic work state and cancellation, not application task semantics. |
| Context compaction | `context_compaction.go`, `context_compaction_types.go`, `context_compaction_stuck_test.go` | Compaction runner is kernel-owned context control. Triggers submit kernel commands; shells and apps do not summarize history. |
| Projection and inspection | `session_projection.go`, `projections.go`, `ui_timeline_projection.go`, `inspection_types.go`, `evidence_redaction.go`, `timeline_projection_test.go`, `projection_shape_test.go` | These files build safe read models. UI shells must consume projections, not reinterpret raw events. |
| HTTP transport | `http.go`, `http_turn.go`, `http_tools.go`, `http_work.go`, `http_memory.go`, `http_inspection.go`, `http_approvals.go` | HTTP files are transport adapters: auth, decode, route, delegate, encode. They do not own lifecycle or truth. |
| Local secret and platform adapters | `local_secret.go`, `dpapi_*.go` | Local credential protection remains an adapter under kernel credential boundaries. Raw secrets do not enter model context. |
| Governance and pressure tests | `architecture_boundary_test.go`, `pressure_test.go`, `kernel_test.go`, `test_tempdir_test.go` | `kernel_test.go` is still a central legacy test surface. New owner tests should prefer owner/topic files. |

## Boundary Classification Table

| Surface | Classification | May live under `internal/kernel`? | Rule |
| --- | --- | --- | --- |
| `Kernel` facade, ledger append/replay, event envelope, `SubmitTurn` orchestration | Kernel core | Yes | Keep as the composition boundary until owner ports are stable. Do not add domain behavior here. |
| Model Gateway, Tool Runtime, Authority Plane, Work Registry, Accumulation, Resource owner, Job owner, Projection owner | Kernel-owned subsystem | Yes, and may later extract into kernel subpackages | Must expose owner APIs and keep facts, validation, replay, and projections close to the owner. |
| HTTP transport for kernel commands and projections | Kernel transport adapter | Yes, only as thin delegation | Auth/decode/route/error-map/encode only. No owner replay, provider context assembly, or policy decisions. |
| Built-in OpenAI-compatible adapter and `provider_command` boundary | Kernel-owned provider boundary | Yes | Provider specifics are translated behind Model Gateway contracts. New provider ecosystems should prefer external commands/adapters. |
| Generic shell/process execution | Kernel-owned tool primitive | Yes | Generic execution, bounded output, sandbox/readiness, approval, and evidence only. No application protocol details. |
| Skill metadata catalog | Kernel projection of user-space assets | Yes, metadata only | Names/descriptions can be indexed. Bodies, examples, package paths, and app instructions require generic resource/context admission. |
| Feishu, WeChat, email, calendar, document, OCR, CodeGraph, WebUI, desktop UI, console UI, channel connectors | User-space application or connector | No | Put under `internal/applications`, `cmd/*`, skills, external adapters, or connector runtimes. They talk to kernel primitives. |
| External CLI command shapes such as `lark-cli im +messages-send` | User-space adapter implementation detail | No | Kernel may run generic shell effects, but must not own app CLI argv/protocols. |
| Application outbound delivery, retry, receipt, reconciliation, source cursor, source verification | User-space connector owner | No | Connector owns external protocol translation and delivery evidence. Kernel owns its own facts only. |

## Directory Rules For Blocking Application Drift

- `internal/kernel` must not contain files, packages, tool names, DTOs, tests, or comments that make Feishu, WeChat, email, calendar, document, OCR, WebUI, desktop UI, console UI, CodeGraph, or other application domains into kernel owners.
- Application-specific protocol translation belongs under `internal/applications/<runtime>` or a `cmd/<adapter>` entrypoint. If it needs kernel support, reduce it to a generic primitive first and open a kernel issue for that primitive.
- Default kernel tools remain generic. Adding an app-specific model-visible tool is rejected unless the requirement proves it is actually a generic kernel primitive with a domain-neutral name, schema, authority policy, and replay contract.
- Kernel tests may use application-shaped strings only as hostile fixtures or boundary examples. Long-term tests should assert the generic owner rule, not the absence of every possible app name.
- HTTP, CLI, desktop, WebUI, Feishu, and future channel shells may submit typed kernel commands and read projections. They cannot assemble provider context, decide permission/sandbox/approval, write memory truth, mint tool results, or rewrite ledger facts.
- External ids, roles, channels, paths, and credentials are foreign protocol data until translated by a boundary owner. They do not become kernel ids, authority, or public refs by being present in an inbound payload.

## Allowed Gradual Extraction Targets

The current single Go package is allowed while the kernel is small, but new
growth should prefer extracting behind owner ports instead of adding more central
helpers. Candidate package names are design targets; extraction requires
contract tests before moving callers.

| Target package | Current source area | Extraction condition |
| --- | --- | --- |
| `internal/kernel/modelgateway` | provider resilience first; later provider, provider command, OpenAI-compatible adapter, model context, usage accounting, retry/repair | Provider request/response types and context projection ports can be moved without changing `SubmitTurn` semantics. |
| `internal/kernel/toolruntime` | scheduling policy first; later tool registry, gateway, execution, loop guard, tool DTOs | Tool invocation context is narrow enough that tools no longer need unrelated `Kernel` authority. |
| `internal/kernel/projection` | session, UI timeline, detail, audit/context projection helpers | Projections can compose owner read models without reimplementing owner replay. |
| `internal/kernel/resource` | resource registry, resource read, future generic context hydration | Resource refs, bounded reads, grants, and hydration facts have a stable owner API. Phase A has moved descriptor/result types and registry/read logic here while the root package keeps compatibility aliases. |
| `internal/kernel/authority` | authority gate, sandbox readiness, approval owner | Approval/sandbox command path is stable enough to expose an authority-plane port. |
| `internal/kernel/jobruntime` | managed jobs, observations, process executor integration | Attach/detach, progress snapshots, cancellation, and observation delivery have stable replay semantics. |
| `internal/kernel/accumulation` | memory candidate/review/recall | Memory owner has a stable store and projection port independent of session projection. |
| `internal/kernel/workregistry` | work records and cancellation | Work owner needs no direct access to `Kernel` internals beyond event append/replay ports. |
| `internal/kernel/transport/http` | HTTP route files | Route handlers can import owner ports without creating an import cycle or duplicating owner policy. |

## First Relocation Candidates

1. Provider gateway.
   - Why: provider retry/repair, provider-command strictness, model config,
     context projection, and usage accounting already form a coherent boundary.
   - First completed port: provider retry/repair classification and attempt
     projection under `internal/kernel/modelgateway`.
   - Next ports: `BuildProviderContext`, `CallProvider`, provider command
     transport, context projection, and usage/attempt evidence append hooks.

2. Tool runtime.
   - Why: registry, scheduling, execution, storm guard, and tool result taxonomy
     are already mostly separate from session projection.
   - First completed port: scheduling policy, access plans, execution batch
     planning, and scheduling DTO aliases under `internal/kernel/toolruntime`.
   - Next ports: `ToolGateway` plus a narrow `toolInvocationContext` that
     exposes only authority, ledger append, job/operation, and resource access
     needed by a tool.

3. Timeline and projections.
   - Why: UI timeline/detail, session projection, audit/context projection, and
     redaction are read models. They are high-risk if shells start parsing raw
     events directly.
   - First port: owner replay helpers feeding a projection composer, with no
     direct transport ownership.

4. Resource.
   - Why: resource read and future context hydration are the next pressure point
     for avoiding skill-specific tools such as retired `skill.read`.
   - First port: resource descriptor/read/admission API with bounded output and
     derivation refs.

## Do Not Move Yet

| Surface | Reason to keep in place for now |
| --- | --- |
| `Kernel` facade | It is still the public composition point for local kernel use. Moving it early would create package churn without reducing authority. |
| `Config` and config normalization | Config feeds provider, authority, storage, skill roots, context, shell, and transport readiness. Split only when owner config ports are stable. |
| `SubmitTurn` main loop | It is the current control loop joining interface admission, provider calls, tool rounds, pause/resume, compaction, and final response. Extract owners first, then slim the loop. |
| Ledger event schema | Event schema is the fact envelope for replay. Moving it before owner event ports settle would create import cycles and schema churn. |

## Data Flow

```text
ledger events
  -> owner replay helpers
  -> session projection composer
  -> HTTP/CLI/UI/application transport projection
```

Transport flow stays:

```text
request -> auth/content-type -> decode/parse route -> owner API -> error map -> JSON response
```

Transport cannot replay ledger facts, merge owner state, or decide owner policy.

## Protocol

No runtime protocol is added. The protocol is an executable governance contract:

- architecture tests scan central files for owner replay drift;
- architecture tests keep DTO ownership visible through file placement;
- active issues must cite this requirement and design;
- implementation plans record the local Codex/Reasonix reference scan;
- implementation plans record `Reference Behavior Red Tests` that translate
  reference behavior into Genesis same-semantics red conditions before code;
- closing gates compare code, docs, issues, and BDD examples before commit.

## Failure Semantics

A governance failure is a test failure, not a runtime failure. The fix is to move logic to the owner helper, split the DTO into the owner file, shrink the transport handler, update the governing design, or record a temporary exception with an active issue.

## Permission And Authority

This design does not grant runtime authority. It constrains code authority: central coordinators may call owner APIs, but they do not mint owner facts or bypass owner validation.

Tool executor authority should be narrow. The long-term shape is a tool invocation context exposing only the owner capabilities needed to validate, authorize, execute, append evidence, and project the result for that tool.

## Recovery And Observability

Recovery still comes from the ledger and owner replay. Structure guards ensure the replay code remains close to the owner that understands the event. Periodic governance review checks architecture, feature behavior, directory structure, and document lifetime together. Documents that no longer represent active requirements, designs, or current issue gaps are deleted, condensed, or moved to retirement evidence.

## Reference Alignment

Codex keeps typed runtime contracts around tool execution: `codex-rs/core/src/tools/registry.rs` defines `CoreToolRuntime` over `ToolInvocation`, and `codex-rs/core/src/tools/handlers/unified_exec/exec_command.rs` implements an exec handler through that tool runtime instead of giving every tool arbitrary session authority. Codex tests also exercise compaction, approval, exec, and event behavior through core events rather than UI-only state.

Reasonix records an acyclic dependency direction in `docs/SPEC.md`: `cli -> {agent, plugin, config} -> {tool, provider}`. Its `internal/tool/tool.go` keeps a per-run registry and its `internal/permission` design gates each tool call independently of CLI. Its `control.Controller` is a frontend-agnostic control layer, which is useful as a reference but also shows the risk Genesis should avoid as the kernel grows.

Genesis aligns with the registry, owner, and projection ideas but intentionally does not copy either project's application-specific package layout.

## Rejected Alternatives

- Keep `kernel.go` as the single replay switch for every owner. Rejected because it makes new owner growth too easy and hides authority boundaries.
- Keep all DTOs in `types.go`. Rejected because file-level structure is part of the review surface in a fast AI-assisted codebase.
- Add line-count caps. Rejected because readability and owner placement matter more than arbitrary size.
- Preserve obsolete process documents indefinitely. Rejected because stale documents become false architecture.
