# ClawFactory Architecture Document

## Overview

ClawFactory is a locally-running multi-agent orchestration platform with a control plane / data plane separation architecture. The platform core is developed in Go, running as a single-process HTTP service. Agents communicate with the platform through the ARI (Agent Runtime Interface) HTTP REST protocol, making it language-agnostic.

## Core Design Principles

- **Interface Abstraction**: All components are defined through Go interfaces, making implementations swappable
- **Modularity**: Control plane and data plane are independent Go packages, ready for future microservice decomposition
- **Protocol-Driven**: Agents connect via the standard ARI protocol, supporting any programming language
- **Security-First**: RBAC permission management and tool usage restrictions are built into the architecture from day one

## Technology Stack

| Component | Technology | Rationale |
|-----------|-----------|-----------|
| HTTP Router | [go-chi/chi](https://github.com/go-chi/chi) | Lightweight, idiomatic Go, 100% net/http compatible |
| SQLite Driver | [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) | Pure Go implementation, no CGO required, simplifies cross-compilation |
| CLI Framework | [spf13/cobra](https://github.com/spf13/cobra) | Most popular CLI framework in the Go ecosystem |
| Property Testing | [pgregory.net/rapid](https://pkg.go.dev/pgregory.net/rapid) | Mature property-based testing library for Go |
| Configuration | Environment variables + JSON config files | Simple and flexible |
| Example Agents | Python + httpx + openai | Rapid prototyping |

## Overall Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    CLI (claw command)                     │
└──────────────────────┬──────────────────────────────────┘
                       │ HTTP
┌──────────────────────▼──────────────────────────────────┐
│              ClawFactory Platform (Go single process)    │
│  ┌─────────────────────────────────────────────────┐    │
│  │              Control Plane                        │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────────────┐ │    │
│  │  │ Registry │ │Scheduler │ │ Policy Engine    │ │    │
│  │  └──────────┘ └──────────┘ └──────────────────┘ │    │
│  │  ┌──────────────────┐ ┌──────────────────────┐  │    │
│  │  │ Workflow Engine  │ │ ARI Layer (Router)   │  │    │
│  │  └──────────────────┘ └──────────────────────┘  │    │
│  └─────────────────────────────────────────────────┘    │
│  ┌─────────────────────────────────────────────────┐    │
│  │              Data Plane                           │    │
│  │  ┌──────────┐ ┌──────────────┐ ┌────────────┐  │    │
│  │  │TaskQueue │ │Shared Memory │ │State Store │  │    │
│  │  └──────────┘ └──────────────┘ └────────────┘  │    │
│  └─────────────────────────────────────────────────┘    │
└──────────────────────▲──────────────────────────────────┘
                       │ ARI HTTP
┌──────────────────────┴──────────────────────────────────┐
│                 Agents (Independent Processes)            │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐   │
│  │Requirement│ │ Design   │ │ Coding   │ │ Testing  │   │
│  │Agent     │ │ Agent    │ │ Agent    │ │ Agent    │   │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘   │
└─────────────────────────────────────────────────────────┘
```

## Component Details

### 1. ARI Protocol Layer (HTTP Router)

The platform's entry point, responsible for routing all HTTP requests and executing Token authentication middleware. Built on go-chi.

Route structure:
- `GET /health` — Health check (no auth required)
- `POST /v1/register` — Agent registration
- `POST /v1/heartbeat` — Heartbeat reporting
- `GET /v1/tasks` — Task polling
- `POST /v1/tasks/{taskID}/status` — Task status update
- `POST /v1/authorize` — Permission check
- `POST /v1/log` — Log reporting
- `GET /v1/admin/agents` — List all agents
- `DELETE /v1/admin/agents/{agentID}` — Deregister agent
- `POST /v1/admin/workflows` — Submit workflow
- `GET /v1/admin/workflows/{workflowID}` — Query workflow status
- `GET /v1/admin/workflows/{workflowID}/artifacts` — Query artifacts
- `GET /v1/admin/agents/{agentID}/logs` — Query agent logs

### 2. Registry

Manages agent registration, discovery, and health checking.

Key behaviors:
- Duplicate registration (same name + version) returns existing agent_id (idempotent)
- Background goroutine checks heartbeat timeouts every 30 seconds; agents without heartbeat for 90+ seconds are marked offline
- Offline agents recover to online status upon sending a new heartbeat
- Deregistered agents no longer receive tasks

### 3. Scheduler

Assigns tasks to suitable agents.

Scheduling strategy:
- Matches tasks in the queue based on agent capabilities
- Returns empty response when no matching tasks exist
- Offline agents are not assigned tasks
- Task status changes from pending to assigned after assignment

### 4. Policy Engine

Manages scheduling policies, authorization rules, and tool whitelists. Loaded from JSON configuration.

Features:
- RBAC permission verification
- Tool whitelist management
- Rate limiting (calls per minute)
- Retry policies (max retry count)
- Audit logging

### 5. Workflow Engine

Manages the lifecycle of DAG workflows.

Features:
- DAG validation (topological sort for cycle detection)
- Automatically schedules root tasks (nodes with in-degree 0) upon workflow submission
- Checks downstream dependencies when tasks complete
- Marks entire workflow as failed when a task permanently fails
- Marks workflow as completed when all tasks finish

### 6. Task Queue

Manages a persistent queue of pending tasks.

Features:
- Forces status to pending on enqueue
- Dequeues by priority (descending) then creation time (ascending)
- Capability-based filtering
- Restores unfinished tasks on platform restart

### 7. Shared Memory

Manages context and artifact sharing between agents.

Storage structure: `{data_dir}/artifacts/{workflow_id}/{task_id}/{artifact_name}`

Features:
- Workflow-isolated artifact storage
- Query artifacts by workflow or by task
- Retrieve upstream task artifacts for inter-task data passing

### 8. State Store

Unified persistent storage interface, currently implemented with SQLite.

Managed data:
- Agent information and status
- Task information and status
- Workflow definitions and instances
- Logs
- Artifact metadata
- Audit logs

## Data Flow

### Workflow Execution Flow

1. User submits a workflow definition (DAG) via CLI or API
2. Workflow Engine validates DAG legality
3. Root nodes (in-degree 0) are created as Tasks and placed in the Task Queue
4. Agents poll for matching tasks via ARI protocol
5. Agents execute tasks, reporting status and artifacts
6. Upon task completion, Workflow Engine checks downstream dependencies
7. Downstream tasks with satisfied dependencies are placed in the Task Queue
8. Steps 4-7 repeat until all tasks complete or a task permanently fails

### Task State Transitions

```
pending → assigned → running → completed
                        ↓
                     failed → pending (retry)
                        ↓
                permanently_failed
```

## Project Directory Structure

```
ClawFactory/
├── cmd/
│   ├── clawfactory/        # Platform main service entry
│   │   └── main.go
│   └── claw/               # CLI tool entry
│       └── main.go
├── internal/
│   ├── api/                # ARI protocol layer
│   ├── registry/           # Registry
│   ├── scheduler/          # Scheduler
│   ├── policy/             # Policy engine
│   ├── workflow/           # Workflow engine
│   ├── taskqueue/          # Task queue
│   ├── memory/             # Shared memory
│   ├── store/              # State store
│   └── model/              # Data models
├── agents/                 # Python example agents
├── configs/                # Configuration files
├── docs/                   # Documentation
├── go.mod
└── go.sum
```
