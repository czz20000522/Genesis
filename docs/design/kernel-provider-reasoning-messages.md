# Design: Provider Reasoning Messages And Adapter Replay

- **Requirement:** `docs/requirements/kernel-provider-reasoning-messages.md`
- **Owner:** Genesis Kernel Model Gateway, transcript, and projection owners.

## Reference Scan

Codex:

- `codex-rs/core/src/event_mapping.rs` maps provider `ResponseItem::Reasoning`
  into a typed `TurnItem::Reasoning` with a stable item id, summary, and raw
  content.
- `codex-rs/app-server-protocol/src/protocol/v2/item.rs` projects that typed
  reasoning item separately from the agent message. Its event mapping sends
  reasoning deltas as application notifications instead of raw provider JSON.

Reasonix:

- `internal/provider/provider.go` carries reasoning text and an opaque
  reasoning signature on an ordered provider message.
- `internal/agent/agent.go` persists those values with the assistant message
  and streams typed reasoning deltas.
- `internal/provider/openai/openai.go` sends every OpenAI-compatible message
  with a `content` key, sanitizes tool-call pairing, and treats DeepSeek
  `reasoning_content` as response-only: it persists it locally but never sends
  it back upstream.
- `internal/provider/anthropic/anthropic.go` is the contrasting contract: it
  replays a signed thinking block only when that provider requires it.

Provider documentation:

- DeepSeek's reasoning-model guide says one model route rejects prior
  `reasoning_content` in later input.
