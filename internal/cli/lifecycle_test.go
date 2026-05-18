package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestResolveVarFiles_NilAndEmpty covers the two no-op branches.
func TestResolveVarFiles_NilAndEmpty(t *testing.T) {
	t.Parallel()
	got, err := resolveVarFiles(nil)
	if err != nil {
		t.Fatalf("nil input: unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("nil input: want empty, got %v", got)
	}

	got, err = resolveVarFiles([]string{})
	if err != nil {
		t.Fatalf("empty input: unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("empty input: want empty, got %v", got)
	}
}

// TestResolveVarFiles_AbsolutePathPassThrough covers the absolute-path
// branch — we only Clean(), we don't Stat (caller can have set
// --var-file=/path/to/something-that-will-exist-later).
func TestResolveVarFiles_AbsolutePathPassThrough(t *testing.T) {
	t.Parallel()
	// Use a real path that will pass Clean unchanged. Stat is NOT
	// performed on absolute inputs, matching upstream's behaviour.
	abs := "/tmp/awsbnkctl-test-abs/terraform.tfvars"
	got, err := resolveVarFiles([]string{abs})
	if err != nil {
		t.Fatalf("absolute: unexpected error: %v", err)
	}
	if got[0] != abs {
		t.Errorf("absolute: want %q, got %q", abs, got[0])
	}
}

// TestResolveVarFiles_AbsolutePathCleaned ensures filepath.Clean runs on
// absolute inputs (collapses /a/./b and /a//b style noise).
func TestResolveVarFiles_AbsolutePathCleaned(t *testing.T) {
	t.Parallel()
	noisy := "/tmp/awsbnkctl-test//foo/./terraform.tfvars"
	want := "/tmp/awsbnkctl-test/foo/terraform.tfvars"
	got, err := resolveVarFiles([]string{noisy})
	if err != nil {
		t.Fatalf("absolute-clean: unexpected error: %v", err)
	}
	if got[0] != want {
		t.Errorf("absolute-clean: want %q, got %q", want, got[0])
	}
}

// TestResolveVarFiles_RelativeResolvesAgainstCWD is the load-bearing
// case — a user typing `--var-file=./terraform.tfvars` must NOT have
// it resolve against the terraform state-dir CWD downstream.
//
// No t.Parallel() — chdir is process-wide and races with siblings.
func TestResolveVarFiles_RelativeResolvesAgainstCWD(t *testing.T) {
	dir := t.TempDir()
	rel := filepath.Join(dir, "terraform.tfvars")
	if err := os.WriteFile(rel, []byte("x = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// chdir into dir so a relative input resolves to the file we just wrote.
	prev, _ := os.Getwd()
	defer os.Chdir(prev) //nolint:errcheck
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	got, err := resolveVarFiles([]string{"./terraform.tfvars"})
	if err != nil {
		t.Fatalf("relative: unexpected error: %v", err)
	}
	// On some platforms (macOS) the temp dir resolves through a
	// symlink (/var → /private/var). Compare via filepath.EvalSymlinks
	// rather than direct string match.
	gotResolved, _ := filepath.EvalSymlinks(got[0])
	wantResolved, _ := filepath.EvalSymlinks(rel)
	if gotResolved != wantResolved {
		t.Errorf("relative: got %q (resolved %q), want %q (resolved %q)",
			got[0], gotResolved, rel, wantResolved)
	}
}

// TestResolveVarFiles_MissingFile surfaces a clear error including
// BOTH the user input and the resolved absolute. The user has to be
// able to tell *which* path they typed and *where* it resolved to.
//
// No t.Parallel() — chdir is process-wide.
func TestResolveVarFiles_MissingFile(t *testing.T) {
	dir := t.TempDir()
	prev, _ := os.Getwd()
	defer os.Chdir(prev) //nolint:errcheck
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	_, err := resolveVarFiles([]string{"./does-not-exist.tfvars"})
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "./does-not-exist.tfvars") {
		t.Errorf("err should name the user input verbatim: %v", err)
	}
	if !strings.Contains(msg, dir) {
		t.Errorf("err should name the resolved absolute (containing %q): %v", dir, err)
	}
	// Also assert it wraps an os.IsNotExist-style error so callers can
	// switch on it if they want a special message.
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected wrapped os.ErrNotExist, got %v", err)
	}
}

// TestResolveVarFiles_TildeExpansion covers `~` and `~/...` shorthand
// — matches the project convention used elsewhere (e.g. install.go).
func TestResolveVarFiles_TildeExpansion(t *testing.T) {
	t.Parallel()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir on this runner: %v", err)
	}
	// Write a real file under $HOME so Stat doesn't fail.
	dir, err := os.MkdirTemp(home, "awsbnkctl-tilde-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir) //nolint:errcheck
	full := filepath.Join(dir, "v.tfvars")
	if err := os.WriteFile(full, []byte("y = 2\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// `~/<rel>` shorthand — strip $HOME prefix and re-prefix with `~/`.
	relUnderHome := strings.TrimPrefix(full, home+string(filepath.Separator))
	tildePath := "~/" + relUnderHome

	got, err := resolveVarFiles([]string{tildePath})
	if err != nil {
		t.Fatalf("tilde: unexpected error: %v", err)
	}
	if got[0] != full {
		t.Errorf("tilde: want %q, got %q", full, got[0])
	}
}

// TestResolveVarFiles_Idempotent — calling twice on an already-resolved
// slice must be a no-op. Lets us call at composite *and* leaf RunE
// entries without double-wrapping.
func TestResolveVarFiles_Idempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f := filepath.Join(dir, "v.tfvars")
	if err := os.WriteFile(f, []byte("z = 3\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	once, err := resolveVarFiles([]string{f})
	if err != nil {
		t.Fatal(err)
	}
	twice, err := resolveVarFiles(once)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(once) != fmt.Sprint(twice) {
		t.Errorf("not idempotent: once=%v, twice=%v", once, twice)
	}
}

// TestResolveVarFiles_MixedAbsoluteAndRelative — multi-entry chain
// with a mix of absolute and relative paths, all valid. Each resolved
// independently; order preserved.
//
// No t.Parallel() — chdir is process-wide.
func TestResolveVarFiles_MixedAbsoluteAndRelative(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.tfvars")
	b := filepath.Join(dir, "b.tfvars")
	for _, p := range []string{a, b} {
		if err := os.WriteFile(p, []byte("x = 1\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	prev, _ := os.Getwd()
	defer os.Chdir(prev) //nolint:errcheck
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	got, err := resolveVarFiles([]string{a, "./b.tfvars"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2", len(got))
	}
	gotA, _ := filepath.EvalSymlinks(got[0])
	wantA, _ := filepath.EvalSymlinks(a)
	if gotA != wantA {
		t.Errorf("entry 0: got %q, want %q", got[0], a)
	}
	gotB, _ := filepath.EvalSymlinks(got[1])
	wantB, _ := filepath.EvalSymlinks(b)
	if gotB != wantB {
		t.Errorf("entry 1: got %q, want %q", got[1], b)
	}
}
