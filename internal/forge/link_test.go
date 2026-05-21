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

func TestLink_IsRegistered(t *testing.T) {
	cases := []struct {
		status string
		want   bool
	}{
		{"", true},           // backward compat: empty = registered
		{"registered", true}, // explicit registered
		{"pending", false},   // pending = not registered
		{"other", false},     // unknown = not registered
	}
	for _, tc := range cases {
		l := &Link{Status: tc.status}
		if got := l.IsRegistered(); got != tc.want {
			t.Errorf("IsRegistered(%q) = %v, want %v", tc.status, got, tc.want)
		}
	}
}

func TestWriteReadLink_StatusRoundtrip(t *testing.T) {
	dir := t.TempDir()
	want := &Link{
		ProjectID: 5,
		ClusterID: 10,
		Workspace: "test",
		Status:    "pending",
	}
	if err := WriteLink(dir, want); err != nil {
		t.Fatalf("WriteLink: %v", err)
	}
	got, err := ReadLink(dir)
	if err != nil {
		t.Fatalf("ReadLink: %v", err)
	}
	if got.Status != "pending" {
		t.Errorf("Status = %q, want %q", got.Status, "pending")
	}
	if got.IsRegistered() {
		t.Error("IsRegistered() should return false for pending link")
	}
}

func TestReadLink_BackwardCompatNoStatus(t *testing.T) {
	// Simulate a link file written before slice 4 (no "status" field).
	dir := t.TempDir()
	// Write a JSON blob without a status field.
	legacyJSON := []byte(`{"project_id":7,"cluster_id":42,"workspace":"default"}` + "\n")
	if err := os.WriteFile(LinkPath(dir), legacyJSON, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := ReadLink(dir)
	if err != nil {
		t.Fatalf("ReadLink: %v", err)
	}
	if got.Status != "" {
		t.Errorf("Status = %q, want empty string for backward compat", got.Status)
	}
	if !got.IsRegistered() {
		t.Error("IsRegistered() should return true for link with no status field")
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
