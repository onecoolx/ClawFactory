# ClawFactory Technical Roadmap

## Current Version Assessment (v0.3.0 — Observability)

### Implemented Features

| Module | Status | Notes |
|--------|--------|-------|
| ARI Protocol Layer | ✅ Done | 19 HTTP endpoints (including 5 new in v0.3), Token auth middleware, task failure auto-retry |
| Registry | ✅ Done | Idempotent registration, heartbeat, offline detection, deregistration, offline task auto-requeue |
| Scheduler | ✅ Done | Capability matching, status filtering, least-active-tasks load balancing, assigned_to persistence |
| Policy Engine | ✅ Done | RBAC permissions, tool whitelist, rate limiting, audit logs |
| Workflow Engine | ✅ Done | DAG validation, root task scheduling, dependency checking, status derivation |
| Task Queue | ✅ Done | Priority ordering, capability matching, unfinished task recovery, removed SQLiteStore type assertion dependency |
| Shared Memory | ✅ Done | Filesystem storage, workflow isolation, metadata persistence |
| State Store (SQLite) | ✅ Done | 10 tables, WAL mode, foreign key constraints, 6 new methods (v0.2), 5 new methods (v0.3: events + webhooks) |
| Prometheus Metrics | ✅ Done | `/metrics` endpoint, 7 custom business metrics (New in v0.3) |
| Structured Logging | ✅ Done | slog JSON format, log level control, request-level TraceID (New in v0.3) |
| Event System | ✅ Done | 10 event types, SQLite persistence, query API (New in v0.3) |
| Webhook Notifications | ✅ Done | CRUD API, async dispatch, 5s timeout (New in v0.3) |
| CLI Tool | ✅ Done | workflow submit/status/artifacts, agent list/logs |
| Python Example Agents | ✅ Done | 4 agents (requirement/design/coding/testing) |
| Property Tests | ✅ Done | 42 property tests (v0.1: P1-P25, v0.2: P26-P33, v0.3: P34-P42) + unit tests, all passing |
| Documentation | ✅ Done | Bilingual (Chinese + English): architecture, API, getting-started, guide, examples, roadmap |

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
| Load balancing | ✅ Multiple strategies | ✅ Least-active-tasks load balancing (v0.2) |
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

#### 4. Observability (Gap: Medium)

| Capability | K8s | ClawFactory Current |
|-----------|-----|-------------------|
| Structured logging | ✅ | ✅ slog JSON format + log level control (v0.3) |
| Metrics monitoring | ✅ Prometheus | ✅ Prometheus `/metrics` endpoint + 7 custom metrics (v0.3) |
| Distributed tracing | ✅ OpenTelemetry | ⚠️ Request-level TraceID (v0.3, not full OpenTelemetry) |
| Event system | ✅ Events | ✅ 10 event types + query API + Webhook notifications (v0.3) |
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
| Webhooks | ✅ Admission Webhook | ✅ Event Webhook notifications (v0.3) |
| API versioning | ✅ Multi-version coexistence | ⚠️ v1 only |
| SDKs | ✅ Multi-language client-go | ⚠️ Python base class only |
| Package management | ✅ Helm | ❌ Not supported |
| Web UI | ✅ Dashboard | ❌ Not supported |

---

## Short-Term Roadmap (v0.2 — v0.5, 3-6 months)

Goal: From prototype to a usable single-machine production version.

### v0.2 — Core Hardening (1-2 months)

