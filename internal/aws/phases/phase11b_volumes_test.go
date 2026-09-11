package phases

import (
	"context"
	"testing"
	"time"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func fastVolumeSweep(t *testing.T) {
	t.Helper()
	oldT, oldI := volumeSweepTimeout, volumeSweepInterval
	volumeSweepTimeout, volumeSweepInterval = 40*time.Millisecond, 5*time.Millisecond
	t.Cleanup(func() { volumeSweepTimeout, volumeSweepInterval = oldT, oldI })
}

// TestDeleteClusterVolumes_DeletesAvailableLeavesBusy: available PVC volumes go,
// an attached one is retried until the budget runs out and then reported.
func TestDeleteClusterVolumes_DeletesAvailableLeavesBusy(t *testing.T) {
	fastVolumeSweep(t)
	m := &mockEC2{volumes: map[string]ec2types.VolumeState{
		"vol-a": ec2types.VolumeStateAvailable,
		"vol-b": ec2types.VolumeStateAvailable,
		"vol-c": ec2types.VolumeStateInUse,
	}}
	deleteClusterVolumes(context.Background(), m, "tracer")
	if m.deleteVolumeCalls != 2 {
		t.Errorf("deleteVolumeCalls = %d, want 2", m.deleteVolumeCalls)
	}
	if _, still := m.volumes["vol-c"]; !still || len(m.volumes) != 1 {
		t.Errorf("only the in-use volume should remain, got %v", m.volumes)
	}
}

// TestDeleteClusterVolumes_NilEC2NoPanic: down with no EC2 client is a no-op.
func TestDeleteClusterVolumes_NilEC2NoPanic(t *testing.T) {
	deleteClusterVolumes(context.Background(), nil, "tracer")
}
