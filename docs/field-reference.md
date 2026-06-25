# Genesis Field Reference

This document records field meanings that are easy to confuse across provider responses, Genesis normalized evidence, and kernel control-plane projections.

Last verified: 2026-06-23.

## Rules

- Provider `usage` is the source of truth for model token accounting. Genesis must not estimate model tokens from text length, rune count, byte count, or local tokenizer guesses unless a provider explicitly supplies that tokenizer contract.
- DeepSeek/OpenAI-compatible token fields are request/exchange-level facts. They describe the whole provider request, not per-fragment or per-turn attribution.
- Kernel compaction is executed by the kernel compaction runner. Model Gateway provides provider context and usage/accounting evidence; shells, app servers, daemons, and provider adapters must not perform compaction themselves.
- Context cache is a provider optimization, not conversation memory. The caller still sends the prompt/context it wants the model to see.
- Realtime chunks are transport, not canonical facts. They become durable only after an owner reduces them to a transcript item, tool result, job fact, checkpoint, or failure evidence.

## Persistence Layers

| Layer | Stored by default | Examples | Not for |
| --- | --- | --- | --- |
| Realtime transport | No. Memory or connection only. | Token deltas, stdout chunks, progress frames, heartbeat frames. | Restart replay, audit, memory, provider context. |
| Session transcript | Yes. | User messages, final assistant-visible replies, model-visible tool calls, model-visible final tool results, approved reasoning summaries or notices. | Provider raw payloads, hidden reasoning, debug spans. |
| Kernel durable facts | Yes. | Checkpoints, operation status, permission denials, terminal job facts, compaction outcomes, memory review decisions, provider usage accounting. | Ordinary UI info logs or stream frames. |
| Security/control audit | Yes, low-noise. | Permission decisions, credential use, dangerous-operation decisions, control-plane writes, governance intake/publication, break-glass actions, security failures. | Ordinary tool success, routine provider calls, UI clicks. |
| Debug trace | Only when explicitly enabled. | Provider projection summaries, response summaries, internal spans, chunk-level diagnostics, gateway decisions. | Canonical replay, memory truth, audit decisions. |

Long-term fact admission is based on meaning, not log level. A normal `model.final` can be durable because it is transcript. A routine HTTP success info line should not be durable. A failure can be durable when it changes state, blocks work, or explains recovery.

## Provider Context Snapshot Fields

Production storage should keep derivation evidence for provider requests, not the full raw prompt by default.

| Field class | Meaning | Owner | Constraint |
| --- | --- | --- | --- |
| Included event refs | Ledger events used to build provider context. | Model Gateway projection. | References facts; does not duplicate full prompt text. |
| Model input kinds | Ordered context categories sent to the provider. | Model Gateway projection. | Category evidence only. |
| Tool manifest hash or refs | Identity of the model-visible tool manifest. | Tool Registry / Model Gateway. | Avoid storing repeated manifest bodies when a stable ref is enough. |
| Skill refs or hashes | Skill metadata selected for the turn. | Skill catalog projection. | No skill body or path by default. |
| Compaction summary ref or hash | Summary evidence included in provider context. | Kernel compaction runner. | The user timeline must not render the summary as chat content. |
| Gateway profile id | Provider gateway profile used for the request. | Model Gateway. | Not a credential and not a secret. |
| Normalized usage summary | Provider-backed token/cache accounting returned by the provider. | Model Gateway normalization. | Request-level evidence, not per-fragment attribution. |

## DeepSeek Usage Fields

Source: DeepSeek API docs for chat completions and context caching.

