package service

import "testing"

func TestNodesFromBrainRootShape(t *testing.T) {
	nodes := availableNodesFromGCS(map[string]any{
		"brain_workerenablearray": []any{"gpu-node-01"},
		"brain_workers": []any{
			map[string]any{
				"brain_workers_name":            "gpu-node-01",
				"brain_workers_address":         "172.18.36.225",
				"brain_workers_xpuname":         "NVIDIA A100",
				"brain_workers_xpucount":        float64(4),
				"brain_workers_enable_xpuarray": []any{float64(0), float64(2)},
			},
			map[string]any{
				"brain_workers_name":            "gpu-node-02",
				"brain_workers_address":         "172.18.36.226",
				"brain_workers_xpuname":         "NVIDIA L40S",
				"brain_workers_xpucount":        float64(8),
				"brain_workers_enable_xpuarray": []any{float64(1)},
			},
		},
	}, []any{
		map[string]any{
			"node_hostname":     "gpu-node-01",
			"node_addr":         "172.18.36.225",
			"node_cpus":         float64(64),
			"node_memory":       float64(540000000000),
			"node_state":        "ready",
			"node_availability": "active",
			"node_role":         "worker",
			"node_os":           "linux",
			"node_architecture": "x86_64",
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
	if nodes[0].WorkersXPUName != "NVIDIA A100" || nodes[0].WorkersXPUCount != 4 {
		t.Fatalf("unexpected worker xpu fields: %#v", nodes[0])
	}
	if nodes[0].NodeCPUs != 64 || nodes[0].NodeMemory != 540000000000 || nodes[0].NodeState != "ready" {
		t.Fatalf("unexpected node fields: %#v", nodes[0])
	}
	if nodes[1].WorkersXPUName != "NVIDIA L40S" || nodes[1].NodeCPUs != 0 {
		t.Fatalf("unexpected unmatched node fields: %#v", nodes[1])
	}
}

func TestNodesFromBrainDataShape(t *testing.T) {
	nodes := availableNodesFromGCS(map[string]any{
		"data": map[string]any{
			"brain_workerenablearray": []any{"gpu-node-03"},
			"brain_workers": []any{
				map[string]any{
					"brain_workers_name":            "gpu-node-03",
					"brain_workers_address":         "172.18.36.227",
					"brain_workers_xpuname":         "Ascend 910B",
					"brain_workers_xpucount":        float64(8),
					"brain_workers_enable_xpuarray": []any{float64(0)},
				},
			},
		},
	}, map[string]any{
		"data": map[string]any{
			"nodes": []any{
				map[string]any{
					"node_hostname":     "another-name",
					"node_addr":         "172.18.36.227",
					"node_cpus":         float64(96),
					"node_state":        "ready",
					"node_availability": "active",
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
	if nodes[0].WorkersXPUName != "Ascend 910B" || nodes[0].WorkersXPUCount != 8 || nodes[0].NodeCPUs != 96 {
		t.Fatalf("unexpected merged fields: %#v", nodes[0])
	}
}
