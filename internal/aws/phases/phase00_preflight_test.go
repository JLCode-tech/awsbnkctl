package phases

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/state"
	"github.com/JLCode-tech/awsbnkctl/internal/intent"
)

func newHostDeviceCluster(instanceType string) *intent.Cluster {
	return newPatternCluster("host-device", instanceType)
}

// newPatternCluster builds a minimal cluster fixture for a given BNK pattern.
// Pattern is set directly (no Load/applyDefaults), so the legacy "host-device"
// alias is normalized by the capability helpers, not rewritten on the struct.
func newPatternCluster(pattern, instanceType string) *intent.Cluster {
	cl := &intent.Cluster{}
	cl.Pattern = pattern
	cl.ClusterSpec = &intent.ClusterSpec{
		NodeGroups: []intent.NodeGroupSpec{
			{Name: "default", InstanceType: instanceType, DesiredSize: 3, MinSize: 1, MaxSize: 5, DiskSize: 50},
		},
	}
	return cl
}

func makeInstanceTypesOut(typ string, maxENIs int32, vcpus int32, memoryMiB int64) *ec2.DescribeInstanceTypesOutput {
	return &ec2.DescribeInstanceTypesOutput{
		InstanceTypes: []ec2types.InstanceTypeInfo{
			{
				InstanceType: ec2types.InstanceType(typ),
				NetworkInfo: &ec2types.NetworkInfo{
					MaximumNetworkInterfaces: aws.Int32(maxENIs),
				},
				VCpuInfo: &ec2types.VCpuInfo{
					DefaultVCpus: aws.Int32(vcpus),
				},
				MemoryInfo: &ec2types.MemoryInfo{
					SizeInMiB: aws.Int64(memoryMiB),
				},
			},
		},
	}
}

func TestCheckHostDeviceCapacity_OK(t *testing.T) {
	cl := newHostDeviceCluster("m5.2xlarge")
	m := &mockEC2{describeInstanceTypesOut: makeInstanceTypesOut("m5.2xlarge", 4, 8, 32768)}
	if err := checkHostDeviceCapacity(context.Background(), cl, m); err != nil {
		t.Fatalf("expected nil error for m5.2xlarge, got %v", err)
	}
}

func TestCheckHostDeviceCapacity_TooSmall(t *testing.T) {
	// dual-interface floor is 3 ENIs; a 2-ENI type fails the ENI check.
	cl := newHostDeviceCluster("t3.medium")
	m := &mockEC2{describeInstanceTypesOut: makeInstanceTypesOut("t3.medium", 2, 2, 4096)}
	err := checkHostDeviceCapacity(context.Background(), cl, m)
	if err == nil {
		t.Fatal("expected error for t3.medium (2 ENIs), got nil")
	}
	if !strings.Contains(err.Error(), "network interfaces") {
		t.Errorf("error %q does not mention 'network interfaces'", err)
	}
	if !strings.Contains(err.Error(), "m5.xlarge") {
		t.Errorf("error %q does not suggest m5.xlarge", err)
	}
}

// TestCheckCapacity_DualInterface_ENIFloor3 asserts a 3-ENI type clears the
// dual-interface floor on the ENI axis (other axes still apply).
func TestCheckCapacity_DualInterface_ENIFloor3(t *testing.T) {
	cl := newPatternCluster("dual-interface", "m5.xlarge")
	m := &mockEC2{describeInstanceTypesOut: makeInstanceTypesOut("m5.xlarge", 3, 8, 32768)}
	if err := checkHostDeviceCapacity(context.Background(), cl, m); err != nil {
		t.Fatalf("expected nil for dual-interface with 3 ENIs/8 vCPU/32GiB, got %v", err)
	}
}

// TestCheckCapacity_ExternalOnly_ENIFloor2 asserts the single-interface floor is
// 2: a 2-ENI type passes, a 1-ENI type fails the ENI check.
func TestCheckCapacity_ExternalOnly_ENIFloor2(t *testing.T) {
	ok := newPatternCluster("external-only", "m5.xlarge")
	m := &mockEC2{describeInstanceTypesOut: makeInstanceTypesOut("m5.xlarge", 2, 8, 32768)}
	if err := checkHostDeviceCapacity(context.Background(), ok, m); err != nil {
		t.Fatalf("expected nil for external-only with 2 ENIs/8 vCPU/32GiB, got %v", err)
	}

	bad := newPatternCluster("external-only", "small.type")
	m2 := &mockEC2{describeInstanceTypesOut: makeInstanceTypesOut("small.type", 1, 8, 32768)}
	err := checkHostDeviceCapacity(context.Background(), bad, m2)
	if err == nil {
		t.Fatal("expected error for external-only with 1 ENI, got nil")
	}
	if !strings.Contains(err.Error(), "network interfaces") {
		t.Errorf("error %q does not mention 'network interfaces'", err)
	}
}

