package phases

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/awsmw"
	"github.com/JLCode-tech/awsbnkctl/internal/aws/state"
	"github.com/JLCode-tech/awsbnkctl/internal/intent"
)

// kubeconfigTemplate is the standard EKS exec-auth kubeconfig format.
// Uses the aws CLI exec plugin so kubectl auto-refreshes credentials.
// D-001 prohibits awsbnkctl from exec'ing aws CLI for its own calls; this
// is a config artifact — a different concern.
const kubeconfigTemplate = `apiVersion: v1
kind: Config
clusters:
- name: {{ .ClusterARN }}
  cluster:
    server: {{ .Endpoint }}
    certificate-authority-data: {{ .CA }}
contexts:
- name: {{ .ClusterARN }}
  context:
    cluster: {{ .ClusterARN }}
    user: {{ .ClusterARN }}
current-context: {{ .ClusterARN }}
users:
- name: {{ .ClusterARN }}
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: aws
      args: ["--region", "{{ .Region }}", "eks", "get-token", "--cluster-name", "{{ .ClusterName }}"]
      interactiveMode: IfAvailable
`

type kubeconfigData struct {
	ClusterARN  string
	Endpoint    string
	CA          string
	Region      string
	ClusterName string
}

// Phase11Kubeconfig writes the EKS exec-auth kubeconfig for the cluster.
// Path: .awsbnkctl/<cluster>/kubeconfig (always regenerated — CA/endpoint may rotate).
// Atomic write via temp file + rename. File permissions 0o600.
//
// State key written: KUBECONFIG_PATH.
// Dry-run: logs would-write, no file written.
func Phase11Kubeconfig(_ context.Context, cl *intent.Cluster, st *state.State, clients *Clients, dryRun bool) error {
	awsmw.CheckAuthOrDie(clients.Profile)
	name := cl.Metadata.Name
	kubeconfigPath := filepath.Join(cl.StateDir(), "kubeconfig")

	if dryRun {
		fmt.Fprintf(os.Stderr, "[phase 11] dry-run: would write kubeconfig to %s\n", kubeconfigPath)
		st.Set("KUBECONFIG_PATH", kubeconfigPath)
		return nil
	}

	clusterARN := st.Get("EKS_CLUSTER_ARN")
	if clusterARN == "" {
		return fmt.Errorf("phase11: EKS_CLUSTER_ARN not in state (run phase08 first)")
	}
	endpoint := st.Get("EKS_ENDPOINT")
	if endpoint == "" {
		return fmt.Errorf("phase11: EKS_ENDPOINT not in state (run phase08 first)")
	}
	ca := st.Get("EKS_CA")
	if ca == "" {
		return fmt.Errorf("phase11: EKS_CA not in state (run phase08 first)")
	}
	region := cl.Metadata.Region

	data := kubeconfigData{
		ClusterARN:  clusterARN,
		Endpoint:    endpoint,
		CA:          ca,
		Region:      region,
		ClusterName: name,
	}

	if err := writeKubeconfig(kubeconfigPath, data); err != nil {
		return fmt.Errorf("phase11: %w", err)
	}
	fmt.Fprintf(os.Stderr, "[phase 11] kubeconfig: wrote %s (exec-auth via `aws eks get-token`)\n", kubeconfigPath)

	st.Set("KUBECONFIG_PATH", kubeconfigPath)
	return st.Save()
}

// Phase11KubeconfigDown removes the kubeconfig file.
// Best-effort: absent file is silently ignored. No AWS API calls.
// CheckAuthOrDie is still required here (D-005): state.env reads/writes
// happen inside this function and SSO expiry must be caught before any
// state mutation, even when no direct AWS SDK calls are made.
func Phase11KubeconfigDown(_ context.Context, cl *intent.Cluster, st *state.State, clients *Clients) error {
	awsmw.CheckAuthOrDie(clients.Profile)
	name := cl.Metadata.Name
	fmt.Fprintf(os.Stderr, "[phase 11 down] kubeconfig: cluster=%s\n", name)

	kubeconfigPath := st.Get("KUBECONFIG_PATH")
	if kubeconfigPath == "" {
		kubeconfigPath = filepath.Join(cl.StateDir(), "kubeconfig")
	}

	err := os.Remove(kubeconfigPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("phase11 down: removing kubeconfig %s: %w", kubeconfigPath, err)
	}
	if err == nil {
		fmt.Fprintf(os.Stderr, "[phase 11 down] removed kubeconfig %s\n", kubeconfigPath)
	} else {
		fmt.Fprintf(os.Stderr, "[phase 11 down] kubeconfig %s already gone\n", kubeconfigPath)
	}
	st.Set("KUBECONFIG_PATH", "")
	return st.Save()
}

// writeKubeconfig renders the kubeconfig template and writes it atomically
// to path. Creates parent directories as needed. File permission is 0o600.
// Rendering is delegated to renderKubeconfig (kubeconfig_render.go) which is
// also used by Phase09 to produce an in-memory kubeconfig for forge.
func writeKubeconfig(path string, data kubeconfigData) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating kubeconfig dir: %w", err)
	}

	rendered, err := renderKubeconfig(data.ClusterARN, data.Endpoint, data.CA, data.ClusterName, data.Region)
	if err != nil {
		return fmt.Errorf("rendering kubeconfig: %w", err)
	}

	// Atomic write: write to .tmp then rename.
	tmpPath := path + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("opening kubeconfig tmp file: %w", err)
	}

	if _, err := f.Write(rendered); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("writing kubeconfig: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("closing kubeconfig tmp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming kubeconfig tmp → final: %w", err)
	}
	return nil
}
