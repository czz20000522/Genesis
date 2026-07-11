# Feishu Profile Readiness Probe Implementation Plan

- **Requirement:** `docs/applications/connector-source-verification-lifecycle-requirement.md`.
- **Design:** `docs/applications/connector-source-verification-lifecycle-design.md`.
- **Issue:** `APP-CONNECTOR-FEISHU-LISTENER-20260623`.

## Reference Scan

Codex reads auth state inside its account processor and deliberately withholds
token material from ordinary status responses. Reasonix keeps cancellation and
readiness at its controller boundary rather than treating a UI signal as a
durable fact. Genesis reuses its generic `ProfileReadinessCommandProbe`: the
Feishu adapter translates `lark-cli auth status` into one bounded typed result;
the connector runtime still owns blocked state and never sees raw CLI output.

## Slice

Add a `--profile-probe` mode to the existing Feishu source adapter. It accepts
only the runtime-supplied profile and selected CLI executable, invokes
`lark-cli auth status`, and writes the generic typed result. Test bot-ready,
structured missing-profile, and unknown/error responses. Wire the documented
probe command into the non-listening `feishu-probe` acceptance command.

## Red Lines

- No token, raw auth payload, refresh action, QR flow, event stream, or message
  send.
- No mapping from readiness to source event verification.
- Unknown upstream status fails closed as `operator_action_required`; it does
  not guess expiry or permission semantics.

## Evidence

Run the profile probe against the installed `genesis` bot profile and the
missing-profile case, then run `genesis-ingress feishu-probe` with the new
profile probe command. Real inbound/outbound Feishu smoke remains a separate
operator event.
