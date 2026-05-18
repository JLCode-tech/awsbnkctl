package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// WriteAppliedTFVars writes terraform.applied.tfvars — a snapshot of the
// var-file inputs terraform actually consumed during a successful apply.
//
// Ported from roksbnkctl@6725db1 (their sprint11 / PRD 07). Adapted for
// awsbnkctl's shape: same phase routing (DetectShape is shared), AWS-
// flavoured redaction list, "awsbnkctl" in the header line.
//
// Arguments:
//
//   - workspace — the awsbnkctl workspace name. Used to resolve the
//     per-phase state dir where the snapshot lands.
//   - phase     — one of "cluster", "trial", or "legacy-single". Picks
//     the target state dir and is recorded in the header comment so the
//     reader can disambiguate which phase produced the file.
//   - sources   — ordered slice of var-file paths exactly as passed to
//     `terraform apply -var-file=...`. Each file is read in order; the
//     output section for source[i] preserves terraform's "later wins"
//     semantics implicitly (the reader can grep top-to-bottom and the
//     last occurrence is the value terraform used).
//
// Output file path:
//
//   - phase "cluster"        → <WorkspaceClusterStateDir>/terraform.applied.tfvars
//   - phase "trial"          → <WorkspaceStateDir>/terraform.applied.tfvars
//   - phase "legacy-single"  → <WorkspaceStateDir>/terraform.applied.tfvars
//
// Returns nil on success. Callers log-and-continue on error — the apply
// succeeded, the snapshot is a nice-to-have output.
func WriteAppliedTFVars(workspace, phase string, sources []string) error {
	target, err := appliedTFVarsPath(workspace, phase)
	if err != nil {
		return err
	}

	body, err := renderAppliedTFVars(phase, sources, time.Now().UTC(), appliedTFVarsVersion())
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return fmt.Errorf("creating state dir for applied tfvars: %w", err)
	}

	// Atomic-rename pattern: write to a tempfile in the same dir, then
	// rename. Avoids leaving a half-written snapshot if the process is
	// killed mid-write.
	tmp, err := os.CreateTemp(filepath.Dir(target), ".terraform.applied.tfvars.*")
	if err != nil {
		return fmt.Errorf("creating temp file for applied tfvars: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.WriteString(body); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("writing applied tfvars: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("closing applied tfvars temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("chmod applied tfvars: %w", err)
	}
	if err := os.Rename(tmpPath, target); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming applied tfvars into place: %w", err)
	}
	return nil
}

// AppliedTFVarsPath returns the snapshot path for (workspace, phase)
// without writing anything. Exposed so callers (or tests) can locate
// the file the same way WriteAppliedTFVars would.
func AppliedTFVarsPath(workspace, phase string) (string, error) {
	return appliedTFVarsPath(workspace, phase)
}

func appliedTFVarsPath(workspace, phase string) (string, error) {
	switch phase {
	case "cluster":
		dir, err := WorkspaceClusterStateDir(workspace)
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "terraform.applied.tfvars"), nil
	case "trial", "legacy-single":
		dir, err := WorkspaceStateDir(workspace)
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "terraform.applied.tfvars"), nil
	default:
		// Fallback: treat unknown phases as trial — keeps the snapshot
		// from being lost on unexpected call paths.
		dir, err := WorkspaceStateDir(workspace)
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "terraform.applied.tfvars"), nil
	}
}

// redactedVarNames lists every variable whose value must be replaced
// with "<redacted>" in the snapshot. Upstream redacts `ibmcloud_api_key`;
// for the AWS retarget the equivalent surface is the static-creds
// triple (in case an operator wires them in via --var-file rather than
// the SDK chain). The list is intentionally narrow — adding entries is
// a one-line change, no config knob.
var redactedVarNames = map[string]struct{}{
	"aws_access_key_id":     {},
	"aws_secret_access_key": {},
	"aws_session_token":     {},
}

// tfvarsAssignmentRE matches one HCL-tfvars assignment per line. The
// snapshot only consumes what awsbnkctl writes (terraform.tfvars,
// terraform.tfvars.user) so the surface is constrained: identifier `=`
// value, where value is one of:
//
//   - a double-quoted string (no embedded newlines, no fancy escapes
//     beyond the standard HCL set — awsbnkctl never emits any)
//   - a bare bool / number (true|false|123|1.5)
//
// Anything more exotic (HCL heredocs, multi-line lists, object literals)
// is out of scope — awsbnkctl doesn't emit them, and the user's
// terraform.tfvars.user is documented as line-oriented. Lines that
// don't match are dropped silently.
var tfvarsAssignmentRE = regexp.MustCompile(
	`^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.+?)\s*$`,
)

