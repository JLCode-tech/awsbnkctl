package phases

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/awsmw"
	"github.com/JLCode-tech/awsbnkctl/internal/aws/state"
	"github.com/JLCode-tech/awsbnkctl/internal/aws/tags"
	"github.com/JLCode-tech/awsbnkctl/internal/intent"
)

// Phase05NAT creates the Elastic IP and NAT Gateway in the first public
// subnet. The NAT GW is placed in the public subnet so private subnets can
// route outbound traffic through it.
//
// Idempotent: lists by tag before creating.
func Phase05NAT(ctx context.Context, cl *intent.Cluster, st *state.State, clients *Clients, dryRun bool) error {
	awsmw.CheckAuthOrDie(clients.Profile)
	name := cl.Metadata.Name

	// The NAT GW lives in the first public subnet.
	publicSubnetsCSV := st.Get("PUBLIC_SUBNETS")
	if publicSubnetsCSV == "" {
		return fmt.Errorf("phase05: PUBLIC_SUBNETS not in state (run phase03 first)")
	}
	firstPublicSubnet := splitFirst(publicSubnetsCSV)

	fmt.Fprintf(os.Stderr, "[phase 05] nat: cluster=%s subnet=%s\n", name, firstPublicSubnet)

	// --- EIP ---
	eipAllocID, err := findEIPByTag(ctx, clients.EC2, name)
	if err != nil {
		return fmt.Errorf("phase05: listing EIPs by tag: %w", err)
	}
	if eipAllocID == "" {
		if dryRun {
			fmt.Fprintf(os.Stderr, "[phase 05] dry-run: would allocate EIP and create NAT GW in %s\n", firstPublicSubnet)
			st.Set("NAT_EIP_ALLOC", "dry-run-eip")
			st.Set("NAT_GW_ID", "dry-run-nat")
			return nil
		}
		eipTags := tags.Merge(
			tags.Required(name, tags.CompEIP),
			cl.Tags,
			cl.Metadata.Labels,
		)
		eipOut, err := clients.EC2.AllocateAddress(ctx, &ec2.AllocateAddressInput{
			Domain: ec2types.DomainTypeVpc,
			TagSpecifications: []ec2types.TagSpecification{
				tagSpecification(ec2types.ResourceTypeElasticIp, eipTags),
			},
		})
		if err != nil {
			return fmt.Errorf("phase05: ec2:AllocateAddress: %w", err)
		}
		eipAllocID = *eipOut.AllocationId
		fmt.Fprintf(os.Stderr, "[phase 05] allocated EIP %s\n", eipAllocID)
	} else {
		fmt.Fprintf(os.Stderr, "[phase 05] EIP %s already exists, skipping\n", eipAllocID)
	}
	st.Set("NAT_EIP_ALLOC", eipAllocID)

	// --- NAT GW ---
	natID, err := findNATByTag(ctx, clients.EC2, name)
	if err != nil {
		return fmt.Errorf("phase05: listing NAT GWs by tag: %w", err)
	}
	if natID != "" {
		fmt.Fprintf(os.Stderr, "[phase 05] NAT GW %s already exists, skipping\n", natID)
	} else {
		natTags := tags.Merge(
			tags.Required(name, tags.CompNAT),
			cl.Tags,
			cl.Metadata.Labels,
		)
		natOut, err := clients.EC2.CreateNatGateway(ctx, &ec2.CreateNatGatewayInput{
			SubnetId:     ptr(firstPublicSubnet),
			AllocationId: ptr(eipAllocID),
			TagSpecifications: []ec2types.TagSpecification{
				tagSpecification(ec2types.ResourceTypeNatgateway, natTags),
			},
		})
		if err != nil {
			return fmt.Errorf("phase05: ec2:CreateNatGateway: %w", err)
		}
		natID = *natOut.NatGateway.NatGatewayId
		fmt.Fprintf(os.Stderr, "[phase 05] created NAT GW %s (waiting for available...)\n", natID)

		if err := waitNATAvailable(ctx, clients.EC2, natID); err != nil {
			return fmt.Errorf("phase05: waiting for NAT GW available: %w", err)
		}
	}

	st.Set("NAT_GW_ID", natID)
	return st.Save()
}

