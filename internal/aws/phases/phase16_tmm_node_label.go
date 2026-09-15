package phases

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/state"
	"github.com/JLCode-tech/awsbnkctl/internal/intent"
)

// Phase16TMMNodeLabel finds the first node with label role=bnk, labels it with
// app=f5-tmm (idempotent via Patch), then resolves the EC2 instance ID from the
// node's providerID, falling back to DescribeInstances filtered on
// private-dns-name + instance-state-name=running + this cluster's vpc-id.
//
// Persists TMM_NODE_NAME and TMM_INSTANCE_ID to state.env.
// Dry-run: sets placeholder values and skips all k8s/EC2 mutations.
// SSO sentinel: CheckAuthOrDie at entry.
func Phase16TMMNodeLabel(ctx context.Context, cl *intent.Cluster, st *state.State, clients *Clients, dryRun bool) error {
	checkAuthOrDie(clients)
	name := cl.Metadata.Name
	fmt.Fprintf(os.Stderr, "[phase 16] TMM node label: cluster=%s\n", name)

	if dryRun {
		fmt.Fprintln(os.Stderr, "[phase 16] dry-run: would find node with role=bnk, label app=f5-tmm, resolve EC2 instance ID")
		st.Set("TMM_NODE_NAME", "dry-run-tmm-node")
		st.Set("TMM_INSTANCE_ID", "dry-run-i-xxx")
		return nil
	}

	if clients.K8s == nil {
		return fmt.Errorf("phase16: K8s client not attached (call AttachK8s after phase 11)")
	}

	// Find first node with label role=bnk.
	nodeList, err := clients.K8s.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: "role=bnk",
	})
	if err != nil {
		return fmt.Errorf("phase16: listing nodes with role=bnk: %w", err)
	}
	if len(nodeList.Items) == 0 {
		return fmt.Errorf("phase16: no nodes found with label role=bnk; ensure node group is ACTIVE and pattern=host-device auto-injected the label")
	}

	node := nodeList.Items[0]
	nodeName := node.Name
	fmt.Fprintf(os.Stderr, "[phase 16] found TMM target node: %s\n", nodeName)

	// Label the node app=f5-tmm (idempotent via strategic merge patch).
	patch := []byte(`{"metadata":{"labels":{"app":"f5-tmm"}}}`)
	if _, err := clients.K8s.CoreV1().Nodes().Patch(ctx, nodeName,
		types.StrategicMergePatchType, patch, metav1.PatchOptions{}); err != nil {
		return fmt.Errorf("phase16: patching node %s with app=f5-tmm: %w", nodeName, err)
	}
	fmt.Fprintf(os.Stderr, "[phase 16] labeled node %s with app=f5-tmm\n", nodeName)

	// Persist TMM_NODE_NAME.
	st.Set("TMM_NODE_NAME", nodeName)

	// Resolve the EC2 instance: the node's providerID is authoritative;
	// DescribeInstances by private DNS name is scoped to this cluster's VPC
	// because private names repeat across VPCs (seen live: a 10.0.1.181
	// jumphost in another VPC was picked and the ENI attach failed on it).
	instanceID, err := resolveEC2InstanceForNode(ctx, clients.EC2, &node, st.Get("VPC_ID"))
	if err != nil {
		return fmt.Errorf("phase16: resolving EC2 instance for node %s: %w", nodeName, err)
	}
	fmt.Fprintf(os.Stderr, "[phase 16] resolved EC2 instance: %s → %s\n", nodeName, instanceID)
	st.Set("TMM_INSTANCE_ID", instanceID)

	return st.Save()
}

// Phase16TMMNodeLabelDown is a no-op. The TMM node is destroyed by node group
// teardown in Phase10NodeGroupDown; relabeling is not required on down.
func Phase16TMMNodeLabelDown(_ context.Context, _ *intent.Cluster, st *state.State, _ *Clients) error {
	fmt.Fprintln(os.Stderr, "[phase 16 down] TMM node label: no-op (node destroyed by node group teardown)")
	st.Set("TMM_NODE_NAME", "")
	st.Set("TMM_INSTANCE_ID", "")
	return st.Save()
}

// instanceIDFromProviderID parses "aws:///<az>/<instance-id>" (the providerID
// the AWS cloud provider sets on every EKS node). Empty when the format is not
// that.
func instanceIDFromProviderID(providerID string) string {
	if !strings.HasPrefix(providerID, "aws:///") {
		return ""
	}
	id := providerID[strings.LastIndex(providerID, "/")+1:]
	if !strings.HasPrefix(id, "i-") {
		return ""
	}
	return id
}

// resolveEC2InstanceForNode returns the EC2 instance backing node: from its
// providerID when set, otherwise the running instance in vpcID whose
// private-dns-name equals the node name.
func resolveEC2InstanceForNode(ctx context.Context, ec2c EC2API, node *corev1.Node, vpcID string) (string, error) {
	if id := instanceIDFromProviderID(node.Spec.ProviderID); id != "" {
		return id, nil
	}
	return resolveEC2InstanceByPrivateDNS(ctx, ec2c, node.Name, vpcID)
}

// resolveEC2InstanceByPrivateDNS looks up a running EC2 instance whose
// private-dns-name matches the given node name (EKS nodes use private DNS as
// the k8s node name), limited to vpcID when non-empty. Returns the InstanceId
// or an error if not found.
func resolveEC2InstanceByPrivateDNS(ctx context.Context, ec2c EC2API, privateDNSName, vpcID string) (string, error) {
	filters := []ec2types.Filter{
		{Name: ptr("private-dns-name"), Values: []string{privateDNSName}},
		{Name: ptr("instance-state-name"), Values: []string{"running"}},
	}
	if vpcID != "" {
		filters = append(filters, ec2types.Filter{Name: ptr("vpc-id"), Values: []string{vpcID}})
	}
	out, err := ec2c.DescribeInstances(ctx, &ec2.DescribeInstancesInput{Filters: filters})
	if err != nil {
		return "", fmt.Errorf("ec2:DescribeInstances (private-dns-name=%s): %w", privateDNSName, err)
	}
	for _, r := range out.Reservations {
		for _, inst := range r.Instances {
			if inst.InstanceId != nil {
				return *inst.InstanceId, nil
			}
		}
	}
	return "", fmt.Errorf("no running EC2 instance found with private-dns-name=%s", privateDNSName)
}
