# ClawFactory Technical Roadmap

## Current Version Assessment (v0.1.0 — Prototype Validation)

### Implemented Features

| Module | Status | Notes |
|--------|--------|-------|
| ARI Protocol Layer | ✅ Done | 14 HTTP endpoints, Token auth middleware |
| Registry | ✅ Done | Idempotent registration, heartbeat, offline detection, deregistration |
| Scheduler | ✅ Done | Capability matching, status filtering; lacks true load balancing |
| Policy Engine | ✅ Done | RBAC permissions, tool whitelist, rate limiting, audit logs |
| Workflow Engine | ✅ Done | DAG validation, root task scheduling, dependency checking, status derivation |
| Task Queue | ✅ Done | Priority ordering, capability matching, unfinished task recovery |
| Shared Memory | ✅ Done | Filesystem storage, workflow isolation, metadata persistence |
| State Store (SQLite) | ✅ Done | 8 tables, WAL mode, foreign key constraints |
| CLI Tool | ✅ Done | workflow submit/status/artifacts, agent list/logs |
| Python Example Agents | ✅ Done | 4 agents (requirement/design/coding/testing) |
| Property Tests | ✅ Done | 25 property tests + unit tests, all passing |
| Documentation | ✅ Done | Bilingual (Chinese + English): architecture, API, getting-started, guide, examples |

### Gap Analysis vs Mature Orchestration Platforms (e.g., Kubernetes)

#### 1. Distribution & High Availability (Gap: Very Large)

| Capability | K8s | ClawFactory Current |
|-----------|-----|-------------------|
| Multi-node deployment | ✅ Cluster mode | ❌ Single process |
| Control plane HA | ✅ etcd cluster + leader election | ❌ Single point of failure |
| Storage HA | ✅ etcd distributed consensus | ❌ SQLite single file |
| Horizontal scaling | ✅ Auto-scaling | ❌ Not supported |
| Service discovery | ✅ DNS + Service | ❌ Local registry only |

#### 2. Scheduling (Gap: Large)

| Capability | K8s | ClawFactory Current |
|-----------|-----|-------------------|
| Multi-dimensional scheduling | ✅ Resources, affinity, taints | ⚠️ Capability tags only |
| Load balancing | ✅ Multiple strategies | ⚠️ Designed but not implemented |
| Preemptive scheduling | ✅ Priority preemption | ❌ Not supported |
| Resource quotas | ✅ ResourceQuota | ❌ Not supported |
| Scheduling queues | ✅ Multiple queues | ⚠️ Single queue |

#### 3. Workflow Capabilities (Gap: Medium)

| Capability | K8s (Argo Workflows) | ClawFactory Current |
|-----------|---------------------|-------------------|
| DAG workflows | ✅ | ✅ Basic implementation |
| Conditional branching | ✅ | ❌ Not supported |
| Loops/retries | ✅ Flexible config | ⚠️ Fixed retry count only |
| Parameter passing | ✅ Template parameters | ⚠️ Key-value input/output only |
| Timeout control | ✅ | ❌ Not supported |
| Pause/resume | ✅ | ❌ Not supported |
| Workflow templates | ✅ | ❌ Not supported |
| Sub-workflows | ✅ | ❌ Not supported |
| Cron scheduling | ✅ | ❌ Not supported |

#### 4. Observability (Gap: Large)

| Capability | K8s | ClawFactory Current |
|-----------|-----|-------------------|
| Structured logging | ✅ | ⚠️ Basic log storage |
| Metrics monitoring | ✅ Prometheus | ❌ Not supported |
| Distributed tracing | ✅ OpenTelemetry | ❌ Not supported |
| Event system | ✅ Events | ❌ Not supported |
| Dashboard | ✅ Grafana | ❌ Not supported |
| Alerting | ✅ AlertManager | ❌ Not supported |

#### 5. Security (Gap: Large)

| Capability | K8s | ClawFactory Current |
|-----------|-----|-------------------|
| Authentication | ✅ Multiple methods | ⚠️ Static tokens only |
| Authorization | ✅ Full RBAC | ⚠️ Basic RBAC |
| Network policies | ✅ NetworkPolicy | ❌ Not supported |
| Secret management | ✅ Secret objects | ❌ Not supported |
| TLS | ✅ Auto certificates | ❌ Not supported |
| Auditing | ✅ Full audit chain | ⚠️ Basic audit logs |

#### 6. Ecosystem & Extensibility (Gap: Very Large)

| Capability | K8s | ClawFactory Current |
|-----------|-----|-------------------|
| Plugin mechanism | ✅ CRD + Operator | ❌ Not supported |
| Webhooks | ✅ Admission Webhook | ❌ Not supported |
| API versioning | ✅ Multi-version coexistence | ⚠️ v1 only |
| SDKs | ✅ Multi-language client-go | ⚠️ Python base class only |
| Package management | ✅ Helm | ❌ Not supported |
| Web UI | ✅ Dashboard | ❌ Not supported |

---

## Short-Term Roadmap (v0.2 — v0.5, 3-6 months)

Goal: From prototype to a usable single-machine production version.

### v0.2 — Core Hardening (1-2 months)

**Scheduler Enhancements**
- [ ] Implement true load balancing (least-connections based on current task count)
- [ ] Support scheduling affinity (agent label matching)
- [ ] Task timeout detection (auto-requeue assigned/running tasks on timeout)

**Workflow Enhancements**
- [ ] Per-node timeout configuration
- [ ] Workflow cancellation (cancel API + cascade cancel pending tasks)
- [ ] Pass upstream artifacts to downstream tasks on retry

