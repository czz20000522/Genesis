# Design: Context Discovery And Accumulation

## Requirement

`docs/requirements/kernel-context-discovery-and-accumulation.md`

Status: approved for phased implementation.

## Boundary And Owner

This design separates three owners:

- Model Gateway owns provider context projection and decides which discovered
  fragments may enter a provider request.
- Accumulation owner owns reviewed long-term semantic entries and their
  lifecycle.
- User-space capability runtime owns capability manifests, runners, doctor
  checks, and capability-local data.

The discovery layer is a bounded query/projection path across those owners. It
does not own execution authority, project truth, connector delivery, tool
results, memory review decisions, or provider-native payloads.

## Data Flow

```text
provider context
  -> fixed small discovery surface
  -> model asks for relevant context/capabilities
  -> discovery resolver searches safe descriptors and active accumulation
  -> resolver returns bounded candidate summaries
  -> model may request a separate owner action
  -> ToolGateway / Resource owner / Capability runtime / Model Gateway
     re-checks authority before any effect or context injection
```

For accumulation creation:

```text
interaction, review, project reading, capability metadata
  -> candidate proposal with source refs
  -> review / reject / supersede / forget
  -> active scoped entry
  -> discovery projection
```

For conflicts:

```text
global accumulation entry
  -> project/task contract conflict detected
  -> result includes conflict notice or is suppressed
  -> local contract wins
  -> global entry remains unchanged
```

## Protocol

### Accumulation Entry

The owner records scoped semantic assertions, not raw documents.

```text
AccumulationEntry
  id
  kind: preference | heuristic | method | lesson | project_overlay |
        capability_hint | memory_fact
  claim
  scope: global | project | workspace | capability
  applies_when
  yields_to
  strength: weak_hint | preference | strong_rule | contract_hint
  source_refs
  status: candidate | active | rejected | superseded | forgotten
```

`claim`, `applies_when`, and `yields_to` are semantic fields. System ids,
timestamps, owner refs, review authority, and status transitions are generated
by the owner.

### Discovery Query

The model-visible query surface stays small. It asks for candidates, not
execution.

```text
DiscoveryQuery
  intent
  current_context_summary
  requested_kinds?
  limit?
```

The kernel may add non-model-visible scope fields from the active session,
workspace, project, and task. The model cannot provide authority fields,
credential refs, approval ids, sandbox profiles, or provider routes.

### Discovery Result

```text
DiscoveryResult
  candidates[]

DiscoveryCandidate
  ref
  kind
  summary
  scope
  applies_when
  confidence: low | medium | high
  conflict_notice?
  source_summary?
```

`ref` is a candidate reference for inspection or a later owner request. It is
not permission. It cannot be passed to arbitrary tools as a storage ref or host
path.

### Capability Descriptor

Capability descriptors are searchable projections of capability packages.

```text
CapabilityDescriptor
  capability_ref
  name
  summary
  intents
  input_summary
  health_summary?
```

The descriptor is not manifest truth. The package manifest under
`~/.genesis/capabilities/<id>` remains the capability runtime's source of truth.

## Failure Semantics

- Invalid discovery query: refused before lookup with a structured reason.
- Discovery index unavailable: return not-ready or empty-result feedback; do
  not fabricate candidates.
- Candidate conflicts with current contract: suppress it or return a conflict
  notice; do not mutate global accumulation.
- Candidate selected for execution: route through the appropriate owner and
  re-check admission there.
- Candidate selected for provider context: Model Gateway performs scope,
  budget, source, and egress checks before injection.

Discovery failures are not provider failures unless the model cannot continue
without the result and the turn owner chooses to fail. Ordinary no-match results
are model-visible observations.

## Permission And Authority

Discovery is read-oriented and bounded, but it can still leak sensitive context
if implemented loosely. The resolver must:

- expose only safe summaries and public refs;
- respect project/workspace/session scope;
- hide credential refs, internal storage refs, host paths, and audit refs;
- reject model-supplied control fields;
- avoid returning full skill bodies, full project docs, or full manifests by
  default.

Execution authority remains with the target owner. A discovered capability does
not grant shell access, connector access, provider route access, workspace
write access, or credential access.

## Recovery And Observability

Accumulation lifecycle events are durable owner facts. Discovery diagnostics are
not durable by default.

Durable:

- candidate proposed;
- candidate approved;
- candidate rejected;
- candidate superseded;
- candidate forgotten;
- capability descriptor publication when it changes discovery truth.

Not durable by default:

- retrieval scoring internals;
- BM25/embedding intermediate results;
- failed no-match query traces;
- full candidate lists used only for ranking;
- model-visible discovery deltas beyond the settled tool/result observation.

Context inspection may show that discovery was used, the kinds returned, and
the refs included in provider context. Debug trace may include bounded ranking
diagnostics when explicitly enabled.

## Storage Shape

The first store should be owner-owned and small:

- accumulation owner stores reviewed entries and lifecycle facts;
- capability runtime stores manifests and local package data;
- discovery index stores rebuildable searchable projections.

The index is not canonical truth. It can be rebuilt from accumulation entries,
capability manifests, and project overlay refs. Large bodies stay in their
owning files, resource objects, or project repositories.

## Rejected Alternatives

- Always injecting every capability schema into provider context is rejected
  because it scales linearly with installed capabilities and wastes context.
- Reintroducing automatic approved-memory recall is rejected because it repeats
  the retired coupling between review, search, egress, and provider context.
- Treating accumulation as a project documentation repository is rejected
  because project docs remain the project truth.
- Letting discovery results execute capabilities directly is rejected because
  discovery is not authority.
- Training or fine-tuning local models from accumulation in this requirement is
  rejected because parameter updates are harder to inspect, delete, and roll
  back than reviewed accumulation entries.