// tfvarsCommentRE strips trailing `# ...` comments off the value
// portion of an assignment so `foo = "bar" # note` round-trips as
// `foo = "bar"`.
var tfvarsCommentRE = regexp.MustCompile(`\s+#.*$`)

// appliedTFVarsVersion returns the awsbnkctl version string for the
// header comment. Wired by the CLI layer at init via
// SetAppliedTFVarsVersion to avoid an import cycle (config <- cli).
// Falls back to "dev" when unset — tests get "dev" without further
// setup.
func appliedTFVarsVersion() string {
	if appliedTFVarsVersionFn != nil {
		if v := appliedTFVarsVersionFn(); v != "" {
			return v
		}
	}
	return "dev"
}

// appliedTFVarsVersionFn is set by the CLI layer's init() to return
// its build-time Version. Left nil in test binaries that don't import
// the CLI package — those get the "dev" fallback.
var appliedTFVarsVersionFn func() string

// SetAppliedTFVarsVersion wires the CLI's Version through to the
// snapshot header. Called from internal/cli/root.go's init(). Same
// seam pattern as exec.SetToolImageTag.
func SetAppliedTFVarsVersion(fn func() string) {
	appliedTFVarsVersionFn = fn
}

// renderAppliedTFVars builds the snapshot body. Lower-case but callable
// from the test file in the same package so tests can pin a fixed
// timestamp + version without touching the filesystem.
func renderAppliedTFVars(phase string, sources []string, now time.Time, version string) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "# Generated by awsbnkctl %s at %s after terraform apply on phase=%s.\n",
		version, now.Format(time.RFC3339), phase)
	fmt.Fprintln(&b, "# Re-generated each apply. Do not edit by hand — your changes will be overwritten.")
	fmt.Fprintln(&b)

	for _, src := range sources {
		label := sourceLabel(src)
		assigns, missing, err := readTFVarsAssignments(src)
		if err != nil {
			return "", err
		}
		if missing {
			fmt.Fprintf(&b, "# === from %s (missing) ===\n", label)
			fmt.Fprintln(&b)
			continue
		}
		fmt.Fprintf(&b, "# === from %s ===\n", label)

		keys := make([]string, 0, len(assigns))
		for k := range assigns {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			if _, redact := redactedVarNames[k]; redact {
				fmt.Fprintf(&b, "%s = \"<redacted>\"  # source: cred chain, not persisted\n", k)
				continue
			}
			fmt.Fprintf(&b, "%s = %s\n", k, assigns[k])
		}
		fmt.Fprintln(&b)
	}
	return b.String(), nil
}

// readTFVarsAssignments reads a tfvars file and returns the assignments
// as name → raw-value strings (the value half is kept verbatim from the
// source — quoted strings retain their quotes, bare bools/numbers stay
// bare). The boolean second return is true when the file was missing
// (not an error; the snapshot is best-effort and the caller emits a
// "missing" section marker so the reader sees that source was
// unavailable).
func readTFVarsAssignments(path string) (map[string]string, bool, error) {
	b, err := os.ReadFile(path) // #nosec G304 -- path comes from varFiles() / appliedTFVarsPath(), workspace-resolved
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "warning: tfvars source %q is missing — skipping in applied snapshot\n", path)
			return nil, true, nil
		}
		return nil, false, fmt.Errorf("reading tfvars source %s: %w", path, err)
	}

	out := make(map[string]string)
	for _, raw := range strings.Split(string(b), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		m := tfvarsAssignmentRE.FindStringSubmatch(line)
		if m == nil {
			// Line doesn't match the supported "name = value" shape
			// (HCL heredoc, multi-line list, etc.). Skip silently —
			// the snapshot is best-effort and awsbnkctl never emits
			// these shapes itself.
			continue
		}
		name := m[1]
		value := tfvarsCommentRE.ReplaceAllString(m[2], "")
		out[name] = value
	}
	return out, false, nil
}

// sourceLabel maps a var-file path to a human-friendly label used in
// the snapshot's section header comments. The mapping is intentionally
// keyed on the basename so the same label survives whether the path is
// absolute or relative.
func sourceLabel(path string) string {
	base := filepath.Base(path)
	switch base {
	case "terraform.tfvars":
		return "config.yaml"
	case "terraform.tfvars.user":
		return "terraform.tfvars.user"
	default:
		return base
	}
}
