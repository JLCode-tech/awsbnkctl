package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	awsaws "github.com/JLCode-tech/awsbnkctl/internal/aws"
	"github.com/JLCode-tech/awsbnkctl/internal/config"
	"github.com/JLCode-tech/awsbnkctl/internal/tf"
)

// Sprint 1 cluster verbs (PRD 07 § "CLI surface"). Two cobra commands:
//
//	awsbnkctl up   cluster [--dry-run] [--workspace <name>]
//	awsbnkctl down cluster [--dry-run] [--workspace <name>]
//
// Both drive `terraform/modules/eks_cluster/` only (Sprint 1 deliverable);
// the full BNK lifecycle (`awsbnkctl up` without `cluster`) lands in
// Sprint 3.
//
// SPIKE DEFERRAL — CRITICAL: This sprint, `--apply` is intentionally
// NOT a supported flag. The operator-run spike (PRD 07 § "Spike
// protocol") validates the design hypothesis against live AWS before
// v0.2; until then the only execution path is `--dry-run` (terraform
// plan). Running `up cluster` without `--dry-run` returns a clear
// message pointing at the spike.

var (
	flagClusterDryRun bool
)

var upClusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Provision the EKS cluster + self-managed SR-IOV node group (Sprint 1; dry-run only)",
	Long: `awsbnkctl up cluster drives terraform/modules/eks_cluster/ — the EKS
control plane + self-managed node group on ENA-SR-IOV-capable instance
types per PRD 07 (docs/prd/07-EKS-CLUSTER-SRIOV.md).

Pass --workspace <name> (or -w) to target a specific workspace; the
verb defaults to the current_workspace pointer (or "default").

Sprint 1 supports --dry-run only (terraform plan). Live apply against
AWS is gated on the operator-run spike (PRD 07 § "Spike protocol")
validating the design hypothesis; that validation lands the v0.2 tag
and unlocks the non-dry-run path.`,
	RunE: runUpCluster,
}

var downClusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Destroy the EKS cluster module (Sprint 1; dry-run only)",
	Long: `awsbnkctl down cluster is the symmetric reverse of "up cluster" —
drives terraform/modules/eks_cluster/ to destroy state. Sprint 1
supports --dry-run only; non-dry-run lands once the spike validates
the design hypothesis (PRD 07 § "Spike protocol").`,
	RunE: runDownCluster,
}

func init() {
	upClusterCmd.Flags().BoolVar(&flagClusterDryRun, "dry-run", false, "terraform plan only — never apply against AWS")
	downClusterCmd.Flags().BoolVar(&flagClusterDryRun, "dry-run", false, "terraform plan only — never apply against AWS")

	upCmd.AddCommand(upClusterCmd)
	downCmd.AddCommand(downClusterCmd)
}

func runUpCluster(cmd *cobra.Command, _ []string) error {
	if !flagClusterDryRun {
		return errors.New("awsbnkctl up cluster requires --dry-run in Sprint 1: live apply is gated on the operator-run PRD 07 spike (see docs/prd/07-EKS-CLUSTER-SRIOV.md § \"Spike protocol\"); v0.2 unlocks the non-dry-run path")
	}
	return runClusterPlan(cmd.Context())
}

func runDownCluster(cmd *cobra.Command, _ []string) error {
	if !flagClusterDryRun {
		return errors.New("awsbnkctl down cluster requires --dry-run in Sprint 1: live destroy is gated on the operator-run PRD 07 spike (see docs/prd/07-EKS-CLUSTER-SRIOV.md § \"Spike protocol\"); v0.2 unlocks the non-dry-run path")
	}
	return runClusterPlan(cmd.Context())
}

