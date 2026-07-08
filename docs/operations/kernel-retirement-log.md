# Kernel Retirement Log

This file records Genesis Kernel issues that are ready for acceptance or retired. It is the repo-owned companion to `docs/operations/kernel-issues.md`.

## Retirement Rules

- `ready_for_acceptance` means the code and verification evidence are ready for user or operator acceptance, but the issue is not fully retired yet.
- `retired` means the user or operator accepted the evidence. A retired issue must be absent from `kernel-issues.md`.
- Every retired entry should be compact: one sentence with the retirement conclusion plus fixing commit evidence. Detailed fix summaries, full verification transcripts, residual production gaps, and boundary proofs belong in the cited commit, tests, requirements/designs, or still-active issues.
- `ready_for_acceptance` entries may temporarily keep the focused evidence needed for acceptance. Compress them when they move to `Retired`.
- Every `KERNEL-BOUNDARY-*` entry and every architecture-type `KERNEL-*` entry must retain either `Reference alignment` or `Rejected drift risk` when moved from the active ledger.
- Entries summarize evidence. They should cite governing requirements and designs only when needed to locate the source of truth; do not copy the full production contract, raw debug output, stream chunks, ordinary info logs, or issue history.
- If an entry is reopened, move it back to `kernel-issues.md` and mark this log entry as reopened with the reason.

## Ready For Acceptance

### KERNEL-PARENT-WORKER-ROLE-BINDING-20260708 - P1 - Parent-worker role binding config projection

- Status: ready_for_acceptance.
- Conclusion: `models.json` can now project parent/worker runtime bindings with parent profile selection, allowed worker roles, preset tool sets, leaf-only worker roles, model/profile route summaries, max_parallel defaults, and no credential or permission-profile leakage.
- Evidence: Fix commit: current Lore commit.; Verification: `go test ./internal/kernel -run "Test(ResolveParentWorkerRuntimeFromGenesis|ResolveProviderConfigFromGenesis|ResolveOpenAICompatibleConfigFromGenesis|ArchitectureBoundary)" -count=1`.
- Reference alignment: Aligned with Codex explicit spawned-agent identity/concurrency control and Reasonix configured subagent model/tool metadata; task graph layout and scheduling stay outside this Phase A slice.

### KERNEL-PARENT-WORKER-INVOCATION-20260708 - P1 - Worker invocation consumes role bindings

- Status: ready_for_acceptance.
- Conclusion: Parent-worker admission now resolves a configured role binding into a normal `AgentInvocation`, uses the role preset tool set, permits multiple same-role invocation identities, and refuses extra requested tools before ledger append.
- Evidence: Fix commit: current Lore commit.; Verification: `go test ./internal/kernel -run "Test(AdmitWorkerInvocationFromRole|AgentInvocation|ResolveParentWorkerRuntimeFromGenesis|ArchitectureBoundary)" -count=1`.
- Reference alignment: Aligned with Codex distinct spawned-agent identities and Reasonix configured subagent tool scoping; HTTP transport, child conversation rendering, and task graph scheduling stay outside this slice.

### KERNEL-JOB-PROGRESS-IDLE-CONTINUATION-20260623 - P2 - Local managed shell job handoff

- Status: ready_for_acceptance.
- Conclusion: Local shell execution now uses a managed runner for foreground and background paths; interrupted foreground waits hand off to kernel-owned managed jobs with bounded output, cancellation, lost-ownership recovery, `job_wait` budgeted inspection, and no pid/signal/process-handle projection.
- Evidence: Fix commit: current Lore commit.; Verification: `go test ./internal/kernel -run "TestLocalForeground|TestRestartMarksLocalManagedJobLostOwnershipWithoutRerun|TestSubmitTurnJobWait" -count=1`; `go test ./internal/kernel -count=1`.
- Reference alignment: Aligned with Codex process-manager ownership and Reasonix session-scoped jobs while rejecting shell `&` and arbitrary pid attach as kernel backgrounding.

### KERNEL-MANAGED-PROCESS-ADMISSION-RACE-20260625 - P1 - Managed process admission ownership

- Status: ready_for_acceptance.
- Conclusion: Local managed job and foreground shell admission now reserve the kernel-owned job/operation slot before starting the real subprocess, so duplicate admission fails before a second process can produce effects.
- Evidence: Fix commit: current Lore commit.; Verification: `go test ./internal/kernel -run "ManagedProcess|ManagedJob|Foreground|LostOwnership|Interrupt" -count=1`.
- Reference alignment: Aligned with Codex/Reasonix process ownership as a pre-effect admission boundary rather than best-effort cancellation after duplicate effects begin.

### KERNEL-TEST-SURFACE-OWNER-SPLIT-20260625 - P3 - Kernel test owner surface

- Status: ready_for_acceptance.
- Conclusion: `internal/kernel/kernel_test.go` was removed as a behavior-test warehouse; its 152 tests now live in owner/topic files for turn lifecycle, HTTP transport, HTTP shell, shell execution, work registry, memory review/context, provider gateway, tool loop, job control, projection read models, compaction, and content fidelity, while helper-only code moved to `kernel_test_helpers_test.go` and cross-owner pressure scenarios remain in dedicated pressure files.
- Evidence: Fix commit: current Lore commit.; Verification: `go test ./internal/kernel -count=1`.
- Reference alignment: Aligned with Codex and Reasonix topic-oriented test suites while rejecting line-count caps as a governance proxy.

### KERNEL-PROJECTION-ARRAY-SHAPE-CONTRACT-20260625 - P3 - Public projection array shape

- Status: ready_for_acceptance.
- Conclusion: Public session, timeline, context inspection, audit, capability, memory, and turn-event projection collections now marshal stable non-null JSON arrays, with projection-tree children normalized recursively and the shape rule documented in the foundation design.
- Evidence: Fix commit: current Lore commit.; Verification: `go test ./internal/kernel -run "Test.*Projection.*Array|Test.*Timeline|Test.*ContextInspection|TestHTTP.*Session|TestHTTP.*Capabilities|TestHTTP.*Memory" -count=1`; `go test ./internal/kernel -count=1`; `go test ./... -count=1`; `go build ./...`; `git diff --check`.
- Reference alignment: Aligned with Reasonix's frontend-facing non-nil array contract and Codex-style typed protocol/schema shape testing while keeping Genesis projection owners responsible for client-facing DTO shape.

### KERNEL-RESOURCE-READ-MODEL-RESULT-REDACTION-20260625 - P2 - resource_read model-result redaction

- Status: ready_for_acceptance.
- Conclusion: `resource_read` keeps raw resource owner text unchanged while paginating over a bounded `ModelResourceReadResult.Text` projection through provider tool results, provider context, provider-command/OpenAI-compatible serialization, session projection, and UI timeline.
- Evidence: Fix commit: current Lore commit.; Verification: `go test ./internal/kernel -run "TestResourceRead" -count=1`; `go test ./internal/kernel -run "Test.*ResourceRead|Test.*ProviderContext|Test.*Timeline" -count=1`; `go test ./internal/kernel -count=1`; `go test ./... -count=1`; `go build ./...`; `git diff --check`.
- Reference alignment: Aligned with Codex's source/sandbox tests that keep denied sensitive read output out of model-visible function-call output and Reasonix's bounded diagnostic projection pattern while preserving Genesis's raw resource-owner truth boundary.

### KERNEL-SKILL-CATALOG-SCAN-BOUNDS-20260625 - P2 - Skill catalog scan bounds

- Status: ready_for_acceptance.
- Conclusion: Skill catalog discovery now bounds recursion depth, candidate count, and `SKILL.md` metadata size before parsing while preserving path-free exclusions and metadata-only provider context.
- Evidence: Fix commit: current Lore commit.; Verification: `go test ./internal/kernel -run "Test.*Skill" -count=1`; `go test ./internal/kernel -count=1`; `go test ./... -count=1`; `go build ./...`; `git diff --check`.
- Reference alignment: Aligned with Reasonix bounded skill-root scanning and Codex model-visible skill-context budgeting while keeping Genesis skill bodies outside default provider context.

### KERNEL-PROVIDER-REASONING-REPLAY-GUARD-20260625 - P2 - Provider reasoning non-replay guard

- Status: ready_for_acceptance.
- Conclusion: OpenAI-compatible `reasoning_content` is now guarded as response-only data; visible assistant content and usage survive, while hidden reasoning is absent from stored events, provider context, provider-command payloads, OpenAI-compatible replay, session/context/audit/UI projections, and same-session history.
- Evidence: Fix commit: current Lore commit.; Verification: `go test ./internal/kernel -run "TestOpenAICompatible.*Reasoning|Test.*ProviderContext.*Reasoning|TestObservabilityProjectionsSeparateRawAuditAndProviderContext" -count=1`; `go test ./internal/kernel -count=1`; `go test ./... -count=1`; `go build ./...`; `git diff --check`.
- Reference alignment: Aligned with Reasonix's explicit DeepSeek/OpenAI `reasoning_content` non-replay guard and Codex's separation between model-visible context, reasoning events, and raw protocol inspection.

### KERNEL-RESOURCE-PURE-READ-PRIMITIVE-20260624 - P1 - Generic resource_read primitive

- Status: ready_for_acceptance.
- Conclusion: `resource_read` established the first default non-shell `pure_read` candidate, with bounded immutable text reads, repair feedback for unknown refs, hidden scheduling metadata, and shell remaining serial.
- Evidence: Fix commit: `5da33e29e`.; Verification: `go test ./internal/kernel -count=1`; `git diff --check`.
- Reference alignment: Matches Reasonix trusted read-only metadata and Codex opt-in parallel support while rejecting shell command text as a read classifier.

### KERNEL-TOOL-SCHEDULING-CONCURRENCY-20260624 - P2 - Tool scheduling and pure-read executor pool

- Status: ready_for_acceptance.
- Conclusion: Tool execution now plans by trusted effect/footprint policy, keeps effectful and process-control work serial, runs only eligible `pure_read` batches concurrently, and commits `tool.result` events in provider call order.
- Evidence: Fix commits: `56dadd307`, `7b06851ff`, `05738e97a`, `efdcd8a20`, `5da33e29e`, `665a9222f`.; Verification: `go test ./internal/kernel -count=1`; `go test -race ./internal/kernel -run "TestExecuteToolBatchesRunsPureReadBatchConcurrently|TestExecuteToolBatchesKeepsProcessIOBatchSerialByDefault|TestResourceReadPreparesPureReadAccessPlan" -count=1`; `git diff --check`.
- Reference alignment: Matches Reasonix's read-only batch fan-out and Codex's explicit parallel support gate while intentionally not adopting shell parallelism.

### KERNEL-UI-LIVE-TIMELINE-PROJECTION-20260624 - P1 - UI timeline projection

- Status: ready_for_acceptance.
- Conclusion: Timeline projection now uses kernel-owned turn trees with live/settled processing groups, user-action approval nodes, selected-node detail projection, and no ordinary chat rows for raw tool/job events.
- Evidence: Fix commits: `a18d44196`, `d4383a53e`.; Verification: `go test ./internal/kernel -count=1`.
- Reference alignment: Keeps Codex-style live work collapse and Reasonix-style standalone approval prompts without letting shells rebuild chat timelines from raw events.

### KERNEL-JOB-CONTROL-INTERRUPT-20260623 - P2 - Turn interruption and foreground shell interrupt semantics

- Status: ready_for_acceptance.
- Conclusion: `InterruptSession` is now the kernel-owned typed control command for active turn cancellation. `POST /sessions/{id}/interrupt` is a thin transport delegate that returns `202 Accepted` for an active turn. Provider-step cancellation writes `assistant.interrupted`, returns `ErrTurnInterrupted`, projects the turn as `interrupted`, and does not write `turn.failed`. Foreground shell context cancellation writes `operation.interrupted`, appends a paired `tool.result(status=interrupted)`, and stops before a final provider step. Tests prove interrupting a later active provider turn does not cancel an existing managed background job.
- Evidence: Fix commits: `6e3287525`.; Verification: `D:\software\Go\bin\go.exe test ./internal/kernel -run 'TestInterruptSession|TestHTTPInterruptSession' -count=1`; `D:\software\Go\bin\go.exe test ./internal/kernel -run 'TestArchitectureBoundary|TestSubmitTurn.*Job|TestSubmitTurn.*Shell|TestHTTP.*Shell|TestHTTPInterruptSession|TestInterruptSession' -count=1`; `D:\software\Go\bin\go.exe test ./internal/kernel -count=1`; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `git diff --check`.
- Reference alignment: Aligned with Codex's split between turn interruption and background terminal/process lifecycle, and Reasonix's per-turn cancel context plus explicit provider interrupted semantics. Genesis records interruption as a kernel turn/control fact while preserving `job_cancel` as the explicit managed-job cancellation path.

### KERNEL-OWNER-SESSION-PROJECTION-20260623 - P1 - Session projection delegates owner replay

