# Kernel Retirement Log

This file records Genesis Kernel issues that are ready for acceptance or retired. It is the repo-owned companion to `docs/operations/kernel-issues.md`.

## Retirement Rules

- `ready_for_acceptance` means the code and verification evidence are ready for user or operator acceptance, but the issue is not fully retired yet.
- `retired` means the user or operator accepted the evidence. A retired issue must be absent from `kernel-issues.md`.
- Every entry must include the issue id, title, fixing commits, verification evidence, residual risk, and retirement reason or retirement condition.
- Every `KERNEL-BOUNDARY-*` entry and every architecture-type `KERNEL-*` entry must retain either `Reference alignment` or `Rejected drift risk` when moved from the active ledger.
- Entries summarize evidence. They should cite governing requirements and designs instead of copying the full production contract, raw debug output, stream chunks, or ordinary info logs.
- If an entry is reopened, move it back to `kernel-issues.md` and mark this log entry as reopened with the reason.

## Ready For Acceptance

### KERNEL-OWNER-SESSION-PROJECTION-20260623 - P1 - Session projection delegates owner replay

- Status: ready_for_acceptance.
- Type: architecture issue.
- Fix commits: `8d969d722`, `694c8e31c`.
- Requirement: `docs/requirements/kernel-owner-structure-governance.md`.
- Design: `docs/design/kernel-owner-structure-governance.md`.
- Reference alignment: Aligned with Codex and Reasonix control-plane separation. Codex keeps tool/session behavior behind typed runtime surfaces and core events; Reasonix records a frontend/controller/agent/tool/provider separation where CLI/frontends do not own tool or provider internals. Genesis keeps `Session()` as a projection entry point instead of a cross-owner replay switch.
- Evidence: `Kernel.Session()` now validates the session id, loads events, delegates to `projectSessionProjection`, and redacts the final projection. The replay switch moved to `session_projection.go`, which composes turn, operation, job, work, memory, and raw event projection helpers outside the core kernel method. `TestArchitectureBoundaryKernelSessionDelegatesOwnerReplay` fails if owner replay markers return to `kernel.go`'s `Session()` body.
- Verification: `D:\software\Go\bin\go.exe test ./internal/kernel -run 'TestArchitectureBoundary(KernelSessionDelegatesOwnerReplay|OwnerDTOsLiveInNamedFiles|HTTPTransportDoesNotReplayOwnerFacts|CoreLoopHasNoProviderNativeWireTerms|KernelIssuesRequireReferenceAlignment)' -count=1`; `D:\software\Go\bin\go.exe test ./internal/kernel -count=1`; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `git diff --check`.
- Acceptance condition: reviewer confirms `kernel.go` no longer owns turn/tool/job/work/memory replay and future owner projection growth must extend owner projection helpers rather than `Kernel.Session()`.
- Residual risk: `session_projection.go` is still a same-package composer. A future package split is intentionally not part of this phase. HTTP transport and ToolRegistry authority remain active P2 issues.

### KERNEL-OWNER-DTO-FILES-20260623 - P1 - Public DTOs live in owner and projection files

- Status: ready_for_acceptance.
- Type: architecture issue.
- Fix commits: `8d969d722`, `694c8e31c`.
- Requirement: `docs/requirements/kernel-owner-structure-governance.md`.
- Design: `docs/design/kernel-owner-structure-governance.md`.
- Reference alignment: Aligned with Reasonix's package-level split between provider, tool, permission, config, and agent types, and with Codex's protocol/runtime separation. Genesis keeps one small kernel package for now, but file names now expose owner placement.
- Evidence: The global `internal/kernel/types.go` file was removed. Public DTOs now live in `config_types.go`, `turn_types.go`, `tool_types.go`, `work_types.go`, `memory_types.go`, `event_types.go`, `inspection_types.go`, `provider_accounting_types.go`, `context_compaction_types.go`, and `skill_catalog_types.go`. `TestArchitectureBoundaryOwnerDTOsLiveInNamedFiles` fails if these owner and projection declarations move back into a global DTO file or the wrong owner file.
- Verification: `D:\software\Go\bin\go.exe test ./internal/kernel -run 'TestArchitectureBoundary(KernelSessionDelegatesOwnerReplay|OwnerDTOsLiveInNamedFiles|HTTPTransportDoesNotReplayOwnerFacts|CoreLoopHasNoProviderNativeWireTerms|KernelIssuesRequireReferenceAlignment)' -count=1`; `D:\software\Go\bin\go.exe test ./internal/kernel -count=1`; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `git diff --check`.
- Acceptance condition: reviewer confirms public DTO ownership is visible at file level without changing runtime schema or adding compatibility aliases.
- Residual risk: file-level grouping is a guardrail, not a package-level owner system. If the kernel grows beyond this phase, a package split may need a separate requirement.

### KERNEL-OWNER-HTTP-TRANSPORT-20260623 - P2 - HTTP transport files stay thin delegates

- Status: ready_for_acceptance.
- Type: architecture issue.
- Fix commits: `8d969d722`, `a959c76de`.
- Requirement: `docs/requirements/kernel-owner-structure-governance.md`.
- Design: `docs/design/kernel-owner-structure-governance.md`.
- Reference alignment: Aligned with Reasonix's frontend/controller separation and Codex's protocol/event surfaces. Genesis HTTP remains a shell/adapter: it authenticates, decodes, delegates to owner APIs, maps owner errors, and encodes projections without owning replay or state transitions.
- Evidence: HTTP transport handlers are split into `http_turn.go`, `http_tools.go`, `http_work.go`, `http_memory.go`, and `http_inspection.go`; `http.go` keeps routing and common transport helpers. `TestArchitectureBoundaryHTTPHandlersLiveInSurfaceFiles` checks handler file ownership, and `TestArchitectureBoundaryHTTPTransportDoesNotReplayOwnerFacts` blocks direct owner append/replay helpers in HTTP files.
- Verification: `D:\software\Go\bin\go.exe test ./internal/kernel -run 'TestHTTP|TestArchitectureBoundaryHTTP' -count=1`; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `git diff --check`.
- Acceptance condition: reviewer confirms HTTP files remain transport surfaces, not hidden turn, tool, work, memory, session, audit, or timeline owners.
- Residual risk: Handler grouping is file-level governance. A future router abstraction may be useful if route count grows, but it is not needed for the current kernel slice.

### KERNEL-OWNER-TOOL-CONTEXT-20260623 - P2 - Tool registry binding uses narrow invocation context

- Status: ready_for_acceptance.
- Type: architecture issue.
- Fix commits: `8d969d722`, `5d244a42e`.
- Requirement: `docs/requirements/kernel-owner-structure-governance.md`.
- Design: `docs/design/kernel-owner-structure-governance.md`.
- Reference alignment: Aligned with Codex's `CoreToolRuntime` over typed `ToolInvocation` and Reasonix's `Tool` interface plus per-run `Registry`. Genesis keeps a registry-owned execution binding, but the binding no longer declares the whole kernel object as the tool authority.
- Evidence: `registeredTool.Prepare` now accepts `toolInvocationContext` instead of `*Kernel`. Default tool entries call only context methods for shell execution, job status, and job cancel preparation. `TestArchitectureBoundaryToolRegistryDoesNotBindWholeKernel` fails if `Prepare func(*Kernel, ...)` or `Prepare: (*Kernel)` returns to `tool_registry.go`.
- Verification: `D:\software\Go\bin\go.exe test ./internal/kernel -run 'TestArchitectureBoundaryToolRegistry|TestSubmitTurn.*Job|TestSubmitTurn.*Shell|TestSubmitTurn.*Tool|TestToolCapability|TestHTTPCapabilities' -count=1`; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `git diff --check`.
- Acceptance condition: reviewer confirms tool registrations no longer declare broad `*Kernel` authority and future tools must extend a narrow invocation context or owner-specific executor interface.
- Residual risk: `toolInvocationContext` is still an internal same-package interface for current built-in tools. A future external plugin/runtime boundary needs a separate requirement if tools become installable outside the kernel package.

### KERNEL-FOREGROUND-TIMEOUT-OUTCOME-20260623 - P2 - Foreground timeout preserves terminal outcome evidence

- Status: ready_for_acceptance.
- Type: runtime/tool issue.
- Fix commits: `dfda23540`.
- Requirement: `docs/requirements/kernel-shell-and-job-control.md`.
- Design: `docs/design/kernel-shell-and-job-control.md`.
- Reference alignment: Aligned with local Codex and Reasonix terminal-equivalent tool behavior. Codex preserves timeout output and execution metadata for model inspection; Reasonix returns timeout as a tool execution outcome after collecting bounded output. Genesis follows the same split: timeout is an executed command result unless process infrastructure itself fails.
- Evidence: Foreground shell timeout now records `operation.failed` with `timed_out=true`, `timeout_reason=foreground_timeout`, positive `elapsed_ms`, exit code evidence, and available bounded stdout/stderr. The model-visible `shell_exec` tool result carries the same timeout metadata and captured output, while `infrastructure_reason` stays empty for ordinary runtime timeout. Timeout remains separate from malformed request feedback and managed-job routing.
- Verification: `TestSubmitTurnForegroundShellTimeoutRecordsTerminalOutcome`; focused timeout/direct-shell/job routing suite; `D:\software\Go\bin\go.exe test ./internal/kernel -run TestArchitectureBoundary -count=1`; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `git diff --check`.
- Acceptance condition: reviewer confirms foreground shell timeout is model-visible command evidence, not `tool_request_invalid`, not `tool_infrastructure_failed`, and not a managed-job receipt.
- Residual risk: provider-stream interruption and foreground shell attach-or-kill behavior remain active under `KERNEL-JOB-CONTROL-INTERRUPT-20260623`; stronger sandbox and approval policy remain active under `KERNEL-SANDBOX-APPROVAL-NEXT-20260623`.

### KERNEL-DIRECT-SHELL-MANAGED-JOB-PARITY-20260623 - P2 - Direct shell transport shares managed-job routing

- Status: ready_for_acceptance.
- Type: runtime/tool transport issue.
- Fix commits: `1070e2ef6`, `b24f9556a`.
- Requirement: `docs/requirements/kernel-shell-and-job-control.md`.
- Design: `docs/design/kernel-shell-and-job-control.md`.
- Reference alignment: Aligned with Codex and Reasonix's shared-owner pattern: transports and shells submit into the core/controller path rather than owning independent tool lifecycle semantics. Genesis keeps direct HTTP as a transport projection over Tool Runtime and managed-job ledger facts.
- Evidence: Direct `POST /tools/shell_exec` now distinguishes omitted `timeout_sec` from explicit invalid values, rejects explicit non-positive timeout before effects, delegates foreground and managed routing to ToolGateway, returns foreground `OperationProjection` for foreground-valid requests, returns HTTP 202 with a redacted `JobProjection` receipt for an admitted host-sandbox long job, and returns the existing operation or job projection on idempotent retry without executing a second effect. Controlled-workspace/default long shell requests are blocked until a controlled managed executor exists. Direct HTTP long jobs do not forge provider-loop `tool.call` or `tool.result` events.
- Verification: `TestHTTPShellExecLongTimeoutReturnsManagedJobReceipt`; `TestHTTPShellExecRejectsExplicitZeroTimeout`; `TestHTTPShellExecLongTimeoutDoesNotBypassDefaultSandbox`; `TestHTTPShellExecManagedJobRetryRedactsTerminalOutput`; `TestHTTPShellExecIdempotencyKeyDoesNotCrossFromOperationToJob`; `TestHTTPShellExecIdempotencyKeyDoesNotCrossFromJobToOperation`; focused HTTP shell and job-control kernel tests; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `D:\software\Go\bin\go.exe test -race ./internal/kernel -count=1`; `git diff --check`.
- Acceptance condition: reviewer confirms direct shell transport no longer has a parallel long-running shell lifecycle and that direct HTTP job receipts are job projections, not ordinary operation projections or fake provider tool results.
- Residual risk: provider-stream and foreground interrupt/attach behavior remain active under `KERNEL-JOB-CONTROL-INTERRUPT-20260623`; stronger sandbox and approval policy remain active under `KERNEL-SANDBOX-APPROVAL-NEXT-20260623`.

### KERNEL-OBSERVATION-DELIVERY-20260623 - P1 - Kernel observation queue and delivery checkpoints

- Status: ready_for_acceptance.
- Type: runtime/model-gateway issue.
- Fix commits: `531f8d008`.
- Requirement: `docs/requirements/kernel-shell-and-job-control.md`.
- Design: `docs/design/kernel-shell-and-job-control.md`.
- Reference alignment: Aligned with Codex's core/session ownership of tool-loop and compaction state: external shells submit typed facts, while the core decides which observations enter provider context and when delivery is recorded. This rejects UI, daemon, or provider-adapter ownership of model-visible observation delivery.
- Evidence: Terminal managed-job facts now become Kernel Observation Queue sources. `ProviderContextProjection` injects undelivered terminal job observations as `kernel_observation_context` before a provider step, `SubmitTurn` records `kernel.observation.delivered` only after the provider call returns successfully, and delivered ids suppress repeat projection after ledger replay. Provider failures append turn failure evidence without marking the observation delivered.
- Verification: `TestSubmitTurnDeliversCompletedJobObservationToNextProviderStep`; `TestProviderFailureDoesNotMarkJobObservationDelivered`; `TestDeliveredJobObservationIsNotProjectedAgainAfterRestart`; focused observation/managed-job suite; `D:\software\Go\bin\go.exe test ./internal/kernel -count=1`; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `D:\software\Go\bin\go.exe test -race ./internal/kernel -count=1`; `git diff --check`.
- Acceptance condition: reviewer confirms terminal job observations are visible to the model only through kernel-owned provider context, are not marked delivered on provider failure, and are not replayed after restart once delivered.
- Residual risk: this is the delivery contract for terminal job observations. Progress snapshots, auto-resume policy, and interrupt/attach behavior remain separate issues. Minimal `job_status` and `job_cancel` are covered by `KERNEL-JOB-CONTROL-MINIMAL-20260623`.

