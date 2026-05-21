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

// Phase06RouteTables creates the public and private route tables, adds the
// default routes (0.0.0.0/0 via IGW for public; via NAT GW for private),
// and associates each subnet with its table.
//
// Idempotent: creates each route table only if absent (tag-based check),
// and skips route/association creation if already present.
func Phase06RouteTables(ctx context.Context, cl *intent.Cluster, st *state.State, clients *Clients, dryRun bool) error {
	awsmw.CheckAuthOrDie(clients.Profile)
	name := cl.Metadata.Name
	vpcID := st.Get("VPC_ID")
	igwID := st.Get("IGW_ID")
	natID := st.Get("NAT_GW_ID")

	if vpcID == "" {
		return fmt.Errorf("phase06: VPC_ID not in state")
	}
	if igwID == "" {
		return fmt.Errorf("phase06: IGW_ID not in state (run phase04 first)")
	}

	fmt.Fprintf(os.Stderr, "[phase 06] route tables: cluster=%s vpc=%s\n", name, vpcID)

	// --- Public route table ---
	pubRTBID, err := findRTBByTagAndVPC(ctx, clients.EC2, name, vpcID, "public")
	if err != nil {
		return fmt.Errorf("phase06: listing public RTBs: %w", err)
	}
	if pubRTBID == "" {
		if dryRun {
			fmt.Fprintf(os.Stderr, "[phase 06] dry-run: would create public route table → IGW\n")
		} else {
			pubRTBID, err = createRTB(ctx, clients.EC2, name, vpcID, "public", cl.Tags, cl.Metadata.Labels)
			if err != nil {
				return fmt.Errorf("phase06: create public RTB: %w", err)
			}
			fmt.Fprintf(os.Stderr, "[phase 06] created public RTB %s\n", pubRTBID)
		}
	} else {
		fmt.Fprintf(os.Stderr, "[phase 06] public RTB %s already exists, skipping\n", pubRTBID)
	}

	if !dryRun && pubRTBID != "" {
		// Default route → IGW.
		if err := ensureRoute(ctx, clients.EC2, pubRTBID, "0.0.0.0/0", igwID, ""); err != nil {
			return fmt.Errorf("phase06: public route → IGW: %w", err)
		}
		// Associate public subnets.
		for _, sid := range splitCSV(st.Get("PUBLIC_SUBNETS")) {
			if err := ensureRTBAssociation(ctx, clients.EC2, pubRTBID, sid); err != nil {
				return fmt.Errorf("phase06: associate public subnet %s: %w", sid, err)
			}
		}
		st.Set("PUBLIC_RTB", pubRTBID)
	}

	// --- Private route table (only if NAT GW exists) ---
	if natID != "" {
		privRTBID, err := findRTBByTagAndVPC(ctx, clients.EC2, name, vpcID, "private")
		if err != nil {
			return fmt.Errorf("phase06: listing private RTBs: %w", err)
		}
		if privRTBID == "" {
			if dryRun {
				fmt.Fprintf(os.Stderr, "[phase 06] dry-run: would create private route table → NAT\n")
			} else {
				privRTBID, err = createRTB(ctx, clients.EC2, name, vpcID, "private", cl.Tags, cl.Metadata.Labels)
				if err != nil {
					return fmt.Errorf("phase06: create private RTB: %w", err)
				}
				fmt.Fprintf(os.Stderr, "[phase 06] created private RTB %s\n", privRTBID)
			}
		} else {
			fmt.Fprintf(os.Stderr, "[phase 06] private RTB %s already exists, skipping\n", privRTBID)
		}

		if !dryRun && privRTBID != "" {
			// Default route → NAT GW.
			if err := ensureRoute(ctx, clients.EC2, privRTBID, "0.0.0.0/0", "", natID); err != nil {
				return fmt.Errorf("phase06: private route → NAT: %w", err)
			}
			// Associate private subnets.
			for _, sid := range splitCSV(st.Get("PRIVATE_SUBNETS")) {
				if err := ensureRTBAssociation(ctx, clients.EC2, privRTBID, sid); err != nil {
					return fmt.Errorf("phase06: associate private subnet %s: %w", sid, err)
				}
			}
			st.Set("PRIVATE_RTB", privRTBID)
		}
	} else {
		fmt.Fprintf(os.Stderr, "[phase 06] no NAT GW in state, skipping private route table\n")
	}

	if dryRun {
		return nil
	}
	return st.Save()
}

