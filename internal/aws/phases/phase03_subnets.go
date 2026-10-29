package phases

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/state"
	"github.com/JLCode-tech/awsbnkctl/internal/aws/tags"
	"github.com/JLCode-tech/awsbnkctl/internal/intent"
)

func resolveClusterAZs(ctx context.Context, cl *intent.Cluster, ec2c EC2API) {
	if cl == nil || ec2c == nil {
		return
	}
	stateName := "state"
	out, err := ec2c.DescribeAvailabilityZones(ctx, &ec2.DescribeAvailabilityZonesInput{
		Filters: []ec2types.Filter{
			{Name: &stateName, Values: []string{"available"}},
		},
	})
	if err != nil || out == nil || len(out.AvailabilityZones) == 0 {
		return
	}
	var avail []string
	availSet := make(map[string]bool, len(out.AvailabilityZones))
	for _, z := range out.AvailabilityZones {
		if z.ZoneName == nil || *z.ZoneName == "" {
			continue
		}
		if z.ZoneType != nil && *z.ZoneType != "" && *z.ZoneType != "availability-zone" {
			continue
		}
		name := *z.ZoneName
		avail = append(avail, name)
		availSet[name] = true
	}
	if len(avail) == 0 {
		return
	}
	sort.Strings(avail)

	needRemap := len(cl.Network.AZs) == 0
	for _, az := range cl.Network.AZs {
		if az == "" || az == "auto" || !availSet[az] {
			needRemap = true
			break
		}
	}
	if needRemap {
		count := len(cl.Network.AZs)
		if count < 2 {
			count = 2
		}
		if count > len(avail) {
			count = len(avail)
		}
		cl.Network.AZs = append([]string(nil), avail[:count]...)
	}
	if len(cl.Network.AZs) == 0 {
		return
	}
	for i := range cl.Network.Subnets.Public {
		az := cl.Network.Subnets.Public[i].AZ
		if az == "" || az == "auto" || !availSet[az] {
			cl.Network.Subnets.Public[i].AZ = cl.Network.AZs[i%len(cl.Network.AZs)]
		}
	}
	for i := range cl.Network.Subnets.Private {
		az := cl.Network.Subnets.Private[i].AZ
		if az == "" || az == "auto" || !availSet[az] {
			cl.Network.Subnets.Private[i].AZ = cl.Network.AZs[i%len(cl.Network.AZs)]
		}
	}
	if cl.Network.DataPath != nil {
		if cl.Network.DataPath.External.CIDR != "" {
			az := cl.Network.DataPath.External.AZ
			if az == "" || az == "auto" || !availSet[az] {
				cl.Network.DataPath.External.AZ = cl.Network.AZs[0]
			}
		}
		if cl.Network.DataPath.Internal.CIDR != "" {
			az := cl.Network.DataPath.Internal.AZ
			if az == "" || az == "auto" || !availSet[az] {
				cl.Network.DataPath.Internal.AZ = cl.Network.AZs[0]
			}
		}
	}
	fmt.Fprintf(os.Stderr, "[phase 03] dynamically discovered available AZs: %v (active: %v)\n", avail, cl.Network.AZs)
}

