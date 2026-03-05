# ClawFactory API 参考文档

## 概述

ClawFactory 通过 ARI（Agent Runtime Interface）HTTP REST 协议提供所有功能。所有接口使用 JSON 格式进行数据交换。

- 默认服务地址：`http://localhost:8080`
- 所有 `/v1/*` 接口需要 Bearer Token 认证
- 健康检查接口 `/health` 无需认证

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

```json
{
  "status": "ok"
}
```

**状态流转规则：**

```
pending → assigned → running → completed
                        ↓
                     failed → pending (自动重试，未超过最大重试次数)
                        ↓
                permanently_failed (超过最大重试次数)
```

当 `status` 为 `completed` 时，平台会自动触发工作流引擎检查下游任务依赖。

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

## 健康检查

### GET /health — 健康检查

无需认证，用于监控和负载均衡器探活。

**成功响应（200）：**

```json
{
  "status": "ok"
}
```
