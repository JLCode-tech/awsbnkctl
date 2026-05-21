package cli

// Sprint 10 / PRD 06 §"`status` command integration" — table tests for
// `writeStatusPhaseLines`'s per-shape deployment lines. The four shapes
// (Empty, ClusterOnly, Split, LegacySingle) each have their own
// expected line-set; we stage the existing `internal/config/testdata/`
// fixtures into a temp ROKSBNKCTL_HOME so `config.DetectShape` picks
// them up the same way it does at runtime.
//
// What we assert: presence/absence of the per-phase deployment lines,
// the script-compat `Last apply` line for ShapeLegacySingle, and the
// shape-callout line. We DON'T assert exact timestamps (file mtimes
// drift); the format token `last apply ` distinguishes
// "deployed (last apply …)" from "not deployed".

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JLCode-tech/awsbnkctl/internal/config"
)

// stageStatusWorkspace stages the on-disk state files that DetectShape
// reads, under a per-test ROKSBNKCTL_HOME. Borrows the same fixtures
// the config package's own tfstate_test.go uses so the test surface
// stays in sync without duplicating files.
//
// Each fixture parameter is the basename of a file under
// internal/config/testdata/. Pass "" to skip writing that side (so we
// can build missing-state scenarios).
func stageStatusWorkspace(t *testing.T, trialFixture, clusterFixture string) string {
	t.Helper()
	t.Setenv(config.ROKSBNKCTLHomeEnv, t.TempDir())
	const ws = "status-test"

	if trialFixture != "" {
		dir, err := config.WorkspaceStateDir(ws)
		if err != nil {
			t.Fatal(err)
		}
		writeStatusFixture(t, trialFixture, filepath.Join(dir, "terraform.tfstate"))
	}
	if clusterFixture != "" {
		dir, err := config.WorkspaceClusterStateDir(ws)
		if err != nil {
			t.Fatal(err)
		}
		writeStatusFixture(t, clusterFixture, filepath.Join(dir, "terraform.tfstate"))
	}
	return ws
}

func writeStatusFixture(t *testing.T, fixture, dst string) {
	t.Helper()
	src := filepath.Join("..", "config", "testdata", fixture)
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", src, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(dst), err)
	}
	if err := os.WriteFile(dst, b, 0o600); err != nil {
		t.Fatalf("writing %s: %v", dst, err)
	}
}

// captureStatusPhaseLines is the common harness: stage fixtures, run
// writeStatusPhaseLines, return the captured string.
func captureStatusPhaseLines(t *testing.T, trialFixture, clusterFixture string) string {
	t.Helper()
	ws := stageStatusWorkspace(t, trialFixture, clusterFixture)
	var buf bytes.Buffer
	writeStatusPhaseLines(&buf, ws)
	return buf.String()
}

func TestWriteStatusPhaseLines_Empty(t *testing.T) {
	got := captureStatusPhaseLines(t, "tfstate_empty.json", "tfstate_empty.json")
	if !strings.Contains(got, "Cluster phase:\tnot deployed") {
		t.Errorf("empty: missing cluster-phase 'not deployed' line:\n%s", got)
	}
	if !strings.Contains(got, "BNK trial:\tnot deployed") {
		t.Errorf("empty: missing BNK-trial 'not deployed' line:\n%s", got)
	}
	if strings.Contains(got, "Last apply:") || strings.Contains(got, "Shape:") {
		t.Errorf("empty: should not emit Last apply / Shape lines:\n%s", got)
	}
}

func TestWriteStatusPhaseLines_ClusterOnly(t *testing.T) {
	// trial = empty fixture, cluster = cluster-only fixture → ClusterOnly shape
	got := captureStatusPhaseLines(t, "tfstate_empty.json", "tfstate_cluster_only.json")
	if !strings.Contains(got, "Cluster phase:\tdeployed (last apply") {
		t.Errorf("cluster-only: missing deployed-cluster line:\n%s", got)
	}
	if !strings.Contains(got, "BNK trial:\tnot deployed") {
		t.Errorf("cluster-only: missing BNK-trial 'not deployed' line:\n%s", got)
	}
}

func TestWriteStatusPhaseLines_Split(t *testing.T) {
	got := captureStatusPhaseLines(t, "tfstate_split.json", "tfstate_cluster_only.json")
	if !strings.Contains(got, "Cluster phase:\tdeployed (last apply") {
		t.Errorf("split: missing deployed-cluster line:\n%s", got)
	}
	if !strings.Contains(got, "BNK trial:\tdeployed (last apply") {
		t.Errorf("split: missing deployed-trial line:\n%s", got)
	}
}

func TestWriteStatusPhaseLines_LegacySingle(t *testing.T) {
	// Trial state contains cluster-phase modules → DetectShape classifies
	// as LegacySingle regardless of the cluster-state side.
	got := captureStatusPhaseLines(t, "tfstate_legacy_single.json", "")
	if !strings.Contains(got, "Shape:\tlegacy single-state") {
		t.Errorf("legacy: missing shape callout line:\n%s", got)
	}
	if !strings.Contains(got, "Last apply:") {
		t.Errorf("legacy: missing v1.0.x Last apply line for script-compat:\n%s", got)
	}
	// Sanity: the new per-phase format must NOT appear for legacy
	// (would confuse a v1.0.x parser).
	if strings.Contains(got, "Cluster phase:\tdeployed") {
		t.Errorf("legacy: must NOT emit per-phase deployment line:\n%s", got)
	}
}

func TestWriteStatusPhaseLines_DetectShapeError_FallsBackToLegacy(t *testing.T) {
	// Malformed tfstate → DetectShape returns an error → fallback to
	// the v1.0.x Last apply line so the user still gets *some* signal.
	got := captureStatusPhaseLines(t, "tfstate_malformed.json", "")
	// The fallback writes "Last apply:" either with a real mtime or
	// "(no state — run `awsbnkctl up`)" if Stat fails. We accept either
	// — the assertion is just that we don't emit a per-phase line.
	if !strings.Contains(got, "Last apply:") {
		t.Errorf("error path: expected fallback Last apply line:\n%s", got)
	}
	if strings.Contains(got, "Cluster phase:") || strings.Contains(got, "BNK trial:") {
		t.Errorf("error path: must NOT emit per-phase lines on DetectShape error:\n%s", got)
	}
}

// TestDeployedLine covers both branches of the deployedLine helper —
// readable file (mtime emitted) vs missing file (falls back to
// "not deployed").
func TestDeployedLine(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	missing := filepath.Join(dir, "no-such.tfstate")
	if got := deployedLine(missing); got != "not deployed" {
		t.Errorf("missing file: got %q, want %q", got, "not deployed")
	}

	present := filepath.Join(dir, "real.tfstate")
	if err := os.WriteFile(present, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := deployedLine(present)
	if !strings.HasPrefix(got, "deployed (last apply ") {
		t.Errorf("present file: got %q, want prefix 'deployed (last apply '", got)
	}
}
