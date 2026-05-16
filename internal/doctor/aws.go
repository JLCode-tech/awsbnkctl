package doctor

import (
	"context"
	"fmt"
	"time"

	awspkg "github.com/JLCode-tech/awsbnkctl/internal/aws"
	"github.com/JLCode-tech/awsbnkctl/internal/config"
)

// awsChecks runs the AWS-shaped pre-flight checks introduced in Sprint 1
// per PRD 07 § "internal/aws/". Replaces the Sprint 0 "AWS support
// coming in Sprint 1" placeholder in doctor.Run().
//
// Three checks, ordered by failure cost (cheapest → most informative):
//
//  1. credentials configured — no API call; just confirms the cred
//     chain resolved a key (env / profile / instance role / SSO).
//  2. STS GetCallerIdentity — one API call; validates the resolved key
//     is accepted by AWS. AccessDenied here means revoked or
//     restricted credentials, distinct from "no creds at all".
//  3. EKS DescribeCluster permission probe — calls DescribeCluster
//     against a deliberately-bogus cluster name; expects
//     ResourceNotFound. AccessDenied here means the cred is valid but
//     lacks the eks:DescribeCluster permission `awsbnkctl up cluster`
//     will need post-apply.
//
// EC2 quota + S3 PutObject probes are deferred to Sprint 2 (per
// PRD 07 § "Spike protocol" and PRD 08 § "S3 supply chain") — both
// require workspace-level context (region, bucket name) that doctor
// doesn't have at pre-flight time on a fresh dev box.
func awsChecks(ctx context.Context, cctx *config.Context) []withWhy {
	var out []withWhy

	// Short timeout — doctor must not block a fresh dev box that
	// happens to be offline. STS + EKS resolve in <2s on a good
	// connection; 10s is generous slack.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	region := awsRegionFromContext(cctx)

	// Check 1: credentials configured (cheap, no API call).
	credSource, credErr := awspkg.CredentialsConfigured(ctx, awspkg.Options{Region: region})
	if credErr != nil {
		out = append(out, withWhy{
			Check: Check{
				Name:   "aws credentials",
				Status: StatusWarning,
				Detail: "no credentials resolved via env / profile / instance role / SSO — set AWS_PROFILE or AWS_ACCESS_KEY_ID before `awsbnkctl up`",
			},
			Why: "every AWS-side operation needs valid credentials",
		})
		// No point running the live checks without creds.
		out = append(out, withWhy{
			Check: Check{Name: "aws sts caller-identity", Status: StatusSkipped, Detail: "skipped (no credentials)"},
			Why:   "validates credentials are accepted by AWS",
		})
		out = append(out, withWhy{
			Check: Check{Name: "aws eks:DescribeCluster permission", Status: StatusSkipped, Detail: "skipped (no credentials)"},
			Why:   "validates `awsbnkctl up cluster` will have permission to read the cluster post-apply",
		})
		return out
	}
	out = append(out, withWhy{
		Check: Check{Name: "aws credentials", Status: StatusOK, Detail: credSource},
		Why:   "every AWS-side operation needs valid credentials",
	})

	// Build a shared Clients for the remaining live checks.
	clients, err := awspkg.NewClients(ctx, awspkg.Options{Region: region})
	if err != nil {
		// Cred chain resolved but client construction failed — usually
		// a malformed profile or region. Surface and stop.
		out = append(out, withWhy{
			Check: Check{
				Name:   "aws sts caller-identity",
				Status: StatusError,
				Detail: fmt.Sprintf("client construction failed: %v", err),
			},
			Why: "validates credentials are accepted by AWS",
		})
		return out
	}

	// Check 2: STS GetCallerIdentity.
	id, err := clients.CallerIdentity(ctx)
	if err != nil {
		out = append(out, withWhy{
			Check: Check{
				Name:   "aws sts caller-identity",
				Status: StatusError,
				Detail: fmt.Sprintf("AccessDenied or transport error: %v", err),
			},
			Why: "validates credentials are accepted by AWS",
		})
		// No point probing EKS if STS already says no.
		out = append(out, withWhy{
			Check: Check{Name: "aws eks:DescribeCluster permission", Status: StatusSkipped, Detail: "skipped (STS failed)"},
			Why:   "validates `awsbnkctl up cluster` will have permission to read the cluster post-apply",
		})
		return out
	}
	out = append(out, withWhy{
		Check: Check{Name: "aws sts caller-identity", Status: StatusOK, Detail: fmt.Sprintf("account=%s arn=%s", id.Account, id.ARN)},
		Why:   "validates credentials are accepted by AWS",
	})

	// Check 3: EKS DescribeCluster permission probe against a bogus
	// cluster name. ResourceNotFound = permission OK. AccessDenied =
	// permission missing. Other errors = unknown; surface verbatim.
	_, err = clients.DescribeCluster(ctx, "awsbnkctl-doctor-permission-probe-does-not-exist")
	switch {
	case err == nil:
		// Cluster with that name actually exists, somehow. Treat as
		// permission OK (we got a response).
		out = append(out, withWhy{
			Check: Check{Name: "aws eks:DescribeCluster permission", Status: StatusOK, Detail: "probe returned (unexpected: a cluster matched the probe name)"},
			Why:   "validates `awsbnkctl up cluster` will have permission to read the cluster post-apply",
		})
	case awspkg.IsResourceNotFound(err):
		out = append(out, withWhy{
			Check: Check{Name: "aws eks:DescribeCluster permission", Status: StatusOK, Detail: "probe returned ResourceNotFound (permission OK)"},
			Why:   "validates `awsbnkctl up cluster` will have permission to read the cluster post-apply",
		})
	default:
		out = append(out, withWhy{
			Check: Check{
				Name:   "aws eks:DescribeCluster permission",
				Status: StatusWarning,
				Detail: fmt.Sprintf("probe failed: %v (may be AccessDenied — verify IAM policy attaches eks:DescribeCluster)", err),
			},
			Why: "validates `awsbnkctl up cluster` will have permission to read the cluster post-apply",
		})
	}

	return out
}

// awsRegionFromContext extracts the AWS region from the workspace
// config if present. Sprint 1 stub: the workspace schema still carries
// IBM-named fields (Sprint 2 / PRD 04 retargets), so we fall back to
// the AWS_REGION env var that the SDK's default chain reads anyway.
// This intentionally accepts an empty region — internal/aws's default
// chain will error out with a clear message at API call time.
func awsRegionFromContext(cctx *config.Context) string {
	if cctx != nil && cctx.Workspace != nil {
		if r := cctx.Workspace.IBMCloud.Region; r != "" {
			return r
		}
	}
	return ""
}