- Status: ready_for_acceptance.
- Conclusion: `Kernel.Session()` now validates the session id, loads events, delegates to `projectSessionProjection`, and redacts the final projection. The replay switch moved to `session_projection.go`, which composes turn, operation, job, work, memory, and raw event projection helpers outside the core kernel method. `TestArchitectureBoundaryKernelSessionDelegatesOwnerReplay` fails if owner replay markers return to `kernel.go`'s `Session()` body.
- Evidence: Fix commits: `8d969d722`, `694c8e31c`.; Verification: `D:\software\Go\bin\go.exe test ./internal/kernel -run 'TestArchitectureBoundary(KernelSessionDelegatesOwnerReplay|OwnerDTOsLiveInNamedFiles|HTTPTransportDoesNotReplayOwnerFacts|CoreLoopHasNoProviderNativeWireTerms|KernelIssuesRequireReferenceAlignment)' -count=1`; `D:\software\Go\bin\go.exe test ./internal/kernel -count=1`; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `git diff --check`.
- Reference alignment: Aligned with Codex and Reasonix control-plane separation. Codex keeps tool/session behavior behind typed runtime surfaces and core events; Reasonix records a frontend/controller/agent/tool/provider separation where CLI/frontends do not own tool or provider internals. Genesis keeps `Session()` as a projection entry point instead of a cross-owner replay switch.

### KERNEL-OWNER-DTO-FILES-20260623 - P1 - Public DTOs live in owner and projection files

- Status: ready_for_acceptance.
- Conclusion: The global `internal/kernel/types.go` file was removed. Public DTOs now live in `config_types.go`, `turn_types.go`, `tool_types.go`, `work_types.go`, `memory_types.go`, `event_types.go`, `inspection_types.go`, `provider_accounting_types.go`, `context_compaction_types.go`, and `skill_catalog_types.go`. `TestArchitectureBoundaryOwnerDTOsLiveInNamedFiles` fails if these owner and projection declarations move back into a global DTO file or the wrong owner file.
- Evidence: Fix commits: `8d969d722`, `694c8e31c`.; Verification: `D:\software\Go\bin\go.exe test ./internal/kernel -run 'TestArchitectureBoundary(KernelSessionDelegatesOwnerReplay|OwnerDTOsLiveInNamedFiles|HTTPTransportDoesNotReplayOwnerFacts|CoreLoopHasNoProviderNativeWireTerms|KernelIssuesRequireReferenceAlignment)' -count=1`; `D:\software\Go\bin\go.exe test ./internal/kernel -count=1`; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `git diff --check`.
- Reference alignment: Aligned with Reasonix's package-level split between provider, tool, permission, config, and agent types, and with Codex's protocol/runtime separation. Genesis keeps one small kernel package for now, but file names now expose owner placement.

### KERNEL-OWNER-HTTP-TRANSPORT-20260623 - P2 - HTTP transport files stay thin delegates

- Status: ready_for_acceptance.
- Conclusion: HTTP transport handlers are split into `http_turn.go`, `http_tools.go`, `http_work.go`, `http_memory.go`, and `http_inspection.go`; `http.go` keeps routing and common transport helpers. `TestArchitectureBoundaryHTTPHandlersLiveInSurfaceFiles` checks handler file ownership, and `TestArchitectureBoundaryHTTPTransportDoesNotReplayOwnerFacts` blocks direct owner append/replay helpers in HTTP files.
- Evidence: Fix commits: `8d969d722`, `a959c76de`.; Verification: `D:\software\Go\bin\go.exe test ./internal/kernel -run 'TestHTTP|TestArchitectureBoundaryHTTP' -count=1`; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `git diff --check`.
- Reference alignment: Aligned with Reasonix's frontend/controller separation and Codex's protocol/event surfaces. Genesis HTTP remains a shell/adapter: it authenticates, decodes, delegates to owner APIs, maps owner errors, and encodes projections without owning replay or state transitions.

### KERNEL-OWNER-TOOL-CONTEXT-20260623 - P2 - Tool registry binding uses narrow invocation context

- Status: ready_for_acceptance.
- Conclusion: `registeredTool.Prepare` now accepts `toolInvocationContext` instead of `*Kernel`. Default tool entries call only context methods for shell execution, job status, and job cancel preparation. `TestArchitectureBoundaryToolRegistryDoesNotBindWholeKernel` fails if `Prepare func(*Kernel, ...)` or `Prepare: (*Kernel)` returns to `tool_registry.go`.
- Evidence: Fix commits: `8d969d722`, `5d244a42e`.; Verification: `D:\software\Go\bin\go.exe test ./internal/kernel -run 'TestArchitectureBoundaryToolRegistry|TestSubmitTurn.*Job|TestSubmitTurn.*Shell|TestSubmitTurn.*Tool|TestToolCapability|TestHTTPCapabilities' -count=1`; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `git diff --check`.
- Reference alignment: Aligned with Codex's `CoreToolRuntime` over typed `ToolInvocation` and Reasonix's `Tool` interface plus per-run `Registry`. Genesis keeps a registry-owned execution binding, but the binding no longer declares the whole kernel object as the tool authority.

### KERNEL-FOREGROUND-TIMEOUT-OUTCOME-20260623 - P2 - Foreground timeout preserves terminal outcome evidence

- Status: ready_for_acceptance.
- Conclusion: Foreground shell timeout now records `operation.failed` with `timed_out=true`, `timeout_reason=foreground_timeout`, positive `elapsed_ms`, exit code evidence, and available bounded stdout/stderr. The model-visible `shell_exec` tool result carries the same timeout metadata and captured output, while `infrastructure_reason` stays empty for ordinary runtime timeout. Timeout remains separate from malformed request feedback and managed-job routing.
- Evidence: Fix commits: `dfda23540`.; Verification: `TestSubmitTurnForegroundShellTimeoutRecordsTerminalOutcome`; focused timeout/direct-shell/job routing suite; `D:\software\Go\bin\go.exe test ./internal/kernel -run TestArchitectureBoundary -count=1`; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `git diff --check`.
- Reference alignment: Aligned with local Codex and Reasonix terminal-equivalent tool behavior. Codex preserves timeout output and execution metadata for model inspection; Reasonix returns timeout as a tool execution outcome after collecting bounded output. Genesis follows the same split: timeout is an executed command result unless process infrastructure itself fails.

### KERNEL-DIRECT-SHELL-MANAGED-JOB-PARITY-20260623 - P2 - Direct shell transport shares managed-job routing

- Status: ready_for_acceptance.
- Conclusion: Direct `POST /tools/shell_exec` now distinguishes omitted `timeout_sec` from explicit invalid values, rejects explicit non-positive timeout before effects, delegates foreground and managed routing to ToolGateway, returns foreground `OperationProjection` for foreground-valid requests, returns HTTP 202 with a redacted `JobProjection` receipt for an admitted host-sandbox long job, and returns the existing operation or job projection on idempotent retry without executing a second effect. Controlled-workspace/default long shell requests are blocked until a controlled managed executor exists. Direct HTTP long jobs do not forge provider-loop `tool.call` or `tool.result` events.
- Evidence: Fix commits: `1070e2ef6`, `b24f9556a`.; Verification: `TestHTTPShellExecLongTimeoutReturnsManagedJobReceipt`; `TestHTTPShellExecRejectsExplicitZeroTimeout`; `TestHTTPShellExecLongTimeoutDoesNotBypassDefaultSandbox`; `TestHTTPShellExecManagedJobRetryRedactsTerminalOutput`; `TestHTTPShellExecIdempotencyKeyDoesNotCrossFromOperationToJob`; `TestHTTPShellExecIdempotencyKeyDoesNotCrossFromJobToOperation`; focused HTTP shell and job-control kernel tests; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `D:\software\Go\bin\go.exe test -race ./internal/kernel -count=1`; `git diff --check`.
- Reference alignment: Aligned with Codex and Reasonix's shared-owner pattern: transports and shells submit into the core/controller path rather than owning independent tool lifecycle semantics. Genesis keeps direct HTTP as a transport projection over Tool Runtime and managed-job ledger facts.

### KERNEL-OBSERVATION-DELIVERY-20260623 - P1 - Kernel observation queue and delivery checkpoints

- Status: ready_for_acceptance.
- Conclusion: Terminal managed-job facts now become Kernel Observation Queue sources. `ProviderContextProjection` injects undelivered terminal job observations as `kernel_observation_context` before a provider step, `SubmitTurn` records `kernel.observation.delivered` only after the provider call returns successfully, and delivered ids suppress repeat projection after ledger replay. Provider failures append turn failure evidence without marking the observation delivered.
- Evidence: Fix commits: `531f8d008`.; Verification: `TestSubmitTurnDeliversCompletedJobObservationToNextProviderStep`; `TestProviderFailureDoesNotMarkJobObservationDelivered`; `TestDeliveredJobObservationIsNotProjectedAgainAfterRestart`; focused observation/managed-job suite; `D:\software\Go\bin\go.exe test ./internal/kernel -count=1`; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `D:\software\Go\bin\go.exe test -race ./internal/kernel -count=1`; `git diff --check`.
- Reference alignment: Aligned with Codex's core/session ownership of tool-loop and compaction state: external shells submit typed facts, while the core decides which observations enter provider context and when delivery is recorded. This rejects UI, daemon, or provider-adapter ownership of model-visible observation delivery.

### KERNEL-JOB-CONTROL-MINIMAL-20260623 - P2 - Minimal generic job status and cancel tools

- Status: ready_for_acceptance.
- Conclusion: The model-visible manifest now includes `shell_exec`, `job_status`, `job_wait`, and `job_cancel`. `job_status` replays current job state from the session ledger without creating operations, `job_wait` uses an explicit timeout budget, and `job_cancel` records semantic `job.cancel_requested` for a non-terminal job without forging terminal cancellation before executor confirmation. Terminal jobs return the current terminal state without writing competing facts. Unknown job ids and model-supplied process/control-plane fields return structured repair feedback.
- Evidence: Fix commits: `ce72dfa44`, `b24f9556a`.; Verification: `TestSubmitTurnProjectsGenericJobControlToolManifest`; `TestSubmitTurnJobStatusReturnsCompletedJobAfterRestartWithoutOperation`; `TestSubmitTurnJobStatusRedactsTerminalOutput`; `TestSubmitTurnJobStatusReturnsRepairFeedbackForUnknownJob`; `TestSubmitTurnRejectsJobControlToolControlPlaneFields`; `TestSubmitTurnJobCancelTerminalJobReturnsCurrentStateWithoutCompetingTerminalEvent`; `TestSubmitTurnJobCancelLedgerOnlyRunningJobRecordsRequestWithoutForgingTerminalFact`; `TestSubmitTurnJobCancelReachesLiveManagedExecutor`; focused job-control suite; `D:\software\Go\bin\go.exe test ./internal/kernel -count=1`; `D:\software\Go\bin\go.exe test ./...`; `D:\software\Go\bin\go.exe build ./...`; `git diff --check`.
- Reference alignment: Aligned with Codex-style process control boundaries: the model receives a typed tool result for a kernel-owned handle, while process mechanics stay behind the runtime. Genesis intentionally exposes `job_status` and `job_cancel`, not process ids, signals, or a `job_terminate` tool.

### KERNEL-SHELL-TIMEOUT-CAP-20260623 - P1 - Foreground shell timeout policy and cap

- Status: ready_for_acceptance.
- Conclusion: `shell_exec` now exposes `timeout_sec` in the model-visible tool schema. Omitted timeout records 30 seconds; `timeout_sec=1` and `timeout_sec=180` run as foreground shell attempts; invalid zero, negative, string, and fractional values return repairable `tool_request_invalid` feedback and create no operation. Requests above the foreground cap route to managed-job admission instead of being treated as validation errors; the current local managed executor admits only host-sandbox jobs and blocks controlled-workspace/default long requests.
- Evidence: Fix commits: `abb6b5d45`.; Verification: `TestSubmitTurnAcceptsForegroundShellTimeoutSeconds`; `TestSubmitTurnDefaultsShellTimeoutToThirtySeconds`; `TestSubmitTurnReturnsRepairFeedbackForInvalidShellTimeoutSeconds`; `TestSubmitTurnRoutesLongShellTimeoutToManagedJobReceipt`; `TestSubmitTurnProjectsRegisteredToolManifestWithoutSkillCatalogContext`; focused timeout suite; `D:\software\Go\bin\go.exe test ./internal/kernel -run TestArchitectureBoundary -count=1`; forbidden marker scan; `git diff --check`; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `D:\software\Go\bin\go.exe test -race ./internal/kernel -count=1`.
- Reference alignment: Aligned with Codex-style tool-loop boundaries where short tools close with a tool result, while long-running work moves behind a managed process/job abstraction. Reasonix also keeps frontend shells behind a controller rather than letting an adapter own lifecycle policy.

### KERNEL-MANAGED-JOB-FOUNDATION-20260623 - P1 - Minimal managed job event model

