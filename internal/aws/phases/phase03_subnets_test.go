package phases

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/awsmw"
	"github.com/JLCode-tech/awsbnkctl/internal/aws/state"
)

func TestPhase03Subnets_CreatesWhenAbsent(t *testing.T) {
	awsmw.ResetForTest()
	dir := t.TempDir()
	st, _ := state.Load(dir)
	st.Set("VPC_ID", "vpc-0abc")

	pubID := "subnet-0pub"
	privID := "subnet-0priv"
	callCount := 0
	ec2m := &mockEC2{}
	ec2m.createSubnetOut = &ec2.CreateSubnetOutput{}

	// Return distinct IDs for successive calls using a call-counter override.
	// Since mockEC2 returns fixed out, we use separate mocks per call via the
	// counter in createSubnetCalls. Here we just verify calls happened and state is set.
	subnetIDMap := []string{pubID, privID}
	_ = subnetIDMap
	_ = callCount

	// Simple mock: first call returns pubID subnet, second privID.
	// We can't easily parameterise per-call responses without a full mock library,
	// so we verify call count and state key presence.
	pubSubnetID := "subnet-0pub1"
	privSubnetID := "subnet-0priv1"
	az := "ap-southeast-2a"
	ec2m.createSubnetOut = &ec2.CreateSubnetOutput{
		Subnet: &ec2types.Subnet{SubnetId: &pubSubnetID, AvailabilityZone: &az},
	}

	// Call with two specs (1 public, 1 private).
	cl := testCluster()
	if err := Phase03Subnets(context.Background(), cl, st, testClients(ec2m), false); err != nil {
		t.Fatalf("Phase03Subnets: %v", err)
	}

	// Both calls go through the same mock which always returns pubSubnetID —
	// that's fine for verifying call count and state-key presence.
	if ec2m.createSubnetCalls != 2 {
		t.Errorf("expected 2 CreateSubnet calls (1 public + 1 private), got %d", ec2m.createSubnetCalls)
	}
	if st.Get("PUBLIC_SUBNETS") == "" {
		t.Error("PUBLIC_SUBNETS not set in state after phase03")
	}
	if st.Get("PRIVATE_SUBNETS") == "" {
		t.Error("PRIVATE_SUBNETS not set in state after phase03")
	}
	_ = privSubnetID
}

func TestPhase03Subnets_SkipsWhenPresent(t *testing.T) {
	awsmw.ResetForTest()
	dir := t.TempDir()
	st, _ := state.Load(dir)
	st.Set("VPC_ID", "vpc-0abc")

	existingID := "subnet-0existing"
	ec2m := &mockEC2{
		describeSubnetsOut: &ec2.DescribeSubnetsOutput{
			Subnets: []ec2types.Subnet{{SubnetId: &existingID}},
		},
	}

	if err := Phase03Subnets(context.Background(), testCluster(), st, testClients(ec2m), false); err != nil {
		t.Fatalf("Phase03Subnets: %v", err)
	}
	if ec2m.createSubnetCalls != 0 {
		t.Errorf("expected 0 CreateSubnet calls when subnets exist, got %d", ec2m.createSubnetCalls)
	}
}

func TestPhase03Subnets_DryRun(t *testing.T) {
	awsmw.ResetForTest()
	dir := t.TempDir()
	st, _ := state.Load(dir)
	st.Set("VPC_ID", "vpc-0abc")
	ec2m := &mockEC2{}

	if err := Phase03Subnets(context.Background(), testCluster(), st, testClients(ec2m), true); err != nil {
		t.Fatalf("Phase03Subnets dry-run: %v", err)
	}
	if ec2m.createSubnetCalls != 0 {
		t.Errorf("expected 0 CreateSubnet calls in dry-run, got %d", ec2m.createSubnetCalls)
	}
}

func TestPhase03Subnets_ErrorsWithoutVPCID(t *testing.T) {
	awsmw.ResetForTest()
	dir := t.TempDir()
	st, _ := state.Load(dir)
	// VPC_ID not set.
	if err := Phase03Subnets(context.Background(), testCluster(), st, testClients(&mockEC2{}), false); err == nil {
		t.Fatal("expected error when VPC_ID missing, got nil")
	}
}
