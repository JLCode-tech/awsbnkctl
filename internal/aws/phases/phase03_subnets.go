package phases

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/awsmw"
	"github.com/JLCode-tech/awsbnkctl/internal/aws/state"
	"github.com/JLCode-tech/awsbnkctl/internal/aws/tags"
	"github.com/JLCode-tech/awsbnkctl/internal/intent"
)

// Phase03Subnets creates the public and private subnets defined in
// cluster.yaml. Each subnet is tagged individually. IDs are written to
// state.env as comma-separated lists (PUBLIC_SUBNETS, PRIVATE_SUBNETS).
func Phase03Subnets(ctx context.Context, cl *intent.Cluster, st *state.State, clients *Clients, dryRun bool) error {
	awsmw.CheckAuthOrDie(clients.Profile)
	name := cl.Metadata.Name
	vpcID := st.Get("VPC_ID")
	if vpcID == "" {
		return fmt.Errorf("phase03: VPC_ID not in state (run phase02 first)")
	}

	fmt.Fprintf(os.Stderr, "[phase 03] subnets: cluster=%s vpc=%s\n", name, vpcID)

	publicIDs, err := provisionSubnets(ctx, clients.EC2, name, vpcID,
		cl.Network.Subnets.Public, tags.CompSubnetPublic, true, cl.Tags, cl.Metadata.Labels, dryRun)
	if err != nil {
		return fmt.Errorf("phase03: public subnets: %w", err)
	}

	privateIDs, err := provisionSubnets(ctx, clients.EC2, name, vpcID,
		cl.Network.Subnets.Private, tags.CompSubnetPrivate, false, cl.Tags, cl.Metadata.Labels, dryRun)
	if err != nil {
		return fmt.Errorf("phase03: private subnets: %w", err)
	}

	if !dryRun {
		st.Set("PUBLIC_SUBNETS", strings.Join(publicIDs, ","))
		st.Set("PRIVATE_SUBNETS", strings.Join(privateIDs, ","))
		return st.Save()
	}
	return nil
}

// Phase03SubnetsDown deletes all subnets tagged for this cluster (reverse
// of Phase03Subnets). Tolerates "already gone".
func Phase03SubnetsDown(ctx context.Context, cl *intent.Cluster, st *state.State, clients *Clients) error {
	awsmw.CheckAuthOrDie(clients.Profile)
	name := cl.Metadata.Name
	fmt.Fprintf(os.Stderr, "[phase 03 down] subnets: cluster=%s\n", name)

	// Collect IDs from state + tag-discovery fallback.
	subnetIDs := collectSubnetIDs(ctx, clients.EC2, name, st)

	for _, sid := range subnetIDs {
		fmt.Fprintf(os.Stderr, "[phase 03 down] deleting subnet %s\n", sid)
		_, err := clients.EC2.DeleteSubnet(ctx, &ec2.DeleteSubnetInput{SubnetId: ptr(sid)})
		if err := ignoreNotFound(err); err != nil {
			return fmt.Errorf("phase03 down: ec2:DeleteSubnet %s: %w", sid, err)
		}
	}

	st.Set("PUBLIC_SUBNETS", "")
	st.Set("PRIVATE_SUBNETS", "")
	return st.Save()
}

// provisionSubnets ensures each SubnetSpec from the intent exists. Returns
// the slice of subnet IDs (in spec order).
func provisionSubnets(ctx context.Context, ec2c EC2API, clusterName, vpcID string,
	specs []intent.SubnetSpec, component string, mapPublicIP bool,
	extraTags, labels map[string]string, dryRun bool) ([]string, error) {

	ids := make([]string, 0, len(specs))
	for i, spec := range specs {
		// Idempotency: check if a subnet with this CIDR + cluster tag already exists.
		existing, err := findSubnetByCIDR(ctx, ec2c, clusterName, vpcID, spec.CIDR)
		if err != nil {
			return nil, fmt.Errorf("listing subnets: %w", err)
		}
		if existing != "" {
			fmt.Fprintf(os.Stderr, "[phase 03] subnet %s (%s) already exists, skipping\n",
				existing, spec.CIDR)
			ids = append(ids, existing)
			continue
		}
		if dryRun {
			fmt.Fprintf(os.Stderr, "[phase 03] dry-run: would create %s subnet %s in %s\n",
				component, spec.CIDR, spec.AZ)
			continue
		}

		// Use a per-subnet name suffix: <component>-<index+1> for uniqueness.
		compName := fmt.Sprintf("%s-%d", component, i+1)
		resourceTags := tags.Merge(
			tags.Required(clusterName, component),
			map[string]string{tags.KeyName: clusterName + "-" + compName},
			extraTags,
			labels,
		)

		out, err := ec2c.CreateSubnet(ctx, &ec2.CreateSubnetInput{
			VpcId:            ptr(vpcID),
			CidrBlock:        ptr(spec.CIDR),
			AvailabilityZone: ptr(spec.AZ),
			TagSpecifications: []ec2types.TagSpecification{
				tagSpecification(ec2types.ResourceTypeSubnet, resourceTags),
			},
		})
		if err != nil {
			return nil, fmt.Errorf("ec2:CreateSubnet %s: %w", spec.CIDR, err)
		}
		sid := *out.Subnet.SubnetId

		// Public subnets get auto-assign public IP so pods have internet egress
		// directly (without NAT) where needed.
		if mapPublicIP {
			if _, err := ec2c.ModifySubnetAttribute(ctx, &ec2.ModifySubnetAttributeInput{
				SubnetId:            ptr(sid),
				MapPublicIpOnLaunch: &ec2types.AttributeBooleanValue{Value: boolPtr(true)},
			}); err != nil {
				return nil, fmt.Errorf("ec2:ModifySubnetAttribute %s: %w", sid, err)
			}
		}

		fmt.Fprintf(os.Stderr, "[phase 03] created %s subnet %s (%s)\n", component, sid, spec.CIDR)
		ids = append(ids, sid)
	}
	return ids, nil
}

// findSubnetByCIDR returns the subnet ID that matches the cluster tag +
// CIDR, or "" if none.
func findSubnetByCIDR(ctx context.Context, ec2c EC2API, clusterName, vpcID, cidr string) (string, error) {
	cidrName := "cidrBlock"
	out, err := ec2c.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		Filters: []ec2types.Filter{
			tags.ClusterFilter(clusterName),
			{Name: ptr("vpc-id"), Values: []string{vpcID}},
			{Name: &cidrName, Values: []string{cidr}},
		},
	})
	if err != nil {
		return "", err
	}
	if len(out.Subnets) == 0 {
		return "", nil
	}
	return *out.Subnets[0].SubnetId, nil
}

// collectSubnetIDs returns all subnet IDs for the cluster from state first,
// falling back to tag-discovery.
func collectSubnetIDs(ctx context.Context, ec2c EC2API, clusterName string, st *state.State) []string {
	var ids []string
	for _, csv := range []string{st.Get("PUBLIC_SUBNETS"), st.Get("PRIVATE_SUBNETS")} {
		if csv == "" {
			continue
		}
		for _, id := range strings.Split(csv, ",") {
			id = strings.TrimSpace(id)
			if id != "" {
				ids = append(ids, id)
			}
		}
	}
	if len(ids) > 0 {
		return ids
	}

	// Tag-discovery fallback.
	out, err := ec2c.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		Filters: []ec2types.Filter{tags.ClusterFilter(clusterName)},
	})
	if err != nil {
		return nil
	}
	for _, s := range out.Subnets {
		if s.SubnetId != nil {
			ids = append(ids, *s.SubnetId)
		}
	}
	return ids
}
