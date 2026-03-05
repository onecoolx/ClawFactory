# ClawFactory Examples

## Example 1: Software Development Workflow

This is the built-in classic example that simulates a complete software development process: Requirements → Design → Coding → Testing.

### Workflow Definition

File: `configs/software-dev-workflow.json`

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
        },
        {
            "id": "design",
            "type": "detailed_design",
            "capabilities": ["detailed_design"],
            "priority": 8
        },
        {
            "id": "coding",
            "type": "coding",
            "capabilities": ["coding"],
            "priority": 6
        },
        {
            "id": "testing",
            "type": "testing",
            "capabilities": ["testing"],
            "priority": 4
        }
    ],
    "edges": [
        {"from": "requirement", "to": "design"},
        {"from": "design", "to": "coding"},
        {"from": "coding", "to": "testing"}
    ]
}
```

### DAG Structure

```
Requirements → Design → Coding → Testing
```

A linear workflow where each stage depends on the previous one.

### Running Steps

```bash
# 1. Start the platform
./bin/clawfactory

# 2. Start four agents (in separate terminals)
cd agents
python requirement_agent.py
python design_agent.py
python coding_agent.py
python testing_agent.py

# 3. Submit the workflow
./bin/claw workflow submit configs/software-dev-workflow.json

# 4. Monitor execution
./bin/claw workflow status <workflow_id>
./bin/claw agent list
```

### Execution Process

1. Platform validates DAG legality
2. `requirement` node has in-degree 0, so a task is created and queued
3. `requirement-agent` pulls the task, calls LLM to analyze requirements, outputs requirement document
4. After requirement task completes, `design` node dependencies are satisfied, design task is created
5. `design-agent` pulls the design task, outputs technical design
6. Process continues until the testing task completes
7. All tasks complete, workflow status becomes `completed`

---

## Example 2: Parallel Processing Workflow

Demonstrates the DAG's parallel execution capability.

### Workflow Definition

```json
{
    "id": "parallel-workflow",
    "name": "Parallel Processing Workflow",
    "nodes": [
        {
            "id": "fetch-data",
            "type": "data_fetch",
            "capabilities": ["data_processing"],
            "input": {"source": "database"},
            "priority": 10
        },
        {
            "id": "analyze-text",
            "type": "text_analysis",
            "capabilities": ["text_analysis"],
            "priority": 8
        },
        {
            "id": "analyze-image",
            "type": "image_analysis",
            "capabilities": ["image_analysis"],
            "priority": 8
        },
        {
            "id": "merge-results",
            "type": "result_merge",
            "capabilities": ["data_processing"],
            "priority": 6
        }
    ],
    "edges": [
        {"from": "fetch-data", "to": "analyze-text"},
        {"from": "fetch-data", "to": "analyze-image"},
        {"from": "analyze-text", "to": "merge-results"},
        {"from": "analyze-image", "to": "merge-results"}
    ]
}
```

### DAG Structure

```
              ┌→ Text Analysis  ─┐
Data Fetch ───┤                   ├──→ Merge Results
              └→ Image Analysis ─┘
```

`analyze-text` and `analyze-image` can execute in parallel. `merge-results` is only scheduled after both complete.

---

## Example 3: Custom Python Agent

### Data Processing Agent

```python
"""Data processing agent: reads input data and performs cleaning and transformation"""
import asyncio
import os
from base_agent import BaseAgent


class DataProcessingAgent(BaseAgent):
    def __init__(self, api_token: str):
        super().__init__(
            name="data-processing-agent",
            capabilities=["data_processing"],
            version="1.0.0",
            api_token=api_token,
        )

    async def execute_task(self, task: dict) -> dict:
        task_id = task["task_id"]
        input_data = task.get("input", {})
        source = input_data.get("source", "unknown")

        await self.report_log(task_id, "info", f"Processing data source: {source}")

        # Simulate data processing
        processed_records = 1000
        await self.report_log(task_id, "info", f"Done, {processed_records} records processed")

        return {
            "processed_count": str(processed_records),
            "source": source,
            "status": "clean",
        }


if __name__ == "__main__":
    token = os.getenv("CLAWFACTORY_TOKEN", "dev-token-001")
    agent = DataProcessingAgent(api_token=token)
    asyncio.run(agent.run())
