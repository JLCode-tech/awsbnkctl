package doctor

// Doctor green-by-default refresh tests.
//
// Contract: `awsbnkctl doctor` exits 0 with zero warnings on a stock
// dev box that has ONLY `terraform` AND `helm` installed. Every
// previously-required-or-warned tool (kubectl, iperf3, dig) is now
// internalised in the binary (kubectl via client-go in awsbnkctl k *;
// iperf3 via the in-cluster fixture; dig via miekg/dns).
//
// These tests pin the contract so a future refactor can't silently
// regress.

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/JLCode-tech/awsbnkctl/internal/config"
)

// TestRunWithWhy_StockDevBox_NoWorkspace asserts: on a host with
// (potentially) no kubectl/iperf3/dig and no workspace initialised,
// the doctor emits no StatusError rows. The only "required" tools
// are terraform + helm; if either isn't on the host, this test
// naturally returns an error row for it — those are the intentional
// hard fails.
//
// AWS-row block (PRD 04): the `aws credentials` row is StatusWarning
// (naming the missing AWS_PROFILE / AWS_ACCESS_KEY_ID env var) and
// every downstream AWS row (sts / eks / ec2 / s3 / iam) is
// StatusSkipped.
//
// The test passes a nil-Workspace config.Context so the workspace +
// IAM-auth checks degrade to "no workspace" / "no credentials"
// warnings, not errors. This is the "fresh dev box, before
// `awsbnkctl init`" shape.
func TestRunWithWhy_StockDevBox_NoWorkspace(t *testing.T) {
	cctx := &config.Context{WorkspaceName: "test-stock-dev"}
	pairs := runWithWhy(context.Background(), cctx)

	for _, p := range pairs {
		switch p.Check.Status {
		case StatusError:
			// The ONLY allowed StatusError rows are `terraform` and
			// `helm` (both required), and only on a host that doesn't
			// actually have the respective binary installed. Every
			// other tool is informational — a StatusError row from
			// kubectl/iperf3/dig indicates a green-by-default
			// regression.
			if p.Check.Name != "terraform" && p.Check.Name != "helm" {
				t.Errorf("non-required check %q is StatusError: %s — Sprint 6 green-by-default contract violated",
					p.Check.Name, p.Check.Detail)
			}
		case StatusWarning:
			// Allowed warnings: `workspace` (unchanged) +
			// `aws credentials` (Sprint 3 visibility relaxation —
			// closes Sprint 2 tech-writer Issue 4).
			switch p.Check.Name {
			case "workspace", "aws credentials":
				// expected
			default:
				t.Errorf("unexpected StatusWarning on %q: %s — Sprint 3 contract allows 'workspace' + 'aws credentials' warnings only",
					p.Check.Name, p.Check.Detail)
			}
		case StatusSkipped:
			// Downstream AWS rows (sts / eks / ec2 / s3 / iam) are
			// Skipped when `aws credentials` failed; that's the
			// documented degrade. Any other Skipped row is a
			// regression.
			if !strings.HasPrefix(p.Check.Name, "aws ") {
				t.Errorf("unexpected StatusSkipped on %q: %s — only `aws *` rows may be Skipped on a stock dev box",
					p.Check.Name, p.Check.Detail)
			}
		}
	}
}

// TestRunWithWhy_InformationalTools_OK pins that every previously-
// optional tool (kubectl / iperf3 / dig) renders as StatusOK
// regardless of whether it's installed. A missing informational tool
// produces StatusOK with an explanatory detail; a present tool
// produces StatusOK with the path.
func TestRunWithWhy_InformationalTools_OK(t *testing.T) {
	cctx := &config.Context{WorkspaceName: "test-info"}
	pairs := runWithWhy(context.Background(), cctx)

	informationalNames := map[string]bool{
		"kubectl": true, "iperf3": true, "dig": true,
	}
	for _, p := range pairs {
		if !informationalNames[p.Check.Name] {
			continue
		}
		if p.Check.Status != StatusOK {
			t.Errorf("informational tool %q: got Status=%s (detail=%q), want StatusOK",
				p.Check.Name, p.Check.Status, p.Check.Detail)
		}
		if !p.Check.Optional {
			t.Errorf("informational tool %q: Optional=%v, want true", p.Check.Name, p.Check.Optional)
		}
	}
}

