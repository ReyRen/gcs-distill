package service

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	gcsclient "github.com/ReyRen/gcs-distill/internal/client/gcs"
	"github.com/ReyRen/gcs-distill/internal/types"
)

type AvailableResources struct {
	Nodes            []AvailableNode `json:"nodes"`
	TrainingScope    string          `json:"training_scope"`
	MaxTrainingNodes int             `json:"max_training_nodes"`
	TelemetryError   string          `json:"telemetry_error,omitempty"`
}

type AvailableNode struct {
	Devices           []AvailableDevice `json:"devices"`
	ObservedAt        *time.Time        `json:"observed_at"`
	TrainingSupported bool              `json:"training_supported"`
	DataSupported     bool              `json:"data_supported"`
	Name              string            `json:"name"`
	Address           string            `json:"address"`
	Available         bool              `json:"available"`
	WorkersXPUName    string            `json:"workers_xpuname"`
	WorkersXPUCount   int               `json:"workers_xpucount"`
	EnableXPUIndices  []int             `json:"enable_xpu_indices"`
	NodeCPUs          int64             `json:"node_cpus"`
	NodeMemory        int64             `json:"node_memory"`
	NodeState         string            `json:"node_state"`
	NodeAvailability  string            `json:"node_availability"`
	NodeRole          string            `json:"node_role"`
	NodeOS            string            `json:"node_os"`
	NodeArchitecture  string            `json:"node_architecture"`
	Raw               any               `json:"raw"`
}

type ResourceService struct {
	gcs         *gcsclient.Client
	dataWorkers []string
}

func NewResourceService(gcsClient *gcsclient.Client, dataWorkers []string) *ResourceService {
	return &ResourceService{gcs: gcsClient, dataWorkers: append([]string(nil), dataWorkers...)}
}

func (s *ResourceService) Available(ctx context.Context) (*AvailableResources, error) {
	brain, err := s.gcs.GetBrain(ctx)
	if err != nil {
		return nil, err
	}
	nodes, err := s.gcs.ListNodes(ctx)
	if err != nil {
		return nil, err
	}
	out := &AvailableResources{Nodes: availableNodesFromGCS(brain, nodes), TrainingScope: "single_node_multi_gpu", MaxTrainingNodes: 1}
	inventory, telemetryErr := s.gcs.GetResourceInventory(ctx)
	if telemetryErr != nil {
		out.TelemetryError = "读取卡级监控失败，请重试"
	}
	enrichResourceNodes(out.Nodes, inventory, s.dataWorkers, time.Now())
	return out, nil
}

type AvailableDevice struct {
	gcsclient.DeviceResource
	SchedulerAvailable bool   `json:"scheduler_available"`
	Selectable         bool   `json:"selectable"`
	State              string `json:"state"`
}

func enrichResourceNodes(nodes []AvailableNode, inventory *gcsclient.ResourceInventory, dataWorkers []string, now time.Time) {
	measurements := map[string]gcsclient.NodeResources{}
	if inventory != nil {
		for _, node := range inventory.Nodes {
			measurements[node.Name] = node
		}
	}
	for i := range nodes {
		node := &nodes[i]
		measured := measurements[node.Name]
		node.ObservedAt = measured.ObservedAt
		node.TrainingSupported = (node.NodeArchitecture == "amd64" || node.NodeArchitecture == "x86_64") && strings.Contains(strings.ToLower(node.WorkersXPUName), "nvidia")
		node.DataSupported = node.Available && slices.Contains(dataWorkers, node.Name)
		cards := map[int]gcsclient.DeviceResource{}
		indices := make([]int, 0, node.WorkersXPUCount)
		for index := 0; index < node.WorkersXPUCount; index++ {
			indices = append(indices, index)
		}
		for _, device := range measured.Devices {
			cards[device.Index] = device
			if device.Index >= 0 && !slices.Contains(indices, device.Index) {
				indices = append(indices, device.Index)
			}
		}
		slices.Sort(indices)
		node.Devices = make([]AvailableDevice, 0, len(indices))
		for _, index := range indices {
			device := cards[index]
			device.Index = index
			if device.Name == "" {
				device.Name = node.WorkersXPUName
			}
			card := AvailableDevice{DeviceResource: device, SchedulerAvailable: slices.Contains(node.EnableXPUIndices, index)}
			switch {
			case !node.Available || node.NodeState != "ready" || node.NodeAvailability != "active":
				card.State = "node_unavailable"
			case !node.TrainingSupported:
				card.State = "unsupported_runtime"
			default:
				card.State = device.UnavailableReason(measured.ObservedAt, now, true)
				if card.State == "" && !card.SchedulerAvailable {
					card.State = "reserved"
				}
				if card.State == "" {
					card.State = "available"
					card.Selectable = true
				}
			}
			node.Devices = append(node.Devices, card)
		}
	}
}

func (s *ResourceService) Validate(ctx context.Context, request *types.ResourceRequest, train, requireIdle bool) error {
	if err := types.NormalizeResourceRequest(request, train); err != nil {
		return err
	}
	for _, node := range request.ExecutorNodes {
		if !slices.Contains(s.dataWorkers, node) {
			return fmt.Errorf("数据执行节点不在已配置的离线节点目录中: %s", node)
		}
	}
	if !train {
		return nil
	}
	selected := make([]gcsclient.SelectedResource, len(request.SelectedResources))
	for i, node := range request.SelectedResources {
		selected[i] = gcsclient.SelectedResource{NodeName: node.NodeName, NodeAddress: node.NodeAddress, XPUIndices: node.XPUIndices}
	}
	return s.gcs.ValidateGPUSelection(ctx, selected, requireIdle)
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
