"""测试智能体：基于代码和需求，生成测试用例"""
import asyncio
import os
from openai import AsyncOpenAI
from base_agent import BaseAgent


class TestingAgent(BaseAgent):
    def __init__(self, api_token: str):
        super().__init__(
            name="testing-agent",
            capabilities=["testing"],
            version="1.0.0",
            api_token=api_token,
        )
        self.llm = AsyncOpenAI(api_key=os.getenv("OPENAI_API_KEY", ""))

    async def execute_task(self, task: dict) -> dict:
        input_data = task.get("input", {})
        prompt = f"请根据以下代码和需求，生成完整的测试用例（包含单元测试和集成测试）：\n\n{input_data}"

        response = await self.llm.chat.completions.create(
            model="gpt-4o-mini",
            messages=[{"role": "user", "content": prompt}],
        )
        result = response.choices[0].message.content
        return {"test_cases": result}


if __name__ == "__main__":
    token = os.getenv("CLAWFACTORY_TOKEN", "dev-token-001")
    agent = TestingAgent(api_token=token)
    asyncio.run(agent.run())
