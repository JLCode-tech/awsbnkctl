package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/cobra"

	"github.com/JLCode-tech/awsbnkctl/internal/config"
)

// `awsbnkctl test hosts {list,add,remove,clear}` manages the workspace's
// `test.connectivity.extra_hosts` slice.
//
// Matches the bnkctl ecosystem standard: idempotent add/remove,
// hermetic persistence through the existing workspace marshaller.

var flagTestHostsClearAuto bool

var testHostsCmd = &cobra.Command{
	Use:   "hosts",
	Short: "Manage test.connectivity.extra_hosts in the workspace config",
	Long: `Test hosts are URLs probed by ` + "`awsbnkctl test connectivity`" + ` and
(in workspace-driven mode) ` + "`awsbnkctl test dns`" + `. They are stored under
` + "`test.connectivity.extra_hosts`" + ` in the workspace's config.yaml.

This command group provides first-class CLI management for that slice:
idempotent add/remove, ` + "`-o json`" + ` on list.`,
	Args: cobra.NoArgs,
}

var testHostsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured test hosts (one per line; -o json for array)",
	Long: `Prints the workspace's test.connectivity.extra_hosts one per line on
stdout. Empty list emits zero bytes + exit 0 (NOT an error).

With ` + "`-o json`" + `, emits the slice as a JSON array (` + "`[]`" + ` when empty).`,
	Args: cobra.NoArgs,
	RunE: runTestHostsList,
}

var testHostsAddCmd = &cobra.Command{
	Use:   "add <url> [<url> ...]",
	Short: "Append URLs to test.connectivity.extra_hosts (idempotent)",
	Long: `Appends each <url> to test.connectivity.extra_hosts. Idempotent —
adding an already-present URL is a no-op (logs to stderr; exit 0).

Each <url> is validated via std-lib url.Parse. Insertion order is preserved.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runTestHostsAdd,
}

var testHostsRemoveCmd = &cobra.Command{
	Use:   "remove <url> [<url> ...]",
	Short: "Remove URLs from test.connectivity.extra_hosts (idempotent)",
	Long: `Removes each <url> from test.connectivity.extra_hosts. Idempotent —
removing an absent URL is a no-op (logs to stderr; exit 0). Preserves
the order of remaining entries.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runTestHostsRemove,
}

var testHostsClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Remove ALL entries from test.connectivity.extra_hosts",
	Long: `Clears test.connectivity.extra_hosts. Confirmation prompt defaults to
No; pass ` + "`--auto`" + ` to skip the prompt.`,
	Args: cobra.NoArgs,
	RunE: runTestHostsClear,
}

func init() {
	testHostsClearCmd.Flags().BoolVarP(&flagTestHostsClearAuto, "yes", "y", false, "skip the confirmation prompt")
	testHostsClearCmd.Flags().BoolVar(&flagTestHostsClearAuto, "auto", false, "skip the confirmation prompt (alias for --yes)")
	testHostsCmd.AddCommand(testHostsListCmd, testHostsAddCmd, testHostsRemoveCmd, testHostsClearCmd)
}

func mutateExtraHosts(workspace string, fn func([]string) []string) error {
	ws, err := config.LoadWorkspace(workspace)
	if err != nil {
		return err
	}
	ws.Test.Connectivity.ExtraHosts = fn(ws.Test.Connectivity.ExtraHosts)
	return config.SaveWorkspace(workspace, ws)
}

func loadExtraHosts(workspace string) ([]string, error) {
	ws, err := config.LoadWorkspace(workspace)
	if err != nil {
		return nil, err
	}
	return ws.Test.Connectivity.ExtraHosts, nil
}

func runTestHostsList(_ *cobra.Command, _ []string) error {
	cctx, err := requireWorkspace()
	if err != nil {
		return err
	}
	hosts, err := loadExtraHosts(cctx.WorkspaceName)
	if err != nil {
		return err
	}
	if flagOutput == "json" {
		if hosts == nil {
			hosts = []string{}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(hosts)
	}
	for _, h := range hosts {
		fmt.Fprintln(os.Stdout, h)
	}
	return nil
}

func runTestHostsAdd(_ *cobra.Command, args []string) error {
	cctx, err := requireWorkspace()
	if err != nil {
		return err
	}
	for _, raw := range args {
		if err := validateHostURL(raw); err != nil {
			return err
		}
	}
	return mutateExtraHosts(cctx.WorkspaceName, func(cur []string) []string {
		out := append([]string(nil), cur...)
		for _, raw := range args {
			if containsHostString(out, raw) {
				fmt.Fprintf(os.Stderr, "test hosts: %q already present; no-op\n", raw)
				continue
			}
			out = append(out, raw)
			fmt.Fprintf(os.Stderr, "✓ added %q to test.connectivity.extra_hosts\n", raw)
		}
		return out
	})
}

func runTestHostsRemove(_ *cobra.Command, args []string) error {
	cctx, err := requireWorkspace()
	if err != nil {
		return err
	}
	return mutateExtraHosts(cctx.WorkspaceName, func(cur []string) []string {
		toRemove := make(map[string]struct{}, len(args))
		for _, raw := range args {
			toRemove[raw] = struct{}{}
			if !containsHostString(cur, raw) {
				fmt.Fprintf(os.Stderr, "test hosts: %q not present; no-op\n", raw)
			}
		}
		out := make([]string, 0, len(cur))
		for _, existing := range cur {
			if _, drop := toRemove[existing]; drop {
				fmt.Fprintf(os.Stderr, "✓ removed %q from test.connectivity.extra_hosts\n", existing)
				continue
			}
			out = append(out, existing)
		}
		return out
	})
}

func runTestHostsClear(_ *cobra.Command, _ []string) error {
	cctx, err := requireWorkspace()
	if err != nil {
		return err
	}
	if !flagTestHostsClearAuto {
		if !promptYesNo("Clear all test hosts?", false) {
			fmt.Fprintln(os.Stderr, "aborted.")
			return nil
		}
	}
	if err := mutateExtraHosts(cctx.WorkspaceName, func(_ []string) []string {
		return nil
	}); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "✓ cleared test.connectivity.extra_hosts for workspace %q\n", cctx.WorkspaceName)
	return nil
}

func validateHostURL(raw string) error {
	if raw == "" {
		return fmt.Errorf("empty URL is invalid")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid host URL %q: %w", raw, err)
	}
	if u.Scheme == "" && u.Host == "" && u.Path == "" {
		return fmt.Errorf("invalid host URL %q", raw)
	}
	return nil
}

func containsHostString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
