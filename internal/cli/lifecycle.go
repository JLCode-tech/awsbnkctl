package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	awspkg "github.com/JLCode-tech/awsbnkctl/internal/aws"
	"github.com/JLCode-tech/awsbnkctl/internal/aws/phases"
	"github.com/JLCode-tech/awsbnkctl/internal/aws/state"
	"github.com/JLCode-tech/awsbnkctl/internal/config"
	"github.com/JLCode-tech/awsbnkctl/internal/forge"
	"github.com/JLCode-tech/awsbnkctl/internal/intent"
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

	// flagRegisterWithForge wires the P2 auto-handoff: after a
	// successful `awsbnkctl up`, register the resulting EKS cluster
	// with a running bnk-forge instance over MCP. No-op in dry-run.
	// Equivalent to running `awsbnkctl forge register` post-apply,
	// but bundled so operators don't have to remember the second
	// command.
	flagRegisterWithForge bool

	// flagConfig activates the new Go-SDK phased path when set.
	// When empty, up/down fall through to the existing TF path unchanged.
	// This is the dispatch gate for the post-Terraform direction
	// (docs/POST_TERRAFORM_DIRECTION.md).
	flagConfig string

	// flagYes skips the interactive "type 'destroy' to proceed" prompt
	// in `awsbnkctl down --config <file>`. Equivalent to the --yes/-y
	// flag in aws-gpu-setup/down.sh.
	flagYes bool
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
	upCmd.Flags().BoolVar(&flagRegisterWithForge, "register-with-forge", false, "after a successful apply, register the EKS cluster with bnk-forge over MCP (no-op in --dry-run)")
	upCmd.Flags().StringVar(&flagConfig, "config", "", "path to cluster.yaml; activates Go-SDK phased path (bypasses TF)")

	applyCmd.Flags().BoolVar(&flagAuto, "auto", false, "skip the confirmation prompt")
	applyCmd.Flags().BoolVar(&flagNoKubeconfig, "no-kubeconfig", false, "skip the post-apply admin kubeconfig fetch")
	downCmd.Flags().BoolVar(&flagAuto, "auto", false, "skip the destroy confirmation")
	downCmd.Flags().BoolVar(&flagLifecycleDryRun, "dry-run", false, "terraform plan -destroy only — never destroy against AWS")
	downCmd.Flags().StringVar(&flagConfig, "config", "", "path to cluster.yaml; activates Go-SDK phased path (bypasses TF)")
	downCmd.Flags().BoolVar(&flagYes, "yes", false, "skip the interactive destroy confirmation (required with --config)")
	planCmd.Flags().BoolVar(&flagLifecycleDryRun, "dry-run", true, "alias for `awsbnkctl up --dry-run` (always plans, never applies)")

	for _, c := range []*cobra.Command{upCmd, planCmd, applyCmd, downCmd} {
		c.Flags().StringArrayVar(&flagVarFiles, "var-file", nil, "extra TF var-file (repeatable; later files override earlier)")
	}

	rootCmd.AddCommand(initCmd, upCmd, planCmd, applyCmd, downCmd)
}

// resolveVarFiles normalises --var-file entries to absolute paths
// against the invocation CWD. Terraform runs with CWD = the per-phase
// state directory (~/.awsbnkctl/<workspace>/state/tf-source/), so a
// user's `--var-file=./terraform.tfvars` would otherwise resolve there
// instead of in the shell directory they typed it from.
//
// Order:
//  1. `~` / `~/...` expansion via os.UserHomeDir.
//  2. Absolute paths pass through unchanged (just cleaned).
//  3. Relative paths join against os.Getwd().
//  4. os.Stat against the resolved absolute, so a typo or wrong-CWD
//     surfaces *before* terraform runs with a message that names both
//     the user-supplied input and the resolved absolute.
//
// Idempotent on already-absolute slices — safe to call once at the
// RunE entry of every lifecycle command.
//
// Ported from roksbnkctl@28ccc59 (sprint12 var-file relative-path fix).
// This codebase has the same bug *plus* a wiring gap: `flagVarFiles`
// was previously declared but never threaded to tfws.Plan/Apply/Destroy.
// Both are fixed in this commit.
func resolveVarFiles(vfs []string) ([]string, error) {
	if len(vfs) == 0 {
		return vfs, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("resolve --var-file: %w", err)
	}
	out := make([]string, len(vfs))
	for i, vf := range vfs {
		expanded := vf
		if expanded == "~" || strings.HasPrefix(expanded, "~/") {
			if home, herr := os.UserHomeDir(); herr == nil {
				if expanded == "~" {
					expanded = home
				} else {
					expanded = filepath.Join(home, expanded[2:])
				}
			}
		}
		if filepath.IsAbs(expanded) {
			out[i] = filepath.Clean(expanded)
			continue
		}
		abs := filepath.Join(cwd, expanded)
		if _, err := os.Stat(abs); err != nil {
			return nil, fmt.Errorf("--var-file %s (resolved to %s): %w", vf, abs, err)
		}
		out[i] = abs
	}
	return out, nil
}

