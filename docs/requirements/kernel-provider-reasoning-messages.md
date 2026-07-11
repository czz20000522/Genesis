# Requirement: Provider Reasoning Messages And Adapter Replay

- **Status:** approved for phased implementation.
- **Owner:** Genesis Kernel model gateway, session transcript, and provider-context boundaries.
- **Scope:** preserve provider reasoning as a durable assistant message, project it to users, and let the selected provider adapter decide whether and how that message re-enters later provider requests.

## Background

Reasoning-capable providers return a user-readable thinking trace beside final
assistant text. Users need that trace after restart, in the session timeline,
and during review. Providers do not share one continuation rule: some reject a
prior reasoning field, while others require it after a tool call or require a
signed continuation block.

Genesis currently reduces a provider response to final text, tool calls, model,
and usage. Its one `hidden_reasoning_policy=discard` setting cannot express
per-message continuation rules. It also causes a configured llama.cpp adapter
to discard reasoning that the user expects to inspect.

## Production Target

Genesis records reasoning as a durable assistant message with a stable kernel
schema. Session replay projects it next to associated final text or tool
activity. The Model Gateway passes canonical assistant messages to the selected
adapter. The adapter maps vendor fields into the canonical schema on input and
maps only required canonical parts back into its vendor request shape on output.

The kernel never uses a global rule to decide whether reasoning is replayed.
Each adapter evaluates the recorded message, current continuation shape, and its
documented provider contract.

## Users And Roles

User:

- can expand persisted reasoning in a conversation and see it after a desktop,
  daemon, or connector restart;
- receives final assistant text separately from reasoning.

Kernel:

- owns durable reasoning facts, session replay, timeline projection, bounded
  retention, and refusal of malformed canonical reasoning records;
- does not interpret vendor field names or decide provider-specific replay.

Provider adapter:

- owns inbound vendor-field mapping, outbound request mapping, and the
  per-message decision to replay, omit, or forbid a reasoning record;
- may retain bounded opaque replay state when an upstream contract requires a
  signature or continuation token;
- never exposes vendor-native replay state in user, model, or ordinary
  inspection projections.

Desktop and other applications render kernel reasoning projections and realtime
reasoning deltas. They do not reconstruct provider history, alter replay
directives, or create transcript facts.

## Core Semantics

1. A `ReasoningMessage` is a durable assistant message. It has a kernel id,
   text, turn id, creation time, and optional adapter-owned replay record.
2. A provider response may contain reasoning with a final answer, tool calls, or
   both. The kernel records reasoning before associated terminal-final or tool
   continuation facts.
3. Reasoning text is visible through session and timeline projections. Final
   answer text remains a separate assistant final projection.
4. A replay record is bound to one configured adapter binding. It has a stable
   replay disposition and bounded adapter-owned state. The kernel may store it
   for restart recovery but does not decode it, copy it to another adapter, or
   expose it as a model-visible field.
5. The adapter chooses replay per recorded message and continuation:
   `required`, `omitted`, or `forbidden`. A profile selects an adapter but
   cannot override a message-level provider rule.
6. A provider request uses an ordered canonical conversation when the adapter
   needs assistant-message structure. The adapter emits its own role, content,
   field, and ordering syntax.
7. A provider switch preserves user-visible reasoning and final messages. It
   excludes replay records from the old binding and explains the suppression.
8. Realtime reasoning deltas are transport data until the provider response
   settles. The completed reasoning message, not every delta, is ledger truth.
9. Reasoning and replay state count toward declared context and storage limits.

## Failure Semantics

- Unknown vendor response fields remain adapter concerns. An adapter maps them
  deliberately or rejects the response with a provider error.
- A malformed canonical reasoning message, invalid replay disposition, wrong
  adapter binding, oversized replay state, or replay state on another adapter
  fails before a later provider request is sent.
- If an adapter requires replay state that is absent or invalid, the turn fails
  with a typed provider continuation error. Genesis does not substitute final
  text for a required provider continuation block.
- If an adapter forbids replay, Genesis persists and projects the reasoning but
  omits it from that adapter's next request.
- Provider change, compaction, and restart preserve an explanation of a
  suppressed replay record without exposing opaque contents.

## Non-Goals

- No raw vendor response JSON in kernel projections or model context.
- No global `reasoning_content` field in user-facing transport contracts.
- No promise that all providers expose or permit user-visible reasoning.
- No cross-provider conversion of adapter replay state.
- No desktop-owned transcript store or provider-context assembly.

## Phased Delivery

### Phase A: Canonical durable reasoning messages

Add canonical reasoning to the provider response, ledger event, session replay,
timeline projection, and desktop rendering. Prove the llama.cpp provider-command
adapter preserves and projects reasoning after a restart. This phase deliberately
does not create or persist adapter replay state; it still falls short when a
provider requires structured replay across a later user turn.

### Phase B: Adapter-directed continuation replay

Replace flattened assistant-history transport with ordered canonical conversation
messages at the provider boundary. The configured DeepSeek V4 adapter treats
`reasoning_content` as response-only: it retains canonical reasoning locally
for projection but omits it from every later request, including a tool
continuation. The durable record remains bound to its emitting adapter for
audit, but DeepSeek needs no replay state.

### Phase C: Additional adapter contracts and compaction

Add adapters only after their official continuation rules are recorded. Prove
provider switching, compaction, replay-state suppression, and recovery for each
new contract.

The first Phase C slice is the `zai-glm` adapter profile `glm-5.2`, used through
the configured OpenCode Go OpenAI-compatible route. Z.AI documents that GLM
returns `reasoning_content`; ordinary requests clear earlier reasoning by
default, while a preserved-thinking tool continuation must replay the complete,
unchanged, ordered reasoning field and send `thinking.clear_thinking=false`.
Genesis therefore records the returned text, clears it on an ordinary later user
turn, and replays it only with the same-bound assistant tool call and its tool
result. It sends `thinking.clear_thinking=true` for the ordinary path and
`false` for the required continuation path. This slice carries no opaque
vendor state, and it does not make the rule apply to GLM-5.1, MiniMax, or any
unbound OpenAI-compatible model.

## Acceptance Criteria

1. A provider reasoning response creates a durable reasoning message and a
   session/timeline projection after restart.
2. The desktop renders reasoning as an expandable section distinct from final
   assistant text.
3. The llama.cpp command adapter maps `reasoning_content` into the canonical
   message without silently discarding it.
4. The DeepSeek adapter omits reasoning from an ordinary later user request
   while retaining the local projection.
5. The DeepSeek adapter never emits retained `reasoning_content` in later
   requests, including assistant tool-call messages; it serializes required
   empty assistant `content` alongside native `tool_calls`.
6. A missing required GLM reasoning message or mismatched GLM adapter binding
   fails closed before egress. Opaque signed-state, provider-switch explanation,
   and byte-limit contracts remain Phase C work.
7. Realtime deltas do not become a durable stream of ledger facts; only the
   settled reasoning message survives replay.
8. The `zai-glm` / `glm-5.2` binding maps GLM reasoning into the canonical
   message, sends the documented clear-thinking flag, and fails before egress
   if a required same-binding tool continuation cannot be reconstructed.

## Relationship To Existing Issues

`KERNEL-PROVIDER-REASONING-MESSAGES-20260710` tracks the implementation gap.
`APP-FIRST-RUN-PROVIDER-COMMAND-ACCEPTANCE-20260710` is the downstream
acceptance-runbook gap and must use the canonical reasoning path when Phase A
is available.
