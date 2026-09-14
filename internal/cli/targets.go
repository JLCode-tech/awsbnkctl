package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/JLCode-tech/awsbnkctl/internal/config"
	"github.com/JLCode-tech/awsbnkctl/internal/forge"
	"github.com/JLCode-tech/awsbnkctl/internal/k8s/bnkscan"
	"github.com/JLCode-tech/awsbnkctl/internal/remote"
)

// Local flag values for `targets add`. Reset every invocation; cobra's
// flag binding writes into these vars when --host etc. is parsed.
var (
	flagTargetHost      string
	flagTargetUser      string
	flagTargetPort      int
	flagTargetKeyPath   string
	flagTargetKeySource string
)

var targetsCmd = &cobra.Command{
	Use:   "targets",
	Short: "Manage SSH targets used by --on; discover MCP endpoints for Forge",
	Long: `Targets are named SSH endpoints stored under the workspace's
` + "`targets:`" + ` block. They become reachable via the persistent --on flag
on commands like ` + "`awsbnkctl exec`" + `, ` + "`awsbnkctl shell`" + `, ` + "`awsbnkctl kubectl`" + `, etc.

A jumphost target is auto-populated after a successful ` + "`awsbnkctl up`" + `
when the upstream HCL provisions one (testing_tgw_jumphost outputs).`,
}

var targetsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all targets in the current workspace",
	RunE:  runTargetsList,
}

var targetsShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show detail for one target",
	Args:  cobra.ExactArgs(1),
	RunE:  runTargetsShow,
}

var targetsAddCmd = &cobra.Command{
	Use:   "add <name> --host H --user U [--port P] [--key-path P | --key-source S]",
	Short: "Add or update a target",
	Args:  cobra.ExactArgs(1),
	RunE:  runTargetsAdd,
}

var targetsRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a target",
	Args:  cobra.ExactArgs(1),
	RunE:  runTargetsRemove,
}

func init() {
	targetsAddCmd.Flags().StringVar(&flagTargetHost, "host", "", "host or IP")
	targetsAddCmd.Flags().StringVar(&flagTargetUser, "user", "", "remote user")
	targetsAddCmd.Flags().IntVar(&flagTargetPort, "port", 0, "ssh port (default 22)")
	targetsAddCmd.Flags().StringVar(&flagTargetKeyPath, "key-path", "", "path to a PEM private key")
	targetsAddCmd.Flags().StringVar(&flagTargetKeySource, "key-source", "", `key source — "agent" or "tf-output:<name>"`)
	_ = targetsAddCmd.MarkFlagRequired("host")
	_ = targetsAddCmd.MarkFlagRequired("user")

	targetsCmd.AddCommand(targetsListCmd, targetsShowCmd, targetsAddCmd, targetsRemoveCmd)
	rootCmd.AddCommand(targetsCmd)
}

func runTargetsList(_ *cobra.Command, _ []string) error {
	cctx, err := requireWorkspace()
	if err != nil {
		return err
	}
	ts, err := remote.ListTargets(cctx.WorkspaceName)
	if err != nil {
		return err
	}
	if len(ts) == 0 {
		fmt.Fprintf(os.Stderr, "no targets in workspace %q (add one with `awsbnkctl targets add`)\n", cctx.WorkspaceName)
		return nil
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tHOST\tUSER\tKEY")
	for _, t := range ts {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", t.Name, hostPort(t), t.User, t.KeySourceDescription())
	}
	return tw.Flush()
}

func runTargetsShow(_ *cobra.Command, args []string) error {
	cctx, err := requireWorkspace()
	if err != nil {
		return err
	}
	t, err := remote.LoadTarget(cctx.WorkspaceName, args[0])
	if err != nil {
		return err
	}
	fmt.Printf("name:        %s\n", t.Name)
	fmt.Printf("host:        %s\n", t.Host)
	fmt.Printf("port:        %d\n", t.Port)
	fmt.Printf("user:        %s\n", t.User)
	if t.KeyPath != "" {
		fmt.Printf("key_path:    %s\n", t.KeyPath)
	}
	if t.KeySource != "" {
		fmt.Printf("key_source:  %s\n", t.KeySource)
	}
	return nil
}

func runTargetsAdd(_ *cobra.Command, args []string) error {
	cctx, err := requireWorkspace()
	if err != nil {
		return err
	}
	if flagTargetKeyPath == "" && flagTargetKeySource == "" {
		return errors.New("one of --key-path or --key-source is required")
	}
	cfg := config.TargetCfg{
		Host:      flagTargetHost,
		Port:      flagTargetPort,
		User:      flagTargetUser,
		KeyPath:   flagTargetKeyPath,
		KeySource: flagTargetKeySource,
	}
	if err := remote.SetTarget(cctx.WorkspaceName, args[0], cfg); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "✓ wrote target %q to workspace %q\n", args[0], cctx.WorkspaceName)
	return nil
}

func runTargetsRemove(_ *cobra.Command, args []string) error {
	cctx, err := requireWorkspace()
	if err != nil {
		return err
	}
	if err := remote.RemoveTarget(cctx.WorkspaceName, args[0]); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "✓ removed target %q\n", args[0])
	return nil
}

