# Kernel Retirement Log

This file records Genesis Kernel issues that are ready for acceptance or retired. It is the repo-owned companion to `docs/operations/kernel-issues.md`.

## Retirement Rules

- `ready_for_acceptance` means the code and verification evidence are ready for user or operator acceptance, but the issue is not fully retired yet.
- `retired` means the user or operator accepted the evidence. A retired issue must be absent from `kernel-issues.md`.
- Every entry must include the issue id, title, fixing commits, verification evidence, residual risk, and retirement reason or retirement condition.
- Every `KERNEL-BOUNDARY-*` entry must retain its `Reference alignment` field when moved from the active ledger.
- If an entry is reopened, move it back to `kernel-issues.md` and mark this log entry as reopened with the reason.

## Ready For Acceptance

### KERNEL-TOOL-RESULT-TAXONOMY-20260622 - P0 - Tool loop must preserve terminal-equivalent command results

- Status: ready_for_acceptance.
- Fix commits: `0c7960172`.
- Evidence: `TestSubmitTurnReturnsRepairFeedbackForInvalidShellArguments` first failed because malformed `shell.exec` arguments terminated the turn as `model tool call rejected`, then passed after the Tool System began returning structured `tool_request_invalid` repair feedback to the model without executing an operation. `TestSubmitTurnRejectsInvalidToolCallIDBeforeToolCallEvent` proves an unrecoverable bad `tool_call_id` fails before `model.tool_call` evidence is appended. `TestSubmitTurnReturnsRepairFeedbackForMixedModelToolBatchBeforeAnyEffect`, `TestSubmitTurnReturnsRepairFeedbackForUnknownModelToolArgumentFields`, `TestSubmitTurnReturnsRepairFeedbackForUnknownSkillReadBeforeShellEffect`, and `TestSubmitTurnReturnsRepairFeedbackForChangedSkillReadBatchBeforeShellEffect` prove invalid mixed batches create no shell effects while returning repair results for each call. `TestSubmitTurnFeedsNonZeroShellExitToModel` proves an executed shell command with nonzero exit returns model-visible `status=failed`, `exit_code`, and stderr while omitting ledger control-plane handles from the model-facing tool result. `TestExecShellReportsHeadTailTruncationMetadata` proves long stdout/stderr are bounded with head/tail content, truncation flags, original byte counts, omitted byte counts, and `output_truncation=head_tail`. `TestSubmitTurnReportsToolInfrastructureFailureSeparately` proves ledger/tool infrastructure failure records `tool_infrastructure_failed` rather than a command stderr. `TestSubmitTurnSkillReadUnavailableRepairDoesNotExposeInstructionPath` and `TestExecShellControlledReadFailureDoesNotExposeAbsolutePath` were added after security review to prove internal file paths are not surfaced through repair messages or controlled-shell synthetic stderr. Independent architecture review found model-visible repair content duplicated `tool_call_id` and that docs incorrectly classified provider adapter errors as Tool System infrastructure; both were fixed. A brief over-sanitizing attempt for unknown tool names was rejected after user review because it diverged from Codex-style repair feedback and local terminal semantics. Verification passed: `go test ./internal/kernel -count=1`; `go test ./... -count=1`; `go build ./...`; `go test -race ./internal/kernel -count=1`; `git diff --check`; the repository scan for numbered route prefixes returned no matches.
- Acceptance condition: reviewer confirms model tool results follow Codex-style recoverable feedback: invalid requests with valid protocol handles return model-repairable evidence without effects, command exits remain terminal-equivalent command evidence, permission blocks remain operation blockers, and tool infrastructure failures are separate from command stderr. Reviewer also confirms model-facing tool result content excludes operation/session/turn/idempotency handles while authorized session/API projections retain ledger evidence.
- Residual risk: default-mode controlled shell commands synthesize a small path-free stderr for internal read/write failures instead of reproducing a native shell's full diagnostic. Yolo-mode shell execution still preserves real shell stdout/stderr under the head/tail cap. Provider failures remain Model Gateway failures and are intentionally outside this Tool System taxonomy.

### KERNEL-CAPABILITIES-20260622 - P1 - Shells and daemons need a protected kernel capability projection

