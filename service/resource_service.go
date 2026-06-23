package service

import (
	"context"
	"encoding/json"

	gcsclient "github.com/ReyRen/gcs-distill/internal/client/gcs"
)

type AvailableResources struct {
	Nodes []AvailableNode `json:"nodes"`
}

type AvailableNode struct {
	Name             string `json:"name"`
	Address          string `json:"address"`
	Available        bool   `json:"available"`
	WorkersXPUName   string `json:"workers_xpuname"`
	WorkersXPUCount  int    `json:"workers_xpucount"`
	EnableXPUIndices []int  `json:"enable_xpu_indices"`
	NodeCPUs         int64  `json:"node_cpus"`
	NodeMemory       int64  `json:"node_memory"`
	NodeState        string `json:"node_state"`
	NodeAvailability string `json:"node_availability"`
	NodeRole         string `json:"node_role"`
	NodeOS           string `json:"node_os"`
	NodeArchitecture string `json:"node_architecture"`
	Raw              any    `json:"raw"`
}

type ResourceService struct {
	gcs *gcsclient.Client
}

func NewResourceService(gcsClient *gcsclient.Client) *ResourceService {
	return &ResourceService{gcs: gcsClient}
}

func (s *ResourceService) Available(ctx context.Context) (*AvailableResources, error) {
	brain, err := s.gcs.GetBrain(ctx)
	if err != nil {
		return nil, err
	}
	nodes, _ := s.gcs.ListNodes(ctx)
	return &AvailableResources{Nodes: availableNodesFromGCS(brain, nodes)}, nil
}

type brainWorkerSnapshot struct {
	Name             string `json:"brain_workers_name"`
	Address          string `json:"brain_workers_address"`
	WorkersXPUName   string `json:"brain_workers_xpuname"`
	WorkersXPUCount  int    `json:"brain_workers_xpucount"`
	EnableXPUIndices []int  `json:"brain_workers_enable_xpuarray"`
}

type nodeSnapshot struct {
	ID           string `json:"node_id"`
	Hostname     string `json:"node_hostname"`
	Address      string `json:"node_addr"`
	CPUs         int64  `json:"node_cpus"`
	Memory       int64  `json:"node_memory"`
	State        string `json:"node_state"`
	Availability string `json:"node_availability"`
	Role         string `json:"node_role"`
	OS           string `json:"node_os"`
	Architecture string `json:"node_architecture"`
}

func availableNodesFromGCS(brain map[string]any, nodesRaw any) []AvailableNode {
	enable, workers := brainSnapshots(brain)

	enableSet := make(map[string]struct{}, len(enable))
	for _, name := range enable {
		enableSet[name] = struct{}{}
	}

	nodeIndex := indexNodeSnapshots(nodesRaw)
	nodes := make([]AvailableNode, 0, len(workers))
	for _, worker := range workers {
		_, ok := enableSet[worker.Name]
		node, hasNode := matchNodeSnapshot(worker, nodeIndex)
		raw := map[string]any{"brain_worker": worker}
		if hasNode {
			raw["node"] = node
		}

		nodes = append(nodes, AvailableNode{
			Name:             worker.Name,
			Address:          worker.Address,
			Available:        ok,
			WorkersXPUName:   worker.WorkersXPUName,
			WorkersXPUCount:  worker.WorkersXPUCount,
			EnableXPUIndices: append([]int(nil), worker.EnableXPUIndices...),
			NodeCPUs:         node.CPUs,
			NodeMemory:       node.Memory,
			NodeState:        node.State,
			NodeAvailability: node.Availability,
			NodeRole:         node.Role,
			NodeOS:           node.OS,
			NodeArchitecture: node.Architecture,
			Raw:              raw,
		})
	}
	return nodes
}

func brainSnapshots(brain map[string]any) ([]string, []brainWorkerSnapshot) {
	var decoded struct {
		Data struct {
			BrainWorkerEnableArray []string              `json:"brain_workerenablearray"`
			BrainWorkers           []brainWorkerSnapshot `json:"brain_workers"`
		} `json:"data"`
		BrainWorkerEnableArray []string              `json:"brain_workerenablearray"`
		BrainWorkers           []brainWorkerSnapshot `json:"brain_workers"`
	}

	raw, _ := json.Marshal(brain)
	_ = json.Unmarshal(raw, &decoded)
	if len(decoded.Data.BrainWorkers) > 0 {
		return decoded.Data.BrainWorkerEnableArray, decoded.Data.BrainWorkers
	}
	return decoded.BrainWorkerEnableArray, decoded.BrainWorkers
}

func indexNodeSnapshots(raw any) map[string]nodeSnapshot {
	index := make(map[string]nodeSnapshot)
	for _, node := range nodeSnapshots(raw) {
		for _, key := range []string{node.Hostname, node.Address, node.ID} {
			if key == "" {
				continue
			}
			if _, exists := index[key]; !exists {
				index[key] = node
			}
		}
	}
	return index
}

func matchNodeSnapshot(worker brainWorkerSnapshot, index map[string]nodeSnapshot) (nodeSnapshot, bool) {
	for _, key := range []string{worker.Name, worker.Address} {
		if key == "" {
			continue
		}
		if node, ok := index[key]; ok {
			return node, true
		}
	}
	return nodeSnapshot{}, false
}

func nodeSnapshots(value any) []nodeSnapshot {
	raw, _ := json.Marshal(value)
	var nodes []nodeSnapshot
	if err := json.Unmarshal(raw, &nodes); err == nil && len(nodes) > 0 {
		return nodes
	}

	var decoded struct {
		Items []nodeSnapshot `json:"items"`
		Nodes []nodeSnapshot `json:"nodes"`
		Data  struct {
			Items []nodeSnapshot `json:"items"`
			Nodes []nodeSnapshot `json:"nodes"`
		} `json:"data"`
	}
	_ = json.Unmarshal(raw, &decoded)
	switch {
	case len(decoded.Items) > 0:
		return decoded.Items
	case len(decoded.Nodes) > 0:
		return decoded.Nodes
	case len(decoded.Data.Items) > 0:
		return decoded.Data.Items
	default:
		return decoded.Data.Nodes
	}
}