// runUp wires `awsbnkctl up` (no subcommand) — full-lifecycle path.
//
// When --config is provided, dispatches to the new Go-SDK phased path
// (docs/POST_TERRAFORM_DIRECTION.md). Otherwise falls through to the
// existing Terraform path (unchanged).
func runUp(cmd *cobra.Command, _ []string) error {
	// --- New Go-SDK phased path ---
	if flagConfig != "" {
		return runPhasedUp(cmd.Context(), flagConfig, flagLifecycleDryRun)
	}

	// --- Legacy Terraform path (unchanged) ---
	if !flagLifecycleDryRun {
		return errors.New("awsbnkctl up requires --dry-run in Sprint 3: live apply is gated on the operator-run PRD 07 spike (see docs/prd/07-EKS-CLUSTER-SRIOV.md § \"Spike protocol\"); v0.2 unlocks the non-dry-run path")
	}
	resolved, err := resolveVarFiles(flagVarFiles)
	if err != nil {
		return err
	}
	flagVarFiles = resolved
	if err := runFullLifecyclePlan(cmd.Context()); err != nil {
		return err
	}
	// P2: auto-register with forge over MCP after a successful apply.
	// Dry-run skips because forge.Register needs a real EKS cluster to
	// describe + generate a kubeconfig for — and there isn't one yet.
	// The flag still wires through so operators get a single command
	// once v0.2 unlocks live apply.
	if flagRegisterWithForge {
		if flagLifecycleDryRun {
			fmt.Fprintln(os.Stderr, "→ --register-with-forge: dry-run, skipping forge registration (would run `forge register` after live apply)")
			return nil
		}
		return registerWithForgePostApply(cmd.Context())
	}
	return nil
}

// registerWithForgePostApply runs the same flow as `awsbnkctl forge
// register` — used by `awsbnkctl up --register-with-forge`. Pulled out
// of runUp so the dry-run / live-apply branching stays readable, and
// so it can be unit-tested independently of the lifecycle path.
func registerWithForgePostApply(ctx context.Context) error {
	cctx, err := requireWorkspace()
	if err != nil {
		return fmt.Errorf("forge register after apply: %w", err)
	}
	wsDir, err := config.WorkspaceDir(cctx.WorkspaceName)
	if err != nil {
		return fmt.Errorf("forge register after apply: resolving workspace dir: %w", err)
	}

	clusterName := cctx.Workspace.Cluster.Name
	if clusterName == "" {
		return fmt.Errorf("forge register after apply: workspace cluster.name is empty")
	}
	region := cctx.Workspace.AWS.Region
	if region == "" {
		return fmt.Errorf("forge register after apply: workspace AWS.region is empty")
	}

	// Generate the kubeconfig in-process via the EKS presigned-URL
	// flow. Matches what `awsbnkctl forge register` does without a
	// --kubeconfig override.
	clients, err := awspkg.NewClients(ctx, awspkg.Options{
		Region:  region,
		Profile: cctx.Workspace.AWS.Profile,
	})
	if err != nil {
		return fmt.Errorf("forge register after apply: aws clients: %w", err)
	}
	ci, err := clients.DescribeCluster(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("forge register after apply: eks describe-cluster %s: %w", clusterName, err)
	}
	yaml, err := clients.KubeconfigFromCluster(ci)
	if err != nil {
		return fmt.Errorf("forge register after apply: generate kubeconfig: %w", err)
	}

	fc := forge.NewClient("")
	if !flagQuiet {
		fmt.Fprintf(os.Stderr, "→ forge MCP: %s\n", fc.URL())
	}

	regCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	res, err := forge.Register(regCtx, fc, forge.RegisterRequest{
		WorkspaceName: cctx.WorkspaceName,
		WorkspaceDir:  wsDir,
		ClusterName:   clusterName,
		Region:        region,
		Kubeconfig:    []byte(yaml),
	})
	if err != nil {
		return fmt.Errorf("forge register after apply: %w", err)
	}
	fmt.Fprintf(os.Stderr, "✓ registered with forge (project_id=%d cluster_id=%d)\n",
		res.Link.ProjectID, res.Link.ClusterID)
	return nil
}

func runPlan(cmd *cobra.Command, _ []string) error {
	// `awsbnkctl plan` is always a dry-run alias for `awsbnkctl up --dry-run`.
	resolved, err := resolveVarFiles(flagVarFiles)
	if err != nil {
		return err
	}
	flagVarFiles = resolved
	return runFullLifecyclePlan(cmd.Context())
}

func runApply(_ *cobra.Command, _ []string) error {
	return errors.New("awsbnkctl apply is gated on the operator-run PRD 07 spike (docs/prd/07-EKS-CLUSTER-SRIOV.md § \"Spike protocol\"); use `awsbnkctl up --dry-run` until v0.2 unlocks live apply")
}

