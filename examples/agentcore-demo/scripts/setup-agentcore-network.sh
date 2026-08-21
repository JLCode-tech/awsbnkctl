#!/bin/bash
set -euo pipefail

# Provisions the AWS network seam so an external AgentCore runtime can reach
# the BNK VIP over VPC/ENI routing.
#
# Usage:
#   AWS_PROFILE=<profile> KUBECONFIG=<path> ./scripts/setup-agentcore-network.sh [cluster-name]
#
# The cluster name defaults to "bnk-agentcore-demo". All AWS resource IDs are
# discovered at runtime from tags or from the live Gateway object. Nothing is
# hardcoded.

CLUSTER_NAME="${1:-bnk-agentcore-demo}"
REGION="${AWS_REGION:-$(aws configure get region 2>/dev/null || echo "ap-southeast-2")}"
export AWS_DEFAULT_REGION="$REGION"

BNK_DNS_ZONE="${BNK_DNS_ZONE:-bnk-demo.internal}"
BNK_INGRESS_HOST="${BNK_INGRESS_HOST:-bnk-ingress.${BNK_DNS_ZONE}}"

KUBECONFIG="${KUBECONFIG:-}"
if [ -n "$KUBECONFIG" ]; then
    export KUBECONFIG
fi

echo "Discovering VPC ID for cluster $CLUSTER_NAME..."
VPC_ID=$(aws ec2 describe-vpcs \
    --filters \
        "Name=tag:awsbnkctl:cluster,Values=${CLUSTER_NAME}" \
        "Name=tag:awsbnkctl:component,Values=vpc" \
    --query "Vpcs[0].VpcId" --output text)
if [ "$VPC_ID" == "None" ] || [ -z "$VPC_ID" ]; then
    echo "Could not find VPC for cluster $CLUSTER_NAME"
    exit 1
fi
echo "Found VPC: $VPC_ID"

echo "Discovering VPC CIDR..."
VPC_CIDR=$(aws ec2 describe-vpcs \
    --vpc-ids "$VPC_ID" \
    --query "Vpcs[0].CidrBlock" --output text)
echo "Found VPC CIDR: $VPC_CIDR"

echo "Discovering BNK VIP from Gateway bnk-agentcore-demo-gateway..."
if ! VIP_IP=$(kubectl get gateway bnk-agentcore-demo-gateway -n default -o jsonpath='{.spec.addresses[0].value}' 2>/dev/null); then
    echo "Failed to read Gateway. Is KUBECONFIG set and pointing at the right cluster?"
    exit 1
fi
if [ -z "$VIP_IP" ]; then
    echo "Gateway bnk-agentcore-demo-gateway has no spec.addresses[0].value"
    exit 1
fi
echo "Found BNK VIP: $VIP_IP"

echo "Discovering private subnets for the AgentCore runtime..."
AGENT_SUBNETS_JSON=$(aws ec2 describe-subnets \
    --filters \
        "Name=vpc-id,Values=$VPC_ID" \
        "Name=tag:awsbnkctl:cluster,Values=$CLUSTER_NAME" \
        "Name=tag:awsbnkctl:component,Values=subnet-private" \
    --query 'sort_by(Subnets, &AvailabilityZone)[].SubnetId' --output json | jq -c .)
if [ "$AGENT_SUBNETS_JSON" == "[]" ] || [ -z "$AGENT_SUBNETS_JSON" ]; then
    echo "Could not find private subnets tagged awsbnkctl:component=subnet-private"
    exit 1
fi
echo "Found private subnets: $AGENT_SUBNETS_JSON"

AGENT_SG_NAME="${CLUSTER_NAME}-agent"
echo "Checking for existing Agent SG ($AGENT_SG_NAME)..."
AGENT_SG_ID=$(aws ec2 describe-security-groups \
    --filters \
        "Name=group-name,Values=$AGENT_SG_NAME" \
        "Name=vpc-id,Values=$VPC_ID" \
    --query "SecurityGroups[0].GroupId" --output text 2>/dev/null || echo "None")

if [ "$AGENT_SG_ID" == "None" ] || [ -z "$AGENT_SG_ID" ]; then
    echo "Creating Agent SG..."
    AGENT_SG_ID=$(aws ec2 create-security-group \
        --group-name "$AGENT_SG_NAME" \
        --description "AgentCore Runtime SG for $CLUSTER_NAME" \
        --vpc-id "$VPC_ID" \
        --query "GroupId" --output text)
    aws ec2 create-tags --resources "$AGENT_SG_ID" \
        --tags Key=Name,Value="$AGENT_SG_NAME" Key=awsbnkctl:cluster,Value="$CLUSTER_NAME" Key=awsbnkctl:managed,Value=true
    echo "Created Agent SG: $AGENT_SG_ID"
else
    echo "Found existing Agent SG: $AGENT_SG_ID"
fi

echo "Authorizing ingress from VPC CIDR ($VPC_CIDR) to Agent SG ($AGENT_SG_ID) on port 8080..."
aws ec2 authorize-security-group-ingress \
    --group-id "$AGENT_SG_ID" \
    --protocol tcp --port 8080 --cidr "$VPC_CIDR" 2>/dev/null || echo "Ingress rules already exist."

TMM_SG_NAME="${CLUSTER_NAME}-bnk-data"
echo "Discovering TMM external ENI SG ($TMM_SG_NAME)..."
TMM_SG_ID=$(aws ec2 describe-security-groups \
    --filters \
        "Name=group-name,Values=$TMM_SG_NAME" \
        "Name=vpc-id,Values=$VPC_ID" \
    --query "SecurityGroups[0].GroupId" --output text || echo "None")

