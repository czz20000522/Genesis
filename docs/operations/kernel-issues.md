# Kernel Issue Ledger

This file is the repo-owned ledger for active Genesis Kernel issues. Feishu Base is the collaboration inbox; this file is the durable project source for issues that still need code, verification, or user acceptance.

Retired issues must not remain here. Move accepted retirements to `docs/operations/kernel-retirement-log.md` with the fixing commits, verification evidence, residual risks, and retirement reason.

## Ledger Rules

- Keep only `new`, `open`, `in_progress`, or otherwise active issues in this file.
- Do not record application-specific feature work as kernel work unless it changes a kernel primitive.
- Do not add versioned HTTP route names as current contracts. HTTP is transport; current kernel endpoints are unversioned.
- `ready_for_acceptance` issues move to the retirement log as retirement candidates and leave this active ledger.
- Feishu/Base links may point to collaboration artifacts, but this repo must contain enough evidence to understand the current status without opening Feishu.

## Active Issues

### KERNEL-TOOL-RESULT-TAXONOMY-20260622 - P0 - Tool loop must preserve terminal-equivalent command results

- Status: new.
- Problem: Genesis Kernel must distinguish three failure classes: invalid tool call requests, executed command failures, and tool/kernel infrastructure failures. Non-zero `shell.exec` exits are returned as `operation.failed`, but malformed model tool calls currently terminate the turn as `tool_call_rejected` rather than giving the model structured repair feedback when the protocol can still continue. Long stdout/stderr output is capped without truncation metadata, so the model cannot reliably understand whether it saw the whole terminal result.
- Suggestion: Define and implement a Tool System error taxonomy: `tool_request_invalid` for invalid arguments that produce no effect and can be returned to the model when a valid tool call id exists; `operation.blocked` for policy/permission rejection with no command execution; `operation.failed` for commands that executed and exited non-zero while preserving `exit_code`, stdout, and stderr; and `tool_infrastructure_failed` for ledger, shell runtime, provider adapter, or other kernel/tool infrastructure failures that must not be disguised as command stderr. Add controlled stdout/stderr presentation with head+tail truncation, truncation flags, and original size metadata.
- Evidence: `internal/kernel/shell.go` sets non-zero command exits to `operation.Status = "failed"` and returns the operation; `internal/kernel/model_tools.go` marshals operations back to the model as tool results; `internal/kernel/kernel.go` currently treats malformed model tool calls as a terminal turn failure; the current capped buffer keeps only a byte prefix and does not expose truncation state.
- Verification: A model-requested `shell.exec` command with interpreter/compiler syntax error returns a tool result containing `status=failed`, non-zero `exit_code`, stderr, and enough stdout/stderr metadata for the model to answer. A missing required argument or out-of-range tool argument does not execute effects and either returns structured repair feedback to the model or records a clearly unrecoverable protocol failure when the loop cannot safely continue. Long stdout/stderr returns head+tail content with `stdout_truncated`, `stderr_truncated`, and original byte or line counts. Simulated ledger/provider/shell infrastructure failures are not represented as ordinary command failures.
- Priority: P0.
