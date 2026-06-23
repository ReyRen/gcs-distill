package service

import "testing"

func TestNodesFromBrainRootShape(t *testing.T) {
	nodes := nodesFromBrain(map[string]any{
		"brain_workerenablearray": []any{"gpu-node-01"},
		"brain_workers": []any{
			map[string]any{
				"brain_workers_name":            "gpu-node-01",
				"brain_workers_address":         "172.18.36.225",
				"brain_workers_enable_xpuarray": []any{float64(0), float64(2)},
			},
			map[string]any{
				"brain_workers_name":            "gpu-node-02",
				"brain_workers_address":         "172.18.36.226",
				"brain_workers_enable_xpuarray": []any{float64(1)},
			},
		},
	})

	if len(nodes) != 2 {
		t.Fatalf("len(nodes) = %d, want 2", len(nodes))
	}
	if !nodes[0].Available {
		t.Fatalf("nodes[0].Available = false, want true")
	}
	if nodes[1].Available {
		t.Fatalf("nodes[1].Available = true, want false")
	}
	if got := nodes[0].EnableXPUIndices; len(got) != 2 || got[0] != 0 || got[1] != 2 {
		t.Fatalf("nodes[0].EnableXPUIndices = %#v, want [0 2]", got)
	}
}

func TestNodesFromBrainDataShape(t *testing.T) {
	nodes := nodesFromBrain(map[string]any{
		"data": map[string]any{
			"brain_workerenablearray": []any{"gpu-node-03"},
			"brain_workers": []any{
				map[string]any{
					"brain_workers_name":            "gpu-node-03",
					"brain_workers_address":         "172.18.36.227",
					"brain_workers_enable_xpuarray": []any{float64(0)},
				},
			},
		},
	})

	if len(nodes) != 1 {
		t.Fatalf("len(nodes) = %d, want 1", len(nodes))
	}
	if nodes[0].Name != "gpu-node-03" || nodes[0].Address != "172.18.36.227" || !nodes[0].Available {
		t.Fatalf("unexpected node: %#v", nodes[0])
	}
}