### KERNEL-JOB-CONTROL-MINIMAL-20260623 - P2 - Minimal generic job status and cancel tools

- Status: ready_for_acceptance.
- Type: runtime/tool issue.
- Fix commits: `ce72dfa44`, `b24f9556a`.
- Requirement: `docs/requirements/kernel-shell-and-job-control.md`.
- Design: `docs/design/kernel-shell-and-job-control.md`.
- Reference alignment: Aligned with Codex-style process control boundaries: the model receives a typed tool result for a kernel-owned handle, while process mechanics stay behind the runtime. Genesis intentionally exposes `job_status` and `job_cancel`, not process ids, signals, or a `job_terminate` tool.
- Evidence: The model-visible manifest now includes `shell_exec`, `job_status`, and `job_cancel`. `job_status` replays current job state from the session ledger without creating operations and redacts terminal output before returning model-visible JSON. `job_cancel` records semantic `job.cancel_requested` for a non-terminal job and does not forge terminal cancellation before executor confirmation. Terminal jobs return the current terminal state without writing competing facts. Unknown job ids and model-supplied process/control-plane fields return structured repair feedback.
- Verification: `TestSubmitTurnProjectsGenericJobControlToolManifest`; `TestSubmitTurnJobStatusReturnsCompletedJobAfterRestartWithoutOperation`; `TestSubmitTurnJobStatusRedactsTerminalOutput`; `TestSubmitTurnJobStatusReturnsRepairFeedbackForUnknownJob`; `TestSubmitTurnRejectsJobControlToolControlPlaneFields`; `TestSubmitTurnJobCancelTerminalJobReturnsCurrentStateWithoutCompetingTerminalEvent`; `TestSubmitTurnJobCancelLedgerOnlyRunningJobRecordsRequestWithoutForgingTerminalFact`; `TestSubmitTurnJobCancelReachesLiveManagedExecutor`; focused job-control suite; `D:\software\Go\bin\go.exe test ./internal/kernel -count=1`; `D:\software\Go\bin\go.exe test ./...`; `D:\software\Go\bin\go.exe build ./...`; `git diff --check`.
- Acceptance condition: reviewer confirms job control is a generic kernel primitive, model-visible schema stays free of process mechanics, and terminal job cancellation does not rewrite or duplicate ledger truth.
- Residual risk: job control now reaches the minimal managed shell executor, but foreground attach/kill behavior, progress snapshots, provider-stream interruption, and stronger sandbox/approval integration remain active issues.

### KERNEL-SHELL-TIMEOUT-CAP-20260623 - P1 - Foreground shell timeout policy and cap

- Status: ready_for_acceptance.
- Type: runtime/tool issue.
- Fix commits: `abb6b5d45`.
- Requirement: `docs/requirements/kernel-shell-and-job-control.md`.
- Design: `docs/design/kernel-shell-and-job-control.md`.
- Reference alignment: Aligned with Codex-style tool-loop boundaries where short tools close with a tool result, while long-running work moves behind a managed process/job abstraction. Reasonix also keeps frontend shells behind a controller rather than letting an adapter own lifecycle policy.
- Evidence: `shell_exec` now exposes `timeout_sec` in the model-visible tool schema. Omitted timeout records 30 seconds; `timeout_sec=1` and `timeout_sec=180` run as foreground shell attempts; invalid zero, negative, string, and fractional values return repairable `tool_request_invalid` feedback and create no operation. Requests above the foreground cap route to managed-job admission instead of being treated as validation errors; the current local managed executor admits only host-sandbox jobs and blocks controlled-workspace/default long requests.
- Verification: `TestSubmitTurnAcceptsForegroundShellTimeoutSeconds`; `TestSubmitTurnDefaultsShellTimeoutToThirtySeconds`; `TestSubmitTurnReturnsRepairFeedbackForInvalidShellTimeoutSeconds`; `TestSubmitTurnRoutesLongShellTimeoutToManagedJobReceipt`; `TestSubmitTurnProjectsRegisteredToolManifestWithoutSkillCatalogContext`; focused timeout suite; `D:\software\Go\bin\go.exe test ./internal/kernel -run TestArchitectureBoundary -count=1`; forbidden marker scan; `git diff --check`; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `D:\software\Go\bin\go.exe test -race ./internal/kernel -count=1`.
- Acceptance condition: reviewer confirms `timeout_sec > 180` is a valid long-task intent, not an error, and ordinary foreground shell execution is capped by the approved 1 through 180 second contract.
- Residual risk: direct `/tools/shell_exec` managed-job routing is covered by `KERNEL-DIRECT-SHELL-MANAGED-JOB-PARITY-20260623`; foreground timeout outcome metadata is covered by `KERNEL-FOREGROUND-TIMEOUT-OUTCOME-20260623`. Provider-stream and foreground interrupt/attach behavior remain active under `KERNEL-JOB-CONTROL-INTERRUPT-20260623`.

### KERNEL-MANAGED-JOB-FOUNDATION-20260623 - P1 - Minimal managed job event model

- Status: ready_for_acceptance.
- Type: runtime/tool issue.
- Fix commits: `abb6b5d45`.
- Requirement: `docs/requirements/kernel-shell-and-job-control.md`.
- Design: `docs/design/kernel-shell-and-job-control.md`.
- Reference alignment: Aligned with Codex's separation between tool-call closure and managed process lifecycle. The intentional difference is scope: Genesis starts with a generic job primitive and does not copy a coding-agent-specific task runner.
- Evidence: A model `shell_exec` request with `timeout_sec > 180` now records `tool.call`, `job.started`, immediate receipt-style `tool.result`, and `model.final` without pretending final command output is available. The `job.started` event id is the job handle. The provider receives a `managed_job_started` tool result, allowing the tool loop to close while terminal job facts are written later by the managed executor. Session replay preserves the receipt and lifecycle facts.
- Verification: `TestSubmitTurnRoutesLongShellTimeoutToManagedJobReceipt`; focused managed-job suite; `D:\software\Go\bin\go.exe test ./internal/kernel -run TestArchitectureBoundary -count=1`; forbidden marker scan; `git diff --check`; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `D:\software\Go\bin\go.exe test -race ./internal/kernel -count=1`.
- Acceptance condition: reviewer confirms the first managed-job event protocol is sufficient for provider-loop closure and restart-safe evidence while leaving progress snapshots and interrupt/attach behavior to later phases.
- Residual risk: progress snapshots and interrupt/attach behavior remain active issues. Minimal `job_status` and `job_cancel` are covered by `KERNEL-JOB-CONTROL-MINIMAL-20260623`.

### KERNEL-USER-SPACE-BOUNDARY-20260623 - P1 - Kernel, user-space, and agent-framework boundary document

- Status: ready_for_acceptance.
- Type: architecture issue.
- Fix commits: `b61be7b35`, `46c32f0ed`.
- Reference alignment: Codex and Reasonix are strong agent products with kernel-like runtimes. Their useful reference is the separation of core protocol, tool manifests, sandboxing, event truth, projections, and shells. Genesis applies that runtime split as a shared platform contract for multiple user-space applications, not as a copied coding-agent product.
- Evidence: `docs/kernel-contract.md` now contains `Agent Kernel vs Agent Framework` and `System Boundary / Box Model`. The contract defines the LLM as the operator, the kernel as the authority execution and fact boundary, tools as governed reality touchpoints, skills as user-space instruction packages, applications and shells as user-space compositions, and the event log as the system fact layer. `features/kernel/user_space_boundary.feature` records BDD acceptance examples for calculator skills, Feishu daemons, shell-owned context, app-owned memory, and framework attempts to forge kernel facts.
- Verification: The document can answer that a calculator skill is not kernel, a Feishu daemon is not kernel, WebUI cannot assemble provider context, applications cannot write memory truth or tool results directly, and new domain-named capabilities must map to generic kernel primitives before entering the kernel ledger. `git diff --check`; `go test ./internal/kernel -run TestArchitectureBoundary -count=1`; `go test ./... -count=1`; `go build ./...`.
- Acceptance condition: reviewer confirms future Feishu, email, calendar, calculator, OCR, document, app, skill, or agent-framework issues can cite this section to decide whether the requested behavior belongs in kernel or user space.
- Residual risk: this is a document and BDD acceptance boundary, not a full semantic code guard. Existing reference-alignment tests enforce issue structure; deeper automated detection of app-specific kernel drift can be added later only if it checks positive owner behavior instead of retired wording.

### KERNEL-PRESSURE-ACCEPTANCE-20260623 - P1 - Minimal kernel loop needs deterministic pressure verification

- Status: ready_for_acceptance.
- Type: architecture issue.
- Fix commits: `d7a12ee7a`.
- Reference alignment: Codex relies on core/session/tool-loop tests and recovery checks rather than treating shell or app surfaces as the source of truth. Reasonix keeps frontend/controller flows behind reproducible runtime checks. Genesis now has the same class of deterministic core pressure gate without widening the kernel into product-specific adapters.
- Evidence: `TestKernelPressureLongRunningClosedLoop` runs a 12-turn session through successful `shell_exec` calls, repairable invalid tool requests, permission-denied tool results, terminal-equivalent failed command results, one provider failure, automatic context compaction, restart-safe session projection, restart-safe provider-context projection, UI timeline compaction notices without summary leakage, and idempotent turn replay without calling the provider again. The test asserts the ledger reconstructs completed and failed turns, completed/failed/blocked operations, tool call/result events, provider accounting events, compaction completion, and turn failure evidence after restart.
- Verification: `D:\software\Go\bin\go.exe test ./internal/kernel -run TestKernelPressureLongRunningClosedLoop -count=1 -v`; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `CGO_ENABLED=1 D:\software\Go\bin\go.exe test -race ./internal/kernel -count=1`; `git diff --check`.
- Acceptance condition: reviewer confirms the pressure gate exercises only kernel primitives and provides enough sustained evidence for the current minimal closed loop without adding Feishu, email, calendar, WebUI, daemon, or other application owners.
- Residual risk: this is deterministic pressure coverage with a fake provider, not live-provider load testing or user-perceived latency measurement. Live provider stress, performance targets, and long-duration soak tests remain separate operator/product acceptance work.

### KERNEL-CONTEXT-COMPACTION-REFINE-20260622 - P1 - Context compaction needs production-grade selection and retry evidence

- Status: ready_for_acceptance.
- Type: architecture issue.
- Fix commits: `7641953d0`.
- Reference alignment: Codex keeps compaction execution in core/session logic while app shells only trigger a typed core operation; Reasonix records cache/context behavior and compaction lifecycle events outside frontend ownership. Genesis now follows the same control-plane split: Model Gateway records provider usage and tool-round accounting, while the kernel compaction runner owns selection, retry deferral, summary source construction, and compaction evidence.
- Evidence: `ContextPolicy.RetryBackoffTurns` now normalizes to a bounded default and failed summarizer attempts record `context.compaction.failed`; the next eligible trigger can record `context.compaction.deferred` with previous failure and remaining backoff evidence before a later retry. `model.context.accounted` now records provider-visible tool round, call, and result counts in addition to provider usage and processed input tokens. The compaction source is built from completed conversation turns and preserves model-visible tool call/result pairs before the assistant final answer without exposing kernel event or operation ids. Completed compaction records triggering `source_usage` and cache-stability metrics for sampled compacted turns. `docs/kernel-contract.md`, `docs/minimal-closed-loop.md`, `docs/field-reference.md`, and `features/kernel/context_compaction.feature` now describe those behaviors.
- Verification: `TestAutoCompactionBacksOffAfterSummarizerFailure`; `TestModelGatewayAccountsToolRoundBoundaries`; `TestAutoCompactionRecordsUsageEconomicsAndCacheStability`; `TestCompactionSourcePreservesCompletedToolCallResultPairs`; focused compaction/accounting suite; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `CGO_ENABLED=1 D:\software\Go\bin\go.exe test -race ./internal/kernel -count=1`; `git diff --check`.
- Acceptance condition: reviewer confirms compaction remains kernel-owned, provider usage remains provider-backed, tool call/result pairs are preserved at complete-turn boundaries, and the new cache/backoff evidence is sufficient for inspection without leaking internal summaries into user timelines.
- Residual risk: this still does not judge summary quality or optimize long-session economics against a live provider. Those remain operator/product acceptance tasks and should become deterministic tests only when a machine-checkable kernel behavior is identified.

### KERNEL-PROVIDER-CONTEXT-SESSION-HISTORY-20260622 - P1 - Provider context must define and preserve same-session conversation history

- Status: ready_for_acceptance.
- Type: architecture issue.
- Fix commits: `ad05c9950`.
- Reference alignment: Codex keeps conversation state inside the core thread/session context rather than asking shells to resend history, and Reasonix keeps frontend inputs behind a controller-owned context boundary. Genesis now projects same-session completed conversation history from the ledger through `ProviderContextProjection` instead of letting WebUI, provider commands, or external daemons synthesize model-visible history.
- Evidence: `ProviderContextProjection` now prepends a `conversation_history_context` model input fragment built from completed prior turns in the same session. `turn.submitted` records `model_input_kinds` including the history kind when history exists. The projection still omits event ids, operation ids, permission mode, audit fields, raw stdout, and raw stderr. `docs/kernel-contract.md` now states that session history is Model Gateway-owned and shells must not build their own model-visible history.
- Verification: `TestSubmitTurnProviderContextIncludesSameSessionHistory`; `D:\software\Go\bin\go.exe test ./internal/kernel -run "TestSubmitTurnProviderContextIncludesSameSessionHistory|TestResolveProviderConfigFromGenesisRejectsSecretCommandEnvironment|TestArchitectureBoundaryKernelIssuesRequireReferenceAlignment" -count=1`; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `CGO_ENABLED=1 D:\software\Go\bin\go.exe test -race ./internal/kernel -count=1`; `git diff --check`; route version scan returned no matches.
- Acceptance condition: reviewer confirms `session_id` is a model-context container owned by the kernel, and no shell/adapter/daemon needs to reconstruct same-session conversation history for the provider.
- Residual risk: the first history projection records completed user/assistant turns as bounded text context. Richer prior tool visibility, compression, and window policy remain future Model Gateway work and must extend the same projection owner.

