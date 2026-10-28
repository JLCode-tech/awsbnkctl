package phases

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/awsmw"
	"github.com/JLCode-tech/awsbnkctl/internal/aws/state"
	"github.com/JLCode-tech/awsbnkctl/internal/intent"
)

func testCluster() *intent.Cluster {
	return &intent.Cluster{
		Metadata: intent.Metadata{Name: "tracer", Region: "ap-southeast-2"},
		Network: intent.Network{
			VPCCidr: "10.0.0.0/16",
			AZs:     []string{"ap-southeast-2a", "ap-southeast-2b"},
			Subnets: intent.Subnets{
				Public:  []intent.SubnetSpec{{CIDR: "10.0.1.0/24", AZ: "ap-southeast-2a"}},
				Private: []intent.SubnetSpec{{CIDR: "10.0.11.0/24", AZ: "ap-southeast-2a"}},
			},
			NatGateways: 1,
		},
	}
}

func testClients(ec2Mock EC2API) *Clients {
	return &Clients{EC2: ec2Mock, STS: &mockSTSImpl{accountID: "111122223333"}, Profile: "test"}
}

func strPtr(s string) *string { return &s }

func TestPhase02VPC_CreatesWhenAbsent(t *testing.T) {
	awsmw.ResetForTest()
	dir := t.TempDir()
	st, _ := state.Load(dir)

	vpcID := "vpc-0newone"
	cidr := "10.0.0.0/16"
	ec2m := &mockEC2{
		createVpcOut: &ec2.CreateVpcOutput{
			Vpc: &ec2types.Vpc{VpcId: &vpcID, CidrBlock: &cidr},
		},
	}

	if err := Phase02VPC(context.Background(), testCluster(), st, testClients(ec2m), false); err != nil {
		t.Fatalf("Phase02VPC: %v", err)
	}
	if ec2m.createVpcCalls != 1 {
		t.Errorf("expected 1 CreateVpc call, got %d", ec2m.createVpcCalls)
	}
	if st.Get("VPC_ID") != vpcID {
		t.Errorf("VPC_ID in state: got %q, want %q", st.Get("VPC_ID"), vpcID)
	}
}

func TestPhase02VPC_SkipsWhenPresent(t *testing.T) {
	awsmw.ResetForTest()
	dir := t.TempDir()
	st, _ := state.Load(dir)

	existingID := "vpc-0existing"
	ec2m := &mockEC2{
		describeVpcsOut: &ec2.DescribeVpcsOutput{
			Vpcs: []ec2types.Vpc{{VpcId: &existingID}},
		},
	}

	if err := Phase02VPC(context.Background(), testCluster(), st, testClients(ec2m), false); err != nil {
		t.Fatalf("Phase02VPC: %v", err)
	}
	if ec2m.createVpcCalls != 0 {
		t.Errorf("expected 0 CreateVpc calls when VPC exists, got %d", ec2m.createVpcCalls)
	}
	if st.Get("VPC_ID") != existingID {
		t.Errorf("VPC_ID in state: got %q, want %q", st.Get("VPC_ID"), existingID)
	}
}

func TestPhase02VPC_DryRun(t *testing.T) {
	awsmw.ResetForTest()
	dir := t.TempDir()
	st, _ := state.Load(dir)
	ec2m := &mockEC2{}

	if err := Phase02VPC(context.Background(), testCluster(), st, testClients(ec2m), true); err != nil {
		t.Fatalf("Phase02VPC dry-run: %v", err)
	}
	if ec2m.createVpcCalls != 0 {
		t.Errorf("expected 0 CreateVpc calls in dry-run, got %d", ec2m.createVpcCalls)
	}
}

