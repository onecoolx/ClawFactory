# ClawFactory API Reference

## Overview

ClawFactory exposes all functionality through the ARI (Agent Runtime Interface) HTTP REST protocol. All endpoints use JSON for data exchange.

- Default service address: `http://localhost:8080`
- All `/v1/*` endpoints require Bearer Token authentication
- The health check endpoint `/health` requires no authentication
- The Prometheus metrics endpoint `/metrics` requires no authentication (New in v0.3)
- All responses include an `X-Trace-ID` header (New in v0.3)

## Authentication

All endpoints except `/health` require an API Token in the HTTP header:

```
Authorization: Bearer <your-api-token>
```

The default development token is `dev-token-001`. Multiple tokens can be configured in the `api_tokens` array in `configs/config.json`.

Invalid or missing token response:

```json
{
  "error": {
    "code": "UNAUTHORIZED",
    "message": "invalid or missing API token"
  }
}
```

## Error Response Format

All error responses use a unified JSON format:

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Error description"
  }
}
```

### Error Codes

| HTTP Status | Code | Description |
|---|---|---|
| 400 | INVALID_REQUEST | Missing or malformed request parameters |
| 401 | UNAUTHORIZED | Invalid or missing API token |
| 403 | FORBIDDEN | Insufficient permissions |
| 404 | NOT_FOUND | Resource not found (agent, task, workflow) |
| 422 | INVALID_WORKFLOW | Invalid workflow definition (e.g., contains cycles) |
| 500 | INTERNAL_ERROR | Internal server error |

---

## Agent Endpoints

### POST /v1/register — Register Agent

Registers an agent with the platform. Duplicate registration with the same `name` + `version` returns the existing `agent_id` (idempotent).

**Request Body:**

```json
{
  "name": "requirement-agent",
  "capabilities": ["requirement_analysis"],
  "version": "1.0.0"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| name | string | Yes | Agent name |
| capabilities | string[] | Yes | Capability tags for task matching |
| version | string | Yes | Version string |

**Success Response (200):**

```json
{
  "agent_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

---

### POST /v1/heartbeat — Heartbeat

Agents send periodic heartbeats to indicate they are still online. Recommended interval: 30 seconds. Agents without a heartbeat for 90+ seconds are marked `offline`.

**Request Body:**

```json
{
  "agent_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

**Success Response (200):**

```json
{
  "status": "ok"
}
```

---

### GET /v1/tasks — Pull Task

Agents actively poll for tasks matching their capabilities. The scheduler matches tasks in the queue based on the agent's `capabilities`.

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| agent_id | string | Yes | Agent ID |

**Request Example:**

```
GET /v1/tasks?agent_id=a1b2c3d4-e5f6-7890-abcd-ef1234567890
```

**Response with task (200):**

```json
{
  "task_id": "task-001",
  "workflow_id": "wf-001",
  "type": "requirement_analysis",
  "capabilities": ["requirement_analysis"],
  "input": {
    "user_requirement": "Build a user management system"
  },
  "assigned": true
}
```

**Response without task (200):**

```json
{
  "assigned": false
}
```

---

### POST /v1/tasks/{taskID}/status — Update Task Status

Agents update task status during execution.

**Path Parameters:**

| Parameter | Description |
|-----------|-------------|
| taskID | Task ID |

**Request Body:**

```json
{
  "agent_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "status": "completed",
  "output": {
    "requirement_doc": "# Requirements\n..."
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| agent_id | string | Yes | Agent ID |
| status | string | Yes | New status: `running`, `completed`, `failed` |
| output | object | No | Task output (key-value pairs) |
| error | string | No | Failure reason (when status is failed) |

**Success Response (200):**

When `status` is `running` or `completed`:

```json
{
  "status": "ok"
}
```

**Failure Retry Behavior (New in v0.2):**

When `status` is `failed`, the platform automatically invokes the policy engine (`PolicyEngine.ShouldRetry`) to determine whether the task should be retried:

- If `retry_count < max_retries` (should retry): the task status is reset to `pending` and requeued, `retry_count` is incremented by 1, `assigned_to` is cleared, response:

```json
{
  "status": "retrying"
}
```

- If `retry_count >= max_retries` (should not retry): the task remains in `failed` status, `WorkflowEngine.OnTaskPermanentlyFailed` is called to handle workflow-level failure logic, response:

```json
{
  "status": "ok"
}
```

**Task State Transitions:**

```
pending → assigned → running → completed
                        ↓
                     failed ──→ pending (auto-retry if retry_count < max_retries)
                        ↓
                permanently_failed (retry_count >= max_retries)
```

When `status` is `completed`, the platform automatically triggers the workflow engine to check downstream task dependencies.

After retry requeue, the task's priority, capability tags, and input data remain unchanged, ensuring the retried task can be correctly scheduled. The maximum retry count is configured via `max_retries` in `configs/policy.json` (default: 3).

---

### POST /v1/authorize — Permission Check

Agents request permission verification before performing sensitive operations.

**Request Body:**

```json
{
  "agent_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "action": "call_tool",
  "resource": "llm_api"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| agent_id | string | Yes | Agent ID |
| action | string | Yes | Operation type: `call_tool`, `read_memory`, `write_memory` |
| resource | string | Yes | Resource name (tool name or resource path) |

**Allowed Response (200):**

```json
{
  "allowed": true
}
```

**Denied Response (403):**

```json
{
  "allowed": false,
  "reason": "tool not in whitelist for role"
}
```

---

### POST /v1/log — Report Log

Agents report execution logs.

**Request Body:**

```json
{
  "agent_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "task_id": "task-001",
  "level": "info",
  "message": "Starting requirement analysis",
  "timestamp": "2026-03-05T10:30:00Z"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| agent_id | string | Yes | Agent ID |
| task_id | string | No | Associated task ID |
| level | string | Yes | Log level: `info`, `warn`, `error` |
| message | string | Yes | Log content |
| timestamp | string | Yes | ISO 8601 timestamp |

**Success Response (200):**

```json
{
  "status": "ok"
}
```

---

## Admin Endpoints

Admin endpoints are used by the CLI tool and administrators. They also require Bearer Token authentication.

### GET /v1/admin/agents — List All Agents

**Success Response (200):**

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

### DELETE /v1/admin/agents/{agentID} — Deregister Agent

Sets the agent status to `deregistered`. Deregistered agents no longer receive tasks.

**Success Response (200):**

```json
{
  "status": "ok"
}
```

---

### GET /v1/admin/workflows — List Workflow Instances (New in v0.3.1)

Lists all workflow instances in the platform.

**Success Response (200):**

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

### POST /v1/admin/workflows — Submit Workflow

Submits a DAG workflow definition. The platform validates the DAG and automatically schedules root tasks.

**Request Body:**

```json
{
  "id": "software-dev-workflow",
  "name": "Software Development Workflow",
  "nodes": [
    {
      "id": "requirement",
      "type": "requirement_analysis",
      "capabilities": ["requirement_analysis"],
      "input": {"user_requirement": "Build a user management system"},
      "priority": 10
    }
  ],
  "edges": [
    {"from": "requirement", "to": "design"}
  ]
}
```

**Success Response (201):**

```json
{
  "instance_id": "wf-inst-001",
  "definition_id": "software-dev-workflow",
  "status": "running",
  "created_at": "2026-03-05T10:00:00Z",
  "updated_at": "2026-03-05T10:00:00Z"
}
```

**Error Response (422):** Returned when the DAG contains cycles or is otherwise invalid.

---

### GET /v1/admin/workflows/{workflowID} — Query Workflow Status

**Success Response (200):**

```json
{
  "instance_id": "wf-inst-001",
  "definition_id": "software-dev-workflow",
  "status": "running",
  "created_at": "2026-03-05T10:00:00Z",
  "updated_at": "2026-03-05T10:05:00Z"
}
```

Workflow status values: `running`, `completed`, `failed`

---

### GET /v1/admin/workflows/{workflowID}/artifacts — Query Workflow Artifacts

**Success Response (200):**

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

### GET /v1/admin/agents/{agentID}/logs — Query Agent Logs

**Success Response (200):**

```json
[
  {
    "agent_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "task_id": "task-001",
    "level": "info",
    "message": "Starting requirement analysis",
    "timestamp": "2026-03-05T10:30:00Z"
  }
]
```

---

### GET /v1/admin/events — List Events (New in v0.3)

Query platform events with optional filtering by event type and entity ID.

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| event_type | string | No | Filter by event type |
| entity_id | string | No | Filter by entity ID |

**Request Examples:**

```
GET /v1/admin/events?event_type=task.completed
GET /v1/admin/events?entity_id=a1b2c3d4
GET /v1/admin/events?event_type=agent.registered&entity_id=a1b2c3d4
```

**Success Response (200):**

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

**Supported Event Types:**

| Event Type | Description |
|-----------|-------------|
| agent.registered | Agent registered |
| agent.deregistered | Agent deregistered |
| agent.offline | Agent went offline |
| task.assigned | Task assigned to agent |
| task.completed | Task completed |
| task.failed | Task failed |
| task.requeued | Task requeued |
| workflow.submitted | Workflow submitted |
| workflow.completed | Workflow completed |
| workflow.failed | Workflow failed |

---

### POST /v1/admin/webhooks — Create Webhook Subscription (New in v0.3)

Creates a webhook subscription. When a matching event occurs, the platform sends an HTTP POST callback to the subscription URL.

**Request Body:**

```json
{
  "url": "https://example.com/webhook",
  "event_types": ["task.completed", "workflow.completed", "workflow.failed"]
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| url | string | Yes | Callback URL (must be a valid HTTP/HTTPS URL) |
| event_types | string[] | Yes | List of event types to subscribe to |

**Success Response (201):**

```json
{
  "webhook_id": "wh-001",
  "url": "https://example.com/webhook",
  "event_types": ["task.completed", "workflow.completed", "workflow.failed"],
  "created_at": "2026-03-10T10:00:00Z"
}
```

**Webhook Callback Request Body Format:**

When a matching event occurs, the platform sends an HTTP POST request to the subscription URL with the following body:

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

**Notes:**

- Webhook callbacks use a 5-second timeout to prevent slow external systems from affecting platform performance
- Failed callbacks (non-2xx response or timeout) are logged but not retried, and do not block event processing
- Webhook dispatch runs asynchronously in separate goroutines

---

### GET /v1/admin/webhooks — List Webhook Subscriptions (New in v0.3)

**Success Response (200):**

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

### DELETE /v1/admin/webhooks/{webhookID} — Delete Webhook Subscription (New in v0.3)

**Path Parameters:**

| Parameter | Description |
|-----------|-------------|
| webhookID | Webhook subscription ID |

**Success Response (200):**

```json
{
  "status": "ok"
}
```

---

## Prometheus Metrics Endpoint (New in v0.3)

### GET /metrics — Prometheus Metrics

Exposes Prometheus-format monitoring metrics. This endpoint requires no Token authentication and is only available when `metrics_enabled=true` in the configuration.

**Response Format:** Prometheus text exposition format

**Custom Business Metrics:**

| Metric Name | Type | Labels | Description |
|------------|------|--------|-------------|
| clawfactory_tasks_total | Counter | status | Task status change count (completed, failed, pending, assigned) |
| clawfactory_scheduling_duration_seconds | Histogram | — | Task scheduling latency (time from creation to assignment) |
| clawfactory_queue_depth | Gauge | — | Current pending task queue depth |
| clawfactory_agents_online | Gauge | — | Current number of online agents |
| clawfactory_workflow_duration_seconds | Histogram | — | Workflow execution time (from creation to completion/failure) |
| clawfactory_http_requests_total | Counter | method, path, status_code | HTTP request count |
| clawfactory_http_request_duration_seconds | Histogram | method, path | HTTP request latency |

---

## StateStore Interface Methods (New in v0.2)

The following StateStore interface methods were added in v0.2 to support load balancing, automatic retry, offline task requeue, and task assignment persistence. These methods are used internally by platform components (TaskQueue, Scheduler, API handler, heartbeat goroutine) and are not directly exposed as HTTP endpoints.

### ListPendingTasks

```go
ListPendingTasks(capabilities []string) ([]model.Task, error)
```

Returns pending tasks matching the specified capability tags.

- Only returns tasks with `status = "pending"`
- Each returned task has at least one capability tag matching the input `capabilities`
- Results are ordered by priority descending; ties are broken by creation time ascending
- Replaces the type assertion in TaskQueue for the `Dequeue` operation

### ListUnfinishedTasks

```go
ListUnfinishedTasks() ([]model.Task, error)
```

Returns all unfinished tasks.

- Returns tasks with `status` of `pending`, `assigned`, or `running`
- Results are ordered by priority descending
- Used for restoring unfinished tasks on platform restart (`RestoreUnfinished` operation)

### CountAgentActiveTasks

```go
CountAgentActiveTasks(agentID string) (int, error)
```

Returns the number of active tasks for a given agent.

- Counts tasks where `assigned_to` matches and `status` is `assigned` or `running`
- Used by the Scheduler for load balancing: tasks are assigned to the agent with the lowest load

### IncrementTaskRetryCount

```go
IncrementTaskRetryCount(taskID string) error
```

Atomically increments the retry count of a task.

- Performs an atomic `retry_count = retry_count + 1` update
- Also updates the `updated_at` timestamp
- Returns an error if the task does not exist
- Used during automatic task failure retry to track retry attempts

### GetTasksByAssignee

```go
GetTasksByAssignee(agentID string) ([]model.Task, error)
```

Returns all active tasks assigned to a given agent.

- Returns tasks where `assigned_to` matches and `status` is `assigned` or `running`
- Used to query an agent's unfinished tasks when it goes offline, so they can be requeued

### UpdateTaskAssignment

```go
UpdateTaskAssignment(taskID string, agentID string) error
```

Updates the assignment information for a task.

- Updates the `assigned_to` field and `updated_at` timestamp
- Returns an error if the task does not exist
- An empty `agentID` string clears the assignment (used when requeuing tasks)
- Used by the Scheduler to persist assignment after task allocation, and to clear assignment during requeue

---

## StateStore Interface Methods (New in v0.3)

The following StateStore interface methods were added in v0.3 to support the event system and webhook notifications.

### SaveEvent

```go
SaveEvent(event model.Event) error
```

Persists a platform event to the events table.

- Events contain event_id, event_type, entity_type, entity_id, detail (JSON), created_at
- Used by EventBus for synchronous event persistence during publishing

### ListEvents

```go
ListEvents(filter model.EventFilter) ([]model.Event, error)
```

Queries events with optional filtering by event_type and entity_id.

- When `filter.EventType` is non-empty, only events matching that type are returned
- When `filter.EntityID` is non-empty, only events matching that entity are returned
- Both filters can be used simultaneously (AND logic)

### SaveWebhook

```go
SaveWebhook(webhook model.WebhookSubscription) error
```

Saves a webhook subscription configuration.

- event_types are serialized as a JSON string for storage

### ListWebhooks

```go
ListWebhooks() ([]model.WebhookSubscription, error)
```

Lists all webhook subscriptions.

- event_types are deserialized from JSON string to []string

### DeleteWebhook

```go
DeleteWebhook(webhookID string) error
```

Deletes the specified webhook subscription.

---

## StateStore Interface Methods (New in v0.3.1)

The following StateStore interface methods were added in v0.3.1 to support transaction protection and workflow instance listing.

### ListWorkflowInstances

```go
ListWorkflowInstances() ([]model.WorkflowInstance, error)
```

Returns all workflow instances.

- Results are ordered by creation time descending
- Used by the `GET /v1/admin/workflows` endpoint and the `claw workflow list` CLI command

### RunInTransaction

```go
RunInTransaction(fn func(tx *sql.Tx) error) error
```

Executes the given function within a database transaction.

- If `fn` returns nil, the transaction is committed; otherwise it is rolled back
- Used for multi-step database operations requiring atomicity (e.g., task retry, offline task requeue)

### RequeueTaskTx

```go
RequeueTaskTx(tx *sql.Tx, taskID string) error
```

Requeues a task within a transaction: sets status to `pending` and clears `assigned_to`.

- Must be called within a transaction provided by `RunInTransaction`
- Used for task requeue when an agent goes offline

### RetryTaskTx

```go
RetryTaskTx(tx *sql.Tx, taskID string) error
```

Retries a task within a transaction: increments `retry_count`, sets status to `pending`, clears `assigned_to`.

- Must be called within a transaction provided by `RunInTransaction`
- Used for automatic task retry on failure

---

## Response Headers (New in v0.3)

All responses include an `X-Trace-ID` header with a unique UUID identifier, used to correlate logs produced by the same request across different components.

```
X-Trace-ID: 550e8400-e29b-41d4-a716-446655440000
```

---

## Health Check

### GET /health — Health Check

No authentication required. Used for monitoring and load balancer probes.

**Success Response (200):**

```json
{
  "status": "ok"
}
```