| Provider field | Meaning | Genesis interpretation | Do not use as |
| --- | --- | --- | --- |
| `usage.prompt_tokens` | Total input prompt tokens for this request. DeepSeek documents this as `prompt_cache_hit_tokens + prompt_cache_miss_tokens`. | `TokenUsage.InputTokens`. Use for context-window pressure and auto-compaction trigger thresholds. | Per-message, per-turn, or per-fragment token attribution. |
| `usage.completion_tokens` | Generated output tokens for this request. | `TokenUsage.OutputTokens`. | Assistant-visible text length only; some models may include reasoning-token accounting inside completion details. |
| `usage.total_tokens` | Total request tokens: prompt plus completion. | `TokenUsage.TotalTokens`. | A context-window input measure. Use `prompt_tokens` for input pressure. |
| `usage.prompt_cache_hit_tokens` | Prompt tokens served from DeepSeek context cache. | `TokenUsage.CacheHitTokens`. Use for cache-effectiveness inspection and economics. | Durable memory, skipped context, or proof that Genesis can omit those tokens from the request. |
| `usage.prompt_cache_miss_tokens` | Prompt tokens not served from cache and freshly processed by the provider. | `TokenUsage.CacheMissTokens`; also the first provider-backed `processed_input_tokens` source for `model.context.accounted`. | Per-turn fresh-token attribution. It is still request/exchange-level. |
| `usage.prompt_tokens_details.cached_tokens` | OpenAI-style cached prompt-token detail. In live DeepSeek probes it matched `prompt_cache_hit_tokens`. | Fallback source for `TokenUsage.CacheHitTokens` when explicit DeepSeek cache-hit field is absent. | Preferred DeepSeek field when `prompt_cache_hit_tokens` is present. |
| `usage.completion_tokens_details.reasoning_tokens` | Reasoning-token breakdown inside completion tokens when the provider reports it. | Inspection-only for now. Genesis does not currently store a dedicated normalized field. | Assistant final text token count or prompt token count. |

Observed live probe with `deepseek-v4-flash`:

| Request shape | `prompt_tokens` | `prompt_cache_hit_tokens` | `prompt_cache_miss_tokens` | `completion_tokens` | `total_tokens` |
| --- | ---: | ---: | ---: | ---: | ---: |
| Short repeated prompt, attempt 1 | 36 | 0 | 36 | 8 | 44 |
| Short repeated prompt, attempt 2 | 36 | 0 | 36 | 8 | 44 |
| Long shared prefix, attempt 1 | 2672 | 0 | 2672 | 8 | 2680 |
| Long shared prefix, attempt 2 | 2672 | 2560 | 112 | 8 | 2680 |
| Long shared prefix, attempt 3 | 2672 | 2560 | 112 | 8 | 2680 |

## Genesis Normalized Fields

| Genesis field | Stored on | Source | Meaning | Owner |
| --- | --- | --- | --- | --- |
| `TokenUsage.InputTokens` | `model.final.usage`, `context.compaction.usage`, `model.context.accounted.usage` | Provider `prompt_tokens`, or provider-specific input token field when using a non-OpenAI-compatible adapter. | Total provider input tokens for the request/exchange. | Model Gateway normalization. |
| `TokenUsage.OutputTokens` | Same as above | Provider `completion_tokens`, or provider-specific output token field. | Total provider output/completion tokens for the request/exchange. | Model Gateway normalization. |
| `TokenUsage.TotalTokens` | Same as above | Provider `total_tokens`, or `input + output` when provider omits total but supplies both sides. | Total provider tokens for the request/exchange. | Model Gateway normalization. |
| `TokenUsage.CacheHitTokens` | Same as above | DeepSeek `prompt_cache_hit_tokens`; fallback to `prompt_tokens_details.cached_tokens` when explicit hit field is absent. | Input tokens served from provider cache. | Model Gateway normalization. |
| `TokenUsage.CacheMissTokens` | Same as above | DeepSeek `prompt_cache_miss_tokens`; fallback to `input_tokens - cache_hit_tokens` only when the explicit miss field is absent and the arithmetic is valid. | Input tokens freshly processed by the provider. | Model Gateway normalization. |

## Model Input Kinds

`model_input_kinds` records categories of provider-visible context fragments. It is category evidence, not the full rendered provider prompt.

| Kind | Source | Meaning | Owner |
| --- | --- | --- | --- |
| `conversation_history_context` | Ledger completed turns in the same session. | Same-session user, assistant, and model-visible tool call/result history selected for the provider request. | Model Gateway projection. |
| `skill_index_context` | Skill catalog metadata. | Budgeted skill names and descriptions only; no skill bodies or paths. | Model Gateway projection. |
| `approved_memory_context` | Approved memory recalls. | User-approved accumulation facts selected for this turn. | Accumulation plus Model Gateway projection. |
| `kernel_observation_context` | Undelivered kernel observation facts such as terminal job events. | System facts delivered at a provider checkpoint and marked delivered only after provider success. | Interface Kernel / Model Gateway projection. |
| `user_text` | Current `turn.submit` input item. | User or external-application text content. | Interface Kernel admission. |

## Permission Profile Fields

`permission_mode` is the user-facing trust mode. It is not the executor contract by itself. Before admission, Genesis resolves it into separate control-plane facts:

