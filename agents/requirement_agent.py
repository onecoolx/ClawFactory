"""需求分析智能体：接收用户需求描述，输出结构化需求文档"""
import asyncio
import os
from openai import AsyncOpenAI
from base_agent import BaseAgent


class RequirementAgent(BaseAgent):
    def __init__(self, api_token: str):
        super().__init__(
            name="requirement-agent",
            capabilities=["requirement_analysis"],
            version="1.0.0",
            api_token=api_token,
        )
        self.llm = AsyncOpenAI(api_key=os.getenv("OPENAI_API_KEY", ""))

    async def execute_task(self, task: dict) -> dict:
        user_req = task.get("input", {}).get("user_requirement", "")
        prompt = f"请根据以下用户需求，输出结构化的需求分析文档（包含功能需求、非功能需求、用户故事）：\n\n{user_req}"

        response = await self.llm.chat.completions.create(
            model="gpt-4o-mini",
            messages=[{"role": "user", "content": prompt}],
        )
        result = response.choices[0].message.content
        return {"requirement_doc": result}


if __name__ == "__main__":
    token = os.getenv("CLAWFACTORY_TOKEN", "dev-token-001")
    agent = RequirementAgent(api_token=token)
    asyncio.run(agent.run())
