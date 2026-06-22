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

### KERNEL-TOOL-GATEWAY-REGISTRY-20260622 - P1 - Runtime should execute tools only through ToolGateway

- Status: new.
- Type: architecture.
- Problem: The current registry is still a static definition table. The model loop prepares shell and skill calls directly, and HTTP has a shell-specific route. Runtime code therefore still knows concrete shell/skill execution paths instead of depending on a single gateway.
- Suggestion: Introduce a `ToolGateway` / `ToolRegistry.resolve` boundary. Provider tool events should flow through gateway resolution, schema validation, policy, execution, and tool-result event emission. Shell should be one registered tool, not a runtime special case.
- Evidence: Feishu Base record `recvnfUG4qm0mu`.
- Verification: Runtime/model loop calls only the gateway for tool execution; unregistered tools return structured model repair feedback when a provider tool-call slot exists; generated tool manifest comes from the registry; `go test -count=1 ./...` and build pass.
- Reference alignment: Codex keeps tool execution behind governed runtime/tool abstractions rather than direct model-loop shell calls. Reasonix exposes tools through registry/plugin descriptors rather than hardcoding each tool into the controller.

### KERNEL-TOOL-NAMING-UNDERSCORE-20260622 - P1 - Canonical tool ids should not keep dotted names

- Status: new.
- Type: architecture.
- Problem: The kernel currently treats `shell.exec` and `skill.read` as canonical ids while provider adapters translate them to `shell_exec` and `skill_read`. This creates two active names for the same tool and makes adapter compatibility a permanent contract surface.
- Suggestion: Choose one canonical tool id shape. Current Base feedback recommends underscore ids, at least `shell_exec`, across registry, capability projection, ledger/session events, provider tools, docs, tests, and HTTP route naming policy.
- Evidence: Feishu Base record `recvnfTd7Nf0yF`.
- Verification: Active contracts, registry, capability projection, provider requests, ledger/session event tool fields, and tests use the single canonical id. Dotted ids appear only as rejected/retirement history. `go test -count=1 ./...` passes.
- Reference alignment: Codex tool names are provider-safe identifiers such as `exec_command` and `apply_patch`; Reasonix uses names such as `read_file`, `write_file`, and `bash_output`. Genesis should not preserve a second dotted kernel name when provider-safe ids work as the canonical contract.

### KERNEL-SKILL-READ-BOUNDARY-20260622 - P1 - `skill.read` should not be a first model-visible kernel tool

- Status: new.
- Type: architecture.
- Problem: `skill.read` exposes the skill package implementation detail as a default model-visible tool. That risks creating a family of specialized read tools (`read_skill`, `read_prompt`, `read_doc`) instead of keeping skills as user-space context assets and tools as actual governed capabilities.
- Suggestion: Remove `skill.read` / `skill_read` from the default first tool surface, or explicitly demote it to an internal experimental context hydration path. Keep the safe skill catalog summary visible, but do not treat full `SKILL.md` retrieval as a kernel syscall until a generic resource/context contract exists.
- Evidence: Feishu Base record `recvnfTiEnbXh3`.
- Verification: `/capabilities` and provider tool manifests no longer expose `skill.read` / `skill_read` as a default tool; skill catalog discovery still works; no Feishu/email/calendar/doc adapters are added as substitutes; tests prevent dedicated `read_skill`, `read_prompt`, or `read_doc` tools entering the default tool surface.
- Reference alignment: Codex does not expose a `read_skill` or `read_agents_md` tool to the model; Reasonix keeps skill/task concepts separate from ordinary file/process tools. Genesis should not make skill package loading itself a model tool unless it becomes a generic resource contract.

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
