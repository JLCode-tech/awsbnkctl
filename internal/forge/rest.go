package forge

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// RegisterREST mirrors Register's shape but uses forge's REST API instead of
// MCP. Used as a fallback when the MCP catalog does not expose create_project
// or create_cluster (catalog-gap detection in Phase09).
//
// Credentials are hardcoded admin/changeme — forge localhost dev integration.
// Real forge auth is out of scope for slice 4.
func RegisterREST(ctx context.Context, restURL string, req RegisterRequest) (RegisterResult, error) {
	if req.WorkspaceName == "" {
		return RegisterResult{}, fmt.Errorf("forge.RegisterREST: workspace name is required")
	}
	if req.WorkspaceDir == "" {
		return RegisterResult{}, fmt.Errorf("forge.RegisterREST: workspace dir is required")
	}
	if req.ClusterName == "" {
		return RegisterResult{}, fmt.Errorf("forge.RegisterREST: cluster name is required")
	}
	if len(req.Kubeconfig) == 0 {
		return RegisterResult{}, fmt.Errorf("forge.RegisterREST: kubeconfig is empty")
	}
	if req.ProjectName == "" {
		req.ProjectName = "awsbnkctl-" + req.WorkspaceName
	}

	base := strings.TrimRight(restURL, "/")

	token, err := restLogin(ctx, base)
	if err != nil {
		return RegisterResult{}, fmt.Errorf("forge REST login: %w", err)
	}

	proj, err := restCreateProject(ctx, base, token, req)
	if err != nil {
		return RegisterResult{}, fmt.Errorf("forge REST create project: %w", err)
	}

	cluster, err := restCreateCluster(ctx, base, token, proj.ID, req)
	if err != nil {
		// Best-effort rollback.
		_ = restDeleteProject(ctx, base, token, proj.ID)
		return RegisterResult{}, fmt.Errorf("forge REST create cluster: %w", err)
	}

	link := &Link{
		ForgeURL:     restURL,
		ProjectID:    proj.ID,
		ProjectName:  proj.Name,
		ClusterID:    cluster.ID,
		ClusterName:  cluster.Name,
		RegisteredAt: time.Now().UTC(),
		Workspace:    req.WorkspaceName,
		Status:       "registered",
	}
	if err := WriteLink(req.WorkspaceDir, link); err != nil {
		return RegisterResult{Link: link, ForgeURL: restURL},
			fmt.Errorf("registration succeeded but writing forge_link.json failed: %w", err)
	}
	return RegisterResult{Link: link, ForgeURL: restURL}, nil
}

// UnregisterREST tears down the forge-side registration via REST.
// Tolerates 404 responses (operator may have cleaned up via forge UI).
func UnregisterREST(ctx context.Context, restURL string, link *Link) error {
	if link == nil {
		return fmt.Errorf("forge.UnregisterREST: link is nil")
	}
	base := strings.TrimRight(restURL, "/")
	token, err := restLogin(ctx, base)
	if err != nil {
		return fmt.Errorf("forge REST login: %w", err)
	}

	if err := restDeleteCluster(ctx, base, token, link.ProjectID, link.ClusterID); err != nil && !is404(err) {
		return fmt.Errorf("forge REST delete cluster: %w", err)
	}
	return nil
}

// IsMCPCatalogGapErr returns true when err indicates the MCP catalog does not
// expose the requested tool — i.e., the forge MCP server is running an older
// version that pre-dates create_project / create_cluster tools.
// Exported so Phase09 (in the phases package) can use it.
func IsMCPCatalogGapErr(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "tool not found") ||
		strings.Contains(s, "unknown tool") ||
		strings.Contains(s, "method not found") ||
		strings.Contains(s, "no tool named") ||
		strings.Contains(s, "tool_not_found")
}

// ── REST helpers ──────────────────────────────────────────────────────────────

func restLogin(ctx context.Context, base string) (string, error) {
	body := map[string]string{"username": "admin", "password": "changeme"}
	var resp struct {
		Token string `json:"token"`
	}
	if err := restPost(ctx, base+"/api/auth/login", "", body, &resp); err != nil {
		return "", err
	}
	if resp.Token == "" {
		return "", fmt.Errorf("forge REST login: empty token in response")
	}
	return resp.Token, nil
}

