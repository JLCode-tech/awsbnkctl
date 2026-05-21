package phases

import (
	"context"
	"testing"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/awsmw"
	"github.com/JLCode-tech/awsbnkctl/internal/aws/state"
	"github.com/JLCode-tech/awsbnkctl/internal/intent"
)

func testClusterWithEKS() *intent.Cluster {
	cl := testCluster()
	cl.ClusterSpec = &intent.ClusterSpec{
		KubernetesVersion: "1.30",
		NodeGroups: []intent.NodeGroupSpec{
			{Name: "default", InstanceType: "t3.medium", DesiredSize: 1, MinSize: 1, MaxSize: 2, DiskSize: 50},
		},
	}
	return cl
}

func stateWithIAMAndSubnets(t *testing.T) (*state.State, string) {
	t.Helper()
	dir := t.TempDir()
	st, err := state.Load(dir)
	if err != nil {
		t.Fatalf("state.Load: %v", err)
	}
	st.Set("EKS_CLUSTER_ROLE_ARN", "arn:aws:iam::111122223333:role/test-eks-cluster-role")
	st.Set("EKS_NODE_ROLE_ARN", "arn:aws:iam::111122223333:role/test-eks-node-role")
	st.Set("PUBLIC_SUBNETS", "subnet-pub-1,subnet-pub-2")
	st.Set("PRIVATE_SUBNETS", "subnet-priv-1,subnet-priv-2")
	return st, dir
}

func TestPhase08EKSCluster_CreatesClusterWithCorrectSubnets(t *testing.T) {
	awsmw.ResetForTest()
	cl := testClusterWithEKS()
	st, _ := stateWithIAMAndSubnets(t)
	eksMock := newMockEKS()
	clients := &Clients{
		EC2:     &mockEC2{},
		STS:     &mockSTSImpl{accountID: "111122223333"},
		IAM:     newMockIAM(),
		EKS:     eksMock,
		Profile: "test",
	}

	if err := Phase08EKSCluster(context.Background(), cl, st, clients, false); err != nil {
		t.Fatalf("Phase08EKSCluster: %v", err)
	}

	if eksMock.createClusterCalls != 1 {
		t.Errorf("createClusterCalls = %d, want 1", eksMock.createClusterCalls)
	}

	c, ok := eksMock.clusters[cl.Metadata.Name]
	if !ok {
		t.Fatal("cluster not created in mock")
	}

	// Should have all 4 subnets (2 public + 2 private).
	if len(c.ResourcesVpcConfig.SubnetIds) != 4 {
		t.Errorf("subnet count = %d, want 4 (2 public + 2 private)", len(c.ResourcesVpcConfig.SubnetIds))
	}

	// State keys populated.
	if st.Get("EKS_CLUSTER_ARN") == "" {
		t.Error("EKS_CLUSTER_ARN not set in state")
	}
	if st.Get("EKS_ENDPOINT") == "" {
		t.Error("EKS_ENDPOINT not set in state")
	}
	if st.Get("EKS_CA") == "" {
		t.Error("EKS_CA not set in state")
	}
	if st.Get("EKS_OIDC_URL") == "" {
		t.Error("EKS_OIDC_URL not set in state")
	}
	if st.Get("EKS_SECURITY_GROUP") == "" {
		t.Error("EKS_SECURITY_GROUP not set in state")
	}
	if st.Get("EKS_VERSION") != "1.30" {
		t.Errorf("EKS_VERSION = %q, want 1.30", st.Get("EKS_VERSION"))
	}
	if st.Get("EKS_CLUSTER_NAME") != cl.Metadata.Name {
		t.Errorf("EKS_CLUSTER_NAME = %q, want %q", st.Get("EKS_CLUSTER_NAME"), cl.Metadata.Name)
	}
}

