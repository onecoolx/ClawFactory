# ClawFactory User Guide

## Overview

ClawFactory is a multi-agent orchestration platform that uses DAG workflows to define collaboration relationships between agents, automatically scheduling and managing task execution. This guide covers daily usage, configuration management, workflow definition, and agent development.

## CLI Command Reference

`claw` is the ClawFactory command-line management tool, supporting workflow and agent management.

### Global Flags

| Flag | Description | Default |
|------|-------------|---------|
| --url | ClawFactory service address | http://localhost:8080 |
| --token | API Token | dev-token-001 |
| --output-json | Output in JSON format | false |

### Workflow Commands

```bash
# Submit workflow
claw workflow submit <workflow.json>

# Query workflow status
claw workflow status <workflow_id>

# Query workflow artifacts
claw workflow artifacts <workflow_id>
```

### Agent Commands

```bash
# List all agents
claw agent list

# View agent logs
claw agent logs <agent_id>
```

## Workflow Definition

### DAG Structure

Workflows are defined in JSON format. The core is a Directed Acyclic Graph (DAG) composed of nodes and edges.

```json
{
  "id": "my-workflow",
  "name": "My Workflow",
  "nodes": [...],
  "edges": [...]
}
```

### Node Definition

Each node represents a task with a type and required capabilities:

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

- `id`: Unique node identifier within the workflow
- `type`: Task type, identifies the business meaning
- `capabilities`: Required capability tags; the scheduler uses these to match agents
- `input`: Task input parameters as key-value pairs
- `priority`: Priority value; higher numbers are scheduled first (default 0)

### Edge Definition

Edges define dependencies between nodes:

```json
{
  "from": "step-1",
  "to": "step-2"
}
```

This means `step-2` depends on `step-1` and will only be scheduled after `step-1` completes.

### DAG Validation Rules

The platform automatically validates DAG legality upon workflow submission:

1. Must not contain cycles (circular dependencies)
2. Edge-referenced node IDs must exist in the node list
3. Must have at least one root node (in-degree 0)

Validation failure returns HTTP 422.

### Workflow Execution Flow

1. After submission, the platform creates a workflow instance with status `running`
2. All root nodes (in-degree 0) are created as tasks and placed in the task queue
3. Agents poll for matching tasks and execute them
4. Upon task completion, the platform checks if downstream node dependencies are satisfied
5. Downstream nodes with satisfied dependencies are created as new tasks
6. When all tasks complete, workflow status becomes `completed`
7. If any task permanently fails (exceeds max retries), workflow status becomes `failed`

## Configuration Management

### Platform Configuration

Configuration file path priority:
1. Path specified by `CLAWFACTORY_CONFIG` environment variable
2. Default path `configs/config.json`

Environment variables override config file values:

| Environment Variable | Config Field |
|---------------------|-------------|
| CLAWFACTORY_PORT | port |
| CLAWFACTORY_DB_PATH | db_path |
| CLAWFACTORY_DATA_DIR | data_dir |
| CLAWFACTORY_CONFIG | Config file path |
| CLAWFACTORY_POLICY_PATH | Policy config file path |

### Policy Configuration

The policy configuration file `configs/policy.json` controls platform security and scheduling behavior.

#### Retry Policy

```json
{
  "max_retries": 3
}
```

Failed tasks are automatically retried up to `max_retries` times. After exceeding the limit, tasks are marked as permanently failed.

#### Heartbeat Configuration

```json
{
  "heartbeat_interval_seconds": 30,
  "heartbeat_timeout_multiplier": 3
}
```

- `heartbeat_interval_seconds`: Recommended heartbeat interval (seconds)
- `heartbeat_timeout_multiplier`: Timeout multiplier; agents without heartbeat for `interval × multiplier` seconds are marked offline

#### Roles and Permissions

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

- `resource`: Resource pattern, supports `*` wildcard
- `actions`: List of allowed operations

#### Tool Whitelist

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

- `allowed_tools`: List of permitted tools
- `rate_limit_per_minute`: Maximum calls per minute

## Agent Development Guide

### ARI Protocol Overview

Agents communicate with the platform through the ARI (Agent Runtime Interface) HTTP REST protocol. The protocol is language-agnostic — any programming language capable of sending HTTP requests can develop agents.

### Agent Lifecycle

```
Start → Register → [Heartbeat Loop + Task Polling Loop] → Deregister
```

1. **Register**: Call `POST /v1/register` to register with the platform
2. **Heartbeat**: Periodically call `POST /v1/heartbeat` to maintain online status
3. **Poll Tasks**: Poll `GET /v1/tasks` to get matching tasks
4. **Execute Tasks**: Update status to `running` upon receiving a task; update to `completed` or `failed` after execution
5. **Report Logs**: Report logs via `POST /v1/log` during execution

### Python Agent Development

