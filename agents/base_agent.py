"""ARI 协议客户端基类"""
import asyncio
import httpx
import time
from abc import ABC, abstractmethod


class BaseAgent(ABC):
    """ARI 协议客户端基类，实现注册、心跳、任务拉取等通用逻辑"""

    def __init__(self, name: str, capabilities: list[str], version: str, api_token: str):
        self.name = name
        self.capabilities = capabilities
        self.version = version
        self.api_token = api_token
        self.agent_id: str | None = None
        self.base_url = "http://localhost:8080/v1"
        self.heartbeat_interval = 30  # seconds

    def _headers(self) -> dict:
        return {
            "Authorization": f"Bearer {self.api_token}",
            "Content-Type": "application/json",
        }

    async def register(self) -> str:
        """注册到平台，返回 agent_id"""
        async with httpx.AsyncClient() as client:
            resp = await client.post(
                f"{self.base_url}/register",
                json={"name": self.name, "capabilities": self.capabilities, "version": self.version},
                headers=self._headers(),
            )
            resp.raise_for_status()
            data = resp.json()
            self.agent_id = data["agent_id"]
            return self.agent_id

    async def heartbeat(self) -> None:
        """发送心跳"""
        async with httpx.AsyncClient() as client:
            await client.post(
                f"{self.base_url}/heartbeat",
                json={"agent_id": self.agent_id},
                headers=self._headers(),
            )

    async def pull_task(self) -> dict | None:
        """拉取任务，无任务返回 None"""
        async with httpx.AsyncClient() as client:
            resp = await client.get(
                f"{self.base_url}/tasks",
                params={"agent_id": self.agent_id},
                headers=self._headers(),
            )
            resp.raise_for_status()
            data = resp.json()
            if data.get("assigned"):
                return data
            return None

    async def update_task_status(
        self, task_id: str, status: str, output: dict = None, error: str = None
    ) -> None:
        """更新任务状态"""
        payload = {"agent_id": self.agent_id, "status": status}
        if output:
            payload["output"] = output
        if error:
            payload["error"] = error
        async with httpx.AsyncClient() as client:
            await client.post(
                f"{self.base_url}/tasks/{task_id}/status",
                json=payload,
                headers=self._headers(),
            )

    async def report_log(self, task_id: str, level: str, message: str) -> None:
        """上报日志"""
        async with httpx.AsyncClient() as client:
            await client.post(
                f"{self.base_url}/log",
                json={
                    "agent_id": self.agent_id,
                    "task_id": task_id,
                    "level": level,
                    "message": message,
                    "timestamp": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
                },
                headers=self._headers(),
            )

    @abstractmethod
    async def execute_task(self, task: dict) -> dict:
        """执行任务，返回产出物 dict。子类实现。"""
        ...

    async def _heartbeat_loop(self):
        """心跳定时器"""
        while True:
            try:
                await self.heartbeat()
            except Exception as e:
                print(f"[{self.name}] 心跳失败: {e}")
            await asyncio.sleep(self.heartbeat_interval)

    async def _task_loop(self):
        """任务拉取循环"""
        while True:
            try:
                task = await self.pull_task()
                if task:
                    task_id = task["task_id"]
                    await self.report_log(task_id, "info", f"开始执行任务 {task_id}")
                    await self.update_task_status(task_id, "running")
                    try:
                        output = await self.execute_task(task)
                        await self.update_task_status(task_id, "completed", output=output)
                        await self.report_log(task_id, "info", f"任务 {task_id} 完成")
                    except Exception as e:
                        await self.update_task_status(task_id, "failed", error=str(e))
                        await self.report_log(task_id, "error", f"任务 {task_id} 失败: {e}")
            except Exception as e:
                print(f"[{self.name}] 任务拉取失败: {e}")
            await asyncio.sleep(5)

    async def run(self):
        """主循环：注册 → 心跳 + 拉取任务"""
        print(f"[{self.name}] 正在注册...")
        agent_id = await self.register()
        print(f"[{self.name}] 注册成功: {agent_id}")

        await asyncio.gather(self._heartbeat_loop(), self._task_loop())
