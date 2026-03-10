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

## Technical Evolution Direction

> This section documents ClawFactory's long-term technical vision and evolution direction, based on comparative analysis with Kubernetes, OpenClaw, TrustClaw, and similar systems.

### 1. Platform Positioning: Orchestration Platform vs Framework

ClawFactory follows the **platform** approach (similar to how Kubernetes orchestrates containers), rather than the framework approach (like LangChain/CrewAI embedded in applications). Key differences:

| Dimension | Platform (ClawFactory) | Framework (LangChain/CrewAI) |
|-----------|----------------------|------------------------------|
| Deployment model | Standalone service, agents connect via ARI protocol | Embedded in application process |
| Language binding | Language-agnostic (HTTP REST) | Typically bound to a single language |
| Scaling | Horizontal scaling of control plane + agents | In-application scaling |
| Governance | Centralized policies, auditing, monitoring | Scattered across application code |
| Use cases | Enterprise multi-team collaboration, production | Rapid prototyping, single-app orchestration |

This positioning determines ClawFactory's evolution path: strengthening control plane capabilities, standardizing protocols, and providing multi-language SDKs, rather than pursuing framework-level developer convenience.

### 2. Hierarchical Agent Architecture (Long-term Goal)

Currently, ClawFactory agents are flat — all agents interact directly with the control plane. The future evolution targets a layered architecture:

```
┌─────────────────────────────────────────────┐
│          Strategic Layer                      │
│  Planning agents: understand goals,           │
│  decompose into sub-task DAGs                 │
│  (requires LLM, responsible for dynamic DAG)  │
├─────────────────────────────────────────────┤
│          Tactical Layer                       │
│  Coordination agents: manage sub-task         │
│  execution order and dependencies             │
│  (intelligent upgrade of Workflow Engine)      │
├─────────────────────────────────────────────┤
│          Execution Layer                      │
│  Worker agents: execute concrete tasks        │
│  (coding/testing/analysis etc.)               │
│  (corresponds to current 4 example agents)    │
├─────────────────────────────────────────────┤
│          Tool Layer                           │
│  Tool invocations: filesystem, APIs,          │
│  databases, search engines, etc.              │
│  (governed by Policy Engine)                  │
└─────────────────────────────────────────────┘
```

Implementation path:
- v0.5: ARI protocol extension to support agent role labels (strategic/tactical/execution/tool)
- v0.7+: Strategic agents can dynamically submit sub-workflows via API
- v1.x: Complete hierarchical scheduling and permission isolation

### 3. Dynamic DAG & LLM Planning

Current workflows use static DAGs (user-predefined). The long-term goal is dynamic DAG support:

- **Phase 1 (v0.5)**: Conditional branching — decide whether to execute downstream nodes based on upstream output
- **Phase 2 (v0.7+)**: Runtime DAG modification — allow agents to append nodes to running workflows via API
- **Phase 3 (v1.x+)**: LLM-driven planning — strategic agents receive high-level goals and auto-generate execution DAGs

> Note: LLM-driven dynamic planning is a long-term goal. The current stage should focus on reliable static DAG execution and conditional branching, laying the foundation for dynamic capabilities. Premature introduction of LLM planning would increase system complexity, and there is insufficient execution data to validate planning quality.

### 4. Security & Policy Engine Deepening

The current Policy Engine uses static RBAC based on JSON configuration. Evolution direction:

- **v0.4**: JWT authentication replacing static tokens, TLS encrypted transport
- **v0.6+**: Introduce declarative policy language (referencing OPA/Rego or AWS Cedar) for fine-grained context-aware authorization
- **v1.x**: Policy as Code with version management and audit trails
- **Long-term**: Agent trust scoring — dynamically adjust permission boundaries based on historical behavior

The policy engine's goal is to evolve from "static rule matching" to "context-aware dynamic decision-making" while maintaining policy auditability and explainability.

### 5. Memory & Sharing Mechanism Evolution

The current Shared Memory is a simple file system + SQLite metadata implementation. Evolution direction:

| Phase | Capability | Implementation |
|-------|-----------|---------------|
| Current (v0.2) | File artifact storage, workflow isolation | Local filesystem + SQLite |
| v0.6 | Object storage backend, large file support | S3/MinIO compatible interface |
| v0.7+ | Structured context sharing, inter-agent messaging | Redis/NATS message layer |
| v1.x | Long-term memory, cross-workflow knowledge accumulation | Vector database integration |

Core principle: The memory layer is always abstracted through the StateStore interface, with swappable backends.

### 6. Environment Adaptability

ClawFactory's deployment targets extend beyond cloud-native environments:

| Environment | Support Phase | Key Capabilities |
|-------------|--------------|-----------------|
| Development (single machine) | ✅ Current | SQLite, single process, zero-dependency startup |
| On-premises | v0.6+ | PostgreSQL backend, Docker deployment |
| Cloud-native | v0.8+ | K8s Helm Chart, horizontal scaling |
| Hybrid cloud | v1.x | Multi-cluster federation, cross-region scheduling |
| Edge computing | v2.0 | Lightweight control plane, offline mode |

The choice of SQLite as the default storage is deliberate — it ensures a zero-dependency development experience while leaving room for production storage upgrades through the StateStore interface.

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
│   ├── config/             # Configuration utilities
│   ├── registry/           # Registry
│   ├── scheduler/          # Scheduler
│   ├── policy/             # Policy engine
│   ├── workflow/           # Workflow engine
│   ├── taskqueue/          # Task queue
│   ├── memory/             # Shared memory
│   ├── store/              # State store
│   └── model/              # Data models
├── tests/                  # Test files (mirrors source code hierarchy)
│   ├── internal/           # Tests for internal packages
│   └── cmd/                # Tests for cmd packages
├── agents/                 # Python example agents
├── configs/                # Configuration files
├── docs/                   # Documentation
├── go.mod
└── go.sum
```