- Status: ready_for_acceptance.
- Conclusion: A model `shell_exec` request with `timeout_sec > 180` now records `tool.call`, `job.started`, immediate receipt-style `tool.result`, and `model.final` without pretending final command output is available. The `job.started` event id is the job handle. The provider receives a `managed_job_started` tool result, allowing the tool loop to close while terminal job facts are written later by the managed executor. Session replay preserves the receipt and lifecycle facts.
- Evidence: Fix commits: `abb6b5d45`.; Verification: `TestSubmitTurnRoutesLongShellTimeoutToManagedJobReceipt`; focused managed-job suite; `D:\software\Go\bin\go.exe test ./internal/kernel -run TestArchitectureBoundary -count=1`; forbidden marker scan; `git diff --check`; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `D:\software\Go\bin\go.exe test -race ./internal/kernel -count=1`.
- Reference alignment: Aligned with Codex's separation between tool-call closure and managed process lifecycle. The intentional difference is scope: Genesis starts with a generic job primitive and does not copy a coding-agent-specific task runner.

### KERNEL-USER-SPACE-BOUNDARY-20260623 - P1 - Kernel, user-space, and agent-framework boundary document

- Status: ready_for_acceptance.
- Conclusion: `docs/kernel-contract.md` now contains `Agent Kernel vs Agent Framework` and `System Boundary / Box Model`. The contract defines the LLM as the operator, the kernel as the authority execution and fact boundary, tools as governed reality touchpoints, skills as user-space instruction packages, applications and shells as user-space compositions, and the event log as the system fact layer. `features/kernel/user_space_boundary.feature` records BDD acceptance examples for calculator skills, Feishu daemons, shell-owned context, app-owned memory, and framework attempts to forge kernel facts.
- Evidence: Fix commits: `b61be7b35`, `46c32f0ed`.; Verification: The document can answer that a calculator skill is not kernel, a Feishu daemon is not kernel, WebUI cannot assemble provider context, applications cannot write memory truth or tool results directly, and new domain-named capabilities must map to generic kernel primitives before entering the kernel ledger. `git diff --check`; `go test ./internal/kernel -run TestArchitectureBoundary -count=1`; `go test ./... -count=1`; `go build ./...`.
- Reference alignment: Codex and Reasonix are strong agent products with kernel-like runtimes. Their useful reference is the separation of core protocol, tool manifests, sandboxing, event truth, projections, and shells. Genesis applies that runtime split as a shared platform contract for multiple user-space applications, not as a copied coding-agent product.

### KERNEL-PRESSURE-ACCEPTANCE-20260623 - P1 - Minimal kernel loop needs deterministic pressure verification

- Status: ready_for_acceptance.
- Conclusion: `TestKernelPressureLongRunningClosedLoop` runs a 12-turn session through successful `shell_exec` calls, repairable invalid tool requests, permission-denied tool results, terminal-equivalent failed command results, one provider failure, automatic context compaction, restart-safe session projection, restart-safe provider-context projection, UI timeline compaction notices without summary leakage, and idempotent turn replay without calling the provider again. The test asserts the ledger reconstructs completed and failed turns, completed/failed/blocked operations, tool call/result events, provider accounting events, compaction completion, and turn failure evidence after restart.
- Evidence: Fix commits: `d7a12ee7a`.; Verification: `D:\software\Go\bin\go.exe test ./internal/kernel -run TestKernelPressureLongRunningClosedLoop -count=1 -v`; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `CGO_ENABLED=1 D:\software\Go\bin\go.exe test -race ./internal/kernel -count=1`; `git diff --check`.
- Reference alignment: Codex relies on core/session/tool-loop tests and recovery checks rather than treating shell or app surfaces as the source of truth. Reasonix keeps frontend/controller flows behind reproducible runtime checks. Genesis now has the same class of deterministic core pressure gate without widening the kernel into product-specific adapters.

### KERNEL-CONTEXT-COMPACTION-REFINE-20260622 - P1 - Context compaction needs production-grade selection and retry evidence

- Status: ready_for_acceptance.
- Conclusion: `ContextPolicy.RetryBackoffTurns` now normalizes to a bounded default and failed summarizer attempts record `context.compaction.failed`; the next eligible trigger can record `context.compaction.deferred` with previous failure and remaining backoff evidence before a later retry. `model.context.accounted` now records provider-visible tool round, call, and result counts in addition to provider usage and processed input tokens. The compaction source is built from completed conversation turns and preserves model-visible tool call/result pairs before the assistant final answer without exposing kernel event or operation ids. Completed compaction records triggering `source_usage` and cache-stability metrics for sampled compacted turns. `docs/kernel-contract.md`, `docs/minimal-closed-loop.md`, `docs/field-reference.md`, and `features/kernel/context_compaction.feature` now describe those behaviors.
- Evidence: Fix commits: `7641953d0`.; Verification: `TestAutoCompactionBacksOffAfterSummarizerFailure`; `TestModelGatewayAccountsToolRoundBoundaries`; `TestAutoCompactionRecordsUsageEconomicsAndCacheStability`; `TestCompactionSourcePreservesCompletedToolCallResultPairs`; focused compaction/accounting suite; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `CGO_ENABLED=1 D:\software\Go\bin\go.exe test -race ./internal/kernel -count=1`; `git diff --check`.
- Reference alignment: Codex keeps compaction execution in core/session logic while app shells only trigger a typed core operation; Reasonix records cache/context behavior and compaction lifecycle events outside frontend ownership. Genesis now follows the same control-plane split: Model Gateway records provider usage and tool-round accounting, while the kernel compaction runner owns selection, retry deferral, summary source construction, and compaction evidence.

### KERNEL-PROVIDER-CONTEXT-SESSION-HISTORY-20260622 - P1 - Provider context must define and preserve same-session conversation history

- Status: ready_for_acceptance.
- Conclusion: `ProviderContextProjection` now prepends a `conversation_history_context` model input fragment built from completed prior turns in the same session. `turn.submitted` records `model_input_kinds` including the history kind when history exists. The projection still omits event ids, operation ids, permission mode, audit fields, raw stdout, and raw stderr. `docs/kernel-contract.md` now states that session history is Model Gateway-owned and shells must not build their own model-visible history.
- Evidence: Fix commits: `ad05c9950`.; Verification: `TestSubmitTurnProviderContextIncludesSameSessionHistory`; `D:\software\Go\bin\go.exe test ./internal/kernel -run "TestSubmitTurnProviderContextIncludesSameSessionHistory|TestResolveProviderConfigFromGenesisRejectsSecretCommandEnvironment|TestArchitectureBoundaryKernelIssuesRequireReferenceAlignment" -count=1`; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `CGO_ENABLED=1 D:\software\Go\bin\go.exe test -race ./internal/kernel -count=1`; `git diff --check`; route version scan returned no matches.
- Reference alignment: Codex keeps conversation state inside the core thread/session context rather than asking shells to resend history, and Reasonix keeps frontend inputs behind a controller-owned context boundary. Genesis now projects same-session completed conversation history from the ledger through `ProviderContextProjection` instead of letting WebUI, provider commands, or external daemons synthesize model-visible history.

### KERNEL-PROVIDER-COMMAND-ENV-CREDENTIAL-BOUNDARY-20260622 - P1 - provider_command env must not bypass credential plane

- Status: ready_for_acceptance.
- Conclusion: `validateProviderCommandEnv` rejects secret-shaped env names and values before direct provider-command readiness or Genesis `models.json` resolution can pass them to a provider process. `ProviderConfigReason` returns `provider_command_env_secret_rejected`; direct daemon provider selection reports the same structured readiness blocker. README and `docs/kernel-contract.md` now document that `-provider-command-env` is for non-sensitive profile/route-style settings only.
- Evidence: Fix commits: `ad05c9950`.; Verification: `TestResolveProviderConfigFromGenesisRejectsSecretCommandEnvironment`; `TestBuildProviderBlocksSecretShapedCommandEnvironment`; `TestBuildProviderCanPassExplicitCommandEnvironment`; `D:\software\Go\bin\go.exe test ./cmd/genesisd -run "TestBuildProviderBlocksSecretShapedCommandEnvironment|TestBuildProviderCanPassExplicitCommandEnvironment" -count=1`; focused kernel tests; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `CGO_ENABLED=1 D:\software\Go\bin\go.exe test -race ./internal/kernel -count=1`; `git diff --check`.
- Reference alignment: Codex separates credentials and process environment policy from model-visible tool/context state, and Reasonix treats provider configuration as typed runtime config rather than an unbounded secret channel. Genesis now allows provider-command env only for non-sensitive adapter configuration while keeping provider credentials in the credential plane or in the external command's own identity environment.

### KERNEL-ARCHITECTURE-REFERENCE-GUARD-20260622 - P1 - reference-alignment governance guard should not be removed

- Status: ready_for_acceptance.
- Conclusion: `TestArchitectureBoundaryKernelIssuesRequireReferenceAlignment` was restored as a structure guard. It scans active `docs/operations/kernel-issues.md` `KERNEL-*` records and architecture-type or `KERNEL-BOUNDARY-*` retirement entries for `Reference alignment` or `Rejected drift risk`. `docs/operations/kernel-retirement-log.md` rules now match that scope instead of only mentioning `KERNEL-BOUNDARY-*`.
- Evidence: Fix commits: `ad05c9950`.; Verification: `TestArchitectureBoundaryKernelIssuesRequireReferenceAlignment`; focused kernel architecture tests; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `CGO_ENABLED=1 D:\software\Go\bin\go.exe test -race ./internal/kernel -count=1`; `git diff --check`.
- Reference alignment: Codex and Reasonix are external control-plane reference implementations for provider context, tool boundaries, event recovery, and shell/application separation. Genesis now keeps a lightweight executable guard requiring active kernel issues and architecture retirements to retain reference-alignment or explicit drift-risk evidence, without making the test a prose-quality judge.

### KERNEL-PROVIDER-GATEWAY-EVENT-PROJECTION-20260622 - P1 - Provider gateway should be driven by provider-visible event projection

- Status: ready_for_acceptance.
- Conclusion: `ProviderContextProjection` and `Kernel.ProviderContextProjection` derive provider-visible inputs, tool manifest, and tool rounds from stored events before each provider call. The turn loop sends that projection through `ModelRequest` to providers; `modelToolRoundsFromStoredEvents` no longer exposes `tool_call_event_id`; `provider_command` and OpenAI-compatible adapters consume model-visible tool call ids plus result content, not raw ledger or audit identity. Review fix `0721b4116` keeps shell ledger truth separate from redacted projections so provider context, audit replay, event inspection, and session projection cannot accidentally share one over-rich object. `docs/kernel-contract.md` now defines provider context as a projection boundary rather than a raw owner struct.
- Evidence: Fix commits: `0eb426a42`, `0721b4116`.; Verification: `TestObservabilityProjectionsSeparateRawAuditAndProviderContext`; `TestSessionProjectionRedactsTopLevelReadModels`; `TestEvidenceRedactionCoversBareProviderKeysAndJWT`; `TestExecShellRedactsSecretEvidenceInReturnedProjectionButPreservesLedgerTruth`; `TestSubmitTurnUsesToolCallEventIDWhenProviderIDMissing`; `TestSubmitTurnReturnsRepairFeedbackForInvalidShellArguments`; `D:\software\Go\bin\go.exe test ./internal/kernel -count=1`; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `CGO_ENABLED=1 D:\software\Go\bin\go.exe test -race ./internal/kernel -count=1`; `git diff --check`.
- Reference alignment: Codex separates provider/model-visible context from host event identity, and Reasonix separates controller facts from provider and frontend projections. Genesis now rebuilds provider context from the ledger while omitting kernel-owned event, operation, permission, and audit identity from the provider-visible request.

### KERNEL-EVENT-OBSERVABILITY-POLICY-20260622 - P1 - Separate UI timeline, raw event inspection, audit log, and provider context projections

- Status: ready_for_acceptance.
- Conclusion: `AuditReplayResponse`, `Kernel.AuditReplay`, and `GET /turns/{id}/audit` provide replay facts, operation statuses, provider-context input kinds, final usage, failure codes, and truncation metadata. `TurnEvents` and idempotency replay now return inspection events with redacted payload text. `inspectionEventData`, `toInspectionEvent`, and `redactSessionProjection` keep raw event envelopes and session top-level read models inspectable without exposing credential-shaped text. `ProviderContextProjection` omits audit, permission, raw operation, and kernel event identity while preserving model-visible tool result content. `shell_exec` now appends raw observed command/stdout/stderr to the local ledger, then returns and projects redacted evidence; redaction is a projection policy rather than a ledger mutation.
- Evidence: Fix commits: `0eb426a42`, `0721b4116`.; Verification: `TestObservabilityProjectionsSeparateRawAuditAndProviderContext`; `TestSessionProjectionRedactsTopLevelReadModels`; `TestEvidenceRedactionCoversBareProviderKeysAndJWT`; `TestExecShellRedactsSecretEvidenceInReturnedProjectionButPreservesLedgerTruth`; `D:\software\Go\bin\go.exe test ./internal/kernel -count=1`; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `CGO_ENABLED=1 D:\software\Go\bin\go.exe test -race ./internal/kernel -count=1`; `git diff --check`.
- Reference alignment: Codex and Reasonix keep transcript/timeline, protocol or raw event inspection, audit evidence, and provider context separate. Genesis now follows the same boundary: ordinary shells use `/sessions/{id}/timeline`, authorized debugging can use `/turns/{id}/events`, replay/export can use `/turns/{id}/audit`, and providers receive only `ProviderContextProjection`.

