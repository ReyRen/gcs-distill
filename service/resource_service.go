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
	EnableXPUIndices []int  `json:"enable_xpu_indices"`
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
	_, _ = s.gcs.ListNodes(ctx)
	return &AvailableResources{Nodes: nodesFromBrain(brain)}, nil
}

func nodesFromBrain(brain map[string]any) []AvailableNode {
	raw, _ := json.Marshal(brain)
	var decoded struct {
		Data struct {
			BrainWorkerEnableArray []string `json:"brain_workerenablearray"`
			BrainWorkers           []struct {
				Name             string `json:"brain_workers_name"`
				Address          string `json:"brain_workers_address"`
				EnableXPUIndices []int  `json:"brain_workers_enable_xpuarray"`
			} `json:"brain_workers"`
		} `json:"data"`
		BrainWorkerEnableArray []string `json:"brain_workerenablearray"`
		BrainWorkers           []struct {
			Name             string `json:"brain_workers_name"`
			Address          string `json:"brain_workers_address"`
			EnableXPUIndices []int  `json:"brain_workers_enable_xpuarray"`
		} `json:"brain_workers"`
	}
	_ = json.Unmarshal(raw, &decoded)

	enable := decoded.BrainWorkerEnableArray
	workers := decoded.BrainWorkers
	if len(decoded.Data.BrainWorkers) > 0 {
		enable = decoded.Data.BrainWorkerEnableArray
		workers = decoded.Data.BrainWorkers
	}

	enableSet := make(map[string]struct{}, len(enable))
	for _, name := range enable {
		enableSet[name] = struct{}{}
	}

	nodes := make([]AvailableNode, 0, len(workers))
	for _, worker := range workers {
		_, ok := enableSet[worker.Name]
		nodes = append(nodes, AvailableNode{
			Name:             worker.Name,
			Address:          worker.Address,
			Available:        ok,
			EnableXPUIndices: append([]int(nil), worker.EnableXPUIndices...),
			Raw:              worker,
		})
	}
	return nodes
}
