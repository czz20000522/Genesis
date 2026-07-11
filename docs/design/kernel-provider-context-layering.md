# Design: Stable Provider Context Prefix And Adapter Projection

- **Requirement:** `docs/requirements/kernel-provider-context-layering.md`.
- **Owner:** Model Gateway constructs semantic context; adapters translate it.

## Reference Scan

Reasonix resolves base prompt, language policy, memory, and metadata-only skill
index once in `internal/boot/boot.go:159-195`, then constructs a session whose
first message is `system` in `internal/agent/session.go:19-31`. It separately
hashes normalized system text and tools in
`internal/agent/cache_shape.go:36-57`; compaction explicitly retains that
prefix and rewrites only later messages in `internal/agent/compact.go:17-25`.

Codex's configuration distinguishes full-context and post-carried-prefix
compaction limits in `codex-rs/config/src/config_toml.rs:151-155`, preventing
the carried prefix from being mistaken for ordinary history. Genesis aligns
with the prefix/window separation, but keeps the ledger as truth rather than
persisting provider-native request bodies.

Genesis already builds canonical ordered `ModelConversationMessage` values in
`internal/kernel/kernel.go:984-1074`. Its OpenAI-compatible adapter consumes
them in `internal/kernel/openai_compatible.go:335-379`. The current llama.cpp
provider command is the drift: it flattens `InputItems` into one `user`
message and ignores `ModelRequest.Conversation`.

## Semantic Request Shape

```text
ProviderContextProjection
  -> StablePrefix { system text, normalized tool manifest, fingerprint }
  -> VariableWindow { summary, prior messages, dynamic context }
  -> CurrentTurn { latest user message }
  -> ModelRequest.Conversation + ModelRequest.ToolManifest
  -> adapter-specific native request
```

The stable prefix is reconstructed on every request from the current selected
configuration. It is not stored as a raw prompt. The variable window is rebuilt
from ledger events and compaction facts. The latest user input remains a
separate final `user` conversation message.

## Kernel Construction

`modelConversationMessagesFromStoredEvents` becomes the sole semantic builder:

1. prepend one `system` message from `stableSystemPrefix`, which contains the
   small Genesis operating contract and `skillIndexContext`;
2. append compaction summary and completed historical user/assistant/tool
   exchanges in their existing order;
3. append per-turn dynamic context with an explicit provenance label;
4. append the current input as a distinct `user` message;
5. append current-turn tool calls/results on later tool rounds.

`ModelInputItems` remains an inspection/accounting representation. It is no
longer a fallback source for production conversation construction. The skill
index does not appear in the current user message.

## Fingerprint And Inspection

This delivery adds path-free SHA-256 component digests to model-context
accounting: `system_instruction`, `skill_index`, normalized `tool_manifest`,
and non-secret `adapter_binding` (provider, adapter, profile, transport, and
model). Their combined digest remains the prefix fingerprint. Before recording
an accounting event, the kernel compares these component digests with the most
recent accounting event in that session and persists only the changed component
names; the first record is `initial`. Context inspection projects the combined
digest and reason names, never source text or component digests.

`role_policy` remains deliberately absent: there is no kernel-owned selected
role/context-policy snapshot to compare. Adding an invented runtime label would
misdescribe ledger history, so that reason waits for the binding owner.

Tool order is normalized only for fingerprinting. The execution manifest keeps
its existing ordering and authority.

## Adapter Projection

- OpenAI-compatible keeps its existing canonical-conversation projection.
- llama.cpp provider command emits the same ordered `Conversation` list as
  OpenAI-style `{role, content, tool_calls, tool_call_id}` objects, then emits
  its existing translated tools separately. It does not concatenate skill index
  and user input.
- An adapter lacking a system role must explicitly wrap the system prefix using
  a documented native template before variable messages; this is an adapter
  implementation decision, never a kernel fallback.
- DeepSeek keeps `reasoning_content` response-only; GLM replay remains bound to
  its documented adapter profile and is regression-tested unchanged.

## Compaction

The compactor selects only completed conversation turns and their tool
exchanges. It already excludes the session binding and tool manifest. This
slice adds a test proving the reconstructed system prefix and normalized tool
manifest are unchanged before and after compaction; summary placement is after
the system message and before retained variable tail.

## Recovery And Observability

On restart, prefix is regenerated from current selected configuration. A
changed selected adapter/tool/skill set naturally produces a new fingerprint;
the replayed conversation remains valid. A session debug export may include
fingerprint/reason labels but never the raw stable prefix or a host path.

## Rejected Alternatives

- Persist raw provider JSON: rejected because vendor wire format is adapter
  implementation, not kernel truth.
- Put all instructions in every user message: rejected because it destroys role
  semantics and stable-prefix caching.
- Move tools into a text system prompt: rejected because providers expose
  native tool fields and tool authority must remain structured.
