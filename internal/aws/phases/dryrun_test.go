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

// TestDryRun_AllPhasesEndToEnd verifies that running phases 00–07 with
// dryRun=true:
//   - makes zero mutating AWS API calls
//   - all phases return nil
//   - state contains all expected placeholder values
//   - state.env is NOT written to disk
func TestDryRun_AllPhasesEndToEnd(t *testing.T) {
	awsmw.ResetForTest()
	dir := t.TempDir()

	// Phase11Kubeconfig resolves the kubeconfig path via cl.StateDir(), which
	// is relative to the process CWD (".awsbnkctl/<name>/kubeconfig"). Chdir
	// to the temp dir so StateDir() resolves under dir — matching the pattern
	// used by phase11_kubeconfig_test.go.
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	st, err := state.Load(dir)
	if err != nil {
		t.Fatalf("state.Load: %v", err)
	}

	ec2m := &mockEC2{}
	iamm := newMockIAM()
	eksm := newMockEKS()
	clients := &Clients{
		EC2:     ec2m,
		STS:     &mockSTSImpl{accountID: "111122223333"},
		IAM:     iamm,
		EKS:     eksm,
		Profile: "test",
	}
	cl := sydTracerCluster()
	// Add cluster spec for phases 08/10/11.
	cl.ClusterSpec = &intent.ClusterSpec{
		KubernetesVersion: "1.30",
		NodeGroups: []intent.NodeGroupSpec{
			{Name: "default", InstanceType: "t3.medium", DesiredSize: 1, MinSize: 1, MaxSize: 2, DiskSize: 50},
		},
	}
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
	if err := Phase07IAM(ctx, cl, st, clients, true); err != nil {
		t.Fatalf("Phase07IAM: %v", err)
	}
	if err := Phase08EKSCluster(ctx, cl, st, clients, true); err != nil {
		t.Fatalf("Phase08EKSCluster: %v", err)
	}
	// Phase 09: forge disabled on sydTracerCluster (no forge block) — must
	// return nil immediately without any forge HTTP calls.
	if err := Phase09ForgeRegister(ctx, cl, st, clients, true); err != nil {
		t.Fatalf("Phase09ForgeRegister (disabled): %v", err)
	}
	if err := Phase10NodeGroup(ctx, cl, st, clients, true); err != nil {
		t.Fatalf("Phase10NodeGroup: %v", err)
	}
	if err := Phase11Kubeconfig(ctx, cl, st, clients, true); err != nil {
		t.Fatalf("Phase11Kubeconfig: %v", err)
	}

	// Phase 12: requires bnk: block — add it with temp FAR/JWT files.
	farPath := writeDryRunFile(t, dir, "far.json", `{"auths":{}}`)
	jwtPath := writeDryRunFile(t, dir, "license.jwt", "jwt-token")
	cl.Bnk = &intent.BnkSpec{
		FARArchive:         farPath,
		JWT:                jwtPath,
		CertManagerVersion: "1.16.1",
	}
	if err := Phase12K8sFoundation(ctx, cl, st, clients, true); err != nil {
		t.Fatalf("Phase12K8sFoundation dry-run: %v", err)
	}
	if err := Phase13Postflight(ctx, cl, st, clients, true); err != nil {
		t.Fatalf("Phase13Postflight dry-run: %v", err)
	}

	// Zero mutating AWS calls — EC2.
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

	// Zero mutating AWS calls — IAM.
	if iamm.createRoleCalls != 0 {
		t.Errorf("createRoleCalls = %d, want 0", iamm.createRoleCalls)
	}
	if iamm.createInstanceProfileCalls != 0 {
		t.Errorf("createInstanceProfileCalls = %d, want 0", iamm.createInstanceProfileCalls)
	}
	if iamm.addRoleToInstanceProfileCalls != 0 {
		t.Errorf("addRoleToInstanceProfileCalls = %d, want 0", iamm.addRoleToInstanceProfileCalls)
	}

	// Zero mutating AWS calls — EKS.
	if eksm.createClusterCalls != 0 {
		t.Errorf("EKS createClusterCalls = %d, want 0", eksm.createClusterCalls)
	}
	if eksm.createNodegroupCalls != 0 {
		t.Errorf("EKS createNodegroupCalls = %d, want 0", eksm.createNodegroupCalls)
	}

	// Placeholder state values must be present — EC2 phases.
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

	// Placeholder state values — IAM phase 07.
	name := cl.Metadata.Name
	iamChecks := map[string]string{
		"EKS_CLUSTER_ROLE_ARN":       "arn:aws:iam::dry-run:role/" + name + "-eks-cluster-role",
		"EKS_NODE_ROLE_ARN":          "arn:aws:iam::dry-run:role/" + name + "-eks-node-role",
		"NODE_INSTANCE_PROFILE_NAME": name + "-node-instance-profile",
		"NODE_INSTANCE_PROFILE_ARN":  "arn:aws:iam::dry-run:instance-profile/" + name + "-node-instance-profile",
	}
	for key, want := range iamChecks {
		if got := st.Get(key); got != want {
			t.Errorf("dry-run state[%s] = %q, want %q", key, got, want)
		}
	}

	// Placeholder state values — EKS phase 08.
	eksChecks := map[string]string{
		"EKS_CLUSTER_NAME":   name,
		"EKS_CLUSTER_ARN":    "arn:aws:eks:dry-run:cluster/" + name,
		"EKS_ENDPOINT":       "https://dry-run.eks",
		"EKS_CA":             "dry-run-ca",
		"EKS_OIDC_URL":       "https://oidc.eks.dry-run/id/dry-run",
		"EKS_SECURITY_GROUP": "sg-dry-run",
		"EKS_VERSION":        "1.30",
	}
	for key, want := range eksChecks {
		if got := st.Get(key); got != want {
			t.Errorf("dry-run state[%s] = %q, want %q", key, got, want)
		}
	}

	// Node group dry-run state.
	if st.Get("NODEGROUP_DEFAULT_NAME") == "" {
		t.Error("dry-run: NODEGROUP_DEFAULT_NAME not set")
	}
	if st.Get("NODEGROUP_DEFAULT_ARN") == "" {
		t.Error("dry-run: NODEGROUP_DEFAULT_ARN not set")
	}

	// Kubeconfig dry-run state.
	if st.Get("KUBECONFIG_PATH") == "" {
		t.Error("dry-run: KUBECONFIG_PATH not set")
	}

	// state.env must NOT have been written to disk.
	stateEnvPath := filepath.Join(dir, "state.env")
	if _, err := os.Stat(stateEnvPath); err == nil {
		t.Error("state.env was written to disk during dry-run; must not persist")
	}

	// No kubeconfig file written in dry-run.
	// Phase11Kubeconfig writes to cl.StateDir()/kubeconfig which (after Chdir
	// to dir above) resolves to dir/.awsbnkctl/<name>/kubeconfig.
	kubeconfigPath := filepath.Join(dir, ".awsbnkctl", cl.Metadata.Name, "kubeconfig")
	if _, err := os.Stat(kubeconfigPath); err == nil {
		t.Error("kubeconfig was written to disk during dry-run; must not persist")
	}

	// Phase 12 dry-run placeholder state values.
	p12DryRunChecks := []string{
		"BNK_NAMESPACES_CREATED",
		"BNK_FAR_SECRET_NAME",
		"BNK_LICENSE_JWT_SECRET",
		"CERT_MANAGER_VERSION",
		"BNK_SELFSIGNED_ISSUER",
		"BNK_CA_CERT_NAME",
		"BNK_CA_SECRET_NAME",
		"BNK_CA_ISSUER",
	}
	for _, key := range p12DryRunChecks {
		if v := st.Get(key); v == "" {
			t.Errorf("dry-run: state[%s] is empty (phase 12 should set placeholders)", key)
		}
	}
}

