#!/bin/bash
# Traffic generator for BNK AI Gateway Demo

# Egress (Test A) - AgentCore to internal MCP tool
function generate_egress() {
    echo "Running Egress Test (AgentCore -> BNK -> MCP)"
    # Invoking the deployed agent. The agent will call the MCP tool.
    cd /Users/j.lucia/Code/github/awsbnkctl/examples/agentcore-demo/agent
    AWS_PROFILE=Users-292785712872 npx agentcore invoke "What is the financial forecast for Q3?" --non-interactive || true
}

# Ingress (Test B) - External Agent to AgentCore
function generate_ingress() {
    echo "Running Ingress Test (External Agent -> BNK -> AgentCore)"
    cd /Users/j.lucia/Code/github/awsbnkctl/examples/agentcore-demo
    python external-agent.py || true
}

while true; do
    echo "--- Generating BNK AI Traffic ---"
    date
    generate_egress
    generate_ingress
    echo "---------------------------------"
    sleep 300
done