### KERNEL-PROVIDER-COMMAND-ENV-CREDENTIAL-BOUNDARY-20260622 - P1 - provider_command env must not bypass credential plane

- Status: ready_for_acceptance.
- Type: architecture issue.
- Fix commits: `ad05c9950`.
- Reference alignment: Codex separates credentials and process environment policy from model-visible tool/context state, and Reasonix treats provider configuration as typed runtime config rather than an unbounded secret channel. Genesis now allows provider-command env only for non-sensitive adapter configuration while keeping provider credentials in the credential plane or in the external command's own identity environment.
- Evidence: `validateProviderCommandEnv` rejects secret-shaped env names and values before direct provider-command readiness or Genesis `models.json` resolution can pass them to a provider process. `ProviderConfigReason` returns `provider_command_env_secret_rejected`; direct daemon provider selection reports the same structured readiness blocker. README and `docs/kernel-contract.md` now document that `-provider-command-env` is for non-sensitive profile/route-style settings only.
- Verification: `TestResolveProviderConfigFromGenesisRejectsSecretCommandEnvironment`; `TestBuildProviderBlocksSecretShapedCommandEnvironment`; `TestBuildProviderCanPassExplicitCommandEnvironment`; `D:\software\Go\bin\go.exe test ./cmd/genesisd -run "TestBuildProviderBlocksSecretShapedCommandEnvironment|TestBuildProviderCanPassExplicitCommandEnvironment" -count=1`; focused kernel tests; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `CGO_ENABLED=1 D:\software\Go\bin\go.exe test -race ./internal/kernel -count=1`; `git diff --check`.
- Acceptance condition: reviewer confirms provider-command env is not a credential transport and that real provider API keys still use `secret://...` credential store or the external adapter's own secure runtime environment.
- Residual risk: this is a conservative syntactic guard for Genesis-owned config and daemon flags. External provider commands can still read their own host environment; that remains outside the kernel and must be governed by the adapter/operator.

### KERNEL-ARCHITECTURE-REFERENCE-GUARD-20260622 - P1 - reference-alignment governance guard should not be removed

- Status: ready_for_acceptance.
- Type: architecture issue.
- Fix commits: `ad05c9950`.
- Reference alignment: Codex and Reasonix are external control-plane reference implementations for provider context, tool boundaries, event recovery, and shell/application separation. Genesis now keeps a lightweight executable guard requiring active kernel issues and architecture retirements to retain reference-alignment or explicit drift-risk evidence, without making the test a prose-quality judge.
- Evidence: `TestArchitectureBoundaryKernelIssuesRequireReferenceAlignment` was restored as a structure guard. It scans active `docs/operations/kernel-issues.md` `KERNEL-*` records and architecture-type or `KERNEL-BOUNDARY-*` retirement entries for `Reference alignment` or `Rejected drift risk`. `docs/operations/kernel-retirement-log.md` rules now match that scope instead of only mentioning `KERNEL-BOUNDARY-*`.
- Verification: `TestArchitectureBoundaryKernelIssuesRequireReferenceAlignment`; focused kernel architecture tests; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `CGO_ENABLED=1 D:\software\Go\bin\go.exe test -race ./internal/kernel -count=1`; `git diff --check`.
- Acceptance condition: reviewer confirms architecture governance remains executable while avoiding a permanent test that locks historical issue wording or demands reference text for non-architecture maintenance entries.
- Residual risk: the guard checks structure, not the quality of the reference comparison. Human review still has to reject superficial or incorrect comparisons.

### KERNEL-PROVIDER-GATEWAY-EVENT-PROJECTION-20260622 - P1 - Provider gateway should be driven by provider-visible event projection

- Status: ready_for_acceptance.
- Type: architecture issue.
- Fix commits: `0eb426a42`, `0721b4116`.
- Reference alignment: Codex separates provider/model-visible context from host event identity, and Reasonix separates controller facts from provider and frontend projections. Genesis now rebuilds provider context from the ledger while omitting kernel-owned event, operation, permission, and audit identity from the provider-visible request.
- Evidence: `ProviderContextProjection` and `Kernel.ProviderContextProjection` derive provider-visible inputs, tool manifest, and tool rounds from stored events before each provider call. The turn loop sends that projection through `ModelRequest` to providers; `modelToolRoundsFromStoredEvents` no longer exposes `tool_call_event_id`; `provider_command` and OpenAI-compatible adapters consume model-visible tool call ids plus result content, not raw ledger or audit identity. Review fix `0721b4116` keeps shell ledger truth separate from redacted projections so provider context, audit replay, event inspection, and session projection cannot accidentally share one over-rich object. `docs/kernel-contract.md` now defines provider context as a projection boundary rather than a raw owner struct.
- Verification: `TestObservabilityProjectionsSeparateRawAuditAndProviderContext`; `TestSessionProjectionRedactsTopLevelReadModels`; `TestEvidenceRedactionCoversBareProviderKeysAndJWT`; `TestExecShellRedactsSecretEvidenceInReturnedProjectionButPreservesLedgerTruth`; `TestSubmitTurnUsesToolCallEventIDWhenProviderIDMissing`; `TestSubmitTurnReturnsRepairFeedbackForInvalidShellArguments`; `D:\software\Go\bin\go.exe test ./internal/kernel -count=1`; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `CGO_ENABLED=1 D:\software\Go\bin\go.exe test -race ./internal/kernel -count=1`; `git diff --check`.
- Acceptance condition: reviewer confirms provider adapters are driven by provider-visible context only and future provider work extends this projection instead of reading raw ledger, session projections, audit replay, or UI timeline objects.
- Residual risk: future streaming, reasoning-delta, compression, and richer memory-context events need explicit projection extension. The current implementation covers the active turn, tool call, tool result, and final-response event types.

### KERNEL-EVENT-OBSERVABILITY-POLICY-20260622 - P1 - Separate UI timeline, raw event inspection, audit log, and provider context projections

- Status: ready_for_acceptance.
- Type: architecture issue.
- Fix commits: `0eb426a42`, `0721b4116`.
- Reference alignment: Codex and Reasonix keep transcript/timeline, protocol or raw event inspection, audit evidence, and provider context separate. Genesis now follows the same boundary: ordinary shells use `/sessions/{id}/timeline`, authorized debugging can use `/turns/{id}/events`, replay/export can use `/turns/{id}/audit`, and providers receive only `ProviderContextProjection`.
- Evidence: `AuditReplayResponse`, `Kernel.AuditReplay`, and `GET /turns/{id}/audit` provide replay facts, operation statuses, provider-context input kinds, final usage, failure codes, and truncation metadata. `TurnEvents` and idempotency replay now return inspection events with redacted payload text. `inspectionEventData`, `toInspectionEvent`, and `redactSessionProjection` keep raw event envelopes and session top-level read models inspectable without exposing credential-shaped text. `ProviderContextProjection` omits audit, permission, raw operation, and kernel event identity while preserving model-visible tool result content. `shell_exec` now appends raw observed command/stdout/stderr to the local ledger, then returns and projects redacted evidence; redaction is a projection policy rather than a ledger mutation.
- Verification: `TestObservabilityProjectionsSeparateRawAuditAndProviderContext`; `TestSessionProjectionRedactsTopLevelReadModels`; `TestEvidenceRedactionCoversBareProviderKeysAndJWT`; `TestExecShellRedactsSecretEvidenceInReturnedProjectionButPreservesLedgerTruth`; `D:\software\Go\bin\go.exe test ./internal/kernel -count=1`; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `CGO_ENABLED=1 D:\software\Go\bin\go.exe test -race ./internal/kernel -count=1`; `git diff --check`.
- Acceptance condition: reviewer confirms UI shells can use timeline for chat cards, raw event inspection for authorized debugging, audit replay for export/replay facts, and provider context for model continuation without one over-rich object serving every audience.
- Residual risk: the local append-only ledger intentionally preserves raw shell operation evidence for replay and audit truth. Future raw ledger export or remote sync must define a separate privileged surface rather than reusing session, event inspection, audit, timeline, or provider-context projections.

### KERNEL-LIVE-LLM-FIRST-RUN-ACCEPTANCE-20260622 - P0 - Real LLM must have a user-executable first-run acceptance path

- Status: ready_for_acceptance.
- Type: user feedback.
- Fix commits: `fedbc58b3`.
- Reference alignment: Codex and Reasonix keep first-run and live-provider smoke paths executable by operators instead of hiding them only in tests. Genesis now has the same operator-facing acceptance surface while provider credentials and provider-specific account flows remain outside the kernel turn loop.
- Evidence: `scripts/first_run_live_llm_acceptance.ps1` builds `genesisctl.exe` and `genesisd.exe`, writes Genesis provider config through `genesisctl provider-setup`, stores the key behind a `secret://...` ref, starts `genesisd` with `-provider genesis-config`, checks `/ready`, submits a real `/turn`, inspects `/sessions/{id}/timeline`, `/turns/{id}/events`, and `/turns/{id}/context`, restarts the server against the same ledger, replays those projections, and checks missing-credential readiness and turn failures. `docs/operations/live-llm-first-run-acceptance.md` documents the scripted and manual acceptance paths, and README links to the runbook from provider setup.
- Verification: `powershell -NoProfile -ExecutionPolicy Bypass -File scripts\first_run_live_llm_acceptance.ps1 -Help`; `powershell -NoProfile -ExecutionPolicy Bypass -Command '[scriptblock]::Create((Get-Content -Raw scripts\first_run_live_llm_acceptance.ps1)) | Out-Null; "ok"'`; local OpenAI-compatible stub run of `scripts\first_run_live_llm_acceptance.ps1` with a space-containing temp `-WorkRoot` returned `ok=true`, non-fake final text, timeline/events/context counts, restart replay counts, and `provider_credential_missing` / `provider_unavailable` failure probe; `D:\software\Go\bin\go.exe test ./... -count=1`; `D:\software\Go\bin\go.exe build ./...`; `CGO_ENABLED=1 D:\software\Go\bin\go.exe test -race ./internal/kernel -count=1`; `git diff --check`.
- Acceptance condition: operator runs the script with a real provider base URL, model id, and provider API key environment variable and confirms the printed JSON summary reports `ok=true`, non-fake provider output, projection counts, restart replay counts, and `provider_credential_missing` / `provider_unavailable` for the failure probe without exposing the raw API key.
- Residual risk: repository verification can parse and document the script without a live provider key. The final live-provider acceptance remains an operator-run check because it depends on private credentials and the selected provider endpoint.

### KERNEL-UI-TIMELINE-PROJECTION-20260622 - P1 - WebUI needs a dedicated timeline projection instead of raw events

- Status: ready_for_acceptance.
- Fix commits: `7df00cf45`.
- Reference alignment: Reasonix separates event facts from display items and renders tool output through UI-specific cards. Genesis now provides a kernel-owned timeline read model so WebUI remains a shell and does not become the owner for raw event interpretation or tool-call/result merging.
- Evidence: `Kernel.UITimeline` and `GET /sessions/{id}/timeline` project user messages, merged tool cards, assistant messages, and failure notices from ledger events. `tool.call` and `tool.result` are merged by `tool_result.for_event_id`; operation events and raw event names stay out of the main timeline. Timeline items expose preview metadata for tool output and omit kernel event ids, operation ids, provider tool-call ids, and raw event types.
- Verification: `TestUITimelineProjectionMergesToolEventsWithoutAuditFields`; `go test ./... -count=1`; `go build ./...`; `CGO_ENABLED=1 go test -race ./internal/kernel -count=1`; `git diff --check`; active retired-concept and provider-native core scans returned no matches.
- Acceptance condition: reviewer confirms WebUI can consume timeline items without rendering raw events or duplicating owner logic for tool causality.
- Residual risk: the first timeline projection covers current event types. Future reasoning/checkpoint/session-summary events must extend this owner projection rather than being rendered directly by shells.

### KERNEL-CONTEXT-INSPECTION-PROJECTION-20260622 - P1 - Need inspectable runtime context separate from chat timeline

- Status: ready_for_acceptance.
- Fix commits: `7df00cf45`.
- Reference alignment: Reasonix keeps controller context/status separate from transcript items. Genesis now records per-turn provider-visible context snapshots and exposes them through diagnostics inspection, not through the chat timeline.
- Evidence: `turn.submitted` now records model input kinds, tool manifest, safe skill catalog summaries, recalled memory refs, safe provider status, and permission/sandbox summary without storing the fully rendered model-context text in raw events. `Kernel.ContextInspection` and `GET /turns/{id}/context` rebuild that structured snapshot after restart. Older turns without snapshots report `snapshot_unavailable` instead of pretending current runtime state is historical context. Projection output redacts credential-shaped user text and excludes skill paths and bodies.
- Verification: `TestContextInspectionProjectionPersistsProviderVisibleSnapshot`; `TestTurnEvidenceRecordsModelInputKindsWithoutSkillPaths`; `go test ./... -count=1`; `go build ./...`; `CGO_ENABLED=1 go test -race ./internal/kernel -count=1`; `git diff --check`; active retired-concept and provider-native core scans returned no matches.
- Acceptance condition: reviewer confirms users can inspect what context categories and model-visible artifacts reached the provider without polluting the main chat timeline or exposing credentials, skill bodies, or filesystem paths.
- Residual risk: there is no full prompt template/system prompt layer yet, so the first inspection surface records currently implemented model input fragments and tool/skill/memory/provider summaries only.

