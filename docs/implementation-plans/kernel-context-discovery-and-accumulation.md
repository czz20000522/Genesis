# Context Discovery And Accumulation Implementation Plan

> For agentic workers: implement this plan task-by-task. Keep each phase small, test-first, and aligned with the requirement and design. Do not restore automatic memory recall.

**Goal:** Build a low-context discovery and accumulation path without letting long-term memory rewrite provider context or bypass kernel authority.

**Architecture:** Phase 1 reuses the existing memory candidate/review owner as the first accumulation owner surface. Phase 2/3 add a bounded discovery surface over approved accumulation and skill catalog descriptors. Discovery remains a projection and never grants authority.

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

## Phase 2: Bounded Discovery Query

Files:

- Add `internal/kernel/discovery_types.go`.
- Add `internal/kernel/discovery.go`.
- Add `internal/kernel/http_discovery.go`.
- Modify `internal/kernel/config_types.go`.
- Modify `internal/kernel/kernel.go`.
- Modify `internal/kernel/http.go`.
- Modify `internal/kernel/tool_registry.go`.
- Modify `internal/kernel/model_tools.go`.
- Modify `internal/kernel/tool_scheduling.go`.
- Modify `internal/kernel/architecture_boundary_test.go`.
- Add `internal/kernel/discovery_test.go`.

Behavior:

- Add `POST /discovery/query` for operator/application inspection.
- Add the model-visible `context_discover` kernel-control tool.
- Return bounded candidate summaries from approved accumulation only.
- Exclude pending, rejected, superseded, and forgotten candidates from active discovery.
- Reject unknown request fields and unsupported requested kinds.
- Keep discovery as `state_read`; it is not a parallel pure-read primitive because it reads current owner facts.
- Discovery results are hints only. They do not grant tool, connector, resource, provider-context, approval, or credential authority.

Tests:

- Approved accumulation can be discovered by semantic query.
- Non-approved terminal or pending candidates do not appear.
- Discovery does not auto-inject accumulation into provider context.
- HTTP unknown control fields are rejected.
- `context_discover` returns bounded hint results through the normal tool loop without exposing control fields.

## Phase 3: Capability Descriptor Discovery

Files:

- Add `CapabilityDescriptor` as a safe descriptor input from user-space capability runtime.
- Reuse `internal/kernel/skill_catalog.go` as an additional metadata-only capability hint source.
- Reuse `internal/kernel/capabilities.go`.
- Reuse `internal/kernel/discovery.go`.
- Extend `internal/kernel/discovery_test.go`.

Behavior:

- Treat user-space capability runtime descriptors as capability discovery candidates.
- Treat skill catalog metadata as an additional capability hint source.
- Return only skill name, short summary, capability ref, scope, confidence, and source summary.
- Do not project skill root paths, instruction paths, skill bodies, manifests, manifest paths, entrypoints, or runner details.
- Keep manifest truth and runner truth outside the kernel.

Tests:

- User-space capability descriptors can be discovered by intent.
- Capability refs reject host-path-shaped values.
- Capability descriptor discovery returns a matching skill by name/description.
- Discovery output does not leak root path, skill file path, skill body, manifest path, entrypoint, or runner truth.

## Phase 4: Activation Gate

- Automatic provider-context activation remains out of scope.
- Do not add automatic recall, automatic context injection, or discovery-to-provider promotion in this plan.
- A future activation requirement must define the consumer, budget, egress policy, conflict handling, override order, debug evidence, and negative tests before implementation.
