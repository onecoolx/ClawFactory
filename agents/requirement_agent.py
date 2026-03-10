"""Requirement analysis agent: receives user requirements, outputs structured requirement documents"""
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
        self.llm = AsyncOpenAI(api_key=os.getenv("OPENAI_API_KEY", ""), base_url=os.getenv("OPENAI_BASE_URL") or None)

    async def execute_task(self, task: dict) -> dict:
        user_req = task.get("input", {}).get("user_requirement", "")
        prompt = (
            "Based on the following user requirements, produce a structured requirement analysis document "
            "(including functional requirements, non-functional requirements, and user stories):\n\n"
            f"{user_req}"
        )

        response = await self.llm.chat.completions.create(
            model=os.getenv("MODEL_NAME", "gpt-4o-mini"),
            messages=[{"role": "user", "content": prompt}],
        )
        result = response.choices[0].message.content
        return {"requirement_doc": result}


if __name__ == "__main__":
    token = os.getenv("CLAWFACTORY_TOKEN", "dev-token-001")
    agent = RequirementAgent(api_token=token)
    asyncio.run(agent.run())