### KERNEL-LIVE-LLM-FIRST-RUN-ACCEPTANCE-20260622 - P0 - Real LLM must have a user-executable first-run acceptance path

- Status: ready_for_acceptance.
- Conclusion: `scripts/first_run_live_llm_acceptance.ps1` builds `genesisctl.exe` and `genesisd.exe`, writes Genesis provider config through `genesisctl provider-setup`, stores the key behind a `secret://...` ref, starts `genesisd` with `-provider genesis-config`, checks `/ready`, submits a real `/turn`, inspects `/sessions/{id}/timeline`, `/turns/{id}/events`, and `/turns/{id}/context`, restarts the server against the same ledger, replays those projections, and checks missing-credential readiness and turn failures. `docs/operations/live-llm-first-run-acceptance.md` documents the scripted and manual acceptance paths, and README links to the runbook from provider setup.
- Evidence: Fix commits: `fedbc58b3`.; Verification: `powershell -NoProfile -ExecutionPolicy Bypass -File scripts\first_run_live_llm_acceptance.ps1 -Help`; `powershell -NoProfile -ExecutionPolicy Bypass -Command '[scriptblock]::Create((Get-Content -Raw scripts\first_run_live_llm_acceptance.ps1)) | Out-Null; "ok"'`; local OpenAI-compatible stub run of `scripts\first_run_live_llm_acceptance.ps1` with a space-containing temp `-WorkRoot` returned `ok=true`, non-fake final text, timeline/events/context counts, restart replay counts, and `provider_credential_missing` / `provider_unavailable` failure probe; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `CGO_ENABLED=1 D:\software\Go\bin\go.exe test -race ./internal/kernel -count=1`; `git diff --check`.
- Reference alignment: Codex and Reasonix keep first-run and live-provider smoke paths executable by operators instead of hiding them only in tests. Genesis now has the same operator-facing acceptance surface while provider credentials and provider-specific account flows remain outside the kernel turn loop.

### KERNEL-UI-TIMELINE-PROJECTION-20260622 - P1 - WebUI needs a dedicated timeline projection instead of raw events

- Status: ready_for_acceptance.
- Conclusion: `Kernel.UITimeline` and `GET /sessions/{id}/timeline` project user messages, merged tool cards, assistant messages, and failure notices from ledger events. `tool.call` and `tool.result` are merged by `tool_result.for_event_id`; operation events and raw event names stay out of the main timeline. Timeline items expose preview metadata for tool output and omit kernel event ids, operation ids, provider tool-call ids, and raw event types.
- Evidence: Fix commits: `7df00cf45`.; Verification: `TestUITimelineProjectionMergesToolEventsWithoutAuditFields`; `go test ./... -count=1`; `go build ./...`; `CGO_ENABLED=1 go test -race ./internal/kernel -count=1`; `git diff --check`; active retired-concept and provider-native core scans returned no matches.
- Reference alignment: Reasonix separates event facts from display items and renders tool output through UI-specific cards. Genesis now provides a kernel-owned timeline read model so WebUI remains a shell and does not become the owner for raw event interpretation or tool-call/result merging.

### KERNEL-CONTEXT-INSPECTION-PROJECTION-20260622 - P1 - Need inspectable runtime context separate from chat timeline

- Status: ready_for_acceptance.
- Conclusion: `turn.submitted` now records model input kinds, tool manifest, safe skill catalog summaries, recalled memory refs, safe provider status, and permission/sandbox summary without storing the fully rendered model-context text in raw events. `Kernel.ContextInspection` and `GET /turns/{id}/context` rebuild that structured snapshot after restart. Older turns without snapshots report `snapshot_unavailable` instead of pretending current runtime state is historical context. Projection output redacts credential-shaped user text and excludes skill paths and bodies.
- Evidence: Fix commits: `7df00cf45`.; Verification: `TestContextInspectionProjectionPersistsProviderVisibleSnapshot`; `TestTurnEvidenceRecordsModelInputKindsWithoutSkillPaths`; `go test ./... -count=1`; `go build ./...`; `CGO_ENABLED=1 go test -race ./internal/kernel -count=1`; `git diff --check`; active retired-concept and provider-native core scans returned no matches.
- Reference alignment: Reasonix keeps controller context/status separate from transcript items. Genesis now records per-turn provider-visible context snapshots and exposes them through diagnostics inspection, not through the chat timeline.

### KERNEL-PROVIDER-CONTEXT-VISIBILITY-20260622 - P1 - Provider command request must not expose kernel-owned event identity as model-visible state

- Status: ready_for_acceptance.
- Conclusion: `provider_command` no longer serializes internal `ModelToolRound` directly to external command stdin. It now projects prior tool rounds through a provider-command DTO that preserves provider-visible `tool_call_id`, tool name, arguments, and tool result content while omitting `tool_call_event_id`, `event_id`, `operation_id`, `lease_id`, `permission_mode`, and `for_event_id`. Ledger/session projections still retain `tool_call_event_id` and `for_event_id` for audit, replay, and UI merging.
- Evidence: Fix commits: `cf81f3206`.; Verification: `TestProviderCommandRequestOmitsKernelEventIdentity`; `TestCommandProviderMalformedArgumentsReturnRepairFeedback`; focused provider-command/tool-loop tests; `go test ./... -count=1`; `go build ./...`; `CGO_ENABLED=1 go test -race ./internal/kernel -count=1`; `git diff --check`; active retired-concept and provider-native core scans returned no matches.
- Reference alignment: Codex separates model-visible call ids from host/runtime tool identity, and Reasonix keeps provider request payloads distinct from display or audit metadata. Genesis now preserves kernel event ids in ledger/session projections while projecting provider-command requests as model-visible context only.

### KERNEL-MALFORMED-TOOL-ARGS-REPAIR-20260622 - P1 - Malformed provider-command arguments should become model repair feedback

- Status: ready_for_acceptance.
- Conclusion: `provider_command` no longer rejects invalid raw tool arguments in `toModelResponse`. `ModelToolCall` now has one command-boundary shape for malformed arguments: valid JSON arguments use `arguments`, while malformed argument text uses `raw_arguments`. ToolGateway receives malformed arguments, writes `tool.call`, returns `tool_request_invalid`, writes linked `tool.result`, and executes no shell operation.
- Evidence: Fix commits: `b533a889c`, `d6886250d`, `be60fce1a`.; Verification: `TestCommandProviderMalformedArgumentsReturnRepairFeedback`; `TestOpenAICompatibleMalformedToolArgumentsReturnRepairFeedback`; `TestSubmitTurnReturnsRepairFeedbackForInvalidShellArguments`; `TestCommandProviderToolLoopThroughKernel`; focused provider-command/tool-repair/idempotency/readiness suite; `go test ./... -count=1`; `go build ./...`; `CGO_ENABLED=1 go test -race ./internal/kernel -count=1`; `git diff --check`; active retired-concept and provider-native core scans returned no matches.
- Reference alignment: Codex preserves provider protocol pairing while returning recoverable function-call argument errors through tool output when the loop can continue. Reasonix keeps provider tool-call ids for pairing and repairs malformed or missing tool-result pairing at the provider boundary rather than promoting malformed tool arguments into provider infrastructure failures.

### KERNEL-MODEL-SYSTEM-FIELD-BOUNDARY-20260622 - P1 - Model schemas must expose semantic fields only

- Status: ready_for_acceptance.
- Conclusion: `docs/kernel-contract.md` now defines semantic/user-supplied fields versus system-bound/audit-only fields. `shell_exec` continues to expose only `command` and optional `cwd` in the model-visible schema. Strict tool argument decoding rejects injected control-plane fields such as `permission_mode`, `event_id`, `operation_id`, `lease_id`, `task_id`, `tool_call_event_id`, and `provider_tool_call_id` as repairable invalid tool arguments without executing effects.
- Evidence: Fix commits: `d6886250d`.; Verification: `TestSubmitTurnReturnsRepairFeedbackForUnknownModelToolArgumentFields`; `go test ./... -count=1`; `go build ./...`; `CGO_ENABLED=1 go test -race ./internal/kernel -count=1`; `git diff --check`; active retired-concept and provider-native core scans returned no matches.
- Reference alignment: Codex keeps tool input schemas focused on model-action payloads while host identifiers, approvals, sandbox state, and event ids stay host-owned. Reasonix provider/tool abstractions keep provider call ids for pairing but do not ask models to generate host lifecycle ids.

### KERNEL-PROVIDER-GATEWAY-TRANSLATOR-20260622 - P1 - Provider wire compatibility belongs behind gateway translators

- Status: ready_for_acceptance.
- Conclusion: `provider_command` remains the long-lived command boundary for external provider translators, while the built-in OpenAI-compatible adapter is treated as an adapter/translator file. Provider command stderr is redacted before HTTP, event, session, or ledger projection, and command processes inherit only explicitly configured environment variables. `TestArchitectureBoundaryProviderWireTermsStayInsideAdapterFiles` scans runtime Go files and fails if `/chat/completions`, chat-completion structs, token usage wire names, DeepSeek, OpenRouter, or `openai-responses` terms appear outside the explicit adapter file allowlist. `TestArchitectureBoundaryCoreLoopHasNoProviderNativeWireTerms` continues to guard the turn loop, ToolGateway, provider interface, command provider, tool registry, and core types.
- Evidence: Fix commits: `10c11da35`, `d6886250d`, `be60fce1a`.; Verification: `TestArchitectureBoundaryProviderWireTermsStayInsideAdapterFiles`; `TestArchitectureBoundaryCoreLoopHasNoProviderNativeWireTerms`; `TestCommandProviderToolLoopThroughKernel`; `TestProviderCommandFailureRedactsStderrFromTurnAndHTTP`; `TestSubmitTurnExecutesOpenAICompatibleToolCallBeforeFinal`; `go test ./... -count=1`; `go build ./...`; `CGO_ENABLED=1 go test -race ./internal/kernel -count=1`; `git diff --check`; active retired-concept and provider-native core scans returned no matches.
- Reference alignment: Codex keeps provider wire protocol handling behind API/client/protocol modules while the core tool loop consumes typed items. Reasonix registers provider implementations behind a provider abstraction and keeps OpenAI/Anthropic wire terms inside provider packages. Genesis now treats `provider_command` as the preferred provider boundary and constrains provider-native wire terms to adapter/translator files.

### KERNEL-TOOL-CALL-EVENT-ID-20260622 - P1 - Tool call identity should be kernel event id

- Status: ready_for_acceptance.
- Conclusion: `SubmitTurn` now writes `tool.call` events before tool preparation and normalizes each admitted tool slot with two explicit identities: provider-visible `tool_call_id` remains the provider echo id, while kernel-owned `tool_call_event_id` is the `tool.call` event id used for operation idempotency, audit, replay linkage, and `tool.result.for_event_id`. Session event projections store `tool_call_event_id` plus `provider_tool_call_id`, so provider-native ids remain correlation evidence rather than kernel owner truth. Duplicate provider ids in one model batch fail before `tool.call` events or effects. Provider calls with missing or unsafe native ids can still execute through a kernel event id without promoting provider strings into operation identity.
- Evidence: Fix commits: `a4e57c86f`, `fe25c2f7f`.; Verification: `TestSubmitTurnExecutesOpenAICompatibleToolCallBeforeFinal`; `TestSubmitTurnUsesToolCallEventIDWhenProviderIDMissing`; `TestSubmitTurnUsesKernelEventIDForUnsafeProviderToolCallID`; `TestSubmitTurnRejectsDuplicateToolCallIDBeforeAnyEffect`; `TestSubmitTurnReturnsRepairFeedbackForInvalidShellArguments`; `TestOpenAICompatibleMalformedToolArgumentsReturnRepairFeedback`; `TestCommandProviderToolLoopThroughKernel`; `go test ./... -count=1`; `go build ./...`; `CGO_ENABLED=1 go test -race ./internal/kernel -count=1`; `git diff --check`; active retired-concept and provider-native core scans returned no matches.
- Reference alignment: Codex distinguishes provider protocol correlation from internal event/control flow, and tool routing stays typed. Reasonix event-style flows keep local event identity separate from transport correlation. Genesis now keeps provider correlation as adapter data and uses ledger event ids for kernel tool facts.

### KERNEL-PROVIDER-COMMAND-ADAPTER-20260622 - P1 - Provider should prefer external command adapter boundary

