# Requirement: Explicit Unbounded Local Provider Requests

- **Status:** approved for phased implementation.
- **Owner:** Genesis Kernel Model Gateway and the selected local provider
  command adapter.
- **Scope:** permit a deliberately configured local `provider_command` request
  to run without a generated-output cap or an outer deadline while retaining
  caller-driven interruption and every existing bounded default elsewhere.

## Background

The configured llama.cpp Qwen model can legitimately spend many minutes
producing visible reasoning and a final answer. A default `max_tokens` would
truncate a valid answer. The current command provider, live-verification path,
and llama.cpp Python adapter each independently impose a timeout, so the
kernel can kill a healthy local request despite the user not asking to stop it.

## Production Target

An explicitly marked local provider-command route may have no provider-output
ceiling and no deadline introduced by Genesis or its command adapter. A user
interrupt, daemon shutdown, or caller-context cancellation still ends the
request and its owned subprocess. Missing configuration remains bounded by the
existing defaults; cloud routes cannot accidentally inherit this behavior.

## Semantics

1. The config distinguishes an omitted timeout from an explicit unbounded
   local command request; zero must never silently mean both.
2. Only the `provider_command` route selected by an explicit local opt-in may
   omit the command deadline. Existing provider-command and all
   OpenAI-compatible defaults remain bounded.
3. Genesis must not add `max_tokens` to this adapter's llama.cpp request.
4. In unbounded mode, the provider command is created with the caller context,
   not a replacement deadline context. `/interrupt`, caller cancellation, and
   shutdown remain effective.
5. The llama.cpp adapter interprets its explicit zero timeout as no urllib
   deadline. Its omitted setting keeps the current finite default.
6. Provider verification may request an unbounded probe only when the resolved
   route itself declares the local opt-in. A CLI `--timeout-sec 0` cannot make a
   cloud route unbounded.
7. Response-shape and diagnostic byte limits remain independent safety limits;
   this requirement removes neither strict response validation nor bounded
   captured diagnostics.

## Failure Semantics

- A zero or negative timeout on a route without the explicit local opt-in is
  invalid configuration, not an unbounded request.
- An interrupted unbounded request reports the existing cancellation outcome;
  it must not be reported as a timeout.
- A malformed command response, unavailable llama.cpp server, or failed child
  process remains a normal provider failure.
- The user may manually stop the desktop-owned llama.cpp process; the waiting
  turn then returns the corresponding provider failure without corrupting its
  session transcript.

## Non-Goals

- No global removal of provider deadlines.
- No automatic output length policy, hidden `max_tokens`, background retry, or
  detached provider process.
- No change to shell/job timeouts, tool budgets, context-window limits, or
  cloud-provider timeout policy.

## Acceptance Criteria

1. An explicitly configured local Qwen provider command receives no generated
   `max_tokens` and runs past the former deadline until it settles or is
   explicitly interrupted.
2. An ordinary provider-command route still uses its finite default deadline.
3. A cloud route remains bounded even if a user passes a zero CLI verify value.
4. An interrupt cancels an unbounded command child rather than waiting for it
   indefinitely.
5. The configured local first-run acceptance completes a real turn, restarts,
   and replays the durable final/reasoning projection without a timeout cap.
