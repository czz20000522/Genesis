# Application Issue Ledger

This file records active issues for user-space applications that exercise the
Genesis Kernel. Kernel primitive gaps belong in
`docs/operations/kernel-issues.md`.

## Ledger Rules

- Application issues must cite an approved application requirement and design
  unless they are obvious bugs or test gaps.
- Do not record kernel work here. If a gap requires a new kernel primitive,
  create or update the relevant kernel requirement/design/issue.
- Completed issues leave this ledger and move to application retirement evidence
  when such a log is needed.
- Issues should stay small: requirement, design, gap, next slice, evidence,
  verification, and reference alignment.

## Active Issues

### APP-MESSAGE-INGRESS-FEISHU-LISTENER-20260623 - P2 - Feishu inbound listener and adapter retry hardening

- Status: open.
- Requirement: `docs/applications/user-space-message-ingress-runtime-requirement.md`.
- Design: `docs/applications/user-space-message-ingress-runtime-design.md`.
- Gap: Phase A only proves one-shot Feishu inbound envelope submission. It does not run a durable Feishu event listener, verify callback signatures, refresh adapter tokens, or apply inbound retry/backoff policy.
- Next slice: Add a Feishu listener/poller that emits `ChannelMessage` envelopes and keeps signature/token/retry state in adapter-local storage.
- Evidence: Phase A plan explicitly excludes long-running Feishu listener production hardening.
- Verification: A repeated Feishu event must dedupe before kernel turn submission; inbound retry exhaustion must only affect application inbox/submission status.
- Reference alignment: Aligned with Reasonix ACP keeping protocol/session handling outside the controller and Codex app-server keeping client transport ids outside core turn truth.

### APP-MESSAGE-INGRESS-OPERATOR-CONSOLE-20260623 - P2 - Operator console inspection projection

- Status: open.
- Requirement: `docs/applications/user-space-message-ingress-runtime-requirement.md`.
- Design: `docs/applications/user-space-message-ingress-runtime-design.md`.
- Gap: Phase A does not provide the operator console views for capabilities, session timeline, raw events, audit, memory review, job status, or provider context inspection. It only creates the ingress relay needed to submit messages.
- Next slice: Add console inspection commands that read kernel projections and render them without interpreting raw events as application truth.
- Evidence: Phase A plan keeps operator console inspection as future work.
- Verification: Console inspection must fetch kernel projections through HTTP and must not import kernel internals or reconstruct provider context locally.
- Reference alignment: Aligned with Reasonix `serve/wire.go` projecting internal events to wire shape without becoming event truth owner.

### APP-MESSAGE-INGRESS-OUTBOUND-SKILL-PACK-20260623 - P2 - External channel outbound skill pack

- Status: open.
- Requirement: `docs/applications/user-space-message-ingress-runtime-requirement.md`.
- Design: `docs/applications/user-space-message-ingress-runtime-design.md`.
- Gap: The ingress runtime can pass Feishu reply references to the LLM, but a polished application path still needs skill/CLI capability packs that teach the LLM how to call `lark-cli`, mail CLI, and future channel CLIs through kernel-governed tools.
- Next slice: Add or curate user-space skills and smoke instructions for Feishu outbound replies without adding channel-specific kernel packages or gateway reply APIs.
- Evidence: The approved design rejects automatic external-channel reply delivery from ingress runtime.
- Verification: A live Feishu smoke should show inbound message entry through ingress and outbound reply produced only by LLM-requested `shell_exec` using lark-cli.
- Reference alignment: Aligned with the Genesis boundary that external domain actions are skills/CLIs, not kernel owners.
