# Requirement: Parent-Led Worker Runtime

- **Status:** draft.
- **Owner:** Genesis Kernel agent orchestration boundary.
- **Scope:** define the production semantics for a parent-facing agent that delegates bounded work to role-bound leaf workers.

## 背景

Genesis 的长期形态不是一个模型独自完成所有工作，而是一个由核心大脑和多个工作代理组成的本地优先 AI 团队。用户只需要和一个对话入口交互；这个入口负责理解意图、拆解任务、分派工作、收敛结果、安排复查并给出最终答复。

现有内核已经有若干底层能力：模型 profile 配置、provider 模型目录手动刷新、工具注册、工具网关、会话、turn、以及 `AgentInvocation` 的准入事实。它们还没有形成一个稳定的生产语义：哪个模型是 parent，哪个模型适合某个 worker role，worker 能不能继续创建 worker，provider 并发上限如何影响调度，任务图如何成为可恢复的事实。

这份需求定义的是 Genesis 的通用多代理运行时边界。它不定义具体业务角色，也不把某个 provider、某个本地模型、某个记忆策略写死到内核。

## 生产目标

Genesis 支持一个 parent-led worker runtime：

- 用户只与一个 user-facing parent agent 对话；
- parent agent 负责计划、分派、归约、复核和最终答复，不直接承担默认生产劳动；
- worker agent 是叶子节点，只执行被分派的工作，不能再创建 worker；
- worker 的 role、model profile、provider route、预设工具集合、上下文策略和并发上限通过显式配置绑定；
- 一个 provider 可以有多个 model，model catalog 通过手动刷新得到，本地配置是后续运行的确定性输入；
- role binding 持久保存，一次配置后可重复使用；
- parent 创建 worker invocation 时只能选择已绑定的 role，不能凭自然语言临时获得额外权限；
- worker invocation 复用现有 `AgentInvocation` 准入语义，工具集合来自 role binding 预设，parent 只能选择 role，不能临时授予额外工具；
- provider 或 model profile 的并发限制参与调度，本地小模型可以配置为默认轻量 worker，但并非内核强制要求；
- parent 可将任务组织为 task graph，但 task graph 的完整建模、调度和可视化属于独立需求；
- worker 的输出可以像 parent 对话一样单独展示；它不会自动并入 parent 对话历史；
- parent-visible result 是 worker 的受限终态输出、状态、用量和必要证据摘要；
- runtime 记录足够事实以支持重放、恢复、审计和后续 review worker 复查；
- memory、context、accumulation 由后续需求接入，本需求只要求 worker context 必须显式、可解释、可限制。

## 用户与角色

用户：

- 向 parent agent 提出目标；
- 不需要知道具体 worker 数量或 provider 细节；
- 可以选择查看 worker 对话、任务进展、失败原因、最终结果和必要证据。

Parent agent：

- 是唯一默认 user-facing agent；
- 负责将用户目标拆成任务图或一组 worker invocation；
- 选择已配置 role binding；
- 收集 worker 结果，决定是否追加任务、复查或向用户答复；
- 不能绕过内核授权直接给 worker 工具、credential、workspace 或 provider 权限。

Worker agent：

- 由 parent 通过内核准入创建；
- 只能执行一个有边界的任务；
- 使用绑定 role 的 model profile、预设工具集合和上下文策略；
- 不能创建子 worker；
- 返回终态结果、阻塞原因或失败分类。

Operator：

- 配置 provider route、model catalog、model profile 和 role binding；
- 手动刷新 provider model catalog；
- 调整并发上限、默认 worker role 和预设工具集合。

Kernel：

- 拥有配置读取、role binding 解析、invocation 准入、grant 校验、task graph 事实、调度约束、投影和恢复；
- 不拥有业务角色 taxonomy、任务拆解策略、答案风格或 provider SDK 的业务细节。

Provider adapter / provider command：

- 执行具体 provider 协议；
- 不创建 ledger fact；
- 不决定 worker role 或预设工具集合。

## 核心语义

### Provider

Provider 是一个可调用模型服务边界。它可以是云端 OpenAI-compatible API，也可以是本地 llama.cpp 服务。内核只关心 provider route、credential ref、协议类型、readiness 和并发限制；provider 原生 SDK、账号流和私有 HTTP 细节属于 provider adapter 或 command 边界。

### Model Catalog

Model catalog 是 operator 手动刷新后持久化的本地模型列表。刷新不会自动改变 active binding，也不会在启动、ready check 或 turn submit 时自动发生。

### Model Profile