- Status: ready_for_acceptance.
- Conclusion: `CommandProvider` runs a configured external executable, writes one `genesis.provider_command` JSON request to stdin, and accepts one stdout response with `kind=final` or `kind=tool_calls`. It now runs with explicit environment variables only, applies a default bounded timeout when the caller does not configure one, and redacts command stderr before projecting provider failures. `ResolveProviderConfigFromGenesis` can resolve `models.json` routes with `protocol=provider_command`, and `genesisd` can build the command provider through `genesis-config` or direct `-provider provider_command`. The built-in OpenAI-compatible adapter remains available as an operator convenience but is documented as not being the default contract for new provider integrations. `TestArchitectureBoundaryCoreLoopHasNoProviderNativeWireTerms` prevents core turn/tool files from importing OpenAI-native wire terms.
- Evidence: Fix commits: `10c11da35`, `b533a889c`, `be60fce1a`.; Verification: `TestCommandProviderCompletesFromTypedStdoutEvent`; `TestCommandProviderToolLoopThroughKernel`; `TestCommandProviderRejectsInvalidAdapterResults`; `TestCommandProviderDoesNotInheritDaemonEnvironment`; `TestProviderCommandFailureRedactsStderrFromTurnAndHTTP`; `TestCommandProviderAppliesDefaultTimeout`; `TestResolveProviderConfigFromGenesisSelectsCommandProviderRoute`; `TestBuildProviderFromGenesisConfigCanSelectCommandProvider`; `TestArchitectureBoundaryCoreLoopHasNoProviderNativeWireTerms`; `go test ./... -count=1`; `go build ./...`; `CGO_ENABLED=1 go test -race ./internal/kernel -count=1`; `git diff --check`; active retired-concept and provider-native core scans returned no matches.
- Reference alignment: Codex keeps provider wire protocols behind typed model-client surfaces and dispatches tools through typed tool routing. Reasonix keeps provider/tool/plugin concepts registry-driven and uses stdio transport hygiene for extension boundaries. Genesis now keeps `ModelRequest`, `ModelResponse`, `ModelToolCall`, `ModelToolRound`, `ToolSpec`, and session ledger events as the kernel contract while `provider_command` owns the provider step transport.

### KERNEL-MODEL-VISIBLE-TOOL-RESULT-MINIMAL-20260622 - P1 - Model-visible tool results should exclude permission and audit fields

- Status: ready_for_acceptance.
- Conclusion: `modelOperationResult` now returns only terminal-equivalent command evidence: status, executed flag, exit code, bounded stdout/stderr, and truncation metadata. Permission blocks return model-visible `permission_denied` feedback without permission mode, blocker reason, operation id, command, cwd, timestamps, or infrastructure reason. Full `OperationProjection` still records permission mode and blocker reason in session/operation inspection.
- Evidence: Fix commits: `b533a889c`.; Verification: `TestSubmitTurnExecutesOpenAICompatibleToolCallBeforeFinal`; `TestSubmitTurnFeedsNonZeroShellExitToModel`; `TestSubmitTurnReturnsMinimalPermissionDeniedToolResult`; `go test ./... -count=1`; `go build ./...`; `CGO_ENABLED=1 go test -race ./internal/kernel -count=1`; `git diff --check`; active retired-concept and provider-native core scans returned no matches.
- Reference alignment: Codex models terminal output and structured tool errors separately from sandbox, approval, and audit state. Reasonix keeps policy/control metadata out of provider-facing tool content. Genesis now keeps the LLM in the operator role while the kernel retains permission, audit, and recovery evidence in inspection projections.

### KERNEL-SESSION-EVENT-STREAM-UNIFICATION-20260622 - P1 - Session facts should converge on typed event stream

- Status: ready_for_acceptance.
- Conclusion: `docs/kernel-contract.md` defines session events as the primary fact stream and states that session, turn, operation, work, and memory views are derived read models. `SubmitTurn` writes `tool.call`, turn-scoped `operation.*`, and `tool.result` events, with `tool_result.for_event_id` pointing to the originating `tool.call`. Provider replay now rebuilds model tool rounds from stored turn events instead of transient in-memory state. `GET /sessions/{id}` projects ordered events with typed payload data. Long-term tests now assert the current event contract and do not lock retired event/tool names.
- Evidence: Fix commits: `16efa7e86`.; Verification: focused provider tool-loop event tests; `go test ./internal/kernel -count=1`; `go test ./... -count=1`; `go build ./...`; `CGO_ENABLED=1 go test -race ./internal/kernel -count=1`; `git diff --check`; active code/docs retired-concept scan returned no matches.
- Reference alignment: Codex protocol surfaces ordered events and explicit tool call/result relationships; Reasonix-style controller flows keep lifecycle facts behind one control surface. Genesis now treats the session event stream as the fact source and keeps object projections as ledger-derived read models only.

### KERNEL-TOOL-GATEWAY-REGISTRY-20260622 - P1 - Runtime should execute tools only through ToolGateway

- Status: ready_for_acceptance.
- Conclusion: `ToolRegistry` now exposes `ToolSpec` records with `name`, `description`, `input_schema`, `side_effect_level`, and `execution_kind`; `ToolGateway` owns provider tool batch preflight and execution. `SubmitTurn` calls only `ToolGateway.ToolManifest`, `ToolGateway.PrepareBatch`, and `ToolGateway.Execute` for model tool handling. Direct `POST /tools/shell_exec` also enters the same gateway before shell execution. `TestArchitectureBoundaryToolRegistryBindsSurface` and `TestArchitectureBoundaryToolRegistryRejectsIncompleteSpecs` prove tool specs cannot omit required registry fields or use provider-unsafe dotted ids. `TestSubmitTurnProjectsRegisteredToolManifestWithSkillCatalog` proves the provider sees the registry-generated manifest, not a hand-built provider descriptor. Existing generic unsupported-tool and mixed-batch tests continue to prove unregistered tools return model repair feedback without executing effects. Long-term tests that locked retired tool names were removed; active code/docs scans now return no matches for retired tool ids or old registry helper names outside this retirement log.
- Evidence: Fix commits: `c34d2baf5`.; Verification: `go test ./internal/kernel -count=1`; `go test ./... -count=1`; `go build ./...`; `CGO_ENABLED=1 go test -race ./internal/kernel -count=1`; `git diff --check`; active code/docs retired-concept scan returned no matches.
- Reference alignment: Codex ties model-visible tool metadata to governed tool executors, and Reasonix routes agent tool calls through a runtime registry. Genesis now uses one `ToolRegistry` for tool name, description, input schema, side-effect level, execution kind, and executor binding; the turn loop and provider adapters consume a gateway-generated manifest instead of knowing concrete tool execution paths.

### KERNEL-BOUNDARY-REFERENCE-ALIGNMENT-20260622 - P1 - Kernel changes need reference-alignment notes against Codex and Reasonix

- Status: ready_for_acceptance.
- Conclusion: `TestArchitectureBoundaryKernelIssuesRequireReferenceAlignment` proves every active `KERNEL-*` issue has a `Reference alignment` field and future `KERNEL-BOUNDARY-*` retirement entries retain that field. The test intentionally checks structure rather than prose quality, so governance does not become a content judge. Verification passed: focused architecture boundary tests; `go test ./... -count=1`; `go build ./...`; `go test -race ./internal/kernel -count=1`; `git diff --check`; versioned route scan returned no matches.
- Evidence: Fix commits: `b8a013be4`.
- Reference alignment: Codex is the reference for terminal-equivalent tool results, approval/sandbox rigor, and protocol separation; Reasonix is the reference for registry-driven provider/tool/plugin loading and frontend-agnostic control. Genesis now requires issue records to compare those control-plane ideas explicitly instead of relying on review memory or superficial name matching.

### KERNEL-BOUNDARY-SHELL-MINI-RUNTIME-20260622 - P1 - Default shell mode risks becoming a mini shell implementation

- Status: ready_for_acceptance.
- Conclusion: `TestArchitectureBoundaryControlledShellAllowlistStaysSmall` locks the tiny default allowlist; `TestArchitectureBoundaryShellGoOnlyOwnsOrchestration` prevents `shell.go` from importing filesystem/process/path/runtime/syscall packages or redeclaring adapter/parser/runtime functions; `TestArchitectureBoundaryShellRuntimeHasNoApplicationAliases` prevents application/channel aliases in shell runtime files. `TestExecShellDefaultBlocksHardlinkAlias`, `TestExecShellDefaultBlocksRawShellAndEnvironmentAccess`, and existing workspace escape/link tests prove default mode remains controlled rather than a broad shell. Verification passed: focused shell/architecture tests; `go test ./... -count=1`; `go build ./...`; `go test -race ./internal/kernel -count=1`; `git diff --check`; versioned route and shell runtime application-alias scans returned no matches.
- Evidence: Fix commits: `bdf879293`.
- Reference alignment: Codex keeps terminal execution behind a governed process/sandbox path and returns terminal-equivalent results; Reasonix treats shell execution as a tool behind permission/sandbox gates. Genesis now keeps `shell.go` focused on operation orchestration and moves default-mode command parsing, workspace containment, link checks, and raw host-shell execution behind separate runtime adapter files.

### KERNEL-BOUNDARY-PERMISSION-GATE-20260622 - P0 - Permission policy is embedded in shell execution instead of a generic gate

- Status: ready_for_acceptance.
- Conclusion: `authorizeKernelTool` now allows read tools, blocks effect tools in `plan`, allows effect tools in `default` and `yolo`, and fails closed for unknown modes/kinds. `TestArchitectureBoundaryAuthorityGateUsesToolKind` proves those decisions flow from tool kind; `TestExecShellPlanBlocksMutatingCommand` proves shell execution respects the generic gate; `TestSubmitTurnExecutesOpenAICompatibleToolCallBeforeFinal` proves model-requested shell tools pass through the same kernel tool path. Verification passed: focused registry/gate/tool-loop tests; `go test ./... -count=1`; `go build ./...`; `go test -race ./internal/kernel -count=1`; `git diff --check`; versioned route scan returned no matches.
- Evidence: Fix commits: `0729dc4b0`.
- Reference alignment: Reasonix separates pure permission policy/gates from tool implementations; Codex separates approval/sandbox decisions from concrete exec result handling. Genesis now asks a kernel-owned authority gate before effects, so `shell.exec` is one effectful tool under the gate rather than the owner of permission semantics.

### KERNEL-BOUNDARY-TOOL-REGISTRY-20260622 - P0 - Tool descriptors are still hardcoded in the kernel instead of owned by a registry

- Status: ready_for_acceptance.
- Conclusion: the tool registry is now the source for the canonical `shell_exec` tool; model descriptors, capability projection, and model tool preflight project from that registry. Skills are projected separately as path-free metadata catalog entries, not as registered model-visible tools. `TestArchitectureBoundaryToolRegistryBindsSurface` proves every visible tool has descriptor, kind, parameter schema, and handler binding; `TestArchitectureBoundaryCapabilitiesProjectFromToolRegistry` proves capability projection follows the registry; `TestArchitectureBoundaryModelVisibleToolsStayGeneric` prevents Feishu/email/calendar-like application names from entering descriptors. Verification passed: focused registry/gate/tool-loop tests; `go test ./... -count=1`; `go build ./...`; `go test -race ./internal/kernel -count=1`; `git diff --check`; versioned route scan returned no matches.
- Evidence: Fix commits: `0729dc4b0`.
- Reference alignment: Reasonix registers built-ins and plugin tools into a runtime registry; Codex ties model-visible tool metadata to executor contracts. Genesis now has a small `kernelToolDefinition` registry binding descriptor, read/effect kind, and prepare handler for canonical kernel tools.

### KERNEL-BOUNDARY-SEMANTIC-TEXT-20260622 - P0 - Semantic text must not be rejected by secret-shaped heuristics

- Status: ready_for_acceptance.
- Conclusion: `TestSemanticTextFieldsAllowSecretShapedContent` proves Work title, Work cancel reason, memory approval reason, memory rejection reason, memory supersession reason, and supersession replacement text can contain secret-shaped quoted content and are preserved through HTTP owner paths. `TestArchitectureBoundarySemanticFieldsDoNotUseSecretRejector` proves those narrative fields no longer call the secret-shaped rejector. `TestHTTPCreateWorkRejectsInvalidControlRefs`, `TestHTTPCancelWorkRejectsInvalidControlRefs`, `TestHTTPCreateMemoryCandidateRejectsInvalidControlRefs`, `TestHTTPApproveMemoryCandidateRejectsInvalidControlRefs`, `TestHTTPRejectMemoryCandidateRejectsInvalidControlRefs`, and `TestHTTPMemoryCandidateSupersedeRejectsInvalidControlRefs` prove control-plane identifiers, refs, and authorities still fail closed when malformed or secret-shaped. Verification passed: focused semantic/control-ref tests; `go test ./... -count=1`; `go build ./...`; `go test -race ./internal/kernel -count=1`; `git diff --check`; versioned route scan returned no matches.
- Evidence: Fix commits: `3fb91aa8e`.
- Reference alignment: Codex preserves model/user strings as repair feedback or terminal-equivalent tool evidence rather than rejecting them because they resemble secrets; Reasonix keeps schema/permission failures separate from arbitrary narrative content. Genesis now keeps control-plane refs, authorities, session ids, and retry keys grammar-gated while admitting ordinary WorkRegistry and Accumulation narrative text as text.

### KERNEL-TOOL-RESULT-TAXONOMY-20260622 - P0 - Tool loop must preserve terminal-equivalent command results