### KERNEL-PROVIDER-CONTEXT-VISIBILITY-20260622 - P1 - Provider command request must not expose kernel-owned event identity as model-visible state

- Status: ready_for_acceptance.
- Fix commits: `cf81f3206`.
- Reference alignment: Codex separates model-visible call ids from host/runtime tool identity, and Reasonix keeps provider request payloads distinct from display or audit metadata. Genesis now preserves kernel event ids in ledger/session projections while projecting provider-command requests as model-visible context only.
- Evidence: `provider_command` no longer serializes internal `ModelToolRound` directly to external command stdin. It now projects prior tool rounds through a provider-command DTO that preserves provider-visible `tool_call_id`, tool name, arguments, and tool result content while omitting `tool_call_event_id`, `event_id`, `operation_id`, `lease_id`, `permission_mode`, and `for_event_id`. Ledger/session projections still retain `tool_call_event_id` and `for_event_id` for audit, replay, and UI merging.
- Verification: `TestProviderCommandRequestOmitsKernelEventIdentity`; `TestCommandProviderMalformedArgumentsReturnRepairFeedback`; focused provider-command/tool-loop tests; `go test ./... -count=1`; `go build ./...`; `CGO_ENABLED=1 go test -race ./internal/kernel -count=1`; `git diff --check`; active retired-concept and provider-native core scans returned no matches.
- Acceptance condition: reviewer confirms external provider command adapters receive only model-visible tool-loop context and cannot accidentally forward kernel-owned audit or event identity to upstream LLM providers.
- Residual risk: in-process `Provider` implementations still receive the full typed `ModelRequest` because they are part of the kernel test/operator boundary. External provider work should use `provider_command` or a future adapter package with equivalent conformance tests.

### KERNEL-MALFORMED-TOOL-ARGS-REPAIR-20260622 - P1 - Malformed provider-command arguments should become model repair feedback

- Status: ready_for_acceptance.
- Fix commits: `b533a889c`, `d6886250d`, `be60fce1a`.
- Reference alignment: Codex preserves provider protocol pairing while returning recoverable function-call argument errors through tool output when the loop can continue. Reasonix keeps provider tool-call ids for pairing and repairs malformed or missing tool-result pairing at the provider boundary rather than promoting malformed tool arguments into provider infrastructure failures.
- Evidence: `provider_command` no longer rejects invalid raw tool arguments in `toModelResponse`. `ModelToolCall` now has one command-boundary shape for malformed arguments: valid JSON arguments use `arguments`, while malformed argument text uses `raw_arguments`. ToolGateway receives malformed arguments, writes `tool.call`, returns `tool_request_invalid`, writes linked `tool.result`, and executes no shell operation.
- Verification: `TestCommandProviderMalformedArgumentsReturnRepairFeedback`; `TestOpenAICompatibleMalformedToolArgumentsReturnRepairFeedback`; `TestSubmitTurnReturnsRepairFeedbackForInvalidShellArguments`; `TestCommandProviderToolLoopThroughKernel`; focused provider-command/tool-repair/idempotency/readiness suite; `go test ./... -count=1`; `go build ./...`; `CGO_ENABLED=1 go test -race ./internal/kernel -count=1`; `git diff --check`; active retired-concept and provider-native core scans returned no matches.
- Acceptance condition: reviewer confirms malformed provider-command tool arguments are repair feedback when a tool slot can be correlated, not provider command failure, and no external effect occurs.
- Residual risk: provider command adapters must emit malformed argument text through `raw_arguments`; sending malformed text through `arguments` is not the command-boundary contract.

### KERNEL-MODEL-SYSTEM-FIELD-BOUNDARY-20260622 - P1 - Model schemas must expose semantic fields only

- Status: ready_for_acceptance.
- Fix commits: `d6886250d`.
- Reference alignment: Codex keeps tool input schemas focused on model-action payloads while host identifiers, approvals, sandbox state, and event ids stay host-owned. Reasonix provider/tool abstractions keep provider call ids for pairing but do not ask models to generate host lifecycle ids.
- Evidence: `docs/kernel-contract.md` now defines semantic/user-supplied fields versus system-bound/audit-only fields. `shell_exec` continues to expose only `command` and optional `cwd` in the model-visible schema. Strict tool argument decoding rejects injected control-plane fields such as `permission_mode`, `event_id`, `operation_id`, `lease_id`, `task_id`, `tool_call_event_id`, and `provider_tool_call_id` as repairable invalid tool arguments without executing effects.
- Verification: `TestSubmitTurnReturnsRepairFeedbackForUnknownModelToolArgumentFields`; `go test ./... -count=1`; `go build ./...`; `CGO_ENABLED=1 go test -race ./internal/kernel -count=1`; `git diff --check`; active retired-concept and provider-native core scans returned no matches.
- Acceptance condition: reviewer confirms model-visible tool schemas contain only semantic action inputs and model-supplied control-plane fields cannot override kernel-generated event, operation, lease, task, provider, or audit identity.
- Residual risk: this entry covers the current kernel tool schema. Future WorkRegistry or Accumulation model-visible schemas must apply the same field classification before becoming active tool surfaces.

### KERNEL-PROVIDER-GATEWAY-TRANSLATOR-20260622 - P1 - Provider wire compatibility belongs behind gateway translators

- Status: ready_for_acceptance.
- Fix commits: `10c11da35`, `d6886250d`, `be60fce1a`.
- Reference alignment: Codex keeps provider wire protocol handling behind API/client/protocol modules while the core tool loop consumes typed items. Reasonix registers provider implementations behind a provider abstraction and keeps OpenAI/Anthropic wire terms inside provider packages. Genesis now treats `provider_command` as the preferred provider boundary and constrains provider-native wire terms to adapter/translator files.
- Evidence: `provider_command` remains the long-lived command boundary for external provider translators, while the built-in OpenAI-compatible adapter is treated as an adapter/translator file. Provider command stderr is redacted before HTTP, event, session, or ledger projection, and command processes inherit only explicitly configured environment variables. `TestArchitectureBoundaryProviderWireTermsStayInsideAdapterFiles` scans runtime Go files and fails if `/chat/completions`, chat-completion structs, token usage wire names, DeepSeek, OpenRouter, or `openai-responses` terms appear outside the explicit adapter file allowlist. `TestArchitectureBoundaryCoreLoopHasNoProviderNativeWireTerms` continues to guard the turn loop, ToolGateway, provider interface, command provider, tool registry, and core types.
- Verification: `TestArchitectureBoundaryProviderWireTermsStayInsideAdapterFiles`; `TestArchitectureBoundaryCoreLoopHasNoProviderNativeWireTerms`; `TestCommandProviderToolLoopThroughKernel`; `TestProviderCommandFailureRedactsStderrFromTurnAndHTTP`; `TestSubmitTurnExecutesOpenAICompatibleToolCallBeforeFinal`; `go test ./... -count=1`; `go build ./...`; `CGO_ENABLED=1 go test -race ./internal/kernel -count=1`; `git diff --check`; active retired-concept and provider-native core scans returned no matches.
- Acceptance condition: reviewer confirms adding DeepSeek/OpenAI Responses/OpenRouter compatibility now requires an adapter/translator boundary or provider command process and cannot be implemented by adding vendor wire branches to the kernel turn loop or core tool path.
- Residual risk: the built-in OpenAI-compatible adapter still lives in the `internal/kernel` package as a local operator convenience. Moving it to a separate Go package can be a later cleanup, but current guards prevent its wire terms from spreading into core kernel files.

### KERNEL-TOOL-CALL-EVENT-ID-20260622 - P1 - Tool call identity should be kernel event id

- Status: ready_for_acceptance.
- Fix commits: `a4e57c86f`, `fe25c2f7f`.
- Reference alignment: Codex distinguishes provider protocol correlation from internal event/control flow, and tool routing stays typed. Reasonix event-style flows keep local event identity separate from transport correlation. Genesis now keeps provider correlation as adapter data and uses ledger event ids for kernel tool facts.
- Evidence: `SubmitTurn` now writes `tool.call` events before tool preparation and normalizes each admitted tool slot with two explicit identities: provider-visible `tool_call_id` remains the provider echo id, while kernel-owned `tool_call_event_id` is the `tool.call` event id used for operation idempotency, audit, replay linkage, and `tool.result.for_event_id`. Session event projections store `tool_call_event_id` plus `provider_tool_call_id`, so provider-native ids remain correlation evidence rather than kernel owner truth. Duplicate provider ids in one model batch fail before `tool.call` events or effects. Provider calls with missing or unsafe native ids can still execute through a kernel event id without promoting provider strings into operation identity.
- Verification: `TestSubmitTurnExecutesOpenAICompatibleToolCallBeforeFinal`; `TestSubmitTurnUsesToolCallEventIDWhenProviderIDMissing`; `TestSubmitTurnUsesKernelEventIDForUnsafeProviderToolCallID`; `TestSubmitTurnRejectsDuplicateToolCallIDBeforeAnyEffect`; `TestSubmitTurnReturnsRepairFeedbackForInvalidShellArguments`; `TestOpenAICompatibleMalformedToolArgumentsReturnRepairFeedback`; `TestCommandProviderToolLoopThroughKernel`; `go test ./... -count=1`; `go build ./...`; `CGO_ENABLED=1 go test -race ./internal/kernel -count=1`; `git diff --check`; active retired-concept and provider-native core scans returned no matches.
- Acceptance condition: reviewer confirms session, replay, model-visible tool results, and provider adapter correlation keep provider-visible `tool_call_id` separate from kernel-owned `tool_call_event_id`, and never require provider-native ids to be stable kernel facts.
- Residual risk: provider-native ids are still retained in session projections as correlation evidence for adapter debugging. They must not become idempotency keys, operation ids, or owner truth in future tool state.

### KERNEL-PROVIDER-COMMAND-ADAPTER-20260622 - P1 - Provider should prefer external command adapter boundary

- Status: ready_for_acceptance.
- Fix commits: `10c11da35`, `b533a889c`, `be60fce1a`.
- Reference alignment: Codex keeps provider wire protocols behind typed model-client surfaces and dispatches tools through typed tool routing. Reasonix keeps provider/tool/plugin concepts registry-driven and uses stdio transport hygiene for extension boundaries. Genesis now keeps `ModelRequest`, `ModelResponse`, `ModelToolCall`, `ModelToolRound`, `ToolSpec`, and session ledger events as the kernel contract while `provider_command` owns the provider step transport.
- Evidence: `CommandProvider` runs a configured external executable, writes one `genesis.provider_command` JSON request to stdin, and accepts one stdout response with `kind=final` or `kind=tool_calls`. It now runs with explicit environment variables only, applies a default bounded timeout when the caller does not configure one, and redacts command stderr before projecting provider failures. `ResolveProviderConfigFromGenesis` can resolve `models.json` routes with `protocol=provider_command`, and `genesisd` can build the command provider through `genesis-config` or direct `-provider provider_command`. The built-in OpenAI-compatible adapter remains available as an operator convenience but is documented as not being the default contract for new provider integrations. `TestArchitectureBoundaryCoreLoopHasNoProviderNativeWireTerms` prevents core turn/tool files from importing OpenAI-native wire terms.
- Verification: `TestCommandProviderCompletesFromTypedStdoutEvent`; `TestCommandProviderToolLoopThroughKernel`; `TestCommandProviderRejectsInvalidAdapterResults`; `TestCommandProviderDoesNotInheritDaemonEnvironment`; `TestProviderCommandFailureRedactsStderrFromTurnAndHTTP`; `TestCommandProviderAppliesDefaultTimeout`; `TestResolveProviderConfigFromGenesisSelectsCommandProviderRoute`; `TestBuildProviderFromGenesisConfigCanSelectCommandProvider`; `TestArchitectureBoundaryCoreLoopHasNoProviderNativeWireTerms`; `go test ./... -count=1`; `go build ./...`; `CGO_ENABLED=1 go test -race ./internal/kernel -count=1`; `git diff --check`; active retired-concept and provider-native core scans returned no matches.
- Acceptance condition: reviewer confirms new provider integrations can be delivered as external commands without modifying the kernel turn loop, tool gateway, ledger event schema, or app-specific code paths, and that OpenAI-compatible HTTP is not treated as the long-lived provider contract.
- Residual risk: there is not yet a separate external OpenAI command adapter binary in the repo. This retirement proves the kernel boundary and executable contract, not a packaged provider adapter ecosystem.

### KERNEL-MODEL-VISIBLE-TOOL-RESULT-MINIMAL-20260622 - P1 - Model-visible tool results should exclude permission and audit fields

- Status: ready_for_acceptance.
- Fix commits: `b533a889c`.
- Reference alignment: Codex models terminal output and structured tool errors separately from sandbox, approval, and audit state. Reasonix keeps policy/control metadata out of provider-facing tool content. Genesis now keeps the LLM in the operator role while the kernel retains permission, audit, and recovery evidence in inspection projections.
- Evidence: `modelOperationResult` now returns only terminal-equivalent command evidence: status, executed flag, exit code, bounded stdout/stderr, and truncation metadata. Permission blocks return model-visible `permission_denied` feedback without permission mode, blocker reason, operation id, command, cwd, timestamps, or infrastructure reason. Full `OperationProjection` still records permission mode and blocker reason in session/operation inspection.
- Verification: `TestSubmitTurnExecutesOpenAICompatibleToolCallBeforeFinal`; `TestSubmitTurnFeedsNonZeroShellExitToModel`; `TestSubmitTurnReturnsMinimalPermissionDeniedToolResult`; `go test ./... -count=1`; `go build ./...`; `CGO_ENABLED=1 go test -race ./internal/kernel -count=1`; `git diff --check`; active retired-concept and provider-native core scans returned no matches.
- Acceptance condition: reviewer confirms model-visible tool results contain terminal-equivalent output or minimal repair feedback only, while authorized inspection surfaces still expose permission and audit evidence.
- Residual risk: provider-native ids are now correlation fields only, covered by `KERNEL-TOOL-CALL-EVENT-ID-20260622`. Future tool state must continue to avoid treating those ids as kernel identity or idempotency keys.

