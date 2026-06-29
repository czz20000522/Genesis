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
  when such a log is needed. Retirement evidence is compact: one sentence with
  the retirement conclusion plus fixing commit evidence, not a repeated fix
  summary or verification transcript.
- Temporary or narrow implementation slices must name their retirement target.
  Once the approved primitive exists, retire the temporary package, tests, docs,
  and command references instead of preserving a second truth surface.
- Every application issue or next slice must name the kernel primitive or owner
  capability it pressure-tests. If the slice begins faking facts that belong to
  kernel, connector, memory, credential, permission, job, audit, or provider
  owners, stop the app slice and move the gap to the owning issue ledger.
- Issues should stay small: requirement, design, gap, next slice, evidence,
  verification, and reference alignment.

## Active Issues

### APP-CONNECTOR-FEISHU-LISTENER-20260623 - P2 - Connector source verification and lifecycle gate

- Status: open.
- GitHub mirror: #37 tracks this remaining source lifecycle production gap in
  the external issue queue; keep the detailed contract here until the lifecycle
  requirement/design changes.
- Requirement:
  `docs/applications/connector-source-verification-lifecycle-requirement.md`;
  broader connector contract:
  `docs/applications/application-connector-runtime-requirement.md`.
- Design: `docs/applications/connector-source-verification-lifecycle-design.md`;
  broader connector design:
  `docs/applications/application-connector-runtime-design.md`.
- Kernel/owner pressure: connector source verification, inbound dedupe, session
  mapping, turn submission, and connector outbox delivery without kernel Feishu
  ownership.
- Gap: The smoke path now has deterministic ExternalEvent NDJSON ingestion, a `source_command` typed streaming boundary for inbound source adapters, a Feishu source adapter command that owns `lark-cli event consume` and raw Feishu payload parsing, durable connector-local source failure records with redacted diagnostics, connector-local `SourceRun` / `SourceAttempt` / `SourceCursor` / `SourceVerificationEvidence` state, bounded generic retry/backoff for recoverable `source_command` process failures, connector-local operator lifecycle controls, an operator-run `genesis-ingress feishu-probe` source-command readiness report, and an opt-in `--deliver-final` path that turns kernel final text into one connector-owned send-message outbox item. Feishu CLI executable selection and Feishu probe composition now live in the Feishu application package/cmd layer rather than the generic connector runtime. Real source events still default to `unchecked` because source readiness does not imply event verification. Profile readiness blocks source start and final delivery with `missing_profile`, `profile_expired`, `permission_denied`, or `refresh_required`; probe mode reports readiness without starting source adapters, sending messages, or calling kernel; and `--profile-probe-command` can now classify profile posture through a typed connector-local command before source/delivery adapters start. Local verification of `lark-cli --profile codex event consume --help` shows no `--after-event-id` resume flag, so cursor-aware restart cannot be implemented by hardcoding a nonexistent Feishu CLI parameter. Remaining production gap is automatic refresh posture integration and production-grade source recovery beyond bounded retry/backoff.
- Next slice: Keep inbound source intake through `source_command` and keep Feishu event argv in the Feishu source adapter. Product smoke commands must pass the Genesis profile (`--profile genesis`) for the Genesis bot identity; Codex developer-originated test messages may still use the Codex profile as the sender. Future source verification work must add real event authenticity evidence rather than upgrading readiness to verification. Cursor resume must be adapter/driver capability-gated through an actual lark-cli parameter, adapter manifest, or upstream source protocol; until then the cursor is reliable dedupe/recovery evidence, not a promised replay control. Connector-specific reconciliation probes should wait until exact outbound action refs, idempotency keys, or external receipt refs exist; reconciliation remains outbound recovery work.
- Evidence: `genesis-ingress feishu-listen --stdin-jsonl ...` covers automated smoke and dedupe without a source process, with synthetic source validation remaining `unchecked` unless evidence exists. Non-stdin source intake now starts a configured `SourceCommandAdapter` process and consumes typed `source.ready`, `source.event`, `source.cursor`, `source.failed`, and `source.stopped` frames through the source command intake loop, which retries only recoverable runtime failures and does not retry blocked readiness/configuration failures. Verified source events require source/connector/adapter/event-bound evidence with an approved evidence kind before they are handled or recorded. Blocked/degraded source runs carry stable readiness reason codes for source command invalid/runtime failure while preserving operator-readable detail; profile readiness now also blocks source start or final delivery with `missing_profile`, `profile_expired`, `permission_denied`, or `refresh_required` before any external effect. Connector-local `source-clear-blocked`, `source-request-restart`, and `source-reset-cursor --accept-duplicate-risk` record operator source actions without creating kernel facts or fabricating ready state. `cmd/genesis-feishu-source-adapter` owns Feishu event command construction and raw Feishu payload parsing, requires an explicit `--profile`, and emits typed source frames instead of giving runtime raw payloads or Feishu command details. Malformed source frames and malformed Feishu payloads create redacted `SourceFailureRecord` entries before kernel submission. `FileInboundStore`, `FileSourceFailureStore`, `FileSourceLifecycleStore`, and `FileOutboxStore` serialize file-backed load-modify-write operations across process-local instances during smoke use. `genesis-console inspect --source-lifecycle-state ...` projects connector-local source runs, attempts, cursors, verification evidence, and operator actions without kernel mutation. `genesis-ingress feishu-probe --source-command ... --delivery-command ... --profile genesis` validates the source adapter process, profile readiness posture, and connector-command final-delivery surface without starting the listener, sending a message, or calling kernel; `--profile-probe-command` now allows typed connector-local profile posture classification, and regression tests prove probe-reported `profile_expired` / `refresh_required`, explicit `ready=false`, and timed-out probes block before source or delivery adapter effects. The opt-in `--deliver-final` path uses connector outbox delivery rather than kernel Feishu logic.
- Verification: Remaining verification must prove automatic refresh posture integration and production listener supervision beyond bounded retry/backoff without creating a kernel Feishu owner.
- Reference alignment: Aligned with Reasonix ACP keeping protocol/session handling outside the controller and Codex app-server keeping client transport ids outside core turn truth.
