# Kernel Issue Ledger

This file is the repo-owned ledger for active Genesis Kernel issues. Feishu Base is the collaboration inbox; this file is the durable project source for issues that still need code, verification, or user acceptance.

Retired issues must not remain here. Move accepted retirements to `docs/operations/kernel-retirement-log.md` with the fixing commits, verification evidence, residual risks, and retirement reason.

## Ledger Rules

- Keep only `new`, `open`, `in_progress`, or otherwise active issues in this file.
- Do not record application-specific feature work as kernel work unless it changes a kernel primitive.
- Do not add versioned HTTP route names as current contracts. HTTP is transport; current kernel endpoints are unversioned.
- `ready_for_acceptance` issues move to the retirement log as retirement candidates and leave this active ledger.
- Feishu/Base links may point to collaboration artifacts, but this repo must contain enough evidence to understand the current status without opening Feishu.
- Every active `KERNEL-*` issue must include a `Reference alignment` field that compares the issue to Codex, Reasonix, or an explicitly rejected drift risk.
- When an issue removes a concept from the current kernel contract, long-term tests must assert the positive replacement behavior. Do not keep permanent tests whose only purpose is locking retired names, aliases, routes, or historical helper APIs out of the tree; use temporary scans or retirement-log evidence for cleanup windows, then fold the guard into the current owner contract.

## Active Issues

### KERNEL-UI-TIMELINE-PROJECTION-20260622 - P1 - WebUI needs a dedicated timeline projection instead of raw events

- Status: new.
- Type: architecture issue.
- Problem: Genesis now exposes typed raw events, `/turns/{id}/events`, and `/sessions/{id}`, but there is no stable user-facing timeline projection. If WebUI renders raw events directly, it will duplicate tool-call/result merging logic and risks showing audit, checkpoint, session-completion, or kernel-owned fields as chat items.
- Suggestion: Add an owner-level `UiTimelineProjection` that consumes the kernel event log and outputs stable UI items such as user message, assistant reasoning/process card, tool card, assistant final message, and notices. `tool.call` and `tool.result` must merge through `tool_result.for_event_id`. Checkpoint and session-completion events must stay out of the main chat timeline.
- Evidence: Current kernel has raw event/session projections but no `UiTimelineProjection` or stable item union for WebUI consumption.
- Validation: A fixture containing `user.message`, `assistant.reasoning.delta`, `tool.call`, `tool.result`, `assistant.message.completed`, `checkpoint.created`, and `session.completed` projects to user bubble, reasoning/process card, one merged tool card, and assistant reply. Main timeline omits checkpoint/session-completed and hides audit ids; diagnostics projection can still expose raw facts.
- Reference alignment: Reasonix separates event stream facts from display items and renders tool output through a UI-specific `ToolCard`. Genesis should keep that separation so WebUI remains a shell, not a second event owner.

### KERNEL-CONTEXT-INSPECTION-PROJECTION-20260622 - P1 - Need inspectable runtime context separate from chat timeline

- Status: new.
- Type: user feedback.
- Problem: The kernel can assemble provider-visible input items, tool manifest, skill catalog summaries, and memory recall, but users have no per-turn inspection projection showing what the model actually received. `/capabilities` is global state and raw events only record coarse input kinds, so a future WebUI would be tempted to put runtime context into the main chat timeline.
- Suggestion: Add a `ContextInspectionProjection` keyed by turn/session. It should expose non-sensitive per-turn context snapshot data: input items, model-visible tool manifest, skill summaries, memory recall source refs, gateway profile/model, and permission/sandbox summary. It must stay in diagnostics/inspection UI, not the main timeline.
- Evidence: `internal/kernel/model_context.go` builds model input, `internal/kernel/provider.go` defines `ModelRequest`, and `internal/kernel/http.go` exposes readiness/capabilities/events/sessions without a per-turn context inspection surface.
- Validation: A turn with approved memory, skill catalog, and tool manifest can produce inspection data after restart or explicitly report snapshot unavailable. The main timeline omits these context blocks. API keys, authorization headers, credential raw values, and unredacted secrets never appear.
- Reference alignment: Reasonix keeps controller status/context separate from transcript items. Genesis should mirror the control-plane split: chat timeline for user reading, context inspection for debug/understanding, raw ledger for audit/recovery.
