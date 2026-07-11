# Provider Reasoning Messages Implementation Plan

**Goal:** deliver Phase A of the approved reasoning-message requirement: preserve
llama.cpp reasoning as a durable assistant message and project it through the
existing desktop timeline.

**Architecture:** The provider command maps vendor reasoning into one strict
Genesis response field. The kernel records one `model.reasoning` fact before the
existing terminal final fact, then session and timeline replay consume that fact.
This slice records no cross-turn adapter replay state; Phase B owns ordered
continuation replay.

**Red lines:** do not add raw vendor maps to kernel projections, change provider
context replay, add a global reasoning policy, or bypass the Model Gateway.

## Reference Summary

- Codex maps reasoning into a typed turn item and projects it independently.
- Reasonix persists reasoning with assistant history, but its provider-specific
  replay logic is Phase B work here.
- Before this slice, the local llama.cpp adapter received `reasoning_content`
  and dropped it before the strict provider-command boundary.

## Phase A Tasks

### Task 1: Define a canonical reasoning response and command boundary

**Files:**

- Modify: `internal/kernel/provider.go`
- Modify: `internal/kernel/provider_command.go`
- Modify: `internal/kernel/provider_command_test.go`
- Modify: `scripts/providers/llama_cpp_provider_command.py`

1. Add a failing command-provider test whose strict stdout includes a canonical
   `reasoning` object and whose resulting `ModelResponse` retains its text.
2. Run the focused test and observe the expected unknown-field failure.
3. Add the smallest `ReasoningMessage` field to `ModelResponse` and the matching
   strict provider-command response field. Reject empty reasoning objects and
   unknown fields as before.
4. Change the llama.cpp adapter to map upstream `reasoning_content` into that
   response field and extend its self-test.
5. Re-run focused command-provider tests and the Python self-test.

### Task 2: Persist reasoning as a transcript fact

**Files:**

- Modify: `internal/kernel/event_types.go`
- Modify: `internal/kernel/turn_types.go`
- Modify: the kernel turn-loop owner that appends `model.final`
- Modify: `internal/kernel/session_projection.go`
- Modify: `internal/kernel/projections.go`
- Test: a new focused reasoning-message kernel test

1. Add a failing test with a provider that returns reasoning and final text.
   Assert a `model.reasoning` fact precedes `model.final`, survives a new kernel
   over the same ledger, and is projected with the turn.
2. Add the smallest event payload and turn projection needed for the reasoning
   message. Do not persist stream chunks.
3. Append the reasoning fact before final text. Preserve current final response
   and tool-loop behavior when reasoning is absent.
4. Re-run the focused persistence and session replay tests.

### Task 3: Project reasoning into the kernel timeline and desktop transcript

**Files:**

- Modify: `internal/kernel/ui_timeline_projection.go`
- Modify: `desktop/frontend/src/timelineView.ts`
- Modify: `desktop/frontend/src/components/ConversationPane.vue`
- Modify: `desktop/frontend/src/styles.css`
- Test: existing desktop timeline tests plus a focused frontend projection test
  when the current test harness supports it

1. Add a failing timeline test asserting a reasoning item appears before the
   final assistant message and is available after replay.
2. Add an `assistant_reasoning` timeline item with its own stable item id and text.
   Keep adapter replay state absent from the projection.
3. Map that item to a separately labelled reasoning row in the desktop conversation.
   The desktop renders the kernel projection and stores no reasoning state.
4. Re-run focused kernel timeline and desktop tests/build.

### Task 4: Update contracts and close the Phase A evidence

**Files:**

- Modify: `docs/operations/kernel-issues.md`
- Modify: `docs/operations/application-issues.md`
- Modify: `docs/implementation-plans/kernel-provider-reasoning-messages.md`

1. Compare the implementation with the requirement, design, BDD scenarios, and
   this plan. Mark cross-turn required/forbidden replay as Phase B, not done.
2. Run focused tests, `go test ./... -count=1`, `go build ./...`, desktop test
   and build, Python adapter self-test, and `git diff --check`.
3. Run a local llama.cpp turn with reasoning, restart the daemon, and read the
   same session timeline. Record only the result and commands, never the raw
   user model output.