### KERNEL-SESSION-EVENT-STREAM-UNIFICATION-20260622 - P1 - Session facts should converge on typed event stream

- Status: ready_for_acceptance.
- Fix commits: `16efa7e86`.
- Reference alignment: Codex protocol surfaces ordered events and explicit tool call/result relationships; Reasonix-style controller flows keep lifecycle facts behind one control surface. Genesis now treats the session event stream as the fact source and keeps object projections as ledger-derived read models only.
- Evidence: `docs/kernel-contract.md` defines session events as the primary fact stream and states that session, turn, operation, work, and memory views are derived read models. `SubmitTurn` writes `tool.call`, turn-scoped `operation.*`, and `tool.result` events, with `tool_result.for_event_id` pointing to the originating `tool.call`. Provider replay now rebuilds model tool rounds from stored turn events instead of transient in-memory state. `GET /sessions/{id}` projects ordered events with typed payload data. Long-term tests now assert the current event contract and do not lock retired event/tool names.
- Verification: focused provider tool-loop event tests; `go test ./internal/kernel -count=1`; `go test ./... -count=1`; `go build ./...`; `CGO_ENABLED=1 go test -race ./internal/kernel -count=1`; `git diff --check`; active code/docs retired-concept scan returned no matches.
- Acceptance condition: reviewer confirms session event ordering and typed payloads are sufficient for shells to render or replay tool causality without treating turn, operation, work, or memory projection arrays as independent truth.
- Residual risk: the current provider step still uses the built-in OpenAI-compatible adapter. Moving provider protocol handling behind a command adapter remains tracked by `KERNEL-PROVIDER-COMMAND-ADAPTER-20260622`.

### KERNEL-TOOL-GATEWAY-REGISTRY-20260622 - P1 - Runtime should execute tools only through ToolGateway

- Status: ready_for_acceptance.
- Fix commits: `c34d2baf5`.
- Reference alignment: Codex ties model-visible tool metadata to governed tool executors, and Reasonix routes agent tool calls through a runtime registry. Genesis now uses one `ToolRegistry` for tool name, description, input schema, side-effect level, execution kind, and executor binding; the turn loop and provider adapters consume a gateway-generated manifest instead of knowing concrete tool execution paths.
- Evidence: `ToolRegistry` now exposes `ToolSpec` records with `name`, `description`, `input_schema`, `side_effect_level`, and `execution_kind`; `ToolGateway` owns provider tool batch preflight and execution. `SubmitTurn` calls only `ToolGateway.ToolManifest`, `ToolGateway.PrepareBatch`, and `ToolGateway.Execute` for model tool handling. Direct `POST /tools/shell_exec` also enters the same gateway before shell execution. `TestArchitectureBoundaryToolRegistryBindsSurface` and `TestArchitectureBoundaryToolRegistryRejectsIncompleteSpecs` prove tool specs cannot omit required registry fields or use provider-unsafe dotted ids. `TestSubmitTurnProjectsRegisteredToolManifestWithSkillCatalog` proves the provider sees the registry-generated manifest, not a hand-built provider descriptor. Existing generic unsupported-tool and mixed-batch tests continue to prove unregistered tools return model repair feedback without executing effects. Long-term tests that locked retired tool names were removed; active code/docs scans now return no matches for retired tool ids or old registry helper names outside this retirement log.
- Verification: `go test ./internal/kernel -count=1`; `go test ./... -count=1`; `go build ./...`; `CGO_ENABLED=1 go test -race ./internal/kernel -count=1`; `git diff --check`; active code/docs retired-concept scan returned no matches.
- Acceptance condition: reviewer confirms runtime, provider adapters, direct HTTP tool transport, capability projection, and permission gates all derive from `ToolRegistry`/`ToolGateway`, and future tools cannot be added by special-casing concrete tool implementations inside the turn loop.
- Residual risk: provider command extraction is now covered by `KERNEL-PROVIDER-COMMAND-ADAPTER-20260622`; future provider wire compatibility must remain behind command or adapter translator boundaries.

### KERNEL-BOUNDARY-REFERENCE-ALIGNMENT-20260622 - P1 - Kernel changes need reference-alignment notes against Codex and Reasonix

- Status: ready_for_acceptance.
- Fix commits: `b8a013be4`.
- Reference alignment: Codex is the reference for terminal-equivalent tool results, approval/sandbox rigor, and protocol separation; Reasonix is the reference for registry-driven provider/tool/plugin loading and frontend-agnostic control. Genesis now requires issue records to compare those control-plane ideas explicitly instead of relying on review memory or superficial name matching.
- Evidence: `TestArchitectureBoundaryKernelIssuesRequireReferenceAlignment` proves every active `KERNEL-*` issue has a `Reference alignment` field and future `KERNEL-BOUNDARY-*` retirement entries retain that field. The test intentionally checks structure rather than prose quality, so governance does not become a content judge. Verification passed: focused architecture boundary tests; `go test ./... -count=1`; `go build ./...`; `go test -race ./internal/kernel -count=1`; `git diff --check`; versioned route scan returned no matches.
- Acceptance condition: reviewer confirms future kernel boundary work cannot enter the repo issue ledger without an explicit Codex/Reasonix/intentionally-different comparison.
- Residual risk: the test proves the field exists, not that the comparison is wise. Human review still owns architectural judgment.

### KERNEL-BOUNDARY-SHELL-MINI-RUNTIME-20260622 - P1 - Default shell mode risks becoming a mini shell implementation

- Status: ready_for_acceptance.
- Fix commits: `bdf879293`.
- Reference alignment: Codex keeps terminal execution behind a governed process/sandbox path and returns terminal-equivalent results; Reasonix treats shell execution as a tool behind permission/sandbox gates. Genesis now keeps `shell.go` focused on operation orchestration and moves default-mode command parsing, workspace containment, link checks, and raw host-shell execution behind separate runtime adapter files.
- Evidence: `TestArchitectureBoundaryControlledShellAllowlistStaysSmall` locks the tiny default allowlist; `TestArchitectureBoundaryShellGoOnlyOwnsOrchestration` prevents `shell.go` from importing filesystem/process/path/runtime/syscall packages or redeclaring adapter/parser/runtime functions; `TestArchitectureBoundaryShellRuntimeHasNoApplicationAliases` prevents application/channel aliases in shell runtime files. `TestExecShellDefaultBlocksHardlinkAlias`, `TestExecShellDefaultBlocksRawShellAndEnvironmentAccess`, and existing workspace escape/link tests prove default mode remains controlled rather than a broad shell. Verification passed: focused shell/architecture tests; `go test ./... -count=1`; `go build ./...`; `go test -race ./internal/kernel -count=1`; `git diff --check`; versioned route and shell runtime application-alias scans returned no matches.
- Acceptance condition: reviewer confirms `shell.exec` remains a generic terminal/process primitive and that Feishu, email, calendar, document, or channel behavior must arrive through user-space skills, CLIs, or daemons rather than kernel command parsers.
- Residual risk: default mode still contains a deliberately small controlled adapter for local workspace safety. Replacing it with a true sandbox/process adapter remains future work, and any allowlist expansion must pass Tool Registry and Permission Gate review.

### KERNEL-BOUNDARY-PERMISSION-GATE-20260622 - P0 - Permission policy is embedded in shell execution instead of a generic gate

- Status: ready_for_acceptance.
- Fix commits: `0729dc4b0`.
- Reference alignment: Reasonix separates pure permission policy/gates from tool implementations; Codex separates approval/sandbox decisions from concrete exec result handling. Genesis now asks a kernel-owned authority gate before effects, so `shell.exec` is one effectful tool under the gate rather than the owner of permission semantics.
- Evidence: `authorizeKernelTool` now allows read tools, blocks effect tools in `plan`, allows effect tools in `default` and `yolo`, and fails closed for unknown modes/kinds. `TestArchitectureBoundaryAuthorityGateUsesToolKind` proves those decisions flow from tool kind; `TestExecShellPlanBlocksMutatingCommand` proves shell execution respects the generic gate; `TestSubmitTurnExecutesOpenAICompatibleToolCallBeforeFinal` proves model-requested shell tools pass through the same kernel tool path. Verification passed: focused registry/gate/tool-loop tests; `go test ./... -count=1`; `go build ./...`; `go test -race ./internal/kernel -count=1`; `git diff --check`; versioned route scan returned no matches.
- Acceptance condition: reviewer confirms read/effect classification and permission decisions are no longer owned by shell-specific code and future effectful tools must reuse the same gate.
- Residual risk: the gate is intentionally minimal for the current kernel spike. Richer approval prompts, per-tool capability grants, and sandbox profiles need separate contracts before broadening tool execution.

### KERNEL-BOUNDARY-TOOL-REGISTRY-20260622 - P0 - Tool descriptors are still hardcoded in the kernel instead of owned by a registry

- Status: ready_for_acceptance.
- Fix commits: `0729dc4b0`.
- Reference alignment: Reasonix registers built-ins and plugin tools into a runtime registry; Codex ties model-visible tool metadata to executor contracts. Genesis now has a small `kernelToolDefinition` registry binding descriptor, read/effect kind, and prepare handler for canonical kernel tools.
- Evidence: the tool registry is now the source for the canonical `shell_exec` tool; model descriptors, capability projection, and model tool preflight project from that registry. Skills are projected separately as path-free metadata catalog entries, not as registered model-visible tools. `TestArchitectureBoundaryToolRegistryBindsSurface` proves every visible tool has descriptor, kind, parameter schema, and handler binding; `TestArchitectureBoundaryCapabilitiesProjectFromToolRegistry` proves capability projection follows the registry; `TestArchitectureBoundaryModelVisibleToolsStayGeneric` prevents Feishu/email/calendar-like application names from entering descriptors. Verification passed: focused registry/gate/tool-loop tests; `go test ./... -count=1`; `go build ./...`; `go test -race ./internal/kernel -count=1`; `git diff --check`; versioned route scan returned no matches.
- Acceptance condition: reviewer confirms adding a model-visible kernel tool now requires one registry entry with descriptor, kind, and handler binding, and provider/capability surfaces cannot drift through parallel switches.
- Residual risk: the registry is deliberately in-process and static for the spike. Dynamic plugin loading, signed external tools, and richer per-tool policy remain future contracts.

### KERNEL-BOUNDARY-SEMANTIC-TEXT-20260622 - P0 - Semantic text must not be rejected by secret-shaped heuristics

- Status: ready_for_acceptance.
- Fix commits: `3fb91aa8e`.
- Reference alignment: Codex preserves model/user strings as repair feedback or terminal-equivalent tool evidence rather than rejecting them because they resemble secrets; Reasonix keeps schema/permission failures separate from arbitrary narrative content. Genesis now keeps control-plane refs, authorities, session ids, and retry keys grammar-gated while admitting ordinary WorkRegistry and Accumulation narrative text as text.
- Evidence: `TestSemanticTextFieldsAllowSecretShapedContent` proves Work title, Work cancel reason, memory approval reason, memory rejection reason, memory supersession reason, and supersession replacement text can contain secret-shaped quoted content and are preserved through HTTP owner paths. `TestArchitectureBoundarySemanticFieldsDoNotUseSecretRejector` proves those narrative fields no longer call the secret-shaped rejector. `TestHTTPCreateWorkRejectsInvalidControlRefs`, `TestHTTPCancelWorkRejectsInvalidControlRefs`, `TestHTTPCreateMemoryCandidateRejectsInvalidControlRefs`, `TestHTTPApproveMemoryCandidateRejectsInvalidControlRefs`, `TestHTTPRejectMemoryCandidateRejectsInvalidControlRefs`, and `TestHTTPMemoryCandidateSupersedeRejectsInvalidControlRefs` prove control-plane identifiers, refs, and authorities still fail closed when malformed or secret-shaped. Verification passed: focused semantic/control-ref tests; `go test ./... -count=1`; `go build ./...`; `go test -race ./internal/kernel -count=1`; `git diff --check`; versioned route scan returned no matches.
- Acceptance condition: reviewer confirms Genesis no longer uses secret-shaped content heuristics as an admission rule for ordinary work or memory narrative text, while control-plane refs and authorities remain grammar-gated.
- Residual risk: default projections and tool evidence still redact secret-shaped shell/provider/skill evidence where those surfaces can expose credentials. This retirement covers admission for semantic text fields only, not raw-evidence access policy.

### KERNEL-TOOL-RESULT-TAXONOMY-20260622 - P0 - Tool loop must preserve terminal-equivalent command results