func TestPhase08EKSCluster_Idempotent(t *testing.T) {
	awsmw.ResetForTest()
	cl := testClusterWithEKS()
	st, _ := stateWithIAMAndSubnets(t)
	eksMock := newMockEKS()
	clients := &Clients{
		EC2:     &mockEC2{},
		STS:     &mockSTSImpl{accountID: "111122223333"},
		IAM:     newMockIAM(),
		EKS:     eksMock,
		Profile: "test",
	}

	ctx := context.Background()

	// First run.
	if err := Phase08EKSCluster(ctx, cl, st, clients, false); err != nil {
		t.Fatalf("run1 Phase08EKSCluster: %v", err)
	}
	createAfterRun1 := eksMock.createClusterCalls

	// Second run — should not call CreateCluster again.
	if err := Phase08EKSCluster(ctx, cl, st, clients, false); err != nil {
		t.Fatalf("run2 Phase08EKSCluster: %v", err)
	}
	if eksMock.createClusterCalls != createAfterRun1 {
		t.Errorf("run2 called CreateCluster: %d → %d, want no increase", createAfterRun1, eksMock.createClusterCalls)
	}
}

func TestPhase08EKSCluster_MissingClusterSpec(t *testing.T) {
	awsmw.ResetForTest()
	cl := testCluster() // no ClusterSpec
	st, _ := stateWithIAMAndSubnets(t)
	clients := &Clients{
		EC2:     &mockEC2{},
		STS:     &mockSTSImpl{accountID: "111122223333"},
		IAM:     newMockIAM(),
		EKS:     newMockEKS(),
		Profile: "test",
	}

	err := Phase08EKSCluster(context.Background(), cl, st, clients, false)
	if err == nil {
		t.Error("expected error when ClusterSpec is nil, got nil")
	}
}

func TestPhase08EKSClusterDown_ToleratesNotFound(t *testing.T) {
	awsmw.ResetForTest()
	cl := testClusterWithEKS()
	st, _ := stateWithIAMAndSubnets(t)
	eksMock := newMockEKS() // empty — cluster doesn't exist
	clients := &Clients{
		EC2:     &mockEC2{},
		STS:     &mockSTSImpl{accountID: "111122223333"},
		IAM:     newMockIAM(),
		EKS:     eksMock,
		Profile: "test",
	}

	// Down should succeed even though cluster never existed.
	if err := Phase08EKSClusterDown(context.Background(), cl, st, clients); err != nil {
		t.Fatalf("Phase08EKSClusterDown (not-found): %v", err)
	}
}

func TestPhase08EKSClusterDown_DeletesCluster(t *testing.T) {
	awsmw.ResetForTest()
	cl := testClusterWithEKS()
	st, _ := stateWithIAMAndSubnets(t)
	eksMock := newMockEKS()
	clients := &Clients{
		EC2:     &mockEC2{},
		STS:     &mockSTSImpl{accountID: "111122223333"},
		IAM:     newMockIAM(),
		EKS:     eksMock,
		Profile: "test",
	}

	ctx := context.Background()

	// Create first.
	if err := Phase08EKSCluster(ctx, cl, st, clients, false); err != nil {
		t.Fatalf("Phase08EKSCluster: %v", err)
	}

	// Down should delete.
	if err := Phase08EKSClusterDown(ctx, cl, st, clients); err != nil {
		t.Fatalf("Phase08EKSClusterDown: %v", err)
	}

	if eksMock.deleteClusterCalls != 1 {
		t.Errorf("deleteClusterCalls = %d, want 1", eksMock.deleteClusterCalls)
	}
	if _, ok := eksMock.clusters[cl.Metadata.Name]; ok {
		t.Error("cluster still exists in mock after down")
	}

	// State cleared.
	if st.Get("EKS_CLUSTER_NAME") != "" {
		t.Errorf("EKS_CLUSTER_NAME not cleared after down")
	}
}

func TestPhase08EKSCluster_DryRun(t *testing.T) {
	awsmw.ResetForTest()
	cl := testClusterWithEKS()
	st, _ := stateWithIAMAndSubnets(t)
	eksMock := newMockEKS()
	clients := &Clients{
		EC2:     &mockEC2{},
		STS:     &mockSTSImpl{accountID: "111122223333"},
		IAM:     newMockIAM(),
		EKS:     eksMock,
		Profile: "test",
	}

	if err := Phase08EKSCluster(context.Background(), cl, st, clients, true); err != nil {
		t.Fatalf("Phase08EKSCluster dryRun: %v", err)
	}
	if eksMock.createClusterCalls != 0 {
		t.Errorf("dry-run: createClusterCalls = %d, want 0", eksMock.createClusterCalls)
	}
	if st.Get("EKS_CLUSTER_ARN") == "" {
		t.Error("dry-run: EKS_CLUSTER_ARN not populated")
	}
}
