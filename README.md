# ClawFactory

Copyright (c) 2026 Zhang Ji Peng (onecoolx@gmail.com)

A multi-agent orchestration platform with control plane / data plane separation architecture. Built in Go, it runs as a single-process HTTP service. Agents communicate via the ARI (Agent Runtime Interface) REST protocol, making them language-agnostic.

多智能体编排平台，采用控制平面/数据平面分离架构。平台核心用 Go 开发，作为单进程 HTTP 服务运行。智能体通过 ARI（Agent Runtime Interface）REST 协议通信，语言无关。

## Features

- **DAG Workflow Engine** — Define agent collaboration as directed acyclic graphs with automatic dependency resolution
- **Capability-Based Scheduling** — Match tasks to agents based on declared capabilities with load balancing
- **ARI Protocol** — Language-agnostic HTTP REST protocol for agent communication
- **RBAC & Policy Engine** — Role-based access control, tool whitelists, and rate limiting
- **Shared Memory** — Workflow-isolated artifact storage for inter-task data passing
- **SQLite Persistence** — Pure Go SQLite driver (no CGO), automatic task recovery on restart
- **CLI Tool** — `claw` command-line tool for workflow and agent management

## Quick Start

```bash
# Build
go build -o bin/clawfactory ./cmd/clawfactory
go build -o bin/claw ./cmd/claw

# Start the platform
./bin/clawfactory

# Submit a workflow
./bin/claw workflow submit configs/software-dev-workflow.json

# Start example agents (Python)
cd agents && pip install -r requirements.txt
python requirement_agent.py
```

## Architecture

```
CLI (claw) ──HTTP──▶ ClawFactory Platform (Go)
                      ├── Control Plane
                      │   ├── Registry        (agent registration & discovery)
                      │   ├── Scheduler       (capability-based task assignment)
                      │   ├── Policy Engine   (RBAC, tool whitelist, rate limit)
                      │   └── Workflow Engine  (DAG validation & execution)
                      └── Data Plane
                          ├── Task Queue      (priority-based persistent queue)
                          ├── Shared Memory   (artifact storage)
                          └── State Store     (SQLite)

Agents (Python/Go/any) ──ARI HTTP──▶ Platform
```

## Tech Stack

| Component | Technology |
|-----------|-----------|
| HTTP Router | go-chi/chi |
| SQLite | modernc.org/sqlite (pure Go) |
| CLI | spf13/cobra |
| Property Testing | pgregory.net/rapid |
| Example Agents | Python + httpx + openai |

## Project Structure

```
ClawFactory/
├── cmd/clawfactory/     # Platform service entry
├── cmd/claw/            # CLI tool entry
├── internal/
│   ├── api/             # HTTP handlers & middleware
│   ├── model/           # Data models
│   ├── store/           # SQLite state store
│   ├── registry/        # Agent registry
│   ├── scheduler/       # Task scheduler
│   ├── policy/          # Policy engine
│   ├── workflow/        # Workflow engine
│   ├── taskqueue/       # Task queue
│   └── memory/          # Shared memory
├── agents/              # Python example agents
├── configs/             # Configuration files
└── docs/                # Documentation
```

## Documentation / 文档

### English

- [Architecture](docs/en/architecture.md) — System architecture and component design
- [API Reference](docs/en/api-reference.md) — Complete ARI endpoint documentation
- [Getting Started](docs/en/getting-started.md) — Step-by-step setup and first workflow
- [User Guide](docs/en/user-guide.md) — CLI usage, configuration, agent development
- [Examples](docs/en/examples.md) — Workflow examples and code samples
- [Roadmap](docs/en/roadmap.md) — Gap analysis, technical roadmap and milestones

### 中文

- [架构设计](docs/zh/architecture.md) — 系统架构与组件设计
- [API 参考](docs/zh/api-reference.md) — 完整的 ARI 接口文档
- [快速入门](docs/zh/getting-started.md) — 从零开始搭建环境并运行第一个工作流
- [用户手册](docs/zh/user-guide.md) — CLI 使用、配置管理、智能体开发指南
- [示例文档](docs/zh/examples.md) — 工作流示例与代码示例
- [技术路线图](docs/zh/roadmap.md) — 差距分析、技术路线图与里程碑

## Configuration

Platform config: `configs/config.json`
Policy config: `configs/policy.json`
Example workflow: `configs/software-dev-workflow.json`

Environment variables:

| Variable | Description |
|----------|-------------|
| CLAWFACTORY_PORT | HTTP port (default: 8080) |
| CLAWFACTORY_DB_PATH | SQLite database path |
| CLAWFACTORY_DATA_DIR | Data directory |
| CLAWFACTORY_CONFIG | Config file path |
| CLAWFACTORY_POLICY_PATH | Policy config path |

## Testing

```bash
# Run all tests (unit + property-based)
go test ./...

# Verbose output
go test -v ./...
```

The project includes 25 property-based tests using [rapid](https://github.com/flyingmutant/rapid) covering registration idempotency, capability matching, DAG validation, workflow state transitions, and more.

## Requirements

- Go 1.23.0+
- Python 3.10+ (for example agents)

## License

See [LICENSE](LICENSE) file.
