# Requirement: Session Model Binding

- **Status:** approved for implementation planning.
- **Owner:** Genesis Kernel session authority and Model Gateway; desktop is a
  projection and request surface.
- **Scope:** give every desktop Project, Task, and Chat session an explicit,
  durable selected model profile without changing another session's selection.

## Production Target

A new session has no model until its operator selects one. The selected profile
is a durable session fact. Every later turn for that session resolves and uses
that profile; a different session may use a different profile concurrently.
Restart restores both selections. Changing a selected profile affects only
future turns in that one idle session.

## Semantics

1. The kernel owns a `session.model_bound` fact containing the session id and
   validated model-profile id. The fact is visible in safe session projection,
   never in model context, search snippets, or provider prompt fields.
2. A submitted turn for an unbound session fails with stable
   `session_model_unselected`; desktop must prompt for a model rather than
   falling back to a global profile.
3. A model change is rejected while that session has an active turn. When
   accepted it appends a new binding fact and applies only to subsequent turns.
   It does not alter another session, global role binding, or historical turns.
4. Each turn resolves its bound profile through the kernel-owned session
   provider resolver. One provider instance remains fixed for all rounds of
   that turn; adapter-specific continuation/replay rules remain in the Model
   Gateway.
5. The resolver validates that the profile remains configured and credentialed
   before binding or use. A broken profile preserves the old session binding
   but fails the attempted turn with a stable provider reason.
6. The session binding is that session's coordinator provider. It is selected
   at session creation or later changed while idle; no global `coordinator`
   profile needs to be configured first. Separate parent/worker role policy,
   where configured, is not a fallback or persistence source for ordinary
   desktop conversations.
7. Existing development sessions are not migrated from global role bindings.
   They become explicitly selectable through the desktop like all other
   sessions.

## Non-Goals

- No desktop-local session-model truth, global active-chat model, or automatic
  model choice for a newly created session.
- No hot change to an active turn, hidden provider fallback, or cross-session
  propagation.
- No provider configuration mutation, credential writing, or model discovery
  in the kernel session endpoint.
- No replay rewrite: historical messages and reasoning remain facts of their
  original turns.

## Acceptance Criteria

1. Two sessions can bind different configured profiles; turns prove that each
   invokes its own profile after kernel and desktop restart.
2. A new Project, Task, or Chat cannot submit a turn before a profile is
   selected, and its selector remains visible in the composer.
3. Switching an idle session preserves its transcript and changes only later
   turns; switching during an active turn fails without a new binding fact.
4. A profile removed or made unavailable after binding produces an explainable
   provider failure, not a fallback to another model.
