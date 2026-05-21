package phases

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/awsmw"
	"github.com/JLCode-tech/awsbnkctl/internal/aws/state"
	"github.com/JLCode-tech/awsbnkctl/internal/aws/tags"
	"github.com/JLCode-tech/awsbnkctl/internal/intent"
)

// Phase02VPC creates the VPC if it does not already exist (idempotent
// list-by-tag). Saves VPC_ID to state.env on success.
//
// Phase 01 is reserved for IAM (slice 2).
func Phase02VPC(ctx context.Context, cl *intent.Cluster, st *state.State, clients *Clients, dryRun bool) error {
	awsmw.CheckAuthOrDie(clients.Profile)
	name := cl.Metadata.Name
	fmt.Fprintf(os.Stderr, "[phase 02] vpc: cluster=%s cidr=%s\n", name, cl.Network.VPCCidr)

	// List-by-tag idempotency check.
	existing, err := findVPCByTag(ctx, clients.EC2, name)
	if err != nil {
		return fmt.Errorf("phase02: listing VPCs by tag: %w", err)
	}
	if existing != "" {
		fmt.Fprintf(os.Stderr, "[phase 02] vpc %s already exists, skipping\n", existing)
		st.Set("VPC_ID", existing)
		return st.Save()
	}

	if dryRun {
		fmt.Fprintf(os.Stderr, "[phase 02] dry-run: would create VPC cidr=%s\n", cl.Network.VPCCidr)
		return nil
	}

	resourceTags := tags.Merge(
		tags.Required(name, tags.CompVPC),
		cl.Tags,
		cl.Metadata.Labels,
	)

	out, err := clients.EC2.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: ptr(cl.Network.VPCCidr),
		TagSpecifications: []ec2types.TagSpecification{
			tagSpecification(ec2types.ResourceTypeVpc, resourceTags),
		},
	})
	if err != nil {
		return fmt.Errorf("phase02: ec2:CreateVpc: %w", err)
	}
	vpcID := *out.Vpc.VpcId

	// Enable DNS support and hostnames — required for EKS (slice 3).
	if _, err := clients.EC2.ModifyVpcAttribute(ctx, &ec2.ModifyVpcAttributeInput{
		VpcId:            ptr(vpcID),
		EnableDnsSupport: &ec2types.AttributeBooleanValue{Value: boolPtr(true)},
	}); err != nil {
		return fmt.Errorf("phase02: enable DNS support: %w", err)
	}
	if _, err := clients.EC2.ModifyVpcAttribute(ctx, &ec2.ModifyVpcAttributeInput{
		VpcId:              ptr(vpcID),
		EnableDnsHostnames: &ec2types.AttributeBooleanValue{Value: boolPtr(true)},
	}); err != nil {
		return fmt.Errorf("phase02: enable DNS hostnames: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[phase 02] created VPC %s\n", vpcID)
	st.Set("VPC_ID", vpcID)
	return st.Save()
}

// Phase02VPCDown destroys the VPC. Tolerates "already gone".
func Phase02VPCDown(ctx context.Context, cl *intent.Cluster, st *state.State, clients *Clients) error {
	awsmw.CheckAuthOrDie(clients.Profile)
	name := cl.Metadata.Name

	vpcID := st.Get("VPC_ID")
	if vpcID == "" {
		// Tag-discovery fallback.
		var err error
		vpcID, err = findVPCByTag(ctx, clients.EC2, name)
		if err != nil {
			return fmt.Errorf("phase02 down: tag-discovery: %w", err)
		}
	}
	if vpcID == "" {
		fmt.Fprintf(os.Stderr, "[phase 02 down] VPC already gone\n")
		return nil
	}

	fmt.Fprintf(os.Stderr, "[phase 02 down] deleting VPC %s\n", vpcID)
	_, err := clients.EC2.DeleteVpc(ctx, &ec2.DeleteVpcInput{VpcId: ptr(vpcID)})
	if err := ignoreNotFound(err); err != nil {
		return fmt.Errorf("phase02 down: ec2:DeleteVpc: %w", err)
	}

	st.Set("VPC_ID", "")
	return st.Save()
}

// findVPCByTag returns the VPC ID tagged awsbnkctl:cluster=name, or "" if
// none exists.
func findVPCByTag(ctx context.Context, ec2c EC2API, clusterName string) (string, error) {
	out, err := ec2c.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
		Filters: []ec2types.Filter{
			tags.ClusterFilter(clusterName),
		},
	})
	if err != nil {
		return "", err
	}
	if len(out.Vpcs) == 0 {
		return "", nil
	}
	return *out.Vpcs[0].VpcId, nil
}
