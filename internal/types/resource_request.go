package types

import (
	"fmt"
	"strings"
)

func NormalizeResourceRequest(request *ResourceRequest, train bool) error {
	if len(request.ExecutorNodes) > 1 {
		return fmt.Errorf("请选择一个数据执行节点")
	}
	for i, node := range request.ExecutorNodes {
		request.ExecutorNodes[i] = strings.TrimSpace(node)
		if request.ExecutorNodes[i] == "" {
			return fmt.Errorf("数据执行节点名称不能为空")
		}
	}
	if !train {
		request.GPUCount = 0
		request.SelectedResources = nil
		return nil
	}
	if len(request.SelectedResources) != 1 {
		return fmt.Errorf("当前学生训练支持单节点多 GPU，请只选择一个训练节点")
	}
	selected := &request.SelectedResources[0]
	selected.NodeName = strings.TrimSpace(selected.NodeName)
	selected.NodeAddress = strings.TrimSpace(selected.NodeAddress)
	if selected.NodeName == "" || len(selected.XPUIndices) == 0 {
		return fmt.Errorf("请选择训练节点及至少一张 GPU")
	}
	seen := map[int]bool{}
	for _, index := range selected.XPUIndices {
		if index < 0 || seen[index] {
			return fmt.Errorf("GPU 卡号不能为负数或重复")
		}
		seen[index] = true
	}
	if request.GPUCount != 0 && request.GPUCount != len(selected.XPUIndices) {
		return fmt.Errorf("GPU 数量必须与所选卡号数量一致")
	}
	request.GPUCount = len(selected.XPUIndices)
	return nil
}
