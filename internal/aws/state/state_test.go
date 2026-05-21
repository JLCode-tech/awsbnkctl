package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()

	s1, err := Load(dir)
	if err != nil {
		t.Fatalf("Load on empty dir: %v", err)
	}
	s1.Set("VPC_ID", "vpc-0abc123")
	s1.Set("IGW_ID", "igw-0xyz789")
	if err := s1.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	s2, err := Load(dir)
	if err != nil {
		t.Fatalf("Load after save: %v", err)
	}
	if s2.Get("VPC_ID") != "vpc-0abc123" {
		t.Errorf("VPC_ID: got %q, want %q", s2.Get("VPC_ID"), "vpc-0abc123")
	}
	if s2.Get("IGW_ID") != "igw-0xyz789" {
		t.Errorf("IGW_ID: got %q, want %q", s2.Get("IGW_ID"), "igw-0xyz789")
	}
}

func TestLoad_MissingFileReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	// No state.env written yet.
	s, err := Load(dir)
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil State")
	}
	if got := s.Get("ANYTHING"); got != "" {
		t.Errorf("expected empty value, got %q", got)
	}
}

func TestLoad_CorruptFileReturnsError(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.env")
	if err := os.WriteFile(p, []byte("not-kv-line\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for corrupt file, got nil")
	}
}

func TestLoad_IgnoresComments(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.env")
	content := "# this is a comment\nVPC_ID=vpc-001\n# another comment\nIGW_ID=igw-002\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Get("VPC_ID") != "vpc-001" {
		t.Errorf("VPC_ID: got %q", s.Get("VPC_ID"))
	}
	if s.Get("IGW_ID") != "igw-002" {
		t.Errorf("IGW_ID: got %q", s.Get("IGW_ID"))
	}
}

func TestSave_CreatesDir(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "nested", "dir")

	s, _ := Load(dir) // dir doesn't exist yet, Load returns empty State
	s.Set("KEY", "val")
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	s2, err := Load(dir)
	if err != nil {
		t.Fatalf("Load after save: %v", err)
	}
	if s2.Get("KEY") != "val" {
		t.Errorf("KEY: got %q, want val", s2.Get("KEY"))
	}
}

func TestGet_MissingKeyReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	s, _ := Load(dir)
	if got := s.Get("NO_SUCH_KEY"); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}
