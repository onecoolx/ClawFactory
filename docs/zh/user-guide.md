# ClawFactory 用户手册

## 概述

ClawFactory 是一个多智能体编排平台，通过 DAG 工作流定义智能体之间的协作关系，自动调度和管理任务执行。本手册涵盖平台的日常使用、配置管理、工作流定义和智能体开发。

## CLI 命令参考

`claw` 是 ClawFactory 的命令行管理工具，支持工作流管理和智能体管理。

### 全局参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| --url | ClawFactory 服务地址 | http://localhost:8080 |
| --token | API Token | dev-token-001 |
| --output-json | 以 JSON 格式输出 | false |

### 工作流命令

```bash
# 提交工作流
claw workflow submit <workflow.json>

# 查询工作流状态
claw workflow status <workflow_id>

# 查询工作流产出物
claw workflow artifacts <workflow_id>
```

### 智能体命令

```bash
# 列出所有智能体
claw agent list

# 查看智能体日志
claw agent logs <agent_id>
```

## 工作流定义

### DAG 结构

工作流使用 JSON 格式定义，核心是一个有向无环图（DAG），由节点（nodes）和边（edges）组成。

```json
{
  "id": "my-workflow",
  "name": "我的工作流",
  "nodes": [...],
  "edges": [...]
}
```

### 节点定义

每个节点代表一个任务，需要指定类型和所需能力：

```json
{
  "id": "step-1",
  "type": "data_processing",
  "capabilities": ["data_processing"],
  "input": {
    "source": "https://example.com/data.csv"
  },
  "priority": 10
}
```

- `id`：节点唯一标识，在同一工作流内不能重复
- `type`：任务类型，用于标识任务的业务含义
- `capabilities`：所需能力标签列表，调度器根据此字段匹配智能体
- `input`：任务输入参数，键值对格式
- `priority`：优先级，数值越大越优先被调度（默认 0）

### 边定义

边定义节点之间的依赖关系：

```json
{
  "from": "step-1",
  "to": "step-2"
}
```

表示 `step-2` 依赖 `step-1`，只有 `step-1` 完成后 `step-2` 才会被调度。

### DAG 验证规则

提交工作流时，平台会自动验证 DAG 的合法性：

1. 不能包含环（循环依赖）
2. 边引用的节点 ID 必须在节点列表中存在
3. 至少有一个入度为 0 的起始节点

验证失败时返回 HTTP 422 错误。

### 工作流执行流程

1. 提交工作流后，平台创建工作流实例，状态为 `running`
2. 所有入度为 0 的节点被创建为任务，放入任务队列
3. 智能体拉取匹配的任务并执行
4. 任务完成后，平台检查下游节点的所有依赖是否满足
5. 满足依赖的下游节点被创建为新任务放入队列
6. 所有任务完成后，工作流状态变为 `completed`
7. 任何任务永久失败（超过重试次数），工作流状态变为 `failed`

## 配置管理

### 平台配置

配置文件路径优先级：
1. 环境变量 `CLAWFACTORY_CONFIG` 指定的路径
2. 默认路径 `configs/config.json`

环境变量可以覆盖配置文件中的值：

| 环境变量 | 对应配置 |
|----------|----------|
| CLAWFACTORY_PORT | port |
| CLAWFACTORY_DB_PATH | db_path |
| CLAWFACTORY_DATA_DIR | data_dir |
| CLAWFACTORY_CONFIG | 配置文件路径 |
| CLAWFACTORY_POLICY_PATH | 策略配置文件路径 |

### 策略配置

策略配置文件 `configs/policy.json` 控制平台的安全和调度行为。

#### 重试策略

```json
{
  "max_retries": 3
}
```

任务失败后会自动重试，最多重试 `max_retries` 次。超过次数后任务标记为永久失败。

#### 心跳配置

```json
{
  "heartbeat_interval_seconds": 30,
  "heartbeat_timeout_multiplier": 3
}
```

- `heartbeat_interval_seconds`：建议的心跳间隔（秒）
- `heartbeat_timeout_multiplier`：超时倍数，超过 `interval × multiplier` 秒未心跳的智能体标记为 offline

#### 角色与权限

```json
{
  "roles": {
    "developer_agent": {
      "permissions": [
        {"resource": "shared_memory:*", "actions": ["read", "write"]},
        {"resource": "task:*", "actions": ["read", "update"]}
      ]
    }
  }
}
```

- `resource`：资源模式，支持 `*` 通配符
- `actions`：允许的操作列表

#### 工具白名单

```json
{
  "tool_whitelist": {
    "developer_agent": {
      "allowed_tools": ["llm_api", "file_write", "file_read"],
      "rate_limit_per_minute": 60
    }
  }
}
```

- `allowed_tools`：允许使用的工具列表
- `rate_limit_per_minute`：每分钟最大调用次数

## 智能体开发指南

### ARI 协议概述

智能体通过 ARI（Agent Runtime Interface）HTTP REST 协议与平台通信。协议是语言无关的，任何能发送 HTTP 请求的编程语言都可以开发智能体。