// Phase06RouteTablesDown deletes both route tables. Tolerates "already gone".
func Phase06RouteTablesDown(ctx context.Context, cl *intent.Cluster, st *state.State, clients *Clients) error {
	awsmw.CheckAuthOrDie(clients.Profile)
	name := cl.Metadata.Name
	vpcID := st.Get("VPC_ID")
	fmt.Fprintf(os.Stderr, "[phase 06 down] route tables: cluster=%s\n", name)

	// Collect all RTBs for this cluster (state + tag-discovery fallback).
	rtbIDs := collectRTBIDs(ctx, clients.EC2, name, vpcID, st)

	for _, rtbID := range rtbIDs {
		// Disassociate non-main associations first.
		if err := disassociateRTB(ctx, clients.EC2, rtbID); err != nil {
			return fmt.Errorf("phase06 down: disassociate RTB %s: %w", rtbID, err)
		}
		fmt.Fprintf(os.Stderr, "[phase 06 down] deleting RTB %s\n", rtbID)
		_, err := clients.EC2.DeleteRouteTable(ctx, &ec2.DeleteRouteTableInput{
			RouteTableId: ptr(rtbID),
		})
		if err := ignoreNotFound(err); err != nil {
			return fmt.Errorf("phase06 down: ec2:DeleteRouteTable %s: %w", rtbID, err)
		}
	}

	st.Set("PUBLIC_RTB", "")
	st.Set("PRIVATE_RTB", "")
	return st.Save()
}

// --- helpers ---

func createRTB(ctx context.Context, ec2c EC2API, clusterName, vpcID, visibility string,
	extraTags, labels map[string]string) (string, error) {

	compName := tags.CompRTB + "-" + visibility
	resourceTags := tags.Merge(
		tags.Required(clusterName, tags.CompRTB),
		map[string]string{tags.KeyName: clusterName + "-" + compName},
		extraTags,
		labels,
	)
	out, err := ec2c.CreateRouteTable(ctx, &ec2.CreateRouteTableInput{
		VpcId: ptr(vpcID),
		TagSpecifications: []ec2types.TagSpecification{
			tagSpecification(ec2types.ResourceTypeRouteTable, resourceTags),
		},
	})
	if err != nil {
		return "", err
	}
	return *out.RouteTable.RouteTableId, nil
}

// ensureRoute creates the default route if absent. igwID or natGWID must be
// non-empty (mutually exclusive). Swallows RouteAlreadyExists.
func ensureRoute(ctx context.Context, ec2c EC2API, rtbID, cidr, igwID, natGWID string) error {
	in := &ec2.CreateRouteInput{
		RouteTableId:         ptr(rtbID),
		DestinationCidrBlock: ptr(cidr),
	}
	if igwID != "" {
		in.GatewayId = ptr(igwID)
	} else if natGWID != "" {
		in.NatGatewayId = ptr(natGWID)
	}
	_, err := ec2c.CreateRoute(ctx, in)
	if err != nil {
		// Swallow "route already exists".
		type coder interface{ ErrorCode() string }
		e := err
		for e != nil {
			if ce, ok := e.(coder); ok {
				if ce.ErrorCode() == "RouteAlreadyExists" {
					return nil
				}
			}
			type unwrapper interface{ Unwrap() error }
			if u, ok := e.(unwrapper); ok {
				e = u.Unwrap()
			} else {
				break
			}
		}
		return err
	}
	return nil
}

