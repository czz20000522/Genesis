# Local Provider Unbounded Request Implementation Plan

**Goal:** let the explicitly configured local llama.cpp provider run until it
settles or is cancelled, without introducing an output ceiling or weakening
cloud-provider deadlines.

**Architecture:** add one explicit provider-command configuration flag,
validate it while resolving the selected profile, and carry it through the
command provider, verification command, and llama.cpp adapter. The caller
context remains the cancellation owner in unbounded mode.

**Red lines:** do not add `max_tokens`; do not make zero globally unbounded;
do not alter tool/job timeout policy; do not detach or discover model processes.

## Reference Summary

Codex starts a thread from a configuration snapshot before declaring its
runtime ready. Reasonix's ACP factory builds one controller using the selected
session parameters and tests that selected configuration. Genesis follows the
same “resolved owner configuration first” principle, but keeps the opt-in
strictly to a local `provider_command` route.

## Phase A: Contract And Kernel Command Transport

1. Add failing resolver tests in `internal/kernel/model_config_test.go` proving
   that `request_timeout_sec: 0` is rejected without
   `allow_unbounded_request`, accepted only for provider command with the flag,
   and never applies to OpenAI-compatible routes.
2. Add failing command-provider tests in
   `internal/kernel/provider_command_test.go`: an explicit unbounded provider
   survives a short caller-free interval, then returns cancellation after the
   caller context is cancelled; an ordinary provider still times out.
3. Add `AllowUnboundedRequest` to `ProviderCommandConfig`, resolve and
   validate `allow_unbounded_request` in `internal/kernel/model_config.go`,
   and use the supplied context directly only for that configuration.
4. Run the focused resolver and command-provider tests.

## Phase B: Verification And Adapter Boundary

1. Add failing tests in `internal/kernel/provider_verify_test.go` and
   `cmd/genesisctl/main_test.go` proving verify timeout zero is allowed only
   after resolving an explicitly unbounded provider command, and remains
   rejected/effectively bounded for cloud routes.
2. Make `VerifyProviderLive` choose its context after resolution and update
   `genesisctl provider verify` help/validation so zero is an explicit
   configured-local request, while negative values remain invalid.
3. Add Python self-test coverage in `scripts/providers/llama_cpp_provider_command.py`
   for explicit zero using the no-timeout code path and retaining absence of
   `max_tokens`.
4. Update the configured local Qwen route in the user configuration only after
   the tests prove the contract.

## Phase C: Real Acceptance And Drift Gate

1. Run `scripts/first_run_live_llm_acceptance.ps1` against the configured
   local Qwen route with its explicit unbounded verify option; allow it to run
   until the model settles.
2. Restart the daemon against the same ledger and verify final plus reasoning
   replay.
3. Run focused tests, `go test ./... -count=1`, `go build ./...`, desktop
   tests/build as applicable, adapter self-test, and `git diff --check`.
4. Compare the diff against this plan, requirement, design, the active kernel
   issue, and the first-run application issue; move only closed evidence out
   of active ledgers.
