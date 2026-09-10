package egresssnat

import (
	"testing"

	"github.com/JLCode-tech/awsbnkctl/internal/intent"
	"github.com/JLCode-tech/awsbnkctl/internal/scenarios"
)

// TestTmmVlanFollowsPattern pins the default tunnel VLAN to the cluster's
// interface pattern: single-interface clusters only have ext-vlan (the shape
// examples/egress-demo validated), dual-interface keeps int-vlan, and an
// explicit option always wins.
func TestTmmVlanFollowsPattern(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		opt     string
		want    string
	}{
		{"external-only", intent.PatternExternalOnly, "", "ext-vlan"},
		{"sriov-external", intent.PatternSRIOVExternal, "", "ext-vlan"},
		{"dual-interface", intent.PatternDualInterface, "", "int-vlan"},
		{"host-device alias", intent.PatternHostDevice, "", "int-vlan"},
		{"option wins", intent.PatternExternalOnly, "custom-vlan", "custom-vlan"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sctx := &scenarios.Context{
				Cluster: &intent.Cluster{Pattern: tc.pattern},
				Options: map[string]string{},
			}
			if tc.opt != "" {
				sctx.Options["tmm-int-vlan"] = tc.opt
			}
			if got := tmmIntVlan(sctx); got != tc.want {
				t.Errorf("tmmIntVlan(%s) = %q, want %q", tc.pattern, got, tc.want)
			}
		})
	}
	if got := tmmIntVlan(&scenarios.Context{Options: map[string]string{}}); got != "int-vlan" {
		t.Errorf("nil cluster fallback = %q, want int-vlan", got)
	}
}