if [ "$TMM_SG_ID" == "None" ] || [ -z "$TMM_SG_ID" ]; then
    echo "Could not find TMM SG $TMM_SG_NAME"
    exit 1
fi
echo "Found TMM SG: $TMM_SG_ID"

echo "Authorizing ingress from Agent SG ($AGENT_SG_ID) to TMM SG ($TMM_SG_ID)..."
aws ec2 authorize-security-group-ingress \
    --group-id "$TMM_SG_ID" \
    --ip-permissions \
    "IpProtocol=tcp,FromPort=80,ToPort=80,UserIdGroupPairs=[{GroupId=$AGENT_SG_ID,Description=agentcore-to-bnk-vip-http}]" \
    "IpProtocol=tcp,FromPort=443,ToPort=443,UserIdGroupPairs=[{GroupId=$AGENT_SG_ID,Description=agentcore-to-bnk-vip-https}]" 2>/dev/null || echo "Ingress rules already exist."

echo "Checking for existing Route 53 private hosted zone $BNK_DNS_ZONE..."
ZONE_ID=$(aws route53 list-hosted-zones-by-name \
    --dns-name "$BNK_DNS_ZONE" \
    --query "HostedZones[?Name=='${BNK_DNS_ZONE}.'].Id" --output text)

if [ -z "$ZONE_ID" ] || [ "$ZONE_ID" == "None" ]; then
    echo "Creating private hosted zone $BNK_DNS_ZONE..."
    CALLER_REF="setup-$(date +%s)-$$"
    ZONE_ID=$(aws route53 create-hosted-zone \
        --name "$BNK_DNS_ZONE" \
        --caller-reference "$CALLER_REF" \
        --hosted-zone-config Comment="Private zone for $CLUSTER_NAME BNK VIP",PrivateZone=true \
        --vpc VPCRegion="$REGION",VPCId="$VPC_ID" \
        --query "HostedZone.Id" --output text)
    echo "Created hosted zone: $ZONE_ID"
else
    echo "Found existing hosted zone: $ZONE_ID"
    VPC_ASSOC=$(aws route53 get-hosted-zone \
        --id "$ZONE_ID" \
        --query "VPCs[?VPCId=='$VPC_ID'].VPCId" --output text 2>/dev/null || echo "")
    if [ -z "$VPC_ASSOC" ] || [ "$VPC_ASSOC" == "None" ]; then
        echo "Associating VPC with hosted zone..."
        aws route53 associate-vpc-with-hosted-zone \
            --hosted-zone-id "$ZONE_ID" \
            --vpc VPCRegion="$REGION",VPCId="$VPC_ID" || echo "Already associated or error"
    fi
fi

echo "Creating/Updating DNS record $BNK_INGRESS_HOST -> $VIP_IP..."
aws route53 change-resource-record-sets \
    --hosted-zone-id "$ZONE_ID" \
    --change-batch '{
      "Comment": "Upsert BNK Ingress VIP",
      "Changes": [
        {
          "Action": "UPSERT",
          "ResourceRecordSet": {
            "Name": "'"$BNK_INGRESS_HOST"'",
            "Type": "A",
            "TTL": 60,
            "ResourceRecords": [{"Value": "'"$VIP_IP"'"}]
          }
        }
      ]
    }'

cat <<EOF > .agentcore-network-env.json
{
  "CLUSTER_NAME": "$CLUSTER_NAME",
  "VPC_ID": "$VPC_ID",
  "AGENT_SG_ID": "$AGENT_SG_ID",
  "TMM_SG_ID": "$TMM_SG_ID",
  "BNK_DNS_ZONE": "$BNK_DNS_ZONE",
  "BNK_INGRESS_HOST": "$BNK_INGRESS_HOST",
  "VIP_IP": "$VIP_IP",
  "AGENT_SUBNETS_JSON": $(echo "$AGENT_SUBNETS_JSON" | python3 -c 'import sys,json; print(json.dumps(sys.stdin.read().strip()))')
}
EOF

AGENT_SGS_JSON="[\"$AGENT_SG_ID\"]"

echo "Rendering agentcore.json from template..."
sed -e "s|{{ .VpcId }}|$VPC_ID|g" \
    -e "s|{{ .AgentSubnetIdsJson }}|$AGENT_SUBNETS_JSON|g" \
    -e "s|{{ .AgentSecurityGroupIdsJson }}|$AGENT_SGS_JSON|g" \
    agent/agentcore/agentcore.json.tmpl > agent/agentcore/agentcore.json

echo "Rendering harness.json from template..."
sed -e "s|{{ .BnkIngressHost }}|$BNK_INGRESS_HOST|g" \
    agent/app/FinanceAgentV2/harness.json.tmpl > agent/app/FinanceAgentV2/harness.json

# The AgentCore Gateway tool ARN is dynamic and cannot be templated without
# knowing the deployed gateway ARN. The supported CLI path is to add it after
# rendering the base harness.
echo "Adding BnkGatewayTool to harness via agentcore CLI..."
(cd agent && npx agentcore add tool --harness FinanceAgentV2 --type agentcore_gateway --name BnkGatewayTool --gateway BnkGateway --outbound-auth awsIam)

echo "AgentCore network seam setup complete."
echo "Variables written to .agentcore-network-env.json"