## Phase A Acceptance

- The local provider command returns canonical reasoning without losing final
  text or tool calls.
- Kernel restart replay and timeline projection retain reasoning.
- Desktop can show the projected message separately from the assistant final.
- No generic OpenAI-compatible response changes its current fail-closed
  behavior in this slice.

## Phase A Execution Evidence

- The command-response test first failed because `reasoning` was an unknown
  strict field and `ModelResponse` had no canonical field; the focused command,
  persistence/restart, timeline, and desktop tests now pass.
- `scripts/providers/llama_cpp_provider_command.py --self-test` proves final and
  tool-call responses preserve a non-empty `reasoning_content` as canonical
  reasoning text.
- On 2026-07-10, the configured local Qwen provider verified ready through
  `genesisctl`; a real provider-command turn wrote `model.reasoning` before
  `model.final`, and a new daemon over the same SQLite ledger preserved the same
  reasoning id and projected text length in both session and timeline reads.
- The current local baseline is `-c 262144 -ngl auto --cache-type-k q8_0
  --cache-type-v q8_0 --parallel 2` on port 8081. Its probe measured 60.59
  generated tokens per second; later tuning compares against this baseline.

## Remaining After Phase A

Phase B replaces flattened assistant-history transport with ordered canonical
conversation messages. The corrected DeepSeek V4 rule is response-only
reasoning: omit it from ordinary and tool-continuation requests, while retaining
it in the kernel transcript. No provider profile may claim a DeepSeek replay
requirement without current provider evidence.

## Phase B: DeepSeek V4 Adapter-Directed Replay

**Goal:** preserve structured conversation at the provider boundary while the
DeepSeek V4 adapter keeps response-only reasoning out of every later request.

**Architecture:** `ProviderContextProjection` gains canonical ordered messages
derived from ledger facts. It no longer folds completed assistant history into a
user-text input. The OpenAI-compatible provider maps those messages only when
its adapter binding is DeepSeek V4: it maps inbound `reasoning_content` to the
canonical reasoning message and omits it from all outbound messages. The
reasoning message retains its emitting adapter binding for local projection;
no DeepSeek continuation depends on it.

**Reference scan:** Codex's `codex-rs/core/src/event_mapping.rs` maps
`ResponseItem::Reasoning` to a typed turn item. Reasonix's
`internal/provider/provider.go` stores reasoning with its assistant message,
and `internal/provider/anthropic/anthropic.go` replays it only in the same
provider's signed thinking block before `tool_use`. Genesis uses the same
ordered-message and same-binding control, but DeepSeek's official thinking-mode
contract needs canonical text rather than an opaque signature.

**Red lines:** do not add a global policy, raw vendor map, generic opaque-state
container, desktop behavior, provider command replay contract, or a compatibility
reader for flattened assistant history.

### Task 1: Lock the adapter contract with red tests

**Files:**

- Modify: `internal/kernel/openai_compatible_reasoning_test.go`
- Modify: `internal/kernel/tool_loop_integration_test.go`
- Modify: `internal/kernel/model_config_test.go`
- Modify: `features/kernel/provider_reasoning_messages.feature`

1. Add a failing ordinary-follow-up test: the second DeepSeek request contains
   the prior assistant final but neither `reasoning_content` nor its text.
2. Add a failing tool-loop test: the DeepSeek assistant tool-call message has
   `content:""` and native tool calls, but no `reasoning_content` field.
3. Preserve the no-egress test only for provider contracts that require replay,
   such as the configured GLM profile.
4. Replace the old configuration test for `hidden_reasoning_policy` with a test
   that the DeepSeek adapter/profile binding selects this contract.

### Task 2: Carry only canonical ordered messages through the context owner

**Files:**

- Modify: `internal/kernel/provider.go`
- Modify: `internal/kernel/kernel.go`
- Modify: `internal/kernel/model_context.go`
- Modify: `internal/kernel/projections.go`

1. Add the smallest canonical conversation-message type: semantic role, text,
   tool calls, tool-call result linkage, and optional reasoning text with its
   emitting adapter binding.
2. Build that sequence from `turn.submitted`, `model.reasoning`, `tool.call`,
   `tool.result`, and `model.final` ledger facts. Keep compaction summary and
   current user context as distinct messages.
