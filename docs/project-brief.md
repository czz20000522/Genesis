# Genesis Project Brief

## One Line

Genesis is a local-first personal AI runtime that lets an LLM act as a useful
digital self through governed tools, memory, resources, sessions, and
user-space capabilities.

## What Genesis Is Building

Genesis is not just a chat UI and not a single-purpose agent. It is the governed
environment around an LLM:

- a kernel that owns authority, facts, tool execution, memory, credentials,
  event replay, and recovery;
- a desktop shell for everyday interaction;
- user-space connectors for external channels such as Feishu;
- user-space capability packages for personal tools such as local report
  generation;
- a `~/.genesis` user home for config, credentials, models, accumulation,
  skills, capabilities, and runtime state.

The core product idea is an agent kernel / harness runtime: the LLM can touch
local files, external CLIs, capability packages, mobile messages, and the
desktop shell only through governed boundaries.

The product goal is a practical personal digital self: something the user can
talk to, give files to, ask to use local tools, reach from mobile channels, and
trust to preserve evidence and boundaries.

## Why This Exists

Raw LLMs can reason, but they do not own stable execution boundaries. Without a
runtime, local tools scatter across the machine, credentials leak into prompts
or shell commands, session history is hard to replay, and external integrations
become one-off scripts.

Genesis exists to give the model a controlled home:

- the LLM owns semantic intent;
- the kernel owns authority, facts, recovery, and projection;
- applications and connectors own protocol translation;
- skills and capability packages teach the model how to use installed tools
  without turning each tool into kernel code.

## Target User

The primary user is a single local operator who wants a durable personal AI
assistant rather than a multi-user SaaS platform.

Secondary actors:

- the LLM, which proposes actions from bounded context;
- desktop and connector shells, which submit requests and render projections;
- external capability packages, which provide domain-specific user tools;
- development agents, which help build and verify the system but are not the
  product audience.

## Feature Roadmap

### P0: Usable Local Loop

- real provider configuration and readiness;
- session, turn, event, and timeline persistence;
- desktop chat shell over kernel HTTP;
- governed `shell_exec` and narrow read tools;
- skill catalog metadata;
- local credentials and provider setup;
- session debug capture;
- basic material/source intake;
- first user capability package through skill plus governed shell execution.

### P1: Daily Personal Assistant

- desktop chat usability: history, streaming, Markdown rendering, detail panel,
  attachment entry, and readable status;
- Feishu ingress and egress for mobile interaction;
- memory review, supersession, and a redesigned user-visible accumulation path;
- more real capability packages under `~/.genesis/capabilities`;
- clear movement between files, capability outputs, session context, and memory.

### P2: Production Control Surfaces

- capability list and health checks after multiple capabilities exist;
- connector source supervision and delivery recovery;
- long-running managed jobs;
- interactive approval surface;
- workflow runtime for developer-authored fixed flows;
- code intelligence adapter;
- stronger resource owner and artifact lifecycle.

### P3: Long-Term Digital Self

- multi-agent invocation with bounded capability grants;
- task graph and workflow cooperation;
- cloud or server "Genesis home" migration;
- capability and environment sync;
- self-review and learning loops based on accumulated evidence.

## Success Standards

Genesis is successful when:

- the user can open the desktop app and complete real tasks with a live model;
- sessions and important facts replay after restart;
- local files, external capability packages, and mobile messages can enter the
  same governed loop;
- the model can use tools but cannot grant itself authority;
- debug evidence explains poor model behavior without becoming normal
  transcript bloat;
- `~/.genesis` is enough to understand user-owned state, capabilities, models,
  credentials, and accumulation;
- new user tools can be added outside the kernel and used through skills plus
  governed execution.

## Non-Goals

- Genesis is not a multi-user SaaS product.
- Genesis is not a coding-agent product, though it can use coding tools.
- Genesis is not Feishu, email, calendar, OCR, document parsing, or any other
  domain app.
- Genesis does not add a kernel API for every small personal tool.
- Genesis does not treat README, issue ledgers, or implementation plans as the
  product definition source.

## Technical Baseline

- Kernel: Go.
- Kernel distribution target: local `genesisd` binary.
- Product distribution target: local desktop app experience; shells,
  connectors, and capabilities stay in user space.
- Session store: file-backed event frames with SQLite index/read model.
- Desktop: Wails + Vue/Vite + Go bridge over kernel HTTP.
- Provider boundary: Genesis model gateway, `provider_command`, and current
  OpenAI-compatible adapter support.
- Connector boundary: user-space `source_command` and `connector_command`
  adapters.
- User home: `~/.genesis`.
- User capabilities: `~/.genesis/capabilities/<id>` plus skill metadata under
  `~/.genesis/skills`.

## Canonical Sources

- Product definition and roadmap: this document.
- Kernel authority contract: `docs/kernel-contract.md`.
- Minimal kernel loop acceptance: `docs/minimal-closed-loop.md`.
- Development process and document roles: `docs/process.md`.
- Kernel requirements: `docs/requirements/`.
- Kernel designs: `docs/design/`.
- User-space application and capability designs: `docs/applications/`.
- Active issues and retirement evidence: `docs/operations/`.
