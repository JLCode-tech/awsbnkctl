package phases

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/awsmw"
	"github.com/JLCode-tech/awsbnkctl/internal/aws/state"
	"github.com/JLCode-tech/awsbnkctl/internal/aws/tags"
	"github.com/JLCode-tech/awsbnkctl/internal/intent"
)

// Phase10NodeGroup creates managed node groups defined in cluster.yaml.
// One node group per NodeGroupSpec entry. Subnets used are public only.
//
// State keys written per node group: NODEGROUP_<UPPER>_NAME, NODEGROUP_<UPPER>_ARN.
//
// Idempotent: DescribeNodegroup before creating.
// Dry-run: writes placeholder state, no AWS mutations.
func Phase10NodeGroup(ctx context.Context, cl *intent.Cluster, st *state.State, clients *Clients, dryRun bool) error {
	awsmw.CheckAuthOrDie(clients.Profile)
	name := cl.Metadata.Name

	if cl.ClusterSpec == nil {
		return fmt.Errorf("phase10: cluster.yaml must include a 'cluster:' block (see slice-03 docs)")
	}

	if dryRun {
		for _, ng := range cl.ClusterSpec.NodeGroups {
			upper := strings.ToUpper(ng.Name)
			ngName := name + "-ng-" + ng.Name
			fmt.Fprintf(os.Stderr, "[phase 10] dry-run: would create node group %s\n", ngName)
			st.Set("NODEGROUP_"+upper+"_NAME", "dry-run-ng-"+ng.Name)
			st.Set("NODEGROUP_"+upper+"_ARN", "arn:aws:eks:dry-run:nodegroup/"+name+"/"+ngName+"/dry-run")
		}
		return nil
	}

	clusterName := st.Get("EKS_CLUSTER_NAME")
	if clusterName == "" {
		return fmt.Errorf("phase10: EKS_CLUSTER_NAME not in state (run phase08 first)")
	}
	nodeRoleARN := st.Get("EKS_NODE_ROLE_ARN")
	if nodeRoleARN == "" {
		return fmt.Errorf("phase10: EKS_NODE_ROLE_ARN not in state (run phase07 first)")
	}
	publicSubnets := splitCSV(st.Get("PUBLIC_SUBNETS"))
	if len(publicSubnets) == 0 {
		return fmt.Errorf("phase10: PUBLIC_SUBNETS not in state (run phase03 first)")
	}

	for _, ng := range cl.ClusterSpec.NodeGroups {
		if err := ensureNodeGroup(ctx, clients.EKS, name, clusterName, nodeRoleARN, publicSubnets, ng, cl.Tags, cl.Metadata.Labels, st); err != nil {
			return fmt.Errorf("phase10: node group %s: %w", ng.Name, err)
		}
	}
	return nil
}

// Phase10NodeGroupDown deletes all managed node groups for the cluster.
// Tolerates ResourceNotFoundException (already deleted).
func Phase10NodeGroupDown(ctx context.Context, cl *intent.Cluster, st *state.State, clients *Clients) error {
	awsmw.CheckAuthOrDie(clients.Profile)
	name := cl.Metadata.Name
	fmt.Fprintf(os.Stderr, "[phase 10 down] node group: cluster=%s\n", name)

	clusterName := st.Get("EKS_CLUSTER_NAME")
	if clusterName == "" {
		clusterName = name
	}

	if cl.ClusterSpec == nil {
		fmt.Fprintf(os.Stderr, "[phase 10 down] no cluster spec, skipping node group deletion\n")
		return nil
	}

	for _, ng := range cl.ClusterSpec.NodeGroups {
		upper := strings.ToUpper(ng.Name)
		ngName := st.Get("NODEGROUP_" + upper + "_NAME")
		if ngName == "" {
			ngName = clusterName + "-ng-" + ng.Name
		}

		if err := deleteNodeGroup(ctx, clients.EKS, clusterName, ngName); err != nil {
			return fmt.Errorf("phase10 down: node group %s: %w", ngName, err)
		}
		st.Set("NODEGROUP_"+upper+"_NAME", "")
		st.Set("NODEGROUP_"+upper+"_ARN", "")
	}
	return st.Save()
}

