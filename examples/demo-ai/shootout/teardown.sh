#!/usr/bin/env bash
# teardown.sh — tear down the bnk-demo-ai cluster AND the manual shootout
# add-ons that `awsbnkctl down` does not know about.
#
# Order matters: delete the LoadBalancer Services/Gateways FIRST so the AWS LB
# Controller deprovisions their internal NLBs while the cluster still exists —
# otherwise the NLBs leak when the cluster is deleted. Then release the
# jumphost-egress EIP, drop the extra IAM policy, and `awsbnkctl down`.
#
# Run from the repo root:  bash examples/demo-ai/shootout/teardown.sh
set -uo pipefail
CFG=examples/demo-ai/cluster.yaml
CLUSTER=bnk-demo-ai
REGION=ap-southeast-2
NODE_ROLE="${CLUSTER}-eks-node-role"
: "${AWS_PROFILE:?set AWS_PROFILE}"
export KUBECONFIG="$PWD/.awsbnkctl/${CLUSTER}/kubeconfig"
k(){ kubectl "$@"; }

echo "==> 1/4 delete shootout LoadBalancer Services + Gateways (deprovision NLBs)"
k -n sagemaker-proxy delete svc haproxy-lb haproxy-vllm-lb --ignore-not-found 2>/dev/null
k -n sagemaker-proxy delete gateway ai-gateway ai-gateway-vllm --ignore-not-found 2>/dev/null
echo "    waiting ~60s for the AWS LB Controller to delete the NLBs..."
for i in $(seq 1 12); do
  N=$(aws elbv2 describe-load-balancers --region "$REGION" --query "length(LoadBalancers[?contains(DNSName,'haproxy') || contains(DNSName,'envoysag')])" --output text 2>/dev/null)
  echo "    shootout NLBs remaining: ${N:-?}"; [ "${N:-0}" = "0" ] && break; sleep 5
done

echo "==> 2/4 release jumphost-egress EIP(s) tagged for this cluster"
for A in $(aws ec2 describe-addresses --region "$REGION" --filters "Name=tag:awsbnkctl:cluster,Values=${CLUSTER}" --query 'Addresses[].AllocationId' --output text 2>/dev/null); do
  ASSOC=$(aws ec2 describe-addresses --allocation-ids "$A" --region "$REGION" --query 'Addresses[0].AssociationId' --output text 2>/dev/null)
  [ "$ASSOC" != "None" ] && [ -n "$ASSOC" ] && aws ec2 disassociate-address --association-id "$ASSOC" --region "$REGION" 2>/dev/null
  aws ec2 release-address --allocation-id "$A" --region "$REGION" 2>/dev/null && echo "    released EIP $A"
done

echo "==> 3/4 remove the extra SageMaker-invoke inline policy (so down can delete the node role)"
aws iam delete-role-policy --role-name "$NODE_ROLE" --policy-name SageMakerInvokeDemo 2>/dev/null && echo "    removed SageMakerInvokeDemo" || true

echo "==> 4/4 awsbnkctl down (cluster + nodegroups + VPC + SageMaker + jumphost + forge)"
./awsbnkctl down --config "$CFG" --yes

echo "==> sweep: any leftover NLBs / EIPs tagged ${CLUSTER}?"
aws elbv2 describe-load-balancers --region "$REGION" --query "LoadBalancers[?contains(DNSName,'haproxy')||contains(DNSName,'envoysag')].DNSName" --output text 2>/dev/null
aws ec2 describe-addresses --region "$REGION" --filters "Name=tag:awsbnkctl:cluster,Values=${CLUSTER}" --query 'Addresses[].AllocationId' --output text 2>/dev/null
echo "    (empty output above = clean)"
