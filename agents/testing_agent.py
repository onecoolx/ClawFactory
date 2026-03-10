"""Testing agent: based on code and requirements, generates test cases"""
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
        self.llm = AsyncOpenAI(api_key=os.getenv("OPENAI_API_KEY", ""), api_url=os.getenv("OPENAI_API_URL", ""))

    async def execute_task(self, task: dict) -> dict:
        input_data = task.get("input", {})
        prompt = (
            "Based on the following code and requirements, generate complete test cases "
            "(including unit tests and integration tests):\n\n"
            f"{input_data}"
        )

        response = await self.llm.chat.completions.create(
            model=os.getenv("MODULE_NAME", "gpt-4o-mini"),
            messages=[{"role": "user", "content": prompt}],
        )
        result = response.choices[0].message.content
        return {"test_cases": result}


if __name__ == "__main__":
    token = os.getenv("CLAWFACTORY_TOKEN", "dev-token-001")
    agent = TestingAgent(api_token=token)
    asyncio.run(agent.run())