### 智能体生命周期

```
启动 → 注册 → [心跳循环 + 任务拉取循环] → 注销
```

1. **注册**：调用 `POST /v1/register` 注册到平台
2. **心跳**：定期调用 `POST /v1/heartbeat` 保持在线状态
3. **拉取任务**：轮询 `GET /v1/tasks` 获取匹配的任务
4. **执行任务**：收到任务后更新状态为 `running`，执行完成后更新为 `completed` 或 `failed`
5. **上报日志**：执行过程中通过 `POST /v1/log` 上报日志

### Python 智能体开发

项目提供了 Python 基类 `BaseAgent`，封装了 ARI 协议的所有通信逻辑。

#### 创建自定义智能体

```python
import asyncio
import os
from base_agent import BaseAgent


class MyAgent(BaseAgent):
    def __init__(self, api_token: str):
        super().__init__(
            name="my-agent",
            capabilities=["my_capability"],
            version="1.0.0",
            api_token=api_token,
        )

    async def execute_task(self, task: dict) -> dict:
        """实现你的任务逻辑"""
        input_data = task.get("input", {})
        
        # 上报日志
        await self.report_log(task["task_id"], "info", "开始处理任务")
        
        # 执行业务逻辑
        result = do_something(input_data)
        
        # 返回产出物
        return {"output_key": result}


if __name__ == "__main__":
    token = os.getenv("CLAWFACTORY_TOKEN", "dev-token-001")
    agent = MyAgent(api_token=token)
    asyncio.run(agent.run())
```

#### BaseAgent 提供的方法

| 方法 | 说明 |
|------|------|
| `register()` | 注册到平台，返回 agent_id |
| `heartbeat()` | 发送心跳 |
| `pull_task()` | 拉取任务，无任务返回 None |
| `update_task_status(task_id, status, output, error)` | 更新任务状态 |
| `report_log(task_id, level, message)` | 上报日志 |
| `run()` | 主循环（注册 + 心跳 + 任务拉取） |
| `execute_task(task)` | 抽象方法，子类实现任务逻辑 |

#### 配置项

| 属性 | 说明 | 默认值 |
|------|------|--------|
| base_url | 平台服务地址 | http://localhost:8080/v1 |
| heartbeat_interval | 心跳间隔（秒） | 30 |

### Go 智能体开发

也可以用 Go 开发智能体，直接通过 HTTP 客户端调用 ARI 接口：

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

const baseURL = "http://localhost:8080/v1"
const token = "dev-token-001"

func register(name string, capabilities []string, version string) (string, error) {
    body, _ := json.Marshal(map[string]interface{}{
        "name": name, "capabilities": capabilities, "version": version,
    })
    req, _ := http.NewRequest("POST", baseURL+"/register", bytes.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+token)
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    
    var result map[string]string
    json.NewDecoder(resp.Body).Decode(&result)
    return result["agent_id"], nil
}

func main() {
    agentID, err := register("my-go-agent", []string{"data_processing"}, "1.0.0")
    if err != nil {
        panic(err)
    }
    fmt.Printf("注册成功: %s\n", agentID)
    
    // 启动心跳和任务拉取循环...
}
```

### 其他语言

ARI 协议基于标准 HTTP REST，任何语言都可以开发智能体。核心步骤：

1. 实现 HTTP 客户端，携带 `Authorization: Bearer <token>` 头
2. 调用 `POST /v1/register` 注册
3. 定时调用 `POST /v1/heartbeat`（建议 30 秒间隔）
4. 轮询 `GET /v1/tasks?agent_id=xxx` 拉取任务
5. 执行任务后调用 `POST /v1/tasks/{taskID}/status` 更新状态

## 数据存储

### SQLite 数据库

平台使用 SQLite 作为持久化存储，数据库文件默认位于 `data/clawfactory.db`。

存储的数据包括：
- 智能体注册信息和状态
- 任务信息和执行状态
- 工作流定义和实例
- 日志
- 产出物元数据
- 审计日志
- 工具调用速率限制

### 产出物存储

任务产出物以文件形式存储在数据目录下：

```
data/artifacts/{workflow_id}/{task_id}/{artifact_name}
```

产出物的元数据（路径、创建时间等）存储在 SQLite 中。

## 故障排查

### 平台无法启动

1. 检查端口是否被占用：`lsof -i :8080`
2. 检查数据目录权限：确保 `data/` 目录可写
3. 检查配置文件格式：确保 JSON 格式正确

### 智能体无法注册

1. 检查平台是否正在运行：`curl http://localhost:8080/health`
2. 检查 API Token 是否正确
3. 检查网络连接

### 任务不被调度

1. 确认智能体状态为 `online`：`claw agent list`
2. 确认智能体的 `capabilities` 与任务的 `capabilities` 有交集
3. 确认任务状态为 `pending`

### 工作流提交失败

1. 检查 JSON 格式是否正确
2. 检查 DAG 是否包含环
3. 检查边引用的节点 ID 是否存在