- Status: ready_for_acceptance.
- Fix commits: `2654f0877`, `65f004277`.
- Evidence: `TestHTTPCapabilitiesRequiresRuntimeAuth` first failed because `/capabilities` returned 404, then passed after implementation and proves the route requires the runtime bearer token. `TestHTTPCapabilitiesProjectsToolsAndSkillCatalogWithoutPaths` proves authenticated inspection returns canonical `shell.exec` and `skill.read` capability names plus safe skill name/description metadata, while excluding `instruction_path`, `ledger_path`, filesystem paths, and skill bodies. `TestHTTPCapabilitiesReportsPathFreeSkillExclusions` proves missing roots, malformed metadata, unsafe metadata, linked paths, and duplicate skill names are projected only as path-free reason/count diagnostics. `TestHTTPCapabilitiesSanitizesProviderInspectionStatus` and `TestHTTPCapabilitiesSanitizesCredentialShapedProviderTokens` were added after security review found provider readiness strings could leak secret-shaped single tokens; they now prove path-shaped provider names, `secret://...`, `Authorization: Bearer ...`, bare `sk-...`, and embedded `sk-proj-...` tokens are replaced with safe inspection fallbacks. `TestToolCapabilityKindDefaultsUnknown` proves future tools are not silently classified as effectful capabilities unless explicitly mapped. `go test ./internal/kernel -run "TestHTTPCapabilities|TestToolCapabilityKindDefaultsUnknown" -count=1` passed; `go test ./... -count=1` passed; `go build ./...` passed; `go test -race ./internal/kernel -count=1` passed; `git diff --check` passed; the repository scan for numbered route prefixes returned no matches. Independent architecture review reported no kernel/app boundary blocker. Independent security review initially found the provider inspection sanitizer leak; after the credential-shaped token fix it reported no blocking findings.
- Acceptance condition: reviewer confirms `GET /capabilities` is a protected Readiness/Inspection surface derived from kernel-owned provider/runtime/ledger readiness, canonical tool descriptors, and safe skill catalog metadata; it is not an app registry, Feishu/email/WebUI adapter, or second owner for external outbound communication.
- Residual risk: provider inspection reasons are guarded by shape and credential detection rather than a dedicated enum type. Current production provider reasons are fixed `provider_*` codes; future providers must not return arbitrary raw error text through `ProviderStatus` without updating this contract.

### KERNEL-SKILL-READ-20260622 - P0 - Model loop needs governed skill instruction retrieval

- Status: ready_for_acceptance.
- Fix commits: `ff92814db`, `f89f55409`.
- Evidence: `TestSubmitTurnReadsConfiguredSkillBeforeFinal` first failed because the model tool loop rejected `skill.read` as unsupported, then passed after implementation. It proves a provider sees the `skill.read` descriptor, requests a configured skill by catalog name, receives bounded redacted user-space instructions as tool evidence, and then returns the final answer. `TestSubmitTurnRejectsUnknownSkillReadBeforeShellEffect` proves an unknown skill read in the same batch as an otherwise valid `shell.exec` rejects the whole batch before any shell operation is recorded or any output file is created. `TestSubmitTurnRejectsSkillReadPathArgument` proves path-shaped arguments are rejected by strict tool argument decoding. `TestSubmitTurnRejectsSkillReadWhenInstructionFileNoLongerMatchesCatalog` proves a changed instruction file whose front matter no longer matches the startup catalog is rejected. Review found that execution-time `skill.read` failures could still occur after an earlier shell effect in the same batch; `TestSubmitTurnRejectsChangedSkillReadBatchBeforeShellEffect` first failed with a created file, then passed after full `skill.read` projection preparation moved into batch preflight. Review also found duplicate skill names and path exposure; `TestSkillCatalogRejectsDuplicateNames`, `TestSkillCatalogRejectsLinkedSkillDirectories`, and the updated `TestKernelInjectsSkillCatalogBeforeProviderWithoutSkillBodies` prove duplicate names are excluded, linked skill directories are excluded, and filesystem paths no longer enter model context or `skill.read` results. `go test ./... -count=1` passed; `go test -race ./internal/kernel -count=1` passed; `go build ./...` passed; `git diff --check` passed; the repository scan for numbered route or kernel version labels returned no matches; the `instruction_path` scan showed only the negative assertion that `skill.read` must not expose it.
- Acceptance condition: reviewer confirms `skill.read` is a generic read-only Tool System primitive for user-space skill instructions, not a Feishu, email, calendar, document, or channel adapter; the model uses skill names rather than filesystem paths; and every model tool-call batch is fully preflighted before any effectful tool executes.
- Residual risk: skill bodies remain user-space instructions, not signed or kernel-authoritative policy. They are bounded, hidden-control rejected, metadata-revalidated, and secret-redacted, but live skill reload, signed trust, ranking, and richer skill package governance remain separate contracts.

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
