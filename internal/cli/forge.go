package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	awspkg "github.com/JLCode-tech/awsbnkctl/internal/aws"
	"github.com/JLCode-tech/awsbnkctl/internal/config"
	"github.com/JLCode-tech/awsbnkctl/internal/forge"
)

var (
	flagForgeMCPURL    string
	flagForgeProject   string
	flagForgeScan      bool
	flagForgePurge     bool
	flagForgeKubeconf  string
	flagForgeClusterNm string
)

var forgeCmd = &cobra.Command{
	Use:   "forge",
	Short: "Register the current workspace with a BNK-Forge instance over MCP",
	Long: `forge wires this workspace into a running BNK-Forge instance so
forge can manage and observe the AWS infra + EKS + BNK deployment
awsbnkctl provisioned.

Talks to forge's MCP server (Streamable HTTP / JSON-RPC) at
http://localhost:8081/mcp/ by default. Override with --forge-mcp-url or
the AWSBNKCTL_FORGE_MCP_URL environment variable.`,
}

var forgeRegisterCmd = &cobra.Command{
	Use:   "register",
	Short: "Register the workspace's EKS cluster with forge (idempotent)",
	Long: `Creates a forge project (awsbnkctl-<workspace>) and registers
the workspace's EKS cluster under it. Idempotent — re-running on an
already-registered workspace reuses the existing project/cluster.

By default, awsbnkctl uses its in-process EKS auth flow (presigned
sts:GetCallerIdentity URL) to generate a kubeconfig and hand it to forge.
Pass --kubeconfig <path> to upload a pre-existing kubeconfig file instead.

Use --scan to also trigger forge's scan_cluster + bnk_health after
registration as a smoke check.`,
	RunE: runForgeRegister,
}

var forgeStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show this workspace's forge registration state",
	RunE:  runForgeStatus,
}

var forgeUnregisterCmd = &cobra.Command{
	Use:   "unregister",
	Short: "Remove this workspace's forge registration",
	Long: `Deletes the cluster registration from forge and removes the
local forge_link.json. Pass --purge to also delete the forge project
(use with caution — irreversible from awsbnkctl's side).`,
	RunE: runForgeUnregister,
}

func init() {
	forgeCmd.PersistentFlags().StringVar(&flagForgeMCPURL, "forge-mcp-url", "",
		"forge MCP endpoint (default $AWSBNKCTL_FORGE_MCP_URL, fallback "+forge.DefaultMCPURL+")")

	forgeRegisterCmd.Flags().StringVar(&flagForgeProject, "project-name", "",
		"forge project name (default awsbnkctl-<workspace>)")
	forgeRegisterCmd.Flags().StringVar(&flagForgeClusterNm, "cluster-name", "",
		"EKS cluster name to register (default: workspace cluster.name)")
	forgeRegisterCmd.Flags().StringVar(&flagForgeKubeconf, "kubeconfig", "",
		"path to a pre-existing kubeconfig (default: generate in-process via EKS auth)")
	forgeRegisterCmd.Flags().BoolVar(&flagForgeScan, "scan", false,
		"after register, call scan_cluster + bnk_health as a smoke check")

	forgeUnregisterCmd.Flags().BoolVar(&flagForgePurge, "purge", false,
		"also delete the forge project (default: cluster only; project preserved)")

	forgeCmd.AddCommand(forgeRegisterCmd, forgeStatusCmd, forgeUnregisterCmd)
	rootCmd.AddCommand(forgeCmd)
}