// runClusterPlan opens a tf workspace, init+plan, and reports the
// outcome. No apply — both up cluster --dry-run and down cluster
// --dry-run land here this sprint (per the brief).
//
// The function emits a clear "AWS credentials not detected" message
// when the env / shared config is empty, rather than letting
// terraform's "no valid credential sources" surface deep inside the
// plan output. PRD 07 § "internal/aws/" cross-references this with
// the doctor pre-flight.
func runClusterPlan(ctx context.Context) error {
	cctx, err := config.New(flagWorkspace)
	if err != nil {
		return err
	}
	// Workspace optional: a fresh `awsbnkctl up cluster --dry-run` on a
	// clean host should plan against the embedded TF tree with
	// reasonable defaults sourced from env vars (AWS_REGION) + a
	// terraform.tfvars under cwd if present. Sprint 1's wiring is
	// minimal — the full init+wizard lands in Sprint 3.
	if cctx == nil {
		cctx = &config.Context{WorkspaceName: "default"}
	}

	if !awsaws.HasEnvCredentials() {
		fmt.Fprintln(os.Stderr, "warning: AWS credentials not detected (set AWS_PROFILE, AWS_ACCESS_KEY_ID, or run from an instance with an attached role); plan will fail when terraform tries to call AWS APIs")
	}

	stateDir, err := config.WorkspaceStateDir(cctx.WorkspaceName)
	if err != nil {
		return fmt.Errorf("resolving workspace state dir: %w", err)
	}

	var ws *config.Workspace
	if cctx.Workspace != nil {
		ws = cctx.Workspace
	} else {
		// Synthesise a minimal workspace pointing at the embedded TF
		// source so dry-run works without a prior `awsbnkctl init`.
		ws = &config.Workspace{TFSource: config.TFSourceCfg{Type: "embedded"}}
	}

	tfws, err := tf.Open(ctx, cctx.WorkspaceName, ws, stateDir, os.Stdout, os.Stderr)
	if err != nil {
		return err
	}

	// Ensure the per-workspace tfvars file exists. Sprint 1 punts on
	// the full AWS-shaped tfvars rendering (Sprint 2 retargets
	// internal/tf/vars.go off the IBM schema); for now we write an
	// empty placeholder so the var-file argument the Plan() call
	// inherits from the IBM lineage doesn't fail with "file does not
	// exist". The user supplies real values via TF_VAR_* env vars or a
	// terraform.tfvars.user file.
	if err := ensureEmptyTFVars(tfws.TFVarsPath()); err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr, "→ terraform init")
	if err := tfws.Init(ctx); err != nil {
		return fmt.Errorf("terraform init: %w", err)
	}

	fmt.Fprintln(os.Stderr, "→ terraform plan (dry-run; no apply)")
	hasChanges, err := tfws.Plan(ctx)
	if err != nil {
		return fmt.Errorf("terraform plan: %w", err)
	}
	if hasChanges {
		fmt.Fprintln(os.Stderr, "✓ plan complete — changes pending (run `awsbnkctl up cluster` without --dry-run once v0.2 spike validates the design)")
	} else {
		fmt.Fprintln(os.Stderr, "✓ plan complete — no changes")
	}
	return nil
}

// ensureEmptyTFVars writes an empty header tfvars file at path if it
// does not already exist. Idempotent. Sprint 1 stub; Sprint 2 replaces
// this with the full AWS-shaped renderer (currently internal/tf/vars.go
// still carries IBM-named fields).
func ensureEmptyTFVars(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	const body = "# Sprint 1 placeholder. AWS-shaped tfvars renderer lands in Sprint 2 (PRD 04).\n# Pass values via TF_VAR_<name> env vars or terraform.tfvars.user.\n"
	return os.WriteFile(path, []byte(body), 0o644)
}

