# Design: Parent-Led Worker Runtime

- **Requirement:** `docs/requirements/kernel-parent-worker-runtime.md`
- **Owner:** Genesis Kernel agent orchestration boundary.

## 边界与所有者

内核拥有以下通用事实和规则：

- provider route、model catalog、model profile 和 role binding 的配置解析与投影；
- parent profile 与 worker role binding 的引用关系；
- worker invocation 的准入、grant 校验、leaf-only 限制和终态投影；
- task graph 的节点、边、状态、invocation link、调度约束和恢复；
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
  "tool_grants": ["resource_read", "source_read"],
  "context_policy_ref": "context:diff-plus-issue",
  "max_parallel": 4
}
```

Role binding 是准入输入，不是权限本身。内核运行时仍会把 `tool_grants` 降为 `CapabilityGrant` 并按 parent grant、ToolPolicy 和 ToolGateway 逐步校验。

### Parent Binding

Parent binding 定义 user-facing parent：

```json
{
  "parent_id": "coordinator",
  "profile_id": "frontier-parent",
  "allowed_worker_roles": ["local-small-worker", "code-review-worker"],
  "default_worker_role": "local-small-worker",
  "can_create_workers": true
}
```

对外命名使用 `parent` 或 `coordinator`，不使用 `foreground.coordinator` 这类前后台区分。前后台是应用展示问题，不是内核角色语义。

## 数据流

1. Operator 配置 provider route，并按需手动刷新 model catalog。
2. Operator 创建或更新 model profile。
3. Operator 创建 parent binding 和 worker role binding。
4. 用户向 parent 提交 turn。
5. Parent 生成 plan、worker invocation 请求或 task graph 请求。
6. 内核验证 parent 身份、role binding、grant 子集、leaf-only 和并发约束。
7. 内核创建 worker invocation，并按 profile/provider readiness 调用 Model Gateway。
8. Worker 如需工具，ToolGateway 按 invocation grant 校验。
9. Worker 返回 terminal result；内核记录状态、usage 和 sanitized failure。
10. Parent 汇总 worker result，可继续创建 review worker 或输出 final answer。

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
  "tool_grant_summary": ["resource_read", "source_read"],
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

Task graph 是可恢复的通用调度结构：

```json
{
  "graph_id": "taskgraph-1",
  "session_id": "session-a",
  "nodes": [
    {
      "node_id": "find-open-issues",
      "kind": "worker",
      "role_id": "local-small-worker",
      "status": "completed",
      "invocation_id": "invocation-1"
    }
  ],
  "edges": [
    {
      "from": "find-open-issues",
      "to": "fix-issue-1",
      "relation": "depends_on"
    }
  ]
}
```

节点状态至少包括 `pending`、`admitted`、`running`、`blocked`、`completed`、`failed`、`cancelled`。Graph replay 后必须恢复节点状态、依赖、invocation link 和可见失败原因。

Task graph 不决定业务含义。Parent 决定哪些 issue 独立、哪些纠缠、哪些暂时无法处理；内核只保存 parent 已提交的结构和执行状态。

## 权限模型

- 只有 parent binding 可以请求 worker invocation。
- Worker binding 必须 `leaf_only=true`，否则不进入 worker runtime。
- Worker 不获得 `can_create_workers`。
- Worker tool grant 必须是 parent grant 的子集，并受 ToolPolicy 约束。
- Task graph 节点不授予权限；节点只引用 role binding 和 invocation。
- Role label、prompt、模型输出、provider 名称都不授予权限。

## 并发与调度

调度 admission 使用最小上限：

- provider route `max_parallel`；
- model profile `max_parallel`；
- role binding `max_parallel`；
- parent 或 session 级预算上限；
- 当前 ToolPolicy 或 operator policy。

本地 llama.cpp profile 可以配置为 `max_parallel=1`。云端 profile 可以配置更高并发。内核不假设所有 provider 都能并行，也不把并发能力从 provider 名称中推断出来。

## 失败语义

- 缺少 role binding：`role_binding_missing`。
- 绑定 profile 缺失：`model_profile_missing`。
- provider 未 ready：`provider_not_ready`。
- provider/model/profile 并发已满：`parallel_limit_exceeded`。
- worker 请求创建 worker：`worker_delegation_denied`。
- grant 超出 parent：沿用 `capability_grant_exceeds_parent`。
- 工具被 policy 拒绝：沿用 ToolGateway failure class。
- task graph 有环：`task_graph_cycle`。
- 依赖节点失败导致等待节点无法执行：`dependency_failed`。
- worker 无法完成但需要用户或 parent 决策：`worker_blocked`。

失败不会把 provider credential、raw prompt、raw response body 或内部调度细节暴露给模型。

## 恢复与可观测性

持久事实应覆盖：

- role binding snapshot 或 config digest；
- worker invocation admitted / started / terminal；
- task graph submitted / node admitted / node terminal；
- provider readiness summary；
- usage 和 sanitized failure class。

投影面向三类读者：

- 用户：看到任务进展、阻塞、结果和必要证据；
- parent：看到 worker terminal result、review result、可继续调度的节点；
- operator：看到 provider/profile/role readiness、并发占用和配置错误。

Raw provider stream、worker 中间 transcript 和工具 trace 默认属于调试证据，不进入 parent conversation projection。需要持久化时必须先被归约为 transcript、evidence ref、audit fact 或 failure evidence。

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
