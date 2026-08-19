import os
import logging
from mcp.client.streamable_http import streamablehttp_client
from strands.tools.mcp.mcp_client import MCPClient

logger = logging.getLogger(__name__)

def get_finance_tool_mcp_client() -> MCPClient | None:
    """Returns an MCP Client for the finance-tool remote MCP server."""
    url = "http://bnk-ingress.bnk-demo.internal/v1/mcp/forecast"
    return MCPClient(lambda: streamablehttp_client(url))

def get_all_remote_mcp_clients() -> list[MCPClient]:
    """Returns all configured remote MCP clients."""
    clients = [get_finance_tool_mcp_client()]
    return [c for c in clients if c is not None]