func TestCheckHostDeviceCapacity_APIError(t *testing.T) {
	cl := newHostDeviceCluster("m5.xlarge")
	m := &mockEC2{describeInstanceTypesErr: errStub("api boom")}
	err := checkHostDeviceCapacity(context.Background(), cl, m)
	if err == nil {
		t.Fatal("expected error when DescribeInstanceTypes fails, got nil")
	}
	if !strings.Contains(err.Error(), "DescribeInstanceTypes") {
		t.Errorf("error %q does not mention DescribeInstanceTypes", err)
	}
}

func TestCheckHostDeviceCapacity_NoNodeGroups_OK(t *testing.T) {
	cl := &intent.Cluster{}
	cl.Pattern = "host-device"
	m := &mockEC2{}
	if err := checkHostDeviceCapacity(context.Background(), cl, m); err != nil {
		t.Fatalf("expected nil when no node groups defined, got %v", err)
	}
}

func TestCheckHostDeviceCapacity_EmptyInstanceType_OK(t *testing.T) {
	cl := newHostDeviceCluster("")
	m := &mockEC2{}
	if err := checkHostDeviceCapacity(context.Background(), cl, m); err != nil {
		t.Fatalf("expected nil when instance type is empty (Validate fills default later), got %v", err)
	}
}

func TestCheckHostDeviceCapacity_InsufficientCPU(t *testing.T) {
	cl := newHostDeviceCluster("m5.xlarge")
	// 4 ENIs but only 4 vCPUs — should fail on vCPU check.
	m := &mockEC2{describeInstanceTypesOut: makeInstanceTypesOut("m5.xlarge", 4, 4, 32768)}
	err := checkHostDeviceCapacity(context.Background(), cl, m)
	if err == nil {
		t.Fatal("expected error for insufficient vCPUs, got nil")
	}
	if !strings.Contains(err.Error(), "vCPUs") {
		t.Errorf("error %q does not mention 'vCPUs'", err)
	}
	if !strings.Contains(err.Error(), "8") {
		t.Errorf("error %q does not mention required count '8'", err)
	}
}

func TestCheckHostDeviceCapacity_InsufficientMemory(t *testing.T) {
	cl := newHostDeviceCluster("m5.xlarge")
	// 4 ENIs + 8 vCPUs but only 16384 MiB — should fail on memory check.
	m := &mockEC2{describeInstanceTypesOut: makeInstanceTypesOut("m5.xlarge", 4, 8, 16384)}
	err := checkHostDeviceCapacity(context.Background(), cl, m)
	if err == nil {
		t.Fatal("expected error for insufficient memory, got nil")
	}
	if !strings.Contains(err.Error(), "memory") {
		t.Errorf("error %q does not mention 'memory'", err)
	}
	if !strings.Contains(err.Error(), "32768") {
		t.Errorf("error %q does not mention required MiB '32768'", err)
	}
}

func TestCheckHostDeviceCapacity_InsufficientDesiredSize(t *testing.T) {
	cl := newHostDeviceCluster("m5.2xlarge")
	cl.ClusterSpec.NodeGroups[0].DesiredSize = 1
	// 4 ENIs + 8 vCPUs + 32 GiB — all instance checks pass, but desiredSize=1.
	m := &mockEC2{describeInstanceTypesOut: makeInstanceTypesOut("m5.2xlarge", 4, 8, 32768)}
	err := checkHostDeviceCapacity(context.Background(), cl, m)
	if err == nil {
		t.Fatal("expected error for desiredSize=1, got nil")
	}
	if !strings.Contains(err.Error(), "desiredSize") {
		t.Errorf("error %q does not mention 'desiredSize'", err)
	}
	if !strings.Contains(err.Error(), "3") {
		t.Errorf("error %q does not mention required size '3'", err)
	}
}

func TestCheckHostDeviceCapacity_AllFailures(t *testing.T) {
	// t3.medium equivalent: 2 ENI (< dual floor 3), 2 vCPU, 4 GiB (4096 MiB), DesiredSize=1.
	cl := newHostDeviceCluster("t3.medium")
	cl.ClusterSpec.NodeGroups[0].DesiredSize = 1
	m := &mockEC2{describeInstanceTypesOut: makeInstanceTypesOut("t3.medium", 2, 2, 4096)}
	err := checkHostDeviceCapacity(context.Background(), cl, m)
	if err == nil {
		t.Fatal("expected aggregated error, got nil")
	}
	errStr := err.Error()
	for _, substr := range []string{"network interfaces", "vCPUs", "memory", "desiredSize"} {
		if !strings.Contains(errStr, substr) {
			t.Errorf("aggregated error missing %q; got: %s", substr, errStr)
		}
	}
}

// errStub is a tiny error type for tests that need a non-nil error without
// importing fmt every time.
type errStub string

func (e errStub) Error() string { return string(e) }

