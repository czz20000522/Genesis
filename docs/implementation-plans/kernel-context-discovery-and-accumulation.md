# Context Discovery And Accumulation Implementation Plan

> For agentic workers: implement this plan task-by-task. Keep each phase small, test-first, and aligned with the requirement and design. Do not restore automatic memory recall.

**Goal:** Build a low-context discovery and accumulation path without letting long-term memory rewrite provider context or bypass kernel authority.

**Architecture:** Phase 1 reuses the existing memory candidate/review owner as the first accumulation owner surface. Later phases can add bounded discovery queries and capability descriptors, but only after this owner lifecycle is stable.

**Tech Stack:** Go kernel package, existing SQLite ledger, existing `/memory/candidates` control surface, existing session projection.

---

## Reference Scan

Codex reference:

- `D:\software\JetBrains\python_workspace\codex-main\AGENTS.md` defines model-visible context rules: no history rewrite, avoid cache misses, hard caps, and typed context fragments.
- `D:\software\JetBrains\python_workspace\codex-main\codex-rs\core\tests\suite\model_visible_layout.rs` snapshots the exact model-visible layout, which treats context projection shape as a testable contract.

Genesis alignment:

- Accumulation entries must not silently enter provider context.
- Any future discovery result must be bounded, typed, and test-covered as a projection.

Reasonix reference:

- `D:\software\JetBrains\python_workspace\reasonix\internal\memory\memory.go` discovers persistent memory files and saved memories once at boot, then composes them after the base prompt for cache stability.
- `D:\software\JetBrains\python_workspace\reasonix\internal\memory\memory_test.go` tests empty memory as byte-for-byte identity and tests memory ordering after the base prompt.
- `D:\software\JetBrains\python_workspace\reasonix\internal\boot\boot.go` wires `remember` and `forget` tools to a memory store.

Genesis intentional difference:

- Reasonix injects memory into the system prompt as a product choice. Genesis Phase 1 does not inject accumulation into provider context because the discovery consumer, budget, and conflict policy are not implemented yet.

## Reference Behavior Red Tests

- Codex-aligned: approved accumulation metadata must persist without entering provider context automatically.
- Codex-aligned: future discovery/context fragments must be typed and bounded before they can become model-visible.
- Reasonix-aligned: forgetting must be an explicit owner action and must be replay-stable.
- Genesis-specific: a forgotten candidate is terminal; a replay stream that approves after forget is competing review evidence.
- Genesis-specific: semantic fields such as `applies_when` and `yields_to` are user meaning, not kernel control refs.

## Phase 0: Governance Baseline

- Requirement and design are approved for phased implementation.
- This plan is the active implementation plan for Phase 1.
- Out of scope for Phase 1: vector search, auto recall, provider context injection, fine-tuning, model training, and a new discovery tool.

## Phase 1: Reviewed Accumulation Skeleton

Files:

- Modify `internal/kernel/memory_types.go`.
- Modify `internal/kernel/memory.go`.
- Modify `internal/kernel/http_memory.go`.
- Modify `internal/kernel/http.go`.
- Modify `internal/kernel/session_projection.go`.
- Modify `internal/kernel/architecture_boundary_test.go`.
- Modify `internal/kernel/kernel_test_helpers_test.go`.
- Modify `internal/kernel/memory_review_test.go`.

Behavior:

- Keep the existing `/memory/candidates` API as the Phase 1 accumulation owner surface.
- Add semantic metadata to candidates: `kind`, `scope`, `applies_when`, `yields_to`, and `strength`.
- Normalize missing metadata to safe defaults.
- Reject unknown `kind`, `scope`, or `strength`.
- Treat `claim`-like fields as semantic text; do not classify them as control refs or authority ids.
- Add a `forgotten` terminal status and `POST /memory/candidates/{id}/forget`.
- `forgotten` candidates remain auditable but stop appearing as approved or pending candidates.
- `forgotten` candidates cannot later be approved, rejected, or superseded.
- Existing approved candidates still do not auto-recall into provider context.

Tests:

- Metadata is persisted, replayed, and projected.
- Unknown metadata enums fail closed.
- Semantic fields can contain ordinary user text and are not treated as control ids.
- Forgetting is durable, idempotent, and terminal.
- A replay stream that tries to approve after forgetting is rejected as competing review evidence.
- Session projection includes forgotten entries as owner facts without putting them in provider context.

## Future Phases

Phase 2:

- Add a bounded discovery query projection that returns candidate summaries, not authority.
- Add negative tests proving discovery results do not grant tool, connector, resource, or provider-context authority.

Phase 3:

- Add capability descriptors from user-space capability packages.
- Keep capability manifests and runner truth in the capability runtime, not the kernel.

Phase 4:

- Consider automatic provider-context activation only after a separate requirement defines the consumer, budget, egress policy, conflict handling, and negative tests.
