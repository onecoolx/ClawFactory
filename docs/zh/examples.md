# ClawFactory 示例文档

## 示例一：软件开发工作流

这是 ClawFactory 自带的经典示例，模拟一个完整的软件开发流程：需求分析 → 设计 → 编码 → 测试。

### 工作流定义

文件：`configs/software-dev-workflow.json`

```json
{
    "id": "software-dev-workflow",
    "name": "软件开发工作流",
    "nodes": [
        {
            "id": "requirement",
            "type": "requirement_analysis",
            "capabilities": ["requirement_analysis"],
            "input": {"user_requirement": "构建一个用户管理系统"},
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

### DAG 结构

```
需求分析 → 设计 → 编码 → 测试
```

这是一个线性工作流，每个阶段依赖前一个阶段的完成。

### 运行步骤

```bash
# 1. 启动平台
./bin/clawfactory

# 2. 启动四个智能体（分别在不同终端）
cd agents
python requirement_agent.py
python design_agent.py
python coding_agent.py
python testing_agent.py

# 3. 提交工作流
./bin/claw workflow submit configs/software-dev-workflow.json

# 4. 观察执行过程
./bin/claw workflow status <workflow_id>
./bin/claw agent list
```

### 执行过程

1. 平台验证 DAG 合法性
2. `requirement` 节点入度为 0，创建任务放入队列
3. `requirement-agent` 拉取到任务，调用 LLM 分析需求，输出需求文档
4. 需求任务完成后，`design` 节点的依赖满足，创建设计任务
5. `design-agent` 拉取设计任务，输出技术设计方案
6. 依次类推，直到测试任务完成
7. 所有任务完成，工作流状态变为 `completed`

---

## 示例二：并行处理工作流

展示 DAG 的并行执行能力。

### 工作流定义

```json
{
    "id": "parallel-workflow",
    "name": "并行处理工作流",
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

### DAG 结构

```
              ┌→ 文本分析 ─┐
数据获取 ─────┤             ├──→ 结果合并
              └→ 图像分析 ─┘
```

`analyze-text` 和 `analyze-image` 可以并行执行，只有两者都完成后 `merge-results` 才会被调度。

---

## 示例三：自定义 Python 智能体

### 数据处理智能体

```python
"""数据处理智能体：从输入中读取数据并进行清洗和转换"""
import asyncio
import json
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

        await self.report_log(task_id, "info", f"开始处理数据源: {source}")

        # 模拟数据处理
        processed_records = 1000
        await self.report_log(task_id, "info", f"处理完成，共 {processed_records} 条记录")

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

### 多能力智能体

一个智能体可以注册多个能力标签，处理多种类型的任务：

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
            raise ValueError(f"未知任务类型: {task_type}")

    async def _analyze_text(self, task: dict) -> dict:
        # 文本分析逻辑
        return {"analysis": "completed"}

    async def _process_data(self, task: dict) -> dict:
        # 数据处理逻辑
        return {"processed": "true"}

    async def _generate_report(self, task: dict) -> dict:
        # 报告生成逻辑
        return {"report": "generated"}
```

---

## 示例四：使用 curl 手动测试 ARI 协议

不需要编写代码，直接用 `curl` 测试平台的所有接口。

### 健康检查

```bash
curl http://localhost:8080/health
```

### 注册智能体

```bash
curl -X POST http://localhost:8080/v1/register \
  -H "Authorization: Bearer dev-token-001" \
  -H "Content-Type: application/json" \
  -d '{"name":"test-agent","capabilities":["testing"],"version":"1.0.0"}'
```

响应：

```json
{"agent_id":"xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"}
```

### 发送心跳

```bash
curl -X POST http://localhost:8080/v1/heartbeat \
  -H "Authorization: Bearer dev-token-001" \
  -H "Content-Type: application/json" \
  -d '{"agent_id":"<上一步返回的 agent_id>"}'
```

### 拉取任务

```bash
curl "http://localhost:8080/v1/tasks?agent_id=<agent_id>" \
  -H "Authorization: Bearer dev-token-001"
```

### 更新任务状态

```bash
curl -X POST http://localhost:8080/v1/tasks/<task_id>/status \
  -H "Authorization: Bearer dev-token-001" \
  -H "Content-Type: application/json" \
  -d '{"agent_id":"<agent_id>","status":"completed","output":{"result":"done"}}'
```

### 权限检查

```bash
curl -X POST http://localhost:8080/v1/authorize \
  -H "Authorization: Bearer dev-token-001" \
  -H "Content-Type: application/json" \
  -d '{"agent_id":"<agent_id>","action":"call_tool","resource":"llm_api"}'
```

### 上报日志

```bash
curl -X POST http://localhost:8080/v1/log \
  -H "Authorization: Bearer dev-token-001" \
  -H "Content-Type: application/json" \
  -d '{"agent_id":"<agent_id>","task_id":"<task_id>","level":"info","message":"测试日志","timestamp":"2026-03-05T10:00:00Z"}'
```

### 提交工作流

```bash
curl -X POST http://localhost:8080/v1/admin/workflows \
  -H "Authorization: Bearer dev-token-001" \
  -H "Content-Type: application/json" \
  -d @configs/software-dev-workflow.json
```

### 查看智能体列表

```bash
curl http://localhost:8080/v1/admin/agents \
  -H "Authorization: Bearer dev-token-001"
```

---

## 示例五：复杂 DAG 工作流

一个更复杂的工作流示例，包含多层依赖和并行分支：

```json
{
    "id": "ml-pipeline",
    "name": "机器学习流水线",
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

### DAG 结构

```
数据采集 → 数据清洗 → 特征工程 ─┬→ 训练模型A ─┬→ 模型评估 → 部署
                                └→ 训练模型B ─┘
```

两个模型训练任务可以并行执行，评估阶段等待两个模型都训练完成后才开始。