| Field | Meaning | Current values | Owner |
| --- | --- | --- | --- |
| `permission_mode` | User-facing trust mode selected by kernel startup/configuration. | `plan`, `default`, `yolo`. | Tool System authority gate. |
| `authority_policy` | Effect-admission policy derived from `permission_mode`. | `read_only`, `workspace_write`, `full_access`, or `unknown` for fail-closed diagnostics. | Tool System authority gate. |
| `sandbox_profile` | Actual execution isolation profile selected by the kernel. | `read_only`, `controlled_workspace`, `os_workspace`, `host`, or `none` for fail-closed diagnostics. `os_workspace` is a stronger future profile and currently fails closed when no executor can enforce it. | Tool System authority gate. |
| `approval_policy` | Whether an otherwise admissible write effect requires approval before execution. | `never`, `on_request`. `on_request` creates a kernel-owned pending approval for the frozen effect request, returns structured `approval_required` feedback, and executes only after an accepted approval decision. | Tool System authority gate. |

`controlled_workspace` is not an OS-level sandbox claim. It means the current default executor uses Genesis-owned command parsing, workspace containment, and link checks instead of invoking the host shell. Provider-visible tool results must not include these control-plane fields.

## Model Context Accounting

`model.context.accounted` is ledger evidence written after a provider response with usage. It records which provider-context exchange produced the usage, so compaction can use provider-backed facts without owning provider tokenization.

| Genesis field | Meaning | Source | Constraint |
| --- | --- | --- | --- |
| `model_context_accounting.round_index` | Provider call round inside the turn loop. | Kernel turn loop. | Control-plane evidence; not model-visible input. |
| `model_context_accounting.model` | Provider model that returned the usage. | Provider response. | Informational; not a routing source of truth. |
| `model_context_accounting.model_input_kinds` | Ordered context categories sent to provider, such as conversation history and user text. | Provider context projection. | Category evidence only; not full rendered prompt text. |
| `model_context_accounting.history_turn_ids` | Completed same-session turns included in the conversation-history context for that provider request. | Provider context projection. | Complete-turn boundary evidence, not token attribution. |
| `model_context_accounting.compacted_through_turn_id` | Last turn already represented by the latest compaction summary when this context was built. | Kernel compaction projection. | Empty means no prior completed compaction boundary. |
| `model_context_accounting.usage` | Normalized provider usage for this provider request. | Provider response via Model Gateway. | Request/exchange-level token fact. |
| `model_context_accounting.processed_input_tokens` | Provider-backed freshly processed input tokens for this exchange. | `TokenUsage.CacheMissTokens` when present. | May drive cache-aware compaction heuristics, but does not identify which fragment caused the miss. |
| `model_context_accounting.processed_input_token_source` | Evidence source for `processed_input_tokens`. | Currently `prompt_cache_miss_tokens` for DeepSeek/OpenAI-compatible usage. | Must name a provider-backed source, not a local estimate. |
| `model_context_accounting.tool_round_count` | Number of model-visible tool rounds included in this provider context exchange. | Provider context projection. | Boundary evidence only; not an operation count. |
| `model_context_accounting.tool_call_count` | Number of model-visible tool calls included in this provider context exchange. | Provider context projection. | Counts calls sent back to the provider, not direct HTTP tool invocations outside the turn loop. |
| `model_context_accounting.tool_result_count` | Number of model-visible tool results included in this provider context exchange. | Provider context projection. | Used to verify completed tool call/result pairs are preserved across context operations. |

## Compaction Fields

