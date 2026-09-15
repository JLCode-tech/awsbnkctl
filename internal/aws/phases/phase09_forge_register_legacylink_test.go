package phases

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JLCode-tech/awsbnkctl/internal/aws/awsmw"
	"github.com/JLCode-tech/awsbnkctl/internal/aws/state"
	"github.com/JLCode-tech/awsbnkctl/internal/forge"
)

// A link written by the MCP register path has no status field (Status "" ==
// registered per Link.IsRegistered). Down must still take the MCP delete
// path with the recorded ids; REST here always fails so a by-name detour
// would leave the link in place.
func TestPhase09ForgeRegisterDown_LegacyLinkUsesMCP(t *testing.T) {
	awsmw.ResetForTest()
	mcp := newScriptedMCP()
	mcpSrv := httptest.NewServer(http.HandlerFunc(mcp.handler))
	defer mcpSrv.Close()
	restSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rest unavailable", http.StatusInternalServerError)
	}))
	defer restSrv.Close()

	dir := t.TempDir()
	cl := forgeEnabledCluster(mcpSrv.URL+"/mcp/", restSrv.URL)
	st, _ := state.Load(dir)
	restoreWd := chdirTemp(t, dir)
	defer restoreWd()
	if err := forge.WriteLink(cl.StateDir(), &forge.Link{ProjectID: 11, ClusterID: 99, ForgeURL: restSrv.URL, ForgeMCPURL: mcpSrv.URL + "/mcp/"}); err != nil {
		t.Fatal(err)
	}
	clients := &Clients{Profile: "test", ForgeClient: forge.NewClient(mcpSrv.URL + "/mcp/")}
	if err := Phase09ForgeRegisterDown(context.Background(), cl, st, clients, false); err != nil {
		t.Fatal(err)
	}
	if _, err := forge.ReadLink(cl.StateDir()); err == nil {
		t.Fatal("link still present: down did not unregister the MCP-registered cluster (Status \"\" treated as pending)")
	}
}
