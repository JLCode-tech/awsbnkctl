package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/phases"
	"github.com/JLCode-tech/awsbnkctl/internal/intent"
	"github.com/JLCode-tech/awsbnkctl/internal/k8s"
)

var (
	flagBnkHealKubeconfig string
	flagBnkHealConfig     string
	flagBnkHealDryRun     bool
)

// metricsHealWait bounds how long heal waits for metrics.k8s.io after the
// add-on exists (the Deployment needs a node and a first scrape).
const metricsHealWait = 3 * time.Minute

// bnkHealMetrics is the metrics-server part of the heal report.
type bnkHealMetrics struct {
	// Managed is false when heal ran without -f and could not touch the EKS
	// add-on; the API check still runs.
	Managed bool `json:"managed"`
	// Created is true when the add-on was created by this run.
	Created bool `json:"created"`
	// Available is true when metrics.k8s.io/v1beta1 is served.
	Available bool   `json:"available"`
	Detail    string `json:"detail,omitempty"`
}

// bnkHealResult is the JSON document bnk heal prints with -o json.
type bnkHealResult struct {
	Schema  string               `json:"schema"`
	Multus  k8s.MultusHealResult `json:"multus"`
	Metrics bnkHealMetrics       `json:"metricsServer"`
}

var bnkHealCmd = &cobra.Command{
	Use:   "heal",
	Short: "Repair the cluster-side plumbing awsbnkctl up and bnk upgrade also fix, on any BNK generation",
	Long: `awsbnkctl bnk heal runs the idempotent cluster repairs that awsbnkctl up
and awsbnkctl bnk upgrade apply, without provisioning or upgrading anything.
It works on 2.3 and 2.4 clusters alike.

Repairs:
  multus          The Multus thin plugin writes its kubeconfig from the
                  service-account token it holds at pod start and only rewrites
                  it when started with --cleanup-config-on-exit. Without the flag
                  the token expires and every new pod on the node fails with
                  "Multus: ... error waiting for pod: Unauthorized" (TMM stays
                  Pending). heal adds the flag to the kube-multus-ds DaemonSet,
                  which rolls it with a fresh token, and rolls it again if a pod
                  still reports Unauthorized.
  metrics-server  The metrics-server EKS add-on serves metrics.k8s.io (pod and
                  node CPU/memory) for kubectl top and the Forge fleet view.
                  Needs -f so heal can reach the EKS API; with --kubeconfig only
                  the metrics API is checked, not installed.

awsbnkctl doctor --backend k8s reports the same state as "multus kubeconfig
token" and "metrics api" without changing anything.`,
	Example: `  awsbnkctl bnk heal -f clusters/lab/cluster.yaml
  awsbnkctl bnk heal --kubeconfig ~/.kube/lab --dry-run
  awsbnkctl bnk heal -f clusters/lab/cluster.yaml -o json`,
	RunE: runBnkHeal,
}

func init() {
	bnkHealCmd.Flags().StringVar(&flagBnkHealKubeconfig, "kubeconfig", "", "explicit kubeconfig path (takes precedence over --config; default: $KUBECONFIG → ~/.kube/config)")
	bnkHealCmd.Flags().StringVarP(&flagBnkHealConfig, "config", "f", "", "path to cluster.yaml; derives kubeconfig from the cluster's state.env KUBECONFIG_PATH (overridden by --kubeconfig) and enables the EKS add-on repairs")
	bnkHealCmd.Flags().BoolVar(&flagBnkHealDryRun, "dry-run", false, "report what would change without writing to the cluster")
	bnkCmd.AddCommand(bnkHealCmd)
}

