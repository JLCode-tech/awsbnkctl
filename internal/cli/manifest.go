package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/JLCode-tech/awsbnkctl/internal/config"
	"github.com/JLCode-tech/awsbnkctl/internal/manifest"
)

type manifestProbeFlags struct {
	farPath string
	all     bool
	out     string
}

var probeFlags manifestProbeFlags

var manifestCmd = &cobra.Command{
	Use:   "manifest",
	Short: "Inspect and probe F5 BNK release manifests and Bill of Materials (BOM)",
	Long: `Inspect and probe F5 BNK release manifests and Bill of Materials.

Manifests are published under oci://repo.f5.com/release/f5-bigip-k8s-manifest.
The probe command pulls and parses the manifest before deployment to verify version
pinning and constituent chart/image digests without requiring AWS infrastructure.`,
}

var manifestProbeCmd = &cobra.Command{
	Use:   "probe [manifest-version]",
	Short: "Pull and inspect a BNK release manifest from repo.f5.com",
	Long: `Pull and inspect a BNK release manifest from repo.f5.com.

Requires FAR credentials (cne_pull_64.json or FAR tgz archive). If --far is not
supplied, probes look in the current workspace or current working directory.

Examples:
  awsbnkctl manifest probe
  awsbnkctl manifest probe 2.3.0-3.2598.3-0.0.170 --all
  awsbnkctl manifest probe 2.3.0-3.2598.3-0.0.170 --far /path/to/cne_pull_64.json`,
	Args: cobra.MaximumNArgs(1),
	RunE: runManifestProbe,
}

func init() {
	manifestProbeCmd.Flags().StringVar(&probeFlags.farPath, "far", "", "Path to FAR credentials (cne_pull_64.json or .tgz archive)")
	manifestProbeCmd.Flags().BoolVar(&probeFlags.all, "all", false, "Print full Bill of Materials (all charts and docker images)")
	manifestProbeCmd.Flags().StringVar(&probeFlags.out, "out", "", "Directory to keep extracted manifest artifacts (defaults to ephemeral temp dir)")

	manifestCmd.AddCommand(manifestProbeCmd)
	rootCmd.AddCommand(manifestCmd)
}

func runManifestProbe(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	out := cmd.OutOrStdout()

	manifestVersion := manifest.DefaultManifestVersion
	if len(args) > 0 && args[0] != "" {
		manifestVersion = args[0]
	}

	farPath := probeFlags.farPath
	if farPath == "" {
		// Look in current dir, then workspace dir
		candidates := []string{
			"cne_pull_64.json",
			"far.tgz",
			"far.tar.gz",
		}
		if wsDir, err := config.WorkspaceDir(resolvedWorkspaceName()); err == nil {
			candidates = append(candidates,
				filepath.Join(wsDir, "cne_pull_64.json"),
				filepath.Join(wsDir, "far.tgz"),
			)
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				farPath = c
				break
			}
		}
	}

	if farPath == "" {
		return fmt.Errorf("FAR credentials not found — specify via --far <path/to/cne_pull_64.json>")
	}

	username, password, err := manifest.ExtractFARAuth(farPath)
	if err != nil {
		return fmt.Errorf("extract FAR credentials: %w", err)
	}

	cacheDir := probeFlags.out
	if cacheDir == "" {
		tmp, err := os.MkdirTemp("", "awsbnkctl-manifest-probe-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmp)
		cacheDir = tmp
	}

	fmt.Fprintf(out, "Probing %s/%s\n", manifest.ReleaseManifestRepo, manifest.ReleaseManifestChart)
	fmt.Fprintf(out, "  version:  %s\n", manifestVersion)
	fmt.Fprintf(out, "  FAR key:  %s\n\n", farPath)

	m, err := manifest.PullReleaseManifest(ctx, username, password, manifestVersion, cacheDir)
	if err != nil {
		return fmt.Errorf("pull release-manifest %s: %w", manifestVersion, err)
	}

	m.SinkSummary(out)
	if probeFlags.all {
		manifest.PrintFullBOM(out, m)
	}
	if probeFlags.out != "" {
		fmt.Fprintf(out, "\n  saved in: %s\n", cacheDir)
	}

	if m.Version != manifestVersion {
		return fmt.Errorf("release-manifest mismatch: requested %q but manifest declares %q", manifestVersion, m.Version)
	}

	fmt.Fprintf(out, "\nOK: BOM version matches requested release manifest.\n")
	return nil
}