// requireWorkspace loads the workspace context and errors if the
// workspace hasn't been initialised yet — every targets sub-command
// needs a real config.yaml to read/write.
func requireWorkspace() (*config.Context, error) {
	cctx, err := config.New(flagWorkspace)
	if err != nil {
		return nil, err
	}
	if cctx.Workspace == nil {
		return nil, fmt.Errorf("workspace %q is not initialised; run `awsbnkctl init` first", cctx.WorkspaceName)
	}
	return cctx, nil
}

// hostPort renders Host or Host:Port for the targets list.
func hostPort(t *remote.Target) string {
	if t.Port == 0 || t.Port == 22 {
		return t.Host
	}
	return fmt.Sprintf("%s:%d", t.Host, t.Port)
}

// ── targets scan: zero-config MCP endpoint discovery ─────────────────

var (
	flagTargetsScanConfig   string
	flagTargetsScanRegister bool
)

var targetsScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Discover the MCP endpoints a BNK cluster exposes and register them as Forge targets",
	Long: `targets scan indexes the cluster (the same bnkscan pass as forge scan and
status), lists every HTTPRoute that fronts an MCP tool server through a BNK
Gateway with its VIP URLs, auth, persistence profile and tools, and registers
each one in Forge's Target Catalog. Registration is idempotent: a target is
reused by name (mcp-<namespace>-<route>).

Registration needs the cluster's forge_link.json (awsbnkctl forge register).
Without it the endpoints are listed and a hint is printed; --register=false
skips Forge entirely.

Kubeconfig: --kubeconfig, else the cluster.yaml state (-f), else the kubectl
default. The unified flags (--probe, --bearer-env, --forge-rest-url, ...) are
the ones forge scan takes.`,
	Args: cobra.NoArgs,
	RunE: runTargetsScan,
}

func init() {
	f := targetsScanCmd.Flags()
	f.StringVarP(&flagTargetsScanConfig, "config", "f", "", "path to cluster.yaml (kubeconfig, forge link and Forge URL come from its state)")
	f.BoolVar(&flagTargetsScanRegister, "register", true, "register the discovered endpoints in Forge's Target Catalog")
	addScanFlags(f)
	targetsCmd.AddCommand(targetsScanCmd)
}

// targetsScanOutput is the -o json document.
type targetsScanOutput struct {
	Cluster   string                `json:"cluster,omitempty"`
	Endpoints []bnkscan.MCPEndpoint `json:"endpoints"`
	Targets   []forgeScanTarget     `json:"targets,omitempty"`
	Probes    map[string]string     `json:"probeErrors,omitempty"`
	Hint      string                `json:"hint,omitempty"`
}

func runTargetsScan(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, flagForgeScanTimeout)
	defer cancel()

	cl, idx, probes, err := discoverMCP(ctx, flagTargetsScanConfig, flagForgeScanProbe)
	if err != nil {
		return fmt.Errorf("targets scan: %w", err)
	}
	out := targetsScanOutput{Endpoints: idx.MCP, Probes: probes}
	if cl != nil {
		out.Cluster = cl.Metadata.Name
	}
	if flagTargetsScanRegister && len(idx.MCP) > 0 {
		if link := forgeScanLink(cl); link != nil {
			out.Targets = registerDiscoveredTargets(ctx, cl, link, "", forge.RestCreds{}, idx.MCP)
		} else {
			out.Hint = "cluster is not registered with Forge (no forge_link.json); run `awsbnkctl forge register` to enable target registration"
		}
	}

	if flagOutput == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	w := cmd.OutOrStdout()
	if out.Cluster != "" {
		fmt.Fprintf(w, "cluster: %s\n", out.Cluster)
	}
	writeMCPEndpointTable(w, idx)
	for name, perr := range out.Probes {
		fmt.Fprintf(w, "probe %s: %s\n", name, perr)
	}
	writeTargetResults(w, out.Targets)
	if out.Hint != "" {
		fmt.Fprintln(w, out.Hint)
	}
	return nil
}

// writeMCPEndpointTable prints one row per MCP endpoint.
func writeMCPEndpointTable(w io.Writer, idx *bnkscan.Index) {
	if len(idx.MCP) == 0 {
		fmt.Fprintf(w, "no MCP endpoints (BNK %s, %d HTTPRoute(s)); annotate a route with %s=mcp or serve an /mcp path\n",
			idx.Generation, len(idx.HTTPRoutes), bnkscan.AnnotationProtocol)
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tGATEWAY\tREADY\tURL\tAUTH\tPERSISTENCE\tTOOLS")
	for _, e := range idx.MCP {
		persistence := "-"
		if e.Persistence != nil {
			persistence = e.Persistence.Type
		}
		tools := "-"
		if names := e.ToolNames(); len(names) > 0 {
			tools = strings.Join(names, ",")
		}
		fmt.Fprintf(tw, "%s\t%s/%s\t%v\t%s\t%s\t%s\t%s\n",
			e.Name(), e.GatewayNamespace, e.Gateway, e.GatewayReady, or(e.PrimaryURL(), "-"), e.Auth, persistence, tools)
	}
	_ = tw.Flush()
}