// TestRunWithWhy_TerraformIsRequired pins the inverse: `terraform`
// remains a required tool, so a host that lacks it surfaces a
// StatusError. We can only verify this when the test host doesn't
// have terraform installed; skip otherwise.
func TestRunWithWhy_TerraformIsRequired(t *testing.T) {
	if _, err := exec.LookPath("terraform"); err == nil {
		t.Skip("terraform IS on PATH; the missing-required test path can't run on this host")
	}
	cctx := &config.Context{WorkspaceName: "test-tf-required"}
	pairs := runWithWhy(context.Background(), cctx)

	var found bool
	for _, p := range pairs {
		if p.Check.Name == "terraform" {
			found = true
			if p.Check.Status != StatusError {
				t.Errorf("terraform missing → got Status=%s, want StatusError", p.Check.Status)
			}
			if p.Check.Optional {
				t.Errorf("terraform check should be required (Optional=false); got Optional=true")
			}
			if !strings.Contains(strings.ToLower(p.Check.Detail), "not on path") {
				t.Errorf("terraform missing detail = %q; want substring 'not on PATH'", p.Check.Detail)
			}
		}
	}
	if !found {
		t.Errorf("terraform check missing from doctor output")
	}
}

// TestRunWithWhy_HelmIsRequired mirrors TerraformIsRequired for the
// second hard-required tool. Added in v1.0.2 after a live e2e Phase B1
// run revealed the terraform `null_resource` + `local-exec`
// provisioners (cert_manager / flo / cne_instance modules) shell out
// to host `helm` — without it, `awsbnkctl up` fails with
// `exit status 127 — helm: not found` deep into the cluster lifecycle.
// Skip on hosts that have helm installed.
func TestRunWithWhy_HelmIsRequired(t *testing.T) {
	if _, err := exec.LookPath("helm"); err == nil {
		t.Skip("helm IS on PATH; the missing-required test path can't run on this host")
	}
	cctx := &config.Context{WorkspaceName: "test-helm-required"}
	pairs := runWithWhy(context.Background(), cctx)

	var found bool
	for _, p := range pairs {
		if p.Check.Name == "helm" {
			found = true
			if p.Check.Status != StatusError {
				t.Errorf("helm missing → got Status=%s, want StatusError", p.Check.Status)
			}
			if p.Check.Optional {
				t.Errorf("helm check should be required (Optional=false); got Optional=true")
			}
			if !strings.Contains(strings.ToLower(p.Check.Detail), "not on path") {
				t.Errorf("helm missing detail = %q; want substring 'not on PATH'", p.Check.Detail)
			}
		}
	}
	if !found {
		t.Errorf("helm check missing from doctor output")
	}
}

// TestHasFailures_StockDevBoxGreen asserts the exit-code semantic:
// a stock dev box (with `terraform` AND `helm` present) produces no
// HasFailures-reported failures, so `awsbnkctl doctor` exits 0.
//
// This is the contract from PLAN.md §"Gate to Sprint 7" line 481.
// helm was added to the required set in v1.0.2.
// We can only assert it on a host that DOES have both terraform and
// helm installed; skip otherwise.
func TestHasFailures_StockDevBoxGreen(t *testing.T) {
	if _, err := exec.LookPath("terraform"); err != nil {
		t.Skip("terraform NOT on PATH; the green-by-default scenario can't run on this host")
	}
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm NOT on PATH; the green-by-default scenario can't run on this host")
	}
	cctx := &config.Context{WorkspaceName: "test-green"}
	pairs := runWithWhy(context.Background(), cctx)
	checks := make([]Check, 0, len(pairs))
	for _, p := range pairs {
		checks = append(checks, p.Check)
	}
	if HasFailures(checks) {
		var failing []string
		for _, c := range checks {
			if c.Status == StatusError {
				failing = append(failing, c.Name+": "+c.Detail)
			}
		}
		t.Errorf("stock dev box (terraform present, no workspace) should have no failures; failing: %v", failing)
	}
}
