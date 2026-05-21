package tags

import (
	"testing"
)

func TestRequired_HasMandatoryKeys(t *testing.T) {
	m := Required("my-cluster", CompVPC)

	keys := []string{KeyCluster, KeyComponent, KeyManaged, KeyName}
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			t.Errorf("Required missing key %q", k)
		}
	}
	if m[KeyCluster] != "my-cluster" {
		t.Errorf("cluster tag: got %q, want %q", m[KeyCluster], "my-cluster")
	}
	if m[KeyComponent] != CompVPC {
		t.Errorf("component tag: got %q, want %q", m[KeyComponent], CompVPC)
	}
	if m[KeyManaged] != "true" {
		t.Errorf("managed tag: got %q, want true", m[KeyManaged])
	}
	if m[KeyName] != "my-cluster-vpc" {
		t.Errorf("Name tag: got %q, want %q", m[KeyName], "my-cluster-vpc")
	}
}

func TestMerge_LabelsMergeIn(t *testing.T) {
	required := Required("tracer", CompIGW)
	labels := map[string]string{"owner": "jarrod", "env": "dev"}
	extra := map[string]string{"cost-center": "RnD"}

	result := Merge(required, labels, extra)

	// Build a lookup map for assertions.
	got := make(map[string]string, len(result))
	for _, tag := range result {
		got[*tag.Key] = *tag.Value
	}

	if got["owner"] != "jarrod" {
		t.Errorf("label merge: owner got %q", got["owner"])
	}
	if got["cost-center"] != "RnD" {
		t.Errorf("extra tag: cost-center got %q", got["cost-center"])
	}
	// Required keys must still be present.
	if got[KeyCluster] != "tracer" {
		t.Errorf("required key %q lost after merge", KeyCluster)
	}
}

func TestMerge_EmptyMaps(t *testing.T) {
	result := Merge(nil, nil)
	if len(result) != 0 {
		t.Errorf("expected empty result for nil maps, got %d tags", len(result))
	}
}

func TestFilter_ProducesCorrectEC2Filter(t *testing.T) {
	f := Filter(KeyCluster, "tracer")
	if *f.Name != "tag:awsbnkctl:cluster" {
		t.Errorf("filter Name: got %q", *f.Name)
	}
	if len(f.Values) != 1 || f.Values[0] != "tracer" {
		t.Errorf("filter Values: got %v", f.Values)
	}
}

func TestClusterFilter(t *testing.T) {
	f := ClusterFilter("my-cluster")
	if *f.Name != "tag:awsbnkctl:cluster" {
		t.Errorf("ClusterFilter Name: got %q", *f.Name)
	}
	if len(f.Values) != 1 || f.Values[0] != "my-cluster" {
		t.Errorf("ClusterFilter Values: got %v", f.Values)
	}
}

func TestComponentFilter(t *testing.T) {
	f := ComponentFilter(CompSubnetPublic)
	if *f.Name != "tag:awsbnkctl:component" {
		t.Errorf("ComponentFilter Name: got %q", *f.Name)
	}
	if len(f.Values) != 1 || f.Values[0] != CompSubnetPublic {
		t.Errorf("ComponentFilter Values: got %v", f.Values)
	}
}
