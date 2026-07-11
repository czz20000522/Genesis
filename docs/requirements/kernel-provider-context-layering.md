# Requirement: Stable Provider Context Prefix And Variable Conversation Window

- **Status:** approved for phased implementation.
- **Owner:** Genesis Kernel Model Gateway; provider adapters own only native
  request projection.
- **Scope:** construct every model request as a stable prefix, a compactable
  conversation window, and a current-turn tail while retaining one canonical
  semantic conversation for all providers.

## Production Target

Genesis sends the model a role-correct request whose immutable prefix is reused
between turns whenever the selected provider binding, role, visible tool set,
and skill index are unchanged. Conversation history is the only normal
compaction subject. The current user input is always the final user message
before a model response or tool continuation.

## Semantics

1. The kernel owns one canonical ordered `ModelConversationMessage` sequence.
   It begins with exactly one stable `system` prefix message when a provider
   supports a system role; otherwise the adapter performs its documented native
   equivalent without reclassifying the current user request.
2. The stable prefix contains Genesis's provider-neutral operating instruction,
   the selected role/context policy, and the metadata-only skill index. It
   never contains credentials, host paths, kernel IDs, approvals, or a current
   user request.
3. Model-visible tool definitions remain a separate canonical manifest and are
   projected through the provider's native tool field. Their normalized content
   participates in the prefix fingerprint because it affects prompt caching.
4. The variable conversation window contains a prior compaction summary,
   persisted user/assistant/tool exchanges, and admitted per-turn/session
   context such as source snapshots or observations. It follows the stable
   prefix and preserves message roles and tool ordering.
5. The current user input is appended after all variable context. It must not
   be concatenated with skill metadata, a summary, or a system instruction.
6. The kernel computes a path-free prefix fingerprint from normalized stable
   system content, normalized tool manifest, selected provider adapter binding,
   and role/context-policy identity. A changed fingerprint is an intentional
   cache-reset explanation, not a new ledger authority fact.
7. Compaction receives and rewrites only the variable conversation window. It
   does not summarize, copy, or move stable prefix content. The resulting
   summary becomes the first variable conversation item after the prefix.
8. An adapter must consume the canonical conversation when present. Its
   `InputItems` fallback is only for direct legacy/test callers that did not
   supply a canonical conversation.
9. Provider-specific rules decide native projection only: system placement,
   tool wire format, reasoning replay, and any documented template wrapper.
   They may not change semantic role order or hide the latest user request.

## Current Delivery Boundary

This delivery persists a safe component digest for the materialized system
instruction, skill index, tool manifest, and provider adapter binding. It
compares that record with the prior model request in the same session and
projects `initial`, `system_instruction`, `skill_index`, `tool_manifest`, or
`adapter_binding` as applicable. The raw prefix text remains unprojected.

Genesis does not yet materialize a selected role/context-policy binding in this
prefix, so it cannot truthfully emit a `role_policy` reason. That reason remains
deferred until the binding has an owner-local persisted snapshot. The absence of
reasons must not cause automatic cache reuse; the digest remains the
authoritative cache-shape value.

## Failure And Recovery

- An adapter that cannot represent the required canonical role/tool sequence
  fails before egress with a safe provider configuration/error classification.
- A provider response with reasoning but no visible final remains a provider
  failure unless it is a valid tool-call continuation; reasoning is not
  fabricated into a final answer.
- A restart rebuilds the same prefix from selected runtime configuration and
  replays only persisted variable conversation facts. No provider-native prompt
  blob is ledger truth.
- A prefix is inspectable through a safe fingerprint projection; it does not
  expose the system text, local roots, or credentials.

## Non-Goals

- No global vendor prompt template.
- No automatic memory recall or unapproved context injection.
- No persistence of raw provider request JSON as ledger truth.
- No compaction of tools, system instructions, skill bodies, or provider
  credentials.

## Acceptance Criteria

1. A local llama.cpp request preserves system, user, assistant, and tool roles
   from canonical conversation rather than flattening them into one user text.
2. The latest user input occurs after stable prefix and variable history in
   every supported adapter projection.
3. Stable prefix and tool manifest produce the same fingerprint over consecutive
   turns; context inspection reports the exact changed component among system
   instruction, skill index, tool manifest, and adapter binding. Role policy is
   excluded until its selected binding is materialized and persisted.
4. Compaction preserves the stable prefix byte-for-byte while replacing only
   variable history with one summary plus its retained tail.
5. DeepSeek keeps `reasoning_content` response-only while GLM retains its
   documented same-binding tool-continuation replay behavior.
6. No provider request, inspection projection, or fingerprint exposes a host
   path, credential, kernel ID, or raw system prompt body.