- Status: ready_for_acceptance.
- Conclusion: `TestSubmitTurnReturnsRepairFeedbackForInvalidShellArguments` first failed because malformed `shell.exec` arguments terminated the turn as `model tool call rejected`, then passed after the Tool System began returning structured `tool_request_invalid` repair feedback to the model without executing an operation. `TestSubmitTurnRejectsInvalidToolCallIDBeforeToolCallEvent` proves an unrecoverable bad `tool_call_id` fails before `model.tool_call` evidence is appended. `TestSubmitTurnReturnsRepairFeedbackForMixedModelToolBatchBeforeAnyEffect`, `TestSubmitTurnReturnsRepairFeedbackForUnknownModelToolArgumentFields`, `TestSubmitTurnReturnsRepairFeedbackForUnknownSkillReadBeforeShellEffect`, and `TestSubmitTurnReturnsRepairFeedbackForChangedSkillReadBatchBeforeShellEffect` prove invalid mixed batches create no shell effects while returning repair results for each call. `TestSubmitTurnFeedsNonZeroShellExitToModel` proves an executed shell command with nonzero exit returns model-visible `status=failed`, `exit_code`, and stderr while omitting ledger control-plane handles from the model-facing tool result. `TestExecShellReportsHeadTailTruncationMetadata` proves long stdout/stderr are bounded with head/tail content, truncation flags, original byte counts, omitted byte counts, `output_truncation=head_tail`, and a visible `[... N bytes omitted ...]` marker between preserved head and tail content. `TestSubmitTurnReportsToolInfrastructureFailureSeparately` proves ledger/tool infrastructure failure records `tool_infrastructure_failed` rather than a command stderr. `TestSubmitTurnSkillReadUnavailableRepairDoesNotExposeInstructionPath` and `TestExecShellControlledReadFailureDoesNotExposeAbsolutePath` were added after security review to prove internal file paths are not surfaced through repair messages or controlled-shell synthetic stderr. Independent architecture review found model-visible repair content duplicated `tool_call_id` and that docs incorrectly classified provider adapter errors as Tool System infrastructure; both were fixed. A brief over-sanitizing attempt for unknown tool names was rejected after user review because it diverged from Codex-style repair feedback and local terminal semantics. Reopen verification passed: `go test -count=1 ./internal/kernel -run "TestExecShellReportsHeadTailTruncationMetadata|TestSubmitTurnFeedsNonZeroShellExitToModel|TestSubmitTurnReturnsRepairFeedbackForInvalidShellArguments|TestSubmitTurnReportsToolInfrastructureFailureSeparately" -v`; `go test ./... -count=1`; `go build ./...`; `CGO_ENABLED=1 go test -race ./internal/kernel -count=1`; `git diff --check`; the repository scan for numbered route prefixes returned no matches.
- Evidence: Fix commits: `0c7960172`, `fb519b7ae`.

### KERNEL-CONTEXT-PROVENANCE-20260622 - P2 - Model input context needs provenance categories

- Status: ready_for_acceptance.
- Conclusion: `ModelRequest.InputItems` now uses typed `ModelInputItem` fragments instead of public `InputItem`; initial kinds are `skill_catalog_context`, `approved_memory_context`, and `user_text`. `TestModelInputItemsInjectsApprovedMemoryContextBeforeProvider` proves approved memory is no longer just another anonymous user text item. `TestKernelInjectsSkillCatalogBeforeProviderWithoutSkillBodies` proves skill catalog context is categorized before user text and still excludes full skill bodies and paths. `TestTurnEvidenceRecordsModelInputKindsWithoutSkillPaths` proves provider request, turn event data, and session projection expose ordered `model_input_kinds` for skill catalog, approved memory, and user text while the public `input_items` projection remains the original user text only and inspection JSON does not leak instruction paths, skill bodies, or injected catalog text. `TestOpenAICompatibleProviderCompletesAgainstCompatibleServer` and `TestKernelBuildsApprovedMemoryContextBeforeOpenAICompatibleProvider` prove the OpenAI-compatible adapter consumes owner-built model inputs as transport content rather than owning memory or skill context semantics. Verification passed: focused provenance/provider tests; `go test ./... -count=1`; `go test -race ./internal/kernel -count=1`; `go build ./...`; `git diff --check`; the repository scan for numbered route prefixes or kernel version labels returned no matches.
- Evidence: Fix commits: `1c8ec0d88`.

### KERNEL-BUILD-BROKEN-MODEL-INPUT-PROVENANCE-20260622 - P0 - ModelInputItem provenance refactor must compile

- Status: ready_for_acceptance.
- Conclusion: The temporary dirty provenance refactor left several tests and provider stubs on the old `InputItem` contract. Commit `1c8ec0d88` completed the migration by updating fake provider, OpenAI-compatible provider tests, skill catalog provider stubs, typed model context assembly, and session/turn evidence. Verification passed after the migration: focused provenance/provider tests; `go test ./... -count=1`; `go test -race ./internal/kernel -count=1`; `go build ./...`; `git diff --check`; the repository scan for numbered route prefixes or kernel version labels returned no matches. Commit `ba66248fd` records the matching acceptance evidence.
- Evidence: Fix commits: `1c8ec0d88`, `ba66248fd`.

### KERNEL-SKILL-READ-BOUNDARY-20260622 - P1 - Skill instructions should not be a first model-visible kernel tool

- Status: ready_for_acceptance.
- Conclusion: The default kernel tool registry now exposes only `shell_exec`; the former skill-instruction descriptor, OpenAI-compatible translation alias, projection struct, instruction-body reader, and prepare path were removed. `TestSubmitTurnDoesNotExposeSkillReadAsModelTool` proves provider tool manifests include `shell_exec` and exclude the removed skill-instruction tool and provider alias. `TestHTTPCapabilitiesProjectsToolsAndSkillCatalogWithoutPaths` proves `/capabilities` still exposes safe skill name/description metadata while excluding skill-instruction tools, paths, and bodies. Unsupported requests for the removed tool now exercise the generic unsupported-tool repair path, with mixed batches still producing no shell effect. Verification passed: focused skill boundary/capability/tool tests; `go test ./... -count=1`; `go test -race ./internal/kernel -count=1`; `go build ./...`; `git diff --check`; the repository scan for numbered route prefixes or kernel version labels returned no matches.
- Evidence: Fix commits: `0ff18a793`.
- Reference alignment: Codex does not expose a model tool for reading skill packages or agent guidance files directly, and Reasonix keeps skill/task concepts separate from ordinary file/process tools. Genesis now keeps configured skills as metadata-only user-space context until a generic resource/context contract exists.

### KERNEL-TOOL-NAMING-UNDERSCORE-20260622 - P1 - Canonical tool ids should not keep dotted names

- Status: ready_for_acceptance.
- Conclusion: The default tool id is now `shell_exec` in the registry, provider tool specs, model tool calls/results, operation ledger evidence, session projection, capability projection, README, contract docs, and HTTP route `POST /tools/shell_exec`. The OpenAI-compatible adapter no longer has dotted-name mapping functions. `TestArchitectureBoundaryToolRegistryBindsSurface` now fails any dotted tool id at the registry boundary. `TestSubmitTurnExecutesOpenAICompatibleToolCallBeforeFinal` proves provider request, assistant tool replay, and model-visible operation evidence use `shell_exec`. `TestLiveOpenAICompatibleProviderToolLoopThroughKernel` passed against the real configured provider using `shell_exec`. Verification passed: focused naming/provider/HTTP tests; `go test ./... -count=1`; `go test -race ./internal/kernel -count=1`; `go build ./...`; gated live provider tool-loop smoke; `git diff --check`; active code/current docs scans for dotted shell id and adapter mapping functions returned no matches; the versioned route scan returned no matches.
- Evidence: Fix commits: `bbcc60636`.
- Reference alignment: Codex tool names are provider-safe identifiers such as `exec_command` and `apply_patch`; Reasonix uses names such as `read_file`, `write_file`, and `bash_output`. Genesis now uses a provider-safe canonical id for the surviving default tool rather than maintaining separate kernel and provider names.

### KERNEL-CAPABILITIES-20260622 - P1 - Shells and daemons need a protected kernel capability projection

- Status: ready_for_acceptance.
- Conclusion: `TestHTTPCapabilitiesRequiresRuntimeAuth` first failed because `/capabilities` returned 404, then passed after implementation and proves the route requires the runtime bearer token. `TestHTTPCapabilitiesProjectsToolsAndSkillCatalogWithoutPaths` proves authenticated inspection returns canonical `shell_exec` capability metadata plus safe skill name/description metadata, while excluding `instruction_path`, `ledger_path`, filesystem paths, and skill bodies. `TestHTTPCapabilitiesReportsPathFreeSkillExclusions` proves missing roots, malformed metadata, unsafe metadata, linked paths, and duplicate skill names are projected only as path-free reason/count diagnostics. `TestHTTPCapabilitiesSanitizesProviderInspectionStatus` and `TestHTTPCapabilitiesSanitizesCredentialShapedProviderTokens` were added after security review found provider readiness strings could leak secret-shaped single tokens; they now prove path-shaped provider names, `secret://...`, `Authorization: Bearer ...`, bare `sk-...`, and embedded `sk-proj-...` tokens are replaced with safe inspection fallbacks. `TestToolCapabilityKindDefaultsUnknown` proves future tools are not silently classified as effectful capabilities unless explicitly mapped. `go test ./internal/kernel -run "TestHTTPCapabilities|TestToolCapabilityKindDefaultsUnknown" -count=1` passed; `go test ./... -count=1` passed; `go build ./...` passed; `go test -race ./internal/kernel -count=1` passed; `git diff --check` passed; the repository scan for numbered route prefixes returned no matches. Independent architecture review reported no kernel/app boundary blocker. Independent security review initially found the provider inspection sanitizer leak; after the credential-shaped token fix it reported no blocking findings.
- Evidence: Fix commits: `2654f0877`, `65f004277`.

### KERNEL-SKILL-READ-20260622 - P0 - Model loop needs governed skill instruction retrieval

- Status: superseded by `KERNEL-SKILL-READ-BOUNDARY-20260622`; this is not an active acceptance condition.
- Conclusion: the later boundary fix removed the skill-specific descriptor, provider alias, projection struct, instruction-body reader, and prepare path. Current positive tests prove provider tool manifests contain `shell_exec`, capability projection exposes path-free skill catalog metadata, and unsupported removed-tool requests flow through the generic repair path without shell effects. `go test ./... -count=1` passed; `go test -race ./internal/kernel -count=1` passed; `go build ./...` passed; `git diff --check` passed.
- Evidence: Fix commits: `ff92814db`, `f89f55409`.

### KERNEL-SKILL-METADATA-SECURITY-20260622 - P1 - Skill catalog metadata must not inject authority-shaped context

- Status: ready_for_acceptance.
- Conclusion: `TestSkillCatalogRejectsAuthorityAndSecretShapedMetadata` first failed because prompt-injection-shaped, role-marker, tool-protocol, and secret-shaped skill descriptions all entered the catalog. After the fix, it proves only safe skill metadata is injected, while `Ignore previous instructions`, `system:`, `tool_call_id`, invisible-control text, secret-shaped metadata, and full skill bodies are absent from the skill catalog context. Existing `TestKernelInjectsSkillCatalogBeforeProviderWithoutSkillBodies`, `TestMissingAndMalformedSkillCatalogDoesNotBlockTurn`, `TestModelInputItemsInjectsApprovedMemoryContextBeforeProvider`, and `TestKernelBuildsApprovedMemoryContextBeforeOpenAICompatibleProvider` passed, proving safe catalog injection, fail-soft missing/malformed roots, approved memory context, and provider request construction still work. `go test ./...` passed; `go test -race ./internal/kernel -count=1` passed; `go build` passed for both `cmd/genesisd` and `cmd/genesisctl`; `git diff --check` passed; the repository scan for numbered route or kernel version labels returned no matches.
- Evidence: Fix commits: `fc27be8df`, `152c7d102`.

### KERNEL-SKILL-CATALOG-20260622 - P0 - Model context needs generic external skill discovery

- Status: ready_for_acceptance.
- Conclusion: `TestKernelInjectsSkillCatalogBeforeProviderWithoutSkillBodies` first failed because `Config.SkillRoots` did not exist, then passed after implementation. It now proves a configured root containing `lark-im/SKILL.md` and `mail/SKILL.md` injects a concise "Available external skills" catalog before the user turn, includes each skill name and description, keeps filesystem paths internal, and does not inject full skill bodies. `TestMissingAndMalformedSkillCatalogDoesNotBlockTurn` proves missing roots and malformed `SKILL.md` metadata are ignored without blocking turn submission. Existing `TestModelInputItemsInjectsApprovedMemoryContextBeforeProvider` and `TestKernelBuildsApprovedMemoryContextBeforeOpenAICompatibleProvider` passed, proving approved memory context and provider request construction still work with the extended model input path. `go test ./...` passed; `go test -race ./internal/kernel -count=1` passed; `go build` passed for both `cmd/genesisd` and `cmd/genesisctl`; `git diff --check` passed; the repository scan for numbered route or kernel version labels returned no matches.
- Evidence: Fix commits: `c3e20a777`, `5b9b7f0c9`.

### KERNEL-TURN-IDEMPOTENCY-20260622 - P0 - Turn submit retries must not create duplicate model/tool effects

