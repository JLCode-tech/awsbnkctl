package forge

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteReadLink_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	want := &Link{
		ForgeMCPURL:  "http://localhost:8081/mcp/",
		ProjectID:    7,
		ProjectName:  "awsbnkctl-default",
		ClusterID:    42,
		ClusterName:  "bnk-prod",
		RegisteredAt: time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC),
		Workspace:    "default",
	}
	if err := WriteLink(dir, want); err != nil {
		t.Fatalf("WriteLink: %v", err)
	}
	got, err := ReadLink(dir)
	if err != nil {
		t.Fatalf("ReadLink: %v", err)
	}
	if got.ProjectID != want.ProjectID ||
		got.ClusterID != want.ClusterID ||
		got.ProjectName != want.ProjectName ||
		!got.RegisteredAt.Equal(want.RegisteredAt) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestReadLink_Missing(t *testing.T) {
	dir := t.TempDir()
	if _, err := ReadLink(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected ErrNotExist, got %v", err)
	}
}

func TestRemoveLink_Idempotent(t *testing.T) {
	dir := t.TempDir()
	// Removing a missing link must not error.
	if err := RemoveLink(dir); err != nil {
		t.Fatalf("RemoveLink on empty dir: %v", err)
	}
	// Now create one and remove it; file must be gone afterward.
	if err := WriteLink(dir, &Link{Workspace: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := RemoveLink(dir); err != nil {
		t.Fatalf("RemoveLink: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, LinkFileName)); !os.IsNotExist(err) {
		t.Errorf("link file still present after remove: %v", err)
	}
}

func TestWriteLink_Atomic(t *testing.T) {
	// No half-written file: the temp file is renamed into place.
	// We can't easily simulate a crash mid-write, but we can verify
	// no leftover .tmp files after a successful write.
	dir := t.TempDir()
	if err := WriteLink(dir, &Link{Workspace: "x"}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if name == LinkFileName {
			continue
		}
		t.Errorf("unexpected leftover file %q after WriteLink", name)
	}
}