// Phase03Subnets creates the public and private subnets defined in
// cluster.yaml. Each subnet is tagged individually. IDs are written to
// state.env as comma-separated lists (PUBLIC_SUBNETS, PRIVATE_SUBNETS).
//
// When pattern is host-device, also creates BNK_EXT_SUBNET and BNK_INT_SUBNET
// from network.dataPath.
func Phase03Subnets(ctx context.Context, cl *intent.Cluster, st *state.State, clients *Clients, dryRun bool) error {
	checkAuthOrDie(clients)
	name := cl.Metadata.Name
	vpcID := st.Get("VPC_ID")
	if vpcID == "" {
		return fmt.Errorf("phase03: VPC_ID not in state (run phase02 first)")
	}

	fmt.Fprintf(os.Stderr, "[phase 03] subnets: cluster=%s vpc=%s\n", name, vpcID)

	if dryRun {
		// Credential-free dry-run: do not call EC2 DescribeSubnets/CreateSubnet.
		// Populate placeholder IDs so downstream phases can render their plan.
		publicIDs := makeDryRunSubnetIDs("pub", len(cl.Network.Subnets.Public))
		privateIDs := makeDryRunSubnetIDs("priv", len(cl.Network.Subnets.Private))
		st.Set("PUBLIC_SUBNETS", strings.Join(publicIDs, ","))
		st.Set("PRIVATE_SUBNETS", strings.Join(privateIDs, ","))
		// data-path placeholders (BNK patterns). External always; internal only
		// for dual-interface.
		if cl.IsBNKPattern() && cl.Network.DataPath != nil {
			fmt.Fprintf(os.Stderr, "[phase 03] dry-run: would create BNK_EXT_SUBNET %s in %s\n",
				cl.Network.DataPath.External.CIDR, cl.Network.DataPath.External.AZ)
			st.Set("BNK_EXT_SUBNET", "dry-run-subnet-bnk-ext")
			if cl.HasInternalInterface() {
				fmt.Fprintf(os.Stderr, "[phase 03] dry-run: would create BNK_INT_SUBNET %s in %s\n",
					cl.Network.DataPath.Internal.CIDR, cl.Network.DataPath.Internal.AZ)
				st.Set("BNK_INT_SUBNET", "dry-run-subnet-bnk-int")
			}
		}
		return nil
	}

	resolveClusterAZs(ctx, cl, clients.EC2)

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

	st.Set("PUBLIC_SUBNETS", strings.Join(publicIDs, ","))
	st.Set("PRIVATE_SUBNETS", strings.Join(privateIDs, ","))

	// BNK patterns: create data-path subnets. External always; internal only for
	// dual-interface (single-interface patterns reach backends over CNI).
	if cl.IsBNKPattern() && cl.Network.DataPath != nil {
		extID, err := ensureDataPathSubnet(ctx, clients.EC2, name, vpcID,
			cl.Network.DataPath.External, tags.CompSubnetBNKExt, cl.Tags, cl.Metadata.Labels)
		if err != nil {
			return fmt.Errorf("phase03: BNK ext subnet: %w", err)
		}
		st.Set("BNK_EXT_SUBNET", extID)

		if cl.HasInternalInterface() {
			intID, err := ensureDataPathSubnet(ctx, clients.EC2, name, vpcID,
				cl.Network.DataPath.Internal, tags.CompSubnetBNKInt, cl.Tags, cl.Metadata.Labels)
			if err != nil {
				return fmt.Errorf("phase03: BNK int subnet: %w", err)
			}
			st.Set("BNK_INT_SUBNET", intID)
		}
	}

	return st.Save()
}

// Phase03SubnetsDown deletes all subnets tagged for this cluster (reverse
// of Phase03Subnets). Tolerates "already gone". When pattern is host-device,
// also deletes BNK_EXT_SUBNET and BNK_INT_SUBNET.
func Phase03SubnetsDown(ctx context.Context, cl *intent.Cluster, st *state.State, clients *Clients) error {
	checkAuthOrDie(clients)
	name := cl.Metadata.Name
	fmt.Fprintf(os.Stderr, "[phase 03 down] subnets: cluster=%s\n", name)

	vpcID := st.Get("VPC_ID")
	swept := false
	deleteOneSubnet := func(label, sid string) error {
		fmt.Fprintf(os.Stderr, "[phase 03 down] deleting %s (%s)\n", label, sid)
		_, err := clients.EC2.DeleteSubnet(ctx, &ec2.DeleteSubnetInput{SubnetId: ptr(sid)})
		if err = ignoreNotFound(err); err != nil && isDependencyViolation(err) && !swept {
			swept = true
			if vpcID != "" {
				sweepVPCInstancesAndENIs(ctx, clients.EC2, vpcID)
			}
			_, err = clients.EC2.DeleteSubnet(ctx, &ec2.DeleteSubnetInput{SubnetId: ptr(sid)})
			err = ignoreNotFound(err)
		}
		return err
	}

	// Delete data-path subnets first (reverse order).
	for _, key := range []string{"BNK_INT_SUBNET", "BNK_EXT_SUBNET"} {
		sid := st.Get(key)
		if sid == "" {
			continue
		}
		if err := deleteOneSubnet("data-path subnet "+key, sid); err != nil {
			return fmt.Errorf("phase03 down: ec2:DeleteSubnet %s (%s): %w", key, sid, err)
		}
		st.Set(key, "")
	}

	// Collect IDs from state + tag-discovery fallback.
	subnetIDs := collectSubnetIDs(ctx, clients.EC2, name, st)

	for _, sid := range subnetIDs {
		if err := deleteOneSubnet("subnet", sid); err != nil {
			return fmt.Errorf("phase03 down: ec2:DeleteSubnet %s: %w", sid, err)
		}
	}

	st.Set("PUBLIC_SUBNETS", "")
	st.Set("PRIVATE_SUBNETS", "")
	return st.Save()
}

