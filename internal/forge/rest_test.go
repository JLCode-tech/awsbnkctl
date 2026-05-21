package forge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// forgeRESTServer is a minimal httptest server that serves the forge REST
// endpoints needed by RegisterREST and UnregisterREST.
type forgeRESTServer struct {
	// authFail causes the /api/auth/login endpoint to return 401.
	authFail bool
	// projectFail causes POST /api/projects to return 500.
	projectFail bool
	// clusterFail causes POST /api/projects/{id}/k8s/clusters to return 500.
	clusterFail bool
	// calls records the endpoint+method pairs called.
	calls []string
}

func (s *forgeRESTServer) handler(w http.ResponseWriter, r *http.Request) {
	s.calls = append(s.calls, r.Method+" "+r.URL.Path)
	w.Header().Set("Content-Type", "application/json")

	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/api/auth/login":
		if s.authFail {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"token": "test-token"})

	case r.Method == http.MethodPost && r.URL.Path == "/api/projects":
		if s.projectFail {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"project": map[string]any{"id": 11, "name": "awsbnkctl-default"},
			"success": true,
		})

	case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/k8s/clusters"):
		if s.clusterFail {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"cluster": map[string]any{"id": 99, "name": "bnk-prod"},
			"success": true,
		})

	case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/k8s/clusters/"):
		w.WriteHeader(http.StatusNoContent)

	case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/api/projects/"):
		w.WriteHeader(http.StatusNoContent)

	default:
		http.NotFound(w, r)
	}
}

func TestRegisterREST_HappyPath(t *testing.T) {
	srv := &forgeRESTServer{}
	ts := httptest.NewServer(http.HandlerFunc(srv.handler))
	defer ts.Close()

	dir := t.TempDir()
	res, err := RegisterREST(context.Background(), ts.URL, RegisterRequest{
		WorkspaceName: "default",
		WorkspaceDir:  dir,
		ClusterName:   "bnk-prod",
		Region:        "us-east-1",
		Kubeconfig:    []byte("apiVersion: v1\nkind: Config\n"),
	})
	if err != nil {
		t.Fatalf("RegisterREST: %v", err)
	}
	if res.Link == nil {
		t.Fatal("result.Link is nil")
	}
	if res.Link.ProjectID != 11 {
		t.Errorf("ProjectID = %d, want 11", res.Link.ProjectID)
	}
	if res.Link.ClusterID != 99 {
		t.Errorf("ClusterID = %d, want 99", res.Link.ClusterID)
	}
	if res.Link.Status != "registered" {
		t.Errorf("Status = %q, want %q", res.Link.Status, "registered")
	}
	// Link file written.
	if _, err := ReadLink(dir); err != nil {
		t.Errorf("link file not written: %v", err)
	}
}

func TestRegisterREST_AuthFailure(t *testing.T) {
	srv := &forgeRESTServer{authFail: true}
	ts := httptest.NewServer(http.HandlerFunc(srv.handler))
	defer ts.Close()

	_, err := RegisterREST(context.Background(), ts.URL, RegisterRequest{
		WorkspaceName: "default",
		WorkspaceDir:  t.TempDir(),
		ClusterName:   "bnk-prod",
		Kubeconfig:    []byte("k"),
	})
	if err == nil {
		t.Fatal("expected error on auth failure, got nil")
	}
	if !strings.Contains(err.Error(), "login") {
		t.Errorf("expected login error, got: %v", err)
	}
}

func TestRegisterREST_ProjectCreationFailure(t *testing.T) {
	srv := &forgeRESTServer{projectFail: true}
	ts := httptest.NewServer(http.HandlerFunc(srv.handler))
	defer ts.Close()

	_, err := RegisterREST(context.Background(), ts.URL, RegisterRequest{
		WorkspaceName: "default",
		WorkspaceDir:  t.TempDir(),
		ClusterName:   "bnk-prod",
		Kubeconfig:    []byte("k"),
	})
	if err == nil {
		t.Fatal("expected error on project failure, got nil")
	}
}

func TestRegisterREST_ClusterCreationFailure(t *testing.T) {
	srv := &forgeRESTServer{clusterFail: true}
	ts := httptest.NewServer(http.HandlerFunc(srv.handler))
	defer ts.Close()

	_, err := RegisterREST(context.Background(), ts.URL, RegisterRequest{
		WorkspaceName: "default",
		WorkspaceDir:  t.TempDir(),
		ClusterName:   "bnk-prod",
		Kubeconfig:    []byte("k"),
	})
	if err == nil {
		t.Fatal("expected error on cluster failure, got nil")
	}
}

func TestUnregisterREST_404Tolerated(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/auth/login" {
			_ = json.NewEncoder(w).Encode(map[string]string{"token": "tok"})
			return
		}
		// Simulate forge already cleaned up — return 404.
		http.NotFound(w, r)
	}))
	defer ts.Close()

	link := &Link{ProjectID: 11, ClusterID: 99}
	if err := UnregisterREST(context.Background(), ts.URL, link); err != nil {
		t.Fatalf("UnregisterREST should tolerate 404, got: %v", err)
	}
}

func TestIsMCPCatalogGap(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		{"tool not found: create_project", true},
		{"unknown tool create_cluster", true},
		{"method not found", true},
		{"no tool named foo", true},
		{"tool_not_found", true},
		{"connection refused", false},
		{"http 500: internal server error", false},
		{"", false},
	}
	for _, tc := range cases {
		var err error
		if tc.msg != "" {
			err = &testErr{tc.msg}
		}
		if got := IsMCPCatalogGapErr(err); got != tc.want {
			t.Errorf("IsMCPCatalogGapErr(%q) = %v, want %v", tc.msg, got, tc.want)
		}
	}
}

type testErr struct{ msg string }

func (e *testErr) Error() string { return e.msg }
