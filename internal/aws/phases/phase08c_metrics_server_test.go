package phases

import (
	"context"
	"strings"
	"testing"

	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/state"
)

func TestPhase08cMetricsServer_DryRun(t *testing.T) {
	cl, st := phase08bCluster(t)
	clients := testClientsAllMock()
	if err := Phase08cMetricsServer(context.Background(), cl, st, clients, true); err != nil {
		t.Fatal(err)
	}
	if st.Get("METRICS_SERVER_ADDON") != "dry-run-true" {
		t.Errorf("state %q", st.Get("METRICS_SERVER_ADDON"))
	}
	if clients.EKS.(*mockEKS).createAddonCalls != 0 {
		t.Error("dry-run must not create the add-on")
	}
}

func TestEnsureMetricsServerAddon_CreatesOnce(t *testing.T) {
	mock := newMockEKS()
	var log strings.Builder
	created, err := EnsureMetricsServerAddon(context.Background(), mock, "c1", &log)
	if err != nil || !created || mock.createAddonCalls != 1 {
		t.Fatalf("created=%v calls=%d err=%v", created, mock.createAddonCalls, err)
	}
	if mock.lastResolveConflicts != ekstypes.ResolveConflictsOverwrite {
		t.Errorf("ResolveConflicts %v", mock.lastResolveConflicts)
	}
	created, err = EnsureMetricsServerAddon(context.Background(), mock, "c1", &log)
	if err != nil || created || mock.createAddonCalls != 1 {
		t.Fatalf("second call: created=%v calls=%d err=%v", created, mock.createAddonCalls, err)
	}
	if !strings.Contains(log.String(), "already present") {
		t.Errorf("log %q", log.String())
	}
}

func TestPhase08cMetricsServerDown_ClearsState(t *testing.T) {
	cl, _ := phase08bCluster(t)
	st, _ := state.Load(t.TempDir())
	st.Set("METRICS_SERVER_ADDON", "true")
	if err := Phase08cMetricsServerDown(context.Background(), cl, st, testClientsAllMock()); err != nil {
		t.Fatal(err)
	}
	if st.Get("METRICS_SERVER_ADDON") != "" {
		t.Error("state not cleared")
	}
}