// runFullLifecyclePlan drives the Sprint 3 full-lifecycle terraform
// plan: eks_cluster → cert_manager / s3_supply_chain / iam_irsa →
// flo → cne_instance → license / testing. Identical shape to
// runClusterPlan but renders the workspace's tfvars (so the
// supply-chain paths + cluster_name + region surface in the plan)
// rather than the cluster-only empty placeholder.
//
// SPIKE DEFERRAL: this only ever runs `terraform plan` — never
// `apply`. Live apply against AWS is gated on the PRD 07 operator-
// run spike; v0.2 unlocks the non-dry-run path.
func runFullLifecyclePlan(ctx context.Context) error {
	cctx, err := config.New(flagWorkspace)
	if err != nil {
		return err
	}
	if cctx == nil {
		cctx = &config.Context{WorkspaceName: "default"}
	}

	// Sprint 3 tech-writer Issue 3 (medium) — first-run UX gap.
	// Without a prior `awsbnkctl init`, the user hits terraform's
	// "no value for required variable" stack trace for vpc_id,
	// subnet_ids, far_auth_file_local_path, jwt_file_local_path.
	// That's a confusing first encounter — the resolution is `init`,
	// not "fix the terraform error". Surface a friendly message
	// before terraform boots so the user knows what to run.
	//
	// Operators who genuinely want to plan against externally-supplied
	// tfvars (CI on a fresh checkout) can pass --var-file=… or set
	// TF_VAR_* env vars; those paths bypass this gate by virtue of
	// having a workspace OR by failing terraform's check later (the
	// var-file flow needs a workspace anyway for state-dir resolution).
	if cctx.Workspace == nil && len(flagVarFiles) == 0 {
		return fmt.Errorf("workspace %q is not initialised — run `awsbnkctl init -w %s` first to capture region / VPC / subnets / FAR archive / JWT, or pass --var-file=<path> to supply values directly",
			cctx.WorkspaceName, cctx.WorkspaceName)
	}

	if !awsaws.HasEnvCredentials() {
		fmt.Fprintln(os.Stderr, "warning: AWS credentials not detected (set AWS_PROFILE, AWS_ACCESS_KEY_ID, or run from an instance with an attached role); plan will fail when terraform tries to call AWS APIs")
	}

	stateDir, err := config.WorkspaceStateDir(cctx.WorkspaceName)
	if err != nil {
		return fmt.Errorf("resolving workspace state dir: %w", err)
	}

	var ws *config.Workspace
	if cctx.Workspace != nil {
		ws = cctx.Workspace
	} else {
		ws = &config.Workspace{TFSource: config.TFSourceCfg{Type: "embedded"}}
	}

	tfws, err := tf.Open(ctx, cctx.WorkspaceName, ws, stateDir, os.Stdout, os.Stderr)
	if err != nil {
		return err
	}

	// Render the workspace's tfvars (region, vpc_id, subnet_ids,
	// cluster_name, supply-chain paths). Falls back to the
	// placeholder for a workspace with no values set — the user
	// can override via TF_VAR_* env vars or --var-file.
	tfvarsPath := tfws.TFVarsPath()
	if cctx.Workspace != nil {
		if werr := tf.WriteTFVars(tfvarsPath, cctx.Workspace, "", ""); werr != nil {
			return fmt.Errorf("rendering tfvars: %w", werr)
		}
	} else if err := ensureEmptyTFVars(tfvarsPath); err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr, "→ terraform init (full lifecycle: eks_cluster → cert_manager / s3 / iam_irsa → flo → cne_instance → license / testing)")
	if err := tfws.Init(ctx); err != nil {
		return fmt.Errorf("terraform init: %w", err)
	}

	fmt.Fprintln(os.Stderr, "→ terraform plan (dry-run; no apply)")
	hasChanges, err := tfws.Plan(ctx)
	if err != nil {
		return translatePlanError(err, cctx.WorkspaceName)
	}
	if hasChanges {
		fmt.Fprintln(os.Stderr, "✓ plan complete — full Sprint 3 graph planned; changes pending (live apply gated on PRD 07 spike)")
	} else {
		fmt.Fprintln(os.Stderr, "✓ plan complete — no changes")
	}
	return nil
}

// translatePlanError catches the most-common "missing required
// variable" terraform errors and converts them to a single-line
// `awsbnkctl`-shaped hint. The raw terraform error is preserved (still
// printed to stderr by the tf wrapper) so debugging info isn't lost;
// the returned error is the actionable summary the operator sees in
// `awsbnkctl: <err>` on the last line.
//
// Sprint 3 tech-writer Issue 3 carry-over fold.
func translatePlanError(err error, wsName string) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "No value for required variable") ||
		strings.Contains(msg, "no value for required variable") {
		return fmt.Errorf("terraform plan: required variables unset for workspace %q — run `awsbnkctl init -w %s` to capture region / VPC / subnets / FAR archive / JWT, or pass --var-file=<path> with values", wsName, wsName)
	}
	return fmt.Errorf("terraform plan: %w", err)
}
