# Kernel Issue Ledger

This file is the repo-owned ledger for active Genesis Kernel issues. Feishu Base is the collaboration inbox; this file is the durable project source for issues that still need code, verification, or user acceptance.

Retired issues must not remain here. Move accepted retirements to `docs/operations/kernel-retirement-log.md` with the fixing commits, verification evidence, residual risks, and retirement reason.

## Ledger Rules

- Keep only `new`, `open`, `in_progress`, or otherwise active issues in this file.
- Do not record application-specific feature work as kernel work unless it changes a kernel primitive.
- Do not add versioned HTTP route names as current contracts. HTTP is transport; current kernel endpoints are unversioned.
- `ready_for_acceptance` issues move to the retirement log as retirement candidates and leave this active ledger.
- Feishu/Base links may point to collaboration artifacts, but this repo must contain enough evidence to understand the current status without opening Feishu.
- Every active `KERNEL-*` issue must include a `Reference alignment` field that compares the issue to Codex, Reasonix, or an explicitly rejected drift risk.

## Active Issues

### KERNEL-SESSION-EVENT-STREAM-UNIFICATION-20260622 - P1 - Session facts should converge on typed event stream

- Status: new.
- Type: architecture.
- Problem: The ledger has events, but turn, operation, work, and candidate identities are still projected through parallel object-specific paths. Tool calls and operation results are indirectly related rather than causally connected as `tool.call` and `tool.result` events.
- Suggestion: Define session event stream as the primary fact model. Use typed events for provider steps, tool calls, tool results, jobs, and resources; projections remain read models derived from the stream.
- Evidence: Feishu Base record `recvnfToaIqgPW`.
- Verification: Architecture doc defines typed session event stream as the fact source; tool execution writes `tool.call` and `tool.result` with result causality; provider replay and `/sessions/{id}` can derive ordered events; existing object projections are either derived read models or have a deletion plan.
- Reference alignment: Codex protocol surfaces ordered events and tool call/result relationships explicitly. Reasonix controller-style flows keep lifecycle facts behind one control surface. Genesis should not let session truth fragment into unrelated projections as kernel responsibilities grow.

### KERNEL-PROVIDER-COMMAND-ADAPTER-20260622 - P1 - Provider should prefer external command adapter boundary

- Status: new.
- Type: architecture.
- Problem: The kernel currently contains an OpenAI-compatible HTTP provider implementation, including provider-native JSON, HTTP errors, and tool-name translation. That risks pulling provider SDK and vendor protocol details into the kernel.
- Suggestion: Add a `provider_command` contract where the kernel writes canonical typed context/events to stdin and reads typed provider events from stdout. The OpenAI-compatible implementation can move behind an external adapter or remain explicitly experimental, while the kernel owns only typed provider events.
- Evidence: Feishu Base record `recvnfTtk7quMe`.
- Verification: Contract docs define provider command stdin/stdout events and provider-step boundaries; a fake command adapter can complete final text and one tool loop smoke; any retained OpenAI-compatible adapter is not the default kernel contract.
- Reference alignment: Codex isolates provider protocol handling behind its model client/protocol layer and keeps tool loop semantics typed. Reasonix supports provider/plugin style boundaries. Genesis should not bake one provider's HTTP JSON as the kernel's long-lived provider contract.