func TestPhase00Preflight_WithJWT(t *testing.T) {
	dir := t.TempDir()
	st, err := state.Load(dir)
	if err != nil {
		t.Fatalf("state.Load: %v", err)
	}

	jwtFile := filepath.Join(dir, "license.jwt")
	testToken := "eyJhbGciOiJSUzUxMiIsInR5cCI6IkpXVCIsImtpZCI6InYxIiwiamt1IjoiaHR0cHM6Ly9wcm9kdWN0LXRzdC5hcGlzLmY1bmV0d29ya3MubmV0L2VlL3YxL2tleXMvandrcyJ9.eyJzdWIiOiJUU1QtMTIzNDUiLCJpc3MiOiJGNSBJbmMuIiwiZjVfc2F0IjoxODA3ODMzNjAwfQ.sig"
	if err := os.WriteFile(jwtFile, []byte(testToken), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cl := &intent.Cluster{
		Metadata: intent.Metadata{Name: "test-cluster", Region: "us-west-2"},
		Bnk: &intent.BnkSpec{
			JWT: "license.jwt",
		},
		SourcePath: filepath.Join(dir, "cluster.yaml"),
	}

	// 1. With valid test token
	if err := Phase00Preflight(context.Background(), cl, st, nil, true); err != nil {
		t.Fatalf("Phase00Preflight with test token: %v", err)
	}

	// 2. With invalid token content (non-fatal warning note)
	if err := os.WriteFile(jwtFile, []byte("invalid-jwt-data"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := Phase00Preflight(context.Background(), cl, st, nil, true); err != nil {
		t.Fatalf("Phase00Preflight with invalid token: %v", err)
	}

	// 3. With non-existent JWT path (silently skipped if file cannot be read)
	cl.Bnk.JWT = "non-existent.jwt"
	if err := Phase00Preflight(context.Background(), cl, st, nil, true); err != nil {
		t.Fatalf("Phase00Preflight with non-existent token: %v", err)
	}

	// 4. With nil cl.Bnk
	cl.Bnk = nil
	if err := Phase00Preflight(context.Background(), cl, st, nil, true); err != nil {
		t.Fatalf("Phase00Preflight with nil Bnk: %v", err)
	}
}

func TestCheckVPCQuota_UnderLimit(t *testing.T) {
	dir := t.TempDir()
	st, _ := state.Load(dir)
	cl := &intent.Cluster{Metadata: intent.Metadata{Name: "new-cluster", Region: "ap-southeast-2"}}
	m := &mockEC2{
		describeVpcsOut: &ec2.DescribeVpcsOutput{
			Vpcs: []ec2types.Vpc{
				{VpcId: ptr("vpc-1")},
				{VpcId: ptr("vpc-2")},
				{VpcId: ptr("vpc-3")},
			},
		},
	}
	if err := checkVPCQuota(context.Background(), cl, st, m); err != nil {
		t.Fatalf("expected nil when 3/5 VPCs in use, got %v", err)
	}
}

func TestCheckVPCQuota_AtLimitFails(t *testing.T) {
	dir := t.TempDir()
	st, _ := state.Load(dir)
	cl := &intent.Cluster{Metadata: intent.Metadata{Name: "new-cluster", Region: "ap-southeast-2"}}
	m := &mockEC2{
		describeVpcsOut: &ec2.DescribeVpcsOutput{
			Vpcs: []ec2types.Vpc{
				{VpcId: ptr("vpc-1"), Tags: []ec2types.Tag{{Key: ptr("Name"), Value: ptr("other-1")}}},
				{VpcId: ptr("vpc-2"), Tags: []ec2types.Tag{{Key: ptr("Name"), Value: ptr("other-2")}}},
				{VpcId: ptr("vpc-3"), Tags: []ec2types.Tag{{Key: ptr("Name"), Value: ptr("other-3")}}},
				{VpcId: ptr("vpc-4"), Tags: []ec2types.Tag{{Key: ptr("Name"), Value: ptr("other-4")}}},
				{VpcId: ptr("vpc-5"), Tags: []ec2types.Tag{{Key: ptr("Name"), Value: ptr("other-5")}}},
			},
		},
	}
	err := checkVPCQuota(context.Background(), cl, st, m)
	if err == nil {
		t.Fatal("expected VPC quota error when 5/5 VPCs exist, got nil")
	}
	if !strings.Contains(err.Error(), "VpcLimitExceeded") {
		t.Errorf("expected error to mention VpcLimitExceeded, got: %v", err)
	}
}

func TestCheckVPCQuota_ExistingClusterVPCSkips(t *testing.T) {
	dir := t.TempDir()
	st, _ := state.Load(dir)
	cl := &intent.Cluster{Metadata: intent.Metadata{Name: "my-cluster", Region: "ap-southeast-2"}}
	m := &mockEC2{
		describeVpcsOut: &ec2.DescribeVpcsOutput{
			Vpcs: []ec2types.Vpc{
				{VpcId: ptr("vpc-1"), Tags: []ec2types.Tag{{Key: ptr("awsbnkctl:cluster"), Value: ptr("my-cluster")}}},
				{VpcId: ptr("vpc-2")},
				{VpcId: ptr("vpc-3")},
				{VpcId: ptr("vpc-4")},
				{VpcId: ptr("vpc-5")},
			},
		},
	}
	if err := checkVPCQuota(context.Background(), cl, st, m); err != nil {
		t.Fatalf("expected nil when cluster VPC already exists among 5 VPCs, got %v", err)
	}
}
