#!/usr/bin/env python3
"""
Minimal MCP financial tool for the F5 BNK + AgentCore demo.

Implements MCP over streamable-http (stateless), the transport used by both
AWS AgentCore Gateway and the external-agent.py JSON-RPC caller.

Endpoints (as seen by the client through BNK):
  POST /v1/mcp/forecast
  GET  /v1/mcp/forecast/.well-known/agent-card.json

The A2A agent card is served at the well-known path for Forge discovery.
"""

import json
import random

from fastmcp import FastMCP
from starlette.applications import Starlette
from starlette.requests import Request
from starlette.responses import Response
from starlette.routing import Mount, Route


def _make_agent_card() -> dict:
    return {
        "name": "Finance Tool",
        "description": "Financial forecasting tool",
        "url": "http://bnk-ingress.bnk-demo.internal/v1/mcp/forecast",
        "version": "1.0.0",
        "protocolVersion": "a2a-v1.0",
        "capabilities": {"streaming": False, "pushNotifications": False},
        "skills": [
            {
                "id": "financial-forecast",
                "name": "Financial Forecast",
                "description": "Provides a simple financial forecast for a symbol.",
                "tags": ["finance", "forecast"],
                "examples": ["forecast AAPL for 30 days"],
            }
        ],
        "defaultInputModes": ["text"],
        "defaultOutputModes": ["text"],
        "provider": {"name": "F5 BNK Demo"},
    }


async def agent_card(request: Request):
    # Forge probes via the k8s service proxy API, which loads text/plain
    # responses, converts them through json.loads() -> str(), and then passes
    # the result to json.loads() again. Returning a JSON-encoded JSON string
    # makes the final json.loads() in forge produce the original dict.
    return Response(content=json.dumps(json.dumps(_make_agent_card())), media_type="text/plain")


mcp = FastMCP("finance-tool")


@mcp.tool()
def forecast(symbol: str, days: int = 30) -> str:
    """Provides a simple financial forecast for a symbol."""
    growth = random.randint(5, 20)
    return f"Forecast for {symbol} over {days} days: up (expected growth: {growth}%)."


if __name__ == "__main__":
    import uvicorn

    mcp_app = mcp.http_app(
        transport="streamable-http",
        path="/",
        json_response=True,
        stateless_http=True,
    )

    app = Starlette(
        debug=False,
        lifespan=mcp_app.router.lifespan_context,
        routes=[
            Route("/.well-known/agent-card.json", agent_card),
            Mount("/", app=mcp_app),
        ],
    )
    uvicorn.run(app, host="0.0.0.0", port=8080)
