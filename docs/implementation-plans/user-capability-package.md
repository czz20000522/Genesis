# User Capability Package Implementation Plan

- **Requirement:** `docs/requirements/user-capability-package.md`.
- **Design:** `docs/applications/user-capability-package-design.md`.
- **Issue:** `APP-USER-CAPABILITY-PACKAGES-20260711`.

## Reference Scan

Codex `core-skills/src/service.rs` owns filesystem skill discovery and
immutable snapshots, while its skill render path projects locators rather than
turn authority. Reasonix's `internal/inspect/inspect.go` aggregates safe
capability surfaces for its frontend. Genesis aligns with the safe-projection
split but intentionally keeps entrypoint execution outside the kernel and does
not add installation behavior.

## Phase A: Shared Inspector

Move the manifest type, safe path validation, health inspection, and package
command construction from `cmd/genesisctl` into one reusable user-space package.
Lock list/doctor/run behavior with existing CLI tests plus new package tests.

## Phase B: Kernel Descriptor Wiring

Have `genesisd` load descriptors from the configured capability root at startup
and pass only safe descriptors to `kernel.Config.CapabilityDescriptors`. Add
tests for healthy siblings, malformed isolation, and no path/entrypoint leak.

## Phase C: Real Package Evidence (deferred)

There is no real capability package available now. Video transcript extraction
remains a shared Skill or future Workflow, and no report-generation capability
exists. Do not create a synthetic package simply for acceptance. When a real
user-owned package is selected, prove list/doctor/run and daemon discovery; add
a second distinct package only after it exists as real work.