- Status: ready_for_acceptance.
- Conclusion: `TestHTTPTurnSubmitIdempotencyKeyReturnsExistingTurnAfterRestart` first failed because `idempotency_key` was rejected as an unknown `POST /turn` field, then passed after implementation. It proves a duplicate `session_id + idempotency_key` retry after restart returns the original `turn_id` and final answer, does not call the retry provider, and leaves one turn plus two turn events in session projection. `TestHTTPTurnSubmitIdempotencyKeyReturnsExistingFailureAfterRestart` proves a failed provider turn replays the original failure on retry without calling a now-available provider, so the same caller retry boundary cannot silently change effects. A later review found the retry still returned only an HTTP error envelope for failed turns, forcing shells to fetch `/sessions` for evidence. Fix commit `190cd56d9` changes failed idempotent retries to return the original failed `turn_id`, ordered events, and `error.code` from `POST /turn` while preserving the failure HTTP status. `TestHTTPTurnSubmitIdempotencyKeyRequiresValidExplicitSession` proves malformed idempotency keys and keys without explicit `session_id` fail before ledger append. The broader focused suite covering turn admission, provider failure, model tool loop, shell idempotency, and work idempotency passed; `go test ./...` passed; `go test -race ./internal/kernel -count=1` passed; `go build` passed for both `cmd/genesisd` and `cmd/genesisctl`; `git diff --check` passed; the repository scan for numbered route or kernel version labels returned no matches.
- Evidence: Fix commits: `18b6e029e`, `7277032e2`, `190cd56d9`.

### KERNEL-WORK-IDEMPOTENCY-20260622 - P1 - Work submit retries create duplicate work records

- Status: ready_for_acceptance.
- Conclusion: `TestHTTPWorkSubmitIdempotencyKeyReturnsExistingWorkAfterRestart` proves duplicate `POST /work` calls with the same `session_id + idempotency_key` return the original work after restart, preserve the original title/source, and append only one `work.submitted` event. `TestHTTPCreateWorkRejectsInvalidIdempotencyKey` proves malformed retry keys fail before ledger append. The broader WorkRegistry evidence in `KERNEL-WORK-REGISTRY-20260622` also proves submit/read/cancel projection, cancel idempotency, terminal cancel race handling, invalid source/audit ref rejection, and restart-safe session projection. Current verification reran the Work idempotency tests as part of the broader turn/tool/work idempotency suite, then `go test ./...`, `go test -race ./internal/kernel -count=1`, both binary builds, `git diff --check`, and the no-version scan passed.
- Evidence: Fix commits: `71f4abc95`, `3e606b5a0`, `231363fd5`.

### KERNEL-MEMORY-RECALL-20260622 - P1 - Memory recall needs an explicit kernel observation surface

- Status: ready_for_acceptance.
- Conclusion: `TestHTTPMemoryRecallReturnsApprovedOnlyAfterRestartWithoutLedgerAppend` first failed with HTTP 404 for `POST /memory/recall`, then passed after implementation. It proves the protected recall preview returns approved memory refs after restart, excludes pending, rejected, superseded, and pending replacement candidates that would otherwise match the same query, and does not append ledger events. `TestHTTPMemoryRecallRejectsBadInputAndAuth` proves missing runtime auth returns 401, unsupported input item types return 400 before recall, and hidden control text returns 403. Existing turn recall and ingress tests still pass. `go test ./internal/kernel -run "TestHTTPMemoryRecall" -count=1 -v` passed; `go test ./internal/kernel -run "TestHTTPMemory|TestApprovedMemory|TestUnapprovedMemory|TestSubmitTurnRecordsIngressRisk|TestHTTPAcceptsRiskyUserData|TestHTTPRejectsNestedControlField|TestHTTPBlocksInvisibleIngressMarker" -count=1 -v` passed; `go test ./...` passed; `go test -race ./internal/kernel -count=1` passed; `go build` passed for both `cmd/genesisd` and `cmd/genesisctl`; `git diff --check` passed; the repository scan for numbered route or kernel version labels returned no matches.
- Evidence: Fix commits: `0ec14a963`, `094a67559`.

### KERNEL-MEMORY-SUPERSEDE-20260622 - P1 - Memory review needs explicit supersession

- Status: ready_for_acceptance.
- Conclusion: `TestHTTPMemoryCandidateSupersedeCreatesPendingReplacementAfterRestart` first failed because `MemorySupersessionProjection`, `MemoryCandidateSuperseded`, and `SupersedeMemoryCandidate` did not exist, then passed after implementation. It proves an approved candidate can be superseded through `POST /memory/candidates/{id}/supersede`, the original candidate replays as `status=superseded` with authority/reason/evidence and `replacement_candidate_id`, the replacement candidate replays as `status=pending`, superseded and pending replacement memories are excluded from recall, and the replacement recalls only after separate approval. `TestSupersedeMemoryCandidateIsIdempotentWithoutAppendingDuplicateReplacement` proves duplicate supersede calls preserve the first replacement and append only one `memory.candidate.superseded` event. `TestHTTPMemoryCandidateSupersedeRejectsMissingEvidence` proves supersession requires replacement text, replacement source, authority, reason, and evidence before candidate lookup. `TestHTTPSupersededMemoryCandidateCannotBeApprovedOrRejected` proves the original superseded candidate cannot later be approved or rejected through the minimal review surface. `TestMemoryReplayRejectsReviewAfterSupersede` proves replay fails closed if a corrupted ledger tries to apply approval after supersession. Review-fix `TestMemoryReplayRejectsDuplicateSupersedeWithModifiedReplacement` proves replay now rejects a duplicate supersession that tries to mutate the replacement payload under the same replacement id. `TestHTTPMemoryCandidateSupersedeRejectsInvalidAuditRefsAndSecretShapedText` proves replacement source, supersession authority, reason, and evidence reject invalid refs or secret-shaped content before ledger append. `TestHTTPCreateMemoryCandidateRejectsInvalidAuditRefsAndSecretShapedText`, `TestHTTPApproveMemoryCandidateRejectsInvalidAuditRefsAndSecretShapedText`, and `TestHTTPRejectMemoryCandidateRejectsInvalidAuditRefsAndSecretShapedText` prove the shared Accumulation audit boundary is enforced across create, approve, reject, and supersede. `TestConcurrentMemorySupersedeWritesOnlyOneTerminalDecision` proves supersede participates in the same terminal review race fixture as approve/reject. `go test ./internal/kernel -run "TestHTTPCreateMemoryCandidate|TestHTTPMemoryCandidateApprove|TestHTTPApproveMemoryCandidate|TestHTTPMemoryCandidateReject|TestHTTPRejectMemoryCandidate|TestHTTPRejectedMemoryCandidate|TestHTTPApprovedMemoryCandidate|TestRejectMemoryCandidate|TestConcurrentMemoryReview|TestConcurrentMemorySupersede|TestHTTPMemoryCandidateSupersede|TestSupersedeMemoryCandidate|TestMemoryReplayRejects|TestHTTPSupersededMemoryCandidate|TestHTTPWork|TestHTTPCancelWork|TestHTTPCreateWork|TestWorkReplay|TestConcurrentWork" -count=1 -v` passed; `go test ./...` passed; `go test -race ./internal/kernel -count=1` passed; `go build` passed for both `cmd/genesisd` and `cmd/genesisctl`; `git diff --check` passed; the repository scan for numbered route or kernel version labels returned no matches.
- Evidence: Fix commits: `7235aae74`, `2dc9a34e1`, `750a9be2f`.

### KERNEL-WORK-REGISTRY-20260622 - P0 - Minimal WorkRegistry needs a durable submit and cancel loop

- Status: ready_for_acceptance.
- Conclusion: `TestHTTPWorkSubmitCancelReadAndSessionProjectionAfterRestart` first failed with 404 for `/work`, then passed after implementation. It proves `POST /work` creates an open work record with `source_ref`, `GET /work/{id}` reads it after restart, `POST /work/{id}/cancel` persists a canceled state with authority/reason/evidence, and `GET /sessions/{id}` projects the canceled work after another restart. `TestHTTPWorkSubmitIdempotencyKeyReturnsExistingWorkAfterRestart` proves submit retries with the same `session_id + idempotency_key` return the original work after restart, preserve the original title/source, and append only one `work.submitted` event. `TestHTTPCreateWorkRejectsInvalidIdempotencyKey` proves invalid retry keys fail before ledger append. `TestHTTPCancelWorkIsIdempotentWithoutOverwritingEvidence` proves duplicate cancel calls preserve the first cancel evidence and append only one `work.canceled` event. `TestConcurrentWorkCancelWritesOnlyOneTerminalDecision` proves same-process concurrent cancel callers observe one terminal cancel decision and only one cancel event is appended. `TestWorkReplayRejectsCompetingCancelEvidence` proves a corrupted or competing ledger with two different cancel evidence records fails closed during `Work` and `Session` replay instead of last-writer-wins overwrite. `TestHTTPCreateWorkRequiresSourceRef` proves submit requires source evidence. `TestHTTPCreateWorkRejectsInvalidAuditRefsAndSecretShapedText` and `TestHTTPCancelWorkRejectsInvalidAuditRefsAndSecretShapedText` prove work session id, title, source ref, cancel authority, cancel reason, and cancel evidence ref reject invalid audit shapes or secret-shaped content before ledger append. `go test ./internal/kernel -run "TestHTTPWork|TestHTTPCancelWork|TestHTTPCreateWork|TestWorkReplay|TestConcurrentWork" -count=1 -v` passed; `go test ./...` passed; `go test -race ./internal/kernel -count=1` passed after installing the local Windows gcc toolchain; `go build` passed for both `cmd/genesisd` and `cmd/genesisctl`; `git diff --check` passed; the repository scan for versioned route or kernel version labels returned no matches.
- Evidence: Fix commits: `71f4abc95`, `3e606b5a0`, `231363fd5`.

### KERNEL-MEMORY-REJECT-20260622 - P1 - Memory review needs a reject path

- Status: ready_for_acceptance.
- Conclusion: `TestHTTPMemoryCandidateRejectAndReadAfterRestart` first failed with 404 for `/memory/candidates/{id}/reject`, then passed after implementation. It proves a rejected candidate records `rejection_authority`, `rejection_reason`, and `rejection_evidence_ref`, appears under `status=rejected` after restart, disappears from `status=pending`, remains readable with rejection evidence, projects through `GET /sessions/{id}`, and is not recalled into a later turn. `TestHTTPRejectedMemoryCandidateCannotBeApproved` proves a rejected candidate cannot later be approved into active memory through the minimal review surface. `TestHTTPApprovedMemoryCandidateCannotBeRejected` proves approved memory cannot be overwritten by a rejection. `TestRejectMemoryCandidateIsIdempotentWithoutAppendingDuplicateEvent` proves duplicate reject calls do not append competing rejection evidence. `TestConcurrentMemoryReviewWritesOnlyOneTerminalDecision` first failed with two successful terminal review decisions, then passed after the kernel serialized memory review transitions. `TestHTTPRejectMemoryCandidateRejectsMissingEvidence` proves rejection evidence is required before candidate lookup. Existing memory approval and recall tests still pass. `go test ./...` passed; `go build` passed for both `cmd/genesisd` and `cmd/genesisctl`; `git diff --check` passed; the repository scan for versioned route or kernel version labels returned no matches.
- Evidence: Fix commits: `ac2d01571`, `72f1fbe2d`, `69229422a`.

### KERNEL-USAGE-SUMMARY-20260622 - P1 - Final answer must project provider usage summary

- Status: ready_for_acceptance.
- Conclusion: `TestHTTPFinalUsageSummarySurvivesSessionReplay` first failed because `final.usage` was absent, then passed after provider usage normalization and final-event persistence. The test proves an OpenAI-compatible `usage.prompt_tokens/completion_tokens/total_tokens` response becomes `usage.input_tokens/output_tokens/total_tokens` on `POST /turn` and survives restart through `GET /sessions/{id}`. `TestOpenAICompatibleProviderCompletesAgainstCompatibleServer` proves the provider adapter returns normalized usage on `ModelResponse`. `go test ./...` passed; `go build` passed for both `cmd/genesisd` and `cmd/genesisctl`; `git diff --check` passed; the repository scan for versioned route or kernel version labels returned no matches.
- Evidence: Fix commits: `5efb9c01f`, `7a5364171`.

### KERNEL-TOOL-BATCH-AUTH-20260622 - P1 - Mixed model tool-call batches must fail before any effect

- Status: ready_for_acceptance.
- Conclusion: `TestSubmitTurnRejectsMixedModelToolBatchBeforeAnyEffect` proves a provider batch containing allowed `shell.exec` plus unsupported `email.send` returns `ErrModelToolCallRejected`, creates no output file, and leaves no operation projection. `TestSubmitTurnRejectsUnknownModelToolArgumentFields` proves authority-shaped unknown argument fields such as `permission_mode` are rejected before any shell effect. `TestSubmitTurnRejectsUnsupportedModelToolCall` still proves unsupported single-call batches record `turn.submitted`, `model.tool_call`, and `turn.failed` without effects. `go test ./...` passed; `go build` passed for both `cmd/genesisd` and `cmd/genesisctl`; `git diff --check` passed; the repository scan for versioned route or kernel version labels returned no matches.
- Evidence: Fix commits: `5754c4297`, `166fd1116`.

