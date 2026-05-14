package cli

import (
	"os"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/JLCode-tech/awsbnkctl/internal/k8s"
)

var (
	kApplyFilename  string
	kApplyNamespace string
	kApplyForce     bool
)

func newKApplyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply -f <file-or-dir> [-n <ns>] [--force]",
		Short: "Server-side apply YAML/JSON manifests, directories, or kustomize bases",
		Long: `Server-side apply with field-manager '` + k8s.FieldManager + `'.

  -f <file>       single YAML/JSON file (multi-doc YAML supported)
  -f <dir>        directory: kustomization.yaml-detected → krusty build;
                  otherwise recursive *.yaml / *.yml
  -f -            stdin (multi-doc YAML)

--force passes through to SSA's force-conflicts flag, identical to
kubectl apply --server-side --force-conflicts.

Examples:

  awsbnkctl k apply -f deploy.yaml -n f5-bnk
  awsbnkctl k apply -f manifests/
  cat deploy.yaml | awsbnkctl k apply -f -`,
		RunE: runKApply,
	}
	flags := cmd.Flags()
	flags.StringVarP(&kApplyFilename, "filename", "f", "", "file, directory, or '-' for stdin")
	flags.StringVarP(&kApplyNamespace, "namespace", "n", "", "namespace for namespaced resources without an explicit namespace field")
	flags.BoolVar(&kApplyForce, "force", false, "force-conflicts on server-side apply (kubectl apply --force-conflicts)")
	_ = cmd.MarkFlagRequired("filename")
	return cmd
}

func init() {
	kCmd.AddCommand(newKApplyCmd())
}

func runKApply(cmd *cobra.Command, _ []string) error {
	opts := &k8s.ApplyOptions{
		Filename:  kApplyFilename,
		Namespace: kApplyNamespace,
		Force:     kApplyForce,
		IOStreams: genericiooptions.IOStreams{
			In:     os.Stdin,
			Out:    os.Stdout,
			ErrOut: os.Stderr,
		},
	}
	return opts.Run(cmd.Context())
}
