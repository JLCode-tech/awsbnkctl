package phases

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/state"
	"github.com/JLCode-tech/awsbnkctl/internal/aws/tags"
	"github.com/JLCode-tech/awsbnkctl/internal/intent"
)

// Phase02VPC creates the VPC if it does not already exist (idempotent
// list-by-tag). Saves VPC_ID to state.env on success.
//
// Phase 01 is reserved for IAM (slice 2).
func Phase02VPC(ctx context.Context, cl *intent.Cluster, st *state.State, clients *Clients, dryRun bool) error {
	checkAuthOrDie(clients)
	name := cl.Metadata.Name
	fmt.Fprintf(os.Stderr, "[phase 02] vpc: cluster=%s cidr=%s\n", name, cl.Network.VPCCidr)

	if dryRun {
		fmt.Fprintf(os.Stderr, "[phase 02] dry-run: would create VPC cidr=%s\n", cl.Network.VPCCidr)
		st.Set("VPC_ID", "dry-run-vpc")
		return nil
	}

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

// Phase02VPCDown destroys the VPC (and any additional orphaned VPCs tagged for
// this cluster). Tolerates "already gone" and sweeps residual EICEs, available
// ENIs, non-default security groups, subnets, non-main route tables, and
// attached IGWs before calling DeleteVpc.
func Phase02VPCDown(ctx context.Context, cl *intent.Cluster, st *state.State, clients *Clients) error {
	checkAuthOrDie(clients)
	name := cl.Metadata.Name

	seen := make(map[string]struct{})
	var vpcIDs []string
	addVPC := func(id string) {
		if id == "" || id == "dry-run-vpc" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		vpcIDs = append(vpcIDs, id)
	}

	addVPC(st.Get("VPC_ID"))
	taggedVPCs, err := findVPCsByTag(ctx, clients.EC2, name)
	if err != nil {
		if len(vpcIDs) == 0 {
			return fmt.Errorf("phase02 down: tag-discovery: %w", err)
		}
	}
	for _, id := range taggedVPCs {
		addVPC(id)
	}

	if len(vpcIDs) == 0 {
		fmt.Fprintf(os.Stderr, "[phase 02 down] VPC already gone\n")
		return nil
	}

	for _, vpcID := range vpcIDs {
		sweepVPCResiduals(ctx, clients.EC2, vpcID)
		fmt.Fprintf(os.Stderr, "[phase 02 down] deleting VPC %s\n", vpcID)
		_, err := clients.EC2.DeleteVpc(ctx, &ec2.DeleteVpcInput{VpcId: ptr(vpcID)})
		if err := ignoreNotFound(err); err != nil {
			return fmt.Errorf("phase02 down: ec2:DeleteVpc %s: %w", vpcID, err)
		}
	}

	st.Set("VPC_ID", "")
	return st.Save()
}

// sweepVPCResiduals performs a best-effort cleanup of residual resources inside
// vpcID that commonly block DeleteVpc with DependencyViolation when state.env
// was partial or external scripts created helper resources in the VPC.
func sweepVPCResiduals(ctx context.Context, ec2c EC2API, vpcID string) {
	vpcFilter := []ec2types.Filter{{Name: ptr("vpc-id"), Values: []string{vpcID}}}

	// 1. Delete any lingering EC2 Instance Connect Endpoints in the VPC.
	if eiceOut, err := ec2c.DescribeInstanceConnectEndpoints(ctx, &ec2.DescribeInstanceConnectEndpointsInput{
		Filters: vpcFilter,
	}); err == nil {
		for _, ep := range eiceOut.InstanceConnectEndpoints {
			if ep.InstanceConnectEndpointId == nil || ep.State == ec2types.Ec2InstanceConnectEndpointStateDeleteComplete {
				continue
			}
			eiceID := *ep.InstanceConnectEndpointId
			fmt.Fprintf(os.Stderr, "[phase 02 down] sweeping residual EICE %s in %s\n", eiceID, vpcID)
			if _, err := ec2c.DeleteInstanceConnectEndpoint(ctx, &ec2.DeleteInstanceConnectEndpointInput{
				InstanceConnectEndpointId: ptr(eiceID),
			}); err == nil {
				_ = waitEICEDeleted(ctx, ec2c, eiceID)
			}
		}
	}

	// 2. Delete any unattached ("available") ENIs in the VPC.
	if eniOut, err := ec2c.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{
		Filters: vpcFilter,
	}); err == nil {
		for _, eni := range eniOut.NetworkInterfaces {
			if eni.NetworkInterfaceId == nil || eni.Status != ec2types.NetworkInterfaceStatusAvailable {
				continue
			}
			eniID := *eni.NetworkInterfaceId
			fmt.Fprintf(os.Stderr, "[phase 02 down] sweeping residual available ENI %s in %s\n", eniID, vpcID)
			_, _ = ec2c.DeleteNetworkInterface(ctx, &ec2.DeleteNetworkInterfaceInput{
				NetworkInterfaceId: ptr(eniID),
			})
		}
	}

	// 3. Revoke ingress and egress rules on non-default security groups, then delete them.
	if sgOut, err := ec2c.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		Filters: vpcFilter,
	}); err == nil {
		var nonDefaultSGs []ec2types.SecurityGroup
		for _, sg := range sgOut.SecurityGroups {
			if sg.GroupId == nil || (sg.GroupName != nil && *sg.GroupName == "default") {
				continue
			}
			nonDefaultSGs = append(nonDefaultSGs, sg)
		}
		for _, sg := range nonDefaultSGs {
			if len(sg.IpPermissions) > 0 {
				_, _ = ec2c.RevokeSecurityGroupIngress(ctx, &ec2.RevokeSecurityGroupIngressInput{
					GroupId:       sg.GroupId,
					IpPermissions: sg.IpPermissions,
				})
			}
			if len(sg.IpPermissionsEgress) > 0 {
				_, _ = ec2c.RevokeSecurityGroupEgress(ctx, &ec2.RevokeSecurityGroupEgressInput{
					GroupId:       sg.GroupId,
					IpPermissions: sg.IpPermissionsEgress,
				})
			}
		}
		for _, sg := range nonDefaultSGs {
			fmt.Fprintf(os.Stderr, "[phase 02 down] sweeping residual security group %s in %s\n", *sg.GroupId, vpcID)
			_, _ = ec2c.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{
				GroupId: sg.GroupId,
			})
		}
	}

	// 4. Delete any remaining subnets in the VPC.
	if subOut, err := ec2c.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		Filters: vpcFilter,
	}); err == nil {
		for _, sub := range subOut.Subnets {
			if sub.SubnetId == nil {
				continue
			}
			fmt.Fprintf(os.Stderr, "[phase 02 down] sweeping residual subnet %s in %s\n", *sub.SubnetId, vpcID)
			_, _ = ec2c.DeleteSubnet(ctx, &ec2.DeleteSubnetInput{
				SubnetId: sub.SubnetId,
			})
		}
	}

	// 5. Disassociate and delete any remaining non-main route tables in the VPC.
	if rtbOut, err := ec2c.DescribeRouteTables(ctx, &ec2.DescribeRouteTablesInput{
		Filters: vpcFilter,
	}); err == nil {
		for _, rtb := range rtbOut.RouteTables {
			if rtb.RouteTableId == nil {
				continue
			}
			isMain := false
			for _, assoc := range rtb.Associations {
				if assoc.Main != nil && *assoc.Main {
					isMain = true
					continue
				}
				if assoc.RouteTableAssociationId != nil {
					_, _ = ec2c.DisassociateRouteTable(ctx, &ec2.DisassociateRouteTableInput{
						AssociationId: assoc.RouteTableAssociationId,
					})
				}
			}
			if !isMain {
				fmt.Fprintf(os.Stderr, "[phase 02 down] sweeping residual route table %s in %s\n", *rtb.RouteTableId, vpcID)
				_, _ = ec2c.DeleteRouteTable(ctx, &ec2.DeleteRouteTableInput{
					RouteTableId: rtb.RouteTableId,
				})
			}
		}
	}

	// 6. Detach and delete any remaining Internet Gateways attached to the VPC.
	if igwOut, err := ec2c.DescribeInternetGateways(ctx, &ec2.DescribeInternetGatewaysInput{
		Filters: []ec2types.Filter{{Name: ptr("attachment.vpc-id"), Values: []string{vpcID}}},
	}); err == nil {
		for _, igw := range igwOut.InternetGateways {
			if igw.InternetGatewayId == nil {
				continue
			}
			igwID := *igw.InternetGatewayId
			fmt.Fprintf(os.Stderr, "[phase 02 down] sweeping residual IGW %s attached to %s\n", igwID, vpcID)
			_, _ = ec2c.DetachInternetGateway(ctx, &ec2.DetachInternetGatewayInput{
				InternetGatewayId: ptr(igwID),
				VpcId:             ptr(vpcID),
			})
			_, _ = ec2c.DeleteInternetGateway(ctx, &ec2.DeleteInternetGatewayInput{
				InternetGatewayId: ptr(igwID),
			})
		}
	}
}

// findVPCByTag returns the first VPC ID tagged awsbnkctl:cluster=name, or "" if
// none exists.
func findVPCByTag(ctx context.Context, ec2c EC2API, clusterName string) (string, error) {
	ids, err := findVPCsByTag(ctx, ec2c, clusterName)
	if err != nil || len(ids) == 0 {
		return "", err
	}
	return ids[0], nil
}

// findVPCsByTag returns all VPC IDs tagged awsbnkctl:cluster=name.
func findVPCsByTag(ctx context.Context, ec2c EC2API, clusterName string) ([]string, error) {
	out, err := ec2c.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
		Filters: []ec2types.Filter{
			tags.ClusterFilter(clusterName),
		},
	})
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, v := range out.Vpcs {
		if v.VpcId != nil && *v.VpcId != "" {
			ids = append(ids, *v.VpcId)
		}
	}
	return ids, nil
}
