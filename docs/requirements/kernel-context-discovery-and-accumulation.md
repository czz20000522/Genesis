# Requirement: Context Discovery And Accumulation

- **Status:** draft.
- **Owner:** Genesis Kernel, Model Gateway, and Accumulation owner.
- **Scope:** low-context discovery for user capabilities, scoped user knowledge, project overlays, and long-term accumulation.

## Background

Genesis will accumulate more user-space capabilities, skills, project rules, and
long-term user preferences than can fit in every provider request. If every
capability schema, skill body, project note, and memory fact is always injected
into provider context, Genesis will waste context budget and blur three
separate concerns:

- discovery of relevant capabilities or knowledge;
- execution authority for tools and effects;
- long-term accumulation truth.

The retired automatic memory recall path proved this risk. It coupled review
truth, search policy, egress policy, and provider-context injection before
Genesis had a clear consumer. Genesis still needs long-term learning, but that
learning must remain scoped, reviewable, and subordinate to current user,
project, and task contracts.

This requirement defines a low-context discovery layer. The model can discover
candidate capabilities, user rules, project overlays, and accumulated lessons
when needed, but discovery results do not grant authority and do not directly
rewrite provider context, tool manifests, or memory truth.

## Production Target

Genesis provides a small, stable discovery surface that lets the model ask for
relevant user-space capabilities, scoped rules, project overlays, and long-term
accumulation entries without injecting the full registry into every provider
request.

Production behavior:

- always-present provider context stays small and kernel-owned;
- extensible capabilities and accumulated knowledge are discovered on demand;
- accumulation stores scoped semantic assertions, not project documentation
  copies or raw session logs;
- current user instructions, project contracts, and task constraints override
  global accumulation;
- discovery returns candidates and explanations, not execution authority;
- execution still goes through ToolGateway, Authority Plane, resource admission,
  connector/outbox, or another approved owner path;
- automatic recall into provider context remains out of scope until a separate
  requirement names the consumer, budget, egress policy, and negative tests.

## Users And Roles

Ordinary user:

- can build long-term preferences, habits, methods, and reusable lessons over
  many sessions and projects;
- can review, approve, supersede, or forget accumulation entries;
- can see why a discovered rule or capability was suggested.

LLM:

- can ask for relevant capabilities, rules, lessons, or project overlays through
  a small model-visible discovery surface;
- receives bounded candidate summaries and applicability notes;
- cannot treat discovery candidates as permission, credential access, tool
  authority, project truth, or provider context truth.

Kernel:

- owns admission, provider-context projection, tool authority, session facts,
  memory/accumulation review facts, and conflict precedence;
- rejects model-supplied control fields and authority fields.

User-space capability runtime:

- owns capability package manifests, doctor checks, local runner details, and
  capability-local data;
- may publish safe searchable descriptors to the discovery index;
- cannot write kernel ledger facts, memory truth, tool results, or provider
  context directly.

Project repository:

- owns its requirement, design, implementation, and source-code truth;
- may be referenced by accumulation through source refs or overlays;
- is not copied into accumulation as a second documentation repository.

## Core Semantics

### Discovery Is Not Authority

Discovery answers "what might be relevant?" It does not answer "what is allowed
to execute?"

A discovered capability, rule, or memory entry is only a candidate. Any later
effect must still pass the owner that controls that effect:

- ToolGateway for model-requested tools;
- Resource owner for resource reads or hydration;
- Capability runtime for user-space capability execution;
- Connector/outbox owner for external delivery;
- Model Gateway for provider context assembly.

### Accumulation Is User-Level Learning

Accumulation stores durable user-level semantic assets:

- tendencies and preferences;
- heuristics and methods;
- repeated lessons and review gates;
- scoped rules or contract hints;
- project overlays that point to project truth without copying it;
- capability-use hints and searchable capability descriptors;
- approved small-grain memory facts.

An accumulation entry should be useful beyond one live turn. It should carry a
scope, applicability condition, source refs, and conflict behavior.

Accumulation is not:

- a project documentation warehouse;
- a duplicate of repository `docs/`;
- a complete session transcript;
- a debug-trace store;
- a provider raw-payload archive;
- a tool stdout warehouse;
- a capability package manifest truth store;
- a substitute for current user instructions, project requirements, or task
  acceptance criteria.

### Scope And Conflict Precedence

Runtime applies context in this order:

