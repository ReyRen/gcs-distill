package types

import "testing"

func TestSingleNodeSelectionValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		req  ResourceRequest
	}{
		{"automatic", ResourceRequest{GPUCount: 2}},
		{"multiple_nodes", ResourceRequest{SelectedResources: []SelectedResource{{NodeName: "a", XPUIndices: []int{0}}, {NodeName: "b", XPUIndices: []int{1}}}}},
		{"empty_node", ResourceRequest{SelectedResources: []SelectedResource{{XPUIndices: []int{0}}}}},
		{"empty_cards", ResourceRequest{SelectedResources: []SelectedResource{{NodeName: "a"}}}},
		{"duplicate", ResourceRequest{SelectedResources: []SelectedResource{{NodeName: "a", XPUIndices: []int{1, 1}}}}},
		{"negative", ResourceRequest{SelectedResources: []SelectedResource{{NodeName: "a", XPUIndices: []int{-1}}}}},
		{"count_mismatch", ResourceRequest{GPUCount: 1, SelectedResources: []SelectedResource{{NodeName: "a", XPUIndices: []int{6, 7}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := NormalizeResourceRequest(&tc.req, true); err == nil {
				t.Fatal("invalid explicit selection accepted")
			}
		})
	}
	req := ResourceRequest{SelectedResources: []SelectedResource{{NodeName: " a ", XPUIndices: []int{6, 7}}}}
	if err := NormalizeResourceRequest(&req, true); err != nil || req.GPUCount != 2 || req.SelectedResources[0].NodeName != "a" {
		t.Fatalf("selection not normalized: %+v, %v", req, err)
	}
	req.ExecutorNodes = []string{"cpu"}
	if err := NormalizeResourceRequest(&req, false); err != nil || req.GPUCount != 0 || req.SelectedResources != nil || req.ExecutorNodes[0] != "cpu" {
		t.Fatalf("data-only request retained training resources: %+v", req)
	}
}
