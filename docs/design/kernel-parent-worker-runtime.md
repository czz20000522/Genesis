# Design: Parent-Led Worker Runtime

- **Requirement:** `docs/requirements/kernel-parent-worker-runtime.md`
- **Owner:** Genesis Kernel agent orchestration boundary.

## 边界与所有者

内核拥有以下通用事实和规则：

- provider route、model catalog、model profile 和 role binding 的配置解析与投影；
- parent profile 与 worker role binding 的引用关系；
- worker invocation 的准入、grant 校验、leaf-only 限制和终态投影；
- worker invocation 可被独立 task graph 引用的身份、状态和投影；
- provider/profile/role 并发上限的 admission 和 scheduling 约束；
- sanitized failure class、usage、readiness 和审计相关事实。

内核不拥有以下内容：

- 具体业务角色 taxonomy 和提示词；
- parent 如何拆解任务的策略；
- provider 原生 SDK、账号流、价格和余额；
- 长期记忆召回、语义清洁、代码理解策略；
- 桌面或 CLI 如何展示任务图。

Provider context 仍由 Model Gateway 组装。工具执行仍经过 ToolRegistry 和 ToolGateway。应用和 shell 只能提交 typed request，不直接写 ledger fact。

## 配置对象

### Provider Route

Provider route 描述一个模型服务入口：

```json
{
  "provider_id": "local-qwen",
  "protocol": "openai-chat-completions",
  "base_url": "http://127.0.0.1:8080/v1",
  "credential_ref": "secret:local-qwen",
  "max_parallel": 1
}
```

本地 llama.cpp 服务和云端 API 在内核语义上没有区别。差异只体现在 route、credential、readiness、上下文能力和并发上限。

### Model Catalog

Model catalog 是 `provider models refresh` 的结果。它是本地配置，不是 ledger truth。刷新成功后供 profile 绑定使用；刷新失败不改变现有 profile。

### Model Profile

Model profile 是 role 可以绑定的运行配置：

```json
{
  "profile_id": "local-qwen-worker",
  "provider_id": "local-qwen",
  "model_id": "Qwen-AgentWorld-35B-A3B-UD-Q4_K_M",
  "context_budget": 262144,
  "max_parallel": 1,
  "cache_policy": {
    "kv_quantization": "q8_0"
  }
}
```

`context_budget` 是 Genesis 可用上下文策略的一部分，不要求在 provider request 中写死 `max_tokens`。缓存量化、并发限制等本地运行参数可以作为 operator 配置进入 profile readiness，但 provider command 执行细节不进入模型可见 schema。

### Role Binding

Role binding 把 worker role 绑定到 profile 与能力：

```json
{
  "role_id": "code-review-worker",
  "profile_id": "cloud-reviewer",
  "leaf_only": true,
  "tool_set": ["resource_read", "source_read"],
  "context_policy_ref": "context:diff-plus-issue",
  "max_parallel": 4
}
```

Role binding 是准入输入，不是权限本身。内核运行时会把预设 `tool_set` 降为 `CapabilityGrant` 并按 ToolPolicy、parent 可创建范围和 ToolGateway 逐步校验。Parent 只能选择 role，不能在调用时给 worker 追加工具。

同一 `role_id` 可以同时创建多个 worker invocation。它们共享 role binding，但拥有不同 invocation id、任务输入、状态和输出；并发由 role/profile/provider 上限限制。`max_parallel` 省略或非正值时归一为 6；对独占本地模型，operator 必须显式配置 1。

### Parent Binding

Parent binding 定义 user-facing parent：

```json
{
  "parent_id": "coordinator",
  "profile_id": "frontier-parent",
  "allowed_worker_roles": ["local-small-worker", "code-review-worker"],
  "default_worker_role": "local-small-worker",
  "can_create_workers": true,
  "max_children": 24
}
```

对外命名使用 `parent` 或 `coordinator`，不使用 `foreground.coordinator` 这类前后台区分。前后台是应用展示问题，不是内核角色语义。

`max_children` 对一个 parent 的所有未终态 worker invocation 合并计数，不按 role 分开计算；省略或非正值时归一为 24。内核将 parent binding id 快照写入 worker invocation，因此重启后的准入仍可按原 parent 计数，不由应用临时状态决定。

## 数据流