type restProject struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func restCreateProject(ctx context.Context, base, token string, req RegisterRequest) (restProject, error) {
	body := map[string]any{
		"name":           req.ProjectName,
		"project_type":   "cloud-aws",
		"cloud_provider": "aws",
		"region":         req.Region,
		"environment":    "dev",
		"description":    fmt.Sprintf("Created by awsbnkctl for workspace %q", req.WorkspaceName),
	}
	// Forge has THREE response shapes in the wild for POST /api/projects:
	//   A. wrapped:     {project: {id, name, ...}, success: true}  (>=2.10.x MCP tool docs)
	//   B. flat-id:     {id, name, ...}
	//   C. flat-prefix: {success, project_id, name, message}       (localhost dev, verified 2026-05-21)
	// Probe all three and use whichever has a non-zero ID.
	var resp struct {
		ID        int         `json:"id"`
		ProjectID int         `json:"project_id"`
		Name      string      `json:"name"`
		Project   restProject `json:"project"`
		Success   bool        `json:"success"`
	}
	if err := restPost(ctx, base+"/api/projects", token, body, &resp); err != nil {
		return restProject{}, err
	}
	if resp.Project.ID != 0 {
		return resp.Project, nil
	}
	if resp.ID != 0 {
		return restProject{ID: resp.ID, Name: resp.Name}, nil
	}
	if resp.ProjectID != 0 {
		return restProject{ID: resp.ProjectID, Name: resp.Name}, nil
	}
	return restProject{}, fmt.Errorf("forge REST create project: no project ID in response (tried wrapped, flat-id, project_id shapes)")
}

type restCluster struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func restCreateCluster(ctx context.Context, base, token string, projectID int, req RegisterRequest) (restCluster, error) {
	// Forge REST expects the kubeconfig BASE64-ENCODED (verified against
	// localhost forge 2026-05-21: raw YAML returns "Invalid base64
	// kubeconfig: Incorrect padding"). Encode here, not at the caller,
	// so the MCP path (which sends raw bytes per the existing client)
	// is unaffected.
	body := map[string]any{
		"name":           req.ClusterName,
		"cloud_provider": "aws",
		"region":         req.Region,
		"kubeconfig":     base64.StdEncoding.EncodeToString(req.Kubeconfig),
	}
	// Same three-shape tolerance as restCreateProject (wrapped, flat-id,
	// flat-prefix).
	var resp struct {
		ID        int         `json:"id"`
		ClusterID int         `json:"cluster_id"`
		Name      string      `json:"name"`
		Cluster   restCluster `json:"cluster"`
		Success   bool        `json:"success"`
	}
	url := fmt.Sprintf("%s/api/projects/%d/k8s/clusters", base, projectID)
	if err := restPost(ctx, url, token, body, &resp); err != nil {
		return restCluster{}, err
	}
	if resp.Cluster.ID != 0 {
		return resp.Cluster, nil
	}
	if resp.ID != 0 {
		return restCluster{ID: resp.ID, Name: resp.Name}, nil
	}
	if resp.ClusterID != 0 {
		return restCluster{ID: resp.ClusterID, Name: resp.Name}, nil
	}
	return restCluster{}, fmt.Errorf("forge REST create cluster: no cluster ID in response (tried wrapped, flat-id, cluster_id shapes)")
}

func restDeleteCluster(ctx context.Context, base, token string, projectID, clusterID int) error {
	url := fmt.Sprintf("%s/api/projects/%d/k8s/clusters/%d", base, projectID, clusterID)
	return restDelete(ctx, url, token)
}

func restDeleteProject(ctx context.Context, base, token string, projectID int) error {
	url := fmt.Sprintf("%s/api/projects/%d", base, projectID)
	return restDelete(ctx, url, token)
}

// restPost sends a POST request with JSON body and decodes the JSON response
// into out.
func restPost(ctx context.Context, url, token string, body, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("http POST %s: %w", url, err)
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("http %d from %s: %s", resp.StatusCode, url, truncateREST(string(respBytes), 400))
	}
	if out != nil {
		if err := json.Unmarshal(respBytes, out); err != nil {
			return fmt.Errorf("decode response from %s: %w", url, err)
		}
	}
	return nil
}

// restDelete sends a DELETE request and tolerates 404.
func restDelete(ctx context.Context, url, token string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("http DELETE %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("http 404 from %s", url)
	}
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http %d from %s: %s", resp.StatusCode, url, truncateREST(string(b), 400))
	}
	return nil
}

func truncateREST(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
