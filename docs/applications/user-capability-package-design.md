# Design: User Capability Package

- **Owner:** user-space capability runtime
- **Status:** draft

## Purpose

Genesis should make small user-defined tools easy to organize, discover, run,
and migrate without absorbing their domain logic into the kernel.

Examples include video transcript extraction, code intelligence, paper operator
helpers, local report generators, and future one-off utilities. These are user
capabilities, not Genesis kernel capabilities.

## User Home Boundary

`~/.genesis` is the Genesis user home. It stores user-owned runtime data and
assets for the installed Genesis product, not development artifacts from the
Genesis source repository.

Current and intended top-level meanings:

```text
~/.genesis/
  config/          Genesis configuration, provider profiles, policies
  credentials/     protected local credentials and secret refs
  models/          local model assets used by Genesis or user capabilities
  accumulation/    user memory, knowledge, and long-term accumulated context
  runtime/         Genesis-owned runtime state
  logs/            Genesis runtime logs
  skills/          user skill root
  tools/           local tool assets already managed by the user

  capabilities/    user-space capability packages
  environments/    shared reusable execution environments, if needed
```

`runtime/` is reserved for Genesis runtime state. Do not use it as a name for
shared Python, Node, WSL, or container environments. Use `environments/` if a
shared packaged environment becomes necessary.

## Capability Package Layout

Each user capability is a self-contained directory under
`~/.genesis/capabilities/<capability-id>`.

```text
~/.genesis/capabilities/
  video-transcript/
    genesis.capability.json
    SKILL.md
    pyproject.toml
    scripts/
    video_transcript/
    data/
      outputs/
      cache/
      logs/
      state/
```

The package contains code, a manifest, and optional model-facing instructions.
Its own outputs, cache, logs, and state live under the package's `data/`
directory unless a specific owner later promotes some result into a Genesis
resource, memory, transcript, or audit fact.

This keeps capabilities portable without scattering their operational data
across the machine.

## Manifest

The minimum manifest describes how Genesis can inspect and invoke the package.

```json
{
  "id": "video-transcript",
  "name": "视频字幕提取",
  "description": "从抖音或 Bilibili 链接生成字幕文件。",
  "runtime_ref": "python-asr",
  "entrypoint": "scripts/video-transcript.ps1",
  "skill": "SKILL.md",
  "data_dir": "data",
  "inputs": ["url", "share_text"],
  "outputs": ["srt", "txt", "json"]
}
```

The manifest is for system discovery and health checks. `SKILL.md` is for model
guidance: when to use the capability, what inputs are valid, what command shape
is expected, and how to interpret failures.

Skills and capabilities remain orthogonal:

- installing a skill does not grant tool authority;
- installing a capability does not automatically inject full instructions into
  every turn;
- the kernel still decides tool execution authority through generic primitives.

## Environment Boundary

Capability packages should prefer code plus manifest, not vendored heavyweight
environments.

If several capabilities share expensive dependencies, Genesis may provide a
shared environment under:

```text
~/.genesis/environments/
  python-asr/
  node-playwright/
```

The capability manifest references the environment by `runtime_ref`. The
environment can be rebuilt on a new machine from a documented recipe. Large
assets such as ASR models belong in `~/.genesis/models`, not inside every
capability package.

Do not introduce per-capability virtual environments, containers, or package
managers until real dependency conflicts require them.

## Migration

The first migration unit is:

```text
~/.genesis/capabilities/
~/.genesis/config/
~/.genesis/credentials/   # machine/user-protected; may need re-auth or re-seal
~/.genesis/models/        # optional if models can be redownloaded
~/.genesis/accumulation/
```

`~/.genesis/environments` should be rebuildable. `data/` under a capability is
copied only when the user wants to keep that capability's outputs, cache, or
state.

## Non-goals

- No plugin marketplace.
- No scanning the whole disk for tools.
- No kernel package manager.
- No domain-specific kernel APIs for each small capability.
- No automatic promotion of capability outputs into memory or resources.

## Boundary Rule

Genesis can organize and run external user capabilities, but the capability's
domain logic stays outside the kernel. The stable contract is:

```text
User request
  -> skill/capability discovery
  -> generic governed execution
  -> capability-local data
  -> optional explicit promotion into Genesis-owned facts
```

If a capability needs credentials, provider context, memory truth, audit truth,
or long-running recovery, it must go through the existing Genesis owner for that
concern instead of storing hidden authority in the capability package.
