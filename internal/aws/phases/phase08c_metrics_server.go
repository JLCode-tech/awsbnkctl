package phases

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/smithy-go"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/state"
	"github.com/JLCode-tech/awsbnkctl/internal/intent"
)

const (
	// MetricsServerAddonName is the EKS managed add-on that serves the
	// metrics.k8s.io API (pod and node CPU/memory). Forge's fleet view and
	// kubectl top read it through the kubeconfig; without it neither sees
	// any resource usage for the BNK pods.
	MetricsServerAddonName = "metrics-server"
	// metricsServerAddonActiveTimeout is short on purpose: the add-on is
	// created before the node group exists, so its Deployment cannot schedule
	// yet. Non-ACTIVE at the deadline is a warning; the metrics API is
	// checked again by doctor and bnk heal once nodes are up.
	metricsServerAddonActiveTimeout = 2 * time.Minute
	metricsServerAddonPollInterval  = 10 * time.Second
)

// Phase08cMetricsServer adopts the metrics-server EKS managed add-on.
// Ordering: after Phase08 (cluster ACTIVE), before Phase10 (node group), like
// vpc-cni; the Deployment schedules as soon as the first node joins.
func Phase08cMetricsServer(ctx context.Context, cl *intent.Cluster, st *state.State, clients *Clients, dryRun bool) error {
	checkAuthOrDie(clients)
	fmt.Fprintf(os.Stderr, "[phase 08c] metrics-server add-on: cluster=%s\n", cl.Metadata.Name)

	if dryRun {
		fmt.Fprintln(os.Stderr, "[phase 08c] dry-run: would create the metrics-server EKS add-on (metrics.k8s.io for kubectl top and the Forge fleet view)")
		st.Set("METRICS_SERVER_ADDON", "dry-run-true")
		return nil
	}

	if _, err := EnsureMetricsServerAddon(ctx, clients.EKS, cl.Metadata.Name, os.Stderr); err != nil {
		return fmt.Errorf("phase08c: metrics-server add-on: %w", err)
	}
	st.Set("METRICS_SERVER_ADDON", "true")
	return st.Save()
}

// Phase08cMetricsServerDown leaves the add-on in place; it is removed with
// the cluster by Phase08 down. Only state is cleared.
func Phase08cMetricsServerDown(ctx context.Context, cl *intent.Cluster, st *state.State, clients *Clients) error {
	checkAuthOrDie(clients)
	fmt.Fprintf(os.Stderr, "[phase 08c down] metrics-server add-on left in place (removed with the cluster by phase08 down): cluster=%s\n", cl.Metadata.Name)
	st.Set("METRICS_SERVER_ADDON", "")
	return st.Save()
}

// EnsureMetricsServerAddon creates the metrics-server managed add-on when the
// cluster lacks it and polls for ACTIVE (best effort, see the timeout note).
// It returns true when the add-on was created by this call. Shared by
// phase 08c and bnk heal, so an existing cluster of any BNK generation gets
// the metrics API the same way a new one does.
func EnsureMetricsServerAddon(ctx context.Context, eksClient EKSAPI, clusterName string, log io.Writer) (bool, error) {
	if log == nil {
		log = io.Discard
	}
	desc, err := eksClient.DescribeAddon(ctx, &eks.DescribeAddonInput{
		ClusterName: &clusterName,
		AddonName:   aws.String(MetricsServerAddonName),
	})
	created := false
	switch {
	case err == nil:
		fmt.Fprintf(log, "[metrics-server] add-on already present (status=%s)\n", desc.Addon.Status)
	default:
		var apiErr smithy.APIError
		if !errors.As(err, &apiErr) || apiErr.ErrorCode() != "ResourceNotFoundException" {
			return false, fmt.Errorf("DescribeAddon %s: %w", MetricsServerAddonName, err)
		}
		fmt.Fprintf(log, "[metrics-server] creating the %s EKS add-on\n", MetricsServerAddonName)
		if _, cErr := eksClient.CreateAddon(ctx, &eks.CreateAddonInput{
			ClusterName:      &clusterName,
			AddonName:        aws.String(MetricsServerAddonName),
			ResolveConflicts: ekstypes.ResolveConflictsOverwrite,
		}); cErr != nil {
			return false, fmt.Errorf("CreateAddon %s: %w", MetricsServerAddonName, cErr)
		}
		created = true
	}
	return created, pollMetricsServerAddon(ctx, eksClient, clusterName, log)
}

func pollMetricsServerAddon(ctx context.Context, eksClient EKSAPI, clusterName string, log io.Writer) error {
	deadline := time.Now().Add(metricsServerAddonActiveTimeout)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		out, err := eksClient.DescribeAddon(ctx, &eks.DescribeAddonInput{
			ClusterName: &clusterName,
			AddonName:   aws.String(MetricsServerAddonName),
		})
		if err != nil {
			return fmt.Errorf("DescribeAddon %s during poll: %w", MetricsServerAddonName, err)
		}
		switch out.Addon.Status {
		case ekstypes.AddonStatusActive:
			fmt.Fprintf(log, "[metrics-server] add-on ACTIVE\n")
			return nil
		case ekstypes.AddonStatusCreateFailed, ekstypes.AddonStatusDeleteFailed:
			return fmt.Errorf("%s add-on entered terminal state %s", MetricsServerAddonName, out.Addon.Status)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(metricsServerAddonPollInterval):
		}
	}
	fmt.Fprintf(log, "[metrics-server] warning: add-on not ACTIVE within %s (no nodes yet is the usual reason); the metrics API is re-checked by doctor and bnk heal\n", metricsServerAddonActiveTimeout)
	return nil
}
