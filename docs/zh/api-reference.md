# ClawFactory API 参考文档

## 概述

ClawFactory 通过 ARI（Agent Runtime Interface）HTTP REST 协议提供所有功能。所有接口使用 JSON 格式进行数据交换。

- 默认服务地址：`http://localhost:8080`
- 所有 `/v1/*` 接口需要 Bearer Token 认证
- 健康检查接口 `/health` 无需认证
- Prometheus 指标接口 `/metrics` 无需认证（v0.3 新增）
- 所有请求响应头包含 `X-Trace-ID`（v0.3 新增）

## 认证

除 `/health` 外，所有接口需要在 HTTP Header 中携带 API Token：

```
Authorization: Bearer <your-api-token>
```

默认开发 Token 为 `dev-token-001`，可在 `configs/config.json` 的 `api_tokens` 数组中配置多个 Token。

Token 无效或缺失时返回：

```json
{
  "error": {
    "code": "UNAUTHORIZED",
    "message": "invalid or missing API token"
  }
}
```

## 错误响应格式

所有接口的错误响应统一使用以下 JSON 格式：

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "错误描述信息"
  }
}
```

### 错误码一览

| HTTP 状态码 | 错误码 | 说明 |
|---|---|---|
| 400 | INVALID_REQUEST | 请求参数缺失或格式错误 |
| 401 | UNAUTHORIZED | API Token 无效或缺失 |
| 403 | FORBIDDEN | 权限不足 |
| 404 | NOT_FOUND | 资源不存在（智能体、任务、工作流） |
| 422 | INVALID_WORKFLOW | 工作流定义不合法（如包含环） |
| 500 | INTERNAL_ERROR | 服务器内部错误 |

---

## 智能体接口

### POST /v1/register — 注册智能体

将智能体注册到平台。相同 `name` + `version` 重复注册会返回已有的 `agent_id`（幂等性）。

**请求体：**

```json
{
  "name": "requirement-agent",
  "capabilities": ["requirement_analysis"],
  "version": "1.0.0"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| name | string | 是 | 智能体名称 |
| capabilities | string[] | 是 | 能力标签列表，用于任务匹配 |
| version | string | 是 | 版本号 |

**成功响应（200）：**

```json
{
  "agent_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

**错误响应（400）：**

缺少必填字段时返回 `INVALID_REQUEST`。

---

### POST /v1/heartbeat — 心跳上报

智能体定期发送心跳，告知平台自己仍然在线。建议每 30 秒发送一次。超过 90 秒未发送心跳的智能体会被标记为 `offline`。

**请求体：**

```json
{
  "agent_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| agent_id | string | 是 | 智能体 ID |

**成功响应（200）：**

```json
{
  "status": "ok"
}
```

**错误响应（404）：**

智能体不存在时返回 `NOT_FOUND`。

---

### GET /v1/tasks — 拉取任务

智能体主动拉取匹配自身能力的待执行任务。调度器会根据智能体的 `capabilities` 匹配任务队列中的任务。

**查询参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| agent_id | string | 是 | 智能体 ID |

**请求示例：**

```
GET /v1/tasks?agent_id=a1b2c3d4-e5f6-7890-abcd-ef1234567890
```

**有任务时的响应（200）：**

```json
{
  "task_id": "task-001",
  "workflow_id": "wf-001",
  "type": "requirement_analysis",
  "capabilities": ["requirement_analysis"],
  "input": {
    "user_requirement": "构建一个用户管理系统"
  },
  "assigned": true
}
```

**无任务时的响应（200）：**

```json
{
  "assigned": false
}
```

---

### POST /v1/tasks/{taskID}/status — 更新任务状态

智能体在执行任务过程中更新任务状态。

**路径参数：**

| 参数 | 说明 |
|------|------|
| taskID | 任务 ID |

**请求体：**

```json
{
  "agent_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "status": "completed",
  "output": {
    "requirement_doc": "# 需求文档\n..."
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| agent_id | string | 是 | 智能体 ID |
| status | string | 是 | 新状态：`running`、`completed`、`failed` |
| output | object | 否 | 任务产出物（键值对） |
| error | string | 否 | 失败原因（status 为 failed 时使用） |

**成功响应（200）：**

当 `status` 为 `running` 或 `completed` 时：

```json
{
  "status": "ok"
}
```

**失败重试行为（v0.2 新增）：**

当 `status` 为 `failed` 时，平台会自动调用策略引擎（`PolicyEngine.ShouldRetry`）判断是否需要重试：

- 如果 `retry_count < max_retries`（应重试）：任务状态改回 `pending` 重新入队，`retry_count` 递增 1，`assigned_to` 清空，返回：

```json
{
  "status": "retrying"
}
```

- 如果 `retry_count >= max_retries`（不应重试）：任务保持 `failed` 状态，触发 `WorkflowEngine.OnTaskPermanentlyFailed` 处理工作流级别的失败逻辑，返回：

```json
{
  "status": "ok"
}
```

**状态流转规则：**

```
pending → assigned → running → completed
                        ↓
                     failed ──→ pending (自动重试，retry_count < max_retries)
                        ↓
                permanently_failed (retry_count >= max_retries)
```

当 `status` 为 `completed` 时，平台会自动触发工作流引擎检查下游任务依赖。

重试入队后，任务的优先级、能力标签和输入数据保持不变，确保重试任务能被正确调度。最大重试次数通过 `configs/policy.json` 中的 `max_retries` 配置（默认 3 次）。

---

### POST /v1/authorize — 权限检查

智能体在执行敏感操作前请求权限验证。

**请求体：**

```json
{
  "agent_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "action": "call_tool",
  "resource": "llm_api"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| agent_id | string | 是 | 智能体 ID |
| action | string | 是 | 操作类型：`call_tool`、`read_memory`、`write_memory` |
| resource | string | 是 | 资源名称（工具名或资源路径） |

**允许时的响应（200）：**

```json
{
  "allowed": true
}
```

**拒绝时的响应（403）：**

```json
{
  "allowed": false,
  "reason": "tool not in whitelist for role"
}
```

---

### POST /v1/log — 日志上报

智能体上报执行日志。

**请求体：**

```json
{
  "agent_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "task_id": "task-001",
  "level": "info",
  "message": "开始分析用户需求",
  "timestamp": "2026-03-05T10:30:00Z"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| agent_id | string | 是 | 智能体 ID |
| task_id | string | 否 | 关联的任务 ID |
| level | string | 是 | 日志级别：`info`、`warn`、`error` |
| message | string | 是 | 日志内容 |
| timestamp | string | 是 | ISO 8601 格式时间戳 |

**成功响应（200）：**

```json
{
  "status": "ok"
}
```

---

## 管理接口

管理接口供 CLI 工具和管理员使用，同样需要 Bearer Token 认证。

### GET /v1/admin/agents — 列出所有智能体

**成功响应（200）：**

```json
[
  {
    "agent_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "name": "requirement-agent",
    "capabilities": ["requirement_analysis"],
    "version": "1.0.0",
    "status": "online",
    "last_heartbeat": "2026-03-05T10:30:00Z",
    "roles": ["developer_agent"],
    "registered_at": "2026-03-05T10:00:00Z"
  }
]
```

---

### DELETE /v1/admin/agents/{agentID} — 注销智能体

将智能体状态设为 `deregistered`，注销后的智能体不再接收任务。

**路径参数：**

| 参数 | 说明 |
|------|------|
| agentID | 智能体 ID |

**成功响应（200）：**

```json
{
  "status": "ok"
}
```

---

### GET /v1/admin/workflows — 列出所有工作流实例（v0.3.1 新增）

列出平台中所有工作流实例。

**成功响应（200）：**

```json
[
  {
    "instance_id": "wf-inst-001",
    "definition_id": "software-dev-workflow",
    "status": "running",
    "created_at": "2026-03-05T10:00:00Z",
    "updated_at": "2026-03-05T10:05:00Z"
  }
]
```

---

### POST /v1/admin/workflows — 提交工作流

提交一个 DAG 工作流定义，平台会验证 DAG 合法性并自动调度起始任务。

**请求体：**

```json
{
  "id": "software-dev-workflow",
  "name": "软件开发工作流",
  "nodes": [
    {
      "id": "requirement",
      "type": "requirement_analysis",
      "capabilities": ["requirement_analysis"],
      "input": {"user_requirement": "构建一个用户管理系统"},
      "priority": 10
    },
    {
      "id": "design",
      "type": "detailed_design",
      "capabilities": ["detailed_design"],
      "priority": 8
    },
    {
      "id": "coding",
      "type": "coding",
      "capabilities": ["coding"],
      "priority": 6
    },
    {
      "id": "testing",
      "type": "testing",
      "capabilities": ["testing"],
      "priority": 4
    }
  ],
  "edges": [
    {"from": "requirement", "to": "design"},
    {"from": "design", "to": "coding"},
    {"from": "coding", "to": "testing"}
  ]
}
```

**节点字段说明：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | string | 是 | 节点唯一标识 |
| type | string | 是 | 任务类型 |
| capabilities | string[] | 是 | 所需能力标签 |
| input | object | 否 | 任务输入参数 |
| priority | int | 否 | 优先级（数值越大越优先，默认 0） |

**边字段说明：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| from | string | 是 | 源节点 ID |
| to | string | 是 | 目标节点 ID |

**成功响应（201）：**

```json
{
  "instance_id": "wf-inst-001",
  "definition_id": "software-dev-workflow",
  "status": "running",
  "created_at": "2026-03-05T10:00:00Z",
  "updated_at": "2026-03-05T10:00:00Z"
}
```

**错误响应（422）：**

DAG 包含环或定义不合法时返回 `INVALID_WORKFLOW`。

---

### GET /v1/admin/workflows/{workflowID} — 查询工作流状态

**路径参数：**

| 参数 | 说明 |
|------|------|
| workflowID | 工作流实例 ID |

**成功响应（200）：**

```json
{
  "instance_id": "wf-inst-001",
  "definition_id": "software-dev-workflow",
  "status": "running",
  "created_at": "2026-03-05T10:00:00Z",
  "updated_at": "2026-03-05T10:05:00Z"
}
```

工作流状态值：`running`、`completed`、`failed`

---

### GET /v1/admin/workflows/{workflowID}/artifacts — 查询工作流产出物

**路径参数：**

| 参数 | 说明 |
|------|------|
| workflowID | 工作流实例 ID |

**成功响应（200）：**

```json
[
  {
    "workflow_id": "wf-inst-001",
    "task_id": "task-001",
    "name": "requirement_doc",
    "path": "data/artifacts/wf-inst-001/task-001/requirement_doc",
    "created_at": "2026-03-05T10:10:00Z"
  }
]
```

---

### GET /v1/admin/agents/{agentID}/logs — 查询智能体日志

**路径参数：**

| 参数 | 说明 |
|------|------|
| agentID | 智能体 ID |

**成功响应（200）：**

```json
[
  {
    "agent_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "task_id": "task-001",
    "level": "info",
    "message": "开始分析用户需求",
    "timestamp": "2026-03-05T10:30:00Z"
  }
]
```

---

### GET /v1/admin/events — 查询事件列表（v0.3 新增）

查询平台事件记录，支持按事件类型和实体 ID 过滤。

**查询参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| event_type | string | 否 | 事件类型过滤 |
| entity_id | string | 否 | 实体 ID 过滤 |

**请求示例：**

```
GET /v1/admin/events?event_type=task.completed
GET /v1/admin/events?entity_id=a1b2c3d4
GET /v1/admin/events?event_type=agent.registered&entity_id=a1b2c3d4
```

**成功响应（200）：**

```json
[
  {
    "event_id": "evt-001",
    "event_type": "task.completed",
    "entity_type": "task",
    "entity_id": "task-001",
    "detail": "{\"workflow_id\":\"wf-001\"}",
    "created_at": "2026-03-10T10:30:00Z"
  }
]
```

**支持的事件类型：**

| 事件类型 | 说明 |
|---------|------|
| agent.registered | 智能体注册 |
| agent.deregistered | 智能体注销 |
| agent.offline | 智能体离线 |
| task.assigned | 任务被分配 |
| task.completed | 任务完成 |
| task.failed | 任务失败 |
| task.requeued | 任务重新入队 |
| workflow.submitted | 工作流提交 |
| workflow.completed | 工作流完成 |
| workflow.failed | 工作流失败 |

---

### POST /v1/admin/webhooks — 创建 Webhook 订阅（v0.3 新增）

创建一个 Webhook 订阅，当指定类型的事件发生时，平台会向订阅 URL 发送 HTTP POST 回调。

**请求体：**

```json
{
  "url": "https://example.com/webhook",
  "event_types": ["task.completed", "workflow.completed", "workflow.failed"]
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| url | string | 是 | 回调 URL（必须是有效的 HTTP/HTTPS URL） |
| event_types | string[] | 是 | 订阅的事件类型列表 |

**成功响应（201）：**

```json
{
  "webhook_id": "wh-001",
  "url": "https://example.com/webhook",
  "event_types": ["task.completed", "workflow.completed", "workflow.failed"],
  "created_at": "2026-03-10T10:00:00Z"
}
```

**Webhook 回调请求体格式：**

当匹配的事件发生时，平台向订阅 URL 发送 HTTP POST 请求，请求体格式如下：

```json
{
  "event_id": "evt-001",
  "event_type": "task.completed",
  "entity_type": "task",
  "entity_id": "task-001",
  "detail": "{\"workflow_id\":\"wf-001\"}",
  "timestamp": "2026-03-10T10:30:00Z"
}
```

**注意事项：**

- Webhook 回调使用 5 秒超时，防止外部系统响应缓慢影响平台性能
- 回调失败（非 2xx 响应或超时）仅记录日志，不重试，不阻塞事件处理
- Webhook 分发在独立 goroutine 中异步执行

---

### GET /v1/admin/webhooks — 列出所有 Webhook 订阅（v0.3 新增）

**成功响应（200）：**

```json
[
  {
    "webhook_id": "wh-001",
    "url": "https://example.com/webhook",
    "event_types": ["task.completed", "workflow.completed"],
    "created_at": "2026-03-10T10:00:00Z"
  }
]
```

---

### DELETE /v1/admin/webhooks/{webhookID} — 删除 Webhook 订阅（v0.3 新增）

**路径参数：**

| 参数 | 说明 |
|------|------|
| webhookID | Webhook 订阅 ID |

**成功响应（200）：**

```json
{
  "status": "ok"
}
```

---

## Prometheus 指标接口（v0.3 新增）

### GET /metrics — Prometheus 指标

暴露 Prometheus 格式的监控指标数据。该端点无需 Token 认证，仅在配置 `metrics_enabled=true` 时可用。

**响应格式：** Prometheus text exposition format

**自定义业务指标：**

| 指标名称 | 类型 | 标签 | 说明 |
|---------|------|------|------|
| clawfactory_tasks_total | Counter | status | 任务状态变更计数（completed, failed, pending, assigned） |
| clawfactory_scheduling_duration_seconds | Histogram | — | 任务调度延迟（从创建到分配的耗时） |
| clawfactory_queue_depth | Gauge | — | 当前待处理任务队列深度 |
| clawfactory_agents_online | Gauge | — | 当前在线智能体数量 |
| clawfactory_workflow_duration_seconds | Histogram | — | 工作流执行时间（从创建到完成/失败） |
| clawfactory_http_requests_total | Counter | method, path, status_code | HTTP 请求计数 |
| clawfactory_http_request_duration_seconds | Histogram | method, path | HTTP 请求延迟 |

---

## StateStore 接口方法（v0.2 新增）

以下是 v0.2 版本新增的 StateStore 接口方法，用于支持负载均衡、自动重试、离线任务重入队和任务分配持久化等功能。这些方法供平台内部组件（TaskQueue、Scheduler、API handler、心跳 goroutine）调用，不直接暴露为 HTTP 端点。

### ListPendingTasks

```go
ListPendingTasks(capabilities []string) ([]model.Task, error)
```

返回匹配指定能力标签的待处理任务列表。

- 仅返回 `status = "pending"` 的任务
- 任务至少有一个能力标签与输入 `capabilities` 匹配
- 结果按优先级降序排列，优先级相同时按创建时间升序排列
- 用于替代 TaskQueue 中对 SQLiteStore 的类型断言，实现 `Dequeue` 操作

### ListUnfinishedTasks

```go
ListUnfinishedTasks() ([]model.Task, error)
```

返回所有未完成的任务列表。

- 返回 `status` 为 `pending`、`assigned` 或 `running` 的任务
- 结果按优先级降序排列
- 用于平台重启时恢复未完成任务（`RestoreUnfinished` 操作）

### CountAgentActiveTasks

```go
CountAgentActiveTasks(agentID string) (int, error)
```

返回指定智能体当前活跃任务的数量。

- 统计 `assigned_to` 匹配且 `status` 为 `assigned` 或 `running` 的任务数
- 用于 Scheduler 的负载均衡决策：将任务分配给负载最低的智能体

### IncrementTaskRetryCount

```go
IncrementTaskRetryCount(taskID string) error
```

原子递增指定任务的重试计数。

- 执行 `retry_count = retry_count + 1` 的原子更新
- 同时更新 `updated_at` 时间戳
- 任务不存在时返回错误
- 用于任务失败自动重试时记录重试次数

### GetTasksByAssignee

```go
GetTasksByAssignee(agentID string) ([]model.Task, error)
```

返回分配给指定智能体的所有活跃任务。

- 返回 `assigned_to` 匹配且 `status` 为 `assigned` 或 `running` 的任务
- 用于智能体离线时查询其未完成任务并重新入队

### UpdateTaskAssignment

```go
UpdateTaskAssignment(taskID string, agentID string) error
```

更新指定任务的分配信息。

- 更新 `assigned_to` 字段和 `updated_at` 时间戳
- 任务不存在时返回错误
- `agentID` 为空字符串时表示清空分配（用于任务重入队场景）
- 用于 Scheduler 分配任务后持久化分配信息，以及任务重入队时清空分配

---

## StateStore 接口方法（v0.3 新增）

以下是 v0.3 版本新增的 StateStore 接口方法，用于支持事件系统和 Webhook 通知功能。

### SaveEvent

```go
SaveEvent(event model.Event) error
```

持久化一个平台事件到 events 表。

- 事件包含 event_id、event_type、entity_type、entity_id、detail（JSON）、created_at
- 用于 EventBus 发布事件时的同步持久化

### ListEvents

```go
ListEvents(filter model.EventFilter) ([]model.Event, error)
```

查询事件列表，支持按 event_type 和 entity_id 过滤。

- `filter.EventType` 非空时，仅返回匹配该类型的事件
- `filter.EntityID` 非空时，仅返回匹配该实体的事件
- 两个过滤条件可同时使用（AND 关系）

### SaveWebhook

```go
SaveWebhook(webhook model.WebhookSubscription) error
```

保存一个 Webhook 订阅配置。

- event_types 序列化为 JSON 字符串存储

### ListWebhooks

```go
ListWebhooks() ([]model.WebhookSubscription, error)
```

列出所有 Webhook 订阅。

- event_types 从 JSON 字符串反序列化为 []string

### DeleteWebhook

```go
DeleteWebhook(webhookID string) error
```

删除指定的 Webhook 订阅。

---

## StateStore 接口方法（v0.3.1 新增）

以下是 v0.3.1 版本新增的 StateStore 接口方法，用于支持事务保护和工作流实例列表功能。

### ListWorkflowInstances

```go
ListWorkflowInstances() ([]model.WorkflowInstance, error)
```

返回所有工作流实例列表。

- 结果按创建时间降序排列
- 用于 `GET /v1/admin/workflows` 端点和 `claw workflow list` CLI 命令

### RunInTransaction

```go
RunInTransaction(fn func(tx *sql.Tx) error) error
```

在数据库事务中执行给定函数。

- 如果 `fn` 返回 nil，事务提交；否则事务回滚
- 用于需要原子性保证的多步数据库操作（如任务重试、离线任务重入队）

### RequeueTaskTx

```go
RequeueTaskTx(tx *sql.Tx, taskID string) error
```

在事务中重入队任务：将状态设为 `pending`，清空 `assigned_to`。

- 必须在 `RunInTransaction` 提供的事务中调用
- 用于智能体离线时的任务重入队操作

### RetryTaskTx

```go
RetryTaskTx(tx *sql.Tx, taskID string) error
```

在事务中重试任务：递增 `retry_count`，将状态设为 `pending`，清空 `assigned_to`。

- 必须在 `RunInTransaction` 提供的事务中调用
- 用于任务失败时的自动重试操作

---

## 响应头（v0.3 新增）

所有请求的响应头中包含 `X-Trace-ID`，值为唯一的 UUID 标识符，用于关联同一请求在不同组件中产生的日志。

```
X-Trace-ID: 550e8400-e29b-41d4-a716-446655440000
```

---

## 健康检查

### GET /health — 健康检查

无需认证，用于监控和负载均衡器探活。

**成功响应（200）：**

```json
{
  "status": "ok"
}
```
