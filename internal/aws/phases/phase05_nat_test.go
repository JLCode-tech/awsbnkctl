package phases

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/awsmw"
	"github.com/JLCode-tech/awsbnkctl/internal/aws/state"
)

// natCreationMock is a stateful mockEC2 that:
//   - Returns no NAT on the first DescribeNatGateways (findNATByTag → absent).
//   - After CreateNatGateway, the next DescribeNatGateways (waitNATAvailable)
//     returns the new NAT as available.
type natCreationMock struct {
	mockEC2
	natID       string
	createCalls int
}

func (m *natCreationMock) DescribeNatGateways(_ context.Context, _ *ec2.DescribeNatGatewaysInput, _ ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error) {
	if m.natID == "" {
		return &ec2.DescribeNatGatewaysOutput{}, nil
	}
	avail := ec2types.NatGatewayStateAvailable
	return &ec2.DescribeNatGatewaysOutput{
		NatGateways: []ec2types.NatGateway{{NatGatewayId: &m.natID, State: avail}},
	}, nil
}
func (m *natCreationMock) CreateNatGateway(_ context.Context, _ *ec2.CreateNatGatewayInput, _ ...func(*ec2.Options)) (*ec2.CreateNatGatewayOutput, error) {
	m.createNATCalls++
	m.natID = "nat-0new"
	avail := ec2types.NatGatewayStateAvailable
	return &ec2.CreateNatGatewayOutput{
		NatGateway: &ec2types.NatGateway{NatGatewayId: &m.natID, State: avail},
	}, nil
}

func TestPhase05NAT_CreatesWhenAbsent(t *testing.T) {
	awsmw.ResetForTest()
	dir := t.TempDir()
	st, _ := state.Load(dir)
	st.Set("PUBLIC_SUBNETS", "subnet-0pub1")

	eipAllocID := "eipalloc-0new"
	natID := "nat-0new"
	ec2m := &natCreationMock{}
	ec2m.allocAddrOut = &ec2.AllocateAddressOutput{AllocationId: &eipAllocID}

	// Wrap in a Clients using the natCreationMock as EC2API.
	clients := &Clients{EC2: ec2m, STS: &mockSTSImpl{accountID: "111122223333"}, Profile: "test"}

	if err := Phase05NAT(context.Background(), testCluster(), st, clients, false); err != nil {
		t.Fatalf("Phase05NAT: %v", err)
	}
	if ec2m.allocAddrCalls != 1 {
		t.Errorf("expected 1 AllocateAddress call, got %d", ec2m.allocAddrCalls)
	}
	if ec2m.createNATCalls != 1 {
		t.Errorf("expected 1 CreateNatGateway call, got %d", ec2m.createNATCalls)
	}
	if st.Get("NAT_GW_ID") != natID {
		t.Errorf("NAT_GW_ID in state: got %q, want %q", st.Get("NAT_GW_ID"), natID)
	}
	if st.Get("NAT_EIP_ALLOC") != eipAllocID {
		t.Errorf("NAT_EIP_ALLOC in state: got %q, want %q", st.Get("NAT_EIP_ALLOC"), eipAllocID)
	}
}

func TestPhase05NAT_SkipsWhenPresent(t *testing.T) {
	awsmw.ResetForTest()
	dir := t.TempDir()
	st, _ := state.Load(dir)
	st.Set("PUBLIC_SUBNETS", "subnet-0pub1")

	existingEIP := "eipalloc-0existing"
	existingNAT := "nat-0existing"
	availableState := ec2types.NatGatewayStateAvailable

	ec2m := &mockEC2{
		describeAddrsOut: &ec2.DescribeAddressesOutput{
			Addresses: []ec2types.Address{{AllocationId: &existingEIP}},
		},
		describeNATsOut: &ec2.DescribeNatGatewaysOutput{
			NatGateways: []ec2types.NatGateway{
				{NatGatewayId: &existingNAT, State: availableState},
			},
		},
	}

	if err := Phase05NAT(context.Background(), testCluster(), st, testClients(ec2m), false); err != nil {
		t.Fatalf("Phase05NAT: %v", err)
	}
	if ec2m.allocAddrCalls != 0 {
		t.Errorf("expected 0 AllocateAddress calls when EIP exists, got %d", ec2m.allocAddrCalls)
	}
	if ec2m.createNATCalls != 0 {
		t.Errorf("expected 0 CreateNatGateway calls when NAT exists, got %d", ec2m.createNATCalls)
	}
}

func TestPhase05NAT_DryRun(t *testing.T) {
	awsmw.ResetForTest()
	dir := t.TempDir()
	st, _ := state.Load(dir)
	st.Set("PUBLIC_SUBNETS", "subnet-0pub1")
	ec2m := &mockEC2{}

	if err := Phase05NAT(context.Background(), testCluster(), st, testClients(ec2m), true); err != nil {
		t.Fatalf("Phase05NAT dry-run: %v", err)
	}
	if ec2m.allocAddrCalls != 0 || ec2m.createNATCalls != 0 {
		t.Error("expected no AWS mutations in dry-run")
	}
}

func TestPhase05NATDown_WaitsForEIPUnassociation(t *testing.T) {
	awsmw.ResetForTest()
	dir := t.TempDir()
	st, _ := state.Load(dir)
	st.Set("NAT_GW_ID", "nat-0gone")
	st.Set("NAT_EIP_ALLOC", "eipalloc-0release")

	// EIP has no AssociationId — unassociated immediately.
	allocID := "eipalloc-0release"
	ec2m := &mockEC2{
		describeAddrsOut: &ec2.DescribeAddressesOutput{
			Addresses: []ec2types.Address{{AllocationId: &allocID, AssociationId: nil}},
		},
		describeNATsOut: &ec2.DescribeNatGatewaysOutput{
			NatGateways: []ec2types.NatGateway{},
		},
	}

	if err := Phase05NATDown(context.Background(), testCluster(), st, testClients(ec2m)); err != nil {
		t.Fatalf("Phase05NATDown: %v", err)
	}
	if ec2m.releaseAddrCalls != 1 {
		t.Errorf("expected 1 ReleaseAddress call, got %d", ec2m.releaseAddrCalls)
	}
}
