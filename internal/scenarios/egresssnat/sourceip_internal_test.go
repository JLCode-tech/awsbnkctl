package egresssnat

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/state"
	"github.com/JLCode-tech/awsbnkctl/internal/scenarios"
)

func selfIPCtx(t *testing.T) *scenarios.Context {
	t.Helper()
	st, err := state.Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	st.Set("TMM_EXT_SELFIP", "10.50.10.240")
	return &scenarios.Context{Ctx: context.Background(), Out: io.Discard, State: st, Options: map[string]string{}}
}

func fastSourceIPPoll(t *testing.T) {
	t.Helper()
	oldT, oldI := sourceIPPollTimeout, sourceIPPollInterval
	sourceIPPollTimeout, sourceIPPollInterval = 30*time.Millisecond, 5*time.Millisecond
	t.Cleanup(func() { sourceIPPollTimeout, sourceIPPollInterval = oldT, oldI })
}

// tmctlStub answers the two tmctl reads with the given counters and records
// the curls it is asked to run.
type tmctlStub struct {
	virtual, snat int
	curls         int
	curlErr       error
	bump          bool // advance counters on every curl, as a real TMM would
}

func (s *tmctlStub) exec(_ *scenarios.Context, ns, pod, container string, cmd ...string) (string, error) {
	line := strings.Join(cmd, " ")
	switch {
	case strings.Contains(line, "virtual_server_stat"):
		return "name  clientside.tot_conns\n----  ----\n" +
			"f5-cne-system-awsbnkctl-egress-egress-ipv4  " + itoa(s.virtual) + "\nother-vs  99\n", nil
	case strings.Contains(line, "pool_member_stat"):
		return "pool_name addr serverside.tot_conns\n" +
			"snat_automap[0] 00:00:00:00:00:00:00:00:00:00:FF:FF:0A:32:01:66:00:00:00:00 340\n" +
			"snat_automap[0] 00:00:00:00:00:00:00:00:00:00:FF:FF:0A:32:0A:F0:00:00:00:00 " + itoa(s.snat) + "\n", nil
	case strings.HasPrefix(line, "curl"):
		if ns != "awsbnkctl-scn-egress" || pod != "egress-client" || container != "curl" {
			return "", errors.New("curl exec aimed at the wrong pod: " + ns + "/" + pod + "/" + container)
		}
		s.curls++
		if s.bump {
			s.virtual++
			s.snat++
		}
		return "", s.curlErr
	}
	return "", errors.New("unexpected exec: " + line)
}

func itoa(n int) string { return strconv.Itoa(n) }

// TestVerifyTMMEgress_ProvesPath: each curl moves both TMM counters → OK.
func TestVerifyTMMEgress_ProvesPath(t *testing.T) {
	fastSourceIPPoll(t)
	stub := &tmctlStub{virtual: 1, snat: 2, bump: true}
	d := &VerifyDeps{TMMPodFn: func(*scenarios.Context) (string, error) { return "f5-tmm-x", nil }, ExecInPodFn: stub.exec}
	skipped, a := verifyTMMEgress(d, selfIPCtx(t), "awsbnkctl-scn-egress", "f5-cne-system")
	if skipped || !a.OK {
		t.Fatalf("skipped=%v ok=%v got=%q", skipped, a.OK, a.Got)
	}
	if stub.curls != egressCurlCount {
		t.Errorf("curls = %d, want %d", stub.curls, egressCurlCount)
	}
	if !strings.Contains(a.Got, "+3 conns; SNAT to 10.50.10.240 +3 conns") {
		t.Errorf("detail should carry both deltas: %q", a.Got)
	}
}

// TestVerifyTMMEgress_BypassedTMMFails: curls succeed but TMM never sees them
// (traffic left via the node) → fail with the zero deltas in the detail.
func TestVerifyTMMEgress_BypassedTMMFails(t *testing.T) {
	fastSourceIPPoll(t)
	stub := &tmctlStub{virtual: 5, snat: 5}
	d := &VerifyDeps{TMMPodFn: func(*scenarios.Context) (string, error) { return "f5-tmm-x", nil }, ExecInPodFn: stub.exec}
	skipped, a := verifyTMMEgress(d, selfIPCtx(t), "awsbnkctl-scn-egress", "f5-cne-system")
	if skipped || a.OK {
		t.Fatalf("expected a failing, non-skipped assertion; got skipped=%v ok=%v (%s)", skipped, a.OK, a.Got)
	}
	if !strings.Contains(a.Got, "+0 conns") {
		t.Errorf("detail should show the counters did not move: %q", a.Got)
	}
}

// TestVerifyTMMEgress_NoTMMPodIsAFailure: no TMM, no proof.
func TestVerifyTMMEgress_NoTMMPodIsAFailure(t *testing.T) {
	d := &VerifyDeps{
		TMMPodFn:    func(*scenarios.Context) (string, error) { return "", errors.New("no Running pod with app=f5-tmm") },
		ExecInPodFn: (&tmctlStub{}).exec,
	}
	skipped, a := verifyTMMEgress(d, selfIPCtx(t), "ns", "f5-cne-system")
	if skipped || a.OK || !strings.Contains(a.Got, "app=f5-tmm") {
		t.Fatalf("skipped=%v ok=%v got=%q", skipped, a.OK, a.Got)
	}
}

// TestVerifyTMMEgress_SkipsWithoutExec: no exec client → informational, not red.
func TestVerifyTMMEgress_SkipsWithoutExec(t *testing.T) {
	d := &VerifyDeps{TMMPodFn: func(*scenarios.Context) (string, error) { t.Fatal("must not look for TMM"); return "", nil }}
	skipped, a := verifyTMMEgress(d, selfIPCtx(t), "ns", "f5-cne-system")
	if !skipped || !a.OK || !strings.Contains(a.Got, "skipped") {
		t.Fatalf("skipped=%v ok=%v got=%q", skipped, a.OK, a.Got)
	}
}

func TestTmctlParsing(t *testing.T) {
	if got := ipv4Hex("10.50.10.240"); got != "FF:FF:0A:32:0A:F0:00:00:00:00" {
		t.Errorf("ipv4Hex = %q", got)
	}
	if got := ipv4Hex("nope"); got != "" {
		t.Errorf("ipv4Hex(invalid) = %q, want empty", got)
	}
	out := "name x\nfoo-vs   12\nbar-vs 7\n"
	if got := lastIntOnLine(out, "bar-vs"); got != 7 {
		t.Errorf("lastIntOnLine = %d, want 7", got)
	}
	if got := lastIntOnLine(out, "missing"); got != 0 {
		t.Errorf("lastIntOnLine(missing) = %d, want 0", got)
	}
	if got := egressVirtualName("f5-cne-system"); got != "f5-cne-system-awsbnkctl-egress-egress-ipv4" {
		t.Errorf("egressVirtualName = %q", got)
	}
}
