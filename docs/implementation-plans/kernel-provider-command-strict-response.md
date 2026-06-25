# Implementation Plan: Provider Command Strict Response

## Requirement And Design

- Requirement: `docs/requirements/kernel-foundation-capabilities.md`
- Design: `docs/design/kernel-foundation-capabilities.md`
- Issue: `KERNEL-PROVIDER-COMMAND-STRICT-RESPONSE-20260625`

## Reference Scan

- Codex references inspected: `codex-rs/core/src/tools/handlers/multi_agents_v2/*.rs` uses `serde(deny_unknown_fields)` for internal tool command payloads; `codex-rs/core/src/config/config_loader_tests.rs` has strict-config unknown-field tests; tool specs can remain non-strict where the provider API requires it.
- Reasonix references inspected: `internal/installsource/*` and `internal/tool` keep internal request parsing strict where Reasonix owns the protocol, while provider adapters translate vendor-native payloads behind the provider boundary.
- Alignment: Genesis-owned `provider_command` is an internal typed protocol like those command/tool surfaces, so unknown response fields should fail closed.
- Intentional differences: OpenAI-compatible provider decoding remains tolerant of vendor-native extras because that boundary translates external provider JSON into Genesis model responses.
- Drift risks or follow-up issues: Do not make `ModelToolCall.UnmarshalJSON` globally strict if that would reject vendor adapter data before translation; strictness should be applied at the provider_command boundary.

## Reference Behavior Red Tests

| Reference behavior | Genesis equivalent | Test file | Initial red condition | Intentional difference |
| --- | --- | --- | --- | --- |
| Codex internal command payloads reject unknown fields | `provider_command` final response rejects unknown top-level fields | `internal/kernel/provider_command_test.go` | `json.Unmarshal` silently drops `extra` | Vendor-native OpenAI-compatible responses are still tolerant |
| Codex multi-agent tool payloads reject unknown control fields | `provider_command` tool call rejects `lease_id` / `budget_lease_id` / arbitrary unknown fields before `tool.call` admission | `internal/kernel/provider_command_test.go` | nested `ModelToolCall` silently drops unknown fields | Model-visible tool argument validation remains separate |
| Reasonix typed tool protocols expose only owned fields | provider_command request still omits kernel-owned ids and valid final/tool responses still work | `internal/kernel/provider_command_test.go` | existing positive tests remain in central file | Move only new tests in this slice |

## Phase A

- Deliverable: Strict decoder for `providerCommandResponse` and nested provider-command tool calls.
- Red lines:
  - Do not make OpenAI-compatible vendor response decoding strict.
  - Do not expose hidden control-plane fields to provider commands.
  - Do not write `tool.call` facts or execute tool effects after provider-command shape failure.
- Tests:
  - Unknown top-level final response field fails.
  - Tool call `lease_id`, `budget_lease_id`, and arbitrary unknown field fail.
  - Valid final and valid tool-call responses still work.
  - OpenAI-compatible response with irrelevant vendor-native fields remains accepted.
- Evidence:
  - Focused provider-command tests.
  - `go test ./internal/kernel -count=1`.
- Still short of production:
  - Streaming provider command protocol is not designed.

## Retirement Criteria

`KERNEL-PROVIDER-COMMAND-STRICT-RESPONSE-20260625` can retire when the strict provider-command response tests pass, existing provider-command positive paths still pass, and OpenAI-compatible tolerance for vendor extras remains covered.
