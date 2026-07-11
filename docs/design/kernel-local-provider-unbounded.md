# Design: Explicit Unbounded Local Provider Requests

- **Requirement:** `docs/requirements/kernel-local-provider-unbounded.md`.
- **Owner:** Model Gateway configuration resolution and provider-command
  transport.

## Reference Scan

Codex's app server obtains a per-thread configuration snapshot before it starts
a turn in `codex-rs/app-server/src/request_processors/thread_processor.rs` and
uses that snapshot's context when constructing its runtime. Reasonix creates a
controller from the selected session configuration in
`reasonix/internal/cli/acp.go`; its ACP test proves the selected session cwd is
the owner input, rather than a process-wide default. The relevant lesson is
that a long-running operation must be controlled by the selected runtime
configuration and a cancellable owner context, not by a UI-side timer.

Genesis differs deliberately: it does not inherit a broad unbounded mode. The
selected provider-command route is the only owner that may declare it.

## Configuration And Boundary

`models.json` gains one route/gateway field named
`allow_unbounded_request`. It is valid only for a `provider_command` route.
The field defaults to false. `request_timeout_sec` retains its existing
positive-number meaning; an explicit zero is accepted only when
`allow_unbounded_request=true` on the selected provider command.

`ResolvedProviderConfig` carries this resolved semantic as a boolean on
`ProviderCommandConfig`, rather than leaking raw JSON into the provider. The
OpenAI-compatible configuration has no unbounded field.

## Data Flow

```text
models.json selected route
  -> ResolveProviderConfigFromGenesis validates explicit local opt-in
  -> ProviderCommandConfig{AllowUnboundedRequest}
  -> CommandProvider.Complete uses caller ctx directly
  -> llama_cpp_provider_command.py --timeout-sec 0
  -> llama.cpp /chat/completions
  -> strict provider-command response
  -> Model Gateway settles durable reasoning/final facts
```

For the normal path, `NewCommandProvider` keeps the existing finite default and
creates a deadline child context. For the opt-in path it passes the supplied
context straight to `exec.CommandContext`; therefore the turn owner's
interruption signal still kills the owned adapter process.

`VerifyProviderLive` resolves configuration first. `Timeout == 0` means “use
the resolved policy”: it creates a background cancel context only when the
resolved command explicitly permits unbounded requests; otherwise it preserves
the finite verify default. A negative CLI value remains invalid.

The Python adapter uses `urlopen(request)` without a `timeout` argument only
when passed zero. Omitted arguments retain its finite 300-second default. It
continues to omit `max_tokens` entirely.

## Projection And Observability

No session, timeline, model-visible context, or provider response projection
contains the flag or a local path. The normal provider readiness projection
continues to expose only ready/not-ready and safe reason codes. No audit fact
is needed for a configured normal request.

## Failure And Recovery

The caller controls cancellation. On cancellation the command child is killed
by `exec.CommandContext`, and its response is not decoded or committed. On
restart, only already settled ledger facts are replayed; Genesis does not try
to resume a killed in-flight model process.

## Rejected Alternatives

- A very large default timeout is rejected: it is still an arbitrary kill
  switch and cannot express user intent.
- A global zero-is-unbounded convention is rejected: it would make cloud and
  unrelated command profiles unsafe by accident.
- A `max_tokens` default is rejected: it silently truncates a legitimate local
  answer.
- A desktop timer is rejected: desktop is not the provider execution owner.