func TestPhase02VPCDown_ToleratesAlreadyGone(t *testing.T) {
	awsmw.ResetForTest()
	dir := t.TempDir()
	st, _ := state.Load(dir)
	st.Set("VPC_ID", "vpc-0gone")

	// DeleteVpc in the mock always returns nil — already-gone tolerance is handled
	// by ignoreNotFound in production. This test verifies no panic / error on destroy.
	ec2m := &mockEC2{}
	if err := Phase02VPCDown(context.Background(), testCluster(), st, testClients(ec2m)); err != nil {
		t.Fatalf("Phase02VPCDown: %v", err)
	}
}

func TestIgnoreNotFound_SwallowsKnownCodes(t *testing.T) {
	codes := []string{
		"InvalidVpcID.NotFound",
		"InvalidSubnetID.NotFound",
		"InvalidRouteTableID.NotFound",
		"InvalidInternetGatewayID.NotFound",
		"InvalidNatGatewayID.NotFound",
		"InvalidAllocationID.NotFound",
		"InvalidNetworkInterfaceID.NotFound",
	}
	for _, code := range codes {
		err := ignoreNotFound(&notFoundAPIError{code: code})
		if err != nil {
			t.Errorf("ignoreNotFound(%q): expected nil, got %v", code, err)
		}
	}
}

func TestIgnoreNotFound_PassesThroughOtherErrors(t *testing.T) {
	err := ignoreNotFound(&notFoundAPIError{code: "AccessDenied"})
	if err == nil {
		t.Error("ignoreNotFound should not swallow AccessDenied")
	}
}

func TestPhase02VPCDown_SweepsResidualsAndMultipleTaggedVPCs(t *testing.T) {
	awsmw.ResetForTest()
	dir := t.TempDir()
	st, _ := state.Load(dir)
	st.Set("VPC_ID", "vpc-state-1")

	ec2m := &mockEC2{
		describeVpcsOut: &ec2.DescribeVpcsOutput{
			Vpcs: []ec2types.Vpc{
				{VpcId: strPtr("vpc-state-1")},
				{VpcId: strPtr("vpc-orphan-2")},
			},
		},
		describeENIsOut: &ec2.DescribeNetworkInterfacesOutput{
			NetworkInterfaces: []ec2types.NetworkInterface{
				{NetworkInterfaceId: strPtr("eni-avail-1"), Status: ec2types.NetworkInterfaceStatusAvailable},
			},
		},
		describeSGsOut: &ec2.DescribeSecurityGroupsOutput{
			SecurityGroups: []ec2types.SecurityGroup{
				{GroupId: strPtr("sg-default"), GroupName: strPtr("default")},
				{
					GroupId:   strPtr("sg-agent"),
					GroupName: strPtr("tracer-agent"),
					IpPermissions: []ec2types.IpPermission{
						{IpProtocol: strPtr("-1"), UserIdGroupPairs: []ec2types.UserIdGroupPair{{GroupId: strPtr("sg-jumphost")}}},
					},
					IpPermissionsEgress: []ec2types.IpPermission{
						{IpProtocol: strPtr("-1"), UserIdGroupPairs: []ec2types.UserIdGroupPair{{GroupId: strPtr("sg-jumphost")}}},
					},
				},
			},
		},
	}

	if err := Phase02VPCDown(context.Background(), testCluster(), st, testClients(ec2m)); err != nil {
		t.Fatalf("Phase02VPCDown: %v", err)
	}
	if ec2m.deleteVpcCalls != 2 {
		t.Errorf("deleteVpcCalls = %d, want 2 (state VPC + extra tagged orphan VPC)", ec2m.deleteVpcCalls)
	}
	if ec2m.deleteENICalls == 0 {
		t.Errorf("expected residual available ENI to be swept, deleteENICalls = 0")
	}
	if ec2m.revokeIngressCalls == 0 {
		t.Errorf("expected ingress rules on residual SG to be revoked, revokeIngressCalls = 0")
	}
	if ec2m.revokeEgressCalls == 0 {
		t.Errorf("expected egress rules on residual SG to be revoked, revokeEgressCalls = 0")
	}
	if ec2m.deleteSGCalls == 0 {
		t.Errorf("expected non-default residual SG to be deleted, deleteSGCalls = 0")
	}
}
