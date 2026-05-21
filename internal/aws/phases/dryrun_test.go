package phases

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/awsmw"
	"github.com/JLCode-tech/awsbnkctl/internal/aws/state"
	"github.com/JLCode-tech/awsbnkctl/internal/intent"
)

// sydTracerCluster returns a cluster matching the syd-tracer shape:
// 2 public + 2 private subnets across two AZs.
func sydTracerCluster() *intent.Cluster {
	return &intent.Cluster{
		Metadata: intent.Metadata{Name: "syd-tracer", Region: "ap-southeast-2"},
		Network: intent.Network{
			VPCCidr: "10.0.0.0/16",
			AZs:     []string{"ap-southeast-2a", "ap-southeast-2b"},
			Subnets: intent.Subnets{
				Public: []intent.SubnetSpec{
					{CIDR: "10.0.1.0/24", AZ: "ap-southeast-2a"},
					{CIDR: "10.0.2.0/24", AZ: "ap-southeast-2b"},
				},
				Private: []intent.SubnetSpec{
					{CIDR: "10.0.11.0/24", AZ: "ap-southeast-2a"},
					{CIDR: "10.0.12.0/24", AZ: "ap-southeast-2b"},
				},
			},
			NatGateways: 1,
		},
	}
}

// TestDryRun_AllPhasesEndToEnd verifies that running phases 00–06 with
// dryRun=true:
//   - makes zero mutating AWS API calls
//   - all phases return nil
//   - state contains all expected placeholder values
//   - state.env is NOT written to disk
func TestDryRun_AllPhasesEndToEnd(t *testing.T) {
	awsmw.ResetForTest()
	dir := t.TempDir()
	st, err := state.Load(dir)
	if err != nil {
		t.Fatalf("state.Load: %v", err)
	}

	ec2m := &mockEC2{}
	clients := testClients(ec2m)
	cl := sydTracerCluster()
	ctx := context.Background()

	// Run all phases with dryRun=true.
	if err := Phase00Preflight(ctx, cl, st, clients, true); err != nil {
		t.Fatalf("Phase00Preflight: %v", err)
	}
	if err := Phase02VPC(ctx, cl, st, clients, true); err != nil {
		t.Fatalf("Phase02VPC: %v", err)
	}
	if err := Phase03Subnets(ctx, cl, st, clients, true); err != nil {
		t.Fatalf("Phase03Subnets: %v", err)
	}
	if err := Phase04IGW(ctx, cl, st, clients, true); err != nil {
		t.Fatalf("Phase04IGW: %v", err)
	}
	if err := Phase05NAT(ctx, cl, st, clients, true); err != nil {
		t.Fatalf("Phase05NAT: %v", err)
	}
	if err := Phase06RouteTables(ctx, cl, st, clients, true); err != nil {
		t.Fatalf("Phase06RouteTables: %v", err)
	}

	// Zero mutating AWS calls.
	if ec2m.createVpcCalls != 0 {
		t.Errorf("createVpcCalls = %d, want 0", ec2m.createVpcCalls)
	}
	if ec2m.createSubnetCalls != 0 {
		t.Errorf("createSubnetCalls = %d, want 0", ec2m.createSubnetCalls)
	}
	if ec2m.createIGWCalls != 0 {
		t.Errorf("createIGWCalls = %d, want 0", ec2m.createIGWCalls)
	}
	if ec2m.allocAddrCalls != 0 {
		t.Errorf("allocAddrCalls = %d, want 0", ec2m.allocAddrCalls)
	}
	if ec2m.createNATCalls != 0 {
		t.Errorf("createNATCalls = %d, want 0", ec2m.createNATCalls)
	}
	if ec2m.createRTBCalls != 0 {
		t.Errorf("createRTBCalls = %d, want 0", ec2m.createRTBCalls)
	}

	// Placeholder state values must be present.
	checks := map[string]string{
		"VPC_ID":          "dry-run-vpc",
		"PUBLIC_SUBNETS":  "dry-run-subnet-pub-1,dry-run-subnet-pub-2",
		"PRIVATE_SUBNETS": "dry-run-subnet-priv-1,dry-run-subnet-priv-2",
		"IGW_ID":          "dry-run-igw",
		"NAT_EIP_ALLOC":   "dry-run-eip",
		"NAT_GW_ID":       "dry-run-nat",
		"PUBLIC_RTB":      "dry-run-rtb-pub",
		"PRIVATE_RTB":     "dry-run-rtb-priv",
	}
	for key, want := range checks {
		if got := st.Get(key); got != want {
			t.Errorf("state[%s] = %q, want %q", key, got, want)
		}
	}

	// state.env must NOT have been written to disk.
	stateEnvPath := filepath.Join(dir, "state.env")
	if _, err := os.Stat(stateEnvPath); err == nil {
		t.Error("state.env was written to disk during dry-run; must not persist")
	}
}
