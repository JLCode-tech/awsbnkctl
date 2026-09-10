package egresssnat

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/state"
	"github.com/JLCode-tech/awsbnkctl/internal/intent"
	"github.com/JLCode-tech/awsbnkctl/internal/scenarios"
)

func jumphostCtx(t *testing.T) *scenarios.Context {
	t.Helper()
	st, err := state.Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	st.Set("JUMPHOST_INSTANCE_ID", "i-test")
	st.Set("JUMPHOST_BNK_EXT_ENI_IP", "10.0.10.161")
	st.Set("TMM_EXT_SELFIP", "10.0.10.240")
	return &scenarios.Context{
		Ctx:     context.Background(),
		Out:     io.Discard,
		State:   st,
		Cluster: &intent.Cluster{Metadata: intent.Metadata{Region: "ap-southeast-2"}},
		Options: map[string]string{},
	}
}

func fastSourceIPPoll(t *testing.T) {
	t.Helper()
	oldT, oldI := sourceIPPollTimeout, sourceIPPollInterval
	sourceIPPollTimeout, sourceIPPollInterval = 30*time.Millisecond, 5*time.Millisecond
	t.Cleanup(func() { sourceIPPollTimeout, sourceIPPollInterval = oldT, oldI })
}

// TestVerifySourceIP_ProvesSNAT: the reflector on the jumphost sees TMM's
// external SelfIP, so the pod's egress went through TMM. Gating assertion OK.
func TestVerifySourceIP_ProvesSNAT(t *testing.T) {
	fastSourceIPPoll(t)
	var started, stopped int
	var execCmd []string
	d := &VerifyDeps{
		StartResponderFn: func(_ context.Context, _ *scenarios.Context, port int) error { started = port; return nil },
		StopResponderFn:  func(_ context.Context, _ *scenarios.Context, port int) { stopped = port },
		ExecInPodFn: func(_ *scenarios.Context, ns, pod, container string, command ...string) (string, error) {
			execCmd = append([]string{ns, pod, container}, command...)
			return "10.0.10.240\n", nil
		},
	}
	skipped, a := verifySourceIP(d, jumphostCtx(t), "awsbnkctl-scn-egress")
	if skipped || !a.OK {
		t.Fatalf("skipped=%v ok=%v got=%q", skipped, a.OK, a.Got)
	}
	if started != reflectorPort || stopped != reflectorPort {
		t.Errorf("reflector start/stop ports = %d/%d, want %d", started, stopped, reflectorPort)
	}
	if strings.Join(execCmd[:3], "/") != "awsbnkctl-scn-egress/egress-client/curl" || !strings.Contains(strings.Join(execCmd, " "), "http://10.0.10.161:8081/") {
		t.Errorf("exec target/command = %v", execCmd)
	}
}

// TestVerifySourceIP_UncapturedEgressFails: the reflector sees the node or
// pod address instead of the SelfIP → egress did not go through TMM → fail.
func TestVerifySourceIP_UncapturedEgressFails(t *testing.T) {
	fastSourceIPPoll(t)
	d := &VerifyDeps{
		StartResponderFn: func(context.Context, *scenarios.Context, int) error { return nil },
		ExecInPodFn: func(*scenarios.Context, string, string, string, ...string) (string, error) {
			return "10.0.11.37", nil
		},
	}
	skipped, a := verifySourceIP(d, jumphostCtx(t), "ns")
	if skipped || a.OK {
		t.Fatalf("expected a failing, non-skipped assertion; got skipped=%v ok=%v (%s)", skipped, a.OK, a.Got)
	}
	if !strings.Contains(a.Got, `"10.0.11.37"`) || !strings.Contains(a.Got, `"10.0.10.240"`) {
		t.Errorf("detail should name seen and wanted addresses: %q", a.Got)
	}
}

// TestVerifySourceIP_ReflectorStartFailureIsAFailure: no reflector, no proof.
func TestVerifySourceIP_ReflectorStartFailureIsAFailure(t *testing.T) {
	d := &VerifyDeps{
		StartResponderFn: func(context.Context, *scenarios.Context, int) error { return errors.New("ssh: eice tunnel refused") },
		ExecInPodFn:      func(*scenarios.Context, string, string, string, ...string) (string, error) { return "", nil },
	}
	skipped, a := verifySourceIP(d, jumphostCtx(t), "ns")
	if skipped || a.OK || !strings.Contains(a.Got, "eice tunnel refused") {
		t.Fatalf("skipped=%v ok=%v got=%q", skipped, a.OK, a.Got)
	}
}

// TestVerifySourceIP_SkipsWithoutJumphost: no jumphost state → informational,
// not red.
func TestVerifySourceIP_SkipsWithoutJumphost(t *testing.T) {
	d := &VerifyDeps{
		StartResponderFn: func(context.Context, *scenarios.Context, int) error { t.Fatal("must not start"); return nil },
		ExecInPodFn:      func(*scenarios.Context, string, string, string, ...string) (string, error) { return "", nil },
	}
	sctx := &scenarios.Context{Ctx: context.Background(), Options: map[string]string{}}
	skipped, a := verifySourceIP(d, sctx, "ns")
	if !skipped || !a.OK || !strings.Contains(a.Got, "skipped") {
		t.Fatalf("skipped=%v ok=%v got=%q", skipped, a.OK, a.Got)
	}
}
