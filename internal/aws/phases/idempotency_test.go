package phases

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/awsmw"
	"github.com/JLCode-tech/awsbnkctl/internal/aws/state"
)

// idempotencyMockEC2 extends mockEC2 to behave like a real API: after the
// first create call, subsequent describe calls return the created resource so
// the phase skips creation on the second run.
type idempotencyMockEC2 struct {
	mockEC2

	vpcID     string
	subnetIDs []string
	igwID     string
	eipID     string
	natID     string
	pubRTBID  string
	privRTBID string
}

func (m *idempotencyMockEC2) DescribeVpcs(ctx context.Context, in *ec2.DescribeVpcsInput, opts ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
	if m.vpcID != "" {
		return &ec2.DescribeVpcsOutput{Vpcs: []ec2types.Vpc{{VpcId: &m.vpcID}}}, nil
	}
	return &ec2.DescribeVpcsOutput{}, nil
}
func (m *idempotencyMockEC2) CreateVpc(ctx context.Context, in *ec2.CreateVpcInput, opts ...func(*ec2.Options)) (*ec2.CreateVpcOutput, error) {
	m.createVpcCalls++
	m.vpcID = "vpc-idem"
	cidr := "10.0.0.0/16"
	return &ec2.CreateVpcOutput{Vpc: &ec2types.Vpc{VpcId: &m.vpcID, CidrBlock: &cidr}}, nil
}
func (m *idempotencyMockEC2) DescribeSubnets(ctx context.Context, in *ec2.DescribeSubnetsInput, opts ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	if len(m.subnetIDs) > 0 {
		subnets := make([]ec2types.Subnet, len(m.subnetIDs))
		for i, id := range m.subnetIDs {
			id := id
			az := "ap-southeast-2a"
			subnets[i] = ec2types.Subnet{SubnetId: &id, AvailabilityZone: &az}
		}
		return &ec2.DescribeSubnetsOutput{Subnets: subnets}, nil
	}
	return &ec2.DescribeSubnetsOutput{}, nil
}
func (m *idempotencyMockEC2) CreateSubnet(ctx context.Context, in *ec2.CreateSubnetInput, opts ...func(*ec2.Options)) (*ec2.CreateSubnetOutput, error) {
	m.createSubnetCalls++
	id := "subnet-idem"
	az := "ap-southeast-2a"
	m.subnetIDs = append(m.subnetIDs, id)
	return &ec2.CreateSubnetOutput{Subnet: &ec2types.Subnet{SubnetId: &id, AvailabilityZone: &az}}, nil
}
func (m *idempotencyMockEC2) DescribeInternetGateways(ctx context.Context, in *ec2.DescribeInternetGatewaysInput, opts ...func(*ec2.Options)) (*ec2.DescribeInternetGatewaysOutput, error) {
	if m.igwID != "" {
		return &ec2.DescribeInternetGatewaysOutput{
			InternetGateways: []ec2types.InternetGateway{{
				InternetGatewayId: &m.igwID,
				Attachments:       []ec2types.InternetGatewayAttachment{{VpcId: &m.vpcID}},
			}},
		}, nil
	}
	return &ec2.DescribeInternetGatewaysOutput{}, nil
}
func (m *idempotencyMockEC2) CreateInternetGateway(ctx context.Context, in *ec2.CreateInternetGatewayInput, opts ...func(*ec2.Options)) (*ec2.CreateInternetGatewayOutput, error) {
	m.createIGWCalls++
	m.igwID = "igw-idem"
	return &ec2.CreateInternetGatewayOutput{InternetGateway: &ec2types.InternetGateway{InternetGatewayId: &m.igwID}}, nil
}
func (m *idempotencyMockEC2) DescribeAddresses(ctx context.Context, in *ec2.DescribeAddressesInput, opts ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error) {
	if m.eipID != "" {
		return &ec2.DescribeAddressesOutput{
			Addresses: []ec2types.Address{{AllocationId: &m.eipID, AssociationId: nil}},
		}, nil
	}
	return &ec2.DescribeAddressesOutput{}, nil
}
func (m *idempotencyMockEC2) AllocateAddress(ctx context.Context, in *ec2.AllocateAddressInput, opts ...func(*ec2.Options)) (*ec2.AllocateAddressOutput, error) {
	m.allocAddrCalls++
	m.eipID = "eipalloc-idem"
	return &ec2.AllocateAddressOutput{AllocationId: &m.eipID}, nil
}
func (m *idempotencyMockEC2) DescribeNatGateways(ctx context.Context, in *ec2.DescribeNatGatewaysInput, opts ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error) {
	if m.natID != "" {
		avail := ec2types.NatGatewayStateAvailable
		return &ec2.DescribeNatGatewaysOutput{
			NatGateways: []ec2types.NatGateway{{NatGatewayId: &m.natID, State: avail}},
		}, nil
	}
	return &ec2.DescribeNatGatewaysOutput{}, nil
}
func (m *idempotencyMockEC2) CreateNatGateway(ctx context.Context, in *ec2.CreateNatGatewayInput, opts ...func(*ec2.Options)) (*ec2.CreateNatGatewayOutput, error) {
	m.createNATCalls++
	m.natID = "nat-idem"
	avail := ec2types.NatGatewayStateAvailable
	return &ec2.CreateNatGatewayOutput{
		NatGateway: &ec2types.NatGateway{NatGatewayId: &m.natID, State: avail},
	}, nil
}
func (m *idempotencyMockEC2) DescribeRouteTables(ctx context.Context, in *ec2.DescribeRouteTablesInput, opts ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error) {
	var rts []ec2types.RouteTable
	if m.pubRTBID != "" {
		pubName := "tracer-rtb-public"
		rts = append(rts, ec2types.RouteTable{
			RouteTableId: &m.pubRTBID,
			Tags:         []ec2types.Tag{{Key: strPtr("Name"), Value: &pubName}},
		})
	}
	if m.privRTBID != "" {
		privName := "tracer-rtb-private"
		rts = append(rts, ec2types.RouteTable{
			RouteTableId: &m.privRTBID,
			Tags:         []ec2types.Tag{{Key: strPtr("Name"), Value: &privName}},
		})
	}
	return &ec2.DescribeRouteTablesOutput{RouteTables: rts}, nil
}
func (m *idempotencyMockEC2) CreateRouteTable(ctx context.Context, in *ec2.CreateRouteTableInput, opts ...func(*ec2.Options)) (*ec2.CreateRouteTableOutput, error) {
	m.createRTBCalls++
	if m.pubRTBID == "" {
		m.pubRTBID = "rtb-idem-pub"
		return &ec2.CreateRouteTableOutput{RouteTable: &ec2types.RouteTable{RouteTableId: &m.pubRTBID}}, nil
	}
	m.privRTBID = "rtb-idem-priv"
	return &ec2.CreateRouteTableOutput{RouteTable: &ec2types.RouteTable{RouteTableId: &m.privRTBID}}, nil
}