Model profile 是可运行模型配置：provider route、model id、上下文策略、默认参数、并发上限和可见能力摘要。它是 role binding 的目标，不等同于 provider catalog 中的裸 model id。

### Role Binding

Role binding 将一个 worker role 绑定到一个 model profile、一组预设工具、上下文策略和调度限制。Role label 本身不授予权限；权限来自内核准入后的 capability grant。允许同时存在多个相同 role 的 worker invocation；它们是不同 agent 实例，受 role/profile/provider 并发上限约束。

### Parent Profile

Parent profile 定义 user-facing parent agent 使用的模型配置和可请求的 orchestration 权限。Parent 可以创建 worker invocation，但 worker 不能获得相同的创建权限。

### Worker Invocation

Worker invocation 是一次叶子工作执行。它必须引用已存在的 role binding，并通过 `AgentInvocation` 准入。它接收 focused input 和显式 context scope，返回 bounded result。

### Task Graph

Task graph 是独立的通用工作结构需求。Parent-worker runtime 只需要保证 worker invocation 有可被 task graph 引用的身份、状态和可视化投影；节点、边、布局、调度和恢复规则不在本需求中展开。

## 非目标

- 不在本需求中设计长期记忆、语义召回、代码索引或 accumulation 策略。
- 不让 worker 递归创建 worker。
- 不让自然语言 role label 自动获得工具、credential、workspace 或 provider 权限。
- 不自动刷新 provider model catalog。
- 不在内核中写入 provider SDK、供应商账号流、价格查询、余额查询或模型 benchmark。
- 不定义固定的业务角色集合，例如 reviewer、coder、planner 的具体提示词。
- 不要求 parent 永远不能直接答复简单问题；本需求只定义生产工作默认应通过 worker 执行。
- 不在本需求中展开 task graph 的节点/边模型、调度器、布局或可视化交互。

## 分期交付

Phase A：Provider / Model Profile / Role Binding

- 读取并验证 provider route、model catalog、model profile 和 role binding；
- 支持手动 catalog refresh 后的 profile binding；
- 投影可见 role、model、provider readiness 和并发摘要；
- 不运行 worker。

Phase B：Parent 到 Worker 的准入与执行

- parent 可请求基于 role binding 的 worker invocation；
- worker 复用 `AgentInvocation` grant 校验；
- worker 不能创建子 worker；
- 返回 bounded result，不污染 parent transcript。

Phase C：Task Graph 接入边界

- worker invocation 暴露可供 task graph 引用的身份、状态和终态输出；
- 完整 task graph 需求独立建模；
- parent-worker runtime 不实现图布局、节点调度或图状态机。

Phase D：Review / Reduce

- parent 可安排 review worker 检查 worker 产物；
- parent 可基于 review result 追加、重试、合并或拒收节点；
- 投影区分 worker result、review result 和 parent final answer。

Phase E：Memory / Context / Accumulation 接入

- worker context 由显式策略组装；
- parent 可解释为什么某些历史、资源或代码上下文被注入；
- memory 仍是独立需求，不在本需求内落实现细节。

## 验收标准

1. 一个 provider 可以持久配置多个 model，并由 operator 手动刷新 catalog。
2. 一个 model profile 可以选择 catalog 中的某个 model，也可以指向本地 provider route。
3. 一个 role binding 可以绑定 model profile、预设工具集合、上下文策略和并发上限。
4. Parent 创建 worker 时只能选择已配置 role binding。
5. Worker invocation 的工具集合来自 role binding 预设，并经过内核准入校验。
6. Worker 不能创建 worker；任何递归 delegation 请求都被拒绝为权限错误。
7. 可以创建多个相同 role 的 worker agent 实例，直到触达 role/profile/provider 并发上限。
8. 本地单并发模型可以声明 `max_parallel=1`，调度不会并发占用该 profile。
9. 云端 provider/profile 可以声明更高并发，并留给独立 task graph 调度使用。
10. Worker 输出可以作为独立 child conversation projection 展示。
11. Worker 的中间 transcript 和工具 trace 不会自动进入 parent conversation projection。
12. Parent final answer 可以引用 worker terminal result 和 review result。
13. Provider credential、raw prompt、sandbox profile、permission profile 和内部 kernel id 不进入 model-visible role binding schema。

## 与现有文档的关系

- `docs/requirements/kernel-provider-model-refresh.md` 定义 provider model catalog 的手动刷新。
- `docs/requirements/kernel-agent-invocation.md` 定义 bounded child invocation 的准入和执行。
- 本需求把两者连接为 parent-led runtime 的生产目标，并为后续 task graph 与 memory/context 设计提供边界。
