# Genesis Roadmap And Current Stage Task List

## Purpose

This document controls product sequencing. It is not a requirement, design,
implementation plan, or issue ledger.

- Product requirements and owner boundaries remain in `project-brief.md`,
  `kernel-contract.md`, `requirements/`, and `applications/`.
- A phase-local implementation plan exists only while an approved slice is
  being implemented, then retires under `operations/` after its evidence is
  preserved.
- Active, concrete gaps live only in `operations/kernel-issues.md` or
  `operations/application-issues.md`.
- A later stage is not active work merely because it appears in this document.

The route is:

```text
Foundation closure
  -> Desktop daily assistant
  -> User capability system
  -> Mobile and external entry
  -> Accumulation and long-term memory
  -> Fixed workflows
  -> Parent-worker runtime
  -> TaskGraph owner
  -> Continuous autonomy
  -> Genesis Home cold migration
  -> Optional single-writer synchronization
```

## Operating Rules

- Work on one stage at a time. A later-stage primitive may remain in the code,
  but is not a completed product feature before its stage exit criteria pass.
- Start a new implementation slice only after its requirement, design,
  concrete reference scan, and active issue exist. Obvious bug and test-gap
  work may use the documented exception.
- Close a stage with reproducible evidence. A green unit test does not replace
  a required live-provider, process-restart, external-source, or desktop proof.
- Human-only acceptance is tracked as `manual_test_pending` in the applicable
  issue/runbook and does not block implementation of the next approved stage.
  It remains required before any claim that the affected stage is fully
  accepted; it is not silently converted into automated evidence.
- If the next task requires a product choice, unavailable credential, unknown
  upstream protocol behavior, or a new owner boundary, stop and ask before
  implementing. Do not silently select a policy or invent a compatibility
  path.
- Do not create a marketplace, automatic memory injection, TaskGraph
  scheduler, cloud sync, or compatibility layer before the preceding stage
  permits it.

## Stages

### 1. Foundation Closure

**Purpose:** make currently claimed kernel, desktop, provider, material, and
connector surfaces truthful, restart-safe where their contracts promise it,
and accurately documented.

**Exit criteria:**

- A real provider passes the live first-run acceptance and a `genesisd`
  restart preserves the required session, timeline, event, and context
  projections.
- The desktop's currently exposed bridge has a focused build/test proof and no
  known deterministic startup or reconnect break.
- No cancelled external listener remains exposed as an active product path.
  An on-demand external CLI operation is governed tool work, not a connector
  lifecycle claim.
- The generic connector runtime is retained only for a future protocol that
  genuinely needs inbound event ownership and lifecycle evidence.
- Closed implementation plans and campaign notes no longer describe completed
  work as active backlog.

**Red lines:** do not add TaskGraph, new generic tools, new frameworks,
capability marketplace behavior, or a channel-specific kernel owner.

### 2. Desktop Daily Assistant

**Purpose:** let a new user complete a real governed task using only the
desktop application.

**Entry criteria:** Stage 1 is closed and the required live-provider proof is
current.

**Exit criteria:**

- Desktop guides provider setup, credential storage, upstream verification,
  diagnostics, and active profile switching without CLI-only steps.
- A user can create or resume a session, stream a response, submit a material,
  receive tool and approval state, recover a failed turn, and search sessions.
- Desktop either owns its local `genesisd` sidecar or attaches to an external
  kernel without crossing that ownership boundary.
- Restarting the application resumes an existing session and its settled
  projection.

**Red lines:** do not add more diagnostic surfaces after the first user path is
closed. Do not let the desktop assemble provider context or write kernel facts.

### 3. User Capability System

**Purpose:** make installed capability packages discoverable and runnable
without adding domain logic to the kernel.

**Entry criteria:** the desktop closed-loop task is usable and one real
manually installed capability has supplied acceptance evidence.

**Exit criteria:**

- `~/.genesis/capabilities/<id>` packages have a descriptor, health check,
  managed entrypoint, output handling, and package-local state boundary.
- `genesisd` discovers a package descriptor and returns its safe availability
  projection.
- Two materially different packages run through the same generic path without
  kernel changes.

**Red lines:** no marketplace, kernel package manager, arbitrary-path scan, or
automatic promotion of outputs into resources or memory.

**Current status (2026-07-11):** shared inspection and daemon discovery are
implemented, but no real user-owned capability package is installed. Video
transcript extraction remains a general Skill or future Workflow; no
report-generation capability exists. Real-package acceptance is therefore
deferred rather than satisfied with a synthetic package.

