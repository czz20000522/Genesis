# Implementation Plan: Kernel Resource Read

## Phase A - Contract

- Add production requirement and design for resource refs, `resource_read`, and
  pure-read scheduling.
- Open the implementation issue that cites the requirement/design.

## Phase B - Minimal Immutable Text Registry

- Add `ResourceDescriptor` / `ResourceRegistry` types under the kernel owner.
- Add optional configured immutable text resources to `Config`.
- Register `resource_read` with a trusted `pure_read` access plan when invoked
  against a known immutable resource ref.
- Return bounded text, truncation metadata, original size, returned size, and
  next offset.
- Reject unknown refs, invalid offsets/limits, and non-text resources before
  any body read.

## Phase C - Scheduling Proof

- Update tool scheduling tests so `resource_read` is the first default
  non-shell pure-read candidate.
- Prove two compatible resource reads can be planned together and that a write
  fence still prevents read/write reordering.
- Keep execution serial until the executor-pool phase. Planning eligibility and
  real goroutine execution are separate milestones.

## Phase D - Owner Store Proposal

- Stop before connectors depend on durable resources.
- Add a resource owner store proposal for refs, metadata, body refs, grants,
  TTL/retention, and recovery if applications need persistent intake.

## Verification

- `go test ./internal/kernel -run 'TestResource|TestPlanToolExecutionBatches|TestToolScheduling' -count=1`
- `go test ./internal/kernel -count=1`
- `go test ./... -count=1` before retiring the implementation issue.