// ensureRTBAssociation associates subnet with rtb if not already associated.
func ensureRTBAssociation(ctx context.Context, ec2c EC2API, rtbID, subnetID string) error {
	out, err := ec2c.DescribeRouteTables(ctx, &ec2.DescribeRouteTablesInput{
		Filters: []ec2types.Filter{
			{Name: ptr("route-table-id"), Values: []string{rtbID}},
		},
	})
	if err != nil {
		return err
	}
	if len(out.RouteTables) > 0 {
		for _, assoc := range out.RouteTables[0].Associations {
			if assoc.SubnetId != nil && *assoc.SubnetId == subnetID {
				return nil // already associated
			}
		}
	}
	_, err = ec2c.AssociateRouteTable(ctx, &ec2.AssociateRouteTableInput{
		RouteTableId: ptr(rtbID),
		SubnetId:     ptr(subnetID),
	})
	return err
}

// findRTBByTagAndVPC finds the route table tagged for cluster+vpc with a
// Name suffix matching the visibility ("public" / "private").
func findRTBByTagAndVPC(ctx context.Context, ec2c EC2API, clusterName, vpcID, visibility string) (string, error) {
	nameSuffix := clusterName + "-rtb-" + visibility
	out, err := ec2c.DescribeRouteTables(ctx, &ec2.DescribeRouteTablesInput{
		Filters: []ec2types.Filter{
			tags.ClusterFilter(clusterName),
			{Name: ptr("vpc-id"), Values: []string{vpcID}},
			{Name: ptr("tag:" + tags.KeyName), Values: []string{nameSuffix}},
		},
	})
	if err != nil {
		return "", err
	}
	if len(out.RouteTables) == 0 {
		return "", nil
	}
	return *out.RouteTables[0].RouteTableId, nil
}

// collectRTBIDs returns all non-main route tables for the cluster.
func collectRTBIDs(ctx context.Context, ec2c EC2API, clusterName, vpcID string, st *state.State) []string {
	var ids []string
	for _, key := range []string{"PUBLIC_RTB", "PRIVATE_RTB"} {
		if id := st.Get(key); id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) > 0 {
		return ids
	}
	// Tag-discovery fallback.
	filters := []ec2types.Filter{tags.ClusterFilter(clusterName)}
	if vpcID != "" {
		filters = append(filters, ec2types.Filter{
			Name:   ptr("vpc-id"),
			Values: []string{vpcID},
		})
	}
	out, err := ec2c.DescribeRouteTables(ctx, &ec2.DescribeRouteTablesInput{
		Filters: filters,
	})
	if err != nil {
		return nil
	}
	for _, rt := range out.RouteTables {
		if rt.RouteTableId != nil {
			ids = append(ids, *rt.RouteTableId)
		}
	}
	return ids
}

// disassociateRTB removes all explicit (non-main) associations from a route
// table so it can be deleted.
func disassociateRTB(ctx context.Context, ec2c EC2API, rtbID string) error {
	out, err := ec2c.DescribeRouteTables(ctx, &ec2.DescribeRouteTablesInput{
		Filters: []ec2types.Filter{
			{Name: ptr("route-table-id"), Values: []string{rtbID}},
		},
	})
	if err != nil {
		if e2 := ignoreNotFound(err); e2 == nil {
			return nil
		}
		return err
	}
	for _, rt := range out.RouteTables {
		for _, assoc := range rt.Associations {
			if assoc.Main != nil && *assoc.Main {
				continue // can't disassociate the main RTB
			}
			if assoc.RouteTableAssociationId == nil {
				continue
			}
			_, err := ec2c.DisassociateRouteTable(ctx, &ec2.DisassociateRouteTableInput{
				AssociationId: assoc.RouteTableAssociationId,
			})
			if err := ignoreNotFound(err); err != nil {
				return err
			}
		}
	}
	return nil
}

// splitCSV splits a comma-separated string into trimmed, non-empty parts.
func splitCSV(csv string) []string {
	if csv == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