> **v0.2 tech debt fixes completed.** Added 8 new property tests (P26-P33), bringing the total to 33 property tests (with v0.1's P1-P25), all passing.

**Scheduler Enhancements**
- [x] Implement true load balancing (least-active-tasks strategy based on current task count) ✅
- [ ] Support scheduling affinity (agent label matching)
- [ ] Task timeout detection (auto-requeue assigned/running tasks on timeout)

**Workflow Enhancements**
- [ ] Per-node timeout configuration
- [ ] Workflow cancellation (cancel API + cascade cancel pending tasks)
- [ ] Pass upstream artifacts to downstream tasks on retry

**Reliability**
- [x] Auto-requeue assigned/running tasks when agent goes offline ✅
- [x] Task failure auto-retry (wired PolicyEngine.ShouldRetry into API handler) ✅
- [x] Remove TaskQueue type assertion dependency on SQLiteStore (originally planned for v0.6, completed early) ✅
- [x] Persist task assigned_to field to database ✅
- [ ] Comprehensive transaction usage for all database operations
- [ ] Graceful shutdown

**CLI Enhancements**
- [ ] `claw workflow cancel <workflow_id>`
- [ ] `claw workflow list`
- [ ] `claw agent deregister <agent_id>`
- [ ] Colored output and progress display

### v0.3 — Observability (1 month)

> **v0.3 observability completed.** Added 9 new property tests (P34-P42), bringing the total to 42 property tests (with v0.1's P1-P25 and v0.2's P26-P33), all passing.

**Monitoring Metrics**
- [x] Prometheus metrics integration (`/metrics` endpoint) ✅
- [x] Key metrics: task throughput, scheduling latency, queue depth, online agent count ✅
- [x] Workflow execution time statistics ✅

**Structured Logging**
- [x] Replace `log` with `slog` ✅
- [x] Log level control (debug/info/warn/error) ✅
- [x] Request-level trace ID ✅

**Event System**
- [x] Define event types (AgentRegistered, TaskAssigned, WorkflowCompleted, etc.) ✅
- [x] Event storage and query API ✅
- [x] Webhook notifications (callback on workflow completion/failure) ✅

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

**ARI Protocol Extension (Paving the Way for Hierarchical Architecture)**
- [ ] Agent role labels (strategic/tactical/execution/tool)
- [ ] Role-aware scheduling strategies

---

## Mid-Term Roadmap (v0.6 — v1.0, 6-12 months)

Goal: From single-machine to distributed, production-ready deployment.

### v0.6 — Storage Abstraction (1-2 months)

- [x] Abstract SQLite direct queries in TaskQueue.Dequeue to StateStore interface ✅ (completed early in v0.2)
- [ ] Implement PostgreSQL StateStore backend
- [ ] Implement Redis cache layer (hot data acceleration)
- [ ] Configurable storage backend switching
- [ ] S3/MinIO compatible interface for artifact storage (large file scenarios)

### v0.7 — Distributed Architecture (2-3 months)

- [ ] Stateless control plane (all state in external database)
- [ ] Multi-instance deployment (distributed locks via database)
- [ ] Distributed task queue (Redis or NATS based)
- [ ] Agent registry as a service
- [ ] Runtime DAG modification API (allow agents to append nodes to running workflows)

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

### v1.x — Ecosystem Building & Hierarchical Architecture

**Ecosystem Infrastructure**
- [ ] Web Dashboard (workflow visualization, agent monitoring, log viewer)
- [ ] Plugin system (custom scheduling strategies, custom auth backends)
- [ ] Marketplace (shared workflow templates and agents)
- [ ] Multi-tenancy (namespace isolation)
- [ ] API gateway integration

**Hierarchical Agent Architecture**
- [ ] ARI protocol extension: agent role labels (strategic/tactical/execution/tool)
- [ ] Strategic layer agent support: dynamically submit sub-workflows via API
- [ ] Hierarchical scheduling: differentiated scheduling strategies and permission isolation per layer
- [ ] Inter-agent communication: tactical layer coordination agents manage execution layer task dependencies

**Dynamic DAG Capabilities**
- [ ] Runtime DAG modification: allow agents to append nodes to running workflows via API
- [ ] DAG version snapshots: auto-save DAG state before modification, support rollback
- [ ] LLM-driven planning (experimental): strategic agents receive high-level goals and auto-generate execution DAGs

**Policy Engine Deepening**
- [ ] Declarative policy language (referencing OPA/Rego or AWS Cedar)
- [ ] Policy as Code: version management and audit trails
- [ ] Agent trust scoring: dynamically adjust permission boundaries based on historical behavior

**Memory Layer Evolution**
- [ ] Long-term memory: cross-workflow knowledge accumulation
- [ ] Vector database integration: semantic retrieval of historical artifacts and context

**Environment Adaptability**
- [ ] Multi-cluster federation: cross-region scheduling
- [ ] Hybrid cloud deployment support

### v2.0 — Intelligent Orchestration

- [ ] Adaptive scheduling (optimize decisions based on historical execution data)
- [ ] Agent auto-scaling (dynamically start/stop agents based on queue depth)
- [ ] Workflow auto-optimization (identify bottleneck nodes, suggest parallelization)
- [ ] A/B testing framework (compare different agents/models)
- [ ] Cost tracking and optimization (LLM API call cost statistics)
- [ ] Lightweight edge control plane (offline mode, edge computing scenarios)
- [ ] Full LLM-driven dynamic planning (from experimental to production-ready)

---

## Technical Evolution Direction Summary

ClawFactory's technical evolution follows these core principles:

1. **Platform over Framework**: Runs as a standalone orchestration service, with agents connecting via standard protocol (ARI), rather than embedding in application processes. This prioritizes control plane capabilities and protocol standardization.

2. **Incremental Complexity**: Static DAG → conditional branching → runtime modification → LLM-driven planning. Each step builds on the reliable foundation of the previous one. No leapfrogging complexity.

3. **Interface Abstraction Enables Evolution**: The abstract design of core interfaces (StateStore, TaskQueue, SharedMemory) ensures storage backends, message layers, and memory mechanisms can evolve independently without affecting upper-layer logic.

4. **Security Built-in, Not Bolted-on**: From v0.1's static Token + RBAC, to v0.4's JWT + TLS, to the long-term declarative policy language and trust scoring — security capabilities scale with platform maturity.

5. **Environment Adaptability**: SQLite single-machine dev → PostgreSQL on-premises → K8s cloud-native → multi-cluster federation → edge computing. The same codebase adapts to different deployment scenarios.

For detailed technical evolution direction, see the "Technical Evolution Direction" section in `docs/en/architecture.md`.

---

## Milestone Summary

| Version | Goal | Estimated Time |
|---------|------|---------------|
| v0.1 ✅ | Prototype validation: core features + 25 property tests | Done |
| v0.2 ✅ | Core hardening: load balancing + auto-retry + offline requeue + assigned_to persistence + TaskQueue interface fix (5 tech debts, 8 new property tests P26-P33) | Done |
| v0.3 ✅ | Observability: Prometheus metrics + slog structured logging + TraceID + event system + Webhook notifications (9 new property tests P34-P42) | Done |
| v0.4 | Security hardening: JWT + TLS | 1 month |
| v0.5 | Advanced workflows: conditional branching + templates + ARI role labels | 1-2 months |
| v0.6 | Storage abstraction: PostgreSQL + Redis + S3/MinIO artifact storage | 1-2 months |
| v0.7 | Distributed architecture: multi-instance + distributed queue + runtime DAG modification | 2-3 months |
| v0.8 | Cloud-native: Docker + K8s + Helm | 1-2 months |
| v0.9 | Multi-language SDKs | 1-2 months |
| v1.0 | Production ready: E2E tests + benchmarks | 1-2 months |
| v1.x | Ecosystem + hierarchical architecture + dynamic DAG + policy deepening + memory evolution | Ongoing |
| v2.0 | Intelligent orchestration: adaptive scheduling + auto-scaling + LLM-driven planning + edge computing | Long-term |
