package phases

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/awsmw"
	"github.com/JLCode-tech/awsbnkctl/internal/aws/state"
	"github.com/JLCode-tech/awsbnkctl/internal/intent"
)

// Phase00Preflight runs the pre-flight checks before any provisioning begins:
//   - SSO sentinel check (hard-exits if auth failed during a previous API call)
//   - sts:GetCallerIdentity to verify credentials are live
//   - ensures .awsbnkctl/<cluster>/ exists and loads state.env if present
//
// In dry-run mode the state values are set in memory but not persisted to disk.
// Returns the loaded (or newly-created) State for subsequent phases.
func Phase00Preflight(ctx context.Context, cl *intent.Cluster, st *state.State, clients *Clients, dryRun bool) error {
	awsmw.CheckAuthOrDie(clients.Profile)
	fmt.Fprintf(os.Stderr, "[phase 00] preflight: cluster=%s region=%s\n",
		cl.Metadata.Name, cl.Metadata.Region)

	// Verify credentials are live.
	out, err := clients.STS.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return fmt.Errorf("phase00: sts:GetCallerIdentity: %w", err)
	}
	account := ""
	if out.Account != nil {
		account = *out.Account
	}
	fmt.Fprintf(os.Stderr, "[phase 00] authenticated: account=%s\n", account)

	// Ensure the state directory exists and persist the cluster name + region.
	st.Set("CLUSTER_NAME", cl.Metadata.Name)
	st.Set("AWS_REGION", cl.Metadata.Region)
	if !dryRun {
		if err := st.Save(); err != nil {
			return fmt.Errorf("phase00: saving state: %w", err)
		}
	}

	fmt.Fprintf(os.Stderr, "[phase 00] preflight OK\n")
	return nil
}