**Reliability**
- [ ] Auto-requeue assigned/running tasks when agent goes offline
- [ ] Comprehensive transaction usage for all database operations
- [ ] Graceful shutdown

**CLI Enhancements**
- [ ] `claw workflow cancel <workflow_id>`
- [ ] `claw workflow list`
- [ ] `claw agent deregister <agent_id>`
- [ ] Colored output and progress display

### v0.3 — Observability (1 month)

**Monitoring Metrics**
- [ ] Prometheus metrics integration (`/metrics` endpoint)
- [ ] Key metrics: task throughput, scheduling latency, queue depth, online agent count
- [ ] Workflow execution time statistics

**Structured Logging**
- [ ] Replace `log` with `slog`
- [ ] Log level control (debug/info/warn/error)
- [ ] Request-level trace ID

**Event System**
- [ ] Define event types (AgentRegistered, TaskAssigned, WorkflowCompleted, etc.)
- [ ] Event storage and query API
- [ ] Webhook notifications (callback on workflow completion/failure)

### v0.4 — Security Hardening (1 month)

**Authentication**
- [ ] JWT token support (replace static tokens)
- [ ] Token expiration and refresh
- [ ] API key management interface

**TLS Support**
- [ ] HTTPS listener
- [ ] Auto-generated self-signed certificates (dev environment)
- [ ] External certificate configuration

**Secret Management**
- [ ] Encrypted storage for sensitive configuration
- [ ] Environment variable injection for agents

### v0.5 — Advanced Workflow Features (1-2 months)

**Conditional Branching**
- [ ] Node condition expressions (execute based on upstream output)
- [ ] `when` condition syntax

**Parameter System**
- [ ] Workflow-level parameter definitions
- [ ] Inter-node parameter references (`{{tasks.requirement.output.doc}}`)
- [ ] Default values and parameter validation

**Workflow Templates**
- [ ] Template definition and instantiation
- [ ] Parameterized templates
- [ ] Template versioning

**Scheduled Execution**
- [ ] Cron expression support
- [ ] Scheduled workflow triggers

---

## Mid-Term Roadmap (v0.6 — v1.0, 6-12 months)

Goal: From single-machine to distributed, production-ready deployment.

### v0.6 — Storage Abstraction (1-2 months)

- [ ] Abstract SQLite direct queries in TaskQueue.Dequeue to StateStore interface
- [ ] Implement PostgreSQL StateStore backend
- [ ] Implement Redis cache layer (hot data acceleration)
- [ ] Configurable storage backend switching

### v0.7 — Distributed Architecture (2-3 months)

- [ ] Stateless control plane (all state in external database)
- [ ] Multi-instance deployment (distributed locks via database)
- [ ] Distributed task queue (Redis or NATS based)
- [ ] Agent registry as a service

### v0.8 — Containerization & Cloud-Native (1-2 months)

- [ ] Dockerfile with multi-stage build
- [ ] Kubernetes Deployment/Service YAML
- [ ] Helm Chart
- [ ] Health and readiness probes (liveness/readiness)
- [ ] Configuration via ConfigMap/Secret

### v0.9 — Multi-Language SDKs (1-2 months)

- [ ] Go SDK (client library)
- [ ] Python SDK (replace current BaseAgent base class)
- [ ] TypeScript/JavaScript SDK
- [ ] Auto-generated SDKs (from OpenAPI spec)

### v1.0 — Production Ready (1-2 months)

- [ ] Complete E2E test suite
- [ ] Performance benchmarks
- [ ] Stress testing and capacity planning
- [ ] Operations manual and disaster recovery documentation
- [ ] Release process and CHANGELOG

---

## Long-Term Roadmap (v1.x — v2.0, 12-24 months)

Goal: Become a mature multi-agent orchestration platform.

### v1.x — Ecosystem Building

- [ ] Web Dashboard (workflow visualization, agent monitoring, log viewer)
- [ ] Plugin system (custom scheduling strategies, custom auth backends)
- [ ] Marketplace (shared workflow templates and agents)
- [ ] Multi-tenancy (namespace isolation)
- [ ] API gateway integration

### v2.0 — Intelligent Orchestration

- [ ] Adaptive scheduling (optimize decisions based on historical execution data)
- [ ] Agent auto-scaling (dynamically start/stop agents based on queue depth)
- [ ] Workflow auto-optimization (identify bottleneck nodes, suggest parallelization)
- [ ] A/B testing framework (compare different agents/models)
- [ ] Cost tracking and optimization (LLM API call cost statistics)

---

## Milestone Summary

| Version | Goal | Estimated Time |
|---------|------|---------------|
| v0.1 ✅ | Prototype validation: core features + property tests | Done |
| v0.2 | Core hardening: scheduler + reliability | 1-2 months |
| v0.3 | Observability: Prometheus + structured logging | 1 month |
| v0.4 | Security hardening: JWT + TLS | 1 month |
| v0.5 | Advanced workflows: conditional branching + templates | 1-2 months |
| v0.6 | Storage abstraction: PostgreSQL + Redis | 1-2 months |
| v0.7 | Distributed architecture: multi-instance + distributed queue | 2-3 months |
| v0.8 | Cloud-native: Docker + K8s + Helm | 1-2 months |
| v0.9 | Multi-language SDKs | 1-2 months |
| v1.0 | Production ready: E2E tests + benchmarks | 1-2 months |
| v1.x | Ecosystem: Dashboard + plugins + multi-tenancy | Ongoing |
| v2.0 | Intelligent orchestration: adaptive scheduling + auto-scaling | Long-term |
