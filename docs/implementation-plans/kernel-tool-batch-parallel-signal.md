# Implementation Plan: Tool Batch Parallel Signal

- **Issue:** `KERNEL-TOOL-BATCH-PARALLEL-SIGNAL-20260625`
- **Requirement:** `docs/requirements/kernel-foundation-capabilities.md`
- **Design:** `docs/design/kernel-foundation-capabilities.md`
- **Owner:** Genesis Kernel / Tool Runtime

## Goal

Make `ToolExecutionBatch.Parallel` mean the current executor can actually run
the batch concurrently. Compatibility grouping without concurrent execution must
not be projected as parallel work.

## Reference Scan

Reasonix:

- `internal/agent/agent.go` partitions provider-ordered tool calls so only
  contiguous known read-only tools receive `parallel=true`; writer and unknown
  tools remain serial segments.
- `internal/agent/guards_test.go` proves the read-only batch is actually
  concurrent by wall-clock behavior, and separately proves writer/state-like
  tools split read-only segments.
- `internal/tool/tool.go` documents that a batch is parallelized only when every
  call is `ReadOnly`; bash and plugin tools stay conservative when static
  effects are not known.

Codex:

- `codex-rs/codex-mcp/src/tools.rs` keeps `supports_parallel_tool_calls` as
  raw execution metadata for routing and diagnostics. It is not a model-authored
  claim and does not replace runtime support.

Genesis translation:

- `pure_read` batches with trusted compatible locks may keep `Parallel=true`
  because the current executor has a concurrent runner for them.
- `process_io` batches may still group different handles for deterministic
  planning, but they must keep `Parallel=false` until a real process-IO
  concurrent executor and replay contract exists.

## Reference Behavior Red Tests

- Update the existing process-IO planner test so a grouped different-handle
  batch expects `Parallel=false`.
- Add a planner/executor eligibility test proving a batch reports
  `Parallel=true` only when `canExecuteToolBatchConcurrently` is true in the
  current implementation.
- Keep the execution test proving process-IO remains serial and align its
  batch assertion with the new signal semantics.

## Implementation Steps

1. Watch the new/updated tests fail against the existing planner.
2. Change the planner flush path so `Parallel` is true only for multi-call
   `pure_read` batches.
3. Keep process-IO grouping intact but non-parallel.
4. Update requirement/design wording so future projections cannot reinterpret
   `Parallel` as a grouping hint.
5. Retire the issue with compact evidence after focused and broad verification.

## Drift Check

- `ToolExecutionBatch.Parallel` is an execution fact, not a compatibility hint.
- `shell_exec` remains serial and is not inferred pure-read from command text.
- Process/job control is not made concurrent in this slice.
- Provider-visible result ordering remains unchanged.