```

### Multi-Capability Agent

A single agent can register multiple capability tags to handle different task types:

```python
class MultiSkillAgent(BaseAgent):
    def __init__(self, api_token: str):
        super().__init__(
            name="multi-skill-agent",
            capabilities=["text_analysis", "data_processing", "report_generation"],
            version="1.0.0",
            api_token=api_token,
        )

    async def execute_task(self, task: dict) -> dict:
        task_type = task.get("type", "")
        
        if task_type == "text_analysis":
            return await self._analyze_text(task)
        elif task_type == "data_processing":
            return await self._process_data(task)
        elif task_type == "report_generation":
            return await self._generate_report(task)
        else:
            raise ValueError(f"Unknown task type: {task_type}")

    async def _analyze_text(self, task: dict) -> dict:
        return {"analysis": "completed"}

    async def _process_data(self, task: dict) -> dict:
        return {"processed": "true"}

    async def _generate_report(self, task: dict) -> dict:
        return {"report": "generated"}
```

---

## Example 4: Testing ARI Protocol with curl

Test all platform endpoints directly with `curl` without writing any code.

### Health Check

```bash
curl http://localhost:8080/health
```

### Register an Agent

```bash
curl -X POST http://localhost:8080/v1/register \
  -H "Authorization: Bearer dev-token-001" \
  -H "Content-Type: application/json" \
  -d '{"name":"test-agent","capabilities":["testing"],"version":"1.0.0"}'
```

### Send Heartbeat

```bash
curl -X POST http://localhost:8080/v1/heartbeat \
  -H "Authorization: Bearer dev-token-001" \
  -H "Content-Type: application/json" \
  -d '{"agent_id":"<agent_id from previous step>"}'
```

### Pull Task

```bash
curl "http://localhost:8080/v1/tasks?agent_id=<agent_id>" \
  -H "Authorization: Bearer dev-token-001"
```

### Update Task Status

```bash
curl -X POST http://localhost:8080/v1/tasks/<task_id>/status \
  -H "Authorization: Bearer dev-token-001" \
  -H "Content-Type: application/json" \
  -d '{"agent_id":"<agent_id>","status":"completed","output":{"result":"done"}}'
```

### Submit Workflow

```bash
curl -X POST http://localhost:8080/v1/admin/workflows \
  -H "Authorization: Bearer dev-token-001" \
  -H "Content-Type: application/json" \
  -d @configs/software-dev-workflow.json
```

### List Agents

```bash
curl http://localhost:8080/v1/admin/agents \
  -H "Authorization: Bearer dev-token-001"
```

---

## Example 5: Complex DAG Workflow

A more complex workflow with multiple dependency layers and parallel branches:

```json
{
    "id": "ml-pipeline",
    "name": "Machine Learning Pipeline",
    "nodes": [
        {"id": "collect", "type": "data_collection", "capabilities": ["data_processing"], "priority": 10},
        {"id": "clean", "type": "data_cleaning", "capabilities": ["data_processing"], "priority": 9},
        {"id": "feature-eng", "type": "feature_engineering", "capabilities": ["data_processing"], "priority": 8},
        {"id": "train-model-a", "type": "model_training", "capabilities": ["ml_training"], "input": {"model": "random_forest"}, "priority": 7},
        {"id": "train-model-b", "type": "model_training", "capabilities": ["ml_training"], "input": {"model": "xgboost"}, "priority": 7},
        {"id": "evaluate", "type": "model_evaluation", "capabilities": ["ml_evaluation"], "priority": 6},
        {"id": "deploy", "type": "model_deployment", "capabilities": ["deployment"], "priority": 5}
    ],
    "edges": [
        {"from": "collect", "to": "clean"},
        {"from": "clean", "to": "feature-eng"},
        {"from": "feature-eng", "to": "train-model-a"},
        {"from": "feature-eng", "to": "train-model-b"},
        {"from": "train-model-a", "to": "evaluate"},
        {"from": "train-model-b", "to": "evaluate"},
        {"from": "evaluate", "to": "deploy"}
    ]
}
```

### DAG Structure

```
Data Collection → Cleaning → Feature Eng ─┬→ Train Model A ─┬→ Evaluation → Deploy
                                           └→ Train Model B ─┘
```

The two model training tasks can execute in parallel. The evaluation stage waits for both models to finish training.