func runForgeRegister(cmd *cobra.Command, _ []string) error {
	cctx, err := requireWorkspace()
	if err != nil {
		return err
	}
	wsDir, err := config.WorkspaceDir(cctx.WorkspaceName)
	if err != nil {
		return fmt.Errorf("resolving workspace dir: %w", err)
	}

	clusterName := flagForgeClusterNm
	if clusterName == "" {
		clusterName = cctx.Workspace.Cluster.Name
	}
	if clusterName == "" {
		return errors.New("no cluster name — set cluster.name in the workspace or pass --cluster-name")
	}

	region := cctx.Workspace.AWS.Region
	if region == "" {
		return errors.New("workspace AWS.region is empty — run `awsbnkctl init` first")
	}

	// 1) build kubeconfig: either read --kubeconfig <path> or generate
	// in-process via the EKS presigned-URL flow (PRD 07).
	kubeconfigYAML, err := buildKubeconfig(cmd.Context(), cctx, clusterName, region)
	if err != nil {
		return err
	}

	// 2) talk to forge over MCP
	fc := forge.NewClient(flagForgeMCPURL)
	if !flagQuiet {
		fmt.Fprintf(os.Stderr, "→ forge MCP: %s\n", fc.URL())
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
	defer cancel()

	res, err := forge.Register(ctx, fc, forge.RegisterRequest{
		WorkspaceName:    cctx.WorkspaceName,
		WorkspaceDir:     wsDir,
		ProjectName:      flagForgeProject,
		ClusterName:      clusterName,
		Region:           region,
		Kubeconfig:       kubeconfigYAML,
		PostRegisterScan: flagForgeScan,
	})
	if err != nil {
		return err
	}

	// 3) report
	fmt.Printf("✓ registered with forge\n")
	fmt.Printf("  project:   %s (id=%d)\n", res.Link.ProjectName, res.Link.ProjectID)
	fmt.Printf("  cluster:   %s (id=%d)\n", res.Link.ClusterName, res.Link.ClusterID)
	fmt.Printf("  link:      %s\n", forge.LinkPath(wsDir))
	fmt.Printf("  mcp:       %s\n", res.ForgeURL)
	if flagForgeScan {
		fmt.Printf("  scan:      %s\n", oneLine(res.ScanOutput))
		fmt.Printf("  health:    %s\n", oneLine(res.HealthCheck))
	}
	return nil
}

func runForgeStatus(cmd *cobra.Command, _ []string) error {
	cctx, err := requireWorkspace()
	if err != nil {
		return err
	}
	wsDir, err := config.WorkspaceDir(cctx.WorkspaceName)
	if err != nil {
		return fmt.Errorf("resolving workspace dir: %w", err)
	}

	fc := forge.NewClient(flagForgeMCPURL)
	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	st, err := forge.Status(ctx, fc, wsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "workspace %q has no forge link — run `awsbnkctl forge register`\n", cctx.WorkspaceName)
			return nil
		}
		return err
	}

	fmt.Printf("workspace:    %s\n", st.Link.Workspace)
	fmt.Printf("project:      %s (id=%d)\n", st.Link.ProjectName, st.Link.ProjectID)
	fmt.Printf("cluster:      %s (id=%d)\n", st.Link.ClusterName, st.Link.ClusterID)
	fmt.Printf("registered:   %s\n", st.Link.RegisteredAt.Format(time.RFC3339))
	fmt.Printf("forge mcp:    %s\n", st.Link.ForgeMCPURL)
	fmt.Printf("reachable:    %v\n", st.Reachable)
	if st.ForgeVersion != "" {
		fmt.Printf("forge version: %s\n", oneLine(st.ForgeVersion))
	}
	if st.Err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", st.Err)
	}
	return nil
}

func runForgeUnregister(cmd *cobra.Command, _ []string) error {
	cctx, err := requireWorkspace()
	if err != nil {
		return err
	}
	wsDir, err := config.WorkspaceDir(cctx.WorkspaceName)
	if err != nil {
		return fmt.Errorf("resolving workspace dir: %w", err)
	}

	fc := forge.NewClient(flagForgeMCPURL)
	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	if err := forge.Unregister(ctx, fc, wsDir, flagForgePurge); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "workspace %q has no forge link — nothing to do\n", cctx.WorkspaceName)
			return nil
		}
		return err
	}
	fmt.Printf("✓ workspace unregistered from forge%s\n",
		map[bool]string{true: " (project purged)", false: ""}[flagForgePurge])
	return nil
}

// buildKubeconfig returns the kubeconfig bytes for the workspace's EKS
// cluster — either from --kubeconfig <path> or generated in-process via
// the EKS presigned-URL auth flow.
func buildKubeconfig(ctx context.Context, cctx *config.Context, clusterName, region string) ([]byte, error) {
	if flagForgeKubeconf != "" {
		b, err := os.ReadFile(flagForgeKubeconf)
		if err != nil {
			return nil, fmt.Errorf("read --kubeconfig %s: %w", flagForgeKubeconf, err)
		}
		return b, nil
	}

	clients, err := awspkg.NewClients(ctx, awspkg.Options{
		Region:  region,
		Profile: cctx.Workspace.AWS.Profile,
	})
	if err != nil {
		return nil, fmt.Errorf("aws clients: %w", err)
	}

	ci, err := clients.DescribeCluster(ctx, clusterName)
	if err != nil {
		return nil, fmt.Errorf("eks describe-cluster %s: %w", clusterName, err)
	}

	yaml, err := clients.KubeconfigFromCluster(ci)
	if err != nil {
		return nil, fmt.Errorf("generate kubeconfig: %w", err)
	}
	return []byte(yaml), nil
}

// oneLine collapses multi-line tool output to a single line for
// status-line display. Long JSON payloads get truncated.
func oneLine(s string) string {
	const max = 160
	out := make([]byte, 0, len(s))
	for _, r := range s {
		if r == '\n' || r == '\r' {
			out = append(out, ' ')
		} else {
			out = append(out, byte(r))
		}
	}
	if len(out) > max {
		return string(out[:max]) + "…"
	}
	return string(out)
}