// Phase05NATDown destroys the NAT GW and releases the EIP.
//
// Critical post-condition (ported from aws-gpu-setup/down.sh:200-204):
// After DeleteNatGateway, the underlying ENI takes ~30-60s to detach.
// The EIP AssociationId does NOT clear immediately. Releasing the EIP while
// still associated fails with InvalidIPAddress.InUse. We poll until
// AssociationId is nil before calling ReleaseAddress.
func Phase05NATDown(ctx context.Context, cl *intent.Cluster, st *state.State, clients *Clients) error {
	awsmw.CheckAuthOrDie(clients.Profile)
	name := cl.Metadata.Name
	fmt.Fprintf(os.Stderr, "[phase 05 down] nat: cluster=%s\n", name)

	// --- NAT GW ---
	natID := st.Get("NAT_GW_ID")
	if natID == "" {
		var err error
		natID, err = findNATByTag(ctx, clients.EC2, name)
		if err != nil {
			return fmt.Errorf("phase05 down: tag-discovery NAT: %w", err)
		}
	}
	if natID != "" {
		fmt.Fprintf(os.Stderr, "[phase 05 down] deleting NAT GW %s\n", natID)
		_, err := clients.EC2.DeleteNatGateway(ctx, &ec2.DeleteNatGatewayInput{
			NatGatewayId: ptr(natID),
		})
		if err := ignoreNotFound(err); err != nil {
			return fmt.Errorf("phase05 down: ec2:DeleteNatGateway: %w", err)
		}
		// Wait for NAT to reach deleted state so the EIP AssociationId clears.
		if err := waitNATDeleted(ctx, clients.EC2, natID); err != nil {
			return fmt.Errorf("phase05 down: waiting for NAT GW deleted: %w", err)
		}
	} else {
		fmt.Fprintf(os.Stderr, "[phase 05 down] NAT GW already gone\n")
	}
	st.Set("NAT_GW_ID", "")

	// --- EIP ---
	eipAllocID := st.Get("NAT_EIP_ALLOC")
	if eipAllocID == "" {
		var err error
		eipAllocID, err = findEIPByTag(ctx, clients.EC2, name)
		if err != nil {
			return fmt.Errorf("phase05 down: tag-discovery EIP: %w", err)
		}
	}
	if eipAllocID != "" {
		// Wait for AssociationId to clear (NAT ENI detach is async).
		if err := waitEIPUnassociated(ctx, clients.EC2, eipAllocID); err != nil {
			return fmt.Errorf("phase05 down: waiting for EIP unassociation: %w", err)
		}
		fmt.Fprintf(os.Stderr, "[phase 05 down] releasing EIP %s\n", eipAllocID)
		_, err := clients.EC2.ReleaseAddress(ctx, &ec2.ReleaseAddressInput{
			AllocationId: ptr(eipAllocID),
		})
		if err := ignoreNotFound(err); err != nil {
			return fmt.Errorf("phase05 down: ec2:ReleaseAddress: %w", err)
		}
	} else {
		fmt.Fprintf(os.Stderr, "[phase 05 down] EIP already gone\n")
	}
	st.Set("NAT_EIP_ALLOC", "")
	return st.Save()
}

// --- helpers ---

func findEIPByTag(ctx context.Context, ec2c EC2API, clusterName string) (string, error) {
	out, err := ec2c.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{
		Filters: []ec2types.Filter{tags.ClusterFilter(clusterName)},
	})
	if err != nil {
		return "", err
	}
	if len(out.Addresses) == 0 {
		return "", nil
	}
	return *out.Addresses[0].AllocationId, nil
}

func findNATByTag(ctx context.Context, ec2c EC2API, clusterName string) (string, error) {
	tagKey := "tag:" + tags.KeyCluster
	out, err := ec2c.DescribeNatGateways(ctx, &ec2.DescribeNatGatewaysInput{
		Filter: []ec2types.Filter{
			{Name: &tagKey, Values: []string{clusterName}},
			// Exclude terminated/failed gateways.
			{Name: ptr("state"), Values: []string{"pending", "available"}},
		},
	})
	if err != nil {
		return "", err
	}
	if len(out.NatGateways) == 0 {
		return "", nil
	}
	return *out.NatGateways[0].NatGatewayId, nil
}

// waitNATAvailable polls until the NAT GW reaches state "available".
// Timeout: 5 minutes.
func waitNATAvailable(ctx context.Context, ec2c EC2API, natID string) error {
	return pollNATState(ctx, ec2c, natID, ec2types.NatGatewayStateAvailable, 5*time.Minute)
}

// waitNATDeleted polls until the NAT GW reaches state "deleted".
// Timeout: 10 minutes.
func waitNATDeleted(ctx context.Context, ec2c EC2API, natID string) error {
	return pollNATState(ctx, ec2c, natID, ec2types.NatGatewayStateDeleted, 10*time.Minute)
}

func pollNATState(ctx context.Context, ec2c EC2API, natID string, want ec2types.NatGatewayState, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		out, err := ec2c.DescribeNatGateways(ctx, &ec2.DescribeNatGatewaysInput{
			NatGatewayIds: []string{natID},
		})
		if err != nil {
			if err2 := ignoreNotFound(err); err2 == nil {
				// NAT is gone — treat as deleted.
				if want == ec2types.NatGatewayStateDeleted {
					return nil
				}
				return fmt.Errorf("NAT GW %s disappeared while waiting for %s", natID, want)
			}
			return err
		}
		if len(out.NatGateways) == 0 {
			// DescribeNatGateways returns nothing when the ID isn't known to AWS
			// (fully purged) — treat as deleted.
			if want == ec2types.NatGatewayStateDeleted {
				return nil
			}
		} else {
			state := out.NatGateways[0].State
			if state == want {
				fmt.Fprintf(os.Stderr, "[phase 05] NAT GW %s reached state %s\n", natID, want)
				return nil
			}
			if state == ec2types.NatGatewayStateFailed {
				return fmt.Errorf("NAT GW %s entered failed state", natID)
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}
	return fmt.Errorf("timeout waiting for NAT GW %s to reach %s", natID, want)
}

// waitEIPUnassociated polls until the EIP's AssociationId is empty.
// After DeleteNatGateway, the ENI detach is async — the EIP stays associated
// for ~30-60s. Timeout: 5 minutes.
func waitEIPUnassociated(ctx context.Context, ec2c EC2API, allocID string) error {
	deadline := time.Now().Add(5 * time.Minute)
	for time.Now().Before(deadline) {
		out, err := ec2c.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{
			AllocationIds: []string{allocID},
		})
		if err != nil {
			if e2 := ignoreNotFound(err); e2 == nil {
				return nil // EIP already gone
			}
			return err
		}
		if len(out.Addresses) == 0 {
			return nil // gone
		}
		if out.Addresses[0].AssociationId == nil || *out.Addresses[0].AssociationId == "" {
			fmt.Fprintf(os.Stderr, "[phase 05] EIP %s unassociated, safe to release\n", allocID)
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}
	return fmt.Errorf("timeout waiting for EIP %s to unassociate", allocID)
}

// splitFirst returns the first element of a comma-separated string.
func splitFirst(csv string) string {
	for i, c := range csv {
		if c == ',' {
			return csv[:i]
		}
	}
	return csv
}
