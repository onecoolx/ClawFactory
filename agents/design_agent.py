"""设计智能体：基于需求文档，输出技术设计方案"""
import asyncio
import os
from openai import AsyncOpenAI
from base_agent import BaseAgent


class DesignAgent(BaseAgent):
    def __init__(self, api_token: str):
        super().__init__(
            name="design-agent",
            capabilities=["detailed_design"],
            version="1.0.0",
            api_token=api_token,
        )
        self.llm = AsyncOpenAI(api_key=os.getenv("OPENAI_API_KEY", ""))

    async def execute_task(self, task: dict) -> dict:
        input_data = task.get("input", {})
        prompt = f"请根据以下需求文档，输出详细的技术设计方案（包含架构设计、数据模型、接口设计、技术选型）：\n\n{input_data}"

        response = await self.llm.chat.completions.create(
            model="gpt-4o-mini",
            messages=[{"role": "user", "content": prompt}],
        )
        result = response.choices[0].message.content
        return {"design_doc": result}


if __name__ == "__main__":
    token = os.getenv("CLAWFACTORY_TOKEN", "dev-token-001")
    agent = DesignAgent(api_token=token)
    asyncio.run(agent.run())
