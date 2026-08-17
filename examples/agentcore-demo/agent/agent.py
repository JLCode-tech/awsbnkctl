from agentcore import Agent, run_cli
import os

# Create an AgentCore instance
# The tools (including our BNK-protected MCP server) are injected by the AgentCore Runtime
# based on the agentcore.yaml configuration during deployment.
agent = Agent(
    name="Financial Advisor",
    instructions="""
    You are a financial advisor agent. 
    You have access to the internal enterprise finance MCP tool to look up forecasts.
    Always provide the user with the most up to date forecast for the requested quarter.
    """
)

if __name__ == "__main__":
    # Start the local development server or AWS Lambda handler
    run_cli(agent)