3. Make selected built-in adapters consume the ordered sequence rather than
   `conversation_history_context`; retain the flattened input only for the
   unchanged provider-command adapter until its own contract is approved.
4. Clone the sequence at the provider boundary and keep bindings out of session,
   timeline, audit, and context-inspection projections.

### Task 3: Implement the one DeepSeek mapping

**Files:**

- Modify: `internal/kernel/provider_adapter.go`
- Modify: `internal/kernel/openai_compatible.go`
- Modify: `internal/kernel/modelgateway/resilience.go`

1. Replace `hidden_reasoning_policy` with the fixed DeepSeek V4 adapter binding
   predicate; non-DeepSeek vendor reasoning remains fail-closed.
2. Map inbound `reasoning_content` to canonical reasoning and stamp the active
   adapter binding before the kernel persists it.
3. Translate ordered canonical messages to OpenAI-compatible messages. DeepSeek
   omits `reasoning_content` for all messages; a tool-call assistant serializes
   empty `content` and structured `tool_calls`.
4. Keep nonretryable continuation refusal only in adapters whose documented
   contract requires replay.

### Task 4: Verify and close the Phase B slice

**Files:**

- Modify: `docs/operations/kernel-issues.md`
- Modify: `docs/implementation-plans/kernel-provider-reasoning-messages.md`
- Modify: `features/kernel/provider_reasoning_messages.feature`

1. Run focused reasoning, tool-loop, context, and configuration tests, then
   `go test ./... -count=1`, `go build ./...`, desktop tests/build, and
   `git diff --check`.
2. Compare the implementation against this plan, requirement, design, issue,
   and BDD feature. Move only completed Phase B evidence to the retirement log;
   leave opaque signed state, provider switching, and compaction recovery for
   Phase C.

## Phase B Evidence

- `TestDeepSeekThinkingAdapterOmitsReasoningOnOrdinaryFollowUp` proves that a
  persisted, user-visible reasoning message is absent from the next ordinary
  DeepSeek request while the prior final answer remains.
- `TestDeepSeekThinkingAdapterOmitsReasoningForToolContinuation` proves the
  tool continuation also omits response-only reasoning.
- `TestDeepSeekThinkingAdapterSerializesEmptyContentForToolCall` proves the
  DeepSeek assistant tool-call message carries `content:""` and native tool
  calls.
- `TestZAIGLMThinkingAdapterRejectsToolReplayWithoutBoundReasoning` retains the
  no-egress continuation refusal for the profile that actually requires replay.
- The llama.cpp command adapter now consumes canonical conversation separately;
  its reasoning self-test remains independent of DeepSeek's outbound rule.
- Verification ran with `go test ./... -count=1`, `go build ./...`, desktop Go
  tests, desktop frontend tests/build, and `git diff --check`. No live DeepSeek
  request was made: this slice does not switch or mutate the user's configured
  provider profile.

## Phase C: OpenCode Go GLM-5.2 Preserved Thinking

**Goal:** add exactly the documented GLM-5.2 reasoning contract to the selected
OpenAI-compatible adapter binding, without making OpenCode Go a role or
generalizing vendor fields for every compatible model.

**Reference summary:** OpenCode Go advertises `glm-5.2` on its
OpenAI-compatible chat-completions endpoint. Z.AI documents `reasoning_content`
and requires full ordered replay with `thinking.clear_thinking=false` when
preserved thinking continues through tool results; ordinary turns use the
clearing path. Reasonix carries reasoning only on ordered assistant messages
and replays it at the provider-required tool boundary; Codex keeps reasoning as
a typed projected item rather than a raw transport map.

### Task 1: Lock the GLM contract with red tests

**Files:**

- Modify: `internal/kernel/openai_compatible_reasoning_test.go`
- Modify: `features/kernel/provider_reasoning_messages.feature`

1. Add a failing ordinary-follow-up test for binding `zai-glm` / `glm-5.2`.
   It must accept inbound `reasoning_content`, retain the visible final in
   canonical history, omit that reasoning on the later request, and send
   `thinking.clear_thinking=true`.