1. Operator 配置 provider route，并按需手动刷新 model catalog。
2. Operator 创建或更新 model profile。
3. Operator 创建 parent binding 和 worker role binding。
4. 用户向 parent 提交 turn。
5. Parent 生成 plan 或 worker invocation 请求；task graph 请求由独立 task graph 需求定义。
6. 内核验证 parent 身份、role binding、预设工具集合、leaf-only、role 并发与 parent 子代理总量约束。
7. 内核创建 worker invocation，并按 profile/provider readiness 调用 Model Gateway。
8. Worker 如需工具，ToolGateway 按 invocation grant 校验。
9. Worker 返回 terminal result；内核记录状态、usage 和 sanitized failure。
10. Parent 汇总 worker result，可继续创建 review worker 或输出 final answer。

### Parent Dispatch Tool

`delegate_worker` 是 kernel-control tool，不是 shell、connector 或 workflow
command。它的 schema 固定为：

```json
{
  "role_id": "reviewer",
  "task": "Review the bounded worker result and state accept or reject."
}
```

内核以当前 daemon 的 configured parent id 解析 role binding，持久化
delegation 与 focused task，再创建 leaf invocation。工具调用不携带 profile、
tool set、workspace、fork history、credential 或 result channel；这些字段由
role binding 和 kernel-owned parent turn 绑定决定。worker gateway 永远不暴露
`delegate_worker`，因此 leaf worker 无法递归 delegation。

delegation 的即时 tool result 仅为 `queued`/`running` 与 invocation id。
terminal worker result 会作为同一 parent turn 的普通受限 tool result 回投；
parent 的下一次 provider continuation 只看 role、status、bounded final、usage
和 evidence refs。它不看 child raw prompt、reasoning、stream 或 tool trace。
review 是 role 选择；reduce 是 parent continuation，不创建新的 owner/type。

## 协议形状

### Role Binding Projection

投影应暴露语义摘要：

```json
{
  "role_id": "code-review-worker",
  "profile_id": "cloud-reviewer",
  "provider_id": "opencode",
  "model_id": "model-a",
  "ready": true,
  "leaf_only": true,
  "max_parallel": 4,
  "tool_set_summary": ["resource_read", "source_read"],
  "context_policy_ref": "context:diff-plus-issue"
}
```

投影不得暴露 credential ref、API key、raw headers、sandbox profile、permission profile、raw prompt 或 provider 原始响应。

### Worker Invocation Request

```json
{
  "session_id": "session-a",
  "parent_invocation_id": "invocation-parent",
  "role_id": "code-review-worker",
  "task": {
    "title": "review issue fix",
    "input_refs": ["source:diff-1"],
    "instructions": "检查补丁是否满足 issue"
  },
  "context_scope": "diff-plus-issue",
  "idempotency_key": "task-node-3-review"
}
```

内核根据 `role_id` 解析 profile 和 grant，然后调用现有 `AgentInvocation` 准入逻辑。`instructions` 是任务输入，不是权限来源。

### Worker Result

```json
{
  "invocation_id": "invocation-worker",
  "role_id": "code-review-worker",
  "status": "completed",
  "result": "补丁满足 issue，但缺少负例测试。",
  "usage": {
    "input_tokens": 1200,
    "output_tokens": 300
  },
  "evidence_refs": ["tool-result:source-read-7"]
}
```

失败结果使用结构化 failure class，例如 `role_binding_missing`、`provider_not_ready`、`parallel_limit_exceeded`、`worker_blocked`、`provider_failed`、`tool_grant_denied`。

## Task Graph

Task graph 单独建模。Parent-worker runtime 只承诺 worker invocation 暴露 `invocation_id`、`role_id`、`status`、`result`、`usage` 和 `evidence_refs`，让未来 task graph 可以引用并可视化这些 worker。

图节点、边、布局、并行调度、依赖状态机、恢复和可视化交互不在本设计中实现。

## 子代理可视化

Worker 输出可以像 parent 对话一样单独渲染为 child conversation projection。默认视图可以只显示 parent 对话，但运行时必须保留足够投影，让用户展开查看某个 worker 的输入、可见上下文摘要、输出、状态和证据引用。

这不是把 worker transcript 合并进 parent transcript。Parent conversation 只接收 parent final answer 或 parent 选择引用的 worker terminal result。

## 权限模型