1. current user instruction;
2. current task scope and accepted plan;
3. current project requirement, design, contract, and source truth;
4. project-scoped accumulation overlay;
5. global user accumulation;
6. weak historical hints.

Global accumulation provides tendencies and learned heuristics. It does not
override current contracts. If a project contract conflicts with a global
preference, the local contract wins and the discovery result may include a
conflict notice.

A project-specific exception does not mutate global accumulation. It becomes a
project-scoped override. Global accumulation changes only through explicit
review, supersession, or repeated approved learning.

### Candidate Lifecycle

Accumulation entries begin as candidates unless the user explicitly creates an
approved entry through a trusted review path.

Allowed candidate sources:

- user says to remember or preserve a preference;
- model proposes a candidate after a turn;
- reviewer identifies a repeated lesson;
- project docs or external reading produce a durable overlay candidate;
- capability package metadata produces a searchable descriptor candidate.

Candidates require source refs or evidence. Activation requires review. Rejected
and superseded entries cannot be silently recalled. Forgotten entries must stop
appearing in discovery and future training exports.

### Discovery Results

Discovery results are bounded projections. They may include:

- candidate ref;
- kind;
- short claim or capability summary;
- scope;
- applicability note;
- conflict notice when local contracts override the candidate;
- source summary or ref;
- confidence class.

Discovery results must not include:

- raw credentials;
- hidden control fields;
- provider-native payloads;
- full skill bodies by default;
- full project documents by default;
- host paths or internal storage refs;
- permission modes, sandbox profiles, approval ids, or audit refs as
  model-selectable fields.

## Non-Goals

- No automatic approved-memory injection into provider context.
- No model fine-tuning or training pipeline.
- No local small-model router requirement in the first contract.
- No universal vector database requirement.
- No capability marketplace.
- No direct execution from discovery results.
- No dumping all capability schemas into provider context.
- No domain-specific kernel tools for Feishu, email, OCR, video, code graph, or
  other applications.

## Phased Delivery

Phase A: contract and manual owner loop.

- Define accumulation scope, conflict precedence, discovery candidate shape, and
  non-goals.
- Reuse the existing memory candidate/review/supersession lifecycle where it
  fits, but do not reintroduce automatic recall.
- Still short of production: no generalized discovery tool or index.

Phase B: bounded discovery surface.

- Add one small model-visible discovery surface that can query safe descriptors
  and active accumulation entries.
- Return top-K candidate summaries and conflict notices.
- Still short of production: execution remains through existing generic tools
  and user-space runners.

Phase C: capability and project overlays.

- Publish capability descriptors and project overlays into the discovery index
  without copying manifests or project docs as truth.
- Add inspection surfaces explaining what was discoverable, suppressed, or
  overridden.
- Still short of production: no automatic context injection.

Phase D: advanced learning candidates.

- Add repeated-pattern and reading-derived candidate proposal paths.
- Keep activation review-gated.
- Still short of production: no model parameter updates or autonomous training.

## Acceptance Criteria

Positive cases:

- a global preference can be approved and later discovered as a candidate;
- a project-scoped override can suppress a conflicting global preference without
  mutating it;
- a capability descriptor can be discovered without injecting its full manifest
  or skill body into every provider request;
- a project overlay can point to project docs without duplicating them;
- discovery explains why a result was returned and whether it is applicable.

Negative cases:

- discovery result does not grant tool authority;
- model-supplied discovery refs, ids, permission modes, sandbox profiles,
  credential refs, or approval ids do not become owner truth;
- rejected, superseded, or forgotten entries are not returned as active results;
- project documentation remains authoritative over accumulation overlays;
- full session transcripts, raw debug traces, provider payloads, and tool stdout
  do not enter accumulation by default.

Fail-closed and recovery:

- malformed discovery requests fail before owner lookup;
- unavailable discovery index returns structured not-ready or empty-result
  feedback, not fabricated candidates;
- replay reconstructs active, rejected, superseded, and forgotten accumulation
  states from owner facts.

Observability:

- users can inspect candidate source, scope, and status;
- context inspection can show that discovery was used without exposing hidden
  control fields;
- debug trace may capture retrieval diagnostics only when explicitly enabled.

## Relationship To Existing Issues

This requirement governs future work that reintroduces long-term recall,
capability discovery, project overlays, or low-context tool/capability routing.

Existing memory review behavior remains valid owner truth. The retired automatic
recall path stays retired until a future issue cites this requirement and an
approved design.