// writeDryRunFile writes a file for use in dry-run tests and returns its path.
func writeDryRunFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("writeDryRunFile %s: %v", name, err)
	}
	return p
}

// TestDryRun_Phase09ForgeEnabled verifies that Phase09 in dry-run mode with
// forge enabled sets the FORGE_* placeholder state keys and writes no files
// (no link file, no HTTP calls).
func TestDryRun_Phase09ForgeEnabled(t *testing.T) {
	awsmw.ResetForTest()
	dir := t.TempDir()

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	st, err := state.Load(dir)
	if err != nil {
		t.Fatalf("state.Load: %v", err)
	}

	// Seed EKS state (normally written by Phase08).
	st.Set("EKS_CLUSTER_ARN", "arn:aws:eks:ap-southeast-2:111122223333:cluster/syd-tracer")
	st.Set("EKS_ENDPOINT", "https://dry-run.eks")
	st.Set("EKS_CA", "dry-run-ca")

	// Cluster with forge enabled pointing at a non-existent server — must not
	// be contacted in dry-run mode.
	cl := sydTracerCluster()
	cl.Forge = &intent.ForgeSpec{
		Enabled: true,
		MCPURL:  "http://127.0.0.1:19999/mcp/", // no server listening here
		URL:     "http://127.0.0.1:19998",
	}
	cl.ClusterSpec = &intent.ClusterSpec{
		KubernetesVersion: "1.30",
		NodeGroups: []intent.NodeGroupSpec{
			{Name: "default", InstanceType: "t3.medium", DesiredSize: 1, MinSize: 1, MaxSize: 2, DiskSize: 50},
		},
	}

	clients := &Clients{Profile: "test"}

	if err := Phase09ForgeRegister(context.Background(), cl, st, clients, true); err != nil {
		t.Fatalf("Phase09ForgeRegister dry-run: %v", err)
	}

	// Must have placeholder state keys.
	if st.Get("FORGE_PROJECT_ID") != "dry-run-project" {
		t.Errorf("FORGE_PROJECT_ID = %q, want dry-run-project", st.Get("FORGE_PROJECT_ID"))
	}
	if st.Get("FORGE_CLUSTER_ID") != "dry-run-cluster" {
		t.Errorf("FORGE_CLUSTER_ID = %q, want dry-run-cluster", st.Get("FORGE_CLUSTER_ID"))
	}
	if st.Get("FORGE_LINK_PATH") == "" {
		t.Error("FORGE_LINK_PATH not set in dry-run")
	}
	if st.Get("FORGE_STATUS") != "dry-run" {
		t.Errorf("FORGE_STATUS = %q, want dry-run", st.Get("FORGE_STATUS"))
	}

	// No forge_link.json written in dry-run.
	linkPath := filepath.Join(dir, ".awsbnkctl", cl.Metadata.Name, "forge_link.json")
	if _, err := os.Stat(linkPath); err == nil {
		t.Error("forge_link.json was written to disk during dry-run; must not persist")
	}
}