- 只有 parent binding 可以请求 worker invocation。
- Worker binding 必须 `leaf_only=true`，否则不进入 worker runtime。
- Worker 不获得 `can_create_workers`。
- Worker 工具集合来自 role binding 预设，并受 ToolPolicy 与 ToolGateway 约束。
- worker gateway 不包含 `delegate_worker`，即使 parent gateway 包含它。
- Parent 不能在 worker invocation request 中追加 role binding 之外的工具。
- Task graph 节点不授予权限；节点只引用 role binding 和 invocation。
- Role label、prompt、模型输出、provider 名称都不授予权限。

## 并发与调度

调度 admission 使用最小上限：

- provider route `max_parallel`；
- model profile `max_parallel`；
- role binding `max_parallel`；
- parent binding `max_children`；
- parent 或 session 级预算上限；
- 当前 ToolPolicy 或 operator policy。

本地 llama.cpp profile 可以配置为 `max_parallel=1`。云端 profile 可以配置更高并发。Role 未配置上限时默认 6，parent 未配置总量时默认 24。内核不假设所有 provider 都能并行，也不把并发能力从 provider 名称中推断出来。

## 失败语义

- 缺少 role binding：`role_binding_missing`。
- 绑定 profile 缺失：`model_profile_missing`。
- provider 未 ready：`provider_not_ready`。
- provider/model/profile 并发已满：`parallel_limit_exceeded`。
- worker 请求创建 worker：`worker_delegation_denied`。
- role tool set 超出 parent 可创建范围：`capability_grant_exceeds_parent`。
- 工具被 policy 拒绝：沿用 ToolGateway failure class。
- worker 无法完成但需要用户或 parent 决策：`worker_blocked`。
- delegation 启动或恢复失败：`worker_delegation_failed`，并保持 parent turn
  的受限失败 tool result，不泄漏 provider 配置或 raw diagnostics。

失败不会把 provider credential、raw prompt、raw response body 或内部调度细节暴露给模型。

## 恢复与可观测性

持久事实应覆盖：

- role binding snapshot 或 config digest；
- worker invocation admitted / started / terminal；
- child conversation projection state；
- provider readiness summary；
- usage 和 sanitized failure class。

投影面向三类读者：

- 用户：看到任务进展、阻塞、结果和必要证据；
- parent：看到 worker terminal result、review result、可继续调度的节点；
- operator：看到 provider/profile/role readiness、并发占用和配置错误。

Raw provider stream 和工具 trace 默认属于调试证据，不进入 parent conversation projection。Worker 对话投影可以独立展示；需要进入 parent 对话时，必须先由 parent 归约为 final answer、evidence ref、audit fact 或 failure evidence。

Phase A 复用既有 `AgentInvocation` ledger fact 和 parent `tool.call` binding：
它持久化 worker identity、parent turn id、role/profile snapshot 与 child
terminal projection，并在当前 daemon 内异步启动一个 worker。它不恢复或重放
尚未终态的 worker，不建立任务图、批量 scheduler 或 worker-to-worker mailbox。
Phase B 必须把 focused task、parent tool-call binding 和安全 continuation
checkpoint 提升为可恢复状态，才可以在 daemon restart 后恢复；在该证据完成前
不得宣称 restart-safe delegation。

## 参考对齐

Genesis 已有 `AgentInvocation` 设计，保持以下对齐：

- child execution 是内核准入的 control operation；
- parent-child metadata 不能从自然语言推断；
- child tool set 必须过滤；
- parent 只接收 bounded final result；
- role/profile ref 是语义标签，不授予权限。

Provider model refresh 设计继续保持手动刷新：

- catalog refresh 不自动发生；
- profile binding 是显式操作；
- active binding 不因 catalog 刷新而改变。

## 拒绝的方案

- 单一全能模型：会把计划、生产、复查、权限和上下文全部混在一个 hot context 中，不利于任务图和恢复。
- Worker 递归创建 worker：会让权限继承、并发、失败恢复和记忆归因变得不可控。
- 每次运行动态拉取 provider model list：会引入网络不确定性，并让同一配置下的 role binding 不可复现。
- 由 role 名称决定工具权限：自然语言标签不是 authority source。
- 在本需求中设计完整 memory 系统：memory/context/accumulation 需要单独需求；这里只保留显式 context policy 接口。
