package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JLCode-tech/awsbnkctl/internal/forge"
)

func resetTargetsScanFlags() {
	flagTargetsScanConfig = ""
	flagTargetsScanRegister = true
}

func TestTargetsScanRegistered(t *testing.T) {
	names := map[string]bool{}
	for _, sub := range targetsCmd.Commands() {
		names[sub.Name()] = true
	}
	for _, want := range []string{"scan", "list", "show", "add", "remove"} {
		if !names[want] {
			t.Errorf("targets %s not registered (have %v)", want, names)
		}
	}
	for _, flag := range []string{"config", "register", "kubeconfig", "probe", "bearer-env", "forge-rest-url", "forge-user", "forge-pass", "controller-namespace", "timeout"} {
		if targetsScanCmd.Flags().Lookup(flag) == nil {
			t.Errorf("targets scan --%s missing", flag)
		}
	}
	if benchmarkSetupCmd.Flags().Lookup("auto-discover") == nil {
		t.Error("benchmark setup --auto-discover missing")
	}
}

func TestTargetsScan_TextWithoutForgeLink(t *testing.T) {
	dyn, cs := fakeScanClients(t, scanFixture, 1)
	withScanSeams(t, dyn, cs, nil)
	resetTargetsScanFlags()

	var out bytes.Buffer
	targetsScanCmd.SetOut(&out)
	if err := runTargetsScan(targetsScanCmd, nil); err != nil {
		t.Fatalf("run: %v\n%s", err, out.String())
	}
	for _, want := range []string{
		"NAME", "mcp-default-tools", "default/gw", "http://10.0.10.150/mcp", "unknown",
		"not registered with Forge",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "forge targets") {
		t.Errorf("no registration without a link:\n%s", out.String())
	}
}

func TestTargetsScan_RegistersIdempotently(t *testing.T) {
	dyn, cs := fakeScanClients(t, scanFixture, 1)
	withScanSeams(t, dyn, cs, &forge.Link{ClusterID: 23, ClusterName: "demo", ProjectID: 1, Status: "registered"})
	resetTargetsScanFlags()

	// Fake Forge: the target already exists, so the create returns 409 and
	// the lookup by name resolves it.
	creates, lookups := 0, 0
	rest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]string{"token": "t"})
		case r.Method == http.MethodPost && r.URL.Path == forge.BenchmarkTargetEndpoint:
			creates++
			var posted map[string]any
			_ = json.NewDecoder(r.Body).Decode(&posted)
			if posted["name"] != "mcp-default-tools" || posted["cluster_id"] != float64(23) {
				t.Errorf("posted = %v", posted)
			}
			tags, _ := posted["tags"].(map[string]any)
			if tags["protocol"] != "mcp" || tags["cluster"] != "demo" {
				t.Errorf("tags = %v", tags)
			}
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{"detail": "exists"})
		case r.Method == http.MethodGet && r.URL.Path == forge.BenchmarkTargetEndpoint:
			lookups++
			_ = json.NewEncoder(w).Encode(map[string]any{"targets": []map[string]any{{"id": 7, "name": "mcp-default-tools", "cluster_id": 23}}, "total": 1})
		default:
			http.NotFound(w, r)
		}
	}))
	defer rest.Close()
	flagForgeScanRestURL = rest.URL
	flagForgeScanUser, flagForgeScanPass = "u", "p"
	flagOutput = "json"

	var out bytes.Buffer
	targetsScanCmd.SetOut(&out)
	if err := runTargetsScan(targetsScanCmd, nil); err != nil {
		t.Fatalf("run: %v\n%s", err, out.String())
	}
	var doc targetsScanOutput
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("json: %v\n%s", err, out.String())
	}
	if len(doc.Endpoints) != 1 || len(doc.Targets) != 1 || doc.Hint != "" {
		t.Fatalf("doc = %+v", doc)
	}
	if tg := doc.Targets[0]; tg.Error != "" || tg.ID != 7 || tg.Name != "mcp-default-tools" {
		t.Errorf("target = %+v", tg)
	}
	if creates != 1 || lookups < 1 {
		t.Errorf("creates=%d lookups=%d", creates, lookups)
	}
}

func TestTargetsScan_RegisterDisabledAndNoEndpoints(t *testing.T) {
	dyn, cs := fakeScanClients(t, strings.Split(scanFixture, "kind: HTTPRoute")[0], 1)
	withScanSeams(t, dyn, cs, &forge.Link{ClusterID: 23, Status: "registered"})
	resetTargetsScanFlags()
	flagTargetsScanRegister = false

	var out bytes.Buffer
	targetsScanCmd.SetOut(&out)
	if err := runTargetsScan(targetsScanCmd, nil); err != nil {
		t.Fatalf("run: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "no MCP endpoints (BNK 2.4, 0 HTTPRoute(s))") {
		t.Errorf("output:\n%s", out.String())
	}
}

func TestSummarizeTargets(t *testing.T) {
	if got := summarizeTargets(nil); got != "none" {
		t.Errorf("nil = %q", got)
	}
	ts := []forgeScanTarget{{ID: 1}, {ID: 2}, {Error: "boom"}}
	if got := summarizeTargets(ts); got != "2 registered, 1 failed" {
		t.Errorf("mixed = %q", got)
	}
	if got := summarizeTargets(ts[:2]); got != "2 registered" {
		t.Errorf("ok = %q", got)
	}
}
