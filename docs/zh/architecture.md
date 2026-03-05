# ClawFactory 架构设计文档

## 概述

ClawFactory 是一个本地运行的多智能体编排平台，采用控制平面/数据平面分离架构。平台核心用 Go 语言开发，作为单进程 HTTP 服务运行；智能体通过 ARI（Agent Runtime Interface）HTTP REST 协议与平台通信，语言无关。

## 核心设计原则

- **接口抽象**：所有组件通过 Go interface 定义，实现可替换
- **模块化**：控制平面和数据平面为独立 Go package，未来可拆分为微服务
- **协议驱动**：智能体通过标准 ARI 协议接入，支持任意编程语言
- **安全优先**：RBAC 权限管理和工具使用限制从第一天起纳入架构

## 技术选型

| 组件 | 技术 | 选型理由 |
|------|------|----------|
| HTTP 路由 | [go-chi/chi](https://github.com/go-chi/chi) | 轻量、符合 Go 惯用风格、100% 兼容 net/http |
| SQLite 驱动 | [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) | 纯 Go 实现，无需 CGO，简化交叉编译 |
| CLI 框架 | [spf13/cobra](https://github.com/spf13/cobra) | Go 生态最流行的 CLI 框架 |
| 属性测试 | [pgregory.net/rapid](https://pkg.go.dev/pgregory.net/rapid) | Go 生态成熟的属性测试库 |
| 配置管理 | 环境变量 + JSON 配置文件 | 简单灵活 |
| 示例智能体 | Python + httpx + openai | 快速原型开发 |

## 整体架构

```
┌─────────────────────────────────────────────────────────┐
│                    CLI (claw 命令行)                      │
└──────────────────────┬──────────────────────────────────┘
                       │ HTTP
┌──────────────────────▼──────────────────────────────────┐
│              ClawFactory 平台 (Go 单进程)                 │
│  ┌─────────────────────────────────────────────────┐    │
│  │              控制平面 (Control Plane)              │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────────────┐ │    │
│  │  │ Registry │ │Scheduler │ │ Policy Engine    │ │    │
│  │  │ 注册中心  │ │ 调度器    │ │ 策略引擎         │ │    │
│  │  └──────────┘ └──────────┘ └──────────────────┘ │    │
│  │  ┌──────────────────┐ ┌──────────────────────┐  │    │
│  │  │ Workflow Engine  │ │ ARI 协议层 (Router)   │  │    │
│  │  │ 工作流引擎        │ │ HTTP 路由 + 中间件    │  │    │
│  │  └──────────────────┘ └──────────────────────┘  │    │
│  └─────────────────────────────────────────────────┘    │
│  ┌─────────────────────────────────────────────────┐    │
│  │              数据平面 (Data Plane)                │    │
│  │  ┌──────────┐ ┌──────────────┐ ┌────────────┐  │    │
│  │  │TaskQueue │ │Shared Memory │ │State Store │  │    │
│  │  │ 任务队列  │ │ 共享记忆层    │ │ 状态存储    │  │    │
│  │  └──────────┘ └──────────────┘ └────────────┘  │    │
│  └─────────────────────────────────────────────────┘    │
└──────────────────────▲──────────────────────────────────┘
                       │ ARI HTTP
┌──────────────────────┴──────────────────────────────────┐
│                 智能体 (独立进程)                          │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐   │
│  │需求分析   │ │ 设计     │ │ 编码     │ │ 测试     │   │
│  │Agent     │ │ Agent    │ │ Agent    │ │ Agent    │   │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘   │
└─────────────────────────────────────────────────────────┘
```

## 组件说明

### 1. ARI 协议层 (HTTP Router)

平台的入口，负责路由所有 HTTP 请求并执行 Token 认证中间件。基于 go-chi 实现。

路由结构：
- `GET /health` — 健康检查（无需认证）
- `POST /v1/register` — 智能体注册
- `POST /v1/heartbeat` — 心跳上报
- `GET /v1/tasks` — 任务拉取
- `POST /v1/tasks/{taskID}/status` — 任务状态更新
- `POST /v1/authorize` — 权限检查
- `POST /v1/log` — 日志上报
- `GET /v1/admin/agents` — 列出所有智能体
- `DELETE /v1/admin/agents/{agentID}` — 注销智能体
- `POST /v1/admin/workflows` — 提交工作流
- `GET /v1/admin/workflows/{workflowID}` — 查询工作流状态
- `GET /v1/admin/workflows/{workflowID}/artifacts` — 查询产出物
- `GET /v1/admin/agents/{agentID}/logs` — 查询智能体日志

### 2. Registry（注册中心）

管理智能体的注册、发现和健康检查。

关键行为：
- 相同 name + version 重复注册返回已有 agent_id（幂等性）
- 后台 goroutine 每 30 秒检查心跳超时，超过 90 秒未心跳标记为 offline
- offline 智能体重新心跳后恢复为 online
- 注销后状态变为 deregistered，不再接收任务

### 3. Scheduler（调度器）

负责将任务分配给合适的智能体。

调度策略：
- 根据智能体 capabilities 匹配任务队列中的待执行任务
- 无匹配任务时返回空响应
- 非 online 状态的智能体不分配任务
- 分配后任务状态从 pending 变为 assigned

### 4. Policy Engine（策略引擎）

管理调度策略、授权规则和工具白名单。从 JSON 配置文件加载策略。

功能：
- RBAC 权限验证（基于角色的资源访问控制）
- 工具白名单管理
- 速率限制（每分钟调用次数）
- 重试策略（最大重试次数）
- 审计日志记录

### 5. Workflow Engine（工作流引擎）

管理 DAG 工作流的生命周期。

功能：
- DAG 验证（拓扑排序检测环）
- 提交工作流时自动调度起始任务（入度为 0 的节点）
- 任务完成时检查下游依赖，满足条件则调度下游任务
- 任务永久失败时标记整个工作流为 failed
- 所有任务完成时标记工作流为 completed

### 6. Task Queue（任务队列）

管理待执行任务的持久化队列。

功能：
- 入队时强制设置状态为 pending
- 出队按优先级降序、创建时间升序排列
- 能力匹配过滤
- 平台重启时恢复未完成任务

### 7. Shared Memory（共享记忆层）

管理智能体之间的上下文和产出物共享。

存储结构：`{data_dir}/artifacts/{workflow_id}/{task_id}/{artifact_name}`

功能：
- 按工作流隔离产出物
- 支持按工作流、按任务查询产出物
- 获取上游任务产出物（用于任务间数据传递）

### 8. State Store（状态存储）

统一的持久化存储接口，当前实现为 SQLite。

管理的数据：
- 智能体信息和状态
- 任务信息和状态
- 工作流定义和实例
- 日志
- 产出物元数据
- 审计日志

## 数据流

### 工作流执行流程

1. 用户通过 CLI 或 API 提交工作流定义（DAG）
2. Workflow Engine 验证 DAG 合法性
3. 将入度为 0 的起始节点创建为 Task 放入 Task Queue
4. 智能体通过 ARI 协议拉取匹配的任务
5. 智能体执行任务，上报状态和产出物
6. 任务完成后，Workflow Engine 检查下游依赖
7. 满足依赖的下游任务被放入 Task Queue
8. 重复 4-7 直到所有任务完成或某任务永久失败

### 任务状态流转

```
pending → assigned → running → completed
                         ↓
                      failed → pending (重试)
                         ↓
                  permanently_failed
```

## 项目目录结构

```
ClawFactory/
├── cmd/
│   ├── clawfactory/        # 平台主服务入口
│   │   └── main.go
│   └── claw/               # CLI 工具入口
│       └── main.go
├── internal/
│   ├── api/                # ARI 协议层
│   ├── registry/           # 注册中心
│   ├── scheduler/          # 调度器
│   ├── policy/             # 策略引擎
│   ├── workflow/           # 工作流引擎
│   ├── taskqueue/          # 任务队列
│   ├── memory/             # 共享记忆层
│   ├── store/              # 状态存储
│   └── model/              # 数据模型
├── agents/                 # Python 示例智能体
├── configs/                # 配置文件
├── docs/                   # 文档
├── go.mod
└── go.sum
```
