import argparse
import json
import urllib.request
import urllib.error

parser = argparse.ArgumentParser(description="External Agent Simulator")
parser.add_argument("--prompt", type=str, required=True, help="The prompt to send")
args = parser.parse_args()

symbol = "NVDA" if "NVDA" in args.prompt else "AAPL"

BNK_GATEWAY_URL = "http://10.0.10.100/v1/mcp/forecast"
HEADERS = {
    "Host": "bnk-ingress.bnk-demo.internal",
    "Content-Type": "application/json",
    "Accept": "application/json",
    # Checked by mcp-server.py for basic auth
    "Authorization": "Bearer external-agent-token-123" 
}

payload = {
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
        "name": "forecast",
        "arguments": {
            "symbol": symbol,
            "days": 30
        }
    }
}

data = json.dumps(payload).encode('utf-8')
req = urllib.request.Request(BNK_GATEWAY_URL, data=data, headers=HEADERS, method='POST')

print(f"External Agent -> Sending Request to BNK Gateway: {BNK_GATEWAY_URL}")
try:
    with urllib.request.urlopen(req) as response:
        print(f"Response Status: {response.status}")
        print(f"Response Body: {response.read().decode('utf-8')}")
except urllib.error.HTTPError as e:
    print(f"HTTP Error: {e.code} - {e.reason}")
    print(f"Response Body: {e.read().decode('utf-8')}")
except urllib.error.URLError as e:
    print(f"Connection Error (BNK might be blocking/dropping): {e.reason}")