- Genesis follows Reasonix's concrete DeepSeek OpenAI-client behavior for this
  configured adapter: `reasoning_content` is response-only, including tool
  continuations. A future vendor contract that requires replay must be a new
  explicit adapter profile, not a change to DeepSeek's default rule. See
  [reasoning model](https://api-docs.deepseek.com/guides/reasoning_model) and
  [thinking mode](https://api-docs.deepseek.com/guides/thinking_mode).
- Z.AI documents `reasoning_content` for GLM-5.2 and defines
  `thinking.clear_thinking=false` as preserved thinking: the caller must send
  the complete, unmodified, ordered historical reasoning field. Its tool-use
  example repeats that field on the assistant tool-call message before the
  tool result. See [chat completion](https://docs.z.ai/api-reference/llm/chat-completion)
  and [thinking mode](https://docs.z.ai/guides/capabilities/thinking-mode).
- OpenCode Go advertises GLM-5.2 at its OpenAI-compatible chat-completions
  endpoint, but that route identity is not a role or adapter identity. See
  [OpenCode Go](https://dev.opencode.ai/docs/go/).

Genesis aligns with Codex by making reasoning a typed projected item. It aligns
with Reasonix by retaining DeepSeek reasoning for display while keeping it out
of DeepSeek egress. Adapters that truly require replay keep that decision behind
the kernel projection boundary.

## Boundary And Owner

The Model Gateway owns adapter translation. The transcript owner owns durable
reasoning messages. The projection owner renders them for session and timeline
reads. The context owner constructs ordered canonical conversation items from
durable facts. The selected adapter turns those canonical items into one
provider request.

Desktop, connectors, provider commands, and built-in HTTP adapters are
non-owners of transcript truth. A provider command translates
`genesis.provider_command` input and output but cannot mint a ledger fact or
decide a kernel projection.

## Data Flow

```text
provider response or stream
  -> selected adapter maps vendor reasoning fields
  -> Model Gateway validates canonical reasoning and replay record
  -> kernel appends model.reasoning
  -> tool or final response continues through the existing turn owner
  -> session/timeline replay projects the reasoning message
  -> next turn context replays ordered canonical messages
  -> selected adapter applies its per-message dictionary and disposition
  -> provider request
```

`model.reasoning` is emitted before a tool-call continuation or `model.final`.
It is a durable assistant message, not a debug trace. Realtime deltas use stream
transport until settlement; they become one durable message only after a
successful provider response.

## Protocol

The kernel uses stable semantic types:

```go
type ReasoningMessage struct {
    ReasoningID string
	TurnID      string
    Text        string
	CreatedAt   time.Time
}

type ModelResponse struct {
    Reasoning *ReasoningMessage
    Text      string
    Model     string
    ToolCalls []ModelToolCall
    Usage     *TokenUsage
}
```

Phase B records the emitting adapter binding with a reasoning message for audit.
DeepSeek treats its canonical text as response-only and never replays it, so
this phase does not create an opaque replay-state container. A later provider
that requires a signature or token adds an
`AdapterReplayRecord` together with its persistence, byte limit, binding
validation, and fail-closed tests. That state never appears in `FinalMessage`,
ordinary event inspection, timeline, session projection, context inspection, or
another adapter request.

Phase A `provider_command` gains a strict `reasoning` response object containing
only semantic text. The command adapter maps llama.cpp `reasoning_content` into
`reasoning.text`; the kernel assigns the message id, turn id, and creation time.
The command cannot mint replay state. Phase B leaves command replay unchanged;
Phase C adds opaque provider-owned replay records only with the provider contract
that needs them.

Provider context adds ordered canonical messages with semantic `role`, text,
tool calls, tool result linkage, and optional reasoning text plus its adapter
binding. It preserves the existing input-item and tool-result owners, but stops
using a flattened assistant-history string at the selected built-in adapter
boundary. The provider-command adapter follows the same canonical conversation
contract. Only the adapter emits vendor role
fields, `reasoning_content`, signed thinking blocks, or other provider-native
request data.

## Replay Rules

The adapter receives ordered canonical conversation plus current turn shape. It
evaluates each replay record as follows:

| Disposition | Adapter behavior |
| --- | --- |
| `required` | Validate same adapter id and canonical text, then encode the record in the provider-required position. |
| `omitted` | Keep local reasoning and final text, but omit reasoning from this request. |
| `forbidden` | Do not emit reasoning for this provider. A record marked required for this request shape is a continuation failure. |

The DeepSeek V4 adapter selects `omitted` for every later request, including a
reasoning-bearing assistant message followed by tool results. It serializes an
empty `content` field for an assistant tool-call message because DeepSeek's
strict decoder requires the key, while native `tool_calls` remain structured.
The kernel does not let user input, desktop state, or a tool argument override
that decision.

The first Phase C `zai-glm` / `glm-5.2` binding uses the same bounded canonical
text rather than opaque state. It sends `thinking={type:enabled,
clear_thinking:true}` for an ordinary next user turn and omits earlier reasoning
from that request. When an assistant tool-call message is continued by its tool
result, it requires same-binding canonical reasoning, emits that
`reasoning_content` before `tool_calls`, and sends
`thinking={type:enabled, clear_thinking:false}`. Missing, changed, reordered,
or cross-binding reasoning fails locally. The binding is vendor-contract
identity (`zai-glm`), while the configured route remains the independent
OpenCode Go provider route.

## Failure Semantics

- Inbound vendor reasoning without a mapping is `provider_vendor_field_unsupported`.
- Invalid canonical reasoning or replay state is `provider_reasoning_invalid`.
- Required canonical reasoning missing after a GLM tool response or a mismatched
  `zai-glm` binding has the same nonretryable reason and makes no HTTP request.
- A forbidden reasoning field in an outbound request is prevented locally; the
  provider is not contacted.
- Unknown provider-command response fields remain rejected. A command adapter
  maps vendor fields before its strict stdout protocol boundary.

No failure path deletes an already committed user-visible reasoning message.

## Permission And Authority

Reasoning messages are model output reduced by the kernel into transcript facts.
Users can read their session projections. The model cannot supply a reasoning
id, adapter id, disposition, or replay state in `turn.submit` or tool arguments.
Adapter configuration comes from the active profile and stays outside
model-visible schemas.

## Recovery And Observability

SQLite ledger replay rebuilds reasoning messages before their associated tool or
final projections. Restart does not contact a provider to recover old reasoning.

The timeline exposes reasoning id, text, creation time, and turn relationship.
It does not expose adapter replay state. Realtime stream events may carry
reasoning deltas to desktop, but the settled durable message is the only restart
source.

Phase C adds context-inspection reason codes for selected or suppressed
reasoning ids. It does not include opaque replay-state bytes.

## Rejected Alternatives

- A global `hidden_reasoning_policy` is rejected because provider rules can
  differ by tool-call path within one profile.
- Dropping reasoning in `provider_command` is rejected because it loses a
  user-visible assistant message before the kernel can persist it.
- A generic map of raw vendor fields in session projections is rejected because
  it exposes unstable protocol data and makes the kernel a vendor payload store.
- Replaying persisted reasoning as plain user text is rejected because it
  changes role and order semantics and violates providers that forbid the field.
- Letting desktop decide replay is rejected because restart and provider switch
  would create competing context truth.