// TestIdempotency_SecondRunProducesZeroCreateCalls runs all phases twice
// against a stateful in-memory mock. The second run must make zero create calls.
func TestIdempotency_SecondRunProducesZeroCreateCalls(t *testing.T) {
	awsmw.ResetForTest()
	dir := t.TempDir()
	cl := testCluster()
	imock := &idempotencyMockEC2{}
	clients := testClients(imock)

	// Seed state with VPC_ID (normally set by phase02) for first run.
	run := func(label string) {
		st, err := state.Load(dir)
		if err != nil {
			t.Fatalf("%s: Load state: %v", label, err)
		}

		ctx := context.Background()

		if err := Phase02VPC(ctx, cl, st, clients, false); err != nil {
			t.Fatalf("%s Phase02VPC: %v", label, err)
		}
		if err := Phase03Subnets(ctx, cl, st, clients, false); err != nil {
			t.Fatalf("%s Phase03Subnets: %v", label, err)
		}
		if err := Phase04IGW(ctx, cl, st, clients, false); err != nil {
			t.Fatalf("%s Phase04IGW: %v", label, err)
		}
		if err := Phase05NAT(ctx, cl, st, clients, false); err != nil {
			t.Fatalf("%s Phase05NAT: %v", label, err)
		}
		if err := Phase06RouteTables(ctx, cl, st, clients, false); err != nil {
			t.Fatalf("%s Phase06RouteTables: %v", label, err)
		}
	}

	// First run — all resources created.
	run("run1")
	createCallsAfterRun1 := imock.createVpcCalls + imock.createSubnetCalls +
		imock.createIGWCalls + imock.allocAddrCalls + imock.createNATCalls + imock.createRTBCalls
	if createCallsAfterRun1 == 0 {
		t.Fatal("run1: expected some create calls, got 0")
	}

	// Snapshot create counts after run1.
	snap := struct{ vpc, subnet, igw, eip, nat, rtb int }{
		imock.createVpcCalls, imock.createSubnetCalls, imock.createIGWCalls,
		imock.allocAddrCalls, imock.createNATCalls, imock.createRTBCalls,
	}

	// Second run — all resources already exist, so create calls must not increase.
	run("run2")
	if imock.createVpcCalls != snap.vpc {
		t.Errorf("run2: createVpcCalls increased from %d to %d", snap.vpc, imock.createVpcCalls)
	}
	if imock.createSubnetCalls != snap.subnet {
		t.Errorf("run2: createSubnetCalls increased from %d to %d", snap.subnet, imock.createSubnetCalls)
	}
	if imock.createIGWCalls != snap.igw {
		t.Errorf("run2: createIGWCalls increased from %d to %d", snap.igw, imock.createIGWCalls)
	}
	if imock.allocAddrCalls != snap.eip {
		t.Errorf("run2: allocAddrCalls increased from %d to %d", snap.eip, imock.allocAddrCalls)
	}
	if imock.createNATCalls != snap.nat {
		t.Errorf("run2: createNATCalls increased from %d to %d", snap.nat, imock.createNATCalls)
	}
	if imock.createRTBCalls != snap.rtb {
		t.Errorf("run2: createRTBCalls increased from %d to %d", snap.rtb, imock.createRTBCalls)
	}
}