// --- helpers ---

func ensureNodeGroup(
	ctx context.Context,
	eksc EKSAPI,
	clusterDisplayName, clusterName, nodeRoleARN string,
	publicSubnets []string,
	ng intent.NodeGroupSpec,
	extraTags map[string]string,
	labels map[string]string,
	st *state.State,
) error {
	upper := strings.ToUpper(ng.Name)
	ngName := clusterName + "-ng-" + ng.Name

	fmt.Fprintf(os.Stderr, "[phase 10] node group: creating %s (%s × %d)\n", ngName, ng.InstanceType, ng.DesiredSize)

	// Idempotency check.
	descOut, err := eksc.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
		ClusterName:   ptr(clusterName),
		NodegroupName: ptr(ngName),
	})
	if err != nil && !isEKSNotFound(err) {
		return fmt.Errorf("DescribeNodegroup: %w", err)
	}

	if err == nil {
		existing := descOut.Nodegroup
		switch existing.Status {
		case ekstypes.NodegroupStatusActive:
			fmt.Fprintf(os.Stderr, "[phase 10] node group %s already exists, status=ACTIVE, skipping create\n", ngName)
			return populateNodeGroupState(st, existing, upper)
		case ekstypes.NodegroupStatusCreating, ekstypes.NodegroupStatusUpdating:
			fmt.Fprintf(os.Stderr, "[phase 10] node group %s already exists, status=%s, waiting for ACTIVE\n", ngName, existing.Status)
			return waitAndPopulateNodeGroup(ctx, eksc, clusterName, ngName, upper, st)
		case ekstypes.NodegroupStatusCreateFailed, ekstypes.NodegroupStatusDeleteFailed:
			return fmt.Errorf("node group %s in terminal failure status %s", ngName, existing.Status)
		default:
			return fmt.Errorf("node group %s in unexpected status %s", ngName, existing.Status)
		}
	}

	ngTags := tags.EKSTags(
		tags.Required(clusterDisplayName, tags.CompEKSNodeGroup),
		extraTags,
		labels,
	)

	// Kubernetes node labels. K8s label keys can't contain ':' so use the
	// awsbnkctl.io/ prefix (matching the namespace-label convention from
	// docs/POST_TERRAFORM_DIRECTION.md §3 / D-006). The `:` form used for
	// AWS resource tags would cause an EKS InvalidParameterException
	// (regex '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]').
	k8sLabels := map[string]string{
		"awsbnkctl.io/cluster": clusterDisplayName,
	}
	for k, v := range ng.Labels {
		k8sLabels[k] = v
	}

	// Bounds-check before int32 cast (DiskSize/DesiredSize/MinSize/MaxSize
	// come from validated cluster.yaml; defaults are 50/1/1/2). EKS API
	// requires int32. The #nosec annotations satisfy gosec G115 — the
	// bounds check above makes the cast genuinely safe.
	if ng.DiskSize > 1<<30 || ng.DesiredSize > 1<<30 || ng.MinSize > 1<<30 || ng.MaxSize > 1<<30 {
		return fmt.Errorf("nodegroup %s: scaling/disk value too large", ngName)
	}
	diskSize := int32(ng.DiskSize)       // #nosec G115 -- bounded above
	desiredSize := int32(ng.DesiredSize) // #nosec G115 -- bounded above
	minSize := int32(ng.MinSize)         // #nosec G115 -- bounded above
	maxSize := int32(ng.MaxSize)         // #nosec G115 -- bounded above
	_, err = eksc.CreateNodegroup(ctx, &eks.CreateNodegroupInput{
		ClusterName:   ptr(clusterName),
		NodegroupName: ptr(ngName),
		NodeRole:      ptr(nodeRoleARN),
		Subnets:       publicSubnets,
		AmiType:       ekstypes.AMITypesAl2X8664,
		InstanceTypes: []string{ng.InstanceType},
		ScalingConfig: &ekstypes.NodegroupScalingConfig{
			DesiredSize: int32Ptr(desiredSize),
			MinSize:     int32Ptr(minSize),
			MaxSize:     int32Ptr(maxSize),
		},
		DiskSize: &diskSize,
		Labels:   k8sLabels,
		Tags:     ngTags,
	})
	if err != nil {
		return fmt.Errorf("CreateNodegroup %s: %w", ngName, err)
	}
	fmt.Fprintf(os.Stderr, "[phase 10] node group %s: create request sent, waiting for ACTIVE (up to 20 min)\n", ngName)

	return waitAndPopulateNodeGroup(ctx, eksc, clusterName, ngName, upper, st)
}

