package gcs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ReyRen/gcs-distill/internal/config"
)

type ResourceInventory struct {
	Nodes []NodeResources `json:"nodes"`
}

type NodeResources struct {
	Name           string           `json:"name"`
	Architecture   string           `json:"architecture"`
	ObservedAt     *time.Time       `json:"observed_at"`
	Devices        []DeviceResource `json:"devices"`
	TelemetryError string           `json:"telemetry_error,omitempty"`
}

type DeviceResource struct {
	Index           int     `json:"index"`
	Name            string  `json:"name"`
	UUID            string  `json:"uuid"`
	MemoryTotalMiB  *uint64 `json:"memory_total_mib"`
	MemoryUsedMiB   *uint64 `json:"memory_used_mib"`
	MemoryFreeMiB   *uint64 `json:"memory_free_mib"`
	Utilization     *uint32 `json:"utilization_percent"`
	ProcessCount    *int    `json:"process_count"`
	HardwareEnabled *bool   `json:"hardware_enabled"`
	SoftwareEnabled *bool   `json:"software_enabled"`
}

func (c *Client) GetResourceInventory(ctx context.Context) (*ResourceInventory, error) {
	var out ResourceInventory
	if err := c.doJSON(ctx, http.MethodGet, "/resources", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UnavailableReason distinguishes unknown measurements from a measured zero.
func (d DeviceResource) UnavailableReason(observedAt *time.Time, now time.Time, requireIdle bool) string {
	if observedAt == nil || observedAt.After(now.Add(30*time.Second)) {
		return "telemetry_unknown"
	}
	if now.Sub(*observedAt) > 2*time.Minute {
		return "telemetry_stale"
	}
	if d.HardwareEnabled == nil {
		return "telemetry_unknown"
	}
	if !*d.HardwareEnabled {
		return "hardware_unavailable"
	}
	if d.MemoryTotalMiB == nil || *d.MemoryTotalMiB == 0 {
		return "telemetry_unknown"
	}
	if requireIdle {
		if d.ProcessCount == nil || d.SoftwareEnabled == nil {
			return "telemetry_unknown"
		}
		if !*d.SoftwareEnabled {
			return "reserved"
		}
		if *d.ProcessCount > 0 || d.Utilization != nil && *d.Utilization > 0 {
			return "in_use"
		}
	}
	return ""
}

// ValidateTrainingSelection checks a snapshot only. The GCS allocator still
// acquires the final reservation atomically when the training container starts.
func (c *Client) ValidateTrainingSelection(ctx context.Context, selected []SelectedResource, requireIdle bool, target config.TrainingRuntime) error {
	label := target.DeviceLabel()
	if len(selected) != 1 || selected[0].NodeName == "" || len(selected[0].XPUIndices) == 0 {
		return fmt.Errorf("请选择一台训练节点及至少一张 %s", label)
	}
	if !target.AllowsWorker(selected[0].NodeName) {
		return fmt.Errorf("训练节点未配置对应的训练镜像: %s", selected[0].NodeName)
	}
	brain, err := c.GetBrain(ctx)
	if err != nil {
		return err
	}
	var pool struct {
		Workers []struct {
			Name    string `json:"brain_workers_name"`
			Address string `json:"brain_workers_address"`
			Enabled []int  `json:"brain_workers_enable_xpuarray"`
		} `json:"brain_workers"`
		Nodes []string `json:"brain_workerenablearray"`
	}
	if nested, ok := brain["data"].(map[string]any); ok {
		brain = nested
	}
	encoded, _ := json.Marshal(brain)
	if err := json.Unmarshal(encoded, &pool); err != nil {
		return fmt.Errorf("资源池返回格式无效")
	}
	selection := selected[0]
	var found bool
	var enabled map[int]bool
	for _, name := range pool.Nodes {
		if name == selection.NodeName {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("训练节点不可调度: %s", selection.NodeName)
	}
	for _, worker := range pool.Workers {
		if worker.Name != selection.NodeName {
			continue
		}
		if selection.NodeAddress != "" && selection.NodeAddress != worker.Address {
			return fmt.Errorf("训练节点地址与资源目录不一致")
		}
		enabled = make(map[int]bool, len(worker.Enabled))
		for _, index := range worker.Enabled {
			enabled[index] = true
		}
	}
	inventory, err := c.GetResourceInventory(ctx)
	if err != nil {
		return fmt.Errorf("读取 %s 监控失败: %w", label, err)
	}
	for _, node := range inventory.Nodes {
		if node.Name != selection.NodeName {
			continue
		}
		cards := make(map[int]DeviceResource, len(node.Devices))
		for _, card := range node.Devices {
			cards[card.Index] = card
		}
		seen := map[int]bool{}
		for _, index := range selection.XPUIndices {
			card, exists := cards[index]
			if index < 0 || seen[index] || !exists {
				return fmt.Errorf("%s 卡号不存在或重复: %d", label, index)
			}
			seen[index] = true
			if !target.Supports(node.Architecture, card.Name) {
				return fmt.Errorf("当前训练镜像需要 %s %s，节点 %s 的卡 %d 不匹配", target.Architecture(), label, node.Name, index)
			}
			if requireIdle && !enabled[index] {
				return fmt.Errorf("%s %d 已被占用或不可调度", label, index)
			}
			if reason := card.UnavailableReason(node.ObservedAt, time.Now(), requireIdle); reason != "" {
				return fmt.Errorf("%s %d 暂不可用: %s", label, index, reason)
			}
		}
		return nil
	}
	return fmt.Errorf("训练节点缺少 %s 监控: %s", label, selection.NodeName)
}