- Status: ready_for_acceptance.
- Fix commits: `0c7960172`, `fb519b7ae`.
- Reopen note: Feishu Base record `recvnf7FRCr2NG` was reopened after review found that head/tail truncation metadata existed, but model-visible stdout/stderr text still concatenated head and tail without an omission marker. Follow-up commit `fb519b7ae` makes the omission visible in the bounded output body.
- Evidence: `TestSubmitTurnReturnsRepairFeedbackForInvalidShellArguments` first failed because malformed `shell.exec` arguments terminated the turn as `model tool call rejected`, then passed after the Tool System began returning structured `tool_request_invalid` repair feedback to the model without executing an operation. `TestSubmitTurnRejectsInvalidToolCallIDBeforeToolCallEvent` proves an unrecoverable bad `tool_call_id` fails before `model.tool_call` evidence is appended. `TestSubmitTurnReturnsRepairFeedbackForMixedModelToolBatchBeforeAnyEffect`, `TestSubmitTurnReturnsRepairFeedbackForUnknownModelToolArgumentFields`, `TestSubmitTurnReturnsRepairFeedbackForUnknownSkillReadBeforeShellEffect`, and `TestSubmitTurnReturnsRepairFeedbackForChangedSkillReadBatchBeforeShellEffect` prove invalid mixed batches create no shell effects while returning repair results for each call. `TestSubmitTurnFeedsNonZeroShellExitToModel` proves an executed shell command with nonzero exit returns model-visible `status=failed`, `exit_code`, and stderr while omitting ledger control-plane handles from the model-facing tool result. `TestExecShellReportsHeadTailTruncationMetadata` proves long stdout/stderr are bounded with head/tail content, truncation flags, original byte counts, omitted byte counts, `output_truncation=head_tail`, and a visible `[... N bytes omitted ...]` marker between preserved head and tail content. `TestSubmitTurnReportsToolInfrastructureFailureSeparately` proves ledger/tool infrastructure failure records `tool_infrastructure_failed` rather than a command stderr. `TestSubmitTurnSkillReadUnavailableRepairDoesNotExposeInstructionPath` and `TestExecShellControlledReadFailureDoesNotExposeAbsolutePath` were added after security review to prove internal file paths are not surfaced through repair messages or controlled-shell synthetic stderr. Independent architecture review found model-visible repair content duplicated `tool_call_id` and that docs incorrectly classified provider adapter errors as Tool System infrastructure; both were fixed. A brief over-sanitizing attempt for unknown tool names was rejected after user review because it diverged from Codex-style repair feedback and local terminal semantics. Reopen verification passed: `go test -count=1 ./internal/kernel -run "TestExecShellReportsHeadTailTruncationMetadata|TestSubmitTurnFeedsNonZeroShellExitToModel|TestSubmitTurnReturnsRepairFeedbackForInvalidShellArguments|TestSubmitTurnReportsToolInfrastructureFailureSeparately" -v`; `go test ./... -count=1`; `go build ./...`; `CGO_ENABLED=1 go test -race ./internal/kernel -count=1`; `git diff --check`; the repository scan for numbered route prefixes returned no matches.
- Acceptance condition: reviewer confirms model tool results follow Codex-style recoverable feedback: invalid requests with valid protocol handles return model-repairable evidence without effects, command exits remain terminal-equivalent command evidence, permission blocks remain operation blockers, and tool infrastructure failures are separate from command stderr. Reviewer also confirms model-facing tool result content excludes operation/session/turn/idempotency handles while authorized session/API projections retain ledger evidence.
- Residual risk: default-mode controlled shell commands synthesize a small path-free stderr for internal read/write failures instead of reproducing a native shell's full diagnostic. Yolo-mode shell execution still preserves real shell stdout/stderr under the head/tail cap. Provider failures remain Model Gateway failures and are intentionally outside this Tool System taxonomy.

### KERNEL-CONTEXT-PROVENANCE-20260622 - P2 - Model input context needs provenance categories

- Status: ready_for_acceptance.
- Fix commits: `1c8ec0d88`.
- Evidence: `ModelRequest.InputItems` now uses typed `ModelInputItem` fragments instead of public `InputItem`; initial kinds are `skill_catalog_context`, `approved_memory_context`, and `user_text`. `TestModelInputItemsInjectsApprovedMemoryContextBeforeProvider` proves approved memory is no longer just another anonymous user text item. `TestKernelInjectsSkillCatalogBeforeProviderWithoutSkillBodies` proves skill catalog context is categorized before user text and still excludes full skill bodies and paths. `TestTurnEvidenceRecordsModelInputKindsWithoutSkillPaths` proves provider request, turn event data, and session projection expose ordered `model_input_kinds` for skill catalog, approved memory, and user text while the public `input_items` projection remains the original user text only and inspection JSON does not leak instruction paths, skill bodies, or injected catalog text. `TestOpenAICompatibleProviderCompletesAgainstCompatibleServer` and `TestKernelBuildsApprovedMemoryContextBeforeOpenAICompatibleProvider` prove the OpenAI-compatible adapter consumes owner-built model inputs as transport content rather than owning memory or skill context semantics. Verification passed: focused provenance/provider tests; `go test ./... -count=1`; `go test -race ./internal/kernel -count=1`; `go build ./...`; `git diff --check`; the repository scan for numbered route prefixes or kernel version labels returned no matches.
- Acceptance condition: reviewer confirms public turn input remains user/application content, while kernel-built memory and skill context are distinguishable in canonical model request and authorized inspection evidence without elevating user-space skill/memory text to system authority.
- Residual risk: provider transport currently flattens typed fragments into one OpenAI-compatible user message. That is intentional for the current provider protocol; future providers may preserve richer message boundaries only if Model Gateway keeps provenance ownership and adapters remain transport translators.

### KERNEL-BUILD-BROKEN-MODEL-INPUT-PROVENANCE-20260622 - P0 - ModelInputItem provenance refactor must compile

- Status: ready_for_acceptance.
- Fix commits: `1c8ec0d88`, `ba66248fd`.
- Evidence: The temporary dirty provenance refactor left several tests and provider stubs on the old `InputItem` contract. Commit `1c8ec0d88` completed the migration by updating fake provider, OpenAI-compatible provider tests, skill catalog provider stubs, typed model context assembly, and session/turn evidence. Verification passed after the migration: focused provenance/provider tests; `go test ./... -count=1`; `go test -race ./internal/kernel -count=1`; `go build ./...`; `git diff --check`; the repository scan for numbered route prefixes or kernel version labels returned no matches. Commit `ba66248fd` records the matching acceptance evidence.
- Acceptance condition: reviewer confirms the working tree is clean, the Go kernel compiles, and the ModelInputItem provenance refactor is either fully submitted or absent rather than left as a broken dirty diff.
- Residual risk: this was a transient integration break caught during review, not a separate runtime design gap. Future large contract migrations should keep focused compile/test checkpoints before exposing the branch for acceptance.

### KERNEL-SKILL-READ-BOUNDARY-20260622 - P1 - Skill instructions should not be a first model-visible kernel tool

- Status: ready_for_acceptance.
- Fix commits: `0ff18a793`.
- Reference alignment: Codex does not expose a model tool for reading skill packages or agent guidance files directly, and Reasonix keeps skill/task concepts separate from ordinary file/process tools. Genesis now keeps configured skills as metadata-only user-space context until a generic resource/context contract exists.
- Evidence: The default kernel tool registry now exposes only `shell_exec`; the former skill-instruction descriptor, OpenAI-compatible translation alias, projection struct, instruction-body reader, and prepare path were removed. `TestSubmitTurnDoesNotExposeSkillReadAsModelTool` proves provider tool manifests include `shell_exec` and exclude the removed skill-instruction tool and provider alias. `TestHTTPCapabilitiesProjectsToolsAndSkillCatalogWithoutPaths` proves `/capabilities` still exposes safe skill name/description metadata while excluding skill-instruction tools, paths, and bodies. Unsupported requests for the removed tool now exercise the generic unsupported-tool repair path, with mixed batches still producing no shell effect. Verification passed: focused skill boundary/capability/tool tests; `go test ./... -count=1`; `go test -race ./internal/kernel -count=1`; `go build ./...`; `git diff --check`; the repository scan for numbered route prefixes or kernel version labels returned no matches.
- Acceptance condition: reviewer confirms skill packages remain user-space context assets, not default model-visible kernel tools, and future full skill instruction loading must be designed as a generic resource/context boundary rather than a skill-specific syscall.
- Residual risk: the skill catalog currently provides only name/description metadata. Real use of complex external skills may need a future context hydration contract, but adding it requires a separate owner decision and must not create Feishu/email/calendar/document-specific kernel adapters.

### KERNEL-TOOL-NAMING-UNDERSCORE-20260622 - P1 - Canonical tool ids should not keep dotted names

- Status: ready_for_acceptance.
- Fix commits: `bbcc60636`.
- Reference alignment: Codex tool names are provider-safe identifiers such as `exec_command` and `apply_patch`; Reasonix uses names such as `read_file`, `write_file`, and `bash_output`. Genesis now uses a provider-safe canonical id for the surviving default tool rather than maintaining separate kernel and provider names.
- Evidence: The default tool id is now `shell_exec` in the registry, provider tool specs, model tool calls/results, operation ledger evidence, session projection, capability projection, README, contract docs, and HTTP route `POST /tools/shell_exec`. The OpenAI-compatible adapter no longer has dotted-name mapping functions. `TestArchitectureBoundaryToolRegistryBindsSurface` now fails any dotted tool id at the registry boundary. `TestSubmitTurnExecutesOpenAICompatibleToolCallBeforeFinal` proves provider request, assistant tool replay, and model-visible operation evidence use `shell_exec`. `TestLiveOpenAICompatibleProviderToolLoopThroughKernel` passed against the real configured provider using `shell_exec`. Verification passed: focused naming/provider/HTTP tests; `go test ./... -count=1`; `go test -race ./internal/kernel -count=1`; `go build ./...`; gated live provider tool-loop smoke; `git diff --check`; active code/current docs scans for dotted shell id and adapter mapping functions returned no matches; the versioned route scan returned no matches.
- Acceptance condition: reviewer confirms there is no active dotted shell tool contract, no route alias, no adapter-local canonical-name translation, and no provider alias for the removed skill-instruction tool because skill-specific instruction reads were removed from the default tool surface first.
- Residual risk: historical ready/retirement records still mention older dotted ids as evidence. Those are documentation history, not active contracts.

### KERNEL-CAPABILITIES-20260622 - P1 - Shells and daemons need a protected kernel capability projection

- Status: ready_for_acceptance.
- Fix commits: `2654f0877`, `65f004277`.
- Evidence: `TestHTTPCapabilitiesRequiresRuntimeAuth` first failed because `/capabilities` returned 404, then passed after implementation and proves the route requires the runtime bearer token. `TestHTTPCapabilitiesProjectsToolsAndSkillCatalogWithoutPaths` proves authenticated inspection returns canonical `shell_exec` capability metadata plus safe skill name/description metadata, while excluding `instruction_path`, `ledger_path`, filesystem paths, and skill bodies. `TestHTTPCapabilitiesReportsPathFreeSkillExclusions` proves missing roots, malformed metadata, unsafe metadata, linked paths, and duplicate skill names are projected only as path-free reason/count diagnostics. `TestHTTPCapabilitiesSanitizesProviderInspectionStatus` and `TestHTTPCapabilitiesSanitizesCredentialShapedProviderTokens` were added after security review found provider readiness strings could leak secret-shaped single tokens; they now prove path-shaped provider names, `secret://...`, `Authorization: Bearer ...`, bare `sk-...`, and embedded `sk-proj-...` tokens are replaced with safe inspection fallbacks. `TestToolCapabilityKindDefaultsUnknown` proves future tools are not silently classified as effectful capabilities unless explicitly mapped. `go test ./internal/kernel -run "TestHTTPCapabilities|TestToolCapabilityKindDefaultsUnknown" -count=1` passed; `go test ./... -count=1` passed; `go build ./...` passed; `go test -race ./internal/kernel -count=1` passed; `git diff --check` passed; the repository scan for numbered route prefixes returned no matches. Independent architecture review reported no kernel/app boundary blocker. Independent security review initially found the provider inspection sanitizer leak; after the credential-shaped token fix it reported no blocking findings.
- Acceptance condition: reviewer confirms `GET /capabilities` is a protected Readiness/Inspection surface derived from kernel-owned provider/runtime/ledger readiness, canonical tool descriptors, and safe skill catalog metadata; it is not an app registry, Feishu/email/WebUI adapter, or second owner for external outbound communication.
- Residual risk: provider inspection reasons are guarded by shape and credential detection rather than a dedicated enum type. Current production provider reasons are fixed `provider_*` codes; future providers must not return arbitrary raw error text through `ProviderStatus` without updating this contract.

### KERNEL-SKILL-READ-20260622 - P0 - Model loop needs governed skill instruction retrieval

- Status: superseded by `KERNEL-SKILL-READ-BOUNDARY-20260622`; this is not an active acceptance condition.
- Fix commits: `ff92814db`, `f89f55409`.
- Superseded reason: this earlier direction briefly treated skill instruction retrieval as a model-visible tool. Architecture review rejected that boundary: skills are user-space assets, the default kernel tool surface stays generic, and skill catalog exposure is metadata-only until a future generic resource/context contract exists.
- Evidence: the later boundary fix removed the skill-specific descriptor, provider alias, projection struct, instruction-body reader, and prepare path. Current positive tests prove provider tool manifests contain `shell_exec`, capability projection exposes path-free skill catalog metadata, and unsupported removed-tool requests flow through the generic repair path without shell effects. `go test ./... -count=1` passed; `go test -race ./internal/kernel -count=1` passed; `go build ./...` passed; `git diff --check` passed.
- Acceptance condition: reviewer confirms this issue is superseded and must not be used to reintroduce a skill-specific model-visible tool. Future full skill instruction loading requires a separate generic resource/context boundary and new owner decision.
- Residual risk: skill bodies remain user-space instructions, not signed or kernel-authoritative policy. The current kernel only exposes safe metadata; live skill reload, signed trust, ranking, and richer skill package governance remain separate contracts.

### KERNEL-SKILL-METADATA-SECURITY-20260622 - P1 - Skill catalog metadata must not inject authority-shaped context