// ensureDataPathSubnet creates a single BNK data-path subnet if not already
// present (idempotent: lookup-by-tag first). Returns the subnet ID.
func ensureDataPathSubnet(ctx context.Context, ec2c EC2API, clusterName, vpcID string,
	spec intent.SubnetSpec, component string,
	extraTags, labels map[string]string) (string, error) {

	existing, err := findSubnetByCIDR(ctx, ec2c, clusterName, vpcID, spec.CIDR)
	if err != nil {
		return "", fmt.Errorf("listing data-path subnets: %w", err)
	}
	if existing != "" {
		fmt.Fprintf(os.Stderr, "[phase 03] data-path subnet %s (%s) already exists, skipping\n",
			existing, spec.CIDR)
		return existing, nil
	}

	resourceTags := tags.Merge(
		tags.Required(clusterName, component),
		map[string]string{tags.KeyName: clusterName + "-" + component},
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
		return "", fmt.Errorf("ec2:CreateSubnet %s (%s): %w", component, spec.CIDR, err)
	}
	sid := *out.Subnet.SubnetId
	fmt.Fprintf(os.Stderr, "[phase 03] created data-path subnet %s (%s) in %s\n", sid, spec.CIDR, spec.AZ)
	return sid, nil
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

// makeDryRunSubnetIDs returns n placeholder subnet IDs for a given visibility
// label (e.g. "pub", "priv"). Used in dry-run mode to populate state so
// downstream phases can render their plan.
func makeDryRunSubnetIDs(label string, n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("dry-run-subnet-%s-%d", label, i+1)
	}
	return ids
}

// collectSubnetIDs returns all subnet IDs for the cluster by merging state keys
// with tag-discovery and VPC-discovery so partial state never leaves orphaned subnets.
func collectSubnetIDs(ctx context.Context, ec2c EC2API, clusterName string, st *state.State) []string {
	seen := make(map[string]bool)
	var ids []string
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}

	for _, csv := range []string{st.Get("PUBLIC_SUBNETS"), st.Get("PRIVATE_SUBNETS")} {
		if csv == "" {
			continue
		}
		for _, id := range strings.Split(csv, ",") {
			add(id)
		}
	}

	// Tag-discovery (always merged in case state.env was only partially populated).
	if out, err := ec2c.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		Filters: []ec2types.Filter{tags.ClusterFilter(clusterName)},
	}); err == nil && out != nil {
		for _, s := range out.Subnets {
			if s.SubnetId != nil {
				add(*s.SubnetId)
			}
		}
	}

	// VPC-discovery fallback for any untagged/auxiliary subnets inside the cluster VPC.
	if vpcID := st.Get("VPC_ID"); vpcID != "" {
		if out, err := ec2c.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
			Filters: []ec2types.Filter{{Name: ptr("vpc-id"), Values: []string{vpcID}}},
		}); err == nil && out != nil {
			for _, s := range out.Subnets {
				if s.SubnetId != nil {
					add(*s.SubnetId)
				}
			}
		}
	}
	return ids
}