func waitAndPopulateNodeGroup(ctx context.Context, eksc EKSAPI, clusterName, ngName, upper string, st *state.State) error {
	const (
		ngTimeout = 20 * time.Minute
		ngPoll    = 30 * time.Second
	)
	deadline := time.Now().Add(ngTimeout)
	for time.Now().Before(deadline) {
		out, err := eksc.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
			ClusterName:   ptr(clusterName),
			NodegroupName: ptr(ngName),
		})
		if err != nil {
			return fmt.Errorf("waitAndPopulateNodeGroup: DescribeNodegroup: %w", err)
		}
		ng := out.Nodegroup
		switch ng.Status {
		case ekstypes.NodegroupStatusActive:
			fmt.Fprintf(os.Stderr, "[phase 10] node group %s reached state ACTIVE\n", ngName)
			return populateNodeGroupState(st, ng, upper)
		case ekstypes.NodegroupStatusCreateFailed, ekstypes.NodegroupStatusDeleteFailed:
			return fmt.Errorf("node group %s entered failure status %s", ngName, ng.Status)
		case ekstypes.NodegroupStatusCreating, ekstypes.NodegroupStatusUpdating:
			// still in progress
		default:
			return fmt.Errorf("node group %s in unexpected state %s", ngName, ng.Status)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(ngPoll):
		}
	}
	return fmt.Errorf("phase10: timeout waiting for node group %s to become ACTIVE", ngName)
}

func populateNodeGroupState(st *state.State, ng *ekstypes.Nodegroup, upper string) error {
	if ng.NodegroupArn == nil {
		return fmt.Errorf("phase10: nodegroup describe response missing ARN")
	}
	st.Set("NODEGROUP_"+upper+"_NAME", *ng.NodegroupName)
	st.Set("NODEGROUP_"+upper+"_ARN", *ng.NodegroupArn)
	return st.Save()
}

func deleteNodeGroup(ctx context.Context, eksc EKSAPI, clusterName, ngName string) error {
	fmt.Fprintf(os.Stderr, "[phase 10 down] deleting node group %s\n", ngName)
	_, err := eksc.DeleteNodegroup(ctx, &eks.DeleteNodegroupInput{
		ClusterName:   ptr(clusterName),
		NodegroupName: ptr(ngName),
	})
	if err != nil {
		if isEKSNotFound(err) {
			fmt.Fprintf(os.Stderr, "[phase 10 down] node group %s already gone\n", ngName)
			return nil
		}
		return fmt.Errorf("DeleteNodegroup %s: %w", ngName, err)
	}
	return waitNodeGroupDeleted(ctx, eksc, clusterName, ngName)
}

func waitNodeGroupDeleted(ctx context.Context, eksc EKSAPI, clusterName, ngName string) error {
	const (
		deleteTimeout = 15 * time.Minute
		deletePoll    = 30 * time.Second
	)
	fmt.Fprintf(os.Stderr, "[phase 10 down] waiting for node group %s to be deleted\n", ngName)
	deadline := time.Now().Add(deleteTimeout)
	for time.Now().Before(deadline) {
		_, err := eksc.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
			ClusterName:   ptr(clusterName),
			NodegroupName: ptr(ngName),
		})
		if err != nil {
			if isEKSNotFound(err) {
				fmt.Fprintf(os.Stderr, "[phase 10 down] node group %s deleted\n", ngName)
				return nil
			}
			return fmt.Errorf("waitNodeGroupDeleted: DescribeNodegroup: %w", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(deletePoll):
		}
	}
	return fmt.Errorf("phase10 down: timeout waiting for node group %s to be deleted", ngName)
}

// int32Ptr returns a pointer to an int32.
func int32Ptr(v int32) *int32 { return &v }