- Status: ready_for_acceptance.
- Fix commits: `fc27be8df`, `152c7d102`.
- Evidence: `TestSkillCatalogRejectsAuthorityAndSecretShapedMetadata` first failed because prompt-injection-shaped, role-marker, tool-protocol, and secret-shaped skill descriptions all entered the catalog. After the fix, it proves only safe skill metadata is injected, while `Ignore previous instructions`, `system:`, `tool_call_id`, invisible-control text, secret-shaped metadata, and full skill bodies are absent from the skill catalog context. Existing `TestKernelInjectsSkillCatalogBeforeProviderWithoutSkillBodies`, `TestMissingAndMalformedSkillCatalogDoesNotBlockTurn`, `TestModelInputItemsInjectsApprovedMemoryContextBeforeProvider`, and `TestKernelBuildsApprovedMemoryContextBeforeOpenAICompatibleProvider` passed, proving safe catalog injection, fail-soft missing/malformed roots, approved memory context, and provider request construction still work. `go test ./...` passed; `go test -race ./internal/kernel -count=1` passed; `go build` passed for both `cmd/genesisd` and `cmd/genesisctl`; `git diff --check` passed; the repository scan for numbered route or kernel version labels returned no matches.
- Acceptance condition: reviewer confirms skill front matter is treated as untrusted user-space metadata before it becomes kernel-built model context, and that unsafe skill metadata is excluded rather than redacted into trusted context or allowed to block the whole turn.
- Residual risk: metadata filtering is syntactic and conservative. It does not provide signed skill trust, live skill reload, skill ranking, or per-turn skill selection; those remain separate kernel contracts if needed.

### KERNEL-SKILL-CATALOG-20260622 - P0 - Model context needs generic external skill discovery

- Status: ready_for_acceptance.
- Fix commits: `c3e20a777`, `5b9b7f0c9`.
- Evidence: `TestKernelInjectsSkillCatalogBeforeProviderWithoutSkillBodies` first failed because `Config.SkillRoots` did not exist, then passed after implementation. It now proves a configured root containing `lark-im/SKILL.md` and `mail/SKILL.md` injects a concise "Available external skills" catalog before the user turn, includes each skill name and description, keeps filesystem paths internal, and does not inject full skill bodies. `TestMissingAndMalformedSkillCatalogDoesNotBlockTurn` proves missing roots and malformed `SKILL.md` metadata are ignored without blocking turn submission. Existing `TestModelInputItemsInjectsApprovedMemoryContextBeforeProvider` and `TestKernelBuildsApprovedMemoryContextBeforeOpenAICompatibleProvider` passed, proving approved memory context and provider request construction still work with the extended model input path. `go test ./...` passed; `go test -race ./internal/kernel -count=1` passed; `go build` passed for both `cmd/genesisd` and `cmd/genesisctl`; `git diff --check` passed; the repository scan for numbered route or kernel version labels returned no matches.
- Acceptance condition: reviewer confirms the skill catalog is a metadata-only kernel context primitive for user-space skills, not a Feishu, email, calendar, document, or channel adapter, and that the active model still reaches external CLIs only through governed tools such as `shell.exec`.
- Residual risk: the catalog is loaded at kernel startup and intentionally injects only front matter metadata. Live skill reload, ranking, trust policy, and per-turn skill selection are future kernel contracts if they become necessary.

### KERNEL-TURN-IDEMPOTENCY-20260622 - P0 - Turn submit retries must not create duplicate model/tool effects

- Status: ready_for_acceptance.
- Fix commits: `18b6e029e`, `7277032e2`, `190cd56d9`.
- Evidence: `TestHTTPTurnSubmitIdempotencyKeyReturnsExistingTurnAfterRestart` first failed because `idempotency_key` was rejected as an unknown `POST /turn` field, then passed after implementation. It proves a duplicate `session_id + idempotency_key` retry after restart returns the original `turn_id` and final answer, does not call the retry provider, and leaves one turn plus two turn events in session projection. `TestHTTPTurnSubmitIdempotencyKeyReturnsExistingFailureAfterRestart` proves a failed provider turn replays the original failure on retry without calling a now-available provider, so the same caller retry boundary cannot silently change effects. A later review found the retry still returned only an HTTP error envelope for failed turns, forcing shells to fetch `/sessions` for evidence. Fix commit `190cd56d9` changes failed idempotent retries to return the original failed `turn_id`, ordered events, and `error.code` from `POST /turn` while preserving the failure HTTP status. `TestHTTPTurnSubmitIdempotencyKeyRequiresValidExplicitSession` proves malformed idempotency keys and keys without explicit `session_id` fail before ledger append. The broader focused suite covering turn admission, provider failure, model tool loop, shell idempotency, and work idempotency passed; `go test ./...` passed; `go test -race ./internal/kernel -count=1` passed; `go build` passed for both `cmd/genesisd` and `cmd/genesisctl`; `git diff --check` passed; the repository scan for numbered route or kernel version labels returned no matches.
- Acceptance condition: reviewer confirms turn idempotency is an Interface Kernel retry boundary scoped to explicit `session_id + turn.submit + idempotency_key`, not shell/WebUI/daemon-owned retry state, and that the control-plane key is not model-visible input.
- Residual risk: if a duplicate retry arrives while the first idempotent turn is still running, the current spike returns a running-state error instead of a full lease/recovery projection. Long-running turn lease, cancellation, and recovery should be specified as a separate kernel contract before changing that behavior.

### KERNEL-WORK-IDEMPOTENCY-20260622 - P1 - Work submit retries create duplicate work records

- Status: ready_for_acceptance.
- Fix commits: `71f4abc95`, `3e606b5a0`, `231363fd5`.
- Evidence: `TestHTTPWorkSubmitIdempotencyKeyReturnsExistingWorkAfterRestart` proves duplicate `POST /work` calls with the same `session_id + idempotency_key` return the original work after restart, preserve the original title/source, and append only one `work.submitted` event. `TestHTTPCreateWorkRejectsInvalidIdempotencyKey` proves malformed retry keys fail before ledger append. The broader WorkRegistry evidence in `KERNEL-WORK-REGISTRY-20260622` also proves submit/read/cancel projection, cancel idempotency, terminal cancel race handling, invalid source/audit ref rejection, and restart-safe session projection. Current verification reran the Work idempotency tests as part of the broader turn/tool/work idempotency suite, then `go test ./...`, `go test -race ./internal/kernel -count=1`, both binary builds, `git diff --check`, and the no-version scan passed.
- Acceptance condition: reviewer confirms WorkRegistry submit idempotency is scoped to `session_id + work.submit + idempotency_key`, not shell/daemon deduplication, and that retries do not create duplicate resumable work anchors.
- Residual risk: WorkRegistry remains a durable record ledger, not a scheduler. The current mutex protects single-process submit idempotency; future multi-process writers still need transactional compare-and-append semantics.

### KERNEL-MEMORY-RECALL-20260622 - P1 - Memory recall needs an explicit kernel observation surface

- Status: ready_for_acceptance.
- Fix commits: `0ec14a963`, `094a67559`.
- Evidence: `TestHTTPMemoryRecallReturnsApprovedOnlyAfterRestartWithoutLedgerAppend` first failed with HTTP 404 for `POST /memory/recall`, then passed after implementation. It proves the protected recall preview returns approved memory refs after restart, excludes pending, rejected, superseded, and pending replacement candidates that would otherwise match the same query, and does not append ledger events. `TestHTTPMemoryRecallRejectsBadInputAndAuth` proves missing runtime auth returns 401, unsupported input item types return 400 before recall, and hidden control text returns 403. Existing turn recall and ingress tests still pass. `go test ./internal/kernel -run "TestHTTPMemoryRecall" -count=1 -v` passed; `go test ./internal/kernel -run "TestHTTPMemory|TestApprovedMemory|TestUnapprovedMemory|TestSubmitTurnRecordsIngressRisk|TestHTTPAcceptsRiskyUserData|TestHTTPRejectsNestedControlField|TestHTTPBlocksInvisibleIngressMarker" -count=1 -v` passed; `go test ./...` passed; `go test -race ./internal/kernel -count=1` passed; `go build` passed for both `cmd/genesisd` and `cmd/genesisctl`; `git diff --check` passed; the repository scan for numbered route or kernel version labels returned no matches.
- Acceptance condition: reviewer confirms `POST /memory/recall` is a read-only Accumulation observation surface for the conceptual `memory.recall` syscall, not a shell-owned memory owner, model turn, vector search project, or application-specific recall workflow.
- Residual risk: recall policy is still the intentionally simple first-pass text matcher. The route previews current policy only; future richer recall policy, ranking, source existence checks, or audit events need separate contracts.

### KERNEL-MEMORY-SUPERSEDE-20260622 - P1 - Memory review needs explicit supersession

- Status: ready_for_acceptance.
- Fix commits: `7235aae74`, `2dc9a34e1`, `750a9be2f`.
- Evidence: `TestHTTPMemoryCandidateSupersedeCreatesPendingReplacementAfterRestart` first failed because `MemorySupersessionProjection`, `MemoryCandidateSuperseded`, and `SupersedeMemoryCandidate` did not exist, then passed after implementation. It proves an approved candidate can be superseded through `POST /memory/candidates/{id}/supersede`, the original candidate replays as `status=superseded` with authority/reason/evidence and `replacement_candidate_id`, the replacement candidate replays as `status=pending`, superseded and pending replacement memories are excluded from recall, and the replacement recalls only after separate approval. `TestSupersedeMemoryCandidateIsIdempotentWithoutAppendingDuplicateReplacement` proves duplicate supersede calls preserve the first replacement and append only one `memory.candidate.superseded` event. `TestHTTPMemoryCandidateSupersedeRejectsMissingEvidence` proves supersession requires replacement text, replacement source, authority, reason, and evidence before candidate lookup. `TestHTTPSupersededMemoryCandidateCannotBeApprovedOrRejected` proves the original superseded candidate cannot later be approved or rejected through the minimal review surface. `TestMemoryReplayRejectsReviewAfterSupersede` proves replay fails closed if a corrupted ledger tries to apply approval after supersession. Review-fix `TestMemoryReplayRejectsDuplicateSupersedeWithModifiedReplacement` proves replay now rejects a duplicate supersession that tries to mutate the replacement payload under the same replacement id. `TestHTTPMemoryCandidateSupersedeRejectsInvalidAuditRefsAndSecretShapedText` proves replacement source, supersession authority, reason, and evidence reject invalid refs or secret-shaped content before ledger append. `TestHTTPCreateMemoryCandidateRejectsInvalidAuditRefsAndSecretShapedText`, `TestHTTPApproveMemoryCandidateRejectsInvalidAuditRefsAndSecretShapedText`, and `TestHTTPRejectMemoryCandidateRejectsInvalidAuditRefsAndSecretShapedText` prove the shared Accumulation audit boundary is enforced across create, approve, reject, and supersede. `TestConcurrentMemorySupersedeWritesOnlyOneTerminalDecision` proves supersede participates in the same terminal review race fixture as approve/reject. `go test ./internal/kernel -run "TestHTTPCreateMemoryCandidate|TestHTTPMemoryCandidateApprove|TestHTTPApproveMemoryCandidate|TestHTTPMemoryCandidateReject|TestHTTPRejectMemoryCandidate|TestHTTPRejectedMemoryCandidate|TestHTTPApprovedMemoryCandidate|TestRejectMemoryCandidate|TestConcurrentMemoryReview|TestConcurrentMemorySupersede|TestHTTPMemoryCandidateSupersede|TestSupersedeMemoryCandidate|TestMemoryReplayRejects|TestHTTPSupersededMemoryCandidate|TestHTTPWork|TestHTTPCancelWork|TestHTTPCreateWork|TestWorkReplay|TestConcurrentWork" -count=1 -v` passed; `go test ./...` passed; `go test -race ./internal/kernel -count=1` passed; `go build` passed for both `cmd/genesisd` and `cmd/genesisctl`; `git diff --check` passed; the repository scan for numbered route or kernel version labels returned no matches.
- Acceptance condition: reviewer confirms supersession is an Accumulation-owned review decision, not an in-place memory edit, hidden approval, migration shim, or product-specific memory workflow.
- Residual risk: replacement recall still uses the intentionally simple first-pass text matcher after approval. Supersession is atomic inside one JSONL event, but future multi-process ledger writers still need transactional append and stronger corruption repair policy.

### KERNEL-WORK-REGISTRY-20260622 - P0 - Minimal WorkRegistry needs a durable submit and cancel loop

- Status: ready_for_acceptance.
- Fix commits: `71f4abc95`, `3e606b5a0`, `231363fd5`.
- Evidence: `TestHTTPWorkSubmitCancelReadAndSessionProjectionAfterRestart` first failed with 404 for `/work`, then passed after implementation. It proves `POST /work` creates an open work record with `source_ref`, `GET /work/{id}` reads it after restart, `POST /work/{id}/cancel` persists a canceled state with authority/reason/evidence, and `GET /sessions/{id}` projects the canceled work after another restart. `TestHTTPWorkSubmitIdempotencyKeyReturnsExistingWorkAfterRestart` proves submit retries with the same `session_id + idempotency_key` return the original work after restart, preserve the original title/source, and append only one `work.submitted` event. `TestHTTPCreateWorkRejectsInvalidIdempotencyKey` proves invalid retry keys fail before ledger append. `TestHTTPCancelWorkIsIdempotentWithoutOverwritingEvidence` proves duplicate cancel calls preserve the first cancel evidence and append only one `work.canceled` event. `TestConcurrentWorkCancelWritesOnlyOneTerminalDecision` proves same-process concurrent cancel callers observe one terminal cancel decision and only one cancel event is appended. `TestWorkReplayRejectsCompetingCancelEvidence` proves a corrupted or competing ledger with two different cancel evidence records fails closed during `Work` and `Session` replay instead of last-writer-wins overwrite. `TestHTTPCreateWorkRequiresSourceRef` proves submit requires source evidence. `TestHTTPCreateWorkRejectsInvalidAuditRefsAndSecretShapedText` and `TestHTTPCancelWorkRejectsInvalidAuditRefsAndSecretShapedText` prove work session id, title, source ref, cancel authority, cancel reason, and cancel evidence ref reject invalid audit shapes or secret-shaped content before ledger append. `go test ./internal/kernel -run "TestHTTPWork|TestHTTPCancelWork|TestHTTPCreateWork|TestWorkReplay|TestConcurrentWork" -count=1 -v` passed; `go test ./...` passed; `go test -race ./internal/kernel -count=1` passed after installing the local Windows gcc toolchain; `go build` passed for both `cmd/genesisd` and `cmd/genesisctl`; `git diff --check` passed; the repository scan for versioned route or kernel version labels returned no matches.
- Acceptance condition: reviewer confirms WorkRegistry is a kernel-owned coordination evidence ledger with submit/read/cancel semantics, not a scheduler, Feishu task API, shell UI state, or application workflow owner.
- Residual risk: this is a durable record ledger, not background execution. The current mutex protects single-process submit/cancel idempotency; replay fails closed on competing terminal cancel evidence, but future multi-process writers still need transactional compare-and-append semantics. Ref validation is syntactic only; proving referenced event existence can be added later as a separate verification step.