func runDown(cmd *cobra.Command, _ []string) error {
	// --- New Go-SDK phased path ---
	if flagConfig != "" {
		return runPhasedDown(cmd.Context(), flagConfig, flagYes)
	}

	// --- Legacy Terraform path (unchanged) ---
	if !flagLifecycleDryRun {
		return errors.New("awsbnkctl down requires --dry-run in Sprint 3: live destroy is gated on the operator-run PRD 07 spike; v0.2 unlocks the non-dry-run path")
	}
	resolved, err := resolveVarFiles(flagVarFiles)
	if err != nil {
		return err
	}
	flagVarFiles = resolved
	return runFullLifecyclePlan(cmd.Context())
}

// runPhasedUp is the Go-SDK phased provisioning path activated by
// `awsbnkctl up --config <file>`. It reads the cluster.yaml intent,
// constructs AWS clients with the SSO sentinel middleware, then runs
// phases 00 → 06 in order.
//
// When dryRun is true the phase functions print planned actions but make
// zero AWS API mutations.
func runPhasedUp(ctx context.Context, configPath string, dryRun bool) error {
	cl, err := intent.Load(configPath)
	if err != nil {
		return fmt.Errorf("up: %w", err)
	}

	clients, err := phases.NewClients(ctx, cl.Metadata.Region, "")
	if err != nil {
		return fmt.Errorf("up: aws clients: %w", err)
	}

	stateDir := cl.StateDir()
	st, err := state.Load(stateDir)
	if err != nil {
		return fmt.Errorf("up: loading state: %w", err)
	}

	if dryRun {
		fmt.Fprintln(os.Stderr, "→ dry-run: printing plan, no AWS mutations will be made")
	}

	if err := phases.Phase00Preflight(ctx, cl, st, clients, dryRun); err != nil {
		return fmt.Errorf("up: %w", err)
	}
	if err := phases.Phase02VPC(ctx, cl, st, clients, dryRun); err != nil {
		return fmt.Errorf("up: %w", err)
	}
	if err := phases.Phase03Subnets(ctx, cl, st, clients, dryRun); err != nil {
		return fmt.Errorf("up: %w", err)
	}
	if err := phases.Phase04IGW(ctx, cl, st, clients, dryRun); err != nil {
		return fmt.Errorf("up: %w", err)
	}
	if err := phases.Phase05NAT(ctx, cl, st, clients, dryRun); err != nil {
		return fmt.Errorf("up: %w", err)
	}
	if err := phases.Phase06RouteTables(ctx, cl, st, clients, dryRun); err != nil {
		return fmt.Errorf("up: %w", err)
	}

	if dryRun {
		fmt.Fprintln(os.Stderr, "→ dry-run complete")
	} else {
		fmt.Fprintf(os.Stderr, "✓ up complete: cluster=%s state=%s\n", cl.Metadata.Name, stateDir)
	}
	return nil
}

// runPhasedDown is the Go-SDK phased destroy path activated by
// `awsbnkctl down --config <file>`. It reads the cluster.yaml intent,
// loads the IDs cache (with tag-discovery fallback), then destroys
// resources in reverse phase order.
//
// When yes is false the operator is prompted to type 'destroy' to proceed.
func runPhasedDown(ctx context.Context, configPath string, yes bool) error {
	cl, err := intent.Load(configPath)
	if err != nil {
		return fmt.Errorf("down: %w", err)
	}

	clients, err := phases.NewClients(ctx, cl.Metadata.Region, "")
	if err != nil {
		return fmt.Errorf("down: aws clients: %w", err)
	}

	stateDir := cl.StateDir()
	st, err := state.Load(stateDir)
	if err != nil {
		return fmt.Errorf("down: loading state: %w", err)
	}

	if !yes {
		fmt.Fprintf(os.Stderr, "About to DESTROY cluster %q in %s.\n", cl.Metadata.Name, cl.Metadata.Region)
		fmt.Fprintln(os.Stderr, "Type 'destroy' to proceed:")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() && scanner.Text() != "destroy" {
			fmt.Fprintln(os.Stderr, "Aborted.")
			return nil
		}
	}

	// Reverse phase order: 06 → 05 → 04 → 03 → 02.
	if err := phases.Phase06RouteTablesDown(ctx, cl, st, clients); err != nil {
		return fmt.Errorf("down: %w", err)
	}
	if err := phases.Phase05NATDown(ctx, cl, st, clients); err != nil {
		return fmt.Errorf("down: %w", err)
	}
	if err := phases.Phase04IGWDown(ctx, cl, st, clients); err != nil {
		return fmt.Errorf("down: %w", err)
	}
	if err := phases.Phase03SubnetsDown(ctx, cl, st, clients); err != nil {
		return fmt.Errorf("down: %w", err)
	}
	if err := phases.Phase02VPCDown(ctx, cl, st, clients); err != nil {
		return fmt.Errorf("down: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✓ down complete: cluster=%s\n", cl.Metadata.Name)
	return nil
}