2. Add a failing tool-continuation test. The continuation request must send
   `thinking.clear_thinking=false` and place the same-bound reasoning before
   the assistant `tool_calls` and tool result.
3. Add failing missing- and wrong-binding continuation tests that assert
   `provider_reasoning_continuation_unavailable` and zero HTTP requests.
4. Run the focused test and confirm the current generic decoder fails with
   `provider_vendor_field_unsupported`.

### Task 2: Add the narrow adapter dictionary

**Files:**

- Modify: `internal/kernel/provider_adapter.go`
- Modify: `internal/kernel/openai_compatible.go`

1. Add one binding predicate for adapter id `zai-glm`, profile id `glm-5.2`,
   and `openai-chat-completions` transport. Do not accept a route name or a
   model-id prefix as an adapter policy.
2. Let that predicate map inbound `reasoning_content` to the existing canonical
   reasoning message.
3. Add only the typed `thinking.type` and `thinking.clear_thinking` request
   fields. Emit the clearing form for ordinary GLM requests and the preserved
   form only for a validated same-binding tool continuation.
4. Reuse the existing ordered conversation and continuation refusal; do not add
   a generic vendor field map, opaque-state store, or a new provider interface.

### Task 3: Bind the configured GLM profile and prove it

**Files:**

- Modify: `docs/operations/kernel-issues.md`
- User config: `~/.genesis/config/models.json`

1. Change only the configured GLM profile's adapter binding from the route
   label to `zai-glm` / `glm-5.2`; leave the semantic `coder` role, OpenCode Go
   route, credential reference, and model id independent.
2. Run the focused reasoning/provider tests, then `go test ./... -count=1`,
   `go build ./...`, and `git diff --check`.
3. Run `genesisctl provider verify --model-role coder` and one real bounded
   tool-loop turn. Record sanitized readiness and restart evidence only.

### Phase C Red Lines

- Do not make `opencode-go` an adapter or role identity.
- Do not enable GLM preserved thinking for GLM-5.1, MiniMax, or unbound models.
- Do not replay reasoning on ordinary user continuations.
- Do not expose `thinking` or `reasoning_content` as kernel-wide public fields.

### Phase C Evidence: OpenCode Go GLM-5.2 slice

- `TestZAIGLMThinkingAdapterClearsReasoningOnOrdinaryFollowUp` proves inbound
  GLM reasoning becomes the existing canonical message, is absent from a later
  ordinary request, and sets `thinking.clear_thinking=true`.
- `TestZAIGLMThinkingAdapterReplaysSameBoundReasoningForToolContinuation`
  proves a same-binding tool continuation emits the unchanged reasoning before
  its tool calls and sets `thinking.clear_thinking=false`.
- `TestZAIGLMThinkingAdapterRejectsToolReplayWithoutSameBoundReasoning` proves
  missing, profile-mismatched, and case-distinct bindings fail locally with
  `provider_reasoning_continuation_unavailable` and zero provider requests.
- `TestZAIGLMThinkingAdapterStreamClearsReasoningOnOrdinaryFollowUp` and
  `TestZAIGLMThinkingAdapterStreamReplaysSameBoundReasoningForToolContinuation`
  prove the desktop's streaming provider path applies the same clear/preserve
  rules and settles streamed reasoning into the canonical message.
- The configured `coder` role now selects profile `opencode-go-glm-5-2`, whose
  route remains `opencode-go` and whose independent adapter binding is
  `zai-glm` / `glm-5.2`.
- On 2026-07-11, `genesisctl provider verify --model-role coder --profile-id
  opencode-go-glm-5-2 --timeout-sec 30` returned `readiness=ready` for
  `frank/GLM-5.2`. An isolated daemon submitted one `shell_exec(pwd)` tool
  loop, received `TOOL_LOOP_OK`, wrote one `model.reasoning` before the tool
  call and one `model.final`, and retained both records after a daemon restart.

## Remaining After the GLM-5.2 Slice

This completes the selected OpenCode Go GLM-5.2 adapter contract. The broader
production requirement remains incomplete: opaque signed replay state,
provider-switch suppression explanations, compaction recovery, and any further
provider-specific contracts require their own approved slice. This plan does
not authorize a generic OpenAI-compatible vendor map or additional adapters.