### KERNEL-TOOL-LOOP-20260622 - P0 - Turn loop cannot execute model-requested tools

- Status: ready_for_acceptance.
- Conclusion: `TestSubmitTurnExecutesOpenAICompatibleToolCallBeforeFinal` first failed against the old behavior because the OpenAI-compatible request did not include a `shell.exec` tool descriptor and the provider returned HTTP 400. After the fix, the test proves the provider can request `shell.exec`, the kernel executes it through `ToolPolicy`, sends redacted operation evidence back as a tool message, receives the final answer, and replays `turn.submitted`, `model.tool_call`, `operation.running`, `operation.completed`, and `model.final` through `GET /turns/{id}/events` after restart. `TestSubmitTurnRejectsUnsupportedModelToolCall` proves unsupported provider tools fail closed with `tool_call_rejected` and no operation effect. `go test ./...` passed; `go build` passed for both `cmd/genesisd` and `cmd/genesisctl`; `git diff --check` passed; the repository scan for versioned route or phase labels returned no matches.
- Evidence: Fix commits: `209c002a8`, `d6d3ffb7e`.

### KERNEL-TURN-EVENTS-20260622 - P1 - Turn events need a direct observation surface

- Status: ready_for_acceptance.
- Conclusion: `TestHTTPTurnEventsAfterRestart` failed before implementation with HTTP 404 for `/turns/{id}/events`, then passed after implementation; `go test ./...` passed; `go build` passed for both `cmd/genesisd` and `cmd/genesisctl`; `git diff --check` passed; the repository scan for versioned route or phase labels returned no matches; live HTTP smoke submitted a turn, restarted `genesisd`, read `/turns/{id}/events`, and observed `turn.submitted` then `model.final`, with 401 for missing authorization and 404 for an unknown turn.
- Evidence: Fix commits: `0680f1a7a`, `eddad96d4`.

### recvndQ9cGNIqE - P1 - Stale running shell operations must not trap idempotent retries

- Status: ready_for_acceptance.
- Conclusion: the new `TestExecShellStaleRunningIdempotencyKeyFailsClosedAfterRestart` and `TestHTTPShellExecStaleRunningIdempotencyKeyReturnsFailedOperation` first failed against the previous behavior because stale idempotent retries returned `status=running`; after the fix both tests passed. `go test ./...` passed; `go build` passed for both `cmd/genesisd` and `cmd/genesisctl`; `git diff --check` passed. The fixed behavior replays a stale `operation.running`, appends a terminal `operation.failed` event with `blocked_reason=stale_running_operation`, returns the same `operation_id`, and does not execute the retry command.
- Evidence: Fix commits: `9742ad13`, `d274af7f1`.

### recvndHA93jSZH - P1 - Genesis provider credential needs an executable setup path

- Status: ready_for_acceptance.
- Conclusion: `go test ./...` passed; `go build` passed for both `cmd/genesisd` and `cmd/genesisctl`; `git diff --check` passed; the repository scan for versioned route or phase labels returned no matches; `TestSetupOpenAICompatibleProviderWritesConfigAndProtectedCredential`, `TestSetupOpenAICompatibleProviderDryRunWritesNothing`, `TestCorruptSetupCredentialBlocksProviderConfig`, `TestProviderSetupCommandDryRunDoesNotRequireAPIKey`, and `TestProviderSetupCommandWritesCredentialWithoutPrintingSecret` passed; a live local smoke wrote temp `models.json` plus a DPAPI credential record, verified `genesisctl provider-setup` output and generated files did not contain the test secret, started `genesisd` with the generated config and observed `/ready.status=ok`, then corrupted the credential and observed `/ready.status=blocked` with provider reason `provider_credential_missing`.
- Evidence: Fix commits: `1a3fed964`, `0ad989b71`.

### KERNEL-IDEMPOTENCY-20260622 - P0 - Duplicate tool idempotency keys must not execute effects twice

- Status: ready_for_acceptance.
- Conclusion: `go test ./...` passed; build passed; live fake-provider HTTP smoke returned the same `operation_id` for two `/tools/shell.exec` requests with the same `idempotency_key`, preserved file content as `first`, and projected `operation_count=1` plus `event_count=2`; `TestExecShellIdempotencyKeySurvivesRestartWithoutRepeatingEffect` proves restart-safe replay does not execute the second command; `TestExecShellBlockedOperationIsIdempotent` proves blocked operations are idempotent; `TestExecShellRejectsInvalidIdempotencyKey` proves invalid key shapes fail before ledger append; `TestHTTPShellExecIdempotencyKeyReturnsExistingOperation` proves the HTTP transport uses the same kernel behavior.
- Evidence: Fix commits: `d9b65933b`, `76971aef5`.

### recvnd2PDI1LuV - P0 - Minimal Go single-binary spike

- Status: ready_for_acceptance.
- Conclusion: `go test ./...` passed; build passed; `GENESIS_LIVE_PROVIDER=1 go test ./internal/kernel -run TestLiveOpenAICompatibleProviderThroughKernel -count=1 -v` passed using Genesis `~/.genesis/config/models.json` and local `secret://...` credential resolution; binary `/ready` smoke returned `provider=openai-compatible` and `status=ok`; repository version-label scan returned no matches.
- Evidence: Fix commits: `559e1c0c7`, `fd5bf9d8a`, `db9aeca13`, `22d5ca9f4`, `a9b34bda7`, `25e292b81`.

### recvndJWPu1RcN - P0 - Ingress security must not hard-reject ordinary user text

- Status: ready_for_acceptance.
- Conclusion: `go test ./...` passed; build passed; live fake-provider HTTP smoke returned 200 for risk-marker text with 2 `ingress_risks`, 403 `turn_blocked_by_ingress_security` for hidden control text, and 400 `invalid_request` for nested `role`; `TestSubmitTurnRecordsIngressRiskWithoutBlocking` proves prompt-injection samples are accepted as user data and recorded as risk metadata; `TestHTTPAcceptsRiskyUserDataAndRecordsMetadata` proves `System:` log headings and `tool_call_id` / `function_call` fragments do not block `/turn`; `TestHTTPRejectsNestedControlFieldBeforeAdmission` proves malformed nested control fields still return 400 before ledger append; `TestHTTPBlocksInvisibleIngressMarker` proves hidden control text still returns 403 before ledger append.
- Evidence: Fix commit: `330836d7b`.

### recvnd2PDIz0sA - P0 - Minimal `shell.exec` tool runtime and permission gate

- Status: ready_for_acceptance.
- Conclusion: `go test -count=1 ./...` passed; live smoke covered controlled workspace write/read, alias escape blocked, absolute path escape blocked, environment access blocked, and junction CWD blocked.
- Evidence: Fix commits: `924984712`, `6ae64ea5f`, `64aae83cb`, `ab04bf132`.

### recvnd2PDIKruI - P0 - Minimal accumulation loop

- Status: ready_for_acceptance.
- Conclusion: `go test -count=1 ./...` passed; build passed; live smoke covered candidate create/list/read/approve/restart/recall; `TestHTTPMemoryCandidateListAndReadAfterRestart` passed.
- Evidence: Fix commits: `730445409`, `1234f89d4`, `15c320ac0`.

### recvnd2PDIoXVt - P0 - Unified event stream and restart-safe ledger

- Status: ready_for_acceptance.
- Conclusion: `go test -count=1 ./...` passed; provider failure projection is `failed/provider_unavailable`; memory pending list/read is restart-safe; turn recall source points to the candidate `source_ref`.
- Evidence: Fix commits: `559e1c0c7`, `924984712`, `730445409`, `6ae64ea5f`, `8534adff8`, `15c320ac0`.

### recvndgCmpUUTp - P0 - Memory pending queue and source evidence

- Status: ready_for_acceptance.
- Conclusion: missing `source_ref` create returns 400; missing approval evidence returns 400; restart-safe `GET /memory/candidates?status=pending` returns only pending items; `GET /memory/candidates/{id}` exposes approval evidence; unknown status returns 400; missing read returns 404; recall source points to `source_ref`.
- Evidence: Fix commits: `1234f89d4`, `15c320ac0`.

### recvndhZ7RZDvd - P0 - Provider failure must not leave running turns

- Status: ready_for_acceptance.
- Conclusion: `TestHTTPReportsBlockedProvider` passed; live smoke with missing provider base URL returned `/ready=blocked`, `POST /turn=503`, and session projection status `failed` with error `provider_unavailable`.
- Evidence: Fix commit: `8534adff8`.

### recvndhZ7RcTsM - P0 - `shell.exec` default alias workspace escape

- Status: ready_for_acceptance.
- Conclusion: `go test -count=1 ./...` passed; live smoke showed workspace-internal controlled write/read completed, while alias escape, absolute path escape, env access, and junction CWD were blocked.
- Evidence: Fix commits: `6ae64ea5f`, `ab04bf132`.

### recvndkw7apwxx - P1 - Shell evidence secret redaction

- Status: ready_for_acceptance.
- Conclusion: `go test -count=1 ./...` passed; live smoke showed command/stdout entries containing fake API key, bearer token, and JSON `api_key` were replaced with `[REDACTED]` in response and session projection.
- Evidence: Fix commit: `64aae83cb`.

### recvndkw7abn2e - P1 - `shell.exec` default is not OS-level sandbox

- Status: ready_for_acceptance.
- Conclusion: README states default does not invoke an OS shell, expand env, or execute arbitrary interpreters; `go test -count=1 ./...` passed; live smoke blocked env access, alias escape, absolute escape, and junction CWD.
- Evidence: Fix commit: `ab04bf132`.

### recvndkw7almZD - P1 - Memory source refs and approval evidence

- Status: ready_for_acceptance.
- Conclusion: `go test -count=1 ./...` passed; missing `source_ref` returns 400; missing approval reason/evidence returns 400; approved candidate projection includes source and approval evidence; consumer recall source uses `source_ref`.
- Evidence: Fix commit: `1234f89d4`.

### recvndkw7afapL - P2 - Provider adapter must not assemble memory context

- Status: ready_for_acceptance.
- Conclusion: `go test ./...` passed; build passed; `TestKernelBuildsApprovedMemoryContextBeforeOpenAICompatibleProvider` proves approved memory context is assembled by the kernel/model context path before OpenAI-compatible provider transport.
- Evidence: Fix commits: `a93fc9d6f`, `db9aeca13`.

### recvndl0tmzxkL - P0 - Runtime token missing should block readiness

- Status: ready_for_acceptance.
- Conclusion: `go test -count=1 ./...` passed; live smoke with no runtime token returned `/ready.status=blocked` and `runtime_auth.reason=runtime_token_missing`; configured token returned `/ready.status=ok`.
- Evidence: Fix commit: `5948d7ec5`.

### recvndyUquaZ5z - P1 - Repo issue and retirement record sync

- Status: ready_for_acceptance.
- Conclusion: active issue ledger exists at `docs/operations/kernel-issues.md`; ready/retirement evidence exists at `docs/operations/kernel-retirement-log.md`; README links both records; `rg` can find current active issue ids and all current `ready_for_acceptance` issue ids under repo docs.
- Evidence: Fix commits: `fed9d405a`, `83ff63fbe`.

### recvndAOsH7nn4 - P0 - Ledger unavailable must block readiness

- Status: ready_for_acceptance.
- Conclusion: `go test ./...` passed; build passed; `TestReadyBlocksWhenLedgerUnwritable` and `TestHTTPLedgerUnavailableBlocksReadyAndTurn` prove an unwritable ledger makes `/ready.status=blocked` with `ledger.reason=ledger_unwritable`, and `POST /turn` returns 503 `ledger_unwritable` rather than 400 `invalid_request`.
- Evidence: Fix commit: `35c2111c0`.

### recvndDo1ECC5O - P1 - Corrupt ledger replay must block readiness

- Status: ready_for_acceptance.
- Conclusion: `go test ./...` passed; build passed; `TestHTTPCorruptLedgerBlocksReadyReplayAndAppend` proves a corrupt ledger store makes `/ready.status=blocked` with `ledger.reason=ledger_corrupt`, and `/turn`, `/sessions/{id}`, and `/memory/candidates` return 503 `ledger_corrupt` rather than `ledger_unwritable` or `invalid_request`.
- Evidence: Fix commit: `9ad48a7fd`.

### KERNEL-SESSION-STORE-PRODUCTION-CLOSURE-20260630 - P2 - File-backed SQLite session store production closure

- Status: ready_for_acceptance.
- Conclusion: SQLite is now a minimal index/read model over file-backed session event frames: startup reconciles orphan frames into the index, `/sessions` uses the SQLite session list instead of replaying full event bodies, and stale single-writer lock recovery follows the connector lock guard.
- Evidence: Fix commit: `8582d1d11`; tests: `go test ./internal/kernel -run "SQLiteLedger|ListSessions" -count=1`, `go test ./internal/kernel -count=1`, `go test ./... -count=1`, `go build ./...`, `git diff --check`.
