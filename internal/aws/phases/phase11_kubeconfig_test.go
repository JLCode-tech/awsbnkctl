package phases

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/awsmw"
	"github.com/JLCode-tech/awsbnkctl/internal/aws/state"
)

func stateWithEKSOutput(t *testing.T, clusterDir string) *state.State {
	t.Helper()
	st, err := state.Load(clusterDir)
	if err != nil {
		t.Fatalf("state.Load: %v", err)
	}
	st.Set("EKS_CLUSTER_ARN", "arn:aws:eks:ap-southeast-2:111122223333:cluster/test-cluster")
	st.Set("EKS_ENDPOINT", "https://test-cluster.eks.ap-southeast-2.amazonaws.com")
	st.Set("EKS_CA", "dGVzdC1jYQ==")
	return st
}

func TestPhase11Kubeconfig_WritesFile(t *testing.T) {
	awsmw.ResetForTest()
	dir := t.TempDir()
	cl := testClusterWithEKS()
	// Override StateDir to use temp dir.
	cl.Metadata.Name = "test-cluster"
	// state.Load needs the dir to exist
	clusterDir := filepath.Join(dir, ".awsbnkctl", cl.Metadata.Name)
	if err := os.MkdirAll(clusterDir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	st := stateWithEKSOutput(t, clusterDir)
	clients := &Clients{
		EKS:     newMockEKS(),
		Profile: "test",
	}

	// Change cwd so StateDir resolves correctly.
	oldWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := Phase11Kubeconfig(context.Background(), cl, st, clients, false); err != nil {
		t.Fatalf("Phase11Kubeconfig: %v", err)
	}

	// File should exist.
	kubeconfigPath := filepath.Join(clusterDir, "kubeconfig")
	data, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		t.Fatalf("reading kubeconfig: %v", err)
	}

	content := string(data)

	// Check key fields in the output.
	if !strings.Contains(content, "arn:aws:eks:ap-southeast-2:111122223333:cluster/test-cluster") {
		t.Error("kubeconfig missing cluster ARN")
	}
	if !strings.Contains(content, "https://test-cluster.eks.ap-southeast-2.amazonaws.com") {
		t.Error("kubeconfig missing endpoint")
	}
	if !strings.Contains(content, "dGVzdC1jYQ==") {
		t.Error("kubeconfig missing CA data")
	}
	if !strings.Contains(content, "get-token") {
		t.Error("kubeconfig missing exec plugin args (get-token)")
	}
	if !strings.Contains(content, "ap-southeast-2") {
		t.Error("kubeconfig missing region in exec args")
	}
	if !strings.Contains(content, "client.authentication.k8s.io/v1beta1") {
		t.Error("kubeconfig missing exec apiVersion")
	}

	// State key set.
	if st.Get("KUBECONFIG_PATH") == "" {
		t.Error("KUBECONFIG_PATH not set in state")
	}

	// File permissions 0o600.
	info, err := os.Stat(kubeconfigPath)
	if err != nil {
		t.Fatalf("stat kubeconfig: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("kubeconfig mode = %o, want 0600", info.Mode().Perm())
	}
}

func TestPhase11Kubeconfig_DryRun(t *testing.T) {
	awsmw.ResetForTest()
	dir := t.TempDir()
	cl := testClusterWithEKS()
	cl.Metadata.Name = "test-cluster"
	clusterDir := filepath.Join(dir, ".awsbnkctl", cl.Metadata.Name)
	if err := os.MkdirAll(clusterDir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	st := stateWithEKSOutput(t, clusterDir)
	clients := &Clients{EKS: newMockEKS(), Profile: "test"}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := Phase11Kubeconfig(context.Background(), cl, st, clients, true); err != nil {
		t.Fatalf("Phase11Kubeconfig dryRun: %v", err)
	}

	// No file written in dry-run.
	kubeconfigPath := filepath.Join(clusterDir, "kubeconfig")
	if _, err := os.Stat(kubeconfigPath); err == nil {
		t.Error("dry-run: kubeconfig file was written, should not be")
	}

	// But state key is set.
	if st.Get("KUBECONFIG_PATH") == "" {
		t.Error("dry-run: KUBECONFIG_PATH not set")
	}
}

func TestPhase11KubeconfigDown_DeletesFile(t *testing.T) {
	awsmw.ResetForTest()
	dir := t.TempDir()
	cl := testClusterWithEKS()
	cl.Metadata.Name = "test-cluster"
	clusterDir := filepath.Join(dir, ".awsbnkctl", cl.Metadata.Name)
	if err := os.MkdirAll(clusterDir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	st := stateWithEKSOutput(t, clusterDir)
	clients := &Clients{EKS: newMockEKS(), Profile: "test"}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	ctx := context.Background()

	// Write the kubeconfig first.
	if err := Phase11Kubeconfig(ctx, cl, st, clients, false); err != nil {
		t.Fatalf("Phase11Kubeconfig: %v", err)
	}

	// Down should remove it.
	if err := Phase11KubeconfigDown(ctx, cl, st, clients); err != nil {
		t.Fatalf("Phase11KubeconfigDown: %v", err)
	}

	kubeconfigPath := filepath.Join(clusterDir, "kubeconfig")
	if _, err := os.Stat(kubeconfigPath); err == nil {
		t.Error("kubeconfig file still exists after down")
	}

	if st.Get("KUBECONFIG_PATH") != "" {
		t.Error("KUBECONFIG_PATH not cleared after down")
	}
}

func TestPhase11KubeconfigDown_ToleratesAbsentFile(t *testing.T) {
	awsmw.ResetForTest()
	dir := t.TempDir()
	cl := testClusterWithEKS()
	cl.Metadata.Name = "test-cluster"
	clusterDir := filepath.Join(dir, ".awsbnkctl", cl.Metadata.Name)
	if err := os.MkdirAll(clusterDir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	st, err := state.Load(clusterDir)
	if err != nil {
		t.Fatalf("state.Load: %v", err)
	}
	clients := &Clients{EKS: newMockEKS(), Profile: "test"}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	// Down with no file present — should not error.
	if err := Phase11KubeconfigDown(context.Background(), cl, st, clients); err != nil {
		t.Fatalf("Phase11KubeconfigDown (absent): %v", err)
	}
}
