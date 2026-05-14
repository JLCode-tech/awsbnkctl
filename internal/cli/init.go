package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

// Sprint 0 stub. The IBM-coupled wizard (region/resource-group/cluster
// prompts + IAM verify + Trusted Profile bootstrap) was retired with
// internal/ibm. The AWS-shaped wizard lands in Sprint 1 once
// internal/aws exists; see docs/prd/07-EKS-CLUSTER-SRIOV.md for the
// input list (region, cluster_name, vpc_id, subnet_ids, …).
//
// Until then, `awsbnkctl init` reports a clean "not yet implemented"
// rather than running the legacy ROKS prompts against an AWS user.

func runInit(_ *cobra.Command, _ []string) error {
	return errors.New("awsbnkctl init is not implemented yet — AWS support lands in Sprint 1 (see docs/prd/07-EKS-CLUSTER-SRIOV.md)")
}
