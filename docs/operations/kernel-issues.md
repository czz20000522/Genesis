# Kernel Issue Ledger

This file is the repo-owned ledger for active Genesis Kernel issues. Feishu Base is the collaboration inbox; this file is the durable project source for issues that still need code, verification, or user acceptance.

Retired issues must not remain here. Move accepted retirements to `docs/operations/kernel-retirement-log.md` with the fixing commits, verification evidence, residual risks, and retirement reason.

## Ledger Rules

- Keep only `new`, `open`, `in_progress`, or otherwise active issues in this file.
- Do not record application-specific feature work as kernel work unless it changes a kernel primitive.
- Do not add versioned HTTP route names as current contracts. HTTP is transport; current kernel endpoints are unversioned.
- `ready_for_acceptance` issues move to the retirement log as retirement candidates and leave this active ledger after acceptance.
- Feishu/Base links may point to collaboration artifacts, but this repo must contain enough evidence to understand the current status without opening Feishu.

## Active Issues

### recvndJWPu1RcN - P0 - Ingress security must not hard-reject ordinary user text

- Status: in_progress.
- Type: architecture.
- Problem: the current ingress security implementation hard-rejects user text that contains strings such as `tool_call_id`, `function_call`, role labels, XML role tags, or quoted prompt-injection samples. That mixes untrusted content with control-plane authority and prevents Genesis from analyzing logs, prompt documents, model-tool protocol traces, emails, web pages, or hostile prompt samples as normal user data.
- Expected behavior: user text is accepted as user/external data even when it contains prompt-injection or role-marker content. Such text may be recorded as ingress risk metadata, but it must not grant system, developer, tool, credential, or permission authority. Only malformed transport schema, unsupported input item types, hidden control text, real tool invocation requests, or attempts to set kernel-owned control fields fail closed.
- Verification required: `/turn` accepts ordinary text containing `tool_call_id`, `function_call`, `tool_calls`, `System:` log headings, XML/JSON role-marker samples, or quoted prompt-injection examples; session projection records risk metadata where applicable; nested unknown control fields still return 400 before ledger append; invisible-control text still returns 403 before ledger append; `go test ./...` and build pass; route version scan has no matches.
- Source: Feishu Base record `recvndJWPu1RcN`.