| Field | Meaning | Owner | Notes |
| --- | --- | --- | --- |
| `ContextPolicy.ContextWindowTokens` | Configured model context window. `0` disables automatic compaction. | Kernel config. | Threshold input, not provider usage. |
| `ContextPolicy.AutoCompactRatio` | Fraction of the context window that triggers automatic compaction. | Kernel config. | Applied to provider-reported `InputTokens`. |
| `ContextPolicy.RecentTurnLimit` | Minimum number of complete recent conversation turns kept verbatim after compaction. | Kernel compaction runner. | Complete-turn floor. |
| `ContextPolicy.RecentTailTokens` | Optional provider-backed processed input token budget for keeping additional recent complete turns. | Kernel compaction runner consuming Model Gateway accounting. | Must only consume `model.context.accounted` evidence. A missing accounting record stops expansion instead of estimating. |
| `ContextPolicy.RetryBackoffTurns` | Number of completed turns to wait after a compaction summarizer failure before retrying. | Kernel compaction runner. | Prevents immediate repeated summarizer calls after a failure. |
| `context_compaction.source_input_tokens` | Provider input token count that triggered compaction. | Kernel compaction runner. | Comes from the final response usage that triggered auto compaction. |
| `context_compaction.source_usage` | Full normalized usage from the provider exchange that triggered compaction. | Kernel compaction runner consuming Model Gateway-normalized final usage. | Triggering-turn usage, not summarizer usage. |
| `context_compaction.usage` | Provider usage for the summarizer call. | Kernel compaction runner via Model Gateway response. | This is summarizer usage, not the original user turn usage. |
| `context_compaction.compacted_through_turn_id` | Last completed conversation turn folded into the summary. | Kernel compaction runner. | Future provider contexts omit raw turns up to this boundary and include the summary instead. |
| `context_compaction.compacted_turn_count` | Number of completed conversation turns folded into the summary. | Kernel compaction runner. | Does not count current user input or tool rounds. |
| `context_compaction.cache_stability` | Aggregated cache hit/miss inspection facts for the compacted region. | Kernel compaction runner consuming `model.context.accounted` usage. | Diagnostic economics only; not memory and not per-fragment attribution. |
| `context_compaction.previous_failure_reason` | Prior summarizer failure reason copied into a deferred retry event. | Kernel compaction runner. | Inspection evidence for why compaction was deferred. |
| `context_compaction.retry_after_completed_turns` | Backoff policy recorded on running, failed, deferred, and completed compaction events. | Kernel compaction runner. | Policy evidence for retry scheduling. |
| `context_compaction.backoff_remaining_turns` | Remaining completed-turn wait count when a compaction attempt is deferred. | Kernel compaction runner. | Applies only to deferred events. |

## Cache Stability Fields

`context_compaction.cache_stability` summarizes provider cache behavior for completed turns that are folded into a compaction summary. It is deliberately coarse because provider usage is request-level evidence.

| Field | Meaning | Constraint |
| --- | --- | --- |
| `samples` | Number of compacted turns with provider cache hit/miss evidence. | May be lower than `compacted_turn_count` when usage was unavailable. |
| `cache_hit_tokens` | Sum of provider cache-hit input tokens across sampled compacted turns. | Provider-backed total, not durable memory. |
| `cache_miss_tokens` | Sum of provider cache-miss input tokens across sampled compacted turns. | Provider-backed total, not per-message attribution. |
| `hit_rate_per_mille` | Aggregate cache hit rate in per-mille units. | Avoids floating-point drift in ledger evidence. |
| `first_hit_rate_per_mille` | Cache hit rate for the first sampled compacted turn. | Used only for trend comparison. |
| `latest_hit_rate_per_mille` | Cache hit rate for the latest sampled compacted turn. | Used only for trend comparison. |
| `latest_cache_hit_tokens` | Latest sampled turn cache-hit tokens. | Request-level fact. |
| `latest_cache_miss_tokens` | Latest sampled turn cache-miss tokens. | Request-level fact. |
| `trend` | `warming`, `cooling`, `stable`, or `unknown`. | Coarse diagnostic label derived from sampled hit-rate movement. |

## Common Mistakes

| Mistake | Correct reading |
| --- | --- |
| Treating `prompt_cache_hit_tokens` as memory recall. | It is provider cache reuse for input prompt tokens. Genesis still sends the context. |
| Treating `prompt_cache_miss_tokens` as "new user message tokens". | It is request-level freshly processed prompt tokens. It may include system text, history, skill index, memory context, and current user text. |
| Using local text length to fill `RecentTailTokens`. | Do not. `RecentTailTokens` consumes provider-backed accounting only. |
| Letting Model Gateway compact history because it has usage data. | Do not. Model Gateway supplies usage/accounting evidence; kernel compaction runner executes compaction. |
| Letting WebUI, CLI, or a daemon summarize old turns. | Do not. They may submit a typed kernel command, but only the kernel runner writes compaction evidence and changes future context projection. |

## References

- DeepSeek Context Caching: https://api-docs.deepseek.com/guides/kv_cache
- DeepSeek Chat Completion API usage schema: https://api-docs.deepseek.com/api/create-chat-completion
- DeepSeek Token & Token Usage: https://api-docs.deepseek.com/quick_start/token_usage