The project provides a Python base class `BaseAgent` that encapsulates all ARI protocol communication logic.

#### Creating a Custom Agent

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
        """Implement your task logic here"""
        input_data = task.get("input", {})
        
        # Report log
        await self.report_log(task["task_id"], "info", "Starting task processing")
        
        # Execute business logic
        result = do_something(input_data)
        
        # Return output
        return {"output_key": result}


if __name__ == "__main__":
    token = os.getenv("CLAWFACTORY_TOKEN", "dev-token-001")
    agent = MyAgent(api_token=token)
    asyncio.run(agent.run())
```

#### BaseAgent Methods

| Method | Description |
|--------|-------------|
| `register()` | Register with the platform, returns agent_id |
| `heartbeat()` | Send heartbeat |
| `pull_task()` | Pull task, returns None if no task available |
| `update_task_status(task_id, status, output, error)` | Update task status |
| `report_log(task_id, level, message)` | Report log |
| `run()` | Main loop (register + heartbeat + task polling) |
| `execute_task(task)` | Abstract method, implement task logic in subclass |

### Other Languages

The ARI protocol is based on standard HTTP REST, so any language can develop agents. Core steps:

1. Implement an HTTP client with `Authorization: Bearer <token>` header
2. Call `POST /v1/register` to register
3. Periodically call `POST /v1/heartbeat` (recommended 30-second interval)
4. Poll `GET /v1/tasks?agent_id=xxx` for tasks
5. Call `POST /v1/tasks/{taskID}/status` to update task status after execution

## Data Storage

### SQLite Database

The platform uses SQLite for persistent storage. The database file defaults to `data/clawfactory.db`.

Stored data includes:
- Agent registration information and status
- Task information and execution status
- Workflow definitions and instances
- Logs
- Artifact metadata
- Audit logs
- Tool call rate limits

### Artifact Storage

Task artifacts are stored as files in the data directory:

```
data/artifacts/{workflow_id}/{task_id}/{artifact_name}
```

Artifact metadata (path, creation time, etc.) is stored in SQLite.

## v0.2 New Features

### Load Balancing

The platform uses a load-based task assignment strategy. When an agent polls for tasks via `GET /v1/tasks`, the scheduler checks the current load (number of assigned + running tasks) of all online agents with matching capabilities. A task is only assigned to the requesting agent if it has the lowest (or tied for lowest) active task count among candidates; otherwise, an empty result is returned, allowing a less-loaded agent to pick up the task.

In practice:
- Tasks are always assigned to the agent with the fewest active tasks
- When multiple agents have the same load, the first to request gets the task
- When only one matching agent exists, it receives the task regardless of load

### Automatic Retry

When an agent updates a task status to `failed`, the platform automatically checks the retry policy:

1. If `retry_count < max_retries` (default max_retries=3), the task is automatically requeued:
   - `retry_count` is incremented by 1
   - Task status is reset to `pending`
   - `assigned_to` is cleared, awaiting rescheduling
   - Original priority, capabilities, and input data are preserved
   - API returns `{"status": "retrying"}`

2. If `retry_count >= max_retries`, the task is permanently failed:
   - Task status remains `failed`
   - Triggers `OnTaskPermanentlyFailed` in the workflow engine, potentially failing the entire workflow
   - API returns `{"status": "ok"}`

`max_retries` is configured in `configs/policy.json`.

### Offline Agent Task Requeue

The platform detects agent availability through heartbeats. When an agent fails to send a heartbeat for over 90 seconds (default: heartbeat_interval × timeout_multiplier = 30 × 3), it is marked as `offline`.

At that point, all `assigned` and `running` tasks belonging to that agent are automatically requeued:
- Task status is reset to `pending`
- `assigned_to` field is cleared
- Original priority, capabilities, input data, and `retry_count` are preserved

This ensures that even if an agent crashes or loses network connectivity, its unfinished tasks are not lost and will be picked up by other available agents.

## Troubleshooting

### Platform Won't Start

1. Check if the port is in use: `lsof -i :8080`
2. Check data directory permissions: ensure `data/` is writable
3. Check config file format: ensure valid JSON

### Agent Can't Register

1. Check if the platform is running: `curl http://localhost:8080/health`
2. Check if the API token is correct
3. Check network connectivity

### Tasks Not Being Scheduled

1. Confirm agent status is `online`: `claw agent list`
2. Confirm agent `capabilities` overlap with task `capabilities`
3. Confirm task status is `pending`
4. Check load balancing: if a less-loaded matching agent exists, the current agent won't receive tasks

### Tasks Retrying Repeatedly

1. Check the agent's task execution logic for bugs
2. View agent logs: `claw agent logs <agent_id>`
3. Verify that `max_retries` in `configs/policy.json` is set appropriately

### Workflow Submission Fails

1. Check JSON format validity
2. Check if the DAG contains cycles
3. Check if edge-referenced node IDs exist
