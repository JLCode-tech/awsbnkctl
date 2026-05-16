package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

// Sprint 3 retarget. The Sprint 0 lifecycle stubs (`up` / `plan` /
// `apply` / `down`) now drive the full end-to-end terraform graph
// instead of returning a "not implemented" error. `awsbnkctl up
// --dry-run` runs `terraform plan` against the full root module
// graph (eks_cluster → cert_manager / s3_supply_chain / iam_irsa →
// flo → cne_instance → license / testing); live apply is still
// gated on the operator-run spike per PRD 07 § "Spike protocol".
//
// `awsbnkctl up cluster --dry-run` continues to plan just the
// cluster phase for parity with the Sprint 1 surface.

var (
	flagAuto         bool
	flagTFSource     string
	flagUpgradeTF    bool
	flagNoKubeconfig bool
	flagVarFiles     []string

	// flagLifecycleDryRun gates `awsbnkctl up` (no subcommand) +
	// `awsbnkctl down` (no subcommand). Distinct from the
	// cluster-phase flagClusterDryRun in cluster.go so the
	// full-graph and cluster-only paths can move at different
	// cadences as the spike unlocks the live-apply path.
	flagLifecycleDryRun bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Interactive AWS setup; collects region + VPC + subnets + FAR archive + JWT, writes the workspace config (PRD 08).",
	Long: `awsbnkctl init walks through the AWS-shaped prompts (region, VPC, subnets,
cluster name, FAR archive path, subscription JWT path, FLO namespace) and writes
the workspace config.yaml under ~/.awsbnkctl/<workspace>/. The supply-chain
artefacts are uploaded to S3 by 'awsbnkctl up' via aws_s3_object resources, not
by init directly — see PRD 08 § "Open questions" for the rationale.

Use --dry-run to walk the wizard offline (no AWS API calls; useful for
populating a workspace for terraform plan inspection ahead of a real apply).`,
	RunE: runInit,
}

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Provision the EKS cluster + cert-manager + IRSA + FLO + CNEInstance + license (full lifecycle, Sprint 3 dry-run only)",
	Long: `awsbnkctl up drives the full Sprint 3 end-to-end terraform graph:

  eks_cluster ──► cert_manager
              └─► s3_supply_chain + iam_irsa
                    └─► flo
                           └─► cne_instance
                                  └─► license
                                  └─► testing

Sprint 3 supports --dry-run only (terraform plan against the full root
module). Live apply against AWS is gated on the operator-run spike per
PRD 07 § "Spike protocol"; the v0.2 tag unlocks the non-dry-run path.

Subcommand 'awsbnkctl up cluster --dry-run' plans only the cluster
phase per PRD 06 — useful for fast iteration during the spike.`,
	RunE: runUp,
}

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Read-only; show what awsbnkctl up would change (Sprint 3 dry-run alias)",
	RunE:  runPlan,
}

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply Terraform without re-prompting (gated on PRD 07 spike)",
	RunE:  runApply,
}

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Destroy everything in the workspace — terraform destroy (Sprint 3 dry-run only)",
	RunE:  runDown,
}

func init() {
	initCmd.Flags().BoolVar(&flagUpgradeTF, "upgrade-tf", false, "resolve and pin the latest TF release into config.yaml")
	initCmd.Flags().StringVar(&flagTFSource, "tf-source", "", "override TF source (path or URL); pinned into config.yaml")

	upCmd.Flags().BoolVar(&flagAuto, "auto", false, "skip the confirmation prompt before apply")
	upCmd.Flags().StringVar(&flagTFSource, "tf-source", "", "override TF source for this run only")
	upCmd.Flags().BoolVar(&flagNoKubeconfig, "no-kubeconfig", false, "skip the post-apply admin kubeconfig fetch")
	upCmd.Flags().BoolVar(&flagLifecycleDryRun, "dry-run", false, "terraform plan only against the full Sprint 3 graph — never apply against AWS")

	applyCmd.Flags().BoolVar(&flagAuto, "auto", false, "skip the confirmation prompt")
	applyCmd.Flags().BoolVar(&flagNoKubeconfig, "no-kubeconfig", false, "skip the post-apply admin kubeconfig fetch")
	downCmd.Flags().BoolVar(&flagAuto, "auto", false, "skip the destroy confirmation")
	downCmd.Flags().BoolVar(&flagLifecycleDryRun, "dry-run", false, "terraform plan -destroy only — never destroy against AWS")
	planCmd.Flags().BoolVar(&flagLifecycleDryRun, "dry-run", true, "alias for `awsbnkctl up --dry-run` (always plans, never applies)")

	for _, c := range []*cobra.Command{upCmd, planCmd, applyCmd, downCmd} {
		c.Flags().StringArrayVar(&flagVarFiles, "var-file", nil, "extra TF var-file (repeatable; later files override earlier)")
	}

	rootCmd.AddCommand(initCmd, upCmd, planCmd, applyCmd, downCmd)
}

// runUp wires `awsbnkctl up` (no subcommand) — full-lifecycle path.
// Sprint 3 supports --dry-run only; live apply is gated on the
// operator-run spike per PRD 07 § "Spike protocol".
func runUp(cmd *cobra.Command, _ []string) error {
	if !flagLifecycleDryRun {
		return errors.New("awsbnkctl up requires --dry-run in Sprint 3: live apply is gated on the operator-run PRD 07 spike (see docs/prd/07-EKS-CLUSTER-SRIOV.md § \"Spike protocol\"). v0.2 unlocks the non-dry-run path.")
	}
	return runFullLifecyclePlan(cmd.Context())
}

func runPlan(cmd *cobra.Command, _ []string) error {
	// `awsbnkctl plan` is always a dry-run alias for `awsbnkctl up --dry-run`.
	return runFullLifecyclePlan(cmd.Context())
}

func runApply(_ *cobra.Command, _ []string) error {
	return errors.New("awsbnkctl apply is gated on the operator-run PRD 07 spike (docs/prd/07-EKS-CLUSTER-SRIOV.md § \"Spike protocol\"); use `awsbnkctl up --dry-run` until v0.2 unlocks live apply")
}

func runDown(cmd *cobra.Command, _ []string) error {
	if !flagLifecycleDryRun {
		return errors.New("awsbnkctl down requires --dry-run in Sprint 3: live destroy is gated on the operator-run PRD 07 spike. v0.2 unlocks the non-dry-run path.")
	}
	return runFullLifecyclePlan(cmd.Context())
}
