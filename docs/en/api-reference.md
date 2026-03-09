# ClawFactory API Reference

## Overview

ClawFactory exposes all functionality through the ARI (Agent Runtime Interface) HTTP REST protocol. All endpoints use JSON for data exchange.

- Default service address: `http://localhost:8080`
- All `/v1/*` endpoints require Bearer Token authentication
- The health check endpoint `/health` requires no authentication

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

## Health Check

### GET /health — Health Check

No authentication required. Used for monitoring and load balancer probes.

**Success Response (200):**

```json
{
  "status": "ok"
}
```
