package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

// Sprint 0 stub. The IBM-coupled `up`/`plan`/`apply`/`down` lifecycle
// (terraform-exec wrapper threaded with the IBM API key + post-apply
// IBM kubeconfig fetch) was retired alongside internal/ibm and
// terraform/modules/roks_cluster. The AWS-shaped lifecycle —
// terraform/modules/eks_cluster wired against
// terraform-aws-modules/eks/aws ~> 20.x — lands in Sprint 1 (cluster
// phase) and Sprint 3 (full `awsbnkctl up`). See
// docs/prd/07-EKS-CLUSTER-SRIOV.md and docs/PLAN.md § "Sprint 1" /
// "Sprint 3" for the implementation contract.
//
// The cobra command tree stays — `awsbnkctl --help` continues to
// advertise the 4-command lifecycle so users see the shape — but each
// verb errors cleanly until its sprint lands.

var (
	flagAuto         bool
	flagTFSource     string
	flagUpgradeTF    bool
	flagNoKubeconfig bool
	flagVarFiles     []string
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Interactive setup; writes the workspace config.yaml (Sprint 1)",
	Long: `awsbnkctl init walks through the AWS-shaped prompts (region, VPC,
subnets, instance types, cluster name) and writes the workspace config.

Sprint 0 stub: the AWS wizard lands in Sprint 1. See
docs/prd/07-EKS-CLUSTER-SRIOV.md for the input contract.`,
	RunE: runInit,
}

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Provision the EKS cluster and deploy BNK — terraform plan + apply (Sprint 1+3)",
	Long: `awsbnkctl up drives the EKS cluster module (Sprint 1) and the BNK
trial modules (Sprint 3). Sprint 0 stub: errors cleanly until Sprint 1
lands terraform/modules/eks_cluster/.`,
	RunE: runUp,
}

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Read-only; show what awsbnkctl up would change (Sprint 1)",
	RunE:  runPlan,
}

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply Terraform without re-prompting (Sprint 1)",
	RunE:  runApply,
}

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Destroy everything in the workspace — terraform destroy (Sprint 1+3)",
	RunE:  runDown,
}

func init() {
	initCmd.Flags().BoolVar(&flagUpgradeTF, "upgrade-tf", false, "resolve and pin the latest TF release into config.yaml")
	initCmd.Flags().StringVar(&flagTFSource, "tf-source", "", "override TF source (path or URL); pinned into config.yaml")

	upCmd.Flags().BoolVar(&flagAuto, "auto", false, "skip the confirmation prompt before apply")
	upCmd.Flags().StringVar(&flagTFSource, "tf-source", "", "override TF source for this run only")
	upCmd.Flags().BoolVar(&flagNoKubeconfig, "no-kubeconfig", false, "skip the post-apply admin kubeconfig fetch")

	applyCmd.Flags().BoolVar(&flagAuto, "auto", false, "skip the confirmation prompt")
	applyCmd.Flags().BoolVar(&flagNoKubeconfig, "no-kubeconfig", false, "skip the post-apply admin kubeconfig fetch")
	downCmd.Flags().BoolVar(&flagAuto, "auto", false, "skip the destroy confirmation")

	for _, c := range []*cobra.Command{upCmd, planCmd, applyCmd, downCmd} {
		c.Flags().StringArrayVar(&flagVarFiles, "var-file", nil, "extra TF var-file (repeatable; later files override earlier)")
	}

	rootCmd.AddCommand(initCmd, upCmd, planCmd, applyCmd, downCmd)
}

func runUp(_ *cobra.Command, _ []string) error {
	return errors.New("awsbnkctl up is not implemented yet — EKS cluster module lands in Sprint 1, full lifecycle in Sprint 3 (see docs/PLAN.md)")
}

func runPlan(_ *cobra.Command, _ []string) error {
	return errors.New("awsbnkctl plan is not implemented yet — EKS cluster module lands in Sprint 1 (see docs/PLAN.md)")
}

func runApply(_ *cobra.Command, _ []string) error {
	return errors.New("awsbnkctl apply is not implemented yet — EKS cluster module lands in Sprint 1 (see docs/PLAN.md)")
}

func runDown(_ *cobra.Command, _ []string) error {
	return errors.New("awsbnkctl down is not implemented yet — EKS cluster module lands in Sprint 1, full lifecycle in Sprint 3 (see docs/PLAN.md)")
}