### 4. Mobile And External Entry

**Purpose:** add a real external entry only when its product semantics require
inbound event ownership rather than user-directed CLI work.

**Entry criteria:** a desktop task can produce a bounded final result or
artifact and Stage 3 has at least one usable capability package.

**Exit criteria:**

- The selected protocol's inbound identity mapping, source validation posture,
  dedupe, session mapping, turn submission, outbox delivery, and restart
  inspection have real evidence.
- The selected protocol's credential and external-process failure become
  observable connector state and recover through an approved owner path.
- The connector never becomes kernel truth, provider-context, or credential
  authority owner.
- Source recovery claims match upstream reality. If replay is unavailable, the
  operator can see the recovery coverage limit instead of being promised that
  downtime cannot lose messages.

**Red lines:** no assumed cursor replay, no readiness treated as event
authenticity, and no shell loop presented as supervision. Do not turn a
user-directed Feishu CLI operation into a listener without an approved routing
and progress contract.

### 5. Accumulation And Long-Term Memory

**Purpose:** let users review and control reusable learning before it affects a
new task.

**Entry criteria:** repeated desktop or mobile use has produced concrete
preference, method, or project-overlay examples.

**Exit criteria:**

- Users can inspect, approve, reject, supersede, and forget accumulation
  candidates across global, project, and task scopes.
- Each context activation decision records selected and suppressed candidates,
  source refs, scope, conflict result, and reason.
- Context inspection explains why an entry was used or not used without leaking
  hidden authority fields.

**Red lines:** no automatic RAG, no unreviewed recall, no training pipeline,
and no accumulation entry overriding current user or project contracts.

### 6. Fixed Workflows

**Purpose:** run a real repeated production process as a fixed user-space
workflow.

**Entry criteria:** one repeated multi-step task exists and its operator can
name its steps, terminal outcomes, artifact needs, and approval points.

**Exit criteria:**

- A workflow definition is compiled, validated, and bound by hash for each
  run.
- One workflow survives process restart, pauses for approval, handles declared
  retry behavior, can be cancelled, and records a terminal evidence trail.
- Nodes use kernel public primitives and retain only their resulting refs or
  projections.

**Red lines:** workflow definitions are not scripts, LLM nodes cannot change a
running graph, and Workflow does not become a dynamic TaskGraph.

### 7. Parent-Worker Runtime

**Purpose:** let one parent delegate bounded work to role-bound leaf workers
and return a reviewed, unified answer.

**Entry criteria:** an actual task exceeds a single-agent or fixed-workflow
path, with a measurable reason to split work.

**Exit criteria:**

- Role binding determines provider/profile, preset tools, context policy, and
  concurrency limits.
- Parent delegation, worker execution, review, reduce, failure reporting, and
  invocation recovery have end-to-end evidence.
- Desktop projects child conversations, progress, terminal results, and safe
  failure reasons without exposing raw prompts or tool traces to the parent.

**Red lines:** workers remain leaf-only and cannot broaden their grants. This
stage does not create TaskGraph topology, scheduling, or a business-role
taxonomy in the kernel.

### 8. TaskGraph Owner

**Purpose:** own dynamic project work topology separately from workflow and
agent execution.

**Entry criteria:** parent-worker execution has stable invocation identities,
bounded result projections, and a real multi-step project that needs visible
dependencies and evidence.

**Exit criteria:**

- TaskGraph owns graph and node identity, validated edits, dependency state,
  evidence refs, projection, replay, and operator intervention.
- Graph nodes reference admitted invocations and workflow runs but never grant
  authority or execute work directly.

**Red lines:** TaskGraph is not a kernel scheduler, does not choose providers,
grant tools, or write kernel execution facts.

### 9. Continuous Autonomy

**Purpose:** allow an approved long-running project to make bounded progress
across waits, restarts, reviews, and operator interventions.

**Entry criteria:** TaskGraph has proven value on real projects and the user can
state budgets, wait policy, retry policy, stop conditions, and intervention
rules.

**Exit criteria:**

- A multi-hour or multi-day project can decompose, execute, wait, resume,
  expose evidence, and stop at its declared boundaries.
- Result-based self-review may create review evidence or accumulation
  candidates, but cannot rewrite system policy or self-grant authority.

**Red lines:** no self-modifying workflow, no policy mutation from model output,
and no autonomous execution beyond declared budget and stop rules.

