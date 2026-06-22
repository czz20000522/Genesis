# Kernel Issue Ledger

This file is the repo-owned ledger for active Genesis Kernel issues. Feishu Base is the collaboration inbox; this file is the durable project source for issues that still need code, verification, or user acceptance.

Retired issues must not remain here. Move accepted retirements to `docs/operations/kernel-retirement-log.md` with the fixing commits, verification evidence, residual risks, and retirement reason.

## Ledger Rules

- Keep only `new`, `open`, `in_progress`, or otherwise active issues in this file.
- Do not record application-specific feature work as kernel work unless it changes a kernel primitive.
- Do not add versioned HTTP route names as current contracts. HTTP is transport; current kernel endpoints are unversioned.
- `ready_for_acceptance` issues move to the retirement log as retirement candidates and leave this active ledger.
- Feishu/Base links may point to collaboration artifacts, but this repo must contain enough evidence to understand the current status without opening Feishu.

## Active Issues

### KERNEL-BOUNDARY-SEMANTIC-TEXT-20260622

- Priority: P0
- Area: Architecture boundary / admission validation
- Status: in_progress
- Title: Semantic text must not be rejected by secret-shaped heuristics
- Problem: Architecture review found `validateKernelTextNotSecret` applied to narrative fields such as work title, cancel reason, memory approval/rejection/supersession reason, and similar user/operator text. This makes the kernel a content judge and repeats the same drift as the rejected unsupported-tool-name sanitizer: the kernel tries to infer whether arbitrary local text is secret/path-like instead of preserving model/user semantic content.
- Reference alignment: Codex returns model-repair feedback or terminal-equivalent tool results instead of heuristically hiding model-generated strings; Reasonix feeds malformed tool errors/schema back to the model and keeps permission as a separate gate. Neither pattern makes the runtime reject ordinary narrative text because it looks secret-shaped.
- Expected behavior: Control-plane identifiers, refs, authorities, and transport schema remain grammar-gated. Ordinary semantic text is admitted as text; projection/redaction policy may still apply to tool output, skill bodies, credential records, and protected inspection surfaces.
- Verification: Add behavior tests proving secret-shaped narrative text is admitted in work and memory review flows, plus an architecture test proving narrative fields do not call the secret-shaped rejector.

### KERNEL-BOUNDARY-TOOL-REGISTRY-20260622

- Priority: P0
- Area: Tool System / registry boundary
- Status: in_progress
- Title: Tool descriptors are still hardcoded in the kernel instead of owned by a registry
- Problem: `modelToolDescriptors` and `toolCapabilityKind` currently encode the tool surface directly in `internal/kernel`. This works for the spike but diverges from Reasonix's `Tool` interface + per-run `Registry` and Codex's `ToolExecutor` model, where executable runtime and model-visible spec stay tied through a tool owner.
- Reference alignment: Reasonix registers built-ins and plugin tools into a runtime registry; Codex tools implement a shared executor contract with exposure metadata. Genesis should converge on a small kernel tool registry even if the first registry only contains `shell.exec` and `skill.read`.
- Expected behavior: Adding a tool requires one registry entry with descriptor, kind/read-only/effect classification, permission profile, and handler binding. Provider adapters and capability inspection project from that registry, not from parallel switches.
- Current progress: The current spike has a minimal `kernelToolDefinition` registry that binds descriptor, read/effect kind, and prepare handler for `shell.exec` and `skill.read`; model descriptors, capability projection, and model tool preflight now project from that registry.
- Verification: Contract tests fail if a model-visible tool has no registry kind, if capability projection diverges from descriptors, or if application-specific names such as Feishu/email/calendar enter kernel descriptors.

### KERNEL-BOUNDARY-PERMISSION-GATE-20260622

- Priority: P0
- Area: Authority Plane / Tool System
- Status: in_progress
- Title: Permission policy is embedded in shell execution instead of a generic gate
- Problem: `plan/default/yolo` policy currently lives mostly inside `shell.go`. This couples authority semantics to one tool and makes future tools likely to duplicate permission logic.
- Reference alignment: Reasonix separates pure permission `Policy` and optional `Gate` from tool implementation; Codex separates approval/sandbox policy from concrete exec result handling. Genesis needs the same shape: tool execution asks a generic gate before effects.
- Expected behavior: Tool handlers expose read/effect classification and requested effect metadata; a kernel authority gate returns allow/block/reason before execution. Shell remains one handler under that gate, not the owner of permission semantics.
- Current progress: A minimal `authorizeKernelTool` gate now allows read tools, blocks effect tools in `plan`, allows effect tools in `default/yolo`, and fails closed for unknown tool kinds or permission modes. `shell.exec` calls this generic gate before shell-specific workspace/default execution planning.
- Verification: Tests prove plan mode blocks all effect tools through the same gate, read-only tools can proceed, blocked calls return model-visible `operation.blocked` or equivalent repair feedback, and shell-specific code does not own global permission vocabulary.

### KERNEL-BOUNDARY-SHELL-MINI-RUNTIME-20260622

- Priority: P1
- Area: Tool System / shell execution
- Status: new
- Title: Default shell mode risks becoming a mini shell implementation
- Problem: `shell.go` parses a growing subset of commands and implements controlled `echo/printf/set-content/cat/pwd` behavior. That protects the workspace but risks pushing Genesis from a harness runtime into a shell interpreter if each new command is added inside the kernel.
- Reference alignment: Codex routes exec through one sandboxed execution path and reports terminal output; Reasonix treats `bash` as a tool under permission/sandbox gates. Genesis should avoid growing command semantics inside the kernel beyond the minimal safety adapter.
- Expected behavior: Controlled default mode remains deliberately tiny or is replaced by a generic sandbox/process adapter. New application capabilities must arrive through skills/CLIs/daemons, not new kernel command parsers.
- Verification: Contract tests document the current controlled command allowlist, forbid application command aliases in `shell.go`, and require any expansion to reference the Tool Registry and Permission Gate issue.

### KERNEL-BOUNDARY-REFERENCE-ALIGNMENT-20260622

- Priority: P1
- Area: Architecture governance
- Status: new
- Title: Kernel changes need reference-alignment notes against Codex and Reasonix
- Problem: The unsupported-tool-name drift was caught by product review, not by implementation review. Without a repeatable reference-alignment gate, Genesis can keep adding features that resemble Codex/Reasonix names while violating their underlying design ideas.
- Reference alignment: Codex is the reference for terminal-equivalent tool results, approval/sandbox rigor, and protocol separation. Reasonix is the reference for registry-driven provider/tool/plugin loading and frontend-agnostic control. Genesis must compare ideas, not maturity.
- Expected behavior: Each non-trivial kernel feature or boundary change records whether it is aligned with Codex, aligned with Reasonix, intentionally different, or a known drift risk with follow-up issue.
- Verification: Issue and retirement records include a `Reference alignment` paragraph for kernel boundary changes; architecture tests cover the most important recurring drift classes rather than relying on ad hoc review memory.
