# Genesis Kernel

Genesis Kernel is a small local-first runtime for an LLM-driven agent environment.

It is not a web app, CLI app, Feishu adapter, coding agent, or desktop product. Those are shells or external applications. The kernel exposes stable task and tool contracts that any shell can call.

## Kernel Scope

The kernel owns only these planes:

- Interface Kernel: accept turns, normalize input, route sessions, emit events.
- Model Gateway: call configured model providers and project provider failures.
- Tool System: expose tools, enforce permission policy, execute effects, return evidence.
- WorkRegistry: persist work state, cancellation, recovery, and resumable execution records.
- Accumulation: persist memory candidates, approval state, recall records, and source refs.
- Auth/Credential Plane: protect runtime clients and resolve credential refs without leaking secrets.

## Out Of Scope

- CLI, WebUI, desktop UI, browser UI.
- Feishu, WeChat, email, calendar, document, OCR, or other application-specific logic.
- Skill bodies, product prompts, user workflows, and channel daemons.
- Project-specific assumptions from the previous Python implementation.

## Design Rule

External applications are user-space programs. The kernel may receive events from them and may let the active model call their CLIs through governed tools, but it must not become those applications.