### 10. Genesis Home Cold Migration

**Purpose:** move one local Genesis Home to another machine or personal server
without losing authority boundaries or required recovery evidence.

**Entry criteria:** Stage 9 has demonstrated durable value worth moving.

**Exit criteria:**

- `~/.genesis` has an explicit export/import contract for configuration,
  capability packages, accumulation, supported session and workflow state, and
  required evidence.
- Credentials are re-authorized or re-wrapped, never copied as portable raw
  secrets.
- The imported Home resumes supported work with the same authority and identity
  constraints.

**Red lines:** this is a cold, single-writer migration. It does not implement
simultaneous cross-device editing or conflict resolution.

### 11. Optional Single-Writer Synchronization

**Purpose:** connect multiple entry points to one Genesis Home only after cold
migration and continuous use justify it.

**Entry criteria:** cold migration is proven and a concrete multi-device use
case requires live access rather than periodic migration.

**Exit criteria:** one declared Home remains the authority writer, synchronization
preserves evidence and identity boundaries, and credentials retain their
re-authorization boundary.

**Red lines:** no multi-tenant SaaS, offline concurrent merge, or distributed
authority election.

## Current Execution Card: Stage 1 Foundation Closure

**Status:** active.

**Sequencing note (2026-07-11):** the user directed a parallel Stage 2 desktop
provider-control slice before the manual Project / Task / Chat local-Qwen
acceptance is performed. This is not a Stage 1 completion claim: manual
desktop evidence is tracked as `manual_test_pending` and does not block later
approved implementation; connector gates retain their own stage boundaries.

### Ready Work

1. Reconcile the product claim inventory.
   - Read the project brief, active issue ledgers, live-provider acceptance
     runbook, desktop tests, and connector lifecycle records.
   - Remove or correct only claims that disagree with current evidence.
   - Do not reopen closed implementation plans merely to retain checklists.

2. Run the existing live-provider first-run acceptance against a real provider.
   - Use `scripts/first_run_live_llm_acceptance.ps1` only for a generic
     OpenAI-compatible route with an API key supplied through the operator
     environment, never committed or pasted into docs.
   - For a configured `provider_command` route, use the dedicated acceptance
     surface recorded in the active application issue. Do not force that route
     through a generic OpenAI-compatible profile.
   - Record the produced session, turn, restart, and failure-probe evidence in
     an operational record.
   - If it fails, stop the broad closure pass and open one narrow issue for the
     observed deterministic gap.

3. Re-run focused desktop bridge verification.
   - Prove sidecar ownership versus external-kernel attachment, streaming,
     material upload, approval decision, session list/search bridge, and
     restart-safe timeline reads through existing tests and the desktop build.
   - Classify any failure by owner before changing code. A desktop rendering
     gap stays desktop-owned; an authority or projection gap moves to the
     kernel owner.

4. Keep the retired Feishu listener out of the active product surface. Select a
   connector slice only after a future external protocol has an approved
   routing, progress, and recovery contract.

5. Run the Stage 1 closing gate.
   - Run focused checks, then `go test ./... -count=1`, `go build ./...`, and
     `git diff --check` for any changed slice.
   - Compare claims against the project brief, active issue, requirement,
     design, BDD feature, and live evidence.
   - Retire only completed issues and phase plans. Keep unresolved external
     limitations explicit in the active issue rather than hiding them in this
     roadmap.

6. Close provider reasoning semantics before expanding provider setup.
   - Treat reasoning as a persistent assistant message and project it through
     the desktop timeline.
   - Let each adapter map external fields and continuation rules. Do not use a
     global discard or replay setting.
   - Prove a local llama.cpp adapter and conflicting provider replay contracts
     before broadening the first-run acceptance surface.

### Operator Gates

- A real provider credential and reachable provider are required for the live
  acceptance. The repository cannot prove that result without them.
- A future external source requires a valid profile/credential and an upstream
  source contract before Genesis claims source authenticity, replay, or
  automatic refresh behavior.
- If either gate is unavailable, continue only with read-only reconciliation and
  local regression proof. Do not claim Stage 1 closed.

### Stage 1 Completion Record

When the stage closes, add a compact operational record with:

- the live-provider acceptance summary and restart proof;
- desktop verification commands and results;
- the exact connector recovery guarantee and known upstream limit;
- active issue ids remaining, or explicit confirmation that none remain;
- the document claims corrected or retired.