### KERNEL-MEMORY-REJECT-20260622 - P1 - Memory review needs a reject path

- Status: ready_for_acceptance.
- Fix commits: `ac2d01571`, `72f1fbe2d`, `69229422a`.
- Evidence: `TestHTTPMemoryCandidateRejectAndReadAfterRestart` first failed with 404 for `/memory/candidates/{id}/reject`, then passed after implementation. It proves a rejected candidate records `rejection_authority`, `rejection_reason`, and `rejection_evidence_ref`, appears under `status=rejected` after restart, disappears from `status=pending`, remains readable with rejection evidence, projects through `GET /sessions/{id}`, and is not recalled into a later turn. `TestHTTPRejectedMemoryCandidateCannotBeApproved` proves a rejected candidate cannot later be approved into active memory through the minimal review surface. `TestHTTPApprovedMemoryCandidateCannotBeRejected` proves approved memory cannot be overwritten by a rejection. `TestRejectMemoryCandidateIsIdempotentWithoutAppendingDuplicateEvent` proves duplicate reject calls do not append competing rejection evidence. `TestConcurrentMemoryReviewWritesOnlyOneTerminalDecision` first failed with two successful terminal review decisions, then passed after the kernel serialized memory review transitions. `TestHTTPRejectMemoryCandidateRejectsMissingEvidence` proves rejection evidence is required before candidate lookup. Existing memory approval and recall tests still pass. `go test ./...` passed; `go build` passed for both `cmd/genesisd` and `cmd/genesisctl`; `git diff --check` passed; the repository scan for versioned route or kernel version labels returned no matches.
- Acceptance condition: reviewer confirms reject is an Accumulation-owned review decision persisted in the ledger, excluded from recall, and not a shell/UI state overlay.
- Residual risk: supersession is still future work. It should be added as an explicit replacement review event rather than mutating rejected candidates into approved truth. The current review transition lock protects this single-process kernel spike; future multi-process ledger writers need transactional compare-and-append semantics for the same invariant.

### KERNEL-USAGE-SUMMARY-20260622 - P1 - Final answer must project provider usage summary

- Status: ready_for_acceptance.
- Fix commits: `5efb9c01f`, `7a5364171`.
- Evidence: `TestHTTPFinalUsageSummarySurvivesSessionReplay` first failed because `final.usage` was absent, then passed after provider usage normalization and final-event persistence. The test proves an OpenAI-compatible `usage.prompt_tokens/completion_tokens/total_tokens` response becomes `usage.input_tokens/output_tokens/total_tokens` on `POST /turn` and survives restart through `GET /sessions/{id}`. `TestOpenAICompatibleProviderCompletesAgainstCompatibleServer` proves the provider adapter returns normalized usage on `ModelResponse`. `go test ./...` passed; `go build` passed for both `cmd/genesisd` and `cmd/genesisctl`; `git diff --check` passed; the repository scan for versioned route or kernel version labels returned no matches.
- Acceptance condition: reviewer confirms usage is kernel-owned final evidence produced by Model Gateway normalization and not a shell/UI-computed field.
- Residual risk: usage is emitted only when the upstream provider supplies it. Streaming partial usage, per-tool usage, cost accounting, and provider-specific detailed token breakdowns remain future inspection work.

### KERNEL-TOOL-BATCH-AUTH-20260622 - P1 - Mixed model tool-call batches must fail before any effect

- Status: ready_for_acceptance.
- Fix commits: `5754c4297`, `166fd1116`.
- Evidence: `TestSubmitTurnRejectsMixedModelToolBatchBeforeAnyEffect` proves a provider batch containing allowed `shell.exec` plus unsupported `email.send` returns `ErrModelToolCallRejected`, creates no output file, and leaves no operation projection. `TestSubmitTurnRejectsUnknownModelToolArgumentFields` proves authority-shaped unknown argument fields such as `permission_mode` are rejected before any shell effect. `TestSubmitTurnRejectsUnsupportedModelToolCall` still proves unsupported single-call batches record `turn.submitted`, `model.tool_call`, and `turn.failed` without effects. `go test ./...` passed; `go build` passed for both `cmd/genesisd` and `cmd/genesisctl`; `git diff --check` passed; the repository scan for versioned route or kernel version labels returned no matches.
- Acceptance condition: reviewer confirms the Tool System preflights each model tool-call batch as one authority decision before any effect executes.
- Residual risk: this covers the current synchronous model tool loop and canonical `shell.exec` descriptor. Future parallel tool execution or new effectful tools must reuse the same batch preflight boundary before adding concurrency.

### KERNEL-TOOL-LOOP-20260622 - P0 - Turn loop cannot execute model-requested tools

- Status: ready_for_acceptance.
- Fix commits: `209c002a8`, `d6d3ffb7e`.
- Evidence: `TestSubmitTurnExecutesOpenAICompatibleToolCallBeforeFinal` first failed against the old behavior because the OpenAI-compatible request did not include a `shell.exec` tool descriptor and the provider returned HTTP 400. After the fix, the test proves the provider can request `shell.exec`, the kernel executes it through `ToolPolicy`, sends redacted operation evidence back as a tool message, receives the final answer, and replays `turn.submitted`, `model.tool_call`, `operation.running`, `operation.completed`, and `model.final` through `GET /turns/{id}/events` after restart. `TestSubmitTurnRejectsUnsupportedModelToolCall` proves unsupported provider tools fail closed with `tool_call_rejected` and no operation effect. `go test ./...` passed; `go build` passed for both `cmd/genesisd` and `cmd/genesisctl`; `git diff --check` passed; the repository scan for versioned route or phase labels returned no matches.
- Acceptance condition: reviewer confirms model-requested tools run through the same kernel-owned Tool System authority as direct tool calls, and that provider adapters only translate tool schemas and messages.
- Residual risk: the initial model tool surface exposes only canonical `shell.exec`. Richer tools, live provider smoke for tool calls, streaming partial events, and long-running tool cancellation remain future kernel work; external applications must still remain skills, CLIs, or daemons outside the kernel.

### KERNEL-TURN-EVENTS-20260622 - P1 - Turn events need a direct observation surface

- Status: ready_for_acceptance.
- Fix commits: `0680f1a7a`, `eddad96d4`.
- Evidence: `TestHTTPTurnEventsAfterRestart` failed before implementation with HTTP 404 for `/turns/{id}/events`, then passed after implementation; `go test ./...` passed; `go build` passed for both `cmd/genesisd` and `cmd/genesisctl`; `git diff --check` passed; the repository scan for versioned route or phase labels returned no matches; live HTTP smoke submitted a turn, restarted `genesisd`, read `/turns/{id}/events`, and observed `turn.submitted` then `model.final`, with 401 for missing authorization and 404 for an unknown turn.
- Acceptance condition: reviewer confirms `GET /turns/{id}/events` is a kernel-owned observation surface for the conceptual `turn.stream` syscall, not a UI timeline owner and not a commitment to SSE/live streaming.
- Residual risk: this is a read-after-restart event list, not a live push stream. Future shells can consume it immediately, while richer streaming transports should be added only behind the same ledger-owned event truth.

### recvndQ9cGNIqE - P1 - Stale running shell operations must not trap idempotent retries

- Status: ready_for_acceptance.
- Fix commits: `9742ad13`, `d274af7f1`.
- Evidence: the new `TestExecShellStaleRunningIdempotencyKeyFailsClosedAfterRestart` and `TestHTTPShellExecStaleRunningIdempotencyKeyReturnsFailedOperation` first failed against the previous behavior because stale idempotent retries returned `status=running`; after the fix both tests passed. `go test ./...` passed; `go build` passed for both `cmd/genesisd` and `cmd/genesisctl`; `git diff --check` passed. The fixed behavior replays a stale `operation.running`, appends a terminal `operation.failed` event with `blocked_reason=stale_running_operation`, returns the same `operation_id`, and does not execute the retry command.
- Acceptance condition: reviewer confirms a crash between `operation.running` and a terminal event no longer traps idempotent `shell.exec` retries in a permanent running projection, and the kernel does not guess or repeat the effect.
- Residual risk: this is the minimal fail-closed recovery for the current short-lived `shell.exec` tool. Future long-running tools need richer lease, heartbeat, retry, cancellation, and recovery policy before they can safely resume work.

### recvndHA93jSZH - P1 - Genesis provider credential needs an executable setup path

- Status: ready_for_acceptance.
- Fix commits: `1a3fed964`, `0ad989b71`.
- Evidence: `go test ./...` passed; `go build` passed for both `cmd/genesisd` and `cmd/genesisctl`; `git diff --check` passed; the repository scan for versioned route or phase labels returned no matches; `TestSetupOpenAICompatibleProviderWritesConfigAndProtectedCredential`, `TestSetupOpenAICompatibleProviderDryRunWritesNothing`, `TestCorruptSetupCredentialBlocksProviderConfig`, `TestProviderSetupCommandDryRunDoesNotRequireAPIKey`, and `TestProviderSetupCommandWritesCredentialWithoutPrintingSecret` passed; a live local smoke wrote temp `models.json` plus a DPAPI credential record, verified `genesisctl provider-setup` output and generated files did not contain the test secret, started `genesisd` with the generated config and observed `/ready.status=ok`, then corrupted the credential and observed `/ready.status=blocked` with provider reason `provider_credential_missing`.
- Acceptance condition: reviewer confirms setup is an operator setup surface only, not a provider account flow inside runtime, and a new machine can initialize Genesis-owned model gateway config plus `secret://...` credential data without hand-writing `protected_data_b64`.
- Residual risk: real provider account creation, login, billing, quota, and upstream credential issuance remain external. This setup path only stores an already obtained API key and model gateway config for the local kernel.

### KERNEL-IDEMPOTENCY-20260622 - P0 - Duplicate tool idempotency keys must not execute effects twice

- Status: ready_for_acceptance.
- Fix commits: `d9b65933b`, `76971aef5`.
- Evidence: `go test ./...` passed; build passed; live fake-provider HTTP smoke returned the same `operation_id` for two `/tools/shell.exec` requests with the same `idempotency_key`, preserved file content as `first`, and projected `operation_count=1` plus `event_count=2`; `TestExecShellIdempotencyKeySurvivesRestartWithoutRepeatingEffect` proves restart-safe replay does not execute the second command; `TestExecShellBlockedOperationIsIdempotent` proves blocked operations are idempotent; `TestExecShellRejectsInvalidIdempotencyKey` proves invalid key shapes fail before ledger append; `TestHTTPShellExecIdempotencyKeyReturnsExistingOperation` proves the HTTP transport uses the same kernel behavior.
- Acceptance condition: reviewer confirms `idempotency_key` is a kernel control-plane field and duplicate `session_id + tool + idempotency_key` retries return the existing operation without re-executing effects.
- Residual risk: idempotency is currently implemented for `shell.exec`, the only effectful tool in the spike. Future effectful tools must reuse the same ledger-backed boundary before execution.

### recvnd2PDI1LuV - P0 - Minimal Go single-binary spike

- Status: ready_for_acceptance.
- Fix commits: `559e1c0c7`, `fd5bf9d8a`, `db9aeca13`, `22d5ca9f4`, `a9b34bda7`, `25e292b81`.
- Evidence: `go test ./...` passed; build passed; `GENESIS_LIVE_PROVIDER=1 go test ./internal/kernel -run TestLiveOpenAICompatibleProviderThroughKernel -count=1 -v` passed using Genesis `~/.genesis/config/models.json` and local `secret://...` credential resolution; binary `/ready` smoke returned `provider=openai-compatible` and `status=ok`; repository version-label scan returned no matches.
- Acceptance condition: reviewer confirms the spike proves a single Go binary with unversioned `/ready`, `/turn`, `/sessions/{id}`, fake provider mode, OpenAI-compatible provider mode, restart-safe ledger replay, and Genesis-owned live provider config.
- Residual risk: this is still a kernel spike, not a full product shell. Streaming, richer tool loop continuation, duplicate idempotency handling, and long-term storage policy remain future kernel work.

### recvndJWPu1RcN - P0 - Ingress security must not hard-reject ordinary user text

- Status: ready_for_acceptance.
- Fix commit: `330836d7b`.
- Evidence: `go test ./...` passed; build passed; live fake-provider HTTP smoke returned 200 for risk-marker text with 2 `ingress_risks`, 403 `turn_blocked_by_ingress_security` for hidden control text, and 400 `invalid_request` for nested `role`; `TestSubmitTurnRecordsIngressRiskWithoutBlocking` proves prompt-injection samples are accepted as user data and recorded as risk metadata; `TestHTTPAcceptsRiskyUserDataAndRecordsMetadata` proves `System:` log headings and `tool_call_id` / `function_call` fragments do not block `/turn`; `TestHTTPRejectsNestedControlFieldBeforeAdmission` proves malformed nested control fields still return 400 before ledger append; `TestHTTPBlocksInvisibleIngressMarker` proves hidden control text still returns 403 before ledger append.
- Acceptance condition: reviewer confirms the kernel separates data from authority: risky text is metadata, while control-plane forgery or hidden text fails closed.
- Residual risk: risk metadata is recorded in session projection only. Richer downstream isolation policy can be added later, but it must not make prompt text itself an authority boundary.

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
