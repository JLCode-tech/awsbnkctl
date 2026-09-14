package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/JLCode-tech/awsbnkctl/internal/k8s"
)

var (
	flagBnkHealKubeconfig string
	flagBnkHealConfig     string
	flagBnkHealDryRun     bool
)

// bnkHealResult is the JSON document bnk heal prints with -o json.
type bnkHealResult struct {
	Schema string               `json:"schema"`
	Multus k8s.MultusHealResult `json:"multus"`
}

var bnkHealCmd = &cobra.Command{
	Use:   "heal",
	Short: "Repair the cluster-side plumbing awsbnkctl up and bnk upgrade also fix, on any BNK generation",
	Long: `awsbnkctl bnk heal runs the idempotent cluster repairs that awsbnkctl up
(phase 12) and awsbnkctl bnk upgrade (step 0) apply, without provisioning or
upgrading anything. It works on 2.3 and 2.4 clusters alike.

Repairs:
  multus   The Multus thin plugin writes its kubeconfig from the service-account
           token it holds at pod start and only rewrites it when started with
           --cleanup-config-on-exit. Without the flag the token expires and every
           new pod on the node fails with "Multus: ... error waiting for pod:
           Unauthorized" (TMM stays Pending). heal adds the flag to the
           kube-multus-ds DaemonSet, which rolls it with a fresh token, and rolls
           it again if a pod still reports Unauthorized.

awsbnkctl doctor --backend k8s reports the same state as "multus kubeconfig
token" without changing anything.`,
	Example: `  awsbnkctl bnk heal -f clusters/lab/cluster.yaml
  awsbnkctl bnk heal --kubeconfig ~/.kube/lab --dry-run
  awsbnkctl bnk heal -f clusters/lab/cluster.yaml -o json`,
	RunE: runBnkHeal,
}

func init() {
	bnkHealCmd.Flags().StringVar(&flagBnkHealKubeconfig, "kubeconfig", "", "explicit kubeconfig path (takes precedence over --config; default: $KUBECONFIG → ~/.kube/config)")
	bnkHealCmd.Flags().StringVarP(&flagBnkHealConfig, "config", "f", "", "path to cluster.yaml; derives kubeconfig from the cluster's state.env KUBECONFIG_PATH (overridden by --kubeconfig)")
	bnkHealCmd.Flags().BoolVar(&flagBnkHealDryRun, "dry-run", false, "report what would change without writing to the cluster")
	bnkCmd.AddCommand(bnkHealCmd)
}

func runBnkHeal(cmd *cobra.Command, _ []string) error {
	kubeconfigPath := flagBnkHealKubeconfig
	if kubeconfigPath == "" && flagBnkHealConfig != "" {
		derived, err := resolveKubeconfigFromConfig(flagBnkHealConfig)
		if err != nil {
			return fmt.Errorf("bnk heal: %w", err)
		}
		kubeconfigPath = derived
	}
	cs, err := k8s.BuildClientset(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("bnk heal: building kube client: %w", err)
	}

	log := os.Stderr
	res := bnkHealResult{Schema: "awsbnkctl.bnk-heal/v1"}
	mh, err := k8s.EnsureMultusTokenWatch(cmd.Context(), cs, flagBnkHealDryRun, log)
	res.Multus = mh
	if err != nil {
		return fmt.Errorf("bnk heal: multus: %w", err)
	}

	if flagOutput == "json" {
		return json.NewEncoder(os.Stdout).Encode(res)
	}
	switch {
	case !mh.Installed:
		fmt.Fprintln(os.Stdout, "multus   not installed")
	case mh.Patched:
		fmt.Fprintf(os.Stdout, "multus   %s added to %s/%s; DaemonSet rolled\n", k8s.MultusWatchFlag, k8s.MultusNamespace, k8s.MultusDaemonSet)
	case mh.Restarted:
		fmt.Fprintf(os.Stdout, "multus   %s/%s rolled\n", k8s.MultusNamespace, k8s.MultusDaemonSet)
	case flagBnkHealDryRun && (!mh.WatchEnabled || mh.Unauthorized):
		fmt.Fprintf(os.Stdout, "multus   would change: %s\n", mh.Reason)
	default:
		fmt.Fprintf(os.Stdout, "multus   ok: %s\n", mh.Reason)
	}
	return nil
}
