import requests
import json

# This script simulates an external agent running outside of AWS (e.g., in a local rig, Azure, or GCP).
# It attempts to invoke the internal AWS Bedrock FinanceAgent securely via the F5 BNK Gateway.

BNK_GATEWAY_URL = "http://10.0.10.100/v1/agentcore/invoke"
HEADERS = {
    "Host": "bnk-ingress.aws.corp",
    "Content-Type": "application/json",
    # In a real environment, F5 BNK would validate an API key, OAuth token, or mTLS certificate here
    "Authorization": "Bearer external-agent-token-123" 
}

payload = {
    "agentId": "FinanceAgent",
    "prompt": "What is the financial forecast for Q3?"
}

print(f"External Agent -> Sending Request to BNK Gateway: {BNK_GATEWAY_URL}")
try:
    response = requests.post(BNK_GATEWAY_URL, headers=HEADERS, json=payload)
    print(f"Response Status: {response.status_code}")
    print(f"Response Body: {response.text}")
except Exception as e:
    print(f"Connection Error (BNK might be blocking/dropping): {e}")
