# Stable Provider Context Layering Implementation Plan

**Goal:** preserve a cache-stable system/tool prefix and role-correct variable
conversation in every provider projection.

**Architecture:** reuse `ModelRequest.Conversation` as Genesis's one semantic
conversation source. Build stable prefix and current user tail in the kernel;
adapters translate that ordered structure without rebuilding meaning.

**Red lines:** no raw vendor prompt ledger facts, no host paths in prefix
fingerprints, no change to reasoning replay semantics, no default output cap.

## Phase A: Canonical Context And Fingerprint

1. Write failing kernel tests that require one system prefix, a distinct final
   current-user message, and stable normalized fingerprint changes when a
   visible tool or adapter binding changes.
2. Add the minimal stable-prefix builder and opaque fingerprint projection to
   `ModelRequest`/context accounting. Keep `InputItems` only for inspection and
   direct legacy test callers.
3. Change conversation construction so skill index is system prefix content and
   dynamic context precedes—not merges with—the final user message.
4. Persist opaque component digests with model-context accounting, compare them
   with the prior session request, and project only changed reason names through
   context inspection. `role_policy` remains a later selected-binding-snapshot
   slice. Prove compaction preservation.

## Phase B: Adapter-Specific Projection

1. Write failing Python adapter tests for system/user/assistant/tool order and
   for latest user input after stable prefix/history.
2. Change llama.cpp provider command to use canonical conversation when present;
   retain its narrow legacy `InputItems` fallback only when conversation is
   absent.
3. Preserve provider-command reasoning and tool-call response translation.
4. Run DeepSeek/GLM/OpenAI-compatible reasoning continuation regressions.

## Phase C: Local Qwen Acceptance

1. Capture the actual llama.cpp payload through the adapter's safe debug test
   seam and prove a stable prefix plus separate final user message.
2. Run the configured no-cap local Qwen route on Project, Task, and Chat after
   user interruption is available for a genuinely non-terminating turn.
3. Prove prefix fingerprint is stable over equivalent turns and changes only
   when expected; prove compaction leaves the prefix intact.

## Verification

Focused kernel/adapter tests, then `go test ./... -count=1`, `go build ./...`,
adapter self-test, `git diff --check`, and a requirement/design/plan/issue/BDD
drift review.
