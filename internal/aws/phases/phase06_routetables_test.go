package phases

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/awsmw"
	"github.com/JLCode-tech/awsbnkctl/internal/aws/state"
)

func TestPhase06RouteTables_CreatesWhenAbsent(t *testing.T) {
	awsmw.ResetForTest()
	dir := t.TempDir()
	st, _ := state.Load(dir)
	st.Set("VPC_ID", "vpc-0abc")
	st.Set("IGW_ID", "igw-0abc")
	st.Set("NAT_GW_ID", "nat-0abc")
	st.Set("PUBLIC_SUBNETS", "subnet-0pub1")
	st.Set("PRIVATE_SUBNETS", "subnet-0priv1")

	pubRTBID := "rtb-0pub"
	privRTBID := "rtb-0priv"
	callNum := 0
	ec2m := &mockEC2{}
	ec2m.createRTBOut = &ec2.CreateRouteTableOutput{
		RouteTable: &ec2types.RouteTable{RouteTableId: &pubRTBID},
	}
	// Alternate IDs: first call returns pubRTBID, second returns privRTBID.
	// Since mock returns same out, we just validate call count.
	_ = privRTBID
	_ = callNum

	if err := Phase06RouteTables(context.Background(), testCluster(), st, testClients(ec2m), false); err != nil {
		t.Fatalf("Phase06RouteTables: %v", err)
	}
	if ec2m.createRTBCalls != 2 {
		t.Errorf("expected 2 CreateRouteTable calls (public + private), got %d", ec2m.createRTBCalls)
	}
	// Two routes: one IGW, one NAT.
	if ec2m.createRouteCalls != 2 {
		t.Errorf("expected 2 CreateRoute calls, got %d", ec2m.createRouteCalls)
	}
}

func TestPhase06RouteTables_SkipsWhenPresent(t *testing.T) {
	awsmw.ResetForTest()
	dir := t.TempDir()
	st, _ := state.Load(dir)
	st.Set("VPC_ID", "vpc-0abc")
	st.Set("IGW_ID", "igw-0abc")
	st.Set("NAT_GW_ID", "nat-0abc")
	st.Set("PUBLIC_SUBNETS", "subnet-0pub1")
	st.Set("PRIVATE_SUBNETS", "subnet-0priv1")

	pubRTBID := "rtb-0pub-existing"
	privRTBID := "rtb-0priv-existing"
	pubName := "tracer-rtb-public"
	privName := "tracer-rtb-private"

	// Return existing RTBs on describe.
	ec2m := &mockEC2{
		describeRTBsOut: &ec2.DescribeRouteTablesOutput{
			RouteTables: []ec2types.RouteTable{
				{
					RouteTableId: &pubRTBID,
					Tags:         []ec2types.Tag{{Key: strPtr("Name"), Value: &pubName}},
				},
				{
					RouteTableId: &privRTBID,
					Tags:         []ec2types.Tag{{Key: strPtr("Name"), Value: &privName}},
				},
			},
		},
	}

	// Override describeRTBsOut so the findRTBByTagAndVPC for public returns
	// first, then private — in the mock we return all RTBs and rely on name
	// matching. Since findRTBByTagAndVPC uses name filter via the EC2 API
	// which our mock ignores (returns all), we need the mock to handle this.
	// For simplicity, alternate the result on each call.
	call := 0
	pubOut := &ec2.DescribeRouteTablesOutput{
		RouteTables: []ec2types.RouteTable{{RouteTableId: &pubRTBID}},
	}
	privOut := &ec2.DescribeRouteTablesOutput{
		RouteTables: []ec2types.RouteTable{{RouteTableId: &privRTBID}},
	}
	_ = privOut
	_ = call
	_ = ec2m

	// Use a simple mock that returns the pubRTBID for all calls (simulates already-exists).
	ec2m2 := &mockEC2{describeRTBsOut: pubOut}
	_ = ec2m2

	// Just verify zero create calls when describe returns results.
	ec2mSimple := &mockEC2{describeRTBsOut: pubOut}
	if err := Phase06RouteTables(context.Background(), testCluster(), st, testClients(ec2mSimple), false); err != nil {
		t.Fatalf("Phase06RouteTables: %v", err)
	}
	if ec2mSimple.createRTBCalls != 0 {
		t.Errorf("expected 0 CreateRouteTable calls when RTBs exist, got %d", ec2mSimple.createRTBCalls)
	}
}

func TestPhase06RouteTables_DryRun(t *testing.T) {
	awsmw.ResetForTest()
	dir := t.TempDir()
	st, _ := state.Load(dir)
	st.Set("VPC_ID", "vpc-0abc")
	st.Set("IGW_ID", "igw-0abc")
	st.Set("NAT_GW_ID", "nat-0abc")
	st.Set("PUBLIC_SUBNETS", "subnet-0pub1")
	st.Set("PRIVATE_SUBNETS", "subnet-0priv1")
	ec2m := &mockEC2{}

	if err := Phase06RouteTables(context.Background(), testCluster(), st, testClients(ec2m), true); err != nil {
		t.Fatalf("Phase06RouteTables dry-run: %v", err)
	}
	if ec2m.createRTBCalls != 0 || ec2m.createRouteCalls != 0 {
		t.Error("expected no AWS mutations in dry-run")
	}
}

func TestPhase06RouteTables_ErrorsWithoutVPCID(t *testing.T) {
	awsmw.ResetForTest()
	dir := t.TempDir()
	st, _ := state.Load(dir)
	st.Set("IGW_ID", "igw-0abc")
	if err := Phase06RouteTables(context.Background(), testCluster(), st, testClients(&mockEC2{}), false); err == nil {
		t.Fatal("expected error when VPC_ID missing")
	}
}