func runBnkHeal(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	kubeconfigPath := flagBnkHealKubeconfig
	var cl *intent.Cluster
	if flagBnkHealConfig != "" {
		loaded, err := intent.Load(flagBnkHealConfig)
		if err != nil {
			return fmt.Errorf("bnk heal: loading --config %s: %w", flagBnkHealConfig, err)
		}
		cl = loaded
		if kubeconfigPath == "" {
			derived, err := resolveKubeconfigFromConfig(flagBnkHealConfig)
			if err != nil {
				return fmt.Errorf("bnk heal: %w", err)
			}
			kubeconfigPath = derived
		}
	}
	cs, err := k8s.BuildClientset(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("bnk heal: building kube client: %w", err)
	}

	log := os.Stderr
	res := bnkHealResult{Schema: "awsbnkctl.bnk-heal/v1"}

	mh, err := k8s.EnsureMultusTokenWatch(ctx, cs, flagBnkHealDryRun, log)
	res.Multus = mh
	if err != nil {
		return fmt.Errorf("bnk heal: multus: %w", err)
	}

	res.Metrics, err = healMetricsServer(ctx, cl, cs, flagBnkHealDryRun, log)
	if err != nil {
		return fmt.Errorf("bnk heal: metrics-server: %w", err)
	}

	if flagOutput == "json" {
		return json.NewEncoder(os.Stdout).Encode(res)
	}
	switch {
	case !mh.Installed:
		fmt.Fprintln(os.Stdout, "multus          not installed")
	case mh.Patched:
		fmt.Fprintf(os.Stdout, "multus          %s added to %s/%s; DaemonSet rolled\n", k8s.MultusWatchFlag, k8s.MultusNamespace, k8s.MultusDaemonSet)
	case mh.Restarted:
		fmt.Fprintf(os.Stdout, "multus          %s/%s rolled\n", k8s.MultusNamespace, k8s.MultusDaemonSet)
	case flagBnkHealDryRun && (!mh.WatchEnabled || mh.Unauthorized):
		fmt.Fprintf(os.Stdout, "multus          would change: %s\n", mh.Reason)
	default:
		fmt.Fprintf(os.Stdout, "multus          ok: %s\n", mh.Reason)
	}
	fmt.Fprintf(os.Stdout, "metrics-server  %s\n", res.Metrics.Detail)
	return nil
}

// healMetricsServer ensures the EKS add-on when cl is known (heal -f) and
// reports whether metrics.k8s.io is served.
func healMetricsServer(ctx context.Context, cl *intent.Cluster, cs kubernetes.Interface, dryRun bool, log *os.File) (bnkHealMetrics, error) {
	m := bnkHealMetrics{}
	if cl != nil {
		m.Managed = true
		if dryRun {
			if err := k8s.MetricsAPIAvailable(cs); err != nil {
				m.Detail = "would ensure the " + phases.MetricsServerAddonName + " EKS add-on: " + err.Error()
			} else {
				m.Available = true
				m.Detail = "ok: " + k8s.MetricsAPIGroupVersion + " served"
			}
			return m, nil
		}
		clients, err := phases.NewClients(ctx, cl.Metadata.Region, "")
		if err != nil {
			return m, err
		}
		created, err := phases.EnsureMetricsServerAddon(ctx, clients.EKS, cl.Metadata.Name, log)
		if err != nil {
			return m, err
		}
		m.Created = created
		if err := k8s.WaitForMetricsAPI(ctx, cs, metricsHealWait); err != nil {
			m.Detail = "add-on present but " + err.Error()
			return m, nil
		}
		m.Available = true
		if created {
			m.Detail = phases.MetricsServerAddonName + " EKS add-on created; " + k8s.MetricsAPIGroupVersion + " served"
		} else {
			m.Detail = "ok: " + k8s.MetricsAPIGroupVersion + " served"
		}
		return m, nil
	}
	if err := k8s.MetricsAPIAvailable(cs); err != nil {
		m.Detail = err.Error() + " — run bnk heal -f <cluster.yaml> to create the " + phases.MetricsServerAddonName + " EKS add-on"
		return m, nil
	}
	m.Available = true
	m.Detail = "ok: " + k8s.MetricsAPIGroupVersion + " served"
	return m, nil
}
